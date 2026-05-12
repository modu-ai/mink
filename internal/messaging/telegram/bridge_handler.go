package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// typingActionInterval is the cadence at which sendChatAction(typing) is
// re-emitted while the bot is preparing a response (REQ-MTGM-O02). Telegram
// displays the indicator for approximately 5 seconds, so re-emitting every
// 5 seconds keeps the indicator continuously visible.
const typingActionInterval = 5 * time.Second

// startTypingIndicator launches a goroutine that periodically calls
// sendChatAction(typing) for the given chat until ctx is cancelled. The
// goroutine emits the first action immediately, then every typingActionInterval.
//
// @MX:WARN: [AUTO] startTypingIndicator spawns a goroutine bounded by ctx;
// callers MUST cancel ctx (or let it expire) to terminate the goroutine.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P4 REQ-MTGM-O02; goroutine lifetime
// is bounded by context cancellation, preventing leaks.
func startTypingIndicator(ctx context.Context, client Client, chatID int64, logger *zap.Logger) {
	go func() {
		// Send first action immediately so the indicator appears without delay.
		if err := client.SendChatAction(ctx, chatID, ChatActionTyping); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				logger.Warn("sendChatAction failed", zap.Error(err), zap.Int64("chat_id", chatID))
			}
		}
		ticker := time.NewTicker(typingActionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := client.SendChatAction(ctx, chatID, ChatActionTyping); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					logger.Warn("sendChatAction failed", zap.Error(err), zap.Int64("chat_id", chatID))
				}
			}
		}
	}()
}

// AgentQuery is the narrow interface for sending a text message (and optional
// attachment paths) to the Mink agent and receiving a text response.
//
// The concrete implementation (AgentQueryAdapter) wraps the in-process ChatService.
// The narrow interface keeps BridgeQueryHandler independent of the gRPC
// package and makes it trivially mockable in tests.
//
// P3 extends the signature with attachments []string to support file attachment
// forwarding (strategy-p3.md §C.5 option i).
//
// @MX:ANCHOR: [AUTO] AgentQuery decouples BridgeQueryHandler from the gRPC transport.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via BridgeQueryHandler, bootstrap, tests (>= 3 callers).
type AgentQuery interface {
	// Query sends text (and optional attachment paths) to the agent.
	// attachments is a list of local file paths; an empty slice means no attachments.
	Query(ctx context.Context, text string, attachments []string) (string, error)
}

// maxMessageLength is the maximum number of UTF-8 bytes accepted per inbound
// Telegram message (REQ-MTGM-E04).
const maxMessageLength = 4096

