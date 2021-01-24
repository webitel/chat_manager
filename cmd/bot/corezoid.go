package main

import (

	"time"
	"bytes"
	"context"
	"strings"
	"strconv"

	"net"
	"net/url"
	"net/http"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	// "github.com/golang/protobuf/proto"
	errs "github.com/micro/go-micro/v2/errors"

	"github.com/webitel/chat_manager/app"
	// gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"

)

func init() {
	// NewProvider(corezoid)
	Register("corezoid", NewCorezoidBot)
}

// chat request: command/message
type corezoidRequest struct {

	ChatID    string    `json:"id,omitempty"`          // [required] chat.channel.user.id
	Channel   string    `json:"channel,omitempty"`     // [required] underlaying provider name e.g.: telegram, viber, messanger (facebook), skype, slack

	Date      time.Time `json:"-"`                     // [internal] received local timestamp
	Event     string    `json:"action,omitempty"`      // [required] command request !
	Test      bool      `json:"test,omitempty"`        // [optional] bot development indicatior ! TOBE: removed in production !

	From      string    `json:"client_name,omitempty"` // [required] chat.username; remote::display
	Text      string    `json:"text,omitempty"`        // [optional] message text
	// {action:"purpose"} arguments
	ReplyWith string    `json:"replyTo,omitempty"`     // [optional] reply with back-channel type e.g.: chat (this), email etc.
}

// chat response: reply/event/message
type corezoidReply struct {
	 // outcome: response
	 Date      time.Time `json:"-"`                         // [internal] sent local timestamp
	 // {action:"chat"} => oneof {replyAction:(startChat|closeChat|answerToChat)} else ignore
	 Type      string    `json:"replyAction,omitempty"`     // [optional] update event type; oneof (startChat|closeChat|answerToChat)
	 From      string    `json:"operator,omitempty"`        // [required] chat.username; local::display
	 Text      string    `json:"answer,omitempty"`          // [required] message text payload
}

// channel runtime state
type corezoidChatV1 struct {
	//  // ChannelID (internal: Webitel)
	//  ChannelID string
	 // Request message; latest
	 corezoidRequest // json:",embedded"
	 // corresponding reply message
	 corezoidReply // json:",embedded"
}

// Corezoid Chat-Bot gateway runtime driver
type CorezoidBot struct {
	// URL to communicate with a back-channel service provider (proxy)
	URL string
	accessToken string // validate all incoming requests for precense X-Access-Token
	// Client HTTP to communicate with member, remote
	Client *http.Client
	// Gateway service agent
	Gateway *Gateway
}

// NewCorezoidBot initialize new chat service provider
// corresponding to agent.Profile configuration
func NewCorezoidBot(agent *Gateway) (Provider, error) {

	config := agent.Profile
	profile := config.GetVariables()

	host, _ := profile["url"]

	if host == "" {

		return nil, errs.BadRequest(
			"chat.gateway.corezoid.host_url.required",
			"corezoid: provider host URL required",
		)
	}

	hostURL, err := url.Parse(host)

	if err != nil {

		return nil, errs.BadRequest(
			"chat.gateway.corezoid.host_url.invalid",
			"corezoid:host: "+ err.Error(),
		)
	}
	// RESET: normalized !
	host = hostURL.String()
	// X-Access-Token: Authorization required ?
	authZ := profile["access_token"]

	// region: HTTP client/proxy
	var (

		client *http.Client
		proxy = profile["http_proxy"]
	)

	if proxy != "" {
		
		proxyURL, err := url.Parse(proxy)

		if err != nil {

			return nil, errs.BadRequest(
				"chat.gateway.corezoid.proxy_url.invalid",
				"corezoid: proxy: "+ err.Error(),
			)
		}

		// adding the proxy settings to the Transport object
		// Keep SYNC default values with net/http.DefaultTransport
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL), // ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		// adding the Transport object to the httpClient
		client = &http.Client{
			Transport: transport,
		}
	}

	// endregion

	return &CorezoidBot{

		URL:         host,
		accessToken: authZ,
		// NOTE: net/http.DefaultClient used if <nil> pointer
		Client:      client,
		Gateway:     agent,

	},  nil
}

// String "corezoid" provider name
func (_ *CorezoidBot) String() string {
	return "corezoid"
}

// Deregister NOT supported
func (_ *CorezoidBot) Deregister(ctx context.Context) error {
	return nil
}

