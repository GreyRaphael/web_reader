package version

// Version is injected at build time via ldflags, e.g.:
// -ldflags="-X 'web_reader/internal/version.Version=v1.8.0'"
// The "dev" default marks local builds that were not stamped by the
// Makefile; the updater treats it as a development build and refuses
// to self-update.
var Version = "dev"

// Get returns the current application version.
func Get() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
