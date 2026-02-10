package viber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	log2 "github.com/webitel/chat_manager/log"

	"github.com/micro/micro/v3/service/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"github.com/webitel/chat_manager/internal/util"
)

const (
	updateWebhook     = "webhook" // set_webhook:callback(POST)
	updateNewDialog   = "conversation_started"
	updateJoinMember  = "subscribed"
	updateLeftMember  = "unsubscribed"
	updateNewMessage  = "message"
	updateSentMessage = "delivered"
	updateReadMessage = "seen"
	updateFailMessage = "failed"
)

type rawUpdate struct {
	// basic
	Type      string `json:"event"`
	Hostname  string `json:"chat_hostname"`
	Timestamp int64  `json:"timestamp"`
	MessageId uint64 `json:"message_token"`
	// [subscribed]
	User *User `json:"user,omitempty"`

	// [unsubscribed]
	UserId string `json:"user_id,omitempty"`

	// [conversation_started]
	// Type string `json:"type,omitempty"` // "open"
	Context string `json:"context,omitempty"`
	// User *struct{} `json:"user,omitempty"`
	Subscribed bool `json:"subscribed,omitempty"`

	// [message]
	Sender  *User    `json:"sender,omitempty"`
	Message *Message `json:"message,omitempty"`

	// [failed]
	// A string describing the failure.
	Fail string `json:"desc,omitempty"`

	// Silent  bool    `json:"silent"`
	// UserID  string  `json:"user_id,omitempty"`
}

type Update struct {
	// Base
	Type      string `json:"event"`
	Hostname  string `json:"chat_hostname"`
	Timestamp int64  `json:"timestamp"`
	MessageId uint64 `json:"message_token"`
	// Args
	// [conversation_started]
	NewDialog *UpdateNewDialog
	// [subscribed]
	JoinMember *UpdateJoinMember
	// [unsubscribed]
	LeftMember *UpdateLeftMember
	// [message]
	NewMessage *UpdateNewMessage
	// [delivered,seen,failed]
	Message *UpdateMessage
}

type UpdateTypeError string

func (e UpdateTypeError) Error() string {
	return "viber: event \"" + string(e) + "\" type unknown"
}

func (e *Update) UnmarshalJSON(data []byte) error {
	var args rawUpdate
	err := json.Unmarshal(data, &args)
	if err != nil {
		return err
	}
	// basic
	e.Type = args.Type
	e.Hostname = args.Hostname
	e.Timestamp = args.Timestamp
	e.MessageId = args.MessageId
	// update event args
	switch args.Type {
	// "conversation_started"
	case updateNewDialog:
		e.NewDialog = args.NewDialog()
	// "subscribed"
	case updateJoinMember:
		e.JoinMember = args.JoinMember()
	// "unsubscribed"
	case updateLeftMember:
		e.LeftMember = args.LeftMember()
	// "message"
	case updateNewMessage:
		e.NewMessage = args.NewMessage()
	// "delivered"
	case updateSentMessage:
		e.Message = args.SentMessage()
	// "seen"
	case updateReadMessage:
		e.Message = args.ReadMessage()
	// "failed"
	case updateFailMessage:
		e.Message = args.FailMessage()
	// unknown
	default:
		// return error
	}
	return nil
}

type UpdateNewDialog struct {
	Type       string `json:"type,omitempty"` // "open"
	User       *User  `json:"user,omitempty"`
	Context    string `json:"context,omitempty"`
	Subscribed bool   `json:"subscribed,omitempty"`
}

// [conversation_started]
func (e *rawUpdate) NewDialog() *UpdateNewDialog {
	var args *UpdateNewDialog
	if e.Type == updateNewDialog {
		args = &UpdateNewDialog{
			Type:       "open",
			User:       e.User,
			Context:    e.Context,
			Subscribed: e.Subscribed,
		}
	}
	return args
}

type UpdateJoinMember struct {
	User *User `json:"user,omitempty"`
}

// [subscribed]
func (e *rawUpdate) JoinMember() *UpdateJoinMember {
	var args *UpdateJoinMember
	if e.Type == updateJoinMember {
		args = &UpdateJoinMember{
			User: e.User,
		}
	}
	return args
}

type UpdateLeftMember struct {
	UserId string `json:"user_id,omitempty"`
}

// [unsubscribed]
func (e *rawUpdate) LeftMember() *UpdateLeftMember {
	var args *UpdateLeftMember
	if e.Type == updateLeftMember {
		args = &UpdateLeftMember{
			UserId: e.UserId,
		}
	}
	return args
}

