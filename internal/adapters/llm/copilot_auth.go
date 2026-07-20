package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/oauth/device"
)

const (
	CopilotClientID = "Iv1.b507a08c87ecfe98"
	CopilotScope     = "read:user"
)

type GitHubAuth struct {
	httpClient *http.Client
}

func NewGitHubAuth() *GitHubAuth {
	return &GitHubAuth{
		httpClient: &http.Client{},
	}
}

// RequestDeviceCode starts the device flow and returns the code info
func (a *GitHubAuth) RequestDeviceCode(ctx context.Context) (*device.CodeResponse, error) {
	return device.RequestCode(a.httpClient, "https://github.com/login/device/code", CopilotClientID, []string{CopilotScope})
}

// WaitToken polls for the access token
func (a *GitHubAuth) WaitToken(ctx context.Context, code *device.CodeResponse) (string, error) {
	token, err := device.PollToken(a.httpClient, "https://github.com/login/oauth/access_token", CopilotClientID, code)
	if err != nil {
		return "", err
	}

	return token.Token, nil
}

// ExchangeCopilotToken swaps a GitHub token for a Copilot session token
func (a *GitHubAuth) ExchangeCopilotToken(ctx context.Context, githubToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/copilot_internal/v2/token", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("User-Agent", "GAIA-Agent/0.1")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to exchange token, status: %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}