// Register NOT supported
func (_ *CorezoidBot) Register(ctx context.Context, uri string) error {
	return nil
}

// WebHook implementes provider.Receiver interface
func (c *CorezoidBot) WebHook(reply http.ResponseWriter, notice *http.Request) {

	// region: X-Access-Token: Authorization
	if c.accessToken != "" {
		authzToken := notice.Header.Get("X-Access-Token")
		if authzToken != c.accessToken {
			http.Error(reply, "Invalid access token", http.StatusForbidden)
			c.Gateway.Log.Error().Str("error", "invalid access token").Msg("FORBIDDEN")
			return
		}
	}
	// endregion
	
	var (

		update corezoidRequest // command/message
		localtime = app.CurrentTime() // timestamp
	)

	if err := json.NewDecoder(notice.Body).Decode(&update); err != nil {
		log.Error().Err(err).Msg("Failed to decode update request")
		err = errors.Wrap(err, "Failed to decode update request")
		http.Error(reply, err.Error(), http.StatusBadRequest) // 400 
		return
	}

	if update.ChatID == "" {
		log.Error().Msg("Got request with no chat.id; ignore")
		http.Error(reply, "request: chat.id required but missing", http.StatusBadRequest) // 400
		return
	}

	update.Date = localtime

	// region: runtime state update
	state := &corezoidChatV1{
		corezoidRequest: update, // as latest
		// corezoidOutcome: {} // NULLify
	}
	// endregion

	c.Gateway.Log.Debug().

		Str("chat-id", update.ChatID).
		Str("channel", update.Channel).
		Str("action",  update.Event).
		Str("text",    update.Text).

	Msg("RECV update")

	// region: extract end-user contact info
	username := strings.TrimSuffix(update.From," ("+ update.Channel +")")
	username = strings.TrimSpace(username)
	if username == "" {
		username = "noname"
	}
	// fill account info
	contact := &Account{
		ID:        0, // LOOKUP
		FirstName: "",
		LastName:  "",
		Username:  username,
		Channel:   update.Channel,
		Contact:   update.ChatID,
	}
	// endregion
	
	// region: bind internal channel
	chatID := update.ChatID
	channel, err := c.Gateway.GetChannel(
		notice.Context(), chatID, contact,
	)

	if err != nil {
		c.Gateway.Log.Error().
			Str("error", "lookup: "+ err.Error()).
			Msg("CHANNEL")
		
		http.Error(reply,
			errors.Wrap(err, "Failed lookup chat channel").Error(),
			http.StatusInternalServerError, // HTTP 500 Internal Server Error
		)
		return
	}
	// RESET: Latest, NEW state !
	channel.Properties = state
	// endregion

	// region: init chat-flow-routine /start message environment variables
	props := map[string]string {
		"chat-id":     update.ChatID,
		"channel":     update.Channel,
		"action":      update.Event,
		"client_name": update.From,
		// "replyTo":  update.ReplyWith,
		"text":        update.Text,
		"test":        strconv.FormatBool(update.Test),
	}

	text := strings.TrimSpace(update.Text)
	sendMessage := &chat.Message{
		Id:    0, // NEW(!)
		Type: "text",
		Text:  text,
	}

	// region: receive file ...
	// NOTE: Messages with third-party link(s) are NOT delivered ! That's good !
	link := text
	if eol := strings.IndexByte(text,'\n'); eol > 7 { // http[s]://
		link = strings.TrimSpace(text[:eol])
		text = strings.TrimSpace(text[eol+1:])
	}
	// if link != "" { // NOTE: never ! We DO NOT allow empty text message(s) }
	if _, not := url.Parse(link); not == nil {
		// NOTE: We got valid URL;
		// This might be a file document source URL !
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url: link,
		}
		// Optional. Caption or description ...
		sendMessage.Text = text
	}
	// else { // TODO: nothing; We already assign message 'text' by default ! }
	// endregion

	// REACTION !
	switch update.Event {
	case "chat", "startChat": // incoming chat request (!)

		if update.Text == "" {
			// NOTE: We've got here, when consumer sent us any file document, except photo(s) !
			const notice = "Unfortunately, the transfer of third-party files" +
							" is prohibited for security reasons, except images"
			
							// FIXME: Ignore such update(s) ?
			// sendMessage.Type = "file"
			// sendMessage.File = &chat.File{
			// 	// TODO: some "Broken File" identification
			// 	Id:   0,
			// 	Url:  "",
			// 	Size: 0,
			// 	Mime: "",
			// 	Name: "",
			// }
			sendMessage.Type = "text"
			sendMessage.Text = notice
			
			defer func() {

				_ = c.SendNotify(context.Background(),
					&Update{
						ID:    0, // NOTICE
						Chat:  channel, // target
						User:  contact, // sender
						Title: username,
						Event: "text",
						Message: &chat.Message{
							
							Id:   0,
							Type: "text",
							Text: notice,
							File: nil,
							// Variables: nil,
							// Buttons: nil,
							// Contact: nil,
							CreatedAt: app.DateTimestamp(localtime),
							// UpdatedAt:        0,
							
							// ReplyToMessageId: 0, // !!!
							// ReplyToVariables: nil,
							// ForwardFromChatId:    "",
							// ForwardFromMessageId: 0,
							// ForwardFromVariables: nil,
						},
						// Edited:          0,
						// EditedMessageID: 0,
						// JoinMembers:     nil,
						// KickMembers:     nil,
					},
				)

			} ()
			// TODO: SendNotify("Unfortunately, the transfer of third-party files is prohibited for security reasons, except images")
		}

	case "closeChat":
		// TODO: break flow execution !
		if channel.IsNew() {

			channel.Log.Warn().Msg("CLOSE Request NO Channel; IGNORE")
			return // TODO: NOTHING !
		}

		channel.Log.Info().Msg("CLOSE External request; PERFORM")

		// DO: .CloseConversation(!)
		// cause := commandCloseRecvDisposiotion
		err = channel.Close() // (cause) // default: /close request
		
		if err != nil {
			// RESPOND (SEND): err: ${detail}
			http.Error(reply, errors.Wrap(err, "/close").Error(), http.StatusInternalServerError)
			return // 500 Internal Server Error
		}

		return

	case "Предложение", "Жалоба":

		// 1. "Дать ответ для отправки в telegram"
		// 2. "Отправить письмо на email ${box@domain.mx}"
		// 3. "Позвонить по тел. XXXXXXXXXXXX" // starts with country code !
		props["replyTo"] = update.ReplyWith

	default:
		// UNKNOWN !
		c.Gateway.Log.Warn().
			Str("error", update.Event +": reaction not implemented").
			Msg("IGNORE")

		return // HTTP/1.1 200 OK // to avoid redeliver !
	}

	sendMessage.Variables = props
	
	recvUpdate := Update {
	
		ID:      0, // NEW
		Title:   username,
		// ChatID: update.ChatID,
		Chat:    channel, // SENDER (!)
		// Contact: update.Channel,
		User:    contact,
		
		Event:   sendMessage.Type, // "text" or "file" !
		Message: sendMessage,
		// not applicable yet !
		Edited:           0,
		EditedMessageID:  0,
		
		// JoinMembersCount: 0,
		// KickMembersCount: 0,
	}

	// PERFORM: receive incoming update from external chat channel !
	err = c.Gateway.Read(notice.Context(), &recvUpdate)

	if err != nil {
		
		http.Error(reply, errors.Wrap(err, "Failed to deliver chat update").Error(), http.StatusInternalServerError)
		return // HTTP/1.1 500 Server Internal Error
	}

	reply.WriteHeader(http.StatusOK)
	// return // HTTP/1.1 200 OK
}

