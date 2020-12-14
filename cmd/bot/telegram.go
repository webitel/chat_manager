package main

import (
	"github.com/micro/go-micro/v2/errors"
	"strconv"
	"encoding/json"
	
	"context"
	"strings"
	"net/http"

	// gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
)

func init() {
	// NewProvider(telegram)
	Register("telegram", NewTelegramBotV1)
}

// Telegram BOT chat provider
type TelegramBotV1 struct {
	*Gateway
	*telegram.BotAPI
}

// String "telegram" provider's name
func (_ *TelegramBotV1) String() string {
	return "telegram"
}

// NewTelegramBotV1 initialize new agent.profile service provider
func NewTelegramBotV1(agent *Gateway) Provider {

	token, ok := agent.Profile.Variables["token"]
	if !ok {
		agent.Log.Fatal().Msg("token not found")
		return nil
	}
	client := &http.Client{
		Transport: &transportDump{
			r: http.DefaultTransport,
			WithBody: true,
		},
	}

	bot, err := telegram.NewBotAPIWithClient(token, client)
	
	if err != nil {
		// log.Fatal().Msg(err.Error())
		agent.Log.Error().Err(err).
			Int64("pid", agent.Profile.Id).
			Str("gate", "telegram").
			Str("bot", agent.Profile.Name).
			Str("uri", "/" + agent.Profile.UrlId).
			Msg("Failed to init gateway")
		return nil
	}

	return &TelegramBotV1{
		Gateway: agent,
		BotAPI: bot,
	}
}

// Register Telegram Bot Webhook endpoint URI
func (c *TelegramBotV1) Register(ctx context.Context, linkURL string) error {

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
func (c *TelegramBotV1) Deregister(ctx context.Context) error {
	
	res, err := c.BotAPI.RemoveWebhook()
	
	if err != nil {
		return err
	}

	if !res.Ok {

	}

	return nil
}

// SendNotify implements provider.Sender interface for Telegram
func (c *TelegramBotV1) SendNotify(ctx context.Context, notify *Update) error {
	// send *gate.SendMessageRequest
	// externalID, err := strconv.ParseInt(send.ExternalUserId, 10, 64)
	
	channel := notify.Chat
	chatID, err := strconv.ParseInt(channel.ChatID, 10, 64)
	if err != nil {
		return errors.InternalServerError(
			"chat.gateway.telegram.chat.id.invalid",
			"telegram: invalid chat %s unique identifier; expect integer values", channel.ChatID)
	}

	var update telegram.Chattable
	// TODO: resolution for various notify content !
	switch notify.Event {
	case "text", "": // default
	
		sendMessage := notify.Message
		update = telegram.NewMessage(chatID, sendMessage.GetText())
	
	case "file":
	case "send":
	case "edit":
	case "read":
	case "seen":

	case "join":
	case "kick":

	case "typing":
	case "upload":

	case "invite":
	case "closed":
		// SEND: notify meesage text
		sendMessage := notify.Message
		update = telegram.NewMessage(chatID, sendMessage.GetText())
	
	default:

	}

	if update == nil {
		channel.Log.Warn().Str("notify", notify.Event).Str("error", "notify: not implemented").Msg("IGNORE")
		return nil
	}
	
	_, err = c.BotAPI.Send(update)

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

	return nil
}

// WebHook implementes provider.Receiver interface for Telegram
func (c *TelegramBotV1) WebHook(reply http.ResponseWriter, notice *http.Request) {

	var recvUpdate telegram.Update
	err := json.NewDecoder(notice.Body).Decode(&recvUpdate)

	if err != nil {
		http.Error(reply, "Failed to decode telegram .Update message", http.StatusBadRequest)
		return // 400 Bad Request
	}

	recvMessage := recvUpdate.Message
	if recvMessage == nil {
		recvMessage = recvUpdate.EditedMessage
	}

	if recvMessage != recvUpdate.Message {
		
		c.Gateway.Log.Warn().

			Int(  "telegram-id", recvMessage.From.ID).
			Str(  "username",    recvMessage.From.UserName).
			Int64("chat-id",     recvMessage.Chat.ID).
			// Str("first_name", message.From.FirstName).
			// Str("last_name",  message.From.LastName)

		Msg("IGNORE Update; NOT A Text Message")
		
		return // 200 IGNORE
	}

	// sender
	sender := recvMessage.Chat
	user := recvMessage.From

	// region: contact
	contact := &Account{
		ID:        0, // LOOKUP
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Username:  user.UserName,
		Channel:   "telegram",
		Contact:   strconv.Itoa(user.ID),
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
	chatID := strconv.FormatInt(sender.ID, 10)
	channel, err := c.Gateway.GetChannel(
		notice.Context(), chatID, contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err); if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		http.Error(reply, re.Detail, (int)(re.Code))
		return // 503 Bad Gateway
	}

	// channel.Title = sender.Title
	// contact.ID = channel.ContactID

	// endregion

	sendUpdate := Update{
		Title:   channel.Title,
		// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),
		Chat:    channel,
		User:    contact,
		Message: &chat.Message{
			Id:    0, // INCOME: NEW(!)
			Type:  "text",
			Value: &chat.Message_Text{
				Text: recvMessage.Text,
			},
			Variables: nil, // map[string]string{},
		},
	}

	err = c.Gateway.Read(notice.Context(), &sendUpdate)

	if err != nil {
		http.Error(reply, "Failed to deliver telegram .Update message", http.StatusInternalServerError)
		return // 502 Bad Gateway
	}

	reply.WriteHeader(http.StatusOK)
	return // 200 OK
}