package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	spoo "github.com/spoo-me/spoo-go"
)

const (
	// AppID matches the spoo-cli entry in the backend's config/apps.yaml.
	AppID = "spoo-cli"
	// CallbackAddr is fixed: the backend validates redirect_uri by exact
	// match against the registry, so a random port would be rejected.
	CallbackAddr = "127.0.0.1:53682"
	CallbackPath = "/callback"
)

const successHTML = `<!doctype html><html><body style="font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;background:#0f0f14;color:#e5e5ea">
<div style="text-align:center"><h1 style="color:#a78bfa">&#10003; spoo CLI authorized</h1><p>You can close this tab and return to your terminal.</p></div>
</body></html>`

// DeviceFlow drives the spoo.me connected-apps device authorization:
// loopback server → browser consent → one-time code. The protocol
// pieces (PKCE, state, the auth URL) come from the SDK; the browser
// leg and the loopback listener stay here — they are platform
// concerns the SDK deliberately leaves to the app.
type DeviceFlow struct {
	Client      *spoo.Client
	OpenBrowser func(url string) error
	Out         io.Writer // progress messages (stderr)
}

// Run blocks until the consent callback delivers a code, the context
// expires, or the callback is invalid. Returns the one-time code and the
// PKCE verifier that must accompany it in the token exchange.
func (f *DeviceFlow) Run(ctx context.Context) (string, string, error) {
	state, err := spoo.GenerateState()
	if err != nil {
		return "", "", err
	}
	verifier, err := spoo.GenerateCodeVerifier()
	if err != nil {
		return "", "", err
	}

	ln, err := net.Listen("tcp", CallbackAddr)
	if err != nil {
		return "", "", fmt.Errorf("cannot listen on %s (is another spoo login running?): %w", CallbackAddr, err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != CallbackPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("authorization failed: state mismatch — please try again")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("authorization failed: callback carried no code")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, successHTML)
		codeCh <- code
	})}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	authURL := f.Client.DeviceAuthURL(spoo.DeviceAuthParams{
		AppID:         AppID,
		RedirectURI:   "http://" + CallbackAddr + CallbackPath,
		State:         state,
		CodeChallenge: spoo.CodeChallengeS256(verifier),
	})

	fmt.Fprintln(f.Out, "Opening your browser to authorize spoo CLI…")
	fmt.Fprintf(f.Out, "If it doesn't open automatically, visit:\n\n  %s\n\n", authURL)
	if err := f.OpenBrowser(authURL); err != nil {
		fmt.Fprintf(f.Out, "(could not open browser: %v)\n", err)
	}

	select {
	case code := <-codeCh:
		return code, verifier, nil
	case err := <-errCh:
		return "", "", err
	case <-ctx.Done():
		return "", "", fmt.Errorf("login timed out: %w", ctx.Err())
	}
}
