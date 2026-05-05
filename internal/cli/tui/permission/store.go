// Package permission provides persistent storage for tool permission decisions.
package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Decision represents the user's choice for a tool permission request.
type Decision string

const (
	// DecisionAllowAlways persists permission to disk and skips future prompts.
	DecisionAllowAlways Decision = "allow"
	// DecisionDenyAlways persists denial to disk and skips future prompts.
	DecisionDenyAlways Decision = "deny"
)

// schema is the JSON structure for permissions.json.
type schema struct {
	Version int               `json:"version"`
	Tools   map[string]string `json:"tools"`
}

// Store manages persistent permission decisions.
//
// @MX:ANCHOR Store is the persistence contract for permission decisions.
// @MX:REASON fan_in >= 3: permission/update.go (resolve), permission/model.go (fast-path check), tui/update.go (startup load)
type Store struct {
	path string // path to permissions.json (~/.goose/permissions.json)
	mu   sync.Mutex
}

// NewStore creates a new Store that persists decisions at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads all stored permission decisions from disk.
// Returns an empty map if the file does not exist.
func (s *Store) Load() (map[string]Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Decision), nil
		}
		return nil, err
	}

	var sc schema
	if err := json.Unmarshal(data, &sc); err != nil {
		return make(map[string]Decision), nil
	}

	decisions := make(map[string]Decision, len(sc.Tools))
	for tool, d := range sc.Tools {
		decisions[tool] = Decision(d)
	}
	return decisions, nil
}

// Save writes a permission decision for tool to disk using an atomic
// write (tmp file + rename) to avoid partial writes.
func (s *Store) Save(tool string, decision Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load existing decisions to merge.
	existing := make(map[string]string)
	if data, err := os.ReadFile(s.path); err == nil {
		var sc schema
		if json.Unmarshal(data, &sc) == nil && sc.Tools != nil {
			existing = sc.Tools
		}
	}

	existing[tool] = string(decision)

	sc := schema{
		Version: 1,
		Tools:   existing,
	}

	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}

	// Atomic write: write to .tmp then rename.
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

// Has reports whether a persisted decision exists for tool,
// and returns the decision if found.
func (s *Store) Has(tool string) (Decision, bool) {
	decisions, err := s.Load()
	if err != nil {
		return "", false
	}
	d, ok := decisions[tool]
	return d, ok
}
