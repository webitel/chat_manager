package telegram

import (
	"context"
	"strconv"
	"strings"
	"time"

	// "net/url"
	"io/ioutil"
	"net/http"

	"encoding/json"
	"path/filepath"

	"github.com/micro/go-micro/v2/errors"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/bot"

	// gate "github.com/webitel/chat_manager/api/proto/bot"
	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
	chat "github.com/webitel/chat_manager/api/proto/chat"
)

func init() {
	bot.Register("telegram", NewTelegramBot)
}

// Telegram BOT chat provider
type TelegramBot struct {
	*bot.Gateway
	*telegram.BotAPI
}

func (_ *TelegramBot) Close() error {
	return nil
}

// String "telegram" provider's name
func (_ *TelegramBot) String() string {
	return "telegram"
}

// NewTelegramBotV1 initialize new agent.profile service provider
// func NewTelegramBot(agent *bot.Gateway) (bot.Provider, error) {
func NewTelegramBot(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {

	config := agent.Bot
	profile := config.GetMetadata()

	token, ok := profile["token"]
	
	if !ok {
		
		return nil, errors.BadRequest(
			"chat.bot.telegram.token.required",
			"telegram: bot API token required",
		)
	}

	var (
		
		err error
		botAPI *telegram.BotAPI
		httpClient *http.Client
	)

	trace := profile["trace"]
	if on, _ := strconv.ParseBool(trace); on {
		var transport http.RoundTripper
		if httpClient != nil {
			transport = httpClient.Transport
		}
		if transport == nil {
			transport = http.DefaultTransport
		}
		transport = &bot.TransportDump{
			Transport: transport,
			WithBody: true,
		}
		if httpClient == nil {
			httpClient = &http.Client{
				Transport: transport,
			}
		} else {
			httpClient.Transport = transport
		}
	}

	// httpClient = &http.Client{
	// 	Transport: &transportDump{
	// 		r: http.DefaultTransport,
	// 		WithBody: true,
	// 	},
	// }

	if httpClient == nil {
		botAPI, err = telegram.NewBotAPI(token)
	} else {
		botAPI, err = telegram.NewBotAPIWithClient(token, httpClient)
	}

	if err != nil {

		return nil, errors.New(
			"chat.bot.telegram.setup.error",
			"telegram: "+ err.Error(),
			 http.StatusBadGateway,
		)
	}

	return &TelegramBot{
		Gateway: agent,
		BotAPI: botAPI,
	}, nil
}

// Register Telegram Bot Webhook endpoint URI
func (c *TelegramBot) Register(ctx context.Context, linkURL string) error {

	// // webhookInfo := tgbotapi.NewWebhookWithCert(fmt.Sprintf("%s/telegram/%v", cfg.TgWebhook, profile.Id), cfg.CertPath)
	// linkURL := strings.TrimRight(c.Gateway.Internal.URL, "/") +
	// 	("/" + c.Gateway.Profile.UrlId)
	
	webhook := telegram.NewWebhook(linkURL)
	_, err := c.BotAPI.SetWebhook(webhook)
	
	if err != nil {
		c.Gateway.Log.Error().Err(err).Msg("Failed to .Register webhook")
		return err
	}

	return nil
}

// Deregister Telegram Bot Webhook endpoint URI
func (c *TelegramBot) Deregister(ctx context.Context) error {
	
	res, err := c.BotAPI.RemoveWebhook()
	
	if err != nil {
		return err
	}

	if !res.Ok {
		return errors.New(
			"chat.bot.telegram.deregister.error", 
			"telegram: "+ res.Description,
			 (int32)(res.ErrorCode), // FIXME: 502 Bad Gateway ?
		)
	}

	return nil
}

// SendNotify implements provider.Sender interface for Telegram
func (c *TelegramBot) SendNotify(ctx context.Context, notify *bot.Update) error {
	// send *gate.SendMessageRequest
	// externalID, err := strconv.ParseInt(send.ExternalUserId, 10, 64)

	var (

		channel = notify.Chat // recepient
		// localtime = time.Now()
		message = notify.Message

		binding map[string]string
	)

	// region: recover latest chat channel state
	chatID, err := strconv.ParseInt(channel.ChatID, 10, 64)
	if err != nil {
		return errors.InternalServerError(
			"chat.gateway.telegram.chat.id.invalid",
			"telegram: invalid chat %s unique identifier; expect integer values", channel.ChatID)
	}

	if channel.Title == "" {
		// FIXME: .GetChannel() does not provide full contact info on recover,
		//                      just it's unique identifier ...  =(
	}

	// // TESTS
	// props, _ := channel.Properties.(map[string]string)
	// endregion

	bind := func(key, value string) {
		if binding == nil {
			binding = make(map[string]string)
		}
		binding[key] = value
	}

	var update telegram.Chattable
	// TODO: resolution for various notify content !
	switch message.Type { // notify.Event {
	case "text": // default
	
		text := message.GetText()

		msg := telegram.NewMessage(chatID, text)

		if message.Buttons != nil {
			
			if len(message.Buttons) == 0 { // CLEAR Buttons

				msg.ReplyMarkup = telegram.NewRemoveKeyboard(false)

			}else {

				msg.ReplyMarkup = newReplyKeyboard(message.Buttons)

			}

		} else if message.Inline != nil {

			msg.ReplyMarkup = newInlineKeyboard(message.Inline)
		}
		
		update = msg
		// if props != nil {
		// 	title := props["interlocutor"]
		// 	_, title = decodeInterlocutorInfo(title)
		// 	if title != "" {
		// 		text = "["+ title +"] "+ text
		// 	}
		// }
	//	update = telegram.NewMessage(chatID, text)
	
	case "file":

		doc := message.GetFile()

		switch mimeType := doc.Mime; {
		case strings.HasPrefix(mimeType, "image"):

			// uploadFileURL, err := url.Parse(sendMessage.Url)
			// if err != nil {
			// 	panic("sendFile: "+ err.Error())
			// }

			// channel.Log.Debug().Str("url", uploadFileURL.String()).Msg("sendFile")
			// update = telegram.NewPhotoUpload(chatID, *(uploadFileURL))


			data, err := getBytes(doc.Url)
			if err != nil {
				return err
			}

			file := telegram.FileBytes{
				Name:  doc.Name,
				Bytes: data,
			}

			uploadPhoto := telegram.NewPhotoUpload(chatID, file)
			uploadPhoto.Caption = notify.Message.GetText()

			update = uploadPhoto

		default:

			data, err := getBytes(doc.Url)
			if err != nil {
				return err
			}

			file := telegram.FileBytes{
				Name:  doc.Name,
				Bytes: data,
			}

			update = telegram.NewDocumentUpload(chatID, file)
		}
	
	// case "menu":

	// 	msgAgreement := telegram.NewMessage(chatID, message.Text)

	// 	if message.Type == "buttons" {
			
	// 		msgAgreement.ReplyMarkup = newReplyKeyboard(message.Buttons)

	// 	} else if message.Type == "inline" {

	// 		msgAgreement.ReplyMarkup = newInlineKeyboard(message.Buttons)
	// 	}
		
	// 	update = msgAgreement

	// case "edit":
	// case "send":
	
	// case "read":
	// case "seen":

	// case "kicked":
	case "joined": // ACK: ChatService.JoinConversation()

		// newChatMember := message.NewChatMembers[0]
		// // reply.From = newChatMember.GetFirstName()
		// if props == nil {
		// 	props = make(map[string]string)
		// 	channel.Properties = props // autobind
		// }
		// interlocutor := props["interlocutor"]
		// oid, name := decodeInterlocutorInfo(interlocutor)
		// if oid == 0 && name == "" {
		// 	interlocutor = encodeInterlocutorInfo(
		// 		newChatMember.GetId(), newChatMember.GetFirstName(),
		// 	)
		// 	props["interlocutor"] = interlocutor // CACHE
		// 	bind("interlocutor", interlocutor)   // STORE
			
		// }
		// // if props["joined"] == "" {
		// // 	// STORE changes
		// // 	bind("joined", strconv.FormatInt(newChatMember.Id, 10))
		// // 	bind("titled", newChatMember.GetFirstName())
		// // 	// CACHE changes
		// // 	props["joined"] = binding["joined"]
		// // 	props["titled"] = binding["titled"]
		// // }
		// // setup result binding changed !
		// message.Variables = binding

		return nil // +OK; IGNORE!

	case "left":   // ACK: ChatService.LeaveConversation()

		// leftChatMember := message.LeftChatMember
		// // if reply.From == leftChatMember.GetFirstName() {
		// // 	reply.From = "" // TODO: set default ! FIXME: "bot" ?
		// // }

		// if props != nil {

		// 	interlocutor := props["interlocutor"]
		// 	oid, _ := decodeInterlocutorInfo(interlocutor)

		// 	if oid == leftChatMember.Id {
		// 		delete(props, "interlocutor") // CAHCE
		// 		bind("interlocutor", "")      // STORE
		// 		// setup result binding changed !
		// 		message.Variables = binding
		// 	}
		// }

		// // if props != nil && props["joined"] == strconv.FormatInt(leftChatMember.Id, 10) {
		// // 	// UNBIND !
		// // 	bind("joined", "")
		// // 	bind("titled", "")

		// // 	delete(props, "joined")
		// // 	delete(props, "titled")

		// // 	// setup result binding changed !
		// // 	message.Variables = binding
		// // }

		return nil // +OK; IGNORE!

	// case "typing":
	// case "upload":

	// case "invite":
	case "closed":
		// SEND: notify message text
		text := message.GetText()
		// NOTE: sendMessage.Type = 'close'
		update = telegram.NewMessage(chatID, text)
	
	default:

	}

	if update == nil {
		channel.Log.Warn().
			Str("type", message.Type).
			Str("notice", "reaction not implemented").
			Msg("IGNORE")
		return nil
	}
	
	sentMessage, err := c.BotAPI.Send(update)

	if err != nil {
		switch e := err.(type) {
		case telegram.Error:
			const (
				// HTTP/1.1 403 Forbidden
				// Content-Length: 84
				// Access-Control-Allow-Origin: *
				// Access-Control-Expose-Headers: Content-Length,Content-Type,Date,Server,Connection
				// Connection: keep-alive
				// Content-Type: application/json
				// Date: Fri, 11 Dec 2020 11:13:29 GMT
				// Server: nginx/1.16.1
				// Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
				// 
				// {"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}
				ErrBlockedByUser = "Forbidden: bot was blocked by the user"
				// HTTP/1.1 429 Too Many Requests
				// Content-Length: 109
				// Access-Control-Allow-Origin: *
				// Access-Control-Expose-Headers: Content-Length,Content-Type,Date,Server,Connection
				// Connection: keep-alive
				// Content-Type: application/json
				// Date: Fri, 11 Dec 2020 13:12:39 GMT
				// Retry-After: 1
				// Server: nginx/1.16.1
				// Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
				// 
				// {"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}
				ErrTooManyRequests = "Too Many Requests: " // retry after 1
			)
			// HTTP/1.1 403 Forbidden
			if e.Message == ErrBlockedByUser {
				// DO: .CloseConversation(!) cause: blocked by the user
				// REMOVE: runtime state
				_ = channel.Close() // ("telegram:bot: blocked by the user")
			}
			// HTTP/1.1 429 Too Many Requests
			if strings.HasPrefix(e.Message, ErrTooManyRequests) {
				// TODO: breaker !
			}
		}
		// c.Gateway.Log.Error().Err(err).Msg("Failed to send message")
		return err
	}

	// TARGET[chat_id]: MESSAGE[message_id]
	bind(channel.ChatID, strconv.Itoa(sentMessage.MessageID))
	// sentBindings := map[string]string {
	// 	"chat_id":    channel.ChatID,
	// 	"message_id": strconv.Itoa(sentMessage.MessageID),
	// }
	// attach sent message external bindings
	if message.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		message.Variables = binding
	}
	// +OK
	return nil
}

