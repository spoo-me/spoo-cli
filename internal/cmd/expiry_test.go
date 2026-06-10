package cmd

import (
	"strconv"
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
	want := strconv.FormatInt(now.Add(72*time.Hour).Unix(), 10)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
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
