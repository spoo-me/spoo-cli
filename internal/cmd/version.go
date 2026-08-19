package cmd

import "regexp"

// Version is the CLI release, injected by goreleaser via ldflags.
var Version = "dev"

var versionRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,16}$`)

// clientTag identifies the CLI (and its version, when well-formed) to
// the backend so API traffic can be attributed per client. It is
// passed to the SDK, which sends it as X-Spoo-Client.
func clientTag() string {
	if versionRe.MatchString(Version) {
		return "cli/" + Version
	}
	return "cli"
}
