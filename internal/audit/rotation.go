package audit

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultMaxSize is the default maximum size before rotation (100MB).
// REQ-AUDIT-002: 100MB rotation with timestamp suffix + gzip compression
const DefaultMaxSize = 100 * 1024 * 1024

// RotatingWriter writes audit events to a log file with automatic rotation.
// When the log file exceeds MaxSize, it is rotated with a timestamp suffix
// and compressed with gzip.
//
// REQ-AUDIT-002: WHEN audit.log 파일이 100MB를 초과할 때,
//
//	the system SHALL 타임스탬프 접미사로 rotate한다
//	(기존 파일은 gzip 압축)
//
// @MX:ANCHOR: [AUTO] Log rotation manager
// @MX:REASON: Core component for audit log lifecycle management
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-002
type RotatingWriter struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxSize  int64
	writePos int64
	lastHash string // Hash of the last written event for integrity chain
}

// NewRotatingWriter creates a new RotatingWriter with the specified max size.
// If maxSize is 0, DefaultMaxSize (100MB) is used.
//
// The writer creates the log file and parent directory if they don't exist.
func NewRotatingWriter(path string, maxSize int64) (*RotatingWriter, error) {
	if maxSize == 0 {
		maxSize = DefaultMaxSize
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create log file
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to stat log file: %w, and failed to close file: %v", err, closeErr)
		}
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	return &RotatingWriter{
		file:     file,
		path:     path,
		maxSize:  maxSize,
		writePos: info.Size(),
	}, nil
}

// Write writes an audit event to the log file.
// If the write would cause the file to exceed MaxSize, the file is
// rotated before the write.
//
// REQ-AUDIT-002: Rotation without interruption (AC-AUDIT-03)
func (w *RotatingWriter) Write(event AuditEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Set PrevHash for integrity chain before marshaling
	event.PrevHash = w.lastHash

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Append newline
	data = append(data, '\n')
	eventSize := int64(len(data))

	// Check if we need to rotate before writing
	if w.writePos+eventSize > w.maxSize {
		if err := w.rotateLocked(); err != nil {
			return fmt.Errorf("failed to rotate log: %w", err)
		}
	}

	// Write to file
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Sync to disk for durability
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync log: %w", err)
	}

	// Update write position
	w.writePos += eventSize

	// Update lastHash for chain integrity
	w.lastHash = ComputeEventHash(event)

	return nil
}

// rotateLocked performs log rotation.
// This method MUST be called with w.mu held.
//
// Rotation process:
// 1. Close current log file
// 2. Rename with timestamp suffix
// 3. Compress rotated file with gzip
// 4. Create new log file
//
// REQ-AUDIT-002: 타임스탬프 접미사로 rotate + gzip 압축
func (w *RotatingWriter) rotateLocked() error {
	// Close current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	// Generate timestamp suffix
	timestamp := time.Now().UTC().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", w.path, timestamp)

	// Rename current file to rotated path
	if err := os.Rename(w.path, rotatedPath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Compress rotated file immediately for test reliability
	// In production, this could be done asynchronously
	if err := w.compressFile(rotatedPath); err != nil {
		return fmt.Errorf("failed to compress rotated file: %w", err)
	}

	// Create new log file
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	w.file = file
	w.writePos = 0
	w.lastHash = "" // Reset hash chain on rotation

	return nil
}

// compressFile compresses a log file with gzip.
// The original file is replaced with the compressed version.
func (w *RotatingWriter) compressFile(path string) error {
	// Open source file
	srcFile, err := os.Open(path)
	if err != nil {
		return err // Silent fail - rotation already succeeded
	}
	defer func() {
		_ = srcFile.Close()
		// Log warning but continue - file will be closed on process exit
		// This is in a defer during compression, so we can't do much else
	}()

	// Create compressed file
	compressedPath := path + ".gz"
	gzFile, err := os.Create(compressedPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = gzFile.Close()
		// Log warning but continue - file will be closed on process exit
		// This is in a defer during compression, so we can't do much else
	}()

	// Create gzip writer
	gzWriter := gzip.NewWriter(gzFile)
	defer func() {
		_ = gzWriter.Close()
		// Log warning but continue - file will be closed on process exit
		// This is in a defer during compression, so we can't do much else
	}()

	// Copy content
	if _, err := io.Copy(gzWriter, srcFile); err != nil {
		return err
	}

	// Ensure everything is flushed
	if err := gzWriter.Close(); err != nil {
		return err
	}
	if err := gzFile.Close(); err != nil {
		return err
	}

	// Remove original uncompressed file
	return os.Remove(path)
}

// Close closes the log file and releases resources.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil
	return err
}

// MaxSize returns the maximum size before rotation.
func (w *RotatingWriter) MaxSize() int64 {
	if w == nil {
		return DefaultMaxSize
	}
	return w.maxSize
}

// Path returns the path to the current log file.
func (w *RotatingWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}
