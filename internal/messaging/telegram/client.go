package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot/models"
)

// User represents a Telegram bot or user account.
type User struct {
	ID        int64
	Username  string
	IsBot     bool
	FirstName string
}

// SendMessageRequest holds the parameters for sending a message.
type SendMessageRequest struct {
	ChatID int64
	Text   string
	// Silent sets disable_notification=true on the Telegram API call, suppressing
	// phone notifications for the recipient (REQ-MTGM-O01).
	Silent bool
}

// Message represents a sent Telegram message.
type Message struct {
	ID     int
	ChatID int64
	Text   string
	Date   int64
}

// InboundMessage represents an inbound message from a Telegram user.
type InboundMessage struct {
	ID         int
	ChatID     int64
	Text       string
	FromUserID int64
	Date       int64
}

// CallbackQuery represents an inline keyboard button click event.
// The ID must be acknowledged via answerCallbackQuery within 60 seconds.
type CallbackQuery struct {
	// ID is the callback query identifier used to acknowledge the click.
	ID string
	// FromUserID is the Telegram user ID of the person who clicked the button.
	FromUserID int64
	// ChatID is extracted from CallbackQuery.Message.Chat.ID (private chat).
	ChatID int64
	// MessageID is the message that contained the inline keyboard.
	MessageID int
	// Data is the callback_data string set when the button was created (≤ 64 bytes).
	Data string
	// ReceivedAt records when the update was received for timeout checking.
	ReceivedAt time.Time
}

// Update represents a Telegram update (inbound event).
type Update struct {
	UpdateID      int
	Message       *InboundMessage
	CallbackQuery *CallbackQuery // non-nil when an inline keyboard button was clicked
}

// SendMediaRequest holds the parameters for sending a photo or document.
type SendMediaRequest struct {
	// ChatID is the target chat identifier.
	ChatID int64
	// Caption is the optional text caption displayed alongside the media.
	Caption string
	// Path is the local filesystem path to the file to upload.
	// Mutually exclusive with URL.
	Path string
	// URL is the remote URL of the file to send.
	// Mutually exclusive with Path.
	URL string
	// Silent sets disable_notification=true on the Telegram API call, suppressing
	// phone notifications for the recipient (REQ-MTGM-O01).
	Silent bool
}

// ChatActionTyping is the Telegram chat action that shows "Bot is typing..."
// to the recipient. Emitted by startTypingIndicator while the agent processes
// a request (REQ-MTGM-O02).
const ChatActionTyping = "typing"

// EditMessageTextRequest holds parameters for editing the text of an existing message.
// Used by the streaming handler (P4) to update a placeholder message with accumulated
// response chunks (REQ-MTGM-E02).
type EditMessageTextRequest struct {
	// ChatID is the chat that owns the message.
	ChatID int64
	// MessageID is the identifier of the message to edit.
	MessageID int
	// Text is the new message text.
	Text string
}

// SetWebhookRequest holds parameters for the setWebhook Telegram API call.
type SetWebhookRequest struct {
	// URL is the HTTPS endpoint Telegram will POST updates to.
	URL string
	// SecretToken is the X-Telegram-Bot-Api-Secret-Token header value.
	// Telegram echoes this header on every POST so the server can verify origin.
	SecretToken string
	// AllowedUpdates is an optional list of update types to receive
	// (e.g. ["message", "callback_query"]). Empty means all default types.
	AllowedUpdates []string
}

// Client defines the narrow interface for Telegram Bot API operations needed by P1-P4.
//
// @MX:ANCHOR: [AUTO] Client is the primary abstraction for all Telegram API calls.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; fan_in via poller, handler, bootstrap, setup command, and tests.
type Client interface {
	// GetMe returns the bot's own User object. Used during setup to validate the token.
	GetMe(ctx context.Context) (User, error)

	// SendMessage sends a text message to the specified chat.
	SendMessage(ctx context.Context, req SendMessageRequest) (Message, error)

	// GetUpdates fetches pending updates starting from offset using long polling.
	// timeoutSec is the Telegram API long-poll timeout in seconds.
	GetUpdates(ctx context.Context, offset int, timeoutSec int) ([]Update, error)

	// AnswerCallbackQuery sends a response to a callback query (inline keyboard click).
	// Must be called within 60 seconds of the callback event (REQ-MTGM-N04).
	AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error

	// SendPhoto uploads and sends an image to the specified chat.
	SendPhoto(ctx context.Context, req SendMediaRequest) (Message, error)

	// SendDocument uploads and sends a document to the specified chat.
	SendDocument(ctx context.Context, req SendMediaRequest) (Message, error)

	// EditMessageText replaces the text of an existing message. Used by the streaming
	// handler to update the placeholder message with accumulated response chunks (P4).
	EditMessageText(ctx context.Context, req EditMessageTextRequest) (Message, error)

	// SetWebhook registers an HTTPS endpoint with the Telegram API to receive
	// updates via webhook (REQ-MTGM-E07). Returns an error if the endpoint
	// URL is invalid or not reachable by Telegram.
	SetWebhook(ctx context.Context, req SetWebhookRequest) error

	// DeleteWebhook removes the bot's webhook registration, reverting to
	// getUpdates-based long polling (REQ-MTGM-E07). When dropPending is true,
	// Telegram drops all pending updates that arrived while the webhook was set.
	DeleteWebhook(ctx context.Context, dropPending bool) error

	// SendChatAction sends a chat action (e.g. ChatActionTyping) to indicate that
	// the bot is preparing a response. Telegram displays the indicator for
	// approximately 5 seconds. Re-emit every 5 seconds to keep it visible
	// (REQ-MTGM-O02).
	SendChatAction(ctx context.Context, chatID int64, action string) error
}

