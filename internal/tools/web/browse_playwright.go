package web

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// ErrPlaywrightNotInstalled is the canonical sentinel returned by a launcher
// when the Playwright driver / chromium binary is missing.
//
// AC-WEB-011 requires the tool to translate this into the user-facing error
// code "playwright_not_installed" rather than panicking. Wrapping is allowed
// (errors.Is must work).
var ErrPlaywrightNotInstalled = errors.New("playwright not installed")

// PlaywrightSession is the abstract per-call session returned by a launcher.
// M2b only uses Close(); the eventual M2c web_browse implementation will add
// page navigation + DOM extraction methods. Keeping the interface minimal now
// avoids leaking the playwright.Playwright type to callers.
type PlaywrightSession interface {
	Close() error
}

// PlaywrightLauncher abstracts the Playwright entry point so tests can inject
// a launcher that always reports ErrPlaywrightNotInstalled. The production
// implementation wraps playwright.Run.
//
// @MX:NOTE: [AUTO] DI seam — tests inject a failing launcher to exercise AC-WEB-011 without a real chromium binary
// @MX:SPEC: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-011
type PlaywrightLauncher interface {
	// Launch starts a Playwright session. ErrPlaywrightNotInstalled (or any
	// wrapped variant matched by classifyLaunchError) signals a missing
	// driver / browser binary.
	Launch(ctx context.Context) (PlaywrightSession, error)
}

// playwrightSessionAdapter wraps playwright.Playwright so we can return it
// through the PlaywrightSession interface without exposing the upstream type.
type playwrightSessionAdapter struct {
	pw *playwright.Playwright
}

// Close stops the underlying Playwright instance.
func (s *playwrightSessionAdapter) Close() error {
	if s == nil || s.pw == nil {
		return nil
	}
	return s.pw.Stop()
}

// productionLauncher is the default PlaywrightLauncher used when no test
// double is injected. It calls playwright.Run() and translates the
// driver-missing error message into ErrPlaywrightNotInstalled.
type productionLauncher struct{}

// Launch implements PlaywrightLauncher by invoking playwright.Run. Driver /
// binary missing errors are normalised to ErrPlaywrightNotInstalled so the
// tool can return AC-WEB-011's canonical error code.
func (productionLauncher) Launch(_ context.Context) (PlaywrightSession, error) {
	pw, err := playwright.Run()
	if err != nil {
		if isDriverMissingError(err) {
			return nil, fmt.Errorf("%w: %v", ErrPlaywrightNotInstalled, err)
		}
		return nil, err
	}
	return &playwrightSessionAdapter{pw: pw}, nil
}

// classifyLaunchError maps a launcher error to the user-facing error code
// returned by web_browse. It returns "" for nil errors,
// "playwright_not_installed" for the sentinel or any error whose message
// matches the canonical driver-missing patterns, and "playwright_launch_failed"
// for any other launcher failure.
func classifyLaunchError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrPlaywrightNotInstalled) {
		return "playwright_not_installed"
	}
	if isDriverMissingError(err) {
		return "playwright_not_installed"
	}
	return "playwright_launch_failed"
}

// isDriverMissingError matches the well-known playwright.Run() error strings
// that indicate a missing driver / browser binary.
func isDriverMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "please install the driver"):
		return true
	case strings.Contains(msg, "could not get driver instance"):
		return true
	case strings.Contains(msg, "chromium binary not found"):
		return true
	case strings.Contains(msg, "browser executable doesn't exist"):
		return true
	}
	return false
}

// ClassifyLaunchErrorForTest exposes classifyLaunchError to the test package
// without leaking the production type assertions.
func ClassifyLaunchErrorForTest(err error) string {
	return classifyLaunchError(err)
}
