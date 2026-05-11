// Package journal implements the GOOSE evening ritual — a privacy-first
// daily journal with local emotion analysis and long-term memory recall.
// SPEC-GOOSE-JOURNAL-001
package journal

import "time"

// JournalEntry is the caller-supplied input for a single journal write.
type JournalEntry struct {
	// UserID is the owning user; must be non-empty.
	UserID string
	// Date is the local calendar date for this entry; defaults to today.
	Date time.Time
	// Text is the free-form diary text (1 – MaxEntryTextBytes bytes).
	Text string
	// EmojiMood is an optional single emoji expressing the user's mood (≤ 8 bytes).
	EmojiMood string
	// AttachmentPaths are optional file paths; only the path is stored, no analysis.
	AttachmentPaths []string
	// PrivateMode marks the entry as private — LLM access is permanently forbidden.
	PrivateMode bool
}

// StoredEntry is the fully-materialised record persisted in SQLite.
type StoredEntry struct {
	// ID is a UUID primary key.
	ID string `json:"id"`
	// UserID is the owning user.
	UserID string `json:"user_id"`
	// Date is the calendar date (YYYY-MM-DD).
	Date time.Time `json:"date"`
	// Text is the plain-text diary body (privacy-critical: never logged).
	Text string `json:"text"`
	// EmojiMood is the raw emoji from the caller.
	EmojiMood string `json:"emoji_mood,omitempty"`
	// Vad holds the VAD emotion scores (0–1 range).
	Vad Vad `json:"vad"`
	// EmotionTags are the top-3 emotion category labels.
	EmotionTags []string `json:"emotion_tags"`
	// Anniversary holds anniversary metadata when detected (nil in M1).
	Anniversary *Anniversary `json:"anniversary,omitempty"`
	// WordCount is the token count of Text.
	WordCount int `json:"word_count"`
	// CreatedAt is the RFC3339 creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// AllowLoRATraining indicates whether this entry may be used for LoRA fine-tuning.
	AllowLoRATraining bool `json:"allow_lora_training"`
	// CrisisFlag is true when a crisis keyword was detected in Text.
	CrisisFlag bool `json:"crisis_flag"`
	// AttachmentPaths are the file paths supplied by the caller.
	AttachmentPaths []string `json:"attachment_paths,omitempty"`
}

// Vad represents a Valence-Arousal-Dominance emotion triple.
// All values are normalised to [0, 1].
//
// Valence   : 0 = most negative, 0.5 = neutral, 1 = most positive
// Arousal   : 0 = lowest activation, 1 = highest activation
// Dominance : 0 = helpless, 1 = in full control
type Vad struct {
	Valence   float64 `json:"valence"`
	Arousal   float64 `json:"arousal"`
	Dominance float64 `json:"dominance"`
}

// Anniversary describes a matching anniversary event (used in M2+).
type Anniversary struct {
	// Name is the human-readable label, e.g. "결혼기념일".
	Name string `json:"name"`
	// Type is the event category, e.g. "wedding", "birthday".
	Type string `json:"type"`
	// OriginalDate is the date the event first occurred.
	OriginalDate time.Time `json:"original_date"`
	// YearsAgo is the distance from OriginalDate to today in whole years.
	YearsAgo int `json:"years_ago"`
}

// Trend holds aggregated mood statistics for a calendar period (M2+).
type Trend struct {
	// Period is either "week" or "month".
	Period string `json:"period"`
	// From is the inclusive start of the aggregation window.
	From time.Time `json:"from"`
	// To is the inclusive end of the aggregation window.
	To time.Time `json:"to"`
	// AvgValence is the mean valence across entries in the window.
	AvgValence float64 `json:"avg_valence"`
	// AvgArousal is the mean arousal across entries in the window.
	AvgArousal float64 `json:"avg_arousal"`
	// AvgDominance is the mean dominance across entries in the window.
	AvgDominance float64 `json:"avg_dominance"`
	// MoodDistribution maps emotion category labels to occurrence counts.
	MoodDistribution map[string]int `json:"mood_distribution"`
	// EntryCount is the number of journal entries in the window.
	EntryCount int `json:"entry_count"`
	// SparklinePoints holds per-day valence values; NaN indicates a missing day.
	SparklinePoints []float64 `json:"sparkline_points"`
}

// MaxEntryTextBytes is the upper bound for entry text length.
const MaxEntryTextBytes = 10_000

// ErrJournalDisabled is returned when the journal feature is not enabled in config.
const ErrJournalDisabled = journalError("journal disabled: set config.journal.enabled=true to opt in")

// ErrInvalidUserID is returned when UserID is empty or mismatched.
const ErrInvalidUserID = journalError("invalid user ID: must be non-empty")

// ErrPersistFailed is returned when storage insert fails after retries.
const ErrPersistFailed = journalError("journal persist failed after retries; entry lost from queue")

// journalError is an unexported string error type for sentinel errors.
type journalError string

func (e journalError) Error() string { return string(e) }