// SendNotify implements provider.Sender interface
func (c *CorezoidBot) SendNotify(ctx context.Context, notify *Update) error {

	var (

		localtime = app.CurrentTime()
		recepient = notify.Chat // recepient

		update = notify.Message
		chat *corezoidChatV1
	)

	// region: recover chat latest state
	switch props := recepient.Properties.(type) {
	case *corezoidChatV1:
		chat = props
	case map[string]string:
		develop, _ := strconv.ParseBool(props["test"])
		chat = &corezoidChatV1{
			corezoidRequest: corezoidRequest{
				ChatID:    recepient.ChatID,
				Channel:   props["channel"], // notify.User.Channel
				Date:      localtime, // RECOVERED(!)
				Event:     props["action"],
				Test:      develop,
				From:      props["client_name"],
				Text:      props["text"], // /start
				ReplyWith: props["replyTo"], // optional: action related attribute
			},
		}
		if recepient.Title == "" {
			// region: extract end-user contact info
			username := chat.corezoidRequest.From
			username = strings.TrimSuffix(username," ("+ chat.Channel +")")
			username = strings.TrimSpace(username)
			if username == "" {
				username = "noname"
			}
			// contact := &Account{
			// 	ID:        recepient.Account.ID, // MUST
			// 	FirstName: "",
			// 	LastName:  "",
			// 	Username:  username,
			// 	Channel:   chat.Channel,
			// 	Contact:   chat.ChatID,
			// }
			// fill account info
			recepient.Account.Channel = chat.Channel
			recepient.Account.Contact = chat.ChatID
			recepient.Account.Username = username
			// endregion
			recepient.Title = recepient.Account.DisplayName()
		}
		// RECOVERED !
		recepient.Properties = chat
	
	default:
	
		if recepient.Properties != nil {
			// FIXME: inform recepient that error occures ?
			return errs.InternalServerError(
				"chat.gateway.corezoid.channel.recover.error",
				"corezoid: channel %T recover %T state invalid",
				 chat, props,
			)
		}
	}
	// prepare reply message envelope !
	reply := &chat.corezoidReply
	reply.Date = localtime
	// represents operator's name for member side
	// TODO: How to get chat identity for some member side ?
	vars := update.GetVariables()
	// From: chat title in front of member
	title, _ := vars["operator"]
	if title == "" {
		title = "webitel:bot" // default
	}

	// region: event specific reaction !
	switch update.Type { // notify.Event {
	case "","text","file": // default
		// reaction:
		switch chat.corezoidRequest.Event {
		case "chat": // chatting

			// replyAction = startChat|closeChat|answerToChat
			reply.Type = "answerToChat"
			reply.From = title // TODO: resolve sender name

			// region: format reply text ...
			// File ?
			if file := update.File; file != nil {
				reply.Text = file.Url // FIXME: URL is blank ?
			}
			// Text ?
			if text := update.Text; text != "" {
				if reply.Text != "" { // Has URL ?
					reply.Text += "\n" + text // description or caption !
				} else {
					reply.Text = text // message text !
				}
			}
			// endregion

		// case "closeChat": // Requested ?
		// 	reply.Type = "closeChat"
		// 	reply.Text = update.GetText() // reply: message text
		// 	reply.From = title // TODO: resolve sender name
			
		case "Предложение", "Жалоба":
			
			reply.From = title // TODO: resolve sender name
			reply.Text = update.GetText() // reply: message text

		default:
			// panic(errors.Errorf("corezoid: send %q within %q state invalid", notify.Event, chat.corezoidRequest.Event))
			recepient.Log.Warn().Str("notice", 
				chat.corezoidRequest.Event + 
				": reaction to chat event=text not implemented",
			).Msg("IGNORE")

			return nil
		}
	
	// case "file":
	// case "send":
	// case "edit":
	// case "read":
	// case "seen":

	// case "joined":
	// case "kicked":

	// case "typing":
	// case "upload":

	// case "invite":
	case "closed":
		// SEND: typical text notification !
		switch chat.corezoidRequest.Event {
		case "chat", "closeChat":
			// replyAction = startChat|closeChat|answerToChat
			reply.Type = "closeChat"
			reply.Text = update.GetText() // reply: message text
			reply.From = title // TODO: resolve sender name
		
		default:
			
			recepient.Log.Warn().Str("notice", 
				chat.corezoidRequest.Event + 
				": reaction to chat event :closed: intentionally disabled",
			).Msg("IGNORE")
			return nil
		}
	
	default:

		recepient.Log.Warn().Str("notice", 
			"corezoid: reaction to chat event="+ notify.Event +" not implemented",
		).Msg("IGNORE")
		return nil
	}
	// endregion

	// encode result body
	body, err := json.Marshal(chat)
	if err != nil {
		// 500 Failed to encode update request
		return err
	}

	corezoidReq, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	corezoidReq.Header.Set("Content-Type", "application/json; chatset=utf-8")

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	// DO: SEND !
	res, err := client.Do(corezoidReq)
	
	if err != nil {
		return err
	}

	// _, err = ioutil.ReadAll(corezoidRes.Body)
	code := res.StatusCode
	if 200 <= code && code < 300 {
		// Success (!)
		// store latest context response
		// adjust := channel.corezoidOutcome // continuation for latest reply message -if- !adjust.Date.IsZero()
		chat.corezoidReply = *(reply) // shallowcopy
	
	} else {

		recepient.Log.Error().Int("code", code).Str("status", res.Status).Str("error", "send: failure").Msg("SEND")
	}
	
	return nil
}