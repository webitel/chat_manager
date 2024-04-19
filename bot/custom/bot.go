package custom

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/beevik/guid"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/micro/micro/v3/service/errors"
	errors2 "github.com/pkg/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	provider           = "custom"
	HashHeader         = "X-Webitel-Sign"
	sourceVariableName = "source"
)

func init() {
	bot.Register(provider, NewCustomGateway)
}

type CustomGateway struct {
	*bot.Gateway
	params     *CustomBotParameters
	contacts   map[string]*bot.Account
	closeQueue []string
	//broadcastEvents map[string][]*Lookup

	broadcastEvents *lru.LRU[string, []*Lookup]
}

func (c *CustomGateway) String() string {
	return provider
}

func (c *CustomGateway) Deregister(ctx context.Context) error {
	return nil
}

func (c *CustomGateway) Register(ctx context.Context, uri string) error {
	// not needed
	return nil
}

func (c *CustomGateway) Close() error {
	// ?
	return nil
}

type CustomBotParameters struct {
	// secret exchanged between two apps
	Secret string
	// webhook the messages send on
	CustomerWebHook string
}

// Initialization of custom gateway
func NewCustomGateway(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {

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
			"chat.bot.custom.updates.invalid",
			err.Error(),
		)
	}

	parameters, err := getCustomGatewayParamsFromMetadata(metadata)
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

	cache := lru.NewLRU[string, []*Lookup](500, nil, time.Minute*10)
	if err != nil {
		return nil, err
	}

	return &CustomGateway{
		Gateway:         agent,
		params:          parameters,
		contacts:        make(map[string]*bot.Account),
		closeQueue:      make([]string, 0),
		broadcastEvents: cache,
	}, nil
}

func (c *CustomGateway) processCloseQueueByChatId(chatId string) bool {
	var (
		present bool
		index   int
	)
	c.RLock()
	if present, index = contains(c.closeQueue, chatId); present {
		c.closeQueue = remove(c.closeQueue, index)
	}
	c.RUnlock()
	return present
}

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func contains(s []string, i string) (bool, int) {
	for index, val := range s {
		if i == val {
			return true, index
		}
	}
	return false, 0
}

func getCustomGatewayParamsFromMetadata(profile map[string]string) (*CustomBotParameters, error) {
	var res CustomBotParameters
	if v, ok := profile["secret"]; ok {
		res.Secret = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.custom.secret.required",
			"custom: secret required",
		)
	}

	if v, ok := profile["webhook"]; ok {
		_, err := url.ParseRequestURI(v)
		if err != nil {
			return nil, errors.BadRequest(
				"custom.bot.get_custom_bot_params.parse_url.error",
				err.Error(),
			)
		}
		res.CustomerWebHook = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.custom.webhook.required",
			"custom: webhook required",
		)
	}
	return &res, nil
}

