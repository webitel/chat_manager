package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"net/http"

	"encoding/json"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot/telegram/internal/builder"
	"github.com/webitel/chat_manager/bot/telegram/internal/helper"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown"
)

const (
	provider = "telegram"
)

func init() {
	bot.Register(provider, NewTelegramBot)
}

// Telegram BOT chat provider
type TelegramBot struct {
	*bot.Gateway
	*telegram.BotAPI
	contacts map[int64]*bot.Account
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

	// Parse and validate message templates
	var err error
	agent.Template = bot.NewTemplate(provider)
	// Populate telegram-specific markdown-escape helper funcs
	agent.Template.Root().Funcs(
		markdown.TemplateFuncs,
	)
	// Parse message templates
	if err = agent.Template.FromProto(
		agent.Bot.GetUpdates(),
	); err == nil {
		// Quick tests ! <nil> means default (well-known) test cases
		err = agent.Template.Test(nil)
	}
	if err != nil {
		return nil, errors.BadRequest(
			"chat.bot.telegram.updates.invalid",
			err.Error(),
		)
	}

	var (
		botAPI     *telegram.BotAPI
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
			WithBody:  true,
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
		botAPI, err = telegram.NewBotAPIWithClient(
			token, telegram.APIEndpoint, httpClient,
		)
	}

	if err != nil {
		switch e := err.(type) {
		case *telegram.Error:
			if e.Code == 404 {
				err = fmt.Errorf("bot API token is invalid")
			}
		}
		return nil, errors.New(
			"chat.bot.telegram.setup.error",
			"telegram: "+err.Error(),
			http.StatusBadGateway,
		)
	}

	return &TelegramBot{
		Gateway:  agent,
		BotAPI:   botAPI,
		contacts: make(map[int64]*bot.Account),
	}, nil
}

func (*TelegramBot) Close() error {
	return nil
}

// String "telegram" provider's name
func (*TelegramBot) String() string {
	return provider
}

