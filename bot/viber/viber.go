package viber

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
)

type Bot struct {
	Token   string
	Sender  *User
	Client  *http.Client
	Account *Account
	Buttons ButtonOptions
	Gateway *bot.Gateway
}

// constants
const (
	provider    = "viber"
	endpointURL = "https://chatapi.viber.com/pa"
	// MUST: strings.TrimRight(endpointURL, "/")
)

func init() {
	// Register "viber" provider factory method
	bot.Register(provider, New)
}

// New initialize new agent.profile service Viber Bot provider
func New(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {
	// Latest (current) state
	app, _ := state.(*Bot)
	// Validate NEW Options
	profile := agent.Bot.GetMetadata()
	botToken, ok := profile["token"]
	if !ok {
		log.Error().Msg("AppToken not found")
		return nil, errors.BadRequest(
			"chat.gateway.viber.token.required",
			"viber: bot API token required",
		)
	}
	// Sender's specified .Name
	botName, _ := profile["bot_name"]
	// HTTP Client | http.DefaultClient
	var client *http.Client
	trace := profile["trace"]
	if on, _ := strconv.ParseBool(trace); on {
		var transport http.RoundTripper
		if client != nil {
			transport = client.Transport
		}
		if transport == nil {
			transport = http.DefaultTransport
		}
		transport = &bot.TransportDump{
			Transport: transport,
			WithBody:  true,
		}
		if client == nil {
			client = &http.Client{
				Transport: transport,
			}
		} else {
			// NOTE: Be aware of http.DefaultClient.Transport reassignment !
			client.Transport = transport
		}
	}

	// Parse and validate message templates
	agent.Template = bot.NewTemplate(
		provider,
	)

	var err error
	// // Populate viber-specific template helper funcs
	// agent.Template.Root().Funcs(
	// 	markdown.TemplateFuncs,
	// )
	// Parse message templates
	if err = agent.Template.FromProto(
		agent.Bot.GetUpdates(),
	); err == nil {
		// Quick tests ! <nil> means default (well-known) test cases
		err = agent.Template.Test(nil)
	}
	if err != nil {
		return nil, errors.BadRequest(
			"chat.bot.viber.updates.invalid",
			err.Error(),
		)
	}
	// Can we upgrade latest bot account ?
	if app != nil && app.Token != botToken {
		app = nil // NOTE: No ! Brand NEW Account !
	}

	if app == nil {
		app = new(Bot)
	}
	// [RE]Bind
	app.Buttons, err = newButtonOptions(profile)
	if err != nil {
		return nil, err
	}
	app.Gateway = agent
	app.Client = client
	app.Token = botToken

	sender := &User{
		Name: botName,
	}
	// CHECK: Token is still valid !
	me, err := app.getMe(true)
	if err != nil {
		return nil, err
	}

	if sender.Name == "" {
		sender.Name = me.Name
	}
	// Sender account name
	app.Sender = sender

	return app, nil
}

func (*Bot) Close() error {
	return nil
}

func (*Bot) String() string {
	return provider
}

// Register Viber Bot Webhook endpoint URI
func (c *Bot) Register(ctx context.Context, linkURL string) error {

	var (
		res struct {
			Status
			Hostname      string   `json:"chat_hostname"`
			Subscriptions []string `json:"event_types"`
		}
		req = setWebhook{
			CallbackURL: linkURL,
			EventTypes: []string{
				// "delivered",
				// "seen",
				// "failed",
				"message",
				"subscribed",
				"unsubscribed",
				"conversation_started",
			},
			SendName: true,
		}
	)

	err := c.do(req, &res)
	if err == nil {
		err = res.Err()
	}

	if err != nil {
		c.Gateway.Log.Err(err).Msg("viber/bot.setWebhook")
		return err
	}

	// Refresh Account Info
	that := c.Account
	this, _ := c.getMe(true)
	if that != nil && this != nil {
		if c.Sender.Name == that.Name {
			c.Sender.Name = this.Name // Sender NEW Name
		}
	}

	return nil
}

// Deregister viber Bot Webhook endpoint URI
func (c *Bot) Deregister(ctx context.Context) error {

	var (
		res Status
		req = setWebhook{
			CallbackURL: "",
		}
	)

	err := c.do(req, &res)
	if err == nil {
		err = res.Err()
	}

	if err != nil {
		return err
	}

	if me := c.Account; me != nil {
		me.Webhook = ""
		me.Events = nil
	}

	return nil
}

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}

