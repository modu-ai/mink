package fsaccess

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// PolicyReloader watches a security policy file and reloads it on changes.
// It uses polling-based detection (no external dependencies).
//
// REQ-FSACCESS-004: Hot-reload security.yaml on file change
// AC-04: Hot reload functionality
//
// @MX:NOTE: [AUTO] Polling-based policy reloader (fsnotify-free)
// @MX:REASON: Avoids external dependency on fsnotify; polling at configurable interval
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-004, AC-04
type PolicyReloader struct {
	configPath string
	interval   time.Duration
	logger     *zap.Logger

	engine    *DecisionEngine
	mu        sync.RWMutex
	modTime   time.Time
	running   atomic.Bool
	stopCh    chan struct{}
	reloadCnt atomic.Int64
}

// NewPolicyReloader creates a new PolicyReloader.
// The interval controls how often the file is checked for changes (default: 5s).
func NewPolicyReloader(configPath string, engine *DecisionEngine, interval time.Duration, logger *zap.Logger) (*PolicyReloader, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path must not be empty")
	}
	if engine == nil {
		return nil, fmt.Errorf("engine must not be nil")
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}

	// Capture initial mod time
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	return &PolicyReloader{
		configPath: configPath,
		interval:   interval,
		logger:     logger,
		engine:     engine,
		modTime:    info.ModTime(),
		stopCh:     make(chan struct{}),
	}, nil
}

// Start begins polling for file changes in a background goroutine.
func (r *PolicyReloader) Start() {
	if !r.running.CompareAndSwap(false, true) {
		return // already running
	}
	go r.poll()
}

// Stop terminates the polling goroutine.
func (r *PolicyReloader) Stop() {
	if r.running.CompareAndSwap(true, false) {
		close(r.stopCh)
	}
}

// ReloadCount returns the number of successful reloads.
func (r *PolicyReloader) ReloadCount() int64 {
	return r.reloadCnt.Load()
}

// ReloadNow forces an immediate reload of the policy file.
func (r *PolicyReloader) ReloadNow() error {
	return r.reload()
}

func (r *PolicyReloader) poll() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.checkAndReload(); err != nil && r.logger != nil {
				r.logger.Warn("policy reload check failed", zap.Error(err))
			}
		}
	}
}

func (r *PolicyReloader) checkAndReload() error {
	info, err := os.Stat(r.configPath)
	if err != nil {
		return fmt.Errorf("stat failed: %w", err)
	}

	if info.ModTime().After(r.modTime) {
		return r.reload()
	}
	return nil
}

func (r *PolicyReloader) reload() error {
	policy, err := LoadSecurityPolicy(r.configPath)
	if err != nil {
		return fmt.Errorf("load policy failed: %w", err)
	}

	r.mu.Lock()
	r.engine.UpdatePolicy(policy)
	r.modTime = time.Now()
	r.mu.Unlock()

	r.reloadCnt.Add(1)

	if r.logger != nil {
		r.logger.Info("security policy reloaded",
			zap.String("path", r.configPath),
			zap.Int64("reload_count", r.reloadCnt.Load()),
		)
	}
	return nil
}
