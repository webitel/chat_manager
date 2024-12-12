package builder

import (
	"errors"
	"strings"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot/telegram/internal/helper"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown"
	"github.com/webitel/chat_manager/internal/util"
)

const (
	// Ignore button types
	buttonTypeMail  = "mail"
	buttonTypeEmail = "email"

	// Remove button types
	buttonTypeClear          = "clear"
	buttonTypeRemove         = "remove"
	buttonTypeRemoveKeyboard = "remove_keyboard"

	// Other button types
	buttonTypeContact  = "contact"
	buttonTypePhone    = "phone"
	buttonTypeLocation = "location"
	buttonTypePostback = "postback"
	buttonTypeUrl      = "url"
	buttonTypeSwitch   = "switch"
	buttonTypeReply    = "reply"
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

	b.chatAction = "" // telegram.ChatTyping

	text, entities := markdown.TextEntities(text)
	message := telegram.NewMessage(
		b.chatID, text,
	)
	// if len(entities) > 0 {
	message.Entities = entities
	// message.ParseMode = telegram.ModeMarkdownV2
	// }
	b.messageConfig = &message
	b.baseChat = &message.BaseChat

	return nil
}

// SetFile setter for file message, like: image, audio, video and document
func (b *SendMessageBuilder) SetFile(filename, mimetype, url, caption string) error {
	switch util.ParseMediaType(mimetype) {
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

// SetKeyboard setter for classic keyboard
func (b *SendMessageBuilder) SetKeyboard(buttons []*pbchat.Buttons) error {
	var keyboard [][]telegram.KeyboardButton

	for _, markup := range buttons {
		var row []telegram.KeyboardButton
		for _, button := range markup.Button {
			if isMustIgnoreButton(button) {
				continue
			}

			if btn, ok := getKeyboardButton(button); ok {
				row = append(row, btn)
				continue
			}

			row = append(row,
				telegram.NewKeyboardButton(button.Text),
			)
		}

		if len(row) > 0 {
			keyboard = append(keyboard, row)
		}
	}

	if len(keyboard) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewOneTimeReplyKeyboard(keyboard...)
	}

	return nil
}

// SetKeyboard setter for inline keyboard
func (b *SendMessageBuilder) SetInlineKeyboard(buttons []*pbchat.Buttons) error {
	var inlineKeyboard [][]telegram.InlineKeyboardButton

	for _, markup := range buttons {
		var inlineRow []telegram.InlineKeyboardButton
		for _, button := range markup.Button {
			if isMustIgnoreButton(button) {
				continue
			}

			if btn, ok := getInlineKeyboardButton(button); ok {
				inlineRow = append(inlineRow, btn)
				continue
			}

			inlineRow = append(inlineRow,
				telegram.NewInlineKeyboardButtonData(button.Text, button.Code),
			)
		}

		if len(inlineRow) > 0 {
			inlineKeyboard = append(inlineKeyboard, inlineRow)
		}
	}

	if len(inlineKeyboard) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewInlineKeyboardMarkup(inlineKeyboard...)
	}

	return nil
}

// SetMergedKeyboard accepts both conventional and inline keyboards, but the classic keyboard will always be a priority
func (b *SendMessageBuilder) SetMergedKeyboard(buttons []*pbchat.Buttons) error {
	var keyboard [][]telegram.KeyboardButton
	var inlineKeyboard [][]telegram.InlineKeyboardButton

	for _, markup := range buttons {
		var row []telegram.KeyboardButton
		var inlineRow []telegram.InlineKeyboardButton
		for _, button := range markup.Button {
			if isMustIgnoreButton(button) {
				continue
			}

			if isRemoveKeyboard(button) {
				return b.SetRemoveKeyboard()
			}

			if btn, ok := getKeyboardButton(button); ok {
				row = append(row, btn)
				continue
			}

			if btn, ok := getInlineKeyboardButton(button); ok {
				inlineRow = append(inlineRow, btn)
				continue
			}

			row = append(row,
				telegram.NewKeyboardButton(button.Text),
			)
		}

		if len(row) > 0 {
			keyboard = append(keyboard, row)
		}

		if len(inlineRow) > 0 {
			inlineKeyboard = append(inlineKeyboard, inlineRow)
		}
	}

	if len(inlineKeyboard) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewInlineKeyboardMarkup(inlineKeyboard...)
	} else if len(keyboard) > 0 {
		b.baseChat.ReplyMarkup = telegram.NewOneTimeReplyKeyboard(keyboard...)
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
func (b *SendMessageBuilder) BuildWithAction() (message, action telegram.Chattable, err error) {
	message = b.messageConfig
	if b.chatAction != "" {
		action = telegram.NewChatAction(
			b.chatID, b.chatAction,
		)
	}
	return // b.messageConfig, b.chatAction, nil
}

// isMustIgnoreButton returns true if button must be ignored
func isMustIgnoreButton(button *pbchat.Button) bool {
	switch strings.ToLower(button.Type) {
	case buttonTypeMail, buttonTypeEmail:
		return true
	}
	return false
}

// isRemoveKeyboard returns true if button is clear/remove type
func isRemoveKeyboard(button *pbchat.Button) bool {
	switch strings.ToLower(button.Type) {
	case buttonTypeClear, buttonTypeRemove, buttonTypeRemoveKeyboard:
		return true
	}
	return false
}

// getKeyboardButton returns button by type
func getKeyboardButton(button *pbchat.Button) (telegram.KeyboardButton, bool) {
	switch strings.ToLower(button.Type) {
	case buttonTypeContact, buttonTypePhone:
		return telegram.NewKeyboardButtonContact(button.Text), true
	case buttonTypeLocation:
		return telegram.NewKeyboardButtonLocation(button.Text), true
	case buttonTypePostback:
		return telegram.NewKeyboardButton(button.Text), true
	}

	return telegram.KeyboardButton{}, false
}

// getInlineKeyboardButton returns inline button by type
func getInlineKeyboardButton(button *pbchat.Button) (telegram.InlineKeyboardButton, bool) {
	switch strings.ToLower(button.Type) {
	case buttonTypeUrl:
		return telegram.NewInlineKeyboardButtonURL(button.Text, button.Url), true
	case buttonTypeSwitch:
		return telegram.NewInlineKeyboardButtonSwitch(button.Text, button.Code), true
	case buttonTypeReply:
		return telegram.NewInlineKeyboardButtonData(button.Text, button.Code), true
	}

	return telegram.InlineKeyboardButton{}, false
}
