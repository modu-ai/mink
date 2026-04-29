package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileWriter writes audit events to a log file in JSON Lines format.
// Each event is written as a single JSON object followed by a newline.
// AC-AUDIT-01: JSON line format
// AC-AUDIT-02: Append-only integrity
//
// @MX:ANCHOR: [AUTO] Core audit log writer interface
// @MX:REASON: Used by all audit logging components, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-001
type FileWriter struct {
	mu         sync.Mutex
	file       *os.File
	path       string
	writeCount uint64 // Counter for batched sync optimization
}

// NewFileWriter creates a new FileWriter that appends to the specified log file.
// If the log file or its parent directory doesn't exist, it will be created.
// The file is opened in append-only mode with O_APPEND flag to guarantee
// atomic writes at the OS level (AC-AUDIT-02).
//
// REQ-AUDIT-001: JSON line append to audit.log (never update or delete)
func NewFileWriter(path string) (*FileWriter, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file in append-only mode
	// O_APPEND: Ensure atomic writes at OS level
	// O_CREATE: Create file if it doesn't exist
	// O_WRONLY: Write-only mode
	// 0600: Owner read-write only (CRITICAL: prevents other users from modifying logs)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &FileWriter{
		file: file,
		path: path,
	}, nil
}

// Write writes an audit event to the log file.
// The event is serialized to JSON and written as a single line.
// A newline is appended after each event.
//
// This method is thread-safe and can be called concurrently from multiple
// goroutines. The sync.Mutex ensures that writes are atomic.
func (w *FileWriter) Write(event AuditEvent) error {
	if w == nil || w.file == nil {
		return fmt.Errorf("writer is closed or not initialized")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Append newline
	data = append(data, '\n')

	// Write to file
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Batched sync for performance (MEDIUM: sync every 100 writes)
	// This reduces disk I/O while maintaining acceptable durability
	w.writeCount++
	if w.writeCount%100 == 0 {
		return w.file.Sync()
	}

	return nil
}

// Close closes the log file and releases resources.
// After Close is called, any further Write calls will fail.
// Performs a final sync before closing to ensure durability.
func (w *FileWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Final sync to ensure any pending writes are flushed
	_ = w.file.Sync()

	err := w.file.Close()
	w.file = nil
	return err
}

// Path returns the path to the log file.
func (w *FileWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}
