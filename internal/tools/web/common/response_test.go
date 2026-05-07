package common_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/tools/web/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStandardResponseShape verifies that OKResponse and ErrResponse produce
// the exact {ok, data|error, metadata} top-level structure required by AC-WEB-012.
func TestStandardResponseShape(t *testing.T) {
	t.Run("OKResponse has ok=true and data key", func(t *testing.T) {
		payload := map[string]string{"result": "hello"}
		meta := common.Metadata{CacheHit: false, DurationMs: 100}

		resp, err := common.OKResponse(payload, meta)
		require.NoError(t, err)
		assert.True(t, resp.OK)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Data)
		assert.Equal(t, false, resp.Metadata.CacheHit)
		assert.Equal(t, int64(100), resp.Metadata.DurationMs)

		// Unmarshal data back to check the payload
		var got map[string]string
		require.NoError(t, json.Unmarshal(resp.Data, &got))
		assert.Equal(t, "hello", got["result"])
	})

	t.Run("OKResponse with cache_hit=true", func(t *testing.T) {
		meta := common.Metadata{CacheHit: true, DurationMs: 0}
		resp, err := common.OKResponse("cached", meta)
		require.NoError(t, err)
		assert.True(t, resp.OK)
		assert.True(t, resp.Metadata.CacheHit)
	})

	t.Run("ErrResponse has ok=false and error key", func(t *testing.T) {
		meta := common.Metadata{}
		resp := common.ErrResponse("host_blocked", "host is blocklisted", false, 0, meta)
		assert.False(t, resp.OK)
		assert.Nil(t, resp.Data)
		require.NotNil(t, resp.Error)
		assert.Equal(t, "host_blocked", resp.Error.Code)
		assert.Equal(t, "host is blocklisted", resp.Error.Message)
		assert.False(t, resp.Error.Retryable)
		assert.Equal(t, 0, resp.Error.RetryAfterSeconds)
	})

	t.Run("ErrResponse with retryable and retry_after_seconds", func(t *testing.T) {
		meta := common.Metadata{}
		resp := common.ErrResponse("ratelimit_exhausted", "rate limit hit", true, 42, meta)
		assert.False(t, resp.OK)
		require.NotNil(t, resp.Error)
		assert.True(t, resp.Error.Retryable)
		assert.Equal(t, 42, resp.Error.RetryAfterSeconds)
	})

	t.Run("Response marshals to JSON with exact top-level keys", func(t *testing.T) {
		meta := common.Metadata{CacheHit: false, DurationMs: 50}
		resp, err := common.OKResponse("data", meta)
		require.NoError(t, err)

		b, err := json.Marshal(resp)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(b, &raw))

		assert.Contains(t, raw, "ok")
		assert.Contains(t, raw, "metadata")
		// data present for OK response
		assert.Contains(t, raw, "data")
		// error must not appear in ok response
		assert.NotContains(t, raw, "error")
	})

	t.Run("Error response marshals without data key", func(t *testing.T) {
		meta := common.Metadata{}
		resp := common.ErrResponse("permission_denied", "denied", false, 0, meta)

		b, err := json.Marshal(resp)
		require.NoError(t, err)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(b, &raw))

		assert.Contains(t, raw, "ok")
		assert.Contains(t, raw, "error")
		assert.Contains(t, raw, "metadata")
		assert.NotContains(t, raw, "data")
	})

	t.Run("OKResponse returns error for non-marshalable value", func(t *testing.T) {
		// math.NaN() cannot be marshaled to JSON.
		_, err := common.OKResponse(math.NaN(), common.Metadata{})
		assert.Error(t, err)
	})
}

// TestDepsHelpers verifies the SubjectID and Now helper methods on Deps.
func TestDepsHelpers(t *testing.T) {
	t.Run("SubjectID defaults to agent:goose when provider is nil", func(t *testing.T) {
		d := &common.Deps{}
		assert.Equal(t, "agent:goose", d.SubjectID(context.Background()))
	})

	t.Run("SubjectID uses provider when set", func(t *testing.T) {
		d := &common.Deps{
			SubjectIDProvider: func(_ context.Context) string { return "agent:custom" },
		}
		assert.Equal(t, "agent:custom", d.SubjectID(context.Background()))
	})

	t.Run("Now defaults to time.Now when clock is nil", func(t *testing.T) {
		d := &common.Deps{}
		before := time.Now()
		got := d.Now()
		after := time.Now()
		assert.True(t, !got.Before(before) && !got.After(after),
			"Now() without clock should return approximately current time")
	})

	t.Run("Now uses injected clock", func(t *testing.T) {
		fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		d := &common.Deps{Clock: func() time.Time { return fixed }}
		assert.Equal(t, fixed, d.Now())
	})
}
