package domain

import (
	"errors"
	"strings"
)

var (
	ErrSymbolNotFound         = errors.New("symbol not found")
	ErrInvalidQueryParameter  = errors.New("invalid query parameter")
	ErrInvalidSymbolNode      = errors.New("invalid symbol node")
	ErrInvalidEdge            = errors.New("invalid edge")
	ErrDatabaseNotInitialized = errors.New("database not initialized")
)

type SymbolRef string

const (
	KindPackage   = "package"
	KindStruct    = "struct"
	KindInterface = "interface"
	KindFunc      = "func"
	KindMethod    = "method"
	KindField     = "field"
	KindTypeAlias = "type_alias"
)

const (
	EdgeContains   = "CONTAINS"
	EdgeReceiverOf = "RECEIVER_OF"
	EdgeCalls      = "CALLS"
	EdgeImplements = "IMPLEMENTS"
	EdgeImports    = "IMPORTS"
)

type HierarchyDirection string

const (
	DirectionUpstream   HierarchyDirection = "UPSTREAM"
	DirectionDownstream HierarchyDirection = "DOWNSTREAM"
)

// SymbolNode represents a structural code entity.
type SymbolNode struct {
	ID         SymbolRef `json:"id"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	Package    string    `json:"package"`
	File       string    `json:"file"`
	LineStart  int       `json:"line_start"`
	LineEnd    int       `json:"line_end"`
	Signature  string    `json:"signature,omitempty"`
	Doc        string    `json:"doc,omitempty"`
	IsExported bool      `json:"is_exported"`
}

func (n SymbolNode) Validate() error {
	if strings.TrimSpace(string(n.ID)) == "" {
		return errors.New("node ID cannot be empty")
	}
	if strings.TrimSpace(n.Kind) == "" {
		return errors.New("node kind cannot be empty")
	}
	if strings.TrimSpace(n.Name) == "" {
		return errors.New("node name cannot be empty")
	}
	if n.LineStart < 0 || n.LineEnd < 0 || (n.LineStart > n.LineEnd && n.LineEnd != 0) {
		return errors.New("invalid line range")
	}
	return nil
}

// Edge represents a directed relationship between two SymbolNodes.
type Edge struct {
	ID       string    `json:"id"`
	SourceID SymbolRef `json:"source_id"`
	TargetID SymbolRef `json:"target_id"`
	Kind     string    `json:"kind"`
	File     string    `json:"file,omitempty"`
	Line     int       `json:"line,omitempty"`
}

func (e Edge) Validate() error {
	if strings.TrimSpace(string(e.SourceID)) == "" {
		return errors.New("edge source ID cannot be empty")
	}
	if strings.TrimSpace(string(e.TargetID)) == "" {
		return errors.New("edge target ID cannot be empty")
	}
	switch e.Kind {
	case EdgeContains, EdgeReceiverOf, EdgeCalls, EdgeImplements, EdgeImports:
		return nil
	default:
		return errors.New("invalid edge kind: " + e.Kind)
	}
}

// CallerResult models an incoming call reference.
type CallerResult struct {
	Caller   SymbolNode `json:"caller"`
	CallFile string     `json:"call_file"`
	CallLine int        `json:"call_line"`
}

// CalleeResult models an outgoing call reference.
type CalleeResult struct {
	Callee   SymbolNode `json:"callee"`
	CallFile string     `json:"call_file"`
	CallLine int        `json:"call_line"`
}

// CallHierarchyNode models a single node in a call tree.
type CallHierarchyNode struct {
	Symbol     SymbolNode           `json:"symbol"`
	CallSite   string               `json:"call_site,omitempty"`
	CallLine   int                  `json:"call_line,omitempty"`
	IsCircular bool                 `json:"is_circular,omitempty"`
	Children   []*CallHierarchyNode `json:"children,omitempty"`
}

// CallHierarchyTree represents a multi-level caller or callee graph.
type CallHierarchyTree struct {
	Root      SymbolNode           `json:"root"`
	Direction HierarchyDirection   `json:"direction"`
	Nodes     []*CallHierarchyNode `json:"nodes"`
}

// FieldInfo describes a struct field.
type FieldInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Doc        string `json:"doc,omitempty"`
	IsExported bool   `json:"is_exported"`
}

// MethodInfo describes a struct or interface method.
type MethodInfo struct {
	Name       string `json:"name"`
	Signature  string `json:"signature"`
	Doc        string `json:"doc,omitempty"`
	IsExported bool   `json:"is_exported"`
}

// StructDetails represents all structural aspects of a struct.
type StructDetails struct {
	Node       SymbolNode   `json:"node"`
	Fields     []FieldInfo  `json:"fields"`
	Methods    []MethodInfo `json:"methods"`
	Implements []SymbolNode `json:"implements"`
}

// SymbolFilter specifies criteria for querying symbols.
type SymbolFilter struct {
	Name         string `json:"name,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Package      string `json:"package,omitempty"`
	File         string `json:"file,omitempty"`
	ExportedOnly bool   `json:"exported_only,omitempty"`
}

func (f SymbolFilter) Matches(n SymbolNode) bool {
	if f.Name != "" && n.Name != f.Name {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(n.Name, f.Prefix) {
		return false
	}
	if f.Kind != "" && n.Kind != f.Kind {
		return false
	}
	if f.Package != "" && n.Package != f.Package {
		return false
	}
	if f.File != "" && n.File != f.File {
		return false
	}
	if f.ExportedOnly && !n.IsExported {
		return false
	}
	return true
}
