package trajectory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

const (
	// dirSuccess is the sub-directory name for successful trajectories.
	dirSuccess = "success"
	// dirFailed is the sub-directory name for failed trajectories.
	dirFailed = "failed"
	// dateFormat is the UTC date format used for file naming.
	dateFormat = "2006-01-02"
	// filePerm is the POSIX permission for trajectory files (REQ-TRAJECTORY-003).
	filePerm = 0o600
	// dirPerm is the POSIX permission for trajectory directories (REQ-TRAJECTORY-003).
	dirPerm = 0o700
)

// openFile tracks a currently open trajectory file handle.
type openFile struct {
	path         string
	file         *os.File
	bytesWritten int64
	rotationIdx  int
	dateStr      string
}

// Writer implements append-only JSON-L writes with size/date-based rotation.
// All public methods are safe for concurrent use.
//
// @MX:ANCHOR: Writer is the single I/O boundary for all trajectory persistence.
// @MX:REASON: All file creation, rotation, and error handling flows through WriteTrajectory.
// Changes to file layout, permission policy, or rotation logic must update this type only.
// @MX:SPEC: SPEC-GOOSE-TRAJECTORY-001
type Writer struct {
	baseDir      string
	maxFileBytes int64
	currentFiles map[string]*openFile // "success" | "failed" -> handle
	mu           sync.Mutex
	clock        clockwork.Clock
	logger       *zap.Logger
}

// NewWriter creates a Writer rooted at baseDir/trajectories/.
func NewWriter(baseDir string, maxFileBytes int64, clock clockwork.Clock, logger *zap.Logger) *Writer {
	if maxFileBytes <= 0 {
		maxFileBytes = 10_485_760
	}
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &Writer{
		baseDir:      filepath.Join(baseDir, "trajectories"),
		maxFileBytes: maxFileBytes,
		currentFiles: make(map[string]*openFile),
		clock:        clock,
		logger:       logger,
	}
}

// WriteTrajectory serializes t as a single JSON-L line and appends it to the
// appropriate bucket file. Errors are logged but never propagated
// (best-effort, REQ-TRAJECTORY-010).
func (w *Writer) WriteTrajectory(t *Trajectory) error {
	if t == nil {
		return nil
	}

	bucket := dirFailed
	if t.Completed {
		bucket = dirSuccess
	}

	data, err := json.Marshal(t)
	if err != nil {
		w.logWarn("trajectory marshal failed", t.SessionID, "", err)
		return nil // best-effort
	}
	line := append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	of, err := w.getOrOpenFile(bucket)
	if err != nil {
		w.logWarn("trajectory write failed", t.SessionID, "", err)
		return nil // best-effort
	}

	// Rotate if adding this line would exceed the size cap.
	if of.bytesWritten+int64(len(line)) > w.maxFileBytes {
		if err := w.rotate(bucket, of); err != nil {
			w.logWarn("trajectory rotate failed", t.SessionID, of.path, err)
			return nil
		}
		of, err = w.getOrOpenFile(bucket)
		if err != nil {
			w.logWarn("trajectory write failed after rotate", t.SessionID, "", err)
			return nil
		}
	}

	// Single write call per REQ-TRAJECTORY-015.
	n, err := of.file.Write(line)
	if err != nil {
		w.logWarn("trajectory write failed", t.SessionID, of.path, err)
		return nil
	}
	of.bytesWritten += int64(n)
	return nil
}

// Close flushes and closes all open file handles.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, of := range w.currentFiles {
		_ = of.file.Close()
	}
	w.currentFiles = make(map[string]*openFile)
	return nil
}

// getOrOpenFile returns (or creates) the current file handle for bucket.
// Must be called with w.mu held.
func (w *Writer) getOrOpenFile(bucket string) (*openFile, error) {
	now := w.clock.Now().UTC()
	dateStr := now.Format(dateFormat)

	of, exists := w.currentFiles[bucket]
	if exists && of.dateStr == dateStr {
		return of, nil
	}

	// Date rolled over — close old file.
	if exists {
		_ = of.file.Close()
		delete(w.currentFiles, bucket)
	}

	// Create directories with 0700.
	dir := filepath.Join(w.baseDir, bucket)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Enforce directory permission regardless of umask.
	if err := os.Chmod(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("chmod dir %s: %w", dir, err)
	}
	// Also fix parent trajectories/ dir.
	if err := os.Chmod(w.baseDir, dirPerm); err != nil {
		// Non-fatal if parent already has correct perms or is root-owned.
		w.logger.Warn("chmod trajectories/ dir failed (non-fatal)", zap.Error(err))
	}

	path := filepath.Join(dir, dateStr+".jsonl")
	f, err := openFileExclusive(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	// Report current size for rotation continuity on restart.
	info, err := f.Stat()
	var initialBytes int64
	if err == nil {
		initialBytes = info.Size()
	}

	newOf := &openFile{
		path:         path,
		file:         f,
		bytesWritten: initialBytes,
		dateStr:      dateStr,
	}
	w.currentFiles[bucket] = newOf
	return newOf, nil
}

// rotate closes the current file and advances to the next rotation index.
// Must be called with w.mu held.
func (w *Writer) rotate(bucket string, of *openFile) error {
	_ = of.file.Close()
	delete(w.currentFiles, bucket)

	of.rotationIdx++
	dir := filepath.Join(w.baseDir, bucket)
	newPath := filepath.Join(dir, fmt.Sprintf("%s-%d.jsonl", of.dateStr, of.rotationIdx))

	f, err := openFileExclusive(newPath)
	if err != nil {
		return fmt.Errorf("rotate open %s: %w", newPath, err)
	}

	w.currentFiles[bucket] = &openFile{
		path:        newPath,
		file:        f,
		rotationIdx: of.rotationIdx,
		dateStr:     of.dateStr,
	}
	return nil
}

// openFileExclusive opens path for appending with mode 0600 (REQ-TRAJECTORY-003).
func openFileExclusive(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, err
	}
	// Enforce permission explicitly to counteract umask (AC-013).
	if err := os.Chmod(path, filePerm); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("chmod %s: %w", path, err)
	}
	return f, nil
}

func (w *Writer) logWarn(msg, sessionID, path string, err error) {
	if w.logger == nil {
		return
	}
	w.logger.Warn(msg,
		zap.String("session_id", sessionID),
		zap.String("path", path),
		zap.Error(err),
	)
}

// currentFilePathForBucket returns the current open file path for a bucket.
// Used by tests and retention to avoid deleting open handles.
//
//nolint:unused // Accessed via export_test.go for black-box test packages.
func (w *Writer) currentFilePathForBucket(bucket string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if of, ok := w.currentFiles[bucket]; ok {
		return of.path
	}
	return ""
}