// handleCallback processes a callback_query update (inline keyboard button click).
// It acknowledges the callback immediately (fire-and-log), checks the 60-second
// timeout, audits the event, and forwards to the agent (REQ-MTGM-E05).
//
// @MX:NOTE: [AUTO] handleCallback is the dedicated branch for inline keyboard events.
// Per strategy-p3.md §D.3, answerCallbackQuery is called first, then the agent query proceeds.
func (h *BridgeQueryHandler) handleCallback(ctx context.Context, update Update) error {
	cq := update.CallbackQuery
	chatID := cq.ChatID
	msgID := int64(cq.MessageID)
	now := time.Now().UTC()

	// Acknowledge the click immediately to remove the spinner from the user's device.
	// A 400 response means the callback timed out on Telegram's side.
	if err := h.client.AnswerCallbackQuery(ctx, cq.ID); err != nil {
		expired := now.Sub(cq.ReceivedAt) > callbackQueryTimeout
		auditMeta := map[string]any{
			"source":      "callback_query",
			"callback_id": cq.ID,
		}
		if expired {
			auditMeta["callback_expired"] = true
		}
		_ = h.audit.RecordInbound(ctx, chatID, msgID, cq.Data, auditMeta)
		h.logger.Warn("answerCallbackQuery failed", zap.Error(err), zap.Int64("chat_id", chatID))
	}

	// Check 60-second timeout (REQ-MTGM-N04) — expired callbacks are still processed
	// but audited as expired (strategy-p3.md §D.7).
	auditMeta := map[string]any{
		"source":      "callback_query",
		"callback_id": cq.ID,
	}
	if now.Sub(cq.ReceivedAt) > callbackQueryTimeout {
		auditMeta["callback_expired"] = true
	}

	// Check access control (same as message path).
	mapping, found, err := h.store.GetUserMapping(ctx, chatID)
	if err != nil {
		h.logger.Error("store lookup failed", zap.Error(err), zap.Int64("chat_id", chatID))
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}
	if found && !mapping.Allowed {
		// Blocked user — silent drop (REQ-MTGM-N05).
		_ = h.audit.RecordInbound(ctx, chatID, msgID, cq.Data, map[string]any{"dropped_blocked": true})
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}
	if !found {
		// Not registered — send gate message and stop.
		gateMsg := fmt.Sprintf("이 봇은 사전 승인된 사용자만 사용할 수 있습니다. 관리자에게 chat_id `%d` 를 전달하세요.", chatID)
		if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: gateMsg}); sendErr == nil {
			_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), gateMsg, nil)
		}
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	// Audit the inbound callback.
	_ = h.audit.RecordInbound(ctx, chatID, msgID, cq.Data, auditMeta)

	// Forward to agent: callback data is passed as text with prefix (strategy-p3.md §D.4).
	queryText := fmt.Sprintf("[callback_query] data=%q message_id=%d", cq.Data, cq.MessageID)

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, queryErr := h.agent.Query(queryCtx, queryText, nil)
	if queryErr != nil {
		h.logger.Error("agent query failed (callback)", zap.Error(queryErr), zap.Int64("chat_id", chatID))
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: response}); sendErr != nil {
		h.logger.Error("send callback response failed", zap.Error(sendErr), zap.Int64("chat_id", chatID))
	} else {
		_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), response, nil)
	}

	_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
	return nil
}

// streamPrefix is the command prefix that activates the streaming branch.
// Messages starting with "/stream " will use editMessageText-based streaming.
const streamPrefix = "/stream "

// BridgeQueryHandler dispatches inbound Telegram messages to the Mink agent,
// enforces the first-message access gate, records audit events, and persists
// the chat_id mapping and polling offset.
//
// P4 adds an optional streaming branch: when stream is non-nil and the message
// starts with "/stream " (or DefaultStreaming=true), the handler sends a placeholder
// message and updates it via editMessageText as chunks arrive (REQ-MTGM-E02).
//
// T4 adds per-chat FIFO streaming queues (REQ-MTGM-S05): while a streaming
// response is in progress for a given chat_id, additional inbound messages are
// buffered up to streamingQueueDepth; excess messages are dropped with an apology.
//
// @MX:WARN: [AUTO] BridgeQueryHandler.Handle contains >= 8 conditional branches
// for access control, length gate, agent timeout, and error handling.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 REQ-MTGM-S01/S04/N05/E04 require multiple
// independent guards; complexity is inherent to the security model.
type BridgeQueryHandler struct {
	client      Client
	store       Store
	audit       *AuditWrapper
	agent       AgentQuery
	stream      AgentStream // P4: optional streaming agent; nil means streaming is disabled.
	cfg         *Config
	streamQueue *chatStreamQueue // T4: per-chat FIFO queues for in-flight streaming (REQ-MTGM-S05).
	logger      *zap.Logger
}

// NewBridgeQueryHandler constructs a BridgeQueryHandler.
//
// @MX:ANCHOR: [AUTO] NewBridgeQueryHandler is the constructor for the P2 handler.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P2; fan_in via bootstrap.Start, integration tests, and unit tests.
func NewBridgeQueryHandler(
	client Client,
	store Store,
	audit *AuditWrapper,
	agent AgentQuery,
	cfg *Config,
	logger *zap.Logger,
) *BridgeQueryHandler {
	return &BridgeQueryHandler{
		client:      client,
		store:       store,
		audit:       audit,
		agent:       agent,
		cfg:         cfg,
		streamQueue: NewChatStreamQueue(),
		logger:      logger,
	}
}

