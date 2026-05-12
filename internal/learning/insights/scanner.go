package insights

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/modu-ai/mink/internal/learning/trajectory"
	"go.uber.org/zap"
)

const (
	// maxLineBytes is the maximum bytes allowed per single JSON line.
	// Lines exceeding this are skipped with a warning (pathological case protection).
	maxLineBytes = 10 * 1024 * 1024 // 10 MB
)

// TrajectoryReader streams trajectory entries from .jsonl files.
//
// @MX:ANCHOR: [AUTO] Entry point for trajectory consumption in the insights pipeline.
// @MX:REASON: ScanPeriod is called by InsightsEngine.Extract and feeds all
// aggregators. Signature changes propagate to overview, models, tools, activity, analyzer.
// @MX:SPEC: SPEC-GOOSE-INSIGHTS-001
type TrajectoryReader struct {
	baseDir string
	logger  *zap.Logger
}

// NewTrajectoryReader creates a reader rooted at baseDir.
// baseDir is typically $GOOSE_HOME/trajectories.
func NewTrajectoryReader(baseDir string, logger *zap.Logger) *TrajectoryReader {
	return &TrajectoryReader{baseDir: baseDir, logger: logger}
}

// ScanPeriod streams trajectories within the given period.
// bucket selects "success", "failed", or "" (both).
// Files outside the period date range are skipped entirely.
// Malformed JSON lines are logged and skipped; they do not abort scanning.
// The returned channel is closed when scanning is complete.
func (r *TrajectoryReader) ScanPeriod(period InsightsPeriod, bucket string) <-chan *trajectory.Trajectory {
	ch := make(chan *trajectory.Trajectory, 256)
	go func() {
		defer close(ch)
		buckets := r.resolveBuckets(bucket)
		for _, b := range buckets {
			bucketDir := filepath.Join(r.baseDir, b)
			entries, err := os.ReadDir(bucketDir)
			if err != nil {
				if !os.IsNotExist(err) && r.logger != nil {
					r.logger.Warn("cannot read trajectory bucket dir",
						zap.String("dir", bucketDir),
						zap.Error(err))
				}
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if !isDateInPeriod(name, period) {
					continue
				}
				path := filepath.Join(bucketDir, name)
				r.scanFile(path, ch)
			}
		}
	}()
	return ch
}

// ScanAll streams all trajectories in baseDir without period filtering.
// Useful for AllTime() period.
func (r *TrajectoryReader) ScanAll(bucket string) <-chan *trajectory.Trajectory {
	return r.ScanPeriod(AllTime(), bucket)
}

// resolveBuckets returns the list of bucket directory names to scan.
func (r *TrajectoryReader) resolveBuckets(bucket string) []string {
	switch bucket {
	case "success":
		return []string{"success"}
	case "failed":
		return []string{"failed"}
	default:
		return []string{"success", "failed"}
	}
}

// isDateInPeriod checks whether a filename like "YYYY-MM-DD.jsonl" falls within period.
// Non-date filenames are always included (they cannot be filtered by date).
func isDateInPeriod(name string, period InsightsPeriod) bool {
	// Strip .jsonl suffix.
	base := name
	if len(base) > 6 && base[len(base)-6:] == ".jsonl" {
		base = base[:len(base)-6]
	}
	t, err := time.Parse("2006-01-02", base)
	if err != nil {
		// Non-date filename; include it.
		return true
	}
	// Compare dates only (day granularity): file date must not be strictly outside period.
	fileDay := t.UTC().Truncate(24 * time.Hour)
	fromDay := period.From.UTC().Truncate(24 * time.Hour)
	toDay := period.To.UTC().Truncate(24 * time.Hour)
	return !fileDay.Before(fromDay) && !fileDay.After(toDay)
}

// scanFile reads a .jsonl file and sends each valid Trajectory to ch.
// Files >= streamingThresholdBytes are read line-by-line without buffering the whole file.
func (r *TrajectoryReader) scanFile(path string, ch chan<- *trajectory.Trajectory) {
	f, err := os.Open(path)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("cannot open trajectory file",
				zap.String("path", path),
				zap.Error(err))
		}
		return
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && r.logger != nil {
			r.logger.Warn("error closing trajectory file",
				zap.String("path", path),
				zap.Error(closeErr))
		}
	}()

	r.scanReader(f, path, ch)
}

// scanReader reads newline-delimited JSON from r and sends valid Trajectory records.
// It uses bufio.Reader.ReadBytes to avoid token-size limits of bufio.Scanner,
// which has a default MaxScanTokenSize of 64KB.
func (r *TrajectoryReader) scanReader(reader io.Reader, path string, ch chan<- *trajectory.Trajectory) {
	br := bufio.NewReaderSize(reader, 64*1024)
	lineNum := 0
	for {
		lineNum++
		line, err := br.ReadBytes('\n')
		// Trim trailing newline/whitespace.
		line = trimNewline(line)

		if len(line) > 0 {
			if len(line) > maxLineBytes {
				if r.logger != nil {
					r.logger.Warn("trajectory line exceeds max size, skipping",
						zap.String("path", path),
						zap.Int("line_number", lineNum),
						zap.Int("size_bytes", len(line)))
				}
			} else {
				var t trajectory.Trajectory
				if jsonErr := json.Unmarshal(line, &t); jsonErr != nil {
					if r.logger != nil {
						r.logger.Warn("malformed trajectory JSON line, skipping",
							zap.String("path", path),
							zap.Int("line_number", lineNum),
							zap.Error(jsonErr))
					}
				} else {
					ch <- &t
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				if r.logger != nil {
					r.logger.Warn("error reading trajectory file",
						zap.String("path", path),
						zap.Int("line_number", lineNum),
						zap.Error(err))
				}
			}
			break
		}
	}
}

// trimNewline removes trailing \r and \n bytes from a byte slice.
func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// writeTrajectoryLine serializes a Trajectory to a JSON line (for test helpers).
func writeTrajectoryLine(w io.Writer, t *trajectory.Trajectory) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
