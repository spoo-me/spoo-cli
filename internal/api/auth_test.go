package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/device/token" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"access_token":"at","refresh_token":"rt","user":{"id":"1","email":"a@b.c","email_verified":true,"name":"A","plan":"free"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, newTestStore(t, nil))
	tok, err := c.ExchangeDeviceCode(context.Background(), "onetimecode")
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