// WithStream wires an AgentStream into the handler, enabling the P4 streaming
// branch. Returns the handler for method chaining.
//
// When stream is non-nil, messages prefixed with "/stream " or sent when
// DefaultStreaming=true will use the editMessageText-based streaming path.
func (h *BridgeQueryHandler) WithStream(s AgentStream) *BridgeQueryHandler {
	h.stream = s
	return h
}

// callbackQueryTimeout is the Telegram-imposed window to answer a callback.
// Callbacks older than this are logged as expired but still processed
// (strategy-p3.md §D.7).
const callbackQueryTimeout = 60 * time.Second

// Handle processes a single Telegram Update.
//
// Flow summary:
//  1. Dispatch to callback_query branch if present.
//  2. Skip updates with no message text; advance offset.
//  3. Reject over-length messages (> 4096 bytes).
//  4. Enforce first-message gate or auto-admit.
//  5. Drop silently if user is blocked.
//  6. Query the agent with a 30-second timeout.
//  7. Deliver response; advance offset.
func (h *BridgeQueryHandler) Handle(ctx context.Context, update Update) error {
	// 1. Dispatch callback_query updates.
	if update.CallbackQuery != nil {
		return h.handleCallback(ctx, update)
	}

	// 2. Skip empty or no-message updates.
	if update.Message == nil || update.Message.Text == "" {
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	text := update.Message.Text
	chatID := update.Message.ChatID
	msgID := int64(update.Message.ID)
	now := time.Now().UTC()

	// 2. Length gate (REQ-MTGM-E04).
	if len(text) > maxMessageLength {
		if err := h.audit.RecordInbound(ctx, chatID, msgID, text, map[string]any{
			"length_exceeded": true,
			"length":          len(text),
		}); err != nil {
			h.logger.Warn("audit inbound failed", zap.Error(err))
		}
		rejectMsg := fmt.Sprintf("메시지가 너무 깁니다 (max %d chars)", maxMessageLength)
		if sent, err := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: rejectMsg}); err != nil {
			h.logger.Warn("send length rejection failed", zap.Error(err))
		} else {
			_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), rejectMsg, nil)
		}
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	// 3. Look up user mapping.
	mapping, found, err := h.store.GetUserMapping(ctx, chatID)
	if err != nil {
		h.logger.Error("store lookup failed", zap.Error(err), zap.Int64("chat_id", chatID))
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	// 4. Access control.
	if found && !mapping.Allowed {
		// Blocked user — silent drop (REQ-MTGM-N05).
		_ = h.audit.RecordInbound(ctx, chatID, msgID, text, map[string]any{"dropped_blocked": true})
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	if !found {
		if h.cfg.AutoAdmitFirstUser {
			// Auto-admit (REQ-MTGM-S04).
			mapping = UserMapping{
				ChatID:        chatID,
				UserProfileID: fmt.Sprintf("tg-%d", chatID),
				Allowed:       true,
				AutoAdmitted:  true,
				FirstSeenAt:   now,
				LastSeenAt:    now,
			}
			if err := h.store.PutUserMapping(ctx, mapping); err != nil {
				h.logger.Error("auto-admit store failed", zap.Error(err), zap.Int64("chat_id", chatID))
			} else {
				h.logger.Info("auto-admitted chat_id", zap.Int64("chat_id", chatID))
			}
		} else {
			// First-message gate (REQ-MTGM-S01).
			placeholder := UserMapping{
				ChatID:        chatID,
				UserProfileID: fmt.Sprintf("tg-%d", chatID),
				Allowed:       false,
				FirstSeenAt:   now,
				LastSeenAt:    now,
			}
			_ = h.store.PutUserMapping(ctx, placeholder)

			gateMsg := fmt.Sprintf("이 봇은 사전 승인된 사용자만 사용할 수 있습니다. 관리자에게 chat_id `%d` 를 전달하세요.", chatID)
			if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: gateMsg}); sendErr != nil {
				h.logger.Warn("send gate notice failed", zap.Error(sendErr))
			} else {
				_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), gateMsg, nil)
			}
			_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
			return nil
		}
	} else {
		// Update last seen for returning users.
		mapping.LastSeenAt = now
		if err := h.store.PutUserMapping(ctx, mapping); err != nil {
			h.logger.Warn("update last_seen failed", zap.Error(err))
		}
	}

	// Streaming branch (P4): activated by "/stream " prefix or DefaultStreaming=true.
	// Access control gates above must pass before reaching this point.
	if h.stream != nil {
		useStreaming := false
		streamText := text
		if strings.HasPrefix(text, streamPrefix) {
			useStreaming = true
			streamText = strings.TrimPrefix(text, streamPrefix)
		} else if h.cfg.DefaultStreaming {
			useStreaming = true
		}

		if useStreaming {
			// T4: per-chat streaming queue (REQ-MTGM-S05).
			// Try to acquire the streaming slot for this chat_id. If another stream
			// is already active, enqueue the update or drop it with an apology.
			if !h.streamQueue.TryAcquire(chatID) {
				if !h.streamQueue.Enqueue(chatID, update) {
					// Queue full — notify user and drop.
					busyMsg := "이전 응답 진행 중, 잠시 후 다시 시도하세요."
					if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{
						ChatID: chatID,
						Text:   busyMsg,
						Silent: h.cfg.SilentDefault,
					}); sendErr == nil {
						_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), busyMsg, map[string]any{
							"stream_queue_full": true,
						})
					}
					_ = h.audit.RecordInbound(ctx, chatID, msgID, text, map[string]any{
						"stream_queue_dropped": true,
					})
					_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
					return nil
				}
				// Successfully enqueued — in-flight stream will pick it up on release.
				_ = h.audit.RecordInbound(ctx, chatID, msgID, text, map[string]any{
					"stream_queued": true,
				})
				_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
				return nil
			}

			// Streaming slot acquired. Run the stream for this update.
			runErr := h.runStreamingForUpdate(ctx, update, text, msgID, chatID, mapping, streamText)

			// Drain pending updates serially. Release clears active each time,
			// allowing Handle to re-acquire cleanly. This avoids infinite recursion
			// because Handle → tryAcquire succeeds after each release.
			for {
				next, ok := h.streamQueue.Release(chatID)
				if !ok {
					break
				}
				if recurErr := h.Handle(ctx, next); recurErr != nil {
					h.logger.Warn("queued stream handle failed",
						zap.Error(recurErr),
						zap.Int64("chat_id", chatID))
				}
			}
			return runErr
		}
	}

	// Audit the inbound message after access control is resolved.
	inboundMeta := map[string]any{}
	if mapping.AutoAdmitted {
		inboundMeta["auto_admitted"] = true
	}
	_ = h.audit.RecordInbound(ctx, chatID, msgID, text, inboundMeta)

	// 5. Query agent with timeout.
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Emit typing indicator if configured (REQ-MTGM-O02). The goroutine is
	// bounded by queryCtx, which expires when the agent responds or times out.
	if h.cfg.TypingIndicator {
		typingCtx, typingCancel := context.WithCancel(queryCtx)
		defer typingCancel()
		startTypingIndicator(typingCtx, h.client, chatID, h.logger)
	}

	response, queryErr := h.agent.Query(queryCtx, text, nil)
	if queryErr != nil {
		if errors.Is(queryErr, context.DeadlineExceeded) || errors.Is(queryErr, context.Canceled) {
			// Timeout — graceful response, no error propagation.
			timeoutMsg := "처리 시간 초과, 다시 시도해 주세요."
			if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: timeoutMsg}); sendErr != nil {
				h.logger.Warn("send timeout notice failed", zap.Error(sendErr))
			} else {
				_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), timeoutMsg, map[string]any{"query_timeout": true})
			}
			_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
			return nil
		}

		// Other agent errors — graceful response.
		h.logger.Error("agent query failed", zap.Error(queryErr), zap.Int64("chat_id", chatID))
		errMsg := "요청 처리 중 오류가 발생했습니다. 잠시 후 다시 시도해 주세요."
		if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: errMsg}); sendErr != nil {
			h.logger.Warn("send error notice failed", zap.Error(sendErr))
		} else {
			_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), errMsg, map[string]any{"query_error": true})
		}
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	// 6. Deliver agent response.
	sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{ChatID: chatID, Text: response})
	if sendErr != nil {
		h.logger.Error("send response failed", zap.Error(sendErr), zap.Int64("chat_id", chatID))
	} else {
		_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), response, nil)
	}

	_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
	return nil
}

