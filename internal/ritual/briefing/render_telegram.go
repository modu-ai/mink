package briefing

import (
	"context"
	"fmt"
	"strings"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"go.uber.org/zap"
)

// TelegramSender is the minimal interface used by the briefing telegram
// channel. It is satisfied by *telegram.Sender and is defined here so that
// tests can substitute mock senders without taking a dependency on the full
// telegram package.
//
// @MX:NOTE: Interface segregation -- briefing only needs Send.
type TelegramSender interface {
	Send(ctx context.Context, req telegram.SendRequest) (*telegram.SendResponse, error)
}

// TelegramChannelConfig holds the per-invocation settings for the Telegram
// briefing channel.
//
// Token is checked for presence only -- the actual bot token is held inside
// the Sender's client. Token here is used as a sentinel for "is the channel
// configured at all?" and is intentionally never logged.
type TelegramChannelConfig struct {
	// Token is non-empty when MINK_TELEGRAM_TOKEN (or equivalent config)
	// is set. Used only as a presence check; the raw value is never logged
	// (REQ-BR-050).
	Token string

	// ChatID is the target chat. Zero (or invalid) disables the channel
	// for this invocation (REQ-BR-022 graceful disable / EC-004).
	ChatID int64
}

// RenderTelegram builds a telegram.SendRequest for the briefing payload.
// The text is rendered in MarkdownV2-style; Sender.Send is responsible for
// applying EscapeV2 when ParseMode == MarkdownV2.
//
// REQ-BR-002: telegram is one of 3 output channels.
func RenderTelegram(payload *BriefingPayload, chatID int64) telegram.SendRequest {
	return telegram.SendRequest{
		ChatID:    chatID,
		Text:      RenderTelegramText(payload),
		ParseMode: telegram.ParseModeMarkdownV2,
	}
}

// RenderTelegramText returns the MarkdownV2-style body for the briefing
// payload. The output is pre-escape; consumers that bypass Sender.Send
// must call telegram.EscapeV2 on the result before transmission.
func RenderTelegramText(payload *BriefingPayload) string {
	if payload == nil {
		return ""
	}
	var sb strings.Builder

	sb.WriteString("*🌅 MORNING BRIEFING*\n\n")

	// Weather
	sb.WriteString("*🌤️ Weather*\n")
	switch {
	case payload.Weather.Offline || payload.Status["weather"] == "offline":
		sb.WriteString("_offline (cached)_\n")
	case payload.Status["weather"] == "error":
		sb.WriteString("_error_\n")
	case payload.Status["weather"] == "timeout":
		sb.WriteString("_timeout_\n")
	default:
		if payload.Weather.Current != nil {
			c := payload.Weather.Current
			fmt.Fprintf(&sb, "- Temp: %.1f°C (feels %.1f°C)\n", c.Temp, c.FeelsLike)
			fmt.Fprintf(&sb, "- Cond: %s\n", c.Condition)
			if c.Location != "" {
				fmt.Fprintf(&sb, "- Loc: %s\n", c.Location)
			}
		}
		if payload.Weather.AirQuality != nil {
			fmt.Fprintf(&sb, "- AQI: %d (%s)\n", payload.Weather.AirQuality.AQI, payload.Weather.AirQuality.Level)
		}
	}
	sb.WriteString("\n")

	// Journal Recall
	sb.WriteString("*📔 Journal Recall*\n")
	switch {
	case payload.JournalRecall.Offline || payload.Status["journal"] == "offline":
		sb.WriteString("_journal unavailable_\n")
	case payload.Status["journal"] == "error":
		sb.WriteString("_error_\n")
	default:
		if len(payload.JournalRecall.Anniversaries) > 0 {
			for _, a := range payload.JournalRecall.Anniversaries {
				emoji := a.EmojiMood
				if emoji == "" {
					emoji = "📝"
				}
				fmt.Fprintf(&sb, "- %s %dY: %s\n", emoji, a.YearsAgo, a.Date)
			}
		} else {
			sb.WriteString("_no anniversaries today_\n")
		}
		if payload.JournalRecall.MoodTrend != nil {
			fmt.Fprintf(&sb, "- Mood: %s (%s)\n",
				payload.JournalRecall.MoodTrend.Trend,
				payload.JournalRecall.MoodTrend.Period)
		}
	}
	sb.WriteString("\n")

	// Date
	sb.WriteString("*📅 Date*\n")
	fmt.Fprintf(&sb, "- %s (%s)\n", payload.DateCalendar.Today, payload.DateCalendar.DayOfWeek)
	if payload.DateCalendar.SolarTerm != nil {
		fmt.Fprintf(&sb, "- Solar Term: %s (%s)\n",
			payload.DateCalendar.SolarTerm.Name, payload.DateCalendar.SolarTerm.NameHanja)
	}
	if payload.DateCalendar.Holiday != nil {
		fmt.Fprintf(&sb, "- Holiday: %s\n", payload.DateCalendar.Holiday.Name)
	}
	sb.WriteString("\n")

	// Mantra
	if payload.Mantra.Text != "" {
		sb.WriteString("*✨ Mantra*\n")
		fmt.Fprintf(&sb, "_%s_\n", payload.Mantra.Text)
	}

	// T-305: Prepend crisis hotline response when rendered text contains a
	// crisis keyword. REQ-BR-055 / REQ-BR-061 / AC-015.
	return PrependCrisisResponseIfDetected(sb.String())
}

// SendBriefingTelegram dispatches the briefing payload via the supplied
// Telegram sender. Returns the resulting status string ("ok" / "disabled" /
// "error") suitable for the BriefingPayload.Status["telegram"] field.
//
// REQ-BR-022 / EC-004: when the channel is not configured (nil sender,
// missing token, or zero chat_id) the function returns "disabled" with a
// warning log that intentionally does NOT include the chat_id raw value
// (REQ-BR-050).
//
// @MX:ANCHOR: SendBriefingTelegram is the single outbound gateway for the
// telegram briefing channel.
// @MX:REASON: SPEC-MINK-BRIEFING-001 REQ-BR-022, REQ-BR-050; consolidates
// graceful-disable + log redaction in one place.
func SendBriefingTelegram(ctx context.Context, sender TelegramSender, cfg TelegramChannelConfig, payload *BriefingPayload, logger *zap.Logger) (string, error) {
	if sender == nil || cfg.Token == "" || cfg.ChatID == 0 {
		if logger != nil {
			logger.Warn("telegram briefing channel disabled",
				zap.Bool("sender_nil", sender == nil),
				zap.Bool("token_empty", cfg.Token == ""),
				zap.Bool("chat_id_zero", cfg.ChatID == 0),
				// chat_id raw value intentionally NOT included (REQ-BR-050).
			)
		}
		return "disabled", nil
	}
	if payload == nil {
		return "error", fmt.Errorf("briefing telegram: nil payload")
	}

	req := RenderTelegram(payload, cfg.ChatID)
	if _, err := sender.Send(ctx, req); err != nil {
		if logger != nil {
			logger.Warn("telegram briefing send failed",
				zap.String("error_type", classifyError(err)),
				// raw error.Error() and chat_id intentionally not logged.
			)
		}
		return "error", err
	}
	return "ok", nil
}

// classifyError returns a coarse classification of a Sender error suitable
// for logging without leaking PII.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unauthorized"):
		return "unauthorized"
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "context"):
		return "context"
	default:
		return "other"
	}
}
