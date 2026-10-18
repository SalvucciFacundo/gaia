package codemode

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"gaia/internal/core"
)

// Engine executes dynamic Go code snippets in a sandboxed interpreter.
type Engine struct {
	projectRoot string
	registry    *core.ToolRegistry
	timeout     time.Duration
}

// NewEngine creates an Engine instance with a default 15s timeout.
func NewEngine(projectRoot string, registry *core.ToolRegistry) *Engine {
	return &Engine{
		projectRoot: projectRoot,
		registry:    registry,
		timeout:     15 * time.Second,
	}
}

// SetTimeout adjusts the script execution deadline.
func (e *Engine) SetTimeout(d time.Duration) {
	if d > 0 {
		e.timeout = d
	}
}

// wrapCode wraps bare Go statements into a valid Go source file if needed.
func (e *Engine) wrapCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, "package ") {
		return trimmed
	}

	// Bare statements: wrap in package main, common imports, and func main()
	return fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"regexp"
	"time"
	"gaia/gaia"
)

func main() {
	%s
}
`, trimmed)
}

// Execute runs the provided Go code, captures stdout, and handles panics/timeouts.
func (e *Engine) Execute(ctx context.Context, code string) (output string, err error) {
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	sdk := NewGaiaSDK(e.projectRoot, e.registry, &stdoutBuf, execCtx)

	i := interp.New(interp.Options{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})

	// Load standard Go library symbols
	if err := i.Use(stdlib.Symbols); err != nil {
		return "", fmt.Errorf("loading stdlib symbols: %w", err)
	}

	// Register gaia/gaia package symbols
	gaiaSymbols := map[string]map[string]reflect.Value{
		"gaia/gaia": {
			"ReadFile":  reflect.ValueOf(sdk.ReadFile),
			"WriteFile": reflect.ValueOf(sdk.WriteFile),
			"Find":      reflect.ValueOf(sdk.Find),
			"Grep":      reflect.ValueOf(sdk.Grep),
			"Exec":      reflect.ValueOf(sdk.Exec),
			"Print":     reflect.ValueOf(sdk.Print),
			"Printf":    reflect.ValueOf(sdk.Printf),
		},
	}
	if err := i.Use(gaiaSymbols); err != nil {
		return "", fmt.Errorf("registering gaia symbols: %w", err)
	}

	src := e.wrapCode(code)

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runtime panic: %v", r)
			}
		}()
		_, evalErr := i.EvalWithContext(execCtx, src)
		done <- evalErr
	}()

	select {
	case <-execCtx.Done():
		if execCtx.Err() == context.DeadlineExceeded {
			return stdoutBuf.String(), fmt.Errorf("script execution timed out after %v", e.timeout)
		}
		return stdoutBuf.String(), execCtx.Err()
	case evalErr := <-done:
		if evalErr != nil {
			errOutput := stderrBuf.String()
			if errOutput != "" {
				return stdoutBuf.String(), fmt.Errorf("%w\nStderr: %s", evalErr, strings.TrimSpace(errOutput))
			}
			return stdoutBuf.String(), evalErr
		}
	}

	outStr := strings.TrimRight(stdoutBuf.String(), "\r\n")
	return outStr, nil
}
