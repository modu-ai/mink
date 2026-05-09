package telegram

import (
	"sync"
)

// streamingQueueDepth is the maximum number of pending updates buffered per
// chat_id while a streaming response is in progress (REQ-MTGM-S05).
const streamingQueueDepth = 5

// chatStreamQueue tracks active streaming sessions and per-chat FIFO queues.
// All operations are safe for concurrent use.
//
// @MX:ANCHOR: [AUTO] chatStreamQueue is the contention point that serialises
// inbound updates per chat_id during streaming.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-S05; fan_in via Handle
// streaming branch and unit tests (>= 3 callers).
type chatStreamQueue struct {
	mu      sync.Mutex
	active  map[int64]bool
	pending map[int64][]Update
}

// NewChatStreamQueue returns an empty chatStreamQueue ready for use.
func NewChatStreamQueue() *chatStreamQueue {
	return &chatStreamQueue{
		active:  make(map[int64]bool),
		pending: make(map[int64][]Update),
	}
}

// TryAcquire marks chatID as actively streaming. Returns true on success;
// returns false if a stream is already running for this chat_id.
func (q *chatStreamQueue) TryAcquire(chatID int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active[chatID] {
		return false
	}
	q.active[chatID] = true
	return true
}

// Enqueue appends update to chatID's pending FIFO. Returns true on success;
// returns false when the queue is full (REQ-MTGM-S05 max=5).
func (q *chatStreamQueue) Enqueue(chatID int64, update Update) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending[chatID]) >= streamingQueueDepth {
		return false
	}
	q.pending[chatID] = append(q.pending[chatID], update)
	return true
}

// Release clears the active flag for chatID and returns the next pending
// update (FIFO head) if any. When the queue is empty it returns ok=false.
// Active is always cleared so that the next Handle call can re-acquire.
func (q *chatStreamQueue) Release(chatID int64) (Update, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Always clear active so the next stream can acquire.
	delete(q.active, chatID)

	queue := q.pending[chatID]
	if len(queue) == 0 {
		delete(q.pending, chatID)
		return Update{}, false
	}
	next := queue[0]
	if len(queue) == 1 {
		delete(q.pending, chatID)
	} else {
		q.pending[chatID] = queue[1:]
	}
	return next, true
}

// PendingLen returns the current pending depth for chatID. Used in tests.
func (q *chatStreamQueue) PendingLen(chatID int64) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending[chatID])
}

// IsActive reports whether chatID currently has an active streaming session.
// Used in tests.
func (q *chatStreamQueue) IsActive(chatID int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.active[chatID]
}
