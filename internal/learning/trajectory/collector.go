package trajectory

import (
	"context"
	"sync"
	"time"

	"github.com/modu-ai/goose/internal/learning/trajectory/redact"
	"go.uber.org/zap"
)

// hookEvent is the internal event passed through the collector's channel.
type hookEvent struct {
	kind      eventKind
	sessionID string
	entries   []TrajectoryEntry
	success   bool
	meta      TrajectoryMetadata
	model     string
}

type eventKind int

const (
	eventTurn eventKind = iota
	eventTerminal
)

// sessionBuffer holds in-flight entries for a single session.
type sessionBuffer struct {
	sessionID string
	entries   []TrajectoryEntry
	startedAt time.Time
	model     string
	mu        sync.Mutex
}

// Collector receives QueryEngine hook events, buffers turns per session,
// and flushes complete trajectories to Writer on Terminal.
//
// @MX:ANCHOR: Collector is the entry point from QueryEngine into the learning pipeline.
// @MX:REASON: OnTurn and OnTerminal are called from QueryEngine goroutines; they must
// never block (REQ-TRAJECTORY-013). All I/O is routed to the internal worker goroutine.
// @MX:SPEC: SPEC-GOOSE-TRAJECTORY-001
// @MX:WARN: Collector spawns a long-lived worker goroutine. Callers MUST call Close()
// @MX:REASON: Failure to close causes goroutine leak detected by goleak in tests.
type Collector struct {
	cfg       TelemetryConfig
	buffers   map[string]*sessionBuffer
	mu        sync.RWMutex
	writer    *Writer
	redactor  redact.Chain
	events    chan hookEvent
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	logger    *zap.Logger
}

// NewCollector creates a Collector and starts its worker goroutine.
// Call Close() to drain and stop the worker.
func NewCollector(cfg TelemetryConfig, writer *Writer, chain redact.Chain, logger *zap.Logger) *Collector {
	c := &Collector{
		cfg:      cfg,
		buffers:  make(map[string]*sessionBuffer),
		writer:   writer,
		redactor: chain,
		events:   make(chan hookEvent, 4096), // buffered to reduce backpressure
		done:     make(chan struct{}),
		logger:   logger,
	}

	if cfg.Enabled {
		c.wg.Add(1)
		go c.worker()
	}
	return c
}

// OnTurn receives a single turn from PostSamplingHooks.
// It is designed to return in <1ms (REQ-TRAJECTORY-013): it only enqueues
// events on the buffered channel and returns immediately.
func (c *Collector) OnTurn(sessionID string, entries []TrajectoryEntry) {
	if !c.cfg.Enabled {
		return
	}
	ev := hookEvent{
		kind:      eventTurn,
		sessionID: sessionID,
		entries:   entries,
	}
	select {
	case c.events <- ev:
	default:
		// Channel full — drop turn and log a warning rather than blocking.
		if c.logger != nil {
			c.logger.Warn("trajectory event channel full, dropping turn",
				zap.String("session_id", sessionID))
		}
	}
}

// OnTerminal is called when the QueryEngine session ends.
// success=true routes to the success bucket, false to failed.
func (c *Collector) OnTerminal(sessionID string, success bool, meta TrajectoryMetadata) {
	if !c.cfg.Enabled {
		return
	}
	ev := hookEvent{
		kind:      eventTerminal,
		sessionID: sessionID,
		success:   success,
		meta:      meta,
	}
	select {
	case c.events <- ev:
	default:
		if c.logger != nil {
			c.logger.Warn("trajectory event channel full, dropping terminal",
				zap.String("session_id", sessionID))
		}
	}
}

// Close drains the event channel and stops the worker goroutine.
// Close is idempotent and safe to call multiple times.
func (c *Collector) Close(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	var closeErr error
	c.closeOnce.Do(func() {
		close(c.done)

		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			closeErr = ctx.Err()
			return
		}

		if c.writer != nil {
			closeErr = c.writer.Close()
		}
	})
	return closeErr
}

// worker processes events from the channel on a dedicated goroutine.
func (c *Collector) worker() {
	defer c.wg.Done()
	for {
		select {
		case ev := <-c.events:
			c.handleEvent(ev)
		case <-c.done:
			// Drain remaining events before exiting.
			for {
				select {
				case ev := <-c.events:
					c.handleEvent(ev)
				default:
					return
				}
			}
		}
	}
}

func (c *Collector) handleEvent(ev hookEvent) {
	switch ev.kind {
	case eventTurn:
		c.appendTurn(ev.sessionID, ev.entries)
	case eventTerminal:
		c.flushSession(ev.sessionID, ev.success, ev.meta)
	}
}

// appendTurn adds entries to the session buffer and spills if cap exceeded.
func (c *Collector) appendTurn(sessionID string, entries []TrajectoryEntry) {
	buf := c.getOrCreateBuffer(sessionID)
	buf.mu.Lock()
	defer buf.mu.Unlock()

	buf.entries = append(buf.entries, entries...)

	cap := c.cfg.InMemoryTurnCap
	if cap <= 0 {
		cap = 1000
	}
	if len(buf.entries) > cap {
		c.spill(buf)
	}
}

// spill writes the oldest half of buf.entries to disk as a partial fragment.
// Must be called with buf.mu held.
func (c *Collector) spill(buf *sessionBuffer) {
	half := len(buf.entries) / 2
	toSpill := buf.entries[:half]
	buf.entries = buf.entries[half:]

	t := &Trajectory{
		Conversations: toSpill,
		Timestamp:     time.Now().UTC(),
		SessionID:     buf.sessionID,
		Completed:     true, // partial spills go to success bucket for now
		Metadata: TrajectoryMetadata{
			Partial:   true,
			TurnCount: len(toSpill),
		},
	}

	// Apply redaction before write.
	c.applyRedact(t)

	if c.writer != nil {
		_ = c.writer.WriteTrajectory(t)
	}
}

// flushSession assembles the final Trajectory and writes it.
func (c *Collector) flushSession(sessionID string, success bool, meta TrajectoryMetadata) {
	c.mu.Lock()
	buf, ok := c.buffers[sessionID]
	if !ok {
		c.mu.Unlock()
		return
	}
	delete(c.buffers, sessionID)
	c.mu.Unlock()

	buf.mu.Lock()
	entries := buf.entries
	startedAt := buf.startedAt
	model := buf.model
	buf.mu.Unlock()

	meta.TurnCount = len(entries)
	meta.DurationMs = time.Since(startedAt).Milliseconds()

	t := &Trajectory{
		Conversations: entries,
		Timestamp:     time.Now().UTC(),
		Model:         model,
		Completed:     success,
		SessionID:     sessionID,
		Metadata:      meta,
	}

	c.applyRedact(t)

	if c.writer != nil {
		_ = c.writer.WriteTrajectory(t)
	}
}

// applyRedact runs the redaction chain over all non-system entries.
func (c *Collector) applyRedact(t *Trajectory) {
	for i := range t.Conversations {
		e := &redact.Entry{
			From:  string(t.Conversations[i].From),
			Value: t.Conversations[i].Value,
		}
		c.redactor.Apply(e)
		t.Conversations[i].Value = e.Value
	}
}

// getOrCreateBuffer returns an existing buffer or creates a new one.
func (c *Collector) getOrCreateBuffer(sessionID string) *sessionBuffer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if buf, ok := c.buffers[sessionID]; ok {
		return buf
	}
	buf := &sessionBuffer{
		sessionID: sessionID,
		startedAt: time.Now(),
	}
	c.buffers[sessionID] = buf
	return buf
}
