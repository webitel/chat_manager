package builder

import (
	"errors"
	"strings"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot/telegram/internal/helper"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown"
)

// SendMessageBuilder is designed to create a message with text or a file for the Telegram environment
type SendMessageBuilder struct {
	chatID        int64
	chatAction    string
	baseChat      *telegram.BaseChat
	messageConfig telegram.Chattable
}

// NewSendMessageBuilder constructor for SendMessageBuilder struct
func NewSendMessageBuilder() *SendMessageBuilder {
	b := SendMessageBuilder{}
	return &b
}

// SetChatID setter for chat ID, it is required for all messages
func (b *SendMessageBuilder) SetChatID(chatID int64) error {
	if chatID < 0 {
		return errors.New("invalid chat ID")
	}

	b.chatID = chatID

	return nil
}

// SetText setter for text message
func (b *SendMessageBuilder) SetText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("text cannot be allowed in the message to be sent")
	}

	b.chatAction = telegram.ChatTyping

	text, entities := markdown.TextEntities(text)
	message := telegram.NewMessage(
		b.chatID, text,
	)
	message.Entities = entities
	message.ParseMode = telegram.ModeMarkdownV2
	b.messageConfig = &message
	b.baseChat = &message.BaseChat

	return nil
}

// SetFile setter for file message, like: image, audio, video and document
func (b *SendMessageBuilder) SetFile(filename, mimetype, url, caption string) error {
	switch getMediaType(mimetype) {
	case "image":
		b.chatAction = telegram.ChatUploadPhoto

		photo := telegram.NewPhoto(
			b.chatID, helper.NewSendFile(url, filename),
		)
		photo.Caption = caption
		b.messageConfig = &photo
		b.baseChat = &photo.BaseChat

	case "audio":
		b.chatAction = telegram.ChatUploadVoice

		audio := telegram.NewAudio(
			b.chatID, helper.NewSendFile(url, filename),
		)
		audio.Caption = caption
		b.messageConfig = &audio
		b.baseChat = &audio.BaseChat

	case "video":
		b.chatAction = telegram.ChatUploadVideo

		video := telegram.NewVideo(
			b.chatID, helper.NewSendFile(url, filename),
		)
		video.Caption = caption
		b.messageConfig = &video
		b.baseChat = &video.BaseChat

	default:
		b.chatAction = telegram.ChatUploadDocument

		document := telegram.NewDocument(
			b.chatID, helper.NewSendFile(url, filename),
		)
		document.Caption = caption
		b.messageConfig = &document
		b.baseChat = &document.BaseChat
	}

	return nil
}

// SetKeyboad setter for classic keyboard
func (b *SendMessageBuilder) SetKeyboad(buttons []*pbchat.Buttons) error {
	var keyboad []telegram.KeyboardButton
	var removeKeyboad bool

	for _, markup := range buttons {
		var row []telegram.KeyboardButton
		for _, button := range markup.Button {
			if btn, ok := getKeyboardButton(button); ok {
				row = append(row, btn)
				continue
			}

			row = append(row,
				telegram.NewKeyboardButton(button.Text),
			)
		}

		if len(row) > 0 {
			keyboad = append(keyboad, telegram.NewKeyboardButtonRow(row...)...)
		}
	}

	if removeKeyboad {
		b.SetRemoveKeyboard()
	} else if len(keyboad) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewOneTimeReplyKeyboard(keyboad)
	}

	return nil
}

// SetKeyboad setter for inline keyboard
func (b *SendMessageBuilder) SetInlineKeyboad(buttons []*pbchat.Buttons) error {
	var keyboad []telegram.InlineKeyboardButton

	for _, markup := range buttons {
		var row []telegram.InlineKeyboardButton
		for _, button := range markup.Button {
			if btn, ok := getInlineKeyboardButton(button); ok {
				row = append(row, btn)
				continue
			}

			row = append(row,
				telegram.NewInlineKeyboardButtonData(button.Text, button.Code),
			)
		}

		if len(row) > 0 {
			keyboad = append(keyboad, telegram.NewInlineKeyboardRow(row...)...)
		}
	}

	if len(keyboad) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewInlineKeyboardMarkup(keyboad)
	}

	return nil
}

