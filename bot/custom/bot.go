package custom

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/micro/micro/v3/service/errors"
	errors2 "github.com/pkg/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	provider   = "custom_bot"
	HashHeader = "X-Webitel-Sign"
)

func init() {
	bot.Register(provider, NewCustomBot)
}

type CustomBot struct {
	*bot.Gateway
	params   *CustomBotParameters
	contacts map[string]*bot.Account
}

func (c *CustomBot) String() string {
	return provider
}

func (c *CustomBot) Deregister(ctx context.Context) error {
	return nil
}

func (c *CustomBot) Register(ctx context.Context, uri string) error {
	// not needed
	return nil
}

func (c *CustomBot) Close() error {
	// ?
	return nil
}

type CustomBotParameters struct {
	// secret for this group
	Secret string
	// confirmation code used for [WebHook] confirmation
	CustomerWebHook string
}

// Initialization of custom gateway
func NewCustomBot(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {

	config := agent.Bot
	metadata := config.GetMetadata()

	// Parse and validate message templates
	var err error
	agent.Template = bot.NewTemplate(provider)
	// Parse message templates
	if err = agent.Template.FromProto(
		agent.Bot.GetUpdates(),
	); err == nil {
		// Quick tests ! <nil> means default (well-known) test cases
		err = agent.Template.Test(nil)
	}
	if err != nil {
		return nil, errors.BadRequest(
			"chat.bot.custom_bot.updates.invalid",
			err.Error(),
		)
	}

	parameters, err := getCustomBotParamsFromMetadata(metadata)
	if err != nil {
		return nil, err
	}
	var (
		httpClient *http.Client
	)

	trace := metadata["trace"]
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

	return &CustomBot{
		Gateway:  agent,
		params:   parameters,
		contacts: make(map[string]*bot.Account),
	}, nil
}

func getCustomBotParamsFromMetadata(profile map[string]string) (*CustomBotParameters, error) {
	var res CustomBotParameters
	if v, ok := profile["secret"]; ok {
		res.Secret = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.custom_bot.secret.required",
			"custom_bot: secret required",
		)
	}

	if v, ok := profile["webhook"]; ok {
		res.CustomerWebHook = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.custom_bot.webhook.required",
			"custom_bot: webhook required",
		)
	}
	return &res, nil
}

func (c *CustomBot) SendNotify(ctx context.Context, notify *bot.Update) error {
	var (
		channel = notify.Chat
		message = notify.Message

		// the message of the event
		webhookMessage = &Message{ChatId: channel.ChatID, Date: time.Now().Unix()}
		// outgoing event
		event = &Event{Message: webhookMessage}
	)

	if channel.Title == "" {
		// FIXME: .GetChannel() does not provide full contact info on recover,
		//                      just it's unique identifier ...  =(
	}

	webhookMessage.ChatId = channel.ChatID
	switch message.Type {
	case "text":

		messageText := strings.TrimSpace(
			message.GetText(),
		)
		if messageText == "" {
			return nil
		}
		webhookMessage.Text = messageText

	case "file":
		doc := message.GetFile()
		webhookMessage.File = &File{
			Url:  doc.Url,
			Mime: doc.Mime,
			Size: doc.Size,
			Name: doc.Name,
		}
	case "joined":

		peer := contactPeer(message.NewChatMembers[0])
		updates := c.Gateway.Template
		text, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", message.Type).
				Msg("custom/bot.updateChatMember")
		}
		if text == "" {
			return nil
		}
		webhookMessage.Text = text

	case "left":

		peer := contactPeer(message.LeftChatMember)
		updates := c.Gateway.Template
		messageText, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", message.Type).
				Msg("custom/bot.updateLeftMember")
		}

		webhookMessage.Text = messageText

	case "closed":

		updates := c.Gateway.Template
		messageText, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", message.Type).
				Msg("custom/bot.updateChatClose")
		}
		webhookMessage.Text = messageText

	default:

	}
	// Make the request model for the event
	req, err := event.Requestify(http.MethodPost, c.params.CustomerWebHook, c.params.Secret)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("update", message.Type).
			Msg("custom/bot.updateChatError")
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("update", message.Type).
			Msg("custom/bot.updateChatRequest")
	}
	// SUCCESS
	return nil
}