func (c *Bot) SendNotify(ctx context.Context, notify *bot.Update) error {

	var (
		// notify.Dialog
		peerChannel = notify.Chat
		// notify.Message
		sentMessage = notify.Message
		// msgBindings map[string]string
	)

	sendMessage := SendMessage{
		// Target
		PeerId: peerChannel.ChatID,
		// Options
		sendOptions: sendOptions{
			Sender: c.Sender,
		},
	}

	switch sentMessage.Type {

	case "text":

		sendMessage.Text(
			sentMessage.GetText(),
		)

		if sentMessage.Buttons != nil {
			sendMessage.Menu(
				&c.Buttons,
				sentMessage.Buttons,
			)
		}

	case "file":

		sendMessage.Media(
			sentMessage.GetFile(),
			sentMessage.GetText(), // Max 512 characters !
		)

	case "left":

		peer := contactPeer(sentMessage.LeftChatMember)
		updates := c.Gateway.Template
		messageText, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("viber/bot.updateLeftMember")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		sendMessage.Text(messageText)

	case "joined":

		peer := contactPeer(sentMessage.NewChatMembers[0])
		updates := c.Gateway.Template
		messageText, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("vider/bot.updateChatMember")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		// format new message to the engine for saving it in the DB as operator message [WTEL-4695]
		messageToSave := &chat.Message{
			Type:      "text",
			Text:      messageText,
			CreatedAt: time.Now().UnixMilli(),
			From:      peer,
		}
		if peerChannel != nil && peerChannel.ChannelID != "" {
			_, err = c.Gateway.Internal.Client.SaveAgentJoinMessage(ctx, &chat.SaveAgentJoinMessageRequest{Message: messageToSave, Receiver: peerChannel.ChannelID})
			if err != nil {
				return err
			}
		}
		sendMessage.Text(messageText)

	case "closed":

		updates := c.Gateway.Template
		messageText, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("viber/bot.updateChatClose")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		sendMessage.Text(messageText)

	default:
		// UNKNOWN Internal Message Update
		return nil
	}

	// https://developers.viber.com/docs/api/rest-bot-api/#response
	var res SendResponse

	err := c.do(&sendMessage, &res)
	if err == nil {
		err = res.Err()
	}

	if err != nil {
		// Is Viber status error ?
		if rpcErr, is := err.(*Error); is {
			//
			// https://developers.viber.com/docs/api/rest-bot-api/#error-codes
			//
			// (6) receiverNotSubscribed: The receiver is not subscribed to the account
			//
			// NOTE: This might happen, when Viber user opened a deeplink to our bot
			// and got the very first, so called, "welcome" message from our (bot) flow schema
			// https://developers.viber.com/docs/api/rest-bot-api/#sending-a-welcome-message
			// but did nothing more, no any reaction ...
			//
			// Any other messages from our flow schema will fail to send with above status.
			// So here we force close the dialog channel with such Viber member(s) ...
			if rpcErr.IsCode(6) { // && rpcErr.Message == "notSubscribed" {
				defer peerChannel.Close()
			}
		}
		return err
	}

	return nil
}

func (c *Bot) httpClient() (htc *http.Client) {
	htc = c.Client
	if htc == nil {
		htc = http.DefaultClient
	}
	return // htc
}

type request interface {
	method() string
}

type resultError http.Response

func (res *resultError) Error() string {
	return fmt.Sprintf("(%d) %s", res.StatusCode, res.Status)
}

