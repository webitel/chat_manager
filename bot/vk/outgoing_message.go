package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	vk "github.com/SevereCloud/vksdk/v2/api"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
)

type OutgoingMessage struct {
	// Message receiver id (!REQUIRED)
	userId string `json:"user_id,omitempty"`
	// Check message for uniqueness (!REQUIRED)
	UniqueCheck int `json:"random_id,omitempty"`
	// Chats
	peerIds []string `json:"peer_ids,omitempty"`
	// Id of chat
	//	PeerId int `json:"peer_id,omitempty"`
	// Text of message
	Text string `json:"message,omitempty"`
	// Attachment for message (only existing media)
	Attachments []string `json:"attachment,omitempty"`
	// Reply to [message_id]
	ReplyTo int64 `json:"reply_to,omitempty"`
	// BOT Keyboard
	Keyboard *VKKeyboard `json:"keyboard,omitempty"`
}

func (m *OutgoingMessage) SetReceiver(peerId ...string) error {
	switch len(peerId) {
	case 0:
		err := fmt.Errorf("vk: no receiver passed")
		return err
	case 1:
		m.userId = peerId[0]
	default:
		m.peerIds = peerId
	}
	return nil
}

func (m *OutgoingMessage) GetReceiver() []string {
	var peerIds []string
	if m.userId != "" {
		peerIds = append(peerIds, m.userId)
	} else {
		peerIds = m.peerIds
	}
	return peerIds
}

func (m *OutgoingMessage) Params() (*vk.Params, error) {
	var res vk.Params
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return nil, err
	}

	if m.Keyboard != nil {
		stringKeyboard, err := m.Keyboard.String()
		if err != nil {
			return nil, err
		}
		res["keyboard"] = fmt.Sprintf("{\"one_time\": %s, \"buttons\": %s}", strconv.FormatBool(m.Keyboard.OneTime), stringKeyboard)
	}
	if len(m.Attachments) != 0 {
		att := strings.Join(m.Attachments, ",")
		res["attachment"] = att
	}
	if receivers := m.GetReceiver(); len(m.GetReceiver()) != 0 {
		switch len(receivers) {
		case 1:
			rec, err := strconv.Atoi(receivers[0])
			if err != nil {
				return nil, errors.BadRequest("bot.vk.outgoing_message.params.error", err.Error())
			}
			res["user_id"] = rec
		default:
			ids := strings.Join(receivers, ",")
			res["peer_ids"] = ids
		}

	}
	return &res, nil
}

// IsValid returns error if message can't be send
func (m *OutgoingMessage) IsValid() error {
	if m.Text == "" && len(m.Attachments) == 0 {
		return errors.BadRequest("bot.vk.message.no_payload", "vk: message doesn't contain payload")
	}
	if m.userId == "" && /*m.PeerId <= 0 &&*/ len(m.peerIds) == 0 {
		return errors.BadRequest("bot.vk.message.no_receiver", "vk: message doesn't have receiver")
	}

	return nil
}

type VKKeyboard struct {
	// Show only once
	OneTime bool `json:"one_time,omitempty"`
	// Is keyboard on message
	IsInline bool       `json:"inline,omitempty"`
	Buttons  [][]Button `json:"buttons,omitempty"`
}

type Button struct {
	Action map[string]string `json:"action,omitempty"`
}

type DocUploadResponse struct {
	Server int    `json:"server"`
	Photo  string `json:"photo"`
	File   string `json:"file"`
	Hash   string `json:"hash"`
}

// String converts VK Buttons to the string format with this preparing them to send
func (v *VKKeyboard) String() (string, error) {
	var keyboard string
	bytes, err := json.Marshal(v.Buttons)
	if err != nil {
		return "", err
	}
	keyboard = string(bytes)
	return keyboard, nil
}