func (c *CustomBot) WebHook(reply http.ResponseWriter, notice *http.Request) {
	switch notice.Method {
	case http.MethodPost:
	// allowed
	default:

		returnErrorToResp(reply, http.StatusMethodNotAllowed, nil)
		return
	}

	var (
		bodyBuf bytes.Buffer
	)
	_, err := bodyBuf.ReadFrom(notice.Body)
	if !errors2.Is(err, io.EOF) {
		returnErrorToResp(reply, http.StatusBadRequest, nil)
		return
	}
	// check hash
	suspiciousHash := notice.Header.Get(HashHeader)
	if calculateHash(bodyBuf.Bytes(), c.params.Secret) != suspiciousHash { // threat or no sign
		returnErrorToResp(reply, http.StatusForbidden, nil)
		return
	}

	// decode event
	var event Event
	err = json.Unmarshal(bodyBuf.Bytes(), &event)
	if err != nil {
		returnErrorToResp(reply, http.StatusInternalServerError, err)
		return
	}
	defer notice.Body.Close()

	// switch event type
	if closeEvent := event.Close; closeEvent != nil { // close the chat (highest priority)

		err = closeEvent.Normalize() // check for nil values where fields required
		if err != nil {
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}
		// search for the channel to close (contact probably will be in the cache)
		channel, err := c.Gateway.GetChannel(
			context.Background(), closeEvent.ChatId, c.contacts[closeEvent.ChatId],
		)
		if err != nil {
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}
		// close channel
		err = channel.Close()
		if err != nil {
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}

	} else if messageEvent := event.Message; messageEvent != nil { // message to the new or existing chat
		var (
			update         *bot.Update
			conversationId string
		)
		err = messageEvent.Normalize() // check for nil values where fields required
		if err != nil {
			c.Log.Err(err)
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}

		conversationId = messageEvent.ChatId

		channel, err := c.getChannel(
			notice.Context(), messageEvent,
		)
		internalMessage := update.Message

		update = &bot.Update{
			Chat:  channel,
			Title: channel.Title,
			User:  &channel.Account,

			Message: new(chat.Message),
		}
		if messageEvent.Sender.Type != "" {
			update.Message.Variables["source"] = messageEvent.Sender.Type
		}
		if file := messageEvent.File; file != nil {

			internalMessage.Type = bot.FileType
			internalMessage.Text = messageEvent.Text
			internalMessage.File = &chat.File{
				Id:   0,
				Url:  file.Url,
				Mime: file.Mime,
				Name: file.Name,
				Size: file.Size,
			}
		} else {
			internalMessage.Type = bot.TextType
			internalMessage.Text = messageEvent.Text
		}
		update.Message.Variables = map[string]string{
			conversationId: messageEvent.Id,
		}
		err = c.Gateway.Read(notice.Context(), update)
		if err != nil {
			code := http.StatusInternalServerError
			http.Error(reply, "Failed to forward .Update recvEvent", code)
			return // 502 Bad Gateway
		}

	} else { // no payload
		returnErrorStringToResp(reply, http.StatusBadRequest, "no valid payload")
		return
	}
	// encode successful response
	json.NewEncoder(reply).Encode(Response{Success: true})
	reply.WriteHeader(http.StatusOK)
	return
	// return // HTTP/1.1 200 OK
}

func (c *CustomBot) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {

	return nil
}

func calculateHash(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	hash := h.Sum(nil)
	trueHash := hex.EncodeToString(hash)
	return trueHash
}

func (c *CustomBot) getChannel(ctx context.Context, message *Message) (*bot.Channel, error) {
	sender := message.Sender
	if sender == nil {
		return nil, errors2.New("sender is empty")
	}
	chatId := message.ChatId
	if chatId == "" {
		return nil, errors2.New("chat id is empty")
	}
	// check for cache entry
	contact := c.contacts[chatId]

	if contact == nil {

		contact = &bot.Account{

			ID: 0, // LOOKUP

			Channel: provider,
			Contact: sender.Id,

			FirstName: sender.Name,

			Username: sender.Nickname,
		}
		// processed
		c.contacts[chatId] = contact
	}

	return c.Gateway.GetChannel(
		ctx, chatId, contact,
	)
}

func returnErrorToResp(rsp http.ResponseWriter, code int, err error) {
	if err == nil {
		if code == 0 {
			code = http.StatusInternalServerError
		}
		rsp.WriteHeader(code)
		return
	}
	if code == 0 {
		code = http.StatusInternalServerError
	}
	rsp.WriteHeader(code)
	rsp.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rsp).Encode(Response{Error: formatErrorString(err.Error())})
	return
}
func returnErrorStringToResp(rsp http.ResponseWriter, code int, err string) {
	if err == "" {
		if code == 0 {
			code = http.StatusInternalServerError
		}
		rsp.WriteHeader(code)
		return
	}
	if code == 0 {
		code = http.StatusInternalServerError
	}
	rsp.WriteHeader(code)
	json.NewEncoder(rsp).Encode(Response{Error: formatErrorString(err)})
	return
}

func formatErrorString(error string) string {
	return fmt.Sprintf("custom_bot: %s", error)
}

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}
