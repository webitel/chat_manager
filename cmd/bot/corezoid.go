package main

import (

	"time"
	"bytes"
	"context"
	"strings"
	"strconv"
	"unicode"

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

// GetTitle extracts end-user's contact name, chat title
func (m *corezoidRequest) GetTitle() string {
	// crop trailing CHAT channel type suffix
	title := strings.TrimSuffix(m.From," ("+ m.Channel +")")
	title  = strings.TrimSpace(title)
	// if title == "" {
	// 	title = "noname"
	// }
	return title
}

// GetContact returns end-user contact info
func (m *corezoidRequest) GetContact() *Account {
	return &Account{
		ID:        0, // LOOKUP: UNKNOWN !
		Contact:   m.ChatID,
		Channel:   m.Channel,
		FirstName: m.GetTitle(),
		LastName:  "",
		Username:  "",
	}
}

// chat response: reply/event/message
type corezoidReply struct {
	 // outcome: response
	 Date      time.Time `json:"-"`                         // [internal] sent local timestamp
	 // {action:"chat"} => oneof {replyAction:(startChat|closeChat|answerToChat)} else ignore
	 Type      string    `json:"replyAction,omitempty"`     // [optional] update event type; oneof (startChat|closeChat|answerToChat)
	 FromID    int64     `json:"-"`                         // [optional] other side end-user's unique identifier
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

	if on, _ := strconv.ParseBool(profile["trace"]); on {
		var transport http.RoundTripper
		if client != nil {
			transport = client.Transport
		}
		if transport == nil {
			transport = http.DefaultTransport
		}
		transport = &transportDump{
			r: transport,
			WithBody: true,
		}
		if client == nil {
			client = &http.Client{
				Transport: transport,
			}
		} else {
			client.Transport = transport
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

// tries to decode latest provider-specific CHAT channel state ...
func corezoidChannelState(channel *Channel, start *corezoidChatV1) (state *corezoidChatV1, err error) {

	// region: restore latest CHAT state
	if hint := channel.Properties; hint != nil {

		switch hint := hint.(type) {
		// internal CHAT state !
		case *corezoidChatV1:
			state = hint
		// channel START state !
		case map[string]string:
			develop, _ := strconv.ParseBool(hint["test"])
			state = &corezoidChatV1{
				corezoidRequest: corezoidRequest{
					ChatID:    channel.ChatID,
					Channel:   hint["channel"], // notify.User.Channel
					Date:      app.CurrentTime(), // RECOVERED(!)
					Event:     hint["action"],
					Test:      develop,
					From:      hint["client_name"],
					Text:      hint["text"], // /start
					ReplyWith: hint["replyTo"], // optional: action related attribute
				},
				corezoidReply: corezoidReply{
					From:      hint["operator"],
				},
			}
			// RECOVER last interlocutor info !
			reply := &state.corezoidReply
			reply.FromID, reply.From = decodeInterlocutorInfo(reply.From)
			// // RECOVERED !
			// channel.Properties = state
		
		default: // (channel.Properties != nil ) !!!

			return nil, errors.Errorf(
				"corezoid: restore %T channel from %T state invalid",
				 state, hint,
			)
		
		// 	if channel.Properties != nil {
		// 		// FIXME: inform recepient that error occures ?
				// return nil, errs.InternalServerError(
				// 	"chat.gateway.corezoid.channel.recover.error",
				// 	"corezoid: channel %T restore %T state invalid",
				// 	 chat, hint,
				// )
		// 	}
		}
	}
	// endregion
	
	// region: merge latest with current states
	if start != nil {
		if state != nil {
			// TODO: MERGE !
			// region: end-user's contact name, chat title
			// NOTE: consecutive requests for the same subscriber
			//       in some cases come without a username,
			//       so let's try to fix it !
			if len(state.corezoidRequest.From) >
				len(start.corezoidRequest.From) {

				start.corezoidRequest.From =
					state.corezoidRequest.From
			}
			// endregion
			// region: chaining last interlocutor details
			current := &start.corezoidReply
			latest := &state.corezoidReply

			current.FromID, current.From =
				latest.FromID, latest.From
			// endregion
		}
		// RESET: NEW !
		state = start
		// channel.Properties = state

	} // else { // start == nil
	// 	if state == nil {

	// 	}
	// }
	// endregion

	if state == nil {
		// FIXME: No either last `state` restored nor `start` chat request !
		return nil, errors.Errorf(
			"corezoid: chat channel ID=%s state is missing",
			 channel.ChatID,
		)
	}

	// RESTORE: Normalize end-user contact info !
	switch channel.Title {
	case "","noname":
		// fill contact info
		contact := &channel.Account
		
		contact.Contact   = state.ChatID
		contact.Channel   = state.Channel
		contact.FirstName = state.corezoidRequest.GetTitle()
		
		channel.Title = contact.DisplayName()
	}

	// BIND: Chained CHAT state !
	channel.Properties = state

	// SUCCESS !
	return state, nil
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

	// region: runtime state update
	update.Date = localtime
	state := &corezoidChatV1{
		corezoidRequest: update, // as latest
		// corezoidOutcome: {} // NULLify
	}
	// endregion

	// region: extract end-user contact info
	contact := update.GetContact()
	// endregion

	c.Gateway.Log.Debug().

		Str("chat-id", update.ChatID).
		Str("channel", update.Channel).
		Str("action",  update.Event).
		Str("title",   contact.DisplayName()).
		Str("text",    update.Text).

	Msg("RECV Update")

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
			http.StatusInternalServerError,
		)

		return // HTTP 500 Internal Server Error
	}

	// Chain NEW channel state with latest ...
	state, err = corezoidChannelState(channel, state)

	if err != nil {

		c.Gateway.Log.Error().Err(err).
			Str("chat-id", update.ChatID).
			Str("channel", update.Channel).
			Msg("FAILED Restore corezoid chat state")

		http.Error(reply,
			errors.Wrap(err, "Failed restore chat channel state").Error(),
			http.StatusInternalServerError,
		)

		return // HTTP 500 Internal Server Error
	}
	// Restore broken contact info if missing !
	contact = &channel.Account
	// endregion

	// region: init chat-flow-routine /start message environment variables
	text := strings.TrimSpace(update.Text)
	sendMessage := &chat.Message{
		Id:    0,     // NEW(!)
		Type: "text", // DEFAULT
		Text:  text,
	}
	// FIXME: When consumer sent us any file document (except photo and video)
	//        we receive such broken message with blank text inside ! Deal with it !
	if text == "" {
		// NOTE: We've got here, when consumer sent us any file document, except photo or video !
		const notice = "Unfortunately, the transfer of third-party files" +
						" is prohibited for security reasons, except images and video"
		
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
			// Also NOTIFY sender about restriction !
			_ = c.SendNotify(
				context.Background(), &Update{
					ID:    0, // NOTICE
					Chat:  channel, // target
					User:  contact, // sender
					Title: channel.Title,
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
		
		// Ignore CHAT channel creation for broken /start message !
		if channel.IsNew() {
			// Release the request ...
			_, _ = reply.Write(nil)
			// code := http.StatusOK
			// reply.WriteHeader(code)
			return // HTTP 200 OK
		}
	}

	// region: receive file ...
	// NOTE: Messages with third-party link(s) are NOT delivered ! That's good !
	link := text
	if eol := strings.IndexFunc(text,unicode.IsSpace); eol > 6 { // http[s]://
		link = strings.TrimSpace(text[:eol]) // optional: trim right witespace(s)
		text = strings.TrimSpace(text[eol+1:]) // optional: trim left witespace(s)
	}
	// if link != "" { // NOTE: never ! We DO NOT allow empty text message(s) }

	href, _ := url.ParseRequestURI(link)
	
	ok := href != nil

	ok = ok && href.Host != ""
	// ok = ok && href.IsAbs()
	ok = ok && strings.HasPrefix(href.Scheme, "http")

	if ok {
		// NOTE: We got valid URL;
		// This might be a file document source URL !
		sendMessage.Type = "file"
		sendMessage.File = &chat.File{
			Url: href.String(), // link,
		}
		// 
		if strings.HasPrefix(text, link) {
			text = "" // hide file's hyperlink text
		}
		// Optional. Caption or description ...
		sendMessage.Text = text
	}
	// else { // TODO: nothing; We already assign message 'text' by default ! }
	// endregion

	props := map[string]string {
		"chat-id":     update.ChatID,
		"channel":     update.Channel,
		"action":      update.Event,
		"client_name": update.From,
		// "replyTo":  update.ReplyWith,
		"text":        update.Text,
		"test":        strconv.FormatBool(update.Test),
	}

	// REACTION !
	switch update.Event {
	// incoming chat message (!)
	case "chat","startChat":

		// Expected !

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

		return // HTTP 200 OK

	case "Предложение","Жалоба":

		// 1. "Дать ответ для отправки в telegram"
		// 2. "Отправить письмо на email ${box@domain.mx}"
		// 3. "Позвонить по тел. XXXXXXXXXXXX" // starts with country code !
		props["replyTo"] = update.ReplyWith

	default:
		// UNKNOWN !
		c.Gateway.Log.Warn().
			Str("error", update.Event +": reaction not implemented").
			Msg("IGNORE")

		return // HTTP 200 OK // to avoid redeliver !
	}
	// BIND channel START properties !
	if channel.IsNew() {
		sendMessage.Variables = props
	} // else { // BIND message properties ! }

	recvUpdate := Update {
	
		ID:      0, // NEW
		Title:   channel.Title,
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
		
		http.Error(reply,
			errors.Wrap(err, "Failed to deliver CHAT update").Error(),
			http.StatusInternalServerError,
		)
		return // HTTP 500 Internal Server Error
	}

	// HTTP 200 OK
	_, _ = reply.Write(nil)
	// reply.WriteHeader(http.StatusOK)
	// return
}

// SendNotify implements provider.Sender interface
func (c *CorezoidBot) SendNotify(ctx context.Context, notify *Update) error {

	var (

		localtime = app.CurrentTime()
		recepient = notify.Chat // recepient

		update = notify.Message
		// chat *corezoidChatV1
	)

	// region: recover chat latest state
	chat, err := corezoidChannelState(recepient, nil)

	if err != nil {

		re := errs.FromError(err)
		
		if re.Id == "" {
			code := http.StatusInternalServerError
			re.Id = "chat.corezoid.channel.restore.error"
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}

		c.Gateway.Log.Error().Str("error", re.Detail).
			Msg("FAILED Restore corezoid chat state")

		return re // HTTP 500 Internal Server Error
	}
	// endregion

	// prepare reply message envelope !
	reply := &chat.corezoidReply

	// region: event specific reaction !
	switch update.Type { // notify.Event {
	case "text","file": // default
		// reaction:
		switch chat.corezoidRequest.Event {
		case "chat","startChat": // chatting

			// replyAction = startChat|closeChat|answerToChat
			reply.Type = "answerToChat"
			// operator = FIXME: default to ?
			if reply.From == "" {
				reply.From = "bot"
			}
			// reply.Text = defined below !

		// case "closeChat": // Requested ?
		// 	reply.Type = "closeChat"
		// 	reply.Text = defined below !
			
		case "Предложение","Жалоба":

			// reply.Text = defined below !

		default:
			// panic(errors.Errorf("corezoid: send %q within %q state invalid", notify.Event, chat.corezoidRequest.Event))
			recepient.Log.Warn().Str("notice", 
				update.Type + ": reaction to " +
				chat.corezoidRequest.Event + " not implemented",
			).Msg("IGNORE")

			return nil // HTTP 200 OK
		}
	
	// case "file":
	// case "send":
	// case "edit":
	// case "read":
	// case "seen":

	case "joined": // ACK: ChatService.JoinConversation()

		newChatMember := update.NewChatMembers[0]
		if reply.FromID == 0 {
			// CACHE Update CHAT title for recepient !
			reply.FromID = newChatMember.GetId()
			reply.From   = newChatMember.GetFirstName()
			reply.From   = strings.TrimSpace(reply.From)
			// Extract the first word from user's display name; must be the given name
			if gn := strings.IndexFunc(reply.From, unicode.IsSpace); gn > 0 {
				reply.From = reply.From[0:gn]
			}
			// STORE result binding changed !
			update.Variables = map[string]string{
				"operator": encodeInterlocutorInfo(
					reply.FromID, reply.From,
				),
			}
		}
		// Ignore send, just update changes !
		return nil // +OK

	case "left":   // ACK: ChatService.LeaveConversation()

		leftChatMember := update.LeftChatMember
		if reply.FromID == leftChatMember.GetId() {
			// CACHE Cleanup interlocuter info !
			reply.FromID = 0
			reply.From   = "" // TODO: set default ! FIXME: "bot" ?
			// STORE Unbind channel properties !
			update.Variables = map[string]string{
				"operator": "",
			}
		}
		// Ignore send, just update changes !
		return nil // +OK

	// case "typing":
	// case "upload":

	// case "invite":
	case "closed":
		// SEND: typical text notification !
		switch chat.corezoidRequest.Event {
		// FIXME: Should we send "closeChat" [ACK]nowledge ?
		case "chat","closeChat":
			// replyAction = startChat|closeChat|answerToChat
			reply.Type = "closeChat"
			// reply.Text = defined below !
			
			reply.FromID = 0
			reply.From = ""
		
		default:
			
			recepient.Log.Warn().Str("notice",
				update.Type + ": reaction to " +
				chat.corezoidRequest.Event + " intentionally disabled",
			).Msg("IGNORE")

			return nil
		}
	
	default:

		recepient.Log.Warn().Str("notice",
			update.Type + ": reaction to " +
			chat.corezoidRequest.Event + " not implemented",
		).Msg("IGNORE")

		return nil
	}
	// endregion

	// region: format message text ...
	reply.Date = localtime // set reply timestamp
	reply.Text = "" // cleanup latest reply text !
	// File ?
	if doc := update.GetFile(); doc != nil {
		reply.Text = doc.Url // FIXME: URL is blank ?
	}
	// Text ?
	if txt := update.GetText(); txt != "" {
		if reply.Text != "" { // Has URL ?
			// Caption or comment !
			reply.Text += "\n" + txt
		} else {
			// Text message !
			reply.Text = txt
		}
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
		// // Success (!)
		// // store latest context response
		// // adjust := channel.corezoidOutcome // continuation for latest reply message -if- !adjust.Date.IsZero()
		// chat.corezoidReply = *(reply) // shallowcopy
	} else {

		recepient.Log.Error().Int("code", code).Str("status", res.Status).Str("error", "send: failure").Msg("SEND")
	}

	// OK
	return nil
}

func encodeInterlocutorInfo(oid int64, name string) (contact string) {
	contact = strings.TrimSpace(name)
	if oid != 0 {
		if len(contact) != 0 {
			contact += " "
		}
		contact += "<"+ strconv.FormatInt(oid, 10) +">"
	}
	return // contact
}

func decodeInterlocutorInfo(contact string) (oid int64, name string) {

	name = contact
	i := strings.LastIndexByte(name, '<')
	if i != -1 && name[len(name)-1] == '>' {
		oid, _ = strconv.ParseInt(name[i+1:len(name)-1], 10, 64)
		name = strings.TrimSpace(name[:i])
	}
	return // oid, name
}