func (c *CustomGateway) SendNotify(ctx context.Context, notify *bot.Update) error {
	var (
		channel = notify.Chat
		message = notify.Message

		chatId string
		//senderType string

		// the message of the event
		webhookMessage = &Message{Date: time.Now().Unix()}
		// outgoing event
		event = &SendEvent{Message: webhookMessage}
	)

	splittedChatId := strings.Split(channel.ChatID, "|")
	switch len(splittedChatId) {
	case 0:
		err := errors2.New("empty chat id")
		c.Gateway.Log.Err(err).
			Str("update", message.Type).
			Msg("custom/bot.updateChatMember")
		return errors.InternalServerError("custom.bot.send_notify.joined_type.error", err.Error())
	case 1:
		// there was no type of sender
		chatId = splittedChatId[0]
	case 2:
		//senderType = splittedChatId[0]
		chatId = splittedChatId[1]
	default:
		chatId = channel.ChatID
	}
	webhookMessage.ChatId = chatId

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
			return errors.InternalServerError("custom.bot.send_notify.joined_type.error", err.Error())
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
			return errors.InternalServerError("custom.bot.send_notify.left_type.error", err.Error())
		}

		webhookMessage.Text = messageText

	case "closed":

		updates := c.Gateway.Template
		messageText, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", message.Type).
				Msg("custom/bot.updateChatClose")
			return errors.BadRequest("custom.bot.send_notify.closed_type.error", err.Error())
		}
		if messageText != "" {
			webhookMessage.Text = messageText
			// Make the request model for the event
			req, body, err := Requestify(ctx, event, http.MethodPost, c.params.CustomerWebHook, c.params.Secret)
			if err != nil {
				c.Gateway.Log.Err(err).
					Str("update", message.Type).
					Msg("custom/bot.updateChatError")
				return errors.InternalServerError("custom.bot.send_notify.closed_type.construct_request.error", err.Error())
			}
			rsp, err := http.DefaultClient.Do(req)
			if err != nil {
				c.Gateway.Log.Err(err).
					Str("update", message.Type).
					Msg("custom/bot.updateChatHttpRequestError")
				return errors.InternalServerError("custom.bot.send_notify.closed_type.do_request", err.Error())
			}
			c.Gateway.Log.Info().
				Str("update", message.Type).
				Msg(fmt.Sprintf("custom/bot.updateChatRequest; url = %s; http response status=%s; update request=%s", req.URL.String(), rsp.Status, string(body)))

		}

		present := c.processCloseQueueByChatId(chatId)
		// if event was present -- the external close of chat
		if present {
			return nil
		}

		// close was initiated by the operator -- send close event
		event = &SendEvent{Close: &SendClose{ChatId: chatId}}

	default:
		return errors.BadRequest("custom.bot.send_notify.parse_type.wrong", "unsupported message type")
	}
	// Make the request model for the event
	req, body, err := Requestify(ctx, event, http.MethodPost, c.params.CustomerWebHook, c.params.Secret)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("update", message.Type).
			Msg("custom/bot.updateChatError")
		return errors.InternalServerError("custom.bot.send_notify.construct_request.error", err.Error())
	}
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("update", message.Type).
			Msg("custom/bot.updateChatHttpRequestError")
		return errors.InternalServerError("custom.bot.send_notify.do_request.error", err.Error())
	}
	c.Gateway.Log.Trace().
		Str("update", message.Type).
		Msg(fmt.Sprintf("custom/bot.updateChatRequest; url = %s; http response status=%s; update request=%s", req.URL.String(), rsp.Status, string(body)))
	// SUCCESS
	return nil
}