// Option configures a Client instance.
type Option func(*httpClient)

// WithServerURL overrides the Telegram API base URL. Used in tests to point at
// an httptest server.
func WithServerURL(url string) Option {
	return func(c *httpClient) {
		c.serverURL = url
	}
}

// httpClient is the production implementation of Client backed by direct HTTP calls
// to the Telegram Bot API. It uses the go-telegram/bot model types for JSON decoding.
type httpClient struct {
	token     string
	serverURL string // empty means default Telegram API URL
}

// baseURL returns the effective API base URL.
func (c *httpClient) baseURL() string {
	if c.serverURL != "" {
		return c.serverURL
	}
	return "https://api.telegram.org"
}

// NewClient constructs a Client that talks to the Telegram Bot API.
//
// @MX:ANCHOR: [AUTO] NewClient is the primary constructor for the Telegram client.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 P1; fan_in via bootstrap, setup command, and tests.
func NewClient(token string, opts ...Option) (Client, error) {
	if token == "" {
		return nil, fmt.Errorf("telegram: empty bot token")
	}

	c := &httpClient{token: token}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// GetMe returns the bot's identity from the Telegram API.
func (c *httpClient) GetMe(ctx context.Context) (User, error) {
	var u models.User
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "getMe", nil, &u); err != nil {
		return User{}, fmt.Errorf("telegram: getMe: %w", err)
	}
	return modelUser(&u), nil
}

// SendMessage sends a text message and returns the resulting Message.
func (c *httpClient) SendMessage(ctx context.Context, req SendMessageRequest) (Message, error) {
	params := map[string]interface{}{
		"chat_id": req.ChatID,
		"text":    req.Text,
	}
	if req.Silent {
		params["disable_notification"] = true
	}
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendMessage", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: sendMessage: %w", err)
	}
	return modelMessage(&m), nil
}

// GetUpdates fetches updates from the Telegram API starting at offset.
// It performs a single request (not a loop) — the Poller drives the loop.
func (c *httpClient) GetUpdates(ctx context.Context, offset int, timeoutSec int) ([]Update, error) {
	raw, err := httpClientGetUpdates(ctx, c.baseURL(), c.token, offset, timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("telegram: getUpdates: %w", err)
	}
	return convertUpdates(raw), nil
}

// AnswerCallbackQuery sends an empty acknowledgement for an inline keyboard callback.
func (c *httpClient) AnswerCallbackQuery(ctx context.Context, callbackQueryID string) error {
	params := map[string]interface{}{
		"callback_query_id": callbackQueryID,
	}
	var result bool
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "answerCallbackQuery", params, &result); err != nil {
		return fmt.Errorf("telegram: answerCallbackQuery: %w", err)
	}
	return nil
}

// SendPhoto uploads an image file and sends it to the chat.
func (c *httpClient) SendPhoto(ctx context.Context, req SendMediaRequest) (Message, error) {
	params := map[string]interface{}{
		"chat_id": req.ChatID,
	}
	if req.Caption != "" {
		params["caption"] = req.Caption
	}
	if req.URL != "" {
		params["photo"] = req.URL
	} else {
		params["photo"] = "attach://" + req.Path
	}
	if req.Silent {
		params["disable_notification"] = true
	}
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendPhoto", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: sendPhoto: %w", err)
	}
	return modelMessage(&m), nil
}

