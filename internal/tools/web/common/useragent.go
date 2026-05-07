package common

import "fmt"

// version is the current goose-agent version string embedded at build time.
// Override via ldflags: -X github.com/modu-ai/goose/internal/tools/web/common.version=X.Y.Z
var version = "dev"

// UserAgent returns the standard HTTP User-Agent header value for all web tools.
// Format: "goose-agent/{version}" as required by REQ-WEB-003.
func UserAgent() string {
	return fmt.Sprintf("goose-agent/%s", version)
}
