package calendar

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const calendarReadonlyScope = "https://www.googleapis.com/auth/calendar.readonly"

// ParseCredentials reads a Google Cloud credentials.json and returns an oauth2 config.
func ParseCredentials(path string) (*oauth2.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, calendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return cfg, nil
}

// loadToken reads a cached oauth2.Token from disk.
func loadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	return &tok, nil
}

// SaveToken writes an oauth2.Token to disk with restrictive permissions.
func SaveToken(path string, tok *oauth2.Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// persistingTokenSource wraps an oauth2.TokenSource and writes the token to
// disk whenever the access token changes, ensuring long-running processes
// survive token rotation across restarts.
type persistingTokenSource struct {
	mu        sync.Mutex
	inner     oauth2.TokenSource
	tokenPath string
	last      string // last seen access token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.inner.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	changed := tok.AccessToken != p.last
	if changed {
		p.last = tok.AccessToken
	}
	p.mu.Unlock()
	if changed {
		if saveErr := SaveToken(p.tokenPath, tok); saveErr != nil {
			calLog.Warn("failed to persist refreshed token",
				"path", p.tokenPath, "error", saveErr.Error())
		}
	}
	return tok, nil
}