type UpdateNewMessage struct {
	Sender  *User    `json:"sender"`
	Message *Message `json:"message"`
}

// [message]
func (e *rawUpdate) NewMessage() *UpdateNewMessage {
	var args *UpdateNewMessage
	if e.Type == updateNewMessage {
		args = &UpdateNewMessage{
			Sender:  e.Sender,
			Message: e.Message,
		}
	}
	return args
}

type UpdateMessage struct {
	// Timestamp int64  `json:"timestamp"`
	MessageId uint64 `json:"message_token"`
	UserId    string `json:"user_id,omitempty"`
	Status    string `json:"event"`
	Failed    string `json:"desc,omitempty"`
}

// [delivered]
func (e *rawUpdate) SentMessage() *UpdateMessage {
	var args *UpdateMessage
	if e.Type == updateSentMessage {
		args = &UpdateMessage{
			MessageId: e.MessageId,
			UserId:    e.UserId,
			Status:    e.Type,
			Failed:    "",
		}
	}
	return args
}

// [seen]
func (e *rawUpdate) ReadMessage() *UpdateMessage {
	var args *UpdateMessage
	if e.Type == updateReadMessage {
		args = &UpdateMessage{
			MessageId: e.MessageId,
			UserId:    e.UserId,
			Status:    e.Type,
			Failed:    "",
		}
	}
	return args
}

// [failed]
func (e *rawUpdate) FailMessage() *UpdateMessage {
	var args *UpdateMessage
	if e.Type == updateFailMessage {
		args = &UpdateMessage{
			MessageId: e.MessageId,
			UserId:    e.UserId,
			Status:    e.Type,
			Failed:    e.Fail,
		}
	}
	return args
}

// ---------- Update(s) Handler(s) ---------- //

// ON: [conversation_started]
//
// Conversation started event fires when a user opens a conversation with the bot using the “message” button (found on the account’s info screen) or using a deep link.
//
// This event is not considered a subscribe event and doesn’t allow the account to send messages to the user; however, it will allow sending one “welcome message” to the user.
// See sending a welcome message below for more information.
//
// Once a conversation_started callback is received, the service will be able to respond with a JSON containing same parameters as a send_message request.
// The receiver parameter is not mandatory in this case.
//
// Note: the conversation_started callback doesn’t contain the context parameter by default.
// To add this paramater and determine its value, you can use a deeplink like this: viber://pa?chatURI=your_bot_URI&context=your_context
//
// https://developers.viber.com/docs/api/rest-bot-api/#conversation-started
func (c *Bot) onNewDialog(ctx context.Context, event *Update) error {

	var (
		update  = event.NewDialog
		sender  = update.User
		contact = &bot.Account{
			ID:        0, // LOOKUP
			Channel:   provider,
			Contact:   sender.ID,
			FirstName: sender.Name,
		}
	)

	chatID := sender.ID
	dialog, err := c.Gateway.GetChannel(
		ctx, chatID, contact,
	)
	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		return re // 502 Bad Gateway
	}

	if !dialog.IsNew() {
		return nil // FIXME: How can we got here ?
	}

	sendUpdate := bot.Update{
		Chat:  dialog,
		User:  contact,
		Title: dialog.Title,
		Message: &chat.Message{
			Type: "text",
			Text: "/welcome",
		},
	}
	if event.NewDialog.Context != "" {
		sendUpdate.Message.Variables = map[string]string{"ref": event.NewDialog.Context}
	}
	// Start new dialog
	err = c.Gateway.Read(ctx, &sendUpdate)

	log := c.Gateway.Log.With(
		slog.Any("user", log2.SlogObject(&User{ID: sender.ID, Name: sender.Name})),
	)

	if err == nil {
		log.Info("viber/bot.onConversationStarted")
	} else {
		log.Error("viber/bot.onConversationStarted",
			slog.Any("error", err),
		)
	}

	return nil
}

