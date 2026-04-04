package calendar

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// RunLoopbackAuth performs a complete loopback OAuth2 authorization flow.
// It binds an ephemeral TCP port, sets cfg.RedirectURL accordingly, generates a
// CSRF state token, starts a short-lived callback server, and calls onAuthURL
// with the authorization URL (so the caller can open a browser or print it).
// The function blocks until the user authorizes (or denies), then exchanges the
// code for a token and returns it. Times out after 5 minutes.
func RunLoopbackAuth(ctx context.Context, cfg *oauth2.Config, onAuthURL func(authURL string)) (*oauth2.Token, error) {
	// Bind before building the redirect URL to avoid a TOCTOU race where another
	// process grabs the ephemeral port between probe and serve.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("cannot listen on localhost: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", port)

	// Generate a cryptographically random CSRF state token.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("cannot generate state token: %w", err)
	}
	oauthState := base64.RawURLEncoding.EncodeToString(stateBytes)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != oauthState {
			select {
			case errCh <- fmt.Errorf("state mismatch in OAuth callback"):
			default:
			}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			select {
			case errCh <- fmt.Errorf("no code in callback"):
			default:
			}
			fmt.Fprintln(w, "Error: no authorization code received.")
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		fmt.Fprintln(w, "Authorization successful! You can close this tab.")
	})

	srv := &http.Server{Handler: mux}
	// Serve returns http.ErrServerClosed on clean shutdown — expected.
	go srv.Serve(listener) //nolint:errcheck
	// Shutdown error is safe to ignore: this single-use server exits seconds after
	// receiving the OAuth code regardless of whether Shutdown returns an error.
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	authURL := cfg.AuthCodeURL(oauthState, oauth2.AccessTypeOffline)
	onAuthURL(authURL)

	select {
	case code := <-codeCh:
		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange code for token: %w", err)
		}
		return tok, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
