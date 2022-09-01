package markdown

import (
	"io"
	"sync"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/tg"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	bot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type markdown struct {
	renderOptions
	goldmark.Markdown
}

var (
	markdownPool = sync.Pool{
		New: func() any {
			fmt := &markdown{}
			fmt.Markdown = goldmark.New(
				// defaults
				goldmark.WithParser(
					DefaultParser,
				),
				goldmark.WithRendererOptions(
					// defaults
					renderer.WithNodeRenderers(
						util.Prioritized(NewRenderer(), 100),
					),
					// buildin: RenderOptions container
					&fmt.renderOptions,
				),
			)
			return fmt
		},
	}
)

// ParseText into `fmt` *entity.Builder
func ParseText(text string, fmt *entity.Builder) error {
	markdown := markdownPool.Get().(*markdown)
	// defer markdown.release()
	// Bind output *entity.Builder
	markdown.Builder = fmt
	defer func() {
		// Unbind *entity.Builder
		markdown.Builder = nil
		// Release Markdown
		markdownPool.Put(markdown)
	}()

	// Build output message with entities
	err := markdown.Convert(
		[]byte(text), io.Discard,
	)
	// Format error ?
	if err != nil {
		// Rewrite as typical plain text !
		fmt.Reset()
		fmt.Plain(Unescape(text))
		return nil
	}

	return nil
}

// FormatText `message` for MTProto (telegram-app) implementation
func FormatText(message string) message.StyledTextOption {
	return styling.Custom(func(fmt *entity.Builder) error {
		return ParseText(message, fmt)
	})
}

// TextEntities parses `text` for HTTP (telegram-bot) implementation
func TextEntities(text string) (string, []bot.MessageEntity) {
	var (
		fmt entity.Builder
	)
	_ = ParseText(text, &fmt)

	text, entries := fmt.Raw()
	n := len(entries)
	if n == 0 {
		return text, nil
	}
	entities := make([]bot.MessageEntity, 0, n)
	for _, entity := range entries {
		switch e := entity.(type) {
		case *tg.MessageEntityUnknown: // messageEntityUnknown#bb92ba95
		case *tg.MessageEntityMention: // messageEntityMention#fa04579d
		case *tg.MessageEntityHashtag: // messageEntityHashtag#6f635b0d
		case *tg.MessageEntityBotCommand: // messageEntityBotCommand#6cef8ac7
		case *tg.MessageEntityURL: // messageEntityUrl#6ed02538
		case *tg.MessageEntityEmail: // messageEntityEmail#64e475c2
		case *tg.MessageEntityBold: // messageEntityBold#bd610bc9
			entities = append(entities,
				bot.MessageEntity{
					Type:   "bold",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityItalic: // messageEntityItalic#826f8b60
			entities = append(entities,
				bot.MessageEntity{
					Type:   "italic",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityCode: // messageEntityCode#28a20571
			entities = append(entities,
				bot.MessageEntity{
					Type:   "code",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityPre: // messageEntityPre#73924be0
			entities = append(entities,
				bot.MessageEntity{
					Type:     "pre",
					Offset:   e.Offset,
					Length:   e.Length,
					Language: e.Language,
				},
			)
		case *tg.MessageEntityTextURL: // messageEntityTextUrl#76a6d327
			entities = append(entities,
				bot.MessageEntity{
					Type:   "text_link",
					Offset: e.Offset,
					Length: e.Length,
					URL:    e.URL,
				},
			)
		case *tg.MessageEntityMentionName: // messageEntityMentionName#dc7b1140
		case *tg.InputMessageEntityMentionName: // inputMessageEntityMentionName#208e68c9
		case *tg.MessageEntityPhone: // messageEntityPhone#9b69e34b
		case *tg.MessageEntityCashtag: // messageEntityCashtag#4c4e743f
		case *tg.MessageEntityUnderline: // messageEntityUnderline#9c4e7e8b
			entities = append(entities,
				bot.MessageEntity{
					Type:   "underline",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityStrike: // messageEntityStrike#bf0693d4
			entities = append(entities,
				bot.MessageEntity{
					Type:   "strikethrough",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityBlockquote: // messageEntityBlockquote#20df5d0
		case *tg.MessageEntityBankCard: // messageEntityBankCard#761e6af4
		case *tg.MessageEntitySpoiler: // messageEntitySpoiler#32ca960f
			entities = append(entities,
				bot.MessageEntity{
					Type:   "spoiler",
					Offset: e.Offset,
					Length: e.Length,
				},
			)
		case *tg.MessageEntityCustomEmoji: // messageEntityCustomEmoji#c8cf05f8
		default:
		}
	}

	return text, entities
}