// EditMessageText edits the text of an existing message.
// This is used by the streaming handler to update the placeholder message with
// accumulated response text (P4, REQ-MTGM-E02).
func (c *httpClient) EditMessageText(ctx context.Context, req EditMessageTextRequest) (Message, error) {
	params := map[string]interface{}{
		"chat_id":    req.ChatID,
		"message_id": req.MessageID,
		"text":       req.Text,
	}
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "editMessageText", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: editMessageText: %w", err)
	}
	return modelMessage(&m), nil
}

// SetWebhook registers an HTTPS URL with the Telegram API to receive updates.
// Telegram will POST each update to the supplied URL using the secret token as
// the X-Telegram-Bot-Api-Secret-Token header for origin verification.
func (c *httpClient) SetWebhook(ctx context.Context, req SetWebhookRequest) error {
	params := map[string]interface{}{
		"url": req.URL,
	}
	if req.SecretToken != "" {
		params["secret_token"] = req.SecretToken
	}
	if len(req.AllowedUpdates) > 0 {
		params["allowed_updates"] = req.AllowedUpdates
	}
	var result bool
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "setWebhook", params, &result); err != nil {
		return fmt.Errorf("telegram: setWebhook: %w", err)
	}
	return nil
}

// DeleteWebhook removes the bot's webhook registration. When dropPending is
// true, all queued updates that arrived while the webhook was active are
// discarded (REQ-MTGM-E07).
func (c *httpClient) DeleteWebhook(ctx context.Context, dropPending bool) error {
	params := map[string]interface{}{
		"drop_pending_updates": dropPending,
	}
	var result bool
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "deleteWebhook", params, &result); err != nil {
		return fmt.Errorf("telegram: deleteWebhook: %w", err)
	}
	return nil
}

// SendDocument uploads a file and sends it as a document to the chat.
func (c *httpClient) SendDocument(ctx context.Context, req SendMediaRequest) (Message, error) {
	params := map[string]interface{}{
		"chat_id": req.ChatID,
	}
	if req.Caption != "" {
		params["caption"] = req.Caption
	}
	if req.URL != "" {
		params["document"] = req.URL
	} else {
		params["document"] = "attach://" + req.Path
	}
	if req.Silent {
		params["disable_notification"] = true
	}
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendDocument", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: sendDocument: %w", err)
	}
	return modelMessage(&m), nil
}

// SendChatAction sends a chat action (e.g. "typing") to indicate the bot is
// preparing a response. Telegram displays the indicator for approximately
// 5 seconds (REQ-MTGM-O02).
func (c *httpClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	params := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	var result bool
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendChatAction", params, &result); err != nil {
		return fmt.Errorf("telegram: sendChatAction: %w", err)
	}
	return nil
}

// modelUser converts a models.User to our internal User.
func modelUser(u *models.User) User {
	if u == nil {
		return User{}
	}
	return User{
		ID:        u.ID,
		Username:  u.Username,
		IsBot:     u.IsBot,
		FirstName: u.FirstName,
	}
}

// modelMessage converts a models.Message to our internal Message.
func modelMessage(m *models.Message) Message {
	if m == nil {
		return Message{}
	}
	return Message{
		ID:     m.ID,
		ChatID: m.Chat.ID,
		Text:   m.Text,
		Date:   int64(m.Date),
	}
}

// convertUpdates maps raw tgbot updates to internal Update values.
func convertUpdates(raw []*models.Update) []Update {
	out := make([]Update, 0, len(raw))
	for _, u := range raw {
		upd := Update{UpdateID: int(u.ID)}
		if u.Message != nil {
			fromID := int64(0)
			if u.Message.From != nil {
				fromID = u.Message.From.ID
			}
			upd.Message = &InboundMessage{
				ID:         u.Message.ID,
				ChatID:     u.Message.Chat.ID,
				Text:       u.Message.Text,
				FromUserID: fromID,
				Date:       int64(u.Message.Date),
			}
		}
		if u.CallbackQuery != nil {
			cq := u.CallbackQuery
			fromID := cq.From.ID // From is a value type (models.User), always present
			chatID := fromID
			msgID := 0
			// Extract chat_id and message_id from the associated message (if accessible).
			if cq.Message.Type == models.MaybeInaccessibleMessageTypeMessage && cq.Message.Message != nil {
				chatID = cq.Message.Message.Chat.ID
				msgID = cq.Message.Message.ID
			}
			upd.CallbackQuery = &CallbackQuery{
				ID:         cq.ID,
				FromUserID: fromID,
				ChatID:     chatID,
				MessageID:  msgID,
				Data:       cq.Data,
				ReceivedAt: time.Now().UTC(),
			}
		}
		out = append(out, upd)
	}
	return out
}
