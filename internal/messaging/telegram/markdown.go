package telegram

import (
	"encoding/json"
	"strings"
)

// reservedV2 contains all 18 MarkdownV2 reserved characters as defined in
// Telegram Bot API §MarkdownV2 style.
// Characters: _ * [ ] ( ) ~ ` > # + - = | { } . !
const reservedV2 = `_*[]()~` + "`" + `>#+-=|{}.!`

// EscapeV2 escapes all 18 Telegram MarkdownV2 reserved characters in text
// by prepending each with a backslash.
//
// Callers must apply EscapeV2 to every user-supplied or agent-generated string
// before passing it to sendMessage with parse_mode=MarkdownV2.
//
// @MX:NOTE: [AUTO] EscapeV2 covers all 18 reserved chars from Telegram Bot API
// MarkdownV2 spec: _ * [ ] ( ) ~ ` > # + - = | { } . !
// Not idempotent — applying twice escapes the backslashes inserted by the
// first pass. Apply exactly once to each untrusted text fragment.
func EscapeV2(text string) string {
	var b strings.Builder
	b.Grow(len(text) + len(text)/4)
	for _, r := range text {
		if strings.ContainsRune(reservedV2, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// InlineButton represents a single button in a Telegram inline keyboard.
// CallbackData must be ≤ 64 bytes (Telegram Bot API constraint).
type InlineButton struct {
	// Text is the button label shown to the user.
	Text string `json:"text"`
	// CallbackData is the payload sent back when the user presses the button.
	CallbackData string `json:"callback_data"`
}

// RenderInlineKeyboard serialises a keyboard layout to its Telegram API JSON
// representation (a JSON array of arrays of button objects).
//
// Per spec.md §3.1 Area 3, only 1-row keyboards are supported (maxItems=1
// outer array). The function accepts [][]InlineButton for forward-compatibility
// but callers should pass a single-row slice.
//
// Returns "[]" for a nil or empty rows argument.
//
// @MX:NOTE: [AUTO] RenderInlineKeyboard produces the JSON shape expected by
// Telegram's inline_keyboard parameter. Only 1-row keyboards are in-scope
// per SPEC-GOOSE-MSG-TELEGRAM-001 REQ-MTGM-E03.
func RenderInlineKeyboard(rows [][]InlineButton) string {
	if len(rows) == 0 {
		return "[]"
	}
	data, err := json.Marshal(rows)
	if err != nil {
		// json.Marshal on [][]InlineButton can only fail if a field contains
		// an un-serialisable type, which is impossible for string fields.
		return "[]"
	}
	return string(data)
}
