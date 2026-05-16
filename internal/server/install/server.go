// Package install — server.go implements lifecycle helpers for the install HTTP server.
// RunServer is the main entry point called by the CLI --web flag; it binds to a
// random localhost port, spawns the OS browser, and serves until ctx is cancelled.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A
package install

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// RunServer starts the install HTTP server on a random localhost port.
// It blocks until ctx is cancelled or the server returns a non-nil error.
//
// Parameters:
//   - ctx: cancellation signal (SIGINT/SIGTERM from the CLI layer).
//   - openBrowser: when true, the OS default browser is opened to the install URL.
//   - listening: callback invoked with the full install URL before Serve blocks.
//     The CLI layer uses this to print the URL to stdout before blocking.
//
// Security: the listener is always bound to 127.0.0.1 (loopback only).
//
// @MX:ANCHOR: [AUTO] RunServer is the single entry point for the --web onboarding path.
// @MX:REASON: Called by CLI init.go --web and integration tests; signature changes
// must propagate to all callers. Port selection and graceful shutdown are
// load-bearing invariants exercised by tests.
func RunServer(ctx context.Context, openBrowser bool, listening func(url string)) error {
	// Bind to a random loopback port chosen by the OS.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("install: listen failed: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/install", port)

	// Notify the caller of the chosen URL before we block on Serve.
	if listening != nil {
		listening(url)
	}

	// Open the OS browser in the background (best-effort; errors are swallowed).
	if openBrowser {
		go openInBrowser(url)
	}

	handler := NewHandler(HandlerOptions{})
	defer handler.Close()

	srv := &http.Server{
		Handler: handler,
	}

	// Graceful shutdown goroutine: when ctx is cancelled, give the server 5s to drain.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Serve blocks until the listener is closed by Shutdown.
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("install: server error: %w", err)
	}

	// Wait for the shutdown goroutine to finish draining.
	<-shutdownDone
	return nil
}

// openInBrowser launches the OS default browser to the given URL.
// Errors are intentionally swallowed — browser availability is best-effort.
func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// Linux and other POSIX systems.
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
