// Package version holds build-time metadata injected via -ldflags.
package version

// These values are injected at build time using:
//
//	go build -trimpath -ldflags "\
//	  -s -w \
//	  -X github.com/JoWe112/ldapapi-ng/internal/version.Version=$(git describe --tags --always) \
//	  -X github.com/JoWe112/ldapapi-ng/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/JoWe112/ldapapi-ng/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
