package cmd

import (
	"testing"
	"time"
)

func TestParseExpiryPassthroughISO(t *testing.T) {
	got, err := parseExpiry("2027-01-02T15:04:05Z", time.Now())
	if err != nil || got != "2027-01-02T15:04:05Z" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestParseExpiryDuration(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	got, err := parseExpiry("72h", now)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-06-13T12:00:00Z" {
		t.Fatalf("got %q, want 2026-06-13T12:00:00Z", got)
	}
}

// Bare epoch input is normalized to RFC 3339: the backend only parses
// epoch when it arrives as a JSON number, and we send a string field.
func TestParseExpiryEpochString(t *testing.T) {
	got, err := parseExpiry("1781524800", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-06-15T12:00:00Z" {
		t.Fatalf("got %q, want 2026-06-15T12:00:00Z", got)
	}
}

func TestParseExpiryRejectsNegativeDuration(t *testing.T) {
	if _, err := parseExpiry("-5m", time.Now()); err == nil {
		t.Fatal("want error for negative duration")
	}
}

func TestParseExpiryEmpty(t *testing.T) {
	got, err := parseExpiry("", time.Now())
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}