// ON: [subscribed]
//
// Before an account can send messages to a user, the user will need to subscribe to the account.
// Subscribing can take place if the user sends a message to the bot.
// When a user sends its first message to a bot the user will be automatically subscribed to the bot.
// Sending the first message will not trigger a subscribe callback, only a message callback (see receive message from user section).
//
// You will receive a subscribed event when unsubscribed users do the following:
//
// 1. Open conversation with the bot.
// 2. Tap on the 3-dots button in the top right and then on “Chat Info”.
// 3. Tap on “Receive messages”.
//
// Note: A subscribe event will delete any context or tracking_data information related to the conversation. This means that if a user had a conversation with a service and then chose to unsubscribe and subscribe again, a new conversation will be started without any information related to the old conversation.
//
// https://developers.viber.com/docs/api/rest-bot-api/#subscribed
func (c *Bot) onJoinMember(ctx context.Context, event *Update) error {

	return nil // IGNORE

	var (
		update  = event.JoinMember
		sender  = update.User
		contact = &bot.Account{
			ID:        0, // LOOKUP
			Channel:   provider,
			Contact:   sender.ID,
			FirstName: sender.Name,
		}
	)

	chatID := sender.ID
	dialog, err := c.Gateway.GetChannel(
		ctx, chatID, contact,
	)
	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		return re // 502 Bad Gateway
	}

	if !dialog.IsNew() {
		return nil // FIXME: How can we got here ?
	}

	sendUpdate := bot.Update{
		Chat:  dialog,
		User:  contact,
		Title: dialog.Title,
		Message: &chat.Message{
			Type: "text",
			Text: "/subscribed",
		},
	}
	err = c.Gateway.Read(ctx, &sendUpdate)

	log := c.Gateway.Log.With(
		slog.Any("user", log2.SlogObject(&User{ID: sender.ID, Name: sender.Name})),
	)

	if err == nil {
		log.Info("viber/bot.onSubscribed")
	} else {
		log.Error("viber/bot.onSubscribed",
			slog.Any("error", err),
		)
	}

	return nil
}

// ON: [unsubscribed]
//
// The user will have the option to unsubscribe from the PA.
// This will trigger an unsubscribed callback.
//
// https://developers.viber.com/docs/api/rest-bot-api/#unsubscribed
func (c *Bot) onLeftMember(ctx context.Context, event *Update) error {
	// Viber user tap "Do NOT receive messages" from our Bot !
	chatId := event.LeftMember.UserId
	dialog, err := c.Gateway.GetChannel(ctx, chatId, nil)
	if err == nil && !dialog.IsNew() {
		err = dialog.Close()
	}

	log := c.Gateway.Log.With(
		slog.String("userId", chatId),
	)
	if err == nil {
		log.Info("viber/bot.onUnsubscribed")
	} else {
		log.Error("viber/bot.onUnsubscribed",
			slog.Any("error", err),
		)
	}

	return nil
}

