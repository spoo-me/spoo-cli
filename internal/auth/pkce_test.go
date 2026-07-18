package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestNewPKCE(t *testing.T) {
	p, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Verifier) != 43 {
		t.Errorf("verifier len = %d, want 43", len(p.Verifier))
	}
	if len(p.Challenge) != 43 {
		t.Errorf("challenge len = %d, want 43", len(p.Challenge))
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); want != p.Challenge {
		t.Fatalf("challenge = %q, want S256(verifier) = %q", p.Challenge, want)
	}
}

func TestNewPKCEUnique(t *testing.T) {
	a, _ := newPKCE()
	b, _ := newPKCE()
	if a.Verifier == b.Verifier {
		t.Fatal("two verifiers collided")
	}
}

// RFC 7636 Appendix B fixed vector — proves our S256 derivation matches.
func TestS256MatchesRFCVector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	if want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"; got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}
