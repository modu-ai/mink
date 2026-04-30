// export_test.go exposes internal functions for white-box testing.
package trajectory

import "go.uber.org/zap"

// CurrentFilePathForBucket exposes the internal currentFilePathForBucket method for tests.
func (w *Writer) CurrentFilePathForBucket(bucket string) string {
	return w.currentFilePathForBucket(bucket)
}

// LogWarn exposes the internal logWarn method for coverage testing.
func (w *Writer) LogWarn(msg, sessionID, path string, err error) {
	w.logWarn(msg, sessionID, path, err)
}

// SetLogger sets the logger for testing purposes.
func (w *Writer) SetLogger(logger *zap.Logger) {
	w.logger = logger
}