// on: [message]
func (c *Bot) onNewMessage(ctx context.Context, event *Update) error {

	var (
		update  = event.NewMessage
		sender  = update.Sender
		message = update.Message
		contact = &bot.Account{
			ID:        0, // LOOKUP
			Channel:   provider,
			Contact:   sender.ID,
			FirstName: sender.Name,
		}
	)
	// endregion

	// region: channel
	chatID := sender.ID
	channel, err := c.Gateway.GetChannel(
		ctx, chatID, contact,
	)
	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		return re // 502 Bad Gateway
	}

	sendUpdate := bot.Update{
		Title: channel.Title,
		Chat:  channel,
		User:  contact,
	}

	switch message.Type {

	case mediaText:
		// https://developers.viber.com/docs/api/rest-bot-api/#text-message
		sendUpdate.Message = &chat.Message{
			Type: "text",
			Text: parseTextWithEmoji(message.Text),
		}

	case mediaURL:
		// https://developers.viber.com/docs/api/rest-bot-api/#url-message
		sendUpdate.Message = &chat.Message{
			Type: "text",
			Text: message.MediaURL,
		}

	case mediaImage:
		// https://developers.viber.com/docs/api/rest-bot-api/#picture-message
		sendUpdate.Message = &chat.Message{
			Type: "file",
			File: &chat.File{
				Url: message.MediaURL,
				// Mime: "image/*",
				//
				// [message.FileName] MAY be specified
				// but has irrelevant file_name for image
				//
				// Filename (from .MediaURL) will be [auto-]detected (-if- not specified)
				// while /webitel.chat.server/ChatService.SendMessage(!) delivery
			},
			// Description of an image. Caption. Optional.
			Text: parseTextWithEmoji(message.Text),
		}

	case mediaVideo:
		// https://developers.viber.com/docs/api/rest-bot-api/#video-message
		sendUpdate.Message = &chat.Message{
			Type: "file",
			File: &chat.File{
				Url:  message.MediaURL,
				Size: message.FileSize,
				// Mime: "video/*",
			},
		}

	case mediaSticker:
		// https://developers.viber.com/docs/api/rest-bot-api/#sticker-message
		sendUpdate.Message = &chat.Message{
			Type: "file",
			File: &chat.File{
				// message.StickerId,
				Url: message.MediaURL,
				// Mime: "sticker/*", "image/png",
			},
		}

	case mediaFile:
		// https://developers.viber.com/docs/api/rest-bot-api/#file-message
		sendUpdate.Message = &chat.Message{
			Type: "file",
			File: &chat.File{
				Url:  message.MediaURL,
				Name: message.FileName,
				Size: message.FileSize,
			},
		}

	case mediaContact:
		// https://developers.viber.com/docs/api/rest-bot-api/#contact-message

		// sendUpdate.Message = &chat.Message{
		// 	Type: "contact",
		// 	Contact: &chat.Account{
		// 		Channel: "phone",
		// 		Contact: message.Contact.Phone,
		// 	},
		// }

		// Convert given .Contacts to
		// human-readable .Text message
		buf := bytes.NewBuffer(nil)
		err := contactInfo.Execute(
			buf, message.Contact,
		)
		if err != nil {
			buf.Reset()
			_, _ = buf.WriteString(err.Error())
		}

		// sendUpdate.Message = &chat.Message{
		// 	Type: "text",
		// 	Text: buf.String(),
		// }
		sendUpdate.Message = &chat.Message{
			Type: "contact",
			Text: buf.String(),
			Contact: &chat.Account{
				// Id:        0,
				Channel:   "phone",
				Contact:   message.Contact.Phone,
				FirstName: message.Contact.Name,
				// LastName:  "",
			},
		}

		sendUpdate.Message.Contact.FirstName, sendUpdate.Message.Contact.LastName =
			util.ParseFullName(sendUpdate.Message.Contact.FirstName)

		if message.Text == btnShareContactCode {
			// NOTE: This MIGHT be contact Phone from current User (via <share-phone> button)
			sendUpdate.Message.Contact.Id = channel.Account.ID // MARK: sender:owned
		}

	case mediaLocation:
		// https://developers.viber.com/docs/api/rest-bot-api/#location-message
		location := message.Location
		sendUpdate.Message = &chat.Message{
			Type: "text",
			Text: fmt.Sprintf(
				"https://www.google.com/maps/place/%f,%f",
				location.Latitude, location.Longitude,
			),
		}

	default:
		c.Gateway.Log.Error("viber/onNewMessage",
			slog.String("error", "message: type \""+message.Type+"\" unknown"),
		)

		return nil // IGNORE
	}

	err = c.Gateway.Read(ctx, &sendUpdate)

	if err != nil {
		c.Gateway.Log.Error("viber/onNewMessage",
			slog.Any("error", err),
		)
		return err // 502 Bad Gateway
	}

	return nil
}

// ON: [delivered, seen, failed]
//
// Viber offers message status updates for each message sent, allowing the account
// to be notified when the message was delivered to the user’s device (delivered status)
// and when the conversation containing the message was opened (seen status).
//
// The seen callback will only be sent once when the user reads the unread messages,
// regardless of the number of messages sent to them, or the number of devices they are using.
//
// If the message recipient is using their Viber account on multiple devices,
// each of the devices will return a delivered, meaning that several delivered callbacks
// can be received for a single message.
//
// If Viber is unable to deliver the message to the client it will try to deliver it for up to 14 days.
// If the message wasn’t delivered within the 14 days it will not be delivered
// and no “delivered” or “seen” callbacks will be received for it.
//
// https://developers.viber.com/docs/api/rest-bot-api/#message-receipts-callbacks
func (c *Bot) onMsgStatus(ctx context.Context, event *Update) error {
	return nil
	// update := event.Message
	// switch update.Status { // == event.Type
	// case updateSentMessage:
	// 	// NOTE: may trigger several times for single message
	// 	//       for each account's device succesfull delivery
	// 	//
	// 	// update.UserId
	// 	// update.MessageId
	// case updateReadMessage:
	// 	// update.UserId
	// 	// update.MessageId
	// case updateFailMessage:
	// 	// update.UserId
	// 	// update.MessageId
	// 	// update.Failed // ERROR Message
	// default:
	// 	return
	// }
}
