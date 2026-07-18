package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds a proof-key pair for the device authorization flow (RFC 7636).
// The verifier stays on the client; the S256 challenge travels to the
// server at login and is proven by sending the verifier at token exchange.
type PKCE struct {
	Verifier  string
	Challenge string
}

// newPKCE mints a fresh verifier and its S256 challenge. The verifier is a
// 43-character base64url string (32 random bytes), the minimum RFC 7636
// length, which the backend also enforces.
func newPKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}
