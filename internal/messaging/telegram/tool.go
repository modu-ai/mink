package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modu-ai/goose/internal/tools"
)

// toolSender is the narrow interface that telegramSendMessageTool depends on.
// Using this interface instead of *Sender keeps the tool testable with mocks.
type toolSender interface {
	Send(ctx context.Context, req SendRequest) (*SendResponse, error)
}

// telegramSendMessageSchema is the JSON Schema (draft 2020-12) for the
// telegram_send_message tool input. Matches strategy-p3.md §B.4.
//
// chat_id  — integer or username string (@username pattern)
// text     — 1-4096 characters
// parse_mode — MarkdownV2 | HTML | Plain
// reply_to_message_id — positive integer (optional)
// inline_keyboard — 1-row array of button objects (optional)
// attachments — array of {type, path|url} (optional)
// silent — disable_notification flag (optional)
var telegramSendMessageSchema = json.RawMessage(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "required": ["chat_id", "text"],
  "properties": {
    "chat_id": {
      "oneOf": [
        {"type": "integer"},
        {"type": "string"}
      ]
    },
    "text": {"type": "string", "minLength": 0, "maxLength": 4096},
    "parse_mode": {
      "type": "string",
      "enum": ["MarkdownV2", "HTML", "Plain"],
      "default": "MarkdownV2"
    },
    "reply_to_message_id": {"type": "integer", "minimum": 1},
    "inline_keyboard": {
      "type": "array",
      "maxItems": 1,
      "items": {
        "type": "array",
        "maxItems": 8,
        "items": {
          "type": "object",
          "additionalProperties": false,
          "required": ["text", "callback_data"],
          "properties": {
            "text": {"type": "string", "minLength": 1, "maxLength": 64},
            "callback_data": {"type": "string", "minLength": 1, "maxLength": 64}
          }
        }
      }
    },
    "attachments": {
      "type": "array",
      "maxItems": 10,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["type"],
        "properties": {
          "type": {"type": "string", "enum": ["image", "document"]},
          "path": {"type": "string"},
          "url":  {"type": "string"}
        }
      }
    },
    "silent": {"type": "boolean", "default": false}
  }
}`)

// telegramSendMessageInput is the decoded form of a telegram_send_message call.
type telegramSendMessageInput struct {
	ChatID           json.RawMessage   `json:"chat_id"`
	Text             string            `json:"text"`
	ParseMode        string            `json:"parse_mode"`
	ReplyToMessageID int               `json:"reply_to_message_id"`
	InlineKeyboard   [][]InlineButton  `json:"inline_keyboard"`
	Attachments      []attachmentInput `json:"attachments"`
	Silent           bool              `json:"silent"`
}

type attachmentInput struct {
	Type string `json:"type"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

// telegramSendMessageTool implements tools.Tool for the telegram_send_message
// tool. It wraps a Sender and translates JSON input to a SendRequest.
//
// @MX:NOTE: [AUTO] telegramSendMessageTool bridges the TOOLS-001 registry
// to the telegram Sender. The tool enforces the allowed_users gate via Sender.Send
// (REQ-MTGM-N02). Permission modal (CLI-TUI-002) is deferred to P4.
type telegramSendMessageTool struct {
	sender toolSender
}

func (t *telegramSendMessageTool) Name() string {
	return "telegram_send_message"
}

func (t *telegramSendMessageTool) Schema() json.RawMessage {
	return telegramSendMessageSchema
}

func (t *telegramSendMessageTool) Scope() tools.Scope {
	return tools.ScopeShared
}

// Call deserialises the JSON input and dispatches to Sender.Send.
// ErrUnauthorizedChatID is returned as a tool error (IsError=true) rather than
// a hard error so the LLM can observe and handle the restriction.
func (t *telegramSendMessageTool) Call(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var in telegramSendMessageInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.ToolResult{
			Content: []byte(fmt.Sprintf("telegram_send_message: invalid input: %v", err)),
			IsError: true,
		}, nil
	}

	// Decode chat_id — accept integer or numeric string.
	chatID, err := decodeChatID(in.ChatID)
	if err != nil {
		return tools.ToolResult{
			Content: []byte(fmt.Sprintf("telegram_send_message: invalid chat_id: %v", err)),
			IsError: true,
		}, nil
	}

	// Map parse_mode string to ParseMode type.
	var pm ParseMode
	switch in.ParseMode {
	case "MarkdownV2":
		pm = ParseModeMarkdownV2
	case "HTML":
		pm = ParseModeHTML
	default:
		pm = ParseModePlain
	}

	// Convert attachment inputs.
	attachments := make([]Attachment, 0, len(in.Attachments))
	for _, a := range in.Attachments {
		at := AttachmentTypeDocument
		if a.Type == "image" {
			at = AttachmentTypeImage
		}
		attachments = append(attachments, Attachment{
			Type: at,
			Path: a.Path,
			URL:  a.URL,
		})
	}

	req := SendRequest{
		ChatID:           chatID,
		Text:             in.Text,
		ParseMode:        pm,
		ReplyToMessageID: in.ReplyToMessageID,
		InlineKeyboard:   in.InlineKeyboard,
		Attachments:      attachments,
		Silent:           in.Silent,
	}

	resp, err := t.sender.Send(ctx, req)
	if err != nil {
		if errors.Is(err, ErrUnauthorizedChatID) {
			return tools.ToolResult{
				Content: []byte("telegram_send_message: error: unauthorized_chat_id"),
				IsError: true,
			}, nil
		}
		return tools.ToolResult{
			Content: []byte(fmt.Sprintf("telegram_send_message: send failed: %v", err)),
			IsError: true,
		}, nil
	}

	out, _ := json.Marshal(map[string]any{
		"message_id": resp.MessageID,
		"chat_id":    resp.ChatID,
	})
	return tools.ToolResult{Content: out}, nil
}

// decodeChatID parses a chat_id from JSON which may be an integer or a string.
func decodeChatID(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("chat_id is required")
	}
	// Try integer first.
	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}
	// Try string (numeric or @username — we pass through as numeric only).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var parsed int64
		if _, scanErr := fmt.Sscanf(s, "%d", &parsed); scanErr == nil {
			return parsed, nil
		}
		// Non-numeric username strings cannot be resolved to int64 without
		// an API call; return 0 and let the Sender handle the error.
		return 0, fmt.Errorf("chat_id string %q is not a numeric id", s)
	}
	return 0, fmt.Errorf("chat_id must be an integer or numeric string")
}

// WithMessaging returns a tools.Option that registers the telegram_send_message
// tool backed by the given Sender into the provided Registry.
//
// Unlike built-in tools, messaging tools require a live Sender instance, so they
// cannot be registered via init(). This function is called at daemon boot after
// the Sender is fully initialised (strategy-p3.md §B.2 option i).
//
// @MX:ANCHOR: [AUTO] WithMessaging is the runtime entry point for telegram tool registration.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001; fan_in via cmd/goosed/main.go, bootstrap, and tests (>= 3 callers).
func WithMessaging(sender *Sender) tools.Option {
	return func(r *tools.Registry) {
		tool := &telegramSendMessageTool{sender: sender}
		if err := r.Register(tool, tools.SourceBuiltin); err != nil {
			panic(fmt.Sprintf("telegram tool registration failed for %q: %v", tool.Name(), err))
		}
	}
}
