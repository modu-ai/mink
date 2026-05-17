// Package file — permission and cloud-sync detection helpers.
//
// POSIX permission check: verifyMode asserts that the credentials file has
// exactly mode 0600 (owner read/write only).  On Windows this check is skipped
// because NTFS ACL enforcement is deferred to a future ICACLS integration
// (documented in the @MX:WARN tag in backend.go).
//
// Cloud-sync detection: WarnIfCloudSynced returns a non-empty warning string
// when the given path resides inside a known cloud-synchronised folder
// (iCloud Drive, OneDrive, Dropbox, Google Drive, Box).  The write still
// proceeds — this is an informational warning only (UN-2, T-009).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UN-6, AC-CR-027, T-006, T-009)
package file

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// verifyMode checks that the file at path has exactly mode 0600 on POSIX
// systems.  Returns an error if the mode differs.
//
// On Windows this function always returns nil because NTFS volume permissions
// are not reflected in Go's os.FileMode bits (Windows gap, documented in
// backend.go @MX:WARN).
func verifyMode(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file: stat %q: %w", path, err)
	}

	mode := info.Mode().Perm()
	if mode != credentialsFileMode {
		return fmt.Errorf("file: %q has mode %04o, want %04o",
			path, mode, credentialsFileMode)
	}
	return nil
}

// cloudSyncSegments is the ordered list of path segment substrings that
// indicate the path is inside a known cloud-sync folder.
//
// Matching is case-insensitive on macOS and Windows (common for cloud folders
// on those platforms) and case-sensitive on Linux.
//
// The list covers the five vendors named in SPEC T-009:
//
//	iCloud Drive, OneDrive, Dropbox, Google Drive, Box
var cloudSyncSegments = []string{
	"icloud drive",
	"onedrive",
	"dropbox",
	"google drive",
	"box",
}

// WarnIfCloudSynced returns a non-empty warning message when path appears to
// reside inside a well-known cloud-synchronisation folder.  The write still
// proceeds when this function returns a non-empty string — callers should emit
// the warning to stderr and continue.
//
// Detection algorithm: split path into filepath components and check whether
// any component matches a cloud-sync keyword.  Comparison is case-insensitive
// on macOS and Windows; case-sensitive on Linux.
//
// @MX:NOTE: [AUTO] WarnIfCloudSynced is called on every Store to inform the
// user when credentials land in a synced directory (UN-2 trade-off awareness,
// T-009 requirement).
func WarnIfCloudSynced(path string) string {
	absPath := filepath.Clean(path)
	caseInsensitive := runtime.GOOS == "darwin" || runtime.GOOS == "windows"

	// Split the path into its directory components for segment-level matching.
	parts := splitPathSegments(absPath)

	for _, part := range parts {
		for _, keyword := range cloudSyncSegments {
			var matched bool
			if caseInsensitive {
				// macOS / Windows: case-insensitive match (cloudSyncSegments are
				// already lower-case).
				matched = strings.EqualFold(part, keyword)
			} else {
				// Linux: case-sensitive match against the lower-case canonical
				// form.  A user-created "Dropbox" directory must NOT match
				// because Linux filesystems are case-sensitive — the official
				// Dropbox client on Linux uses lower-case "dropbox".
				matched = part == keyword
			}
			if matched {
				return fmt.Sprintf(
					"warning: credentials file %q appears to be inside a "+
						"cloud-sync folder (%q). "+
						"This may expose your credentials to unintended third parties. "+
						"Consider moving ~/.mink/auth/ outside synced directories.",
					path, part)
			}
		}
	}
	return ""
}

// splitPathSegments returns the individual directory and file name components
// of p, excluding empty strings and the root separator.
func splitPathSegments(p string) []string {
	var parts []string
	for {
		dir, base := filepath.Split(p)
		dir = filepath.Clean(dir)
		if base != "" {
			parts = append([]string{base}, parts...)
		}
		if dir == p || dir == "." || dir == "/" || dir == "\\" {
			break
		}
		p = dir
	}
	return parts
}