// runStreamingForUpdate executes a streaming response for a single update.
// The caller must have already verified access control and acquired the
// streamQueue slot for chatID. Audit, streaming, and offset-advance are all
// handled here.
//
// @MX:NOTE: [AUTO] runStreamingForUpdate is extracted from Handle to keep the
// per-chat queue drain loop readable. It owns the full streaming lifecycle for
// one update: audit → typing → stream open → runStreaming → audit out → offset.
func (h *BridgeQueryHandler) runStreamingForUpdate(
	ctx context.Context,
	update Update,
	text string,
	msgID int64,
	chatID int64,
	mapping UserMapping,
	streamText string,
) error {
	inboundMeta := map[string]any{"streaming": true}
	if mapping.AutoAdmitted {
		inboundMeta["auto_admitted"] = true
	}
	_ = h.audit.RecordInbound(ctx, chatID, msgID, text, inboundMeta)

	streamCtx, streamCancel := context.WithTimeout(ctx, 60*time.Second)
	defer streamCancel()

	// Emit typing indicator for the streaming branch (REQ-MTGM-O02).
	if h.cfg.TypingIndicator {
		typingCtx, typingCancel := context.WithCancel(streamCtx)
		defer typingCancel()
		startTypingIndicator(typingCtx, h.client, chatID, h.logger)
	}

	chunkCh, streamErr := h.stream.QueryStream(streamCtx, streamText, nil)
	if streamErr != nil {
		h.logger.Error("stream open failed", zap.Error(streamErr), zap.Int64("chat_id", chatID))
		errMsg := "스트리밍 요청을 시작할 수 없습니다. 잠시 후 다시 시도해 주세요."
		if sent, sendErr := h.client.SendMessage(ctx, SendMessageRequest{
			ChatID: chatID,
			Text:   errMsg,
			Silent: h.cfg.SilentDefault,
		}); sendErr == nil {
			_ = h.audit.RecordOutbound(ctx, chatID, int64(sent.ID), errMsg, map[string]any{
				"stream_error": true,
			})
		}
		_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
		return nil
	}

	result, runErr := runStreaming(streamCtx, h.client, chatID, chunkCh, h.cfg.SilentDefault, h.logger)
	outboundMeta := map[string]any{
		"streaming":         true,
		"edit_count":        result.EditCount,
		"total_duration_ms": result.TotalDuration.Milliseconds(),
	}
	if result.Aborted {
		outboundMeta["streaming_aborted"] = true
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
		outboundMeta["stream_error"] = runErr.Error()
	}
	_ = h.audit.RecordOutbound(ctx, chatID, int64(result.PlaceholderID), result.Final, outboundMeta)
	_ = h.store.PutLastOffset(ctx, int64(update.UpdateID)+1)
	return nil
}
