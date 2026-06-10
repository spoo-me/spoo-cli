package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Simulates the browser leg: the flow opens a URL; we parse state and
// redirect_uri out of it and hit the loopback callback like spoo.me would.
func TestDeviceFlowReturnsCode(t *testing.T) {
	flow := &DeviceFlow{
		APIBase: "https://spoo.example",
		Out:     io.Discard,
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
	code, err := flow.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if code != "thecode" {
		t.Fatalf("code = %q, want thecode", code)
	}
}

func TestDeviceFlowRejectsStateMismatch(t *testing.T) {
	flow := &DeviceFlow{
		APIBase: "https://spoo.example",
		Out:     io.Discard,
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
	if _, err := flow.Run(ctx); err == nil {
		t.Fatal("expected state-mismatch error, got nil")
	}
}