func (c *Bot) do(r request, w interface{}) error {

	var (
		err error
		req *http.Request
		res *http.Response
		buf = bytes.NewBuffer(nil)
		enc = json.NewEncoder(buf)
	)
	// ENCODE Request JSON
	enc.SetEscapeHTML(false)
	err = enc.Encode(r)
	if err != nil {
		return err
	}
	// PREPARE Request JSON
	req, err = http.NewRequest(
		"POST", endpointURL+path.Join("/", r.method()), buf,
	)

	if err != nil {
		return err
	}

	req.Close = true // Connection: close
	req.Header.Set("X-Viber-Auth-Token", c.Token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// PERFORM RPC Request
	res, err = c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if w != nil {
		err = json.NewDecoder(res.Body).Decode(w)
	} else {
		code := res.StatusCode
		switch {
		case 200 <= code && code < 300: // Success
		// case 300 <= code && code < 400: // Redirect
		// case 400 <= code && code < 500: // Client Error(s)
		// case 500 <= code: // Server Error(s)
		default:
			err = (*resultError)(res)
		}
	}

	if err != nil {
		c.Gateway.Log.Err(err).Msg("viber/" + r.method() + ":result")
		return err
	}

	return nil
}

// WebHook implementes provider.Receiver interface for viber
func (c *Bot) WebHook(reply http.ResponseWriter, notice *http.Request) {

	switch notice.Method {
	case http.MethodPost:
		// Handle Update(s) ...
	// // TODO: Viber Bot Public API
	// case http.MethodGet:
	// 	return
	default:
		// Method Not Allowed !
		http.Error(reply,
			"(405) Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	// POST Inbound Update(s) ...
	var event Update
	err := json.NewDecoder(notice.Body).Decode(&event)
	if err != nil {
		c.Gateway.Log.Err(err).Msg("viber/bot.onUpdate")
		return // (200) IGNORE
	}

	var (
		ctx  = notice.Context()
		hook func(ctx context.Context, event *Update) error
	)
	switch event.Type {
	case updateWebhook:
		// {"event":"webhook","timestamp":1663858877101,"chat_hostname":"SN-CHAT-03_","message_token":5753750845017966998}
		return // (200) OK
	case updateNewDialog:
		hook = c.onNewDialog
	case updateNewMessage:
		hook = c.onNewMessage
	case updateJoinMember:
		hook = c.onJoinMember
	case updateLeftMember:
		hook = c.onLeftMember
	case updateSentMessage,
		updateReadMessage,
		updateFailMessage:
		hook = c.onMsgStatus
	default:
		c.Gateway.Log.Warn().
			Str("event", event.Type).
			Str("error", "event: no update reaction").
			Msg("viber/bot.onUpdate")
		return // (200) IGNORE
	}
	// Handle update event
	err = hook(ctx, &event)
	if err != nil {
		c.Gateway.Log.Err(err).
			Msg("viber/bot.on" + strings.Title(event.Type))
		return // (200) IGNORE
	}

	// return // (200) IGNORE [Re]delivery!
}

// Broadcast given `req.Message` message [to] provided `req.Peer(s)`
func (c *Bot) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {

	var (
		setError = func(peerId string, err error) {
			res := rsp.GetFailure()
			if res == nil {
				res = make([]*chat.BroadcastPeer, 0, len(req.GetPeer()))
			}

			var re *status.Status
			switch err := err.(type) {
			case *Error: // Viber Status Error
				// code := err.Code
				// // https://developers.viber.com/docs/api/rest-bot-api/#error-codes
				// switch code {
				// // 5 "receiverNotRegistered" The receiver is not registered to Viber
				// case 5:
				// }
				re = status.New(codes.Code(err.Code), err.Message)
			case *errors.Error:
				re = status.New(codes.Code(err.Code), err.Detail)
			default:
				re = status.New(codes.Unknown, err.Error())
			}

			res = append(res, &chat.BroadcastPeer{
				Peer:  peerId,
				Error: re.Proto(),
			})

			rsp.Failure = res
		}
		// https://developers.viber.com/docs/api/rest-bot-api/#response-1
		res BroadcastResponse
		// https://developers.viber.com/docs/api/rest-bot-api/#post-parameters
		cast = BroadcastMessage{
			// Recipient(s)
			PeerId: req.GetPeer(),
			// SendOptions
			sendOptions: sendOptions{
				Sender: c.Sender,
			},
		}
	)

	// Get message params from request
	message := req.GetMessage()

	// Set text or file to message
	switch message.GetType() {
	case "text":
		cast.Text(message.GetText())
	case "file":
		cast.Media(
			req.Message.GetFile(),
			req.Message.GetText(), // Max 512 characters !
		)
	}

	// Perform broadcast request
	err := c.do(&cast, &res)
	if err != nil {
		return err
	}

	err = res.Err()
	if err != nil {
		return err
	}

	// Populate failed peer(s) status
	for _, fail := range res.FailStatus {
		setError(fail.PeerId, fail.Err())
	}

	return nil
}
