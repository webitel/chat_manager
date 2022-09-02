package gotd

import (
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
)

// TODO: Try to parse and detect styled mode
// https://core.telegram.org/bots/api#formatting-options
func FormatText(text string) message.StyledTextOption {
	return styling.Custom(func(out *entity.Builder) error {
		// TODO: Support
		// - HTML
		// - Markdown
		// - MarkdownV2
		out.Plain(text)
		return nil
	})
}
