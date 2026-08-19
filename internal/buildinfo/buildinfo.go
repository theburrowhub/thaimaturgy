// Package buildinfo carries the binary's version stamp. The values are injected
// at build time via -ldflags, e.g.:
//
//	-X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Version=v1.2.3
//	-X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Commit=abc1234
//	-X github.com/theburrowhub/thaimaturgy/internal/buildinfo.Date=2026-01-02_15:04:05
//
// The Makefile and the release workflow set Version from the semver git tag
// (`git describe --tags` locally; the pushed tag in CI). Every frontend reads
// String() so the version is derived in one place.
package buildinfo

import "runtime/debug"

// Set via -ldflags at build time; empty in a plain `go run`/`go build ./...`.
var (
	Version = ""
	Commit  = ""
	Date    = ""
)

// String returns a concise display version: the injected semver tag (e.g.
// "v0.1.2", or "v0.1.2-3-gabcdef" for an untagged commit past a tag). When no
// version was injected it falls back to the embedded VCS revision ("dev+<sha>")
// or, failing that, "dev".
func String() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				rev := s.Value
				if len(rev) > 12 {
					rev = rev[:12]
				}
				if dirty := vcsModified(bi); dirty {
					rev += "-dirty"
				}
				return "dev+" + rev
			}
		}
	}
	return "dev"
}

func vcsModified(bi *debug.BuildInfo) bool {
	for _, s := range bi.Settings {
		if s.Key == "vcs.modified" {
			return s.Value == "true"
		}
	}
	return false
}
