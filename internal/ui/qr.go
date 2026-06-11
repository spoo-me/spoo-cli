package ui

import (
	"strings"

	"github.com/mdp/qrterminal/v3"
)

// QR renders text as a half-block QR code. The default draws light
// modules on the terminal's own background — the right contrast for
// dark terminals; invert flips it for light ones.
func QR(text string, invert bool) string {
	var b strings.Builder
	cfg := qrterminal.Config{
		Level:          qrterminal.L, // short URLs: low ECC keeps it small
		Writer:         &b,
		HalfBlocks:     true,
		QuietZone:      2,
		BlackChar:      qrterminal.BLACK_BLACK,
		WhiteChar:      qrterminal.WHITE_WHITE,
		WhiteBlackChar: qrterminal.WHITE_BLACK,
		BlackWhiteChar: qrterminal.BLACK_WHITE,
	}
	if invert {
		cfg.BlackChar = qrterminal.WHITE_WHITE
		cfg.WhiteChar = qrterminal.BLACK_BLACK
		cfg.WhiteBlackChar = qrterminal.BLACK_WHITE
		cfg.BlackWhiteChar = qrterminal.WHITE_BLACK
	}
	qrterminal.GenerateWithConfig(text, cfg)
	return strings.TrimRight(b.String(), "\n")
}
