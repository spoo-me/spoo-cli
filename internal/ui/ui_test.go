package ui

import "testing"

func TestCountryLabel(t *testing.T) {
	cases := map[string]string{
		"IN":    "IN",
		"US":    "US",
		"XX":    "Unknown", // backend's no-geo marker, cased like unknown cities
		"India": "India",
		"":      "",
	}
	for in, want := range cases {
		if got := CountryLabel(in); got != want {
			t.Errorf("CountryLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
