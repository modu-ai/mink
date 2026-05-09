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

// PlaywrightSession is the abstract per-call session. It exposes page
// navigation and DOM extraction methods used by the web_browse success path,
// plus Close for resource cleanup.
//
// @MX:ANCHOR: [AUTO] PlaywrightSession interface — production page navigation contract
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 M2c — fan_in >= 3 (browse.go, browse_test.go, production launcher)
type PlaywrightSession interface {
	// Goto navigates the page to url with a timeout hint in milliseconds.
	Goto(ctx context.Context, url string, timeoutMs int) error
	// Title returns the page <title> text.
	Title() (string, error)
	// Content returns the full raw HTML of the loaded page.
	Content() (string, error)
	// InnerText returns the inner text of the first element matching selector.
	// Use "body" for full-page plain text.
	InnerText(selector string) (string, error)
	// Close releases the browser and Playwright subprocess.
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

// playwrightSessionAdapter wraps a playwright Browser + Page so we can return
// the session through the PlaywrightSession interface without exposing upstream types.
type playwrightSessionAdapter struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
}

// Goto navigates the underlying page to the given URL, applying timeoutMs as
// the Playwright navigation timeout.
func (s *playwrightSessionAdapter) Goto(_ context.Context, url string, timeoutMs int) error {
	timeout := float64(timeoutMs)
	_, err := s.page.Goto(url, playwright.PageGotoOptions{
		Timeout: &timeout,
	})
	return err
}

// Title returns the page title.
func (s *playwrightSessionAdapter) Title() (string, error) {
	return s.page.Title()
}

// Content returns the full raw HTML source of the current page.
func (s *playwrightSessionAdapter) Content() (string, error) {
	return s.page.Content()
}

// InnerText returns the inner text of the first element matching selector.
// Uses the Locator-based API as recommended by playwright-go.
func (s *playwrightSessionAdapter) InnerText(selector string) (string, error) {
	return s.page.Locator(selector).InnerText()
}

// Close closes the browser context and stops the Playwright subprocess.
// Errors from each step are joined so callers can see all failures.
func (s *playwrightSessionAdapter) Close() error {
	if s == nil {
		return nil
	}
	var errs []string
	if s.browser != nil {
		if err := s.browser.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("browser.Close: %v", err))
		}
	}
	if s.pw != nil {
		if err := s.pw.Stop(); err != nil {
			errs = append(errs, fmt.Sprintf("pw.Stop: %v", err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// productionLauncher is the default PlaywrightLauncher used when no test
// double is injected. It starts Playwright, launches Chromium headless, and
// opens a new page.
type productionLauncher struct{}

// Launch implements PlaywrightLauncher by invoking playwright.Run, launching
// Chromium headless, and opening a new blank page. Driver / binary missing
// errors are normalised to ErrPlaywrightNotInstalled so the tool can return
// AC-WEB-011's canonical error code.
//
// @MX:WARN: [AUTO] Subprocess launch — Playwright spawns an external chromium process
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 M2c — long-lived subprocess must be closed via session.Close()
func (productionLauncher) Launch(_ context.Context) (PlaywrightSession, error) {
	pw, err := playwright.Run()
	if err != nil {
		if isDriverMissingError(err) {
			return nil, fmt.Errorf("%w: %v", ErrPlaywrightNotInstalled, err)
		}
		return nil, err
	}

	headless := true
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		_ = pw.Stop()
		if isDriverMissingError(err) {
			return nil, fmt.Errorf("%w: %v", ErrPlaywrightNotInstalled, err)
		}
		return nil, fmt.Errorf("chromium launch: %w", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, fmt.Errorf("new page: %w", err)
	}

	return &playwrightSessionAdapter{pw: pw, browser: browser, page: page}, nil
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
