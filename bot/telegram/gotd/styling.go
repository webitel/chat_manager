package client

import (
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
)

// TODO: Try to parse and detect styled mode
func StyledText(text string) message.StyledTextOption {
	return styling.Custom(func(out *entity.Builder) error {
		// TODO:
		out.Plain(text)
		return nil
	})
}

// https://core.telegram.org/bots/api#markdownv2-style
func MarkdownV2(text string) message.StyledTextOption {
	return styling.Custom(func(out *entity.Builder) error {
		// TODO: parse MarkdownV2-styled message text
		// and populate parts to out entity.Builder
		out.Plain(text)
		return nil
	})
}

// https://core.telegram.org/bots/api#markdown-style
func Markdown(text string) message.StyledTextOption {
	return styling.Custom(func(out *entity.Builder) error {
		// TODO: parse Markdown-styled message text
		// and populate parts to out entity.Builder
		out.Plain(text)
		return nil
	})
}

// https://core.telegram.org/bots/api#html-style
func HTML(text string) message.StyledTextOption {
	return styling.Custom(func(out *entity.Builder) error {
		// TODO: parse HTML-styled message text
		// and populate parts to out entity.Builder
		out.Plain(text)
		return nil
	})
}
