package version

var (
	// Version is injected at build time via ldflags, e.g.:
	// -ldflags="-X 'web_reader/internal/version.Version=v1.8.0'"
	Version   = "v1.8.0"
	GitCommit = ""
	BuildDate = ""
)

// Get returns the current application version.
func Get() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