// Register Telegram Bot Webhook endpoint URI
func (c *TelegramBot) Register(ctx context.Context, callbackURL string) error {

	// // webhookInfo := tgbotapi.NewWebhookWithCert(fmt.Sprintf("%s/telegram/%v", cfg.TgWebhook, profile.Id), cfg.CertPath)
	// linkURL := strings.TrimRight(c.Gateway.Internal.URL, "/") +
	// 	("/" + c.Gateway.Profile.UrlId)

	webhook, err := telegram.NewWebhook(callbackURL)
	if err != nil {
		c.Gateway.Log.Error("Failed to .Register webhook",
			slog.Any("error", err),
		)
		return err
	}

	_, err = c.BotAPI.Request(webhook)
	// _, err := c.BotAPI.SetWebhook(webhook)

	if err != nil {
		c.Gateway.Log.Error("Failed to .Register webhook",
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// Deregister Telegram Bot Webhook endpoint URI
func (c *TelegramBot) Deregister(ctx context.Context) error {
	// POST /deleteWebhook
	req := telegram.DeleteWebhookConfig{
		DropPendingUpdates: false,
	}

	res, err := c.BotAPI.Request(req)
	// res, err := c.BotAPI.RemoveWebhook()

	if err != nil {
		// err.(telegram.Error)
		return err
	}

	if !res.Ok {
		return errors.New(
			"chat.bot.telegram.deregister.error",
			"telegram: "+res.Description,
			(int32)(res.ErrorCode), // FIXME: 502 Bad Gateway ?
		)
	}

	return nil
}

// SendNotify implements provider.Sender interface for Telegram
func (c *TelegramBot) SendNotify(ctx context.Context, notify *bot.Update) error {

	var (
		channel = notify.Chat    // recepient
		message = notify.Message // message to send

		binding map[string]string           // variables map
		bind    = func(key, value string) { // set variable to map
			if binding == nil {
				binding = make(map[string]string)
			}
			binding[key] = value
		}
	)

	// REGION: recover latest chat channel state
	chatID, err := strconv.ParseInt(channel.ChatID, 10, 64)
	if err != nil {
		c.Log.Error("TELEGRAM: SEND",
			slog.String("error", "invalid chat "+channel.ChatID+" integer identifier"),
		)
		return errors.InternalServerError(
			"chat.gateway.telegram.chat.id.invalid",
			"telegram: invalid chat %s unique identifier; expect integer values", channel.ChatID)
	}

	if channel.Title == "" {
		// FIXME: .GetChannel() does not provide full contact info on recover,
		//                      just it's unique identifier ...  =(
	}

	// Create message builder
	sendMessageBuilder := builder.NewSendMessageBuilder()

	// Set chat ID to message
	err = sendMessageBuilder.SetChatID(chatID)
	if err != nil {
		return err
	}

	// TODO: resolution for various notify content !
	switch message.Type { // notify.Event {
	case "text": // default
		err := sendMessageBuilder.SetText(message.GetText())
		if err != nil {
			return err
		}

	case "file":
		file := message.GetFile()

		err := sendMessageBuilder.SetFile(file.GetName(), file.GetMime(), file.GetUrl(), message.GetText())
		if err != nil {
			return err
		}

	case "joined": // ACK: ChatService.JoinConversation()
		peer := message.NewChatMembers[0]
		updates := c.Gateway.Template
		text, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Error(
				"telegram/bot.updateChatMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		// IGNORE: message text is missing
		if text == "" {
			return nil
		}

		// Format new message to the engine for saving it in the DB as operator message [WTEL-4695]
		messageToSave := &chat.Message{
			Type:      "text",
			Text:      text,
			CreatedAt: time.Now().UnixMilli(),
			From:      peer,
		}
		if channel != nil && channel.ChannelID != "" {
			_, err = c.Gateway.Internal.Client.SendServiceMessage(
				ctx, &chat.SendServiceMessageRequest{
					Message:  messageToSave,
					Receiver: channel.ChannelID,
				},
			)
			if err != nil {
				return err
			}
		}

		// Set text to message
		err = sendMessageBuilder.SetText(text)
		if err != nil {
			return err
		}

	case "left": // ACK: ChatService.LeaveConversation()
		peer := message.LeftChatMember
		updates := c.Gateway.Template
		text, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Error(
				"telegram/bot.updateLeftMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}

		// IGNORE: message text is missing
		if text == "" {
			return nil
		}

		// Set text to message
		err = sendMessageBuilder.SetText(text)
		if err != nil {
			return err
		}

	case "closed":
		updates := c.Gateway.Template
		text, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Error(
				"telegram/bot.updateChatClose",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}

		// IGNORE: message text is missing
		if text == "" {
			return nil
		}

		// Set text to message
		err = sendMessageBuilder.SetText(text)
		if err != nil {
			return err
		}

		// Remove keyboard
		err = sendMessageBuilder.SetRemoveKeyboard()
		if err != nil {
			return err
		}

	// Not implemented
	// case "typing":
	// case "upload":
	// case "invite":
	// case "edit":
	// case "send":
	// case "read":
	// case "seen":
	// case "kicked":

	default:
		// Pass
	}

	// Inline: Next to this specific message ONLY !
	// Reply:  Persistent keyboard buttons, under the input ! (Location, Contact, Persistent Text postback)
	buttons := message.GetButtons()
	if len(buttons) > 0 {
		// NOT <nil> BUT <zero>: designed to clear all persistent menu (keyboard buttons)
		// Prepare SEND Telegram (Inline|Reply)Keyboard(Markup|Remove)
		// https://core.telegram.org/bots/api#sendmessage
		// https://core.telegram.org/bots/api#updating-messages
		err := sendMessageBuilder.SetMergedKeyboard(buttons)
		if err != nil {
			return err
		}
	}

	// Build message data with action
	data, action, err := sendMessageBuilder.BuildWithAction()
	if err != nil {
		return err
	}

	// Check data is nil
	if data == nil {
		channel.Log.Warn(
			"TELEGRAM: SEND",
			slog.String("send", message.Type),
			slog.String("error", "reaction not implemented"),
		)
		return nil
	}

	// Send message logic
	var sentMessage telegram.Message

retry:
	if action != nil {
		_, err = c.BotAPI.Request(action)
	}

	if err == nil {
		sentMessage, err = c.BotAPI.Send(data)
	}

	if err != nil {
		channel.Log.Error("TELEGRAM: SEND",
			slog.Any("error", err),
		)
		switch e := err.(type) {
		case *telegram.Error:
			switch e.Code {
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
			case 403:
				{
					_ = channel.Close() // ("telegram:bot: blocked by the user")
				}
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
			case 429, 420: // 420 FLOOD_WAIT
				floodWait := time.Duration(e.RetryAfter) * time.Second
				if floodWait > 0 {
					select {
					case <-ctx.Done():
						err = ctx.Err()
						break
					case <-time.After(floodWait):
						goto retry
					}
				}
			}
		}
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

// GetFile is a shorthand for c.BotAPI.GetFile() with some extra .File methods
func (c *TelegramBot) GetFile(fileID string) (helper.File, error) {
	file, err := c.BotAPI.GetFile(
		telegram.FileConfig{
			FileID: fileID,
		},
	)
	if err != nil {
		c.Log.Error("TELEGRAM: FILE",
			slog.Any("error", err),
			slog.String("file-id", fileID),
		)
	}
	return helper.File(file), err
}

func (c *TelegramBot) GetFileDirectURL(fileID string) (string, error) {
	href, err := c.BotAPI.GetFileDirectURL(fileID)
	if err != nil {
		c.Log.Error("TELEGRAM: FILE",
			slog.Any("error", err),
			slog.String("file-id", fileID),
		)
	}
	return href, err
}

// WebHook implementes provider.Receiver interface for Telegram
func (c *TelegramBot) WebHook(reply http.ResponseWriter, notice *http.Request) {

	switch notice.Method {
	case http.MethodPost:
		if notice.Body != nil {
			defer notice.Body.Close()
		}
	// // GET ?query= API extensions
	// case http.MethodGet:
	default:
		// Method Not Allowed !
		http.Error(reply,
			"(405) Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	var recvUpdate telegram.Update
	err := json.NewDecoder(notice.Body).Decode(&recvUpdate)

	if err != nil {
		http.Error(reply, "Failed to decode telegram .Update event", http.StatusBadRequest)
		c.Log.Error("TELEGRAM: UPDATE",
			slog.String("error", "telegram.Update: "+err.Error()),
		)
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

	// Optional. The bot's chat member status was updated in a chat.
	// For private chats, this update is received only
	// when the bot is blocked or unblocked by the user.
	if e := recvUpdate.MyChatMember; e != nil {
		c.onMyChatMember(notice.Context(), e) // hook
		code := http.StatusOK
		reply.WriteHeader(code)
		return // (200) OK
	}

	recvMessage := recvUpdate.Message // SENT NEW (!)
	if recvMessage == nil {
		recvMessage = recvUpdate.EditedMessage // EDITED (!)
	}

	// FIXME: TODO !!!
	if recvUpdate.CallbackQuery != nil {
		// Button has been pressed ! callback ..
		sentMessage := *recvUpdate.CallbackQuery.Message // snap
		sentMessage.Text = recvUpdate.CallbackQuery.Data
		sentMessage.From = recvUpdate.CallbackQuery.From
		recvMessage = &sentMessage
		// NOTE:
		// callback_query.from => is our recepient (*tg.User) !
		// callback_query.message.from => is our bot account,
		//   as the original sender of the message with buttons !

		inlineKeyboardRemove := telegram.NewEditMessageReplyMarkup(
			recvMessage.Chat.ID, recvMessage.MessageID,
			telegram.InlineKeyboardMarkup{
				InlineKeyboard: [][]telegram.InlineKeyboardButton{},
			},
		)

		if _, err = c.BotAPI.Send(inlineKeyboardRemove); err != nil {
			c.Log.Warn("TELEGRAM: INLINE",
				slog.String("error", "InlineKeyboardRemove: "+err.Error()),
			)
		}

	}

	if recvMessage == nil {
		// NOTE: this is NOT either NEW nor EDIT message update; skip processing ...
		// Quick Release Request !
		code := http.StatusOK // 200
		reply.WriteHeader(code)

		c.Gateway.Log.Warn("TELEGRAM: IGNORE",
			slog.Int("code", code),
			slog.String("status", http.StatusText(code)),
			slog.String("notice", "Update event is NOT either NEW nor EDIT Message"),
		)

		return
	}

	// sender: user|chat
	sender := recvMessage.From
	dialog := recvMessage.Chat
	dialogId := strconv.FormatInt(dialog.ID, 10)

	channel, err := c.getChannel(
		notice.Context(), *(dialog),
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
			// HTTP 503 Bad Gateway
		}
		// FIXME: Reply with 200 OK to NOT receive this message again ?!.
		_ = telegram.WriteToHTTPResponse(
			reply, telegram.NewMessage(dialog.ID, re.Detail),
		)
		// reply := telegram.NewMessage(senderChat.ID, re.Detail)
		// defer func() {
		// 	_, _ = c.BotAPI.Send(reply)
		// } ()
		// // http.Error(reply, re.Detail, (int)(re.Code))
		return // HTTP 200 OK; WITH reply error message
	}

	// channel.Title = sender.Title
	// contact.ID = channel.ContactID

	// endregion
	sendUpdate := bot.Update{

		// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),

		// User:  contact,
		Chat:  channel,
		Title: channel.Title,
		User:  &channel.Account,

		Message: new(chat.Message),
	}

	sendMessage := sendUpdate.Message

	coalesce := func(argv ...string) string {
		for _, s := range argv {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
		return ""
	}

	// region: handle message
	// if recvMessage.Document != nil {        // *Document    `json:"document"`
	// } else if recvMessage.Photo != nil {    // *[]PhotoSize `json:"photo"`
	// } else if recvMessage.Audio != nil {    // *Audio       `json:"audio"`
	// } else if recvMessage.Video != nil {    // *Video       `json:"video"`
	// } else if recvMessage.Text != "" {      // string       `json:"text"`
	// } else {}
	// endregion

	if callback := recvUpdate.CallbackQuery; callback != nil {
		// Prepare internal message content
		//
		// Data associated with the callback button.
		sendMessage.Type = "text"
		sendMessage.Text = callback.Data
		// Be aware that a bad client can send arbitrary data in this field.
		if sendMessage.Text == "" {
			sendMessage.Text = "#callback"
		}

	} else if animation := recvMessage.Animation; animation != nil {

		// Message is an animation, information about the animation.
		// For backward compatibility, when this field is set,
		// the document field will also be set

		href, err := c.GetFileDirectURL(animation.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  href, // source URL to download from ...
			Mime: animation.MimeType,
			Name: animation.FileName,
			Size: (int64)(animation.FileSize),
		}
		// Optional. Caption for the animation
		sendMessage.Text = recvMessage.Caption

	} else if document := recvMessage.Document; document != nil {

		href, err := c.GetFileDirectURL(document.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  href, // source URL to download from ...
			Size: (int64)(document.FileSize),
			Mime: document.MimeType,
			Name: document.FileName,
		}
		// Optional. Caption for the document
		sendMessage.Text = recvMessage.Caption

	} else if photo := recvMessage.Photo; len(photo) != 0 {

		const (
			// 20 Mb = 1024 Kb * 1024 b
			fileSizeMax = 20 * 1024 * 1024
		)
		// Message is a photo, available sizes of the photo
		// Lookup for suitable file size for bot to download ...
		// Peek the biggest, last one ...
		i := len(photo) - 1 // From biggest to smallest ...
		for ; i >= 0 && photo[i].FileSize > fileSizeMax; i-- {
			// omit files that are too large,
			// which will result in a download error
		}
		if i < 0 {
			i = 0 // restoring the previous logic: the smallest one !..
		}

		image, err := c.GetFile(photo[i].FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}

		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  image.Link(c.BotAPI.Token), // source URL to download from ...
			Mime: "",                         // autodetect on chat's service .SendMessage()
			// mime.TypeByExtension(path.Ext(image.FileName()))
			// "image/jpg",
			Name: image.FileName(),
			Size: (int64)(image.FileSize),
		}
		// Optional. Caption for the photo
		sendMessage.Text = recvMessage.Caption

	} else if audio := recvMessage.Audio; audio != nil {

		href, err := c.GetFileDirectURL(audio.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  href, // source URL to download from ...
			Size: (int64)(audio.FileSize),
			Mime: audio.MimeType,
			Name: audio.FileName,
		}
		// Optional. Caption for the audio
		sendMessage.Text = coalesce(
			recvMessage.Caption, audio.Title, // "Audio",
		)

	} else if voice := recvMessage.Voice; voice != nil {

		file, err := c.GetFile(voice.FileID)
		// href, err := c.GetFileDirectURL(voice.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  file.Link(c.BotAPI.Token), // source URL to download from ...
			Mime: voice.MimeType,
			Name: file.FileName(),
			Size: (int64)(voice.FileSize),
		}
		// Optional. Caption for the voice
		sendMessage.Text = coalesce(
			recvMessage.Caption, // "Voice",
		)

	} else if video := recvMessage.Video; video != nil {

		href, err := c.GetFileDirectURL(video.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  href, // source to download
			Mime: video.MimeType,
			Name: video.FileName,
			Size: (int64)(video.FileSize),
		}
		// Optional. Caption for the video
		sendMessage.Text = coalesce(
			recvMessage.Caption, // "Video",
		)

	} else if videoNote := recvMessage.VideoNote; videoNote != nil {

		file, err := c.GetFile(videoNote.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}

		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  file.Link(c.BotAPI.Token), // source URL to download from ...
			Mime: "",                        // autodetect // "video/mp4", // videoNote.MimeType,
			Name: file.FileName(),
			Size: (int64)(videoNote.FileSize),
		}
		// FIXME: NOT declared for videoNote !
		sendMessage.Text = coalesce(
			recvMessage.Text, recvMessage.Caption, // "Video Note",
		)

	} else if location := recvMessage.Location; location != nil {

		// FIXME: Google Maps Link to Place with provided coordinates !
		sendMessage.Type = "text"
		sendMessage.Text = fmt.Sprintf(
			"https://www.google.com/maps/place/%f,%f",
			location.Latitude, location.Longitude,
		)

	} else if sticker := recvMessage.Sticker; sticker != nil {

		// NOTE: sticker.FileID provide .tgs The [T]ele[g]ram [S]ticker File Format https://docs.fileformat.com/compression/tgs/
		// So we download and forward the sticker's .Thumbnail image to display !

		thumb, err := c.GetFile(sticker.Thumbnail.FileID)
		if err != nil {
			code := http.StatusOK // 200 OK (IGNORE REDELIVERY)
			switch re := err.(type) {
			case telegram.Error:
				// Failed to GET file !
				// FIXME: Forward error back to Telegram ?
				code = re.Code
			default:
				// JSON Decode errors ! Models (version) mismatched ?
			}
			// FIXME
			http.Error(reply, err.Error(), code)
			c.Log.Error("TELEGRAM: FILE",
				slog.Any("error", err),
				slog.Int("code", code),
				slog.String("status", http.StatusText(code)),
			)
			return // IGNORE Update !
		}
		// Prepare internal message content
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url:  thumb.Link(c.BotAPI.Token), // source to download
			Mime: "",                         // autodetect // "image/jpeg", // sticker.MimeType,
			Name: thumb.FileName(),           // sticker.SetName, // sticker.FileName,
			Size: (int64)(sticker.FileSize),
		}
		// FIXME !
		sendMessage.Text = coalesce(
			sticker.Emoji, sticker.SetName, // "Sticker",
		)

	} else if contact := recvMessage.Contact; contact != nil {

		// NOTE: Client may share any contact from it's contact book
		// This is not always it's own phone number !

		sendMessage.Type = "contact"
		// sendMessage.Text = contact.PhoneNumber
		sendMessage.Contact = &chat.Account{
			Id:        0, // int64(contact.UserID),
			Channel:   "phone",
			Contact:   contact.PhoneNumber,
			FirstName: contact.FirstName,
			LastName:  contact.LastName,
		}

		if contact.UserID == sender.ID {
			sendMessage.Contact.Id = channel.Account.ID // MARK: sender:owned
		}

		contactName := strings.TrimSpace(strings.Join(
			[]string{contact.FirstName, contact.LastName}, " ",
		))

		if contactName != "" {
			// SIP -like AOR ...
			contactName = "<" + contactName + ">"
		}

		contactText := strings.TrimSpace(strings.Join(
			[]string{contactName, contact.PhoneNumber}, " ",
		))
		// Contact: [<.FirstName[ .LastName]> ].PhoneNumber
		sendMessage.Text = contactText

	} else if recvMessage.Text != "" {
		// Prepare internal message content
		sendMessage.Type = "text"
		sendMessage.Text = recvMessage.Text

	} else {
		// ACK: HTTP/1.1 200 OK
		code := http.StatusOK
		reply.WriteHeader(code)
		// IGNORE: not applicable yet !
		channel.Log.Warn("TELEGRAM: UPDATE",
			slog.String("notice", "message: is NOT a text, photo, audio, video or document"),
		)

		return
	}
	// EDITED ?
	if recvMessage == recvUpdate.EditedMessage {
		var (
			timestamp = time.Second       //      seconds = 1e9
			precision = app.TimePrecision // milliseconds = 1e6
		)
		sendMessage.UpdatedAt =
			(int64)(recvMessage.EditDate) * (int64)(timestamp/precision)
	}

	// TODO: ForwardFromMessageID | ReplyToMessageID !
	if recvMessage.ForwardFromMessageID != 0 {

		// sendMessage.ForwardFromMessageId = recvMessage.ForwardFromMessageID
		sendMessage.ForwardFromVariables = map[string]string{
			// FIXME: guess, this can by any telegram-user-related chat,
			//        so we may fail to find corresponding internal message for given binding map
			strconv.FormatInt(recvMessage.ForwardFromChat.ID, 10): strconv.Itoa(recvMessage.ForwardFromMessageID),
			// "chat_id":    strconv.FormatInt(recvMessage.ForwardFromChat.ID, 10),
			// "message_id": strconv.Itoa(recvMessage.ForwardFromMessageID),
		}

	} else if recvMessage.ReplyToMessage != nil {

		// sendMessage.ReplyToMessageId = recvMessage.ReplyToMessage.MessageID
		sendMessage.ReplyToVariables = map[string]string{
			// FIXME: the same chatID ? Is it correct ?
			dialogId: strconv.Itoa(recvMessage.ReplyToMessage.MessageID),
			// "chat_id":    chatID,
			// "message_id": strconv.Itoa(recvMessage.ReplyToMessage.MessageID),
		}

	}
	sendMessage.Variables = map[string]string{
		dialogId: strconv.Itoa(recvMessage.MessageID),
		// "chat_id":    chatID,
		// "message_id": strconv.Itoa(recvMessage.MessageID),
	}
	if channel.IsNew() { // && contact.Username != "" {
		sendMessage.Variables["username"] = sender.UserName // contact.Username
		splitted := strings.Split(recvMessage.Text, " ")
		if len(splitted) > 1 {
			sendMessage.Variables["ref"] = splitted[1]
		}
	}

	err = c.Gateway.Read(notice.Context(), &sendUpdate)

	if err != nil {

		code := http.StatusInternalServerError
		http.Error(reply, "Failed to forward .Update message", code)
		return // 502 Bad Gateway
	}

	code := http.StatusOK
	reply.WriteHeader(code)
	// return // HTTP/1.1 200 OK
}

// Broadcast given `req.Message` message [to] provided `req.Peer(s)`
func (c *TelegramBot) BroadcastMessage(ctx context.Context, req *pbbot.BroadcastMessageRequest, rsp *pbbot.BroadcastMessageResponse) error {

	var (
		setError = func(peerId int, err error) { // set error to response
			res := rsp.GetFailure()
			if res == nil {
				res = make([]*pbbot.BroadcastPeer, 0, len(req.GetPeer()))
			}

			var re *status.Status
			switch err := err.(type) {
			case *telegram.Error:
				re = status.New(codes.Code(err.Code), err.Message)
			case *errors.Error:
				re = status.New(codes.Code(err.Code), err.Detail)
			default:
				re = status.New(codes.Unknown, err.Error())
			}

			res = append(res, &pbbot.BroadcastPeer{
				Peer:  req.Peer[peerId],
				Error: re.Proto(),
			})

			rsp.Failure = res
		}

		setVariable = func(key, value string) { // set variable to response
			if rsp.GetVariables() == nil {
				rsp.Variables = make(map[string]string)
			}
			rsp.Variables[key] = value
		}
	)

	// Get message params from request
	message := req.GetMessage()

	// Sorting out users for sending messages
	for i, peer := range req.GetPeer() {

		// Conversion peer to chat ID
		chatID, err := strconv.ParseInt(peer, 10, 64)
		if err != nil {
			setError(i, errors.BadRequest("", "chat.id: expect integer identifier"))
			continue
		}

		// Start build message for user
		sendMessageBuilder := builder.NewSendMessageBuilder()

		// Set user chat ID to message
		err = sendMessageBuilder.SetChatID(chatID)
		if err != nil {
			setError(i, err)
			continue
		}

		// Set text or file to message
		switch message.GetType() {
		case "text":
			{
				err := sendMessageBuilder.SetText(
					message.GetText(),
				)
				if err != nil {
					setError(i, err)
				}
			}
		case "file":
			{
				file := message.GetFile()
				err := sendMessageBuilder.SetFile(
					file.GetName(), file.GetMime(), file.GetUrl(), message.GetText(),
				)
				if err != nil {
					setError(i, err)
				}
			}
		}

		// Set keyboad to message
		// Important: inline is a priority
		buttons := message.GetButtons()
		if len(buttons) > 0 {
			err := sendMessageBuilder.SetMergedKeyboard(buttons)
			if err != nil {
				setError(i, err)
			}
		}

		// Build message data with action
		data, action, err := sendMessageBuilder.BuildWithAction()
		if err != nil {
			setError(i, err)
			continue
		}

		// Send message to chat with user
		err = c.doTelegramAPI(ctx, func() error {

			if action != nil {
				_, err = c.BotAPI.Request(action)
				if err != nil {
					return err
				}
			}

			resp, err := c.BotAPI.Send(data)
			if err != nil {
				return err
			}

			if message.Id != 0 {
				setVariable(peer, strconv.Itoa(resp.MessageID))
			}

			return nil
		})

		if err != nil {
			setError(i, err)
		}
	}

	return nil
}

func (c *TelegramBot) SendUserAction(ctx context.Context, peerId string, action chat.UserAction) (ok bool, err error) {

	var chatAction string
	switch action {
	case chat.UserAction_Typing:
		chatAction = telegram.ChatTyping
	default:
		// case chat.UserAction_Cancel:
	}

	if chatAction == "" {
		c.Log.Warn("telegram.bot.sendChatAction",
			slog.String("chat_id", peerId),
			slog.String("action", fmt.Sprintf("(%d) %[1]s", action)),
			slog.String("error", "no such [re]action"),
		)
		return // false, err
	}

	chatId, err := strconv.ParseInt(peerId, 10, 64)
	if err != nil {
		// ERR: Peer NOT Acceptable !
		err = errors.BadRequest(
			"chat.telegram.peer.id.invalid",
			"telegram: invalid chat_id=%s input",
			peerId,
		)
		return // false, err
	}

	sendMessage := telegram.NewChatAction(
		chatId, chatAction,
	)

	var sentMessage *telegram.APIResponse
	sentMessage, err = c.BotAPI.Request(sendMessage)

	if err != nil {
		return // false, err
	}

	ok = sentMessage.Ok
	return // true, nil
}

func (c *TelegramBot) doTelegramAPI(ctx context.Context, callback func() error) error {
	return c.do(callback, func(err error) bool {
		switch e := err.(type) {
		case *telegram.Error:
			switch e.Code {
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
			case 403:
				return false
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
			case 429, 420:
				floodwait := time.Duration(e.RetryAfter) * time.Second
				if floodwait > 0 {
					select {
					case <-ctx.Done():
						return false
					case <-time.After(floodwait):
						return true
					}
				}
			}
		}

		return false
	})
}

func (c *TelegramBot) do(callback func() error, handler func(error) bool) error {
	for {
		err := callback()
		if err == nil || !handler(err) {
			return err
		}
	}
}
