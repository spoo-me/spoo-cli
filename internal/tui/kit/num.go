package kit

import (
	"fmt"
	"math"
	"strings"
)

// CompactNum renders a count compactly to ~3 significant figures:
// 938 → "938", 1500 → "1.5k", 51240 → "51.2k", 131881 → "132k",
// 1_500_000 → "1.5M". Keeps dashboard columns narrow and glanceable.
func CompactNum(v float64) string {
	abs := math.Abs(v)
	switch {
	case abs >= 1_000_000:
		return trimDotZero(fmt.Sprintf("%.1f", v/1e6)) + "M"
	case abs >= 100_000:
		return fmt.Sprintf("%.0fk", v/1000)
	case abs >= 1_000:
		return trimDotZero(fmt.Sprintf("%.1f", v/1000)) + "k"
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

func trimDotZero(s string) string { return strings.TrimSuffix(s, ".0") }
