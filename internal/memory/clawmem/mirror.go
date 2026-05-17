// Copyright (C) 2026 MoAI <email@mo.ai.kr>
//
// This file is part of MINK, released under the GNU Affero General Public
// License version 3.0 only.  See LICENSE for details.

package clawmem

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Config holds the clawmem_compat block from memory.yaml.
type Config struct {
	// Enabled controls whether mirror writes are attempted at all.
	// When false, Write is a no-op.
	Enabled bool

	// VaultPath is the root path of the ClawMem vault.
	// A leading "~/" is expanded via os.UserHomeDir().
	// Default: "~/.clawmem/vault".
	VaultPath string
}

// WriteRequest is the input to Mirror.Write.
type WriteRequest struct {
	// Collection is the logical collection name (e.g. "journal").
	Collection string

	// Filename is the base filename (e.g. "2026-05-17.md").
	// It may include a single sub-directory segment.
	Filename string

	// Content is the raw markdown bytes to write to the vault.
	Content []byte
}

// Mirror writes markdown files to a ClawMem vault in addition to the primary
// MINK write path.
//
// Mirror writes are best-effort: if the vault write fails, a structured zap
// warning is emitted and nil is returned so the primary insert is not blocked.
//
// @MX:ANCHOR: [AUTO] Central mirror write entry point shared by sqlite.Writer and future publish hooks.
// @MX:REASON: fan_in >= 3 (sqlite writer integration, T5.8 publish hooks, T5.9 session export).
type Mirror struct {
	cfg       Config
	vaultPath string // expanded vault path (~ resolved)
	logger    *zap.Logger

	// readOnly is set to true on the first write when an unsupported schema
	// version is detected.  Protected by readOnceLog.
	//
	// @MX:WARN: [AUTO] Flag written once then read concurrently; guarded by sync.Once.
	// @MX:REASON: Multiple goroutines may call Write concurrently; the flag must
	// only cause the log to fire once without a lock per-write.
	readOnly    bool
	readOnceLog sync.Once
}

// NewMirror creates a Mirror from cfg.
//
// If cfg.Enabled is false, the returned Mirror is a lightweight no-op
// implementation.  No filesystem access is performed in NewMirror itself.
func NewMirror(cfg Config, logger *zap.Logger) (*Mirror, error) {
	expanded, err := expandHome(cfg.VaultPath)
	if err != nil {
		return nil, err
	}
	return &Mirror{
		cfg:       cfg,
		vaultPath: expanded,
		logger:    logger,
	}, nil
}

// Write mirrors the markdown file described by req to the ClawMem vault.
//
// When Enabled is false the call is a no-op.
//
// If the vault contains an unsupported schema version, mirror writes are
// silently skipped after logging one info event (read-only mode).
//
// Any filesystem error is logged as a warn and nil is returned — the primary
// write path must never be blocked by a mirror failure.
//
// SPEC: SPEC-MINK-MEMORY-QMD-001 T5.6, T5.7
func (m *Mirror) Write(req WriteRequest) error {
	if !m.cfg.Enabled {
		return nil
	}

	// Schema probe: detect version on first write.  The result is cached via
	// readOnceLog + readOnly so subsequent writes skip the fs probe.
	if err := m.checkSchema(); err != nil {
		// checkSchema only returns errors for unexpected I/O failures; log and
		// continue in best-effort mode.
		m.logger.Warn("clawmem_mirror: schema probe error",
			zap.String("op", "clawmem_mirror"),
			zap.Error(err),
		)
	}
	if m.readOnly {
		return nil
	}

	// Build the destination path: {vault}/{collection}/{filename}.
	dst := filepath.Join(m.vaultPath, req.Collection, req.Filename)
	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.logger.Warn("clawmem_mirror: mkdir failed",
			zap.String("op", "clawmem_mirror"),
			zap.String("path", dir),
			zap.Error(err),
		)
		return nil // best-effort
	}

	if err := os.WriteFile(dst, req.Content, 0o600); err != nil {
		m.logger.Warn("clawmem_mirror: write failed",
			zap.String("op", "clawmem_mirror"),
			zap.String("path", dst),
			zap.Error(err),
		)
		return nil // best-effort
	}

	return nil
}

// checkSchema detects the vault schema version and sets readOnly when the
// version is unsupported.  The info log fires at most once.
func (m *Mirror) checkSchema() error {
	// Fast path: already determined on a previous call.
	if m.readOnly {
		return nil
	}

	ver, err := DetectSchemaVersion(m.vaultPath)
	if err != nil {
		return err
	}

	if !IsSupportedVersion(ver) {
		m.readOnceLog.Do(func() {
			m.readOnly = true
			m.logger.Info("clawmem_schema: unsupported version; entering read-only mirror mode",
				zap.String("op", "clawmem_schema"),
				zap.String("detected_version", string(ver)),
				zap.String("mode", "read_only"),
			)
		})
	}
	return nil
}

// expandHome replaces a leading "~/" with the current user's home directory.
// Uses os.UserHomeDir() — not the HOME env var — for portability.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[2:]), nil
}
