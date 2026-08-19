package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	spoo "github.com/spoo-me/spoo-go"
	"github.com/spoo-me/spoo-go/option"
)

var challengeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func testFlowClient() *spoo.Client {
	return spoo.NewClient(option.WithBaseURL("https://spoo.example"))
}

// Simulates the browser leg: the flow opens a URL; we parse state and
// redirect_uri out of it and hit the loopback callback like spoo.me would.
func TestDeviceFlowReturnsCode(t *testing.T) {
	challengeCh := make(chan string, 1)
	flow := &DeviceFlow{
		Client: testFlowClient(),
		Out:    io.Discard,
		OpenBrowser: func(authURL string) error {
			go func() {
				u, err := url.Parse(authURL)
				if err != nil {
					t.Error(err)
					return
				}
				q := u.Query()
				if q.Get("app_id") != "spoo-cli" {
					t.Errorf("app_id = %q", q.Get("app_id"))
				}
				cb := q.Get("redirect_uri")
				if !strings.HasPrefix(cb, "http://127.0.0.1:53682/callback") {
					t.Errorf("redirect_uri = %q", cb)
				}
				if q.Get("code_challenge_method") != "S256" {
					t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
				}
				challenge := q.Get("code_challenge")
				if !challengeRe.MatchString(challenge) {
					t.Errorf("code_challenge = %q, want 43 base64url chars", challenge)
				}
				challengeCh <- challenge
				time.Sleep(50 * time.Millisecond) // let the server start
				resp, err := http.Get(fmt.Sprintf("%s?code=thecode&state=%s", cb, q.Get("state")))
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
			}()
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	code, verifier, err := flow.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if code != "thecode" {
		t.Fatalf("code = %q, want thecode", code)
	}
	if len(verifier) != 43 {
		t.Fatalf("verifier length = %d, want 43", len(verifier))
	}
	if got := spoo.CodeChallengeS256(verifier); got != <-challengeCh {
		t.Fatalf("code_challenge on auth URL does not match S256(verifier): %q", got)
	}
}

func TestDeviceFlowRejectsStateMismatch(t *testing.T) {
	flow := &DeviceFlow{
		Client: testFlowClient(),
		Out:    io.Discard,
		OpenBrowser: func(authURL string) error {
			go func() {
				time.Sleep(50 * time.Millisecond)
				resp, err := http.Get("http://127.0.0.1:53682/callback?code=evil&state=wrong")
				if err == nil {
					resp.Body.Close()
				}
			}()
			return nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := flow.Run(ctx); err == nil {
		t.Fatal("expected state-mismatch error, got nil")
	}
}