func getBytes(url string) ([]byte, error) {
	
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func newReplyKeyboard(buttons []*chat.Buttons) telegram.ReplyKeyboardMarkup {

	var rows = make([][]telegram.KeyboardButton, 0)

	for _, v := range buttons {

		var row = make([]telegram.KeyboardButton, 0)

		for _, b := range v.Button {

			if b.Type == "contact"{

				row = append(row, telegram.NewKeyboardButtonContact(b.Text))

			}else if b.Type == "location"{

				row = append(row, telegram.NewKeyboardButtonLocation(b.Text))

			}else if b.Type == "reply"{

				row = append(row, telegram.NewKeyboardButton(b.Text))

			}
		}
		rows = append(rows, row)
	}
	keyboard := telegram.NewReplyKeyboard(rows...)
	keyboard.OneTimeKeyboard = true

	return keyboard
}

func newInlineKeyboard(buttons []*chat.Buttons) telegram.InlineKeyboardMarkup {

	var rows = make([][]telegram.InlineKeyboardButton, 0)

	for _, v := range buttons {

		var row = make([]telegram.InlineKeyboardButton, 0)

		for _, b := range v.Button {

			if b.Type == "url" {
				row = append(row, telegram.NewInlineKeyboardButtonURL(b.Text, b.Url))

			}else if b.Type =="switch" {
				row = append(row, telegram.NewInlineKeyboardButtonSwitch(b.Text, b.Code))

			}else if b.Type =="postback" && b.Code != "" {
				row = append(row, telegram.NewInlineKeyboardButtonData(b.Text, b.Code))
			}
		}
		rows = append(rows, row)
	}

	keyboard := telegram.NewInlineKeyboardMarkup(rows...)
	
	return keyboard
}

// WebHook implementes provider.Receiver interface for Telegram
func (c *TelegramBot) WebHook(reply http.ResponseWriter, notice *http.Request) {

	var recvUpdate telegram.Update
	err := json.NewDecoder(notice.Body).Decode(&recvUpdate)

	if err != nil {
		http.Error(reply, "Failed to decode telegram .Update message", http.StatusBadRequest)
		return // 400 Bad Request
	}

	// region: handle incoming update
	// if recvUpdate.Message != nil {                   // *Message            `json:"message"`
	// } else if recvUpdate.EditedMessage != nil {      // *Message            `json:"edited_message"`
	// } else if recvUpdate.ChannelPost != nil {        // *Message            `json:"channel_post"`
	// } else if recvUpdate.EditedChannelPost != nil {  // *Message            `json:"edited_channel_post"`
	// } else if recvUpdate.InlineQuery != nil {        // *InlineQuery        `json:"inline_query"`
	// } else if recvUpdate.ChosenInlineResult != nil { // *ChosenInlineResult `json:"chosen_inline_result"`
	// } else if recvUpdate.CallbackQuery != nil {      // *CallbackQuery      `json:"callback_query"`
	// } else if recvUpdate.ShippingQuery != nil {      // *ShippingQuery      `json:"shipping_query"`
	// } else if recvUpdate.PreCheckoutQuery != nil {   // *PreCheckoutQuery   `json:"pre_checkout_query"`
	// } else {}
	// endregion

	recvMessage := recvUpdate.Message // NEW (!)
	if recvMessage == nil {
		recvMessage = recvUpdate.EditedMessage // EDITED (!)
	}

	if recvUpdate.CallbackQuery != nil {
		// TODO Button
		recvMessage = recvUpdate.CallbackQuery.Message
		recvMessage.Text = recvUpdate.CallbackQuery.Data

		removeInline := telegram.NewEditMessageReplyMarkup(recvMessage.Chat.ID, recvMessage.MessageID, telegram.InlineKeyboardMarkup {
			InlineKeyboard: [][]telegram.InlineKeyboardButton{},
		})
		
		_, err := c.BotAPI.Send(removeInline)

		if err != nil {
			c.Gateway.Log.Warn().

				Str("Error ", err.Error()).
				Msg("Failed to remove Inline Keyboard .Telegram")
		}
	}
	
	if recvMessage == nil {
		// NOTE: this is NOT either NEW nor EDIT message update; skip processing ...
		// Quick Release Request !
		code := http.StatusOK // 200
		reply.WriteHeader(code)

		c.Gateway.Log.Warn().

			Int("code", code).
			Str("status", http.StatusText(code)).
			Str("notice", "Update is NOT either NEW nor EDIT Message").

			Msg("IGNORE; NOT a Message Update")

		return
	}
	
	
	
	
	
	
	
	// var recvMessage *telegram.Message

	// if recvUpdate.Message != nil {
	// 	recvMessage = recvUpdate.Message

	// } else if recvUpdate.EditedMessage != nil {
	// 	recvMessage = recvUpdate.EditedMessage
	// 	c.Gateway.Log.Warn().

	// 		Int(  "telegram-id", recvMessage.From.ID).
	// 		Str(  "username",    recvMessage.From.UserName).
	// 		Int64("chat-id",     recvMessage.Chat.ID).
	// 		// Str("first_name", message.From.FirstName).
	// 		// Str("last_name",  message.From.LastName)

	// 	Msg("IGNORE Update; NOT A Text Message")
			
	// 	return // 200 IGNORE

	// } else if recvUpdate.CallbackQuery != nil {
	// 	// TODO Button
	// 	return // 200 IGNORE
	// }

	// // if recvMessage != recvUpdate.Message {
		
	// // 	c.Gateway.Log.Warn().

	// // 		Int(  "telegram-id", recvMessage.From.ID).
	// // 		Str(  "username",    recvMessage.From.UserName).
	// // 		Int64("chat-id",     recvMessage.Chat.ID).
	// // 		// Str("first_name", message.From.FirstName).
	// // 		// Str("last_name",  message.From.LastName)

	// // 	Msg("IGNORE Update; NOT A Text Message")
		
	// // 	return // 200 IGNORE
	// // }

	// sender: user|chat
	senderUser := recvMessage.From
	senderChat := recvMessage.Chat

	// region: contact
	contact := &bot.Account{
		ID:        0, // LOOKUP
		Channel:   "telegram",
		Contact:   strconv.Itoa(senderUser.ID),

		FirstName: senderUser.FirstName,
		LastName:  senderUser.LastName,
		Username:  senderUser.UserName,
	}

	// username := recvMessage.From.FirstName
	// if username != "" && recvMessage.From.LastName != "" {
	// 	username += " " + recvMessage.From.LastName
	// }

	// if username == "" {
	// 	username = recvMessage.From.UserName
	// }
	// endregion

	// region: channel
	chatID := strconv.FormatInt(senderChat.ID, 10)
	channel, err := c.Gateway.GetChannel(
		notice.Context(), chatID, contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err); if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway) 
			// HTTP 503 Bad Gateway
		}
		// FIXME: Reply with 200 OK to NOT receive this message again ?!.
		reply := telegram.NewMessage(senderChat.ID, re.Detail)
		defer func() {
			_, _ = c.BotAPI.Send(reply)
		} ()
		// http.Error(reply, re.Detail, (int)(re.Code))
		return // HTTP 200 OK; WITH reply error message
	}

	// channel.Title = sender.Title
	// contact.ID = channel.ContactID

	// endregion
	sendUpdate := bot.Update {
		
		// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),
		
		User:    contact,
		Chat:    channel,
		Title:   channel.Title,

		Message: new(chat.Message),
	}

	sendMessage := sendUpdate.Message

	// region: handle message
	// if recvMessage.Document != nil {        // *Document    `json:"document"`
	// } else if recvMessage.Photo != nil {    // *[]PhotoSize `json:"photo"`
	// } else if recvMessage.Audio != nil {    // *Audio       `json:"audio"`
	// } else if recvMessage.Video != nil {    // *Video       `json:"video"`
	// } else if recvMessage.Text != "" {      // string       `json:"text"`
	// } else {}
	// endregion

	if recvMessage.Document != nil {

		doc := recvMessage.Document
		URL, err := c.BotAPI.GetFileDirectURL(doc.FileID)
		if err != nil {
			// FIXME: respond with 200 OK ?
			return
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File {
			Url:  URL,
			Size: (int64)(doc.FileSize),
			Mime: doc.MimeType,
			Name: doc.FileName,
		}
		sendMessage.Text = recvMessage.Caption

	} else if recvMessage.Photo != nil {

		const (
			// 20 Mb = 1024 Kb * 1024 b
			fileSizeMax = 20 * 1024 * 1024
		)
		// Message is a photo, available sizes of the photo
		photos := *recvMessage.Photo
		// Lookup for suitable file size for bot to download ...
		e := len(photos)-1 // From biggest to smallest ...
		for ; e >= 0 && photos[e].FileSize > fileSizeMax; e-- {
			// omit files that are too large,
			// which will result in a download error
		}
		if e < 0 {
			e = 0 // restoring the previous logic
		}
		// Peek the biggest, last one ...
		photo := telegram.FileConfig{
			FileID: photos[e].FileID,
		}
		
		doc, err := c.BotAPI.GetFile(photo)
		if err != nil {
			// FIXME: respond with 200 OK ?
			return
		}
		// Get filename from available filepath
		name := filepath.Base(doc.FilePath)
		switch name {
		case "/", ".": // unknown(!)
			name = ""
		}
		// Get URL available for our bot profile
		URL := doc.Link(c.Token)
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File {
			Url:  URL,
			Size: (int64)(doc.FileSize),
			Mime: "image/jpg",
			Name: name,
		}
		sendMessage.Text = recvMessage.Caption

	} else if recvMessage.Audio != nil {

		doc := recvMessage.Audio
		URL, err := c.BotAPI.GetFileDirectURL(doc.FileID)
		if err != nil {
			// FIXME: respond with 200 OK ?
			return
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File {
			Url:  URL,
			Size: (int64)(doc.FileSize),
			Mime: doc.MimeType,
			Name: doc.Title,
		}
		sendMessage.Text = recvMessage.Caption
	
	} else if recvMessage.Contact != nil {

		sendUpdate.Message = &chat.Message {
			Type: "contact",
			Contact: &chat.Account {
				Contact: recvMessage.Contact.PhoneNumber,
				FirstName: recvMessage.Contact.FirstName,
				LastName: recvMessage.Contact.LastName,
				Id: int64(recvMessage.Contact.UserID),
			},
		}

	} else if recvMessage.Video != nil {

		doc := recvMessage.Video
		cfg := telegram.FileConfig{
			FileID: doc.FileID,
		}

		file, _ := c.BotAPI.GetFile(cfg)
		title := filepath.Base(file.FilePath)

		URL := file.Link(c.Token)

		sendUpdate.Message = &chat.Message{
			Type: "file",
			File: &chat.File{
				Url:  URL,
				Name: title,
				Mime: doc.MimeType,
			},
			Text: recvMessage.Caption,
		}

	} else if recvMessage.Text != "" {
		// Prepare internal message content
		sendMessage.Type = "text"
		sendMessage.Text = recvMessage.Text

	} else {
		// ACK: HTTP/1.1 200 OK
		code := http.StatusOK
		reply.WriteHeader(code)
		// IGNORE: not applicable yet !
		channel.Log.Warn().
			Str("notice", "message: is NOT a text, photo, audio, video or file document").
			Msg("IGNORE")
		
		return
	}
	// EDITED ?
	if (recvMessage == recvUpdate.EditedMessage) {
		var (
			timestamp = time.Second       //      seconds = 1e9
			precision = app.TimePrecision // milliseconds = 1e6
		)
		sendMessage.UpdatedAt = 
			(int64)(recvMessage.EditDate)*(int64)(timestamp/precision)
	}

	// TODO: ForwardFromMessageID | ReplyToMessageID !
	if recvMessage.ForwardFromMessageID != 0 {

		// sendMessage.ForwardFromMessageId = recvMessage.ForwardFromMessageID
		sendMessage.ForwardFromVariables = map[string]string{
			// FIXME: guess, this can by any telegram-user-related chat,
			//        so we may fail to find corresponding internal message for given binding map
			strconv.FormatInt(recvMessage.ForwardFromChat.ID, 10):
				strconv.Itoa(recvMessage.ForwardFromMessageID),
			// "chat_id":    strconv.FormatInt(recvMessage.ForwardFromChat.ID, 10),
			// "message_id": strconv.Itoa(recvMessage.ForwardFromMessageID),
		}

	} else if recvMessage.ReplyToMessage != nil {

		// sendMessage.ReplyToMessageId = recvMessage.ReplyToMessage.MessageID
		sendMessage.ReplyToVariables = map[string]string{
			// FIXME: the same chatID ? Is it correct ?
			chatID: strconv.Itoa(recvMessage.ReplyToMessage.MessageID),
			// "chat_id":    chatID,
			// "message_id": strconv.Itoa(recvMessage.ReplyToMessage.MessageID),
		}

	}
	sendMessage.Variables = map[string]string{
		chatID: strconv.Itoa(recvMessage.MessageID),
		// "chat_id":    chatID,
		// "message_id": strconv.Itoa(recvMessage.MessageID),
	}

	err = c.Gateway.Read(notice.Context(), &sendUpdate)

	if err != nil {

		code := http.StatusInternalServerError
		http.Error(reply, "Failed to deliver telegram .Update message", code)
		return // 502 Bad Gateway
	}

	code := http.StatusOK
	reply.WriteHeader(code)
	// return // HTTP/1.1 200 OK
}

// func receiveMessage(e *telegram.Message) {}

// func receiveEditedMessage(e *telegram.Message) {}