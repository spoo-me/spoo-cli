package kit

import "testing"

func TestCompactNum(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"}, {938, "938"}, {1500, "1.5k"}, {8601, "8.6k"},
		{13599, "13.6k"}, {51240, "51.2k"}, {99999, "100k"},
		{131881, "132k"}, {287558, "288k"}, {1_500_000, "1.5M"},
		{2_000_000, "2M"},
	}
	for _, c := range cases {
		if got := CompactNum(c.in); got != c.want {
			t.Errorf("CompactNum(%g) = %q, want %q", c.in, got, c.want)
		}
	}
}
