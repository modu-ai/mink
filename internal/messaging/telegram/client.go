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
}

// Client defines the narrow interface for Telegram Bot API operations needed by P1-P3.
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
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendPhoto", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: sendPhoto: %w", err)
	}
	return modelMessage(&m), nil
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
	var m models.Message
	if err := httpPostJSON(ctx, c.baseURL(), c.token, "sendDocument", params, &m); err != nil {
		return Message{}, fmt.Errorf("telegram: sendDocument: %w", err)
	}
	return modelMessage(&m), nil
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
