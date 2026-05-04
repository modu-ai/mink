// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-017, REQ-BR-011
// AC: AC-BR-015, AC-BR-011
// M4-T6 — resume header parsing and buffer replay wiring.

package bridge

import (
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
)

func TestParseLastSequence_PrefersXLastSequence(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set(HeaderLastSequence, "42")
	h.Set(HeaderLastEventID, "7")
	if got := parseLastSequence(h); got != 42 {
		t.Fatalf("expected X-Last-Sequence priority, got %d", got)
	}
}

func TestParseLastSequence_FallsBackToLastEventID(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set(HeaderLastEventID, "9")
	if got := parseLastSequence(h); got != 9 {
		t.Fatalf("expected SSE fallback to 9, got %d", got)
	}
}

func TestParseLastSequence_MalformedDefaultsToZero(t *testing.T) {
	t.Parallel()
	cases := []string{"", "not-a-number", "-1", "1.5", "abc"}
	for _, c := range cases {
		h := http.Header{}
		h.Set(HeaderLastSequence, c)
		if got := parseLastSequence(h); got != 0 {
			t.Fatalf("malformed %q must yield 0, got %d", c, got)
		}
	}
}

func TestResumer_ReplaysAfterLastSequence(t *testing.T) {
	t.Parallel()
	buf := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	for i := uint64(1); i <= 5; i++ {
		buf.Append(mkMsg(sid, i, "x"))
	}
	r := newResumer(buf, nil)

	h := http.Header{}
	h.Set(HeaderLastSequence, "3")
	got := r.Resume(sid, h)
	if len(got) != 2 {
		t.Fatalf("expected 2 replay messages, got %d", len(got))
	}
	if got[0].Sequence != 4 || got[1].Sequence != 5 {
		t.Fatalf("unexpected replay sequences: %+v", got)
	}
}

func TestResumer_ZeroLastSeqReplaysAll(t *testing.T) {
	t.Parallel()
	buf := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	for i := uint64(1); i <= 3; i++ {
		buf.Append(mkMsg(sid, i, "x"))
	}
	r := newResumer(buf, nil)
	got := r.Resume(sid, http.Header{})
	if len(got) != 3 {
		t.Fatalf("expected full replay, got %d", len(got))
	}
}

func TestResumer_NoBufferedMessagesReturnsNil(t *testing.T) {
	t.Parallel()
	buf := newOutboundBuffer(clockwork.NewFakeClock())
	r := newResumer(buf, nil)
	if got := r.Resume("nobody", http.Header{}); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
