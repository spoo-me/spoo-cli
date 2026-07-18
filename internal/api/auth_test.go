package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/device/token" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]string
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("bad body: %v", err)
		}
		if got["code"] != "onetimecode" {
			t.Errorf("code = %q", got["code"])
		}
		if got["code_verifier"] != "theverifier" {
			t.Errorf("code_verifier = %q", got["code_verifier"])
		}
		w.Write([]byte(`{"access_token":"at","refresh_token":"rt","user":{"id":"1","email":"a@b.c","email_verified":true,"name":"A","plan":"free"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	tok, err := c.ExchangeDeviceCode(context.Background(), "onetimecode", "theverifier")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.User.Email != "a@b.c" {
		t.Fatalf("unexpected: %+v", tok)
	}
}

func TestMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"user":{"id":"1","email":"a@b.c","email_verified":true,"name":"A","plan":"free"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	u, err := c.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@b.c" || !u.EmailVerified {
		t.Fatalf("unexpected user: %+v", u)
	}
}