// SetMargedKeyboad accepts both conventional and inline keyboards, but the classic keyboard will always be a priority
func (b *SendMessageBuilder) SetMargedKeyboad(buttons []*pbchat.Buttons) error {
	var buttonsKeyboad []telegram.KeyboardButton
	var inlineKeyboad []telegram.InlineKeyboardButton
	var removeKeyboad bool

	for _, markup := range buttons {
		var buttonsRow []telegram.KeyboardButton
		var inlineRow []telegram.InlineKeyboardButton
		for _, button := range markup.Button {
			if btn, ok := getKeyboardButton(button); ok {
				buttonsRow = append(buttonsRow, btn)
				continue
			}

			if btn, ok := getInlineKeyboardButton(button); ok {
				inlineRow = append(inlineRow, btn)
				continue
			}

			buttonsRow = append(buttonsRow,
				telegram.NewKeyboardButton(button.Text),
			)
		}

		if len(buttonsRow) > 0 {
			buttonsKeyboad = append(buttonsKeyboad, telegram.NewKeyboardButtonRow(buttonsRow...)...)
		}

		if len(inlineRow) > 0 {
			inlineKeyboad = append(inlineKeyboad, telegram.NewInlineKeyboardRow(inlineRow...)...)
		}
	}

	if removeKeyboad {
		b.SetRemoveKeyboard()
	} else if len(inlineKeyboad) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewInlineKeyboardMarkup(inlineKeyboad)
	} else if len(buttonsKeyboad) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewOneTimeReplyKeyboard(buttonsKeyboad)
	}

	return nil
}

// RemoveKeyboard setter for remove keyboard
func (b *SendMessageBuilder) SetRemoveKeyboard() error {
	b.baseChat.ReplyMarkup = telegram.NewRemoveKeyboard(true)

	return nil
}

// Build all message data
func (b *SendMessageBuilder) Build() (telegram.Chattable, error) {
	return b.messageConfig, nil
}

// Build all message data with chat action status
func (b *SendMessageBuilder) BuildWithAction() (telegram.Chattable, telegram.Chattable, error) {
	action := telegram.NewChatAction(
		b.chatID, b.chatAction,
	)
	return b.messageConfig, action, nil
}

// getMediaType parse mimetype and return file type, like: image, audio and video
func getMediaType(mtyp string) string {
	mtyp = strings.TrimSpace(mtyp)
	mtyp = strings.ToLower(mtyp)
	subt := strings.IndexByte(mtyp, '/')
	if subt > 0 {
		return mtyp[:subt]
	}
	return mtyp
}

// getKeyboardButton returns button by type
func getKeyboardButton(button *pbchat.Button) (telegram.KeyboardButton, bool) {
	switch strings.ToLower(button.Type) {
	case "contact", "phone":
		return telegram.NewKeyboardButtonContact(button.Text), true
	case "email", "mail":
		// Not supported yet
	case "location":
		return telegram.NewKeyboardButtonLocation(button.Text), true
	}

	return telegram.KeyboardButton{}, false
}

// getInlineKeyboardButton returns inline button by type
func getInlineKeyboardButton(button *pbchat.Button) (telegram.InlineKeyboardButton, bool) {
	switch strings.ToLower(button.Type) {
	case "url":
		return telegram.NewInlineKeyboardButtonURL(button.Text, button.Url), true
	case "switch":
		return telegram.NewInlineKeyboardButtonSwitch(button.Text, button.Code), true
	case "reply", "postback":
		return telegram.NewInlineKeyboardButtonData(button.Text, button.Code), true
	}

	return telegram.InlineKeyboardButton{}, false
}
