//go:build linux

package locale

import (
	"bufio"
	"context"
	"os"
	"strings"
)

// detectFromOSAPIs performs Linux-specific locale detection.
//
// Detection order (per research.md §2.1):
//  1. /etc/locale.conf (systemd systems)
//  2. /etc/default/locale (Debian/Ubuntu)
//
// Environment-variable detection is handled generically by detectFromEnv().
func detectFromOSAPIs(_ context.Context) (country, lang string, err error) {
	// Try systemd locale.conf first.
	for _, path := range []string{"/etc/locale.conf", "/etc/default/locale"} {
		if _, serr := statFile(path); serr != nil {
			continue // file does not exist
		}

		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}

		if c, l, ok := parseLocaleConf(string(data)); ok {
			return c, l, nil
		}
	}

	return "", "", ErrNoOSLocale
}

// parseLocaleConf parses KEY=VALUE pairs from /etc/locale.conf or
// /etc/default/locale and extracts the LANG value.
func parseLocaleConf(content string) (country, lang string, ok bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if after, found := strings.CutPrefix(line, "LANG="); found {
			val := strings.Trim(after, `"'`)
			if val != "" && val != "C" && val != "POSIX" {
				c, l, valid := parseLocaleString(val)
				if valid {
					return c, l, true
				}
			}
		}
	}
	return "", "", false
}
