package telegram

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// ErrUnauthorizedChatID is returned when the target chat_id is not in the
// allowed_users list (REQ-MTGM-N02).
var ErrUnauthorizedChatID = errors.New("telegram: unauthorized chat_id")

// ParseMode specifies the formatting mode for outbound messages.
type ParseMode string

const (
	// ParseModeMarkdownV2 activates Telegram MarkdownV2 rendering.
	// All 18 reserved chars must be escaped in the message text.
	ParseModeMarkdownV2 ParseMode = "MarkdownV2"
	// ParseModeHTML activates HTML rendering.
	ParseModeHTML ParseMode = "HTML"
	// ParseModePlain sends the message as plain text (no special rendering).
	ParseModePlain ParseMode = ""
)

// AttachmentType classifies an outbound attachment.
type AttachmentType string

const (
	// AttachmentTypeImage sends the file as a photo (sendPhoto API).
	AttachmentTypeImage AttachmentType = "image"
	// AttachmentTypeDocument sends the file as a document (sendDocument API).
	AttachmentTypeDocument AttachmentType = "document"
)

// Attachment describes a file to be sent alongside the message text.
type Attachment struct {
	// Type determines whether sendPhoto or sendDocument is used.
	Type AttachmentType
	// Path is the local filesystem path to the file (mutually exclusive with URL).
	Path string
	// URL is the remote file URL (mutually exclusive with Path).
	URL string
	// MimeType is an optional hint for the receiver (e.g. "image/jpeg").
	MimeType string
	// SizeBytes is the optional file size hint.
	SizeBytes int64
}

// SendRequest holds the parameters for an outbound Telegram message.
type SendRequest struct {
	// ChatID is the target Telegram chat identifier.
	ChatID int64
	// Text is the message body. Must not exceed 4096 characters.
	Text string
	// ParseMode controls MarkdownV2 / HTML / plain rendering.
	// Defaults to plain if not set.
	ParseMode ParseMode
	// ReplyToMessageID optionally specifies a message to reply to.
	ReplyToMessageID int
	// InlineKeyboard is an optional 1-row inline keyboard layout.
	InlineKeyboard [][]InlineButton
	// Attachments holds optional files to send (images or documents).
	Attachments []Attachment
	// Silent sets disable_notification on the Telegram API call.
	Silent bool
}

// SendResponse is returned by Sender.Send on success.
type SendResponse struct {
	// MessageID is the Telegram message_id assigned by the server.
	MessageID int
	// ChatID echoes back the target chat_id.
	ChatID int64
}

// Sender delivers outbound messages to Telegram users via the Bot API.
//
// Sender enforces the allowed_users gate (REQ-MTGM-N02), applies MarkdownV2
// escaping when requested, and branches between sendMessage/sendPhoto/sendDocument
// depending on the attachment type.
//
// @MX:ANCHOR: [AUTO] Sender.Send is the single outbound gateway for the telegram channel.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via tool.go Call, integration tests,
// bootstrap wiring, and BridgeQueryHandler response path (>= 3 callers).
type Sender struct {
	client Client
	store  Store
	audit  *AuditWrapper
	logger *zap.Logger
}

// NewSender constructs a Sender with the given dependencies.
func NewSender(client Client, store Store, audit *AuditWrapper, logger *zap.Logger) *Sender {
	return &Sender{
		client: client,
		store:  store,
		audit:  audit,
		logger: logger,
	}
}

// Send delivers a message to the specified chat_id.
//
// The method validates that the chat_id is in the allowed_users list before
// invoking any Telegram API call (REQ-MTGM-N02). When ParseMode is MarkdownV2,
// the text is escaped via EscapeV2 before transmission. The first attachment
// in the list determines whether sendPhoto or sendDocument is used; additional
// attachments beyond the first are ignored in this implementation.
//
// An outbound audit event is recorded on success (REQ-MTGM-U01).
func (s *Sender) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	// Gate: verify chat_id is allowed (REQ-MTGM-N02).
	mapping, found, err := s.store.GetUserMapping(ctx, req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("telegram sender: store lookup: %w", err)
	}
	if !found || !mapping.Allowed {
		return nil, ErrUnauthorizedChatID
	}

	// Apply MarkdownV2 escaping if requested.
	text := req.Text
	if req.ParseMode == ParseModeMarkdownV2 {
		text = EscapeV2(text)
	}

	// Dispatch: attachment type determines which API method to use.
	var msg Message
	if len(req.Attachments) > 0 {
		a := req.Attachments[0]
		switch a.Type {
		case AttachmentTypeImage:
			msg, err = s.client.SendPhoto(ctx, SendMediaRequest{
				ChatID:  req.ChatID,
				Caption: text,
				Path:    a.Path,
				URL:     a.URL,
			})
		default: // AttachmentTypeDocument and unknown types
			msg, err = s.client.SendDocument(ctx, SendMediaRequest{
				ChatID:  req.ChatID,
				Caption: text,
				Path:    a.Path,
				URL:     a.URL,
			})
		}
	} else {
		smr := SendMessageRequest{
			ChatID: req.ChatID,
			Text:   text,
		}
		msg, err = s.client.SendMessage(ctx, smr)
	}

	if err != nil {
		return nil, fmt.Errorf("telegram sender: send: %w", err)
	}

	// Record outbound audit event (REQ-MTGM-U01).
	if auditErr := s.audit.RecordOutbound(ctx, req.ChatID, int64(msg.ID), req.Text, nil); auditErr != nil {
		s.logger.Warn("outbound audit failed", zap.Error(auditErr), zap.Int64("chat_id", req.ChatID))
	}

	return &SendResponse{
		MessageID: msg.ID,
		ChatID:    msg.ChatID,
	}, nil
}
