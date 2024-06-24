package custom

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	goerr "errors"
	"fmt"
	"github.com/beevik/guid"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/micro/micro/v3/service/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"google.golang.org/genproto/googleapis/rpc/status"
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
	params   *CustomBotParameters
	contacts map[string]*bot.Account
	// closeQueue is the storage of chatIds for the sync of the webhook and send notify close events.
	//
	// If chat was closed by external user the close event goes to the send notify to send close message.
	closeQueue []string
	// broadcastSync is the channel used to synchronize the webhook method and broadcast method of the gateway
	//
	// (used in flow schemas to get the results of the async broadcast in scheme variables )
	broadcastSync chan ReceiveBroadcast

	// broadcastEvents used to cache the data of the broadcast messages by key-value = eventId-event receivers
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
		broadcastSync:   make(chan ReceiveBroadcast),
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
		err := goerr.New("empty chat id")
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
		if messageText == "" {
			return nil
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
		ctx     = notice.Context()
	)
	_, err := bodyBuf.ReadFrom(notice.Body)
	if err != nil && !goerr.Is(err, io.EOF) {
		c.Gateway.Log.Err(err).
			Msg("custom/bot.readBody")
		returnErrorToResp(reply, http.StatusBadRequest, nil)
		return
	}
	// check hash
	suspiciousHash := notice.Header.Get(HashHeader)
	if validHash := calculateHash(bodyBuf.Bytes(), c.params.Secret); validHash != suspiciousHash { // threat or no sign
		c.Gateway.Log.Err(goerr.New(fmt.Sprintf("wrong hash for the webhook, provided - %s expected - %s", suspiciousHash, validHash))).
			Str("suspicious", suspiciousHash).
			Msg("custom/bot.hashCheck")
		returnErrorToResp(reply, http.StatusForbidden, nil)
		return
	}

	// decode event
	var event ReceiveEvent
	err = json.Unmarshal(bodyBuf.Bytes(), &event)
	if err != nil {
		c.Log.Err(err)
		returnErrorToResp(reply, http.StatusInternalServerError, err)
		return
	}
	defer notice.Body.Close()

	// switch event type
	if closeEvent := event.Close; closeEvent != nil { // close the chat (highest priority)
		err = c.handleChatClose(ctx, closeEvent)
	} else if broadcastEvent := event.Broadcast; broadcastEvent != nil {
		err = c.handleBroadcast(ctx, broadcastEvent)
	} else if messageEvent := event.Message; messageEvent != nil { // message to the new or existing chat
		err = c.handleMessage(ctx, messageEvent)
	} else { // no payload
		err = goerr.New("no valid payload")
	}
	if err != nil {
		c.Log.Err(err)
		returnErrorToResp(reply, http.StatusInternalServerError, err)
		return
	}
	// encode successful response
	headers := reply.Header()
	headers["Content-Type"] = []string{"application/json"}
	json.NewEncoder(reply).Encode(Response{Success: true})
	reply.WriteHeader(http.StatusOK)

	return
}

func (c *CustomGateway) handleMessage(ctx context.Context, msg *Message) error {
	var (
		update         *bot.Update
		conversationId string
	)
	if msg == nil {
		return nil
	}
	err := msg.Normalize() // check for nil values where fields required
	if err != nil {
		return err
	}

	conversationId = msg.ChatId

	channel, err := c.getChannel(
		ctx, msg,
	)
	if err != nil {
		return err
	}

	update = &bot.Update{
		Chat:    channel,
		Title:   channel.Title,
		User:    &channel.Account,
		Message: new(chat.Message),
	}
	internalMessage := update.Message
	if internalMessage.Variables == nil {
		internalMessage.Variables = make(map[string]string)
	}
	internalMessage.CreatedAt = msg.Date
	if channel.IsNew() {
		internalMessage.Variables = msg.Metadata
		if sender := msg.Sender; sender != nil && sender.Type != "" {
			internalMessage.Variables[sourceVariableName] = sender.Type
		}
	}

	if file := msg.File; file != nil {
		internalMessage.Type = bot.FileType
		internalMessage.Text = msg.Text
		internalMessage.File = &chat.File{
			Id:   0,
			Url:  file.Url,
			Mime: file.Mime,
			Name: file.Name,
			Size: file.Size,
		}
	} else {
		internalMessage.Type = bot.TextType
		internalMessage.Text = msg.Text
	}
	// TODO id is empty!
	internalMessage.Variables[conversationId] = msg.Id
	return c.Gateway.Read(ctx, update)
}

func (c *CustomGateway) handleChatClose(ctx context.Context, closeEvent *ReceiveClose) error {
	if closeEvent == nil {
		return nil
	}
	err := closeEvent.Normalize() // check for nil values where fields required
	if err != nil {
		return err
	}
	// search for the channel to close (contact probably will be in the cache)
	// if not then sender = nil will search for the database entry
	sender := c.contacts[closeEvent.ChatId]
	channel, err := c.Gateway.GetChannel(
		ctx, closeEvent.ChatId, sender,
	)
	if err != nil {
		return err
	}
	c.RLock()
	// add the id to the close queue for SendMessage knew if there was external close of the chat
	c.closeQueue = append(c.closeQueue, closeEvent.ChatId)
	c.RUnlock()
	// close channel
	return channel.Close()
}

// handleBroadcast on the webhook used to process failed broadcast receivers
//
// Also syncs the SendBroadcast and WebHook methods
func (c *CustomGateway) handleBroadcast(ctx context.Context, broadcast *ReceiveBroadcast) error {
	if broadcast == nil {
		return nil
	}
	eventId := broadcast.EventId
	var errMessage = ""
	for _, lookup := range broadcast.FailedReceivers {
		errMessage += fmt.Sprintf("{type:%s, id:%s} error: %s; ", lookup.Type, lookup.Id, lookup.Error)
	}
	c.broadcastSync <- *broadcast
	c.Log.Warn().Msg(errMessage)
	c.broadcastEvents.Remove(eventId)
	return nil
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
			err := goerr.New("empty chat id")
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

	failedBroadcast := c.WaitForTheBroadcastChannelOrTimeout(time.Duration(req.Timeout)*time.Millisecond, eventId)
	if failedBroadcast != nil {
		// broadcast returned
		rsp.Failure = make([]*chat.BroadcastPeer, 0)
		for _, receiver := range failedBroadcast.FailedReceivers {
			rsp.Failure = append(rsp.Failure, &chat.BroadcastPeer{
				Peer:  receiver.Id,
				Error: &status.Status{Message: receiver.Error},
			})
		}
	} else {
		// timeout
	}
	// SUCCESS
	return nil
}

func (c *CustomGateway) WaitForTheBroadcastChannelOrTimeout(timeout time.Duration, originalEventId string) *ReceiveBroadcast {
	shouldEnd := time.Now().Add(timeout)
	select {
	case <-time.After(timeout): // requested timeout
	// don't do anything and return nil
	case failedBroadcast := <-c.broadcastSync: // broadcast client - answer
		if failedBroadcast.EventId == originalEventId {
			return &failedBroadcast
		} else {
			return c.WaitForTheBroadcastChannelOrTimeout(shouldEnd.Sub(time.Now()), originalEventId)
		}
	}
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
		return nil, goerr.New("sender is empty")
	}
	chatId := message.ChatId
	if chatId == "" {
		return nil, goerr.New("chat id is empty")
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
	req.Header.Set("Content-Type", "application/json")
	return req, buf.Bytes(), nil
}
