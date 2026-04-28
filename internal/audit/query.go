package audit

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// QueryOptions specifies filtering options for querying audit logs.
// REQ-AUDIT-004: WHEN goose audit query [--since=...] [--type=...] 가 실행되면
//
//	the system SHALL 구조화된 검색 결과를 반환한다
//
// @MX:NOTE: [AUTO] Query filter options for audit log search
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-004
type QueryOptions struct {
	// Since filters events to only those after this time (inclusive)
	Since *time.Time
	// Until filters events to only those before this time (inclusive)
	Until *time.Time
	// Types filters events to only those matching these types
	Types []EventType
}

// Query searches audit logs in the specified directory and returns matching events.
// It reads both the current audit.log file and rotated .gz files.
//
// Files are read in stream (not loaded entirely into memory) to handle large logs.
// Corrupt JSON lines are skipped with a warning logged to stderr.
// Results are sorted by timestamp ascending.
//
// REQ-AUDIT-004: 구조화된 검색 결과를 반환한다
//
// @MX:ANCHOR: [AUTO] Core audit log query function
// @MX:REASON: Primary entry point for audit log search, fan_in >= 3 (CLI + future admin UI + potential analytics)
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-004
func Query(logDir string, opts QueryOptions) ([]AuditEvent, error) {
	var allEvents []AuditEvent

	// Check if directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		// Non-existent directory is not an error - return empty result
		return []AuditEvent{}, nil
	}

	// Find all audit log files (current + rotated)
	files, err := findAuditLogFiles(logDir)
	if err != nil {
		return []AuditEvent{}, fmt.Errorf("failed to find audit log files: %w", err)
	}

	// Read events from each file
	for _, file := range files {
		events, readErr := readEventsFromFile(file)
		if readErr != nil {
			// Log warning but continue processing other files
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", file, readErr)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	// Apply filters
	filtered := applyFilters(allEvents, opts)

	// Ensure we return empty slice instead of nil
	if filtered == nil {
		filtered = []AuditEvent{}
	}

	// Sort by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	return filtered, nil
}

// findAuditLogFiles finds all audit log files in the directory.
// It returns the current audit.log and any rotated .gz files.
func findAuditLogFiles(logDir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()

		// Match current audit.log
		if name == "audit.log" {
			files = append(files, filepath.Join(logDir, name))
			continue
		}

		// Match rotated files: audit.log.YYYYMMDD-HHMMSS.gz
		if strings.HasPrefix(name, "audit.log.") && strings.HasSuffix(name, ".gz") {
			files = append(files, filepath.Join(logDir, name))
		}
	}

	return files, nil
}

// readEventsFromFile reads events from a single log file.
// It handles both plain text and gzip-compressed files.
// Corrupt JSON lines are skipped with a warning.
func readEventsFromFile(filePath string) ([]AuditEvent, error) {
	var reader io.Reader

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Check if file is gzip-compressed
	if strings.HasSuffix(filePath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = file
	}

	// Stream-read file line by line
	var events []AuditEvent
	scanner := bufio.NewScanner(reader)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse JSON line
		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Corrupt JSON line - skip with warning
			fmt.Fprintf(os.Stderr, "Warning: skipping corrupt JSON line %d in %s: %v\n", lineNum, filePath, err)
			continue
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	return events, nil
}

// applyFilters applies time range and event type filters to events.
func applyFilters(events []AuditEvent, opts QueryOptions) []AuditEvent {
	var filtered []AuditEvent

	for _, event := range events {
		// Apply time range filter
		if opts.Since != nil && event.Timestamp.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && event.Timestamp.After(*opts.Until) {
			continue
		}

		// Apply event type filter
		if len(opts.Types) > 0 && !slices.Contains(opts.Types, event.Type) {
			continue
		}

		filtered = append(filtered, event)
	}

	return filtered
}
