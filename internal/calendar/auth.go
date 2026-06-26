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

const authTimeout = 5 * time.Minute

// RunLoopbackAuth performs a complete loopback OAuth2 authorization flow.
// It binds an ephemeral TCP port, generates a CSRF state token, starts a
// short-lived callback server, and calls onAuthURL with the authorization URL
// (so the caller can open a browser or print it). The function blocks until
// the user authorizes (or denies), then exchanges the code for a token and
// returns it. Times out after 5 minutes — the timeout applies to the entire
// flow including the code exchange. The caller's *oauth2.Config is not modified.
func RunLoopbackAuth(ctx context.Context, cfg *oauth2.Config, onAuthURL func(authURL string)) (*oauth2.Token, error) {
	// Enforce the documented 5-minute timeout over the entire flow so that the
	// code exchange and server shutdown also respect it, not just the callback wait.
	flowCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	// Bind before building the redirect URL to avoid a TOCTOU race where another
	// process grabs the ephemeral port between probe and serve.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("cannot listen on localhost: %w", err)
	}
	defer listener.Close() //nolint:errcheck

	port := listener.Addr().(*net.TCPAddr).Port
	localCfg := *cfg
	localCfg.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", port)
	cfg = &localCfg

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
	// Give shutdown a short deadline; this single-use server exits right after
	// the OAuth code is received so the 5s window is always sufficient.
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	authURL := cfg.AuthCodeURL(oauthState, oauth2.AccessTypeOffline)
	onAuthURL(authURL)

	select {
	case code := <-codeCh:
		tok, err := cfg.Exchange(flowCtx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange code for token: %w", err)
		}
		return tok, nil
	case err := <-errCh:
		return nil, err
	case <-flowCtx.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("authorization timed out after %s", authTimeout)
	}
}