func (c *CustomGateway) WebHook(reply http.ResponseWriter, notice *http.Request) {
	switch notice.Method {
	case http.MethodPost:
	// allowed
	default:
		returnErrorToResp(reply, http.StatusMethodNotAllowed, nil)
		return
	}

	var (
		bodyBuf bytes.Buffer
		sender  *bot.Account
	)
	_, err := bodyBuf.ReadFrom(notice.Body)
	if err != nil && !errors2.Is(err, io.EOF) {
		c.Gateway.Log.Err(err).
			Msg("custom/bot.readBody")
		returnErrorToResp(reply, http.StatusBadRequest, nil)
		return
	}
	// check hash
	suspiciousHash := notice.Header.Get(HashHeader)
	if validHash := calculateHash(bodyBuf.Bytes(), c.params.Secret); validHash != suspiciousHash { // threat or no sign
		c.Gateway.Log.Err(errors2.New(fmt.Sprintf("wrong hash for the webhook, provided - %s expected - %s", suspiciousHash, validHash))).
			Str("suspicious", suspiciousHash).
			Msg("custom/bot.hashCheck")
		returnErrorToResp(reply, http.StatusForbidden, nil)
		return
	}

	// decode event
	var event ReceiveEvent
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
		sender = c.contacts[closeEvent.ChatId]
		channel, err := c.Gateway.GetChannel(
			context.Background(), closeEvent.ChatId, sender,
		)
		if err != nil {
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}
		c.RLock()
		c.closeQueue = append(c.closeQueue, closeEvent.ChatId)
		c.RUnlock()
		// close channel
		err = channel.Close()
		if err != nil {
			c.processCloseQueueByChatId(closeEvent.ChatId)
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}

	} else if broadcastEvent := event.Broadcast; broadcastEvent != nil {
		eventId := broadcastEvent.EventId
		if broadcastEvent != nil && len(broadcastEvent.FailedReceivers) != 0 {
			var errMessage = ""
			for _, lookup := range broadcastEvent.FailedReceivers {
				errMessage += fmt.Sprintf("receiver %s|%s error: %s; ", lookup.Type, lookup.Id, lookup.Error)
			}
			c.Log.Warn().Msg(errMessage)
		}
		c.broadcastEvents.Remove(eventId)
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
		if err != nil {
			c.Log.Err(err)
			returnErrorToResp(reply, http.StatusBadRequest, err)
			return
		}

		update = &bot.Update{
			Chat:    channel,
			Title:   channel.Title,
			User:    &channel.Account,
			Message: new(chat.Message),
		}
		internalMessage := update.Message
		internalMessage.CreatedAt = messageEvent.Date
		if channel.IsNew() {
			internalMessage.Variables = messageEvent.Metadata
			if sender := messageEvent.Sender; sender != nil && sender.Type != "" {
				if internalMessage.Variables == nil {
					internalMessage.Variables = map[string]string{}
				}
				internalMessage.Variables[sourceVariableName] = sender.Type
				//metadata, _ := channel.Properties.(map[string]string)
				//if metadata == nil {
				//	metadata = make(map[string]string, 4)
				//}
				//metadata[sourceVariableName] = sender.Type
			}
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
		// TODO id is empty!
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

func (c *CustomGateway) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {
	var (
		eventId   = guid.New().String()
		broadcast = &SendBroadcast{EventId: eventId, Recipients: make([]*Lookup, 0)}
		event     = &SendEvent{Broadcast: broadcast}
	)
	peers := req.GetPeer()
	if len(peers) == 0 {
		description := "no peers were received"
		c.Gateway.Log.Warn().
			Str("broadcast", eventId).
			Msg("custom/bot.broadcastGetPeers")
		return errors.InternalServerError("custom.bot.broadcast.get_peers.error", description)
	}
	for _, peer := range peers {
		var receiverId, receiverType string
		splittedSenderId := strings.Split(peer, "|")
		switch len(splittedSenderId) {
		case 0:
			err := errors2.New("empty chat id")
			c.Gateway.Log.Err(err).
				Str("broadcast", peer).
				Msg("custom/bot.broadcastGetPeers")
			return errors.InternalServerError("custom.bot.broadcast.split_receiver.error", err.Error())
		case 1:
			// there was no type of sender
			receiverId = splittedSenderId[0]
		case 2:
			receiverType = splittedSenderId[0]
			receiverId = splittedSenderId[1]
		default:
			continue
		}
		broadcast.Recipients = append(broadcast.Recipients, &Lookup{
			Id:   receiverId,
			Type: receiverType,
		})
	}
	if message := req.GetMessage(); message != nil {
		broadcast.Text = message.Text
		broadcast.Metadata = message.Variables
	}
	err := broadcast.Normalize()
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("broadcast", eventId).
			Msg("custom/bot.broadcastRequestify")
		return errors.InternalServerError("custom.bot.broadcast.normalize_event.error", err.Error())
	}
	httpRequest, body, err := Requestify(ctx, event, http.MethodPost, c.params.CustomerWebHook, c.params.Secret)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("broadcast", eventId).
			Msg("custom/bot.broadcastRequestify")
		return errors.InternalServerError("custom.bot.broadcast.construct_request.error", err.Error())
	}
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		c.Gateway.Log.Err(err).
			Str("broadcast", eventId).
			Msg("custom/bot.broadcastHttpRequest")
		return errors.InternalServerError("custom.bot.broadcast.do_request.error", err.Error())
	}
	c.broadcastEvents.Add(eventId, broadcast.Recipients)
	c.Gateway.Log.Trace().
		Str("broadcast", eventId).
		Msg(fmt.Sprintf("custom/bot.broadcastRequest; url = %s; http response status=%s; update request=%s", httpRequest.URL.String(), httpResponse.Status, string(body)))
	// SUCCESS
	return nil
}

func calculateHash(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	hash := h.Sum(nil)
	trueHash := hex.EncodeToString(hash)
	return trueHash
}

func (c *CustomGateway) getChannel(ctx context.Context, message *Message) (*bot.Channel, error) {
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
	return fmt.Sprintf("custom: %s", error)
}

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}

func Requestify(ctx context.Context, body any, method string, url string, secret string) (*http.Request, []byte, error) {
	var (
		buf  bytes.Buffer
		copy bytes.Buffer
	)
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		return nil, nil, err
	}
	copy.Write(buf.Bytes())
	req, err := http.NewRequestWithContext(ctx, method, url, &buf)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("X-Webitel-Sign", calculateHash(copy.Bytes(), secret))
	return req, buf.Bytes(), nil
}