// ConvertInternalToOutcomingMessage performs a conversion of an incoming FROM [WEBITEL] message to the VK message structure
func (c *VKBot) ConvertInternalToOutcomingMessage(update *bot.Update) (*OutgoingMessage, error) {
	message := update.Message
	// region PREPARING STRUCT

	// Converting chat id to int
	if update.Chat == nil {
		return nil, errors.BadRequest("bot.vk.check_args.chat.nil", "channel peer is nil")
	}
	channel := update.Chat
	chatId, err := strconv.Atoi(channel.ChatID)
	if err != nil {
		return nil, errors.BadRequest("bot.vk.convert_chat_id.error", err.Error())
	}
	result := &OutgoingMessage{
		//PeerId: chatId,
		userId: strconv.Itoa(chatId),
		//UserId:     []int{chatId},
		UniqueCheck: rand.Intn(1000 * 1000),
		Text:        message.GetText(),
		//Attachments: update.Message.
		ReplyTo: message.GetReplyToMessageId(),
	}
	// endregion

	// region BUILD KEYBOARD
	if update.Message.Buttons != nil {
		keyboard, err := BuildVKKeyboard(message.Buttons)
		if err != nil {
			return nil, err
		}
		result.Keyboard = keyboard
	}
	// endregion

	// region DETERMINE MESSAGE TYPE
	switch message.Type {
	case "text":
		messageText := strings.TrimSpace(
			message.GetText(),
		)
		result.Text = messageText

	case "file":
		doc := message.GetFile()
		// mime.ParseMediaType()
		mediaType := func(mtyp string) string {
			mtyp = strings.TrimSpace(mtyp)
			mtyp = strings.ToLower(mtyp)
			subt := strings.IndexByte(mtyp, '/')
			if subt > 0 {
				return mtyp[:subt]
			}
			return mtyp
		}
		switch mediaType(doc.GetMime()) {
		case "image":
			attachment, err := c.SendPhoto(doc.Name, doc.Url)
			// if can't be sent as image - try to send as document
			if err != nil {
				attachment, err = c.SendDoc(doc.Name, doc.Url, int64(chatId))
				if err != nil {
					return nil, err
				}

			}
			result.Attachments = append(result.Attachments, attachment)
		default:
			attachment, err := c.SendDoc(doc.Name, doc.Url, int64(chatId))
			if err != nil {
				return nil, err
			}
			result.Attachments = append(result.Attachments, attachment)
		}

	case "joined":
		peer := message.NewChatMembers[0]
		updates := c.Gateway.Template
		text, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Error("vk/bot.updateChatMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		//if text != "" {
		// format new message to the engine for saving it in the DB as operator message [WTEL-4695]
		messageToSave := &chat.Message{
			Type:      "text",
			Text:      text,
			CreatedAt: time.Now().UnixMilli(),
			From:      peer,
		}

		if channel.ChannelID != "" {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			_, err = c.Gateway.Internal.Client.SendServiceMessage(ctx, &chat.SendServiceMessageRequest{Message: messageToSave, Receiver: channel.ChannelID})
			cancel()
			if err != nil {
				return nil, err
			}
		}
		result.Text = text
		//}

	case "left":
		peer := message.LeftChatMember
		updates := c.Gateway.Template
		text, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Error("vk/bot.updateChatMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		//if text != "" {
		result.Text = text
		//}

	case "closed":
		updates := c.Gateway.Template
		text, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Error("vk/bot.updateChatMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		//if text != "" {
		result.Text = text
		//}

	default:
		messageText := strings.TrimSpace(
			message.GetText(),
		)
		if messageText != "" {
			result.Text = messageText
		}
	}
	// endregion

	// SUCCESSFUL RESULT
	return result, nil
}

// BuildVKKeyboard performs full conversion FROM [WEBITEL] TO [VK] buttons
func BuildVKKeyboard(in []*chat.Buttons) (*VKKeyboard, error) {
	var (
		result = &VKKeyboard{OneTime: true, Buttons: make([][]Button, 0)}
	)

	for i, buttons := range in {
		result.Buttons = append(result.Buttons, make([]Button, 0))
		for _, button := range buttons.Button {
			switch button.Type {
			case "clear", "remove", "remove_keyboard":
				return &VKKeyboard{OneTime: true, Buttons: make([][]Button, 0)}, nil
			case "message":
				result.IsInline = true
				result.OneTime = false
			}
			result.Buttons[i] = append(result.Buttons[i], *ConvertInternalToVKButton(button))
		}
	}

	return result, nil
}

func ConvertInternalToVKButton(button *chat.Button) *Button {
	var result Button
	result.Action = make(map[string]string)
	switch button.Type {
	//case "clear", "remove", "remove_keyboard":
	//	// Invalidate keyboard (persistent menu)
	//	// return telegram.NewRemoveKeyboard(true)
	//	return nil
	// keyboard_button (persistent menu)
	//case "phone", "contact", "email", "mail":
	//	// NOT SUPPORTED!
	//	result.Action["type"] = "text"
	//	result.Action["label"] = button.Text
	//case "email", "mail":
	//	// NOT Supported !
	case "location":
		result.Action["type"] = "location"
	// inline_keyboard: quick_reply
	case "url":
		result.Action["type"] = "open_link"
		result.Action["link"] = button.Url
		result.Action["label"] = button.Text
	//case "reply": //, "postback":
	//	repliesLayout = append(repliesLayout,
	//		telegram.NewInlineKeyboardButtonData(
	//			button.Text, button.Code,
	//		),
	//	)
	//case "postback":
	//	// NOTE: In this (Telegram) implementation .code attribute cannot be involved,
	//	// so you must be vigilant in handling localized menu button labels as postback messages !
	//	buttonsLayout = append(buttonsLayout,
	//		telegram.NewKeyboardButton(button.Text),
	//	)
	default:

		result.Action["type"] = "text"
		result.Action["label"] = button.Text
	}

	return &result
}
