package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// AgentQuery is the narrow interface for sending a text message to the Goose
// agent and receiving a text response.
//
// The concrete implementation (AgentQueryAdapter) wraps the gRPC AgentServiceClient.Chat
// method. The narrow interface keeps BridgeQueryHandler independent of the gRPC
// package and makes it trivially mockable in tests.
//
// @MX:ANCHOR: [AUTO] AgentQuery decouples BridgeQueryHandler from the gRPC transport.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via BridgeQueryHandler, bootstrap, tests (>= 3 callers).
type AgentQuery interface {
	Query(ctx context.Context, text string) (string, error)
}

// maxMessageLength is the maximum number of UTF-8 bytes accepted per inbound
// Telegram message (REQ-MTGM-E04).
const maxMessageLength = 4096

// BridgeQueryHandler dispatches inbound Telegram messages to the Goose agent,
// enforces the first-message access gate, records audit events, and persists
// the chat_id mapping and polling offset.
//
// @MX:WARN: [AUTO] BridgeQueryHandler.Handle contains >= 8 conditional branches
// for access control, length gate, agent timeout, and error handling.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 REQ-MTGM-S01/S04/N05/E04 require multiple
// independent guards; complexity is inherent to the security model.
type BridgeQueryHandler struct {
	client Client
	store  Store
	audit  *AuditWrapper
	agent  AgentQuery
	cfg    *Config
	logger *zap.Logger
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
		client: client,
		store:  store,
		audit:  audit,
		agent:  agent,
		cfg:    cfg,
		logger: logger,
	}
}

// Handle processes a single Telegram Update.
//
// Flow summary:
//  1. Skip updates with no message text; advance offset.
//  2. Reject over-length messages (> 4096 bytes).
//  3. Enforce first-message gate or auto-admit.
//  4. Drop silently if user is blocked.
//  5. Query the agent with a 30-second timeout.
//  6. Deliver response; advance offset.
func (h *BridgeQueryHandler) Handle(ctx context.Context, update Update) error {
	// 1. Skip empty or no-message updates.
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

	// Audit the inbound message after access control is resolved.
	inboundMeta := map[string]any{}
	if mapping.AutoAdmitted {
		inboundMeta["auto_admitted"] = true
	}
	_ = h.audit.RecordInbound(ctx, chatID, msgID, text, inboundMeta)

	// 5. Query agent with timeout.
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, queryErr := h.agent.Query(queryCtx, text)
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
