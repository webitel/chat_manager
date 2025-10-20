package infobip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
)

// Infobip App Client
type App struct {
	*bot.Gateway              // internal
	*http.Client              // external
	media        *http.Client // HTTP Client for /media proxy
	apiToken     string       // APIKey: Authorization [required]
	baseURL      string       // BaseURL: https://*.api.infobip.com [required]
	keyword      string       // Keyword: the first word that appears in the message before the blank space [optional]
	number       string       // Number: Sender/Recipient Phone Number [optioonal]
}

const (
	providerType = "infobip_whatsapp" // "infobip"
)

func New(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {

	var (
		ok       bool
		app      = &App{Gateway: agent}
		metadata = agent.Bot.GetMetadata()
	)

	app.baseURL, ok = metadata["url"]
	if !ok || app.baseURL == "" {
		return nil, errors.BadRequest(
			"chat.gateway.infobip.url.required",
			"infobip: baseURL required but missing",
		)
	}
	baseURL, err := url.ParseRequestURI(app.baseURL)
	if err != nil || baseURL.Host == "" {
		return nil, errors.BadRequest(
			"chat.gateway.infobip.url.invalid",
			"infobip: baseURL is invalid",
		)
	}
	if !baseURL.IsAbs() {
		baseURL.Scheme = "https"
	}
	// baseURL.Scheme != ""
	// baseURL.Host != ""
	baseURL.User = nil
	baseURL.Opaque = ""
	baseURL.Path = ""
	baseURL.RawPath = ""
	baseURL.ForceQuery = false
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	// baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	app.baseURL = baseURL.String()

	app.apiToken, ok = metadata["api_key"]
	if !ok || app.apiToken == "" {
		return nil, errors.BadRequest(
			"chat.gateway.infobip.api_token.required",
			"infobip: API token required but missing",
		)
	}

	// FIXME: Rename to "sender"
	app.number, _ = metadata["number"]
	// if !ok || app.number == "" {
	// 	return nil, errors.BadRequest(
	// 		"chat.gateway.infobipWA.number.required",
	// 		"infobipWA: bot API number required",
	// 	)
	// }
	// TODO: Check Telephone Number is valid ?
	// 10DLC (10 Digit Long Code)
	for _, d := range app.number {
		switch {
		case '0' <= d && d <= '9':
		case '+' == d: // ???
		default:
			return nil, errors.BadRequest(
				"chat.gateway.infobip.number.invalid",
				"infobip: WhatsApp sender number is invalid",
			)
		}
	}
	switch len(app.number) {
	case 0: // Undefined
	case 10, 12: // 10DLC (10 Digit Long Code) +2 (Country Code)
	default:
		// return nil, errors.BadRequest(
		// 	"chat.gateway.infobip.number.invalid",
		// 	"infobip: WhatsApp sender number invalid length",
		// )
	}

	app.keyword, _ = metadata["keyword"]
	app.keyword = strings.TrimSpace(app.keyword)
	// if app.keyword == "" {
	// 	app.keyword = "webitel" // default
	// }

	// Update running configuration ?
	if run, _ := state.(*App); run != nil {
		// reset from NEW configuration
		run.number = app.number
		run.keyword = app.keyword
		run.baseURL = app.baseURL
		run.apiToken = app.apiToken
		run.Gateway = agent
		app = run // [re]use: current !
	}

	if app.Client == nil {
		// HTTP Client
		client := *(http.DefaultClient) // shallowcopy
		if client.Transport == nil {
			client.Transport = http.DefaultTransport
		}
		// Trace(!)
		client.Transport = &bot.TransportDump{
			Transport: client.Transport,
			WithBody:  true,
		}
		client.Timeout = time.Second * 15
		app.Client = &client
	}

	return app, nil
}

// Implementation
var (
	_ bot.Sender   = (*App)(nil)
	_ bot.Receiver = (*App)(nil)
	_ bot.Provider = (*App)(nil)
)

// String provider's code name
func (c *App) String() string {
	return providerType
}

func trimChars(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	var a, c int
	for a < len(text) {
		_, c = utf8.DecodeRuneInString(text[a:])
		a += c
		if max--; max == 0 {
			text = text[0:a]
			break
		}
	}
	return text
}

func coalesce(text ...string) string {
	for _, next := range text {
		next = strings.TrimSpace(next)
		if next != "" {
			return next
		}
	}
	return ""
}

func quickReplies(buttons []*chat.Buttons) []Button {

	var (
		n       = 3
		actions = make([]Button, 3)
	)

	n = 0
setup:
	for _, layout := range buttons {
		for _, button := range layout.Button {
			switch strings.ToLower(button.Type) {
			case "email", "mail":
			case "phone", "contact":
			case "location":
			// case "postback":
			case "reply", "postback":
				action := &actions[n]
				action.Type = "REPLY"
				action.Title = trimChars(coalesce(button.Text, button.Caption, button.Code), 20)
				action.ID = trimChars(coalesce(button.Code, button.Text, button.Caption), 256)
				if (n)++; n == 3 {
					break setup
				}
			case "url":
			default:
			}
		}
	}

	actions = actions[0:n]
	return actions
}

func (c *App) forwardFile(media *chat.File, recipient *bot.Channel) (*chat.File, error) {

	// CHECK: provided URL is valid ?
	href, err := url.ParseRequestURI(media.Url) // href

	if err != nil {
		return nil, errors.BadRequest(
			"bot.forward.media.url.invalid",
			"forward: content media URL invalid; %s", err,
		)
	}

	ok := href != nil

	ok = ok && href.IsAbs() // ok = ok && strings.HasPrefix(href.Scheme, "http")
	ok = ok && href.Host != ""

	if !ok {
		return nil, errors.BadRequest(
			"bot.forward.media.url.invalid",
			"forward: content media URL invalid;",
		)
	}

	// reset: normalized !
	// doc.Url = href.String()

	// CHECK: filename !
	if media.Name == "" {
		media.Name = path.Base(href.Path)
		switch media.Name {
		case "", ".", "/": // See: path.Base()
			return nil, errors.BadRequest(
				"bot.forward.media.filename.invalid",
				"forward: content media filename is missing or invalid",
			)
		}
	}

	// ctx := context.Background()
	req, err := http.NewRequest(
		http.MethodGet, media.Url, nil,
	)

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: FILE",
			slog.Any("error", err),
			slog.String("url", media.Url),
			slog.String("stage", "http.NewRequest()"),
		)
		return nil, err
	}

	req.Header.Set("Authorization", "App "+c.apiToken)
	// req.Header.Set("Content-Type", "application/json; chatset=utf-8")
	// req.Header.Set("Accept", "application/json")

	// HTTP Client
	httpClient := c.httpMediaClient()
	// PERFORM: Call Send API
	rsp, err := httpClient.Do(req)

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: FILE",
			slog.Any("error", err),
			slog.String("stage", "http.Client.Do()"),
		)
		return nil, err
	}

	defer rsp.Body.Close()

	// https://www.infobip.com/docs/api/channels/whatsapp/whatsapp-inbound-messages/download-whatsapp-inbound-media
	// statusOK := (200 <= rsp.StatusCode && rsp.StatusCode < 300)
	statusOK := (rsp.StatusCode == http.StatusOK)
	if !statusOK {

		var res SendResponse
		_ = json.NewDecoder(rsp.Body).Decode(&res)

		if res.Error != nil {
			// Infobip detailed error
			err = res.Error
		}

		if err == nil {
			// General HTTP error
			err = fmt.Errorf(
				"(#%d) %s", rsp.StatusCode, rsp.Status,
			)
		}

		// Failure ; NON-200 Status !
		return nil, err
	}

	// Content-Type
	if mediaType := rsp.Header.Get("Content-Type"); mediaType != "" {
		// Split: mediatype/subtype[;opt=param]
		if mediaType, _, re := mime.ParseMediaType(mediaType); re == nil {
			media.Mime = mediaType
		}
	}
	// Content-Length
	media.Size = rsp.ContentLength
	// Content-Disposition
	if disposition := rsp.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if filename := params["filename"]; filename != "" {
				// RFC 7578, Section 4.2 requires that if a filename is provided, the
				// directory path information must not be used.
				switch filename = filepath.Base(filename); filename {
				case ".", string(filepath.Separator):
					// invalid
				default:
					media.Name = filename
				}
			}
		}
	}
	// Generate unique filename
	var (
		filebase = time.Now().Format("2006-01-02_15-04-05") // combines media filename with the timestamp received
		filename = filepath.Base(media.Name)
		filexten = filepath.Ext(filename)
	)
	filename = filename[0 : len(filename)-len(filexten)]
	if mediaType := media.Mime; mediaType != "" {
		// Get file extension for MIME type
		var ext []string
		switch filexten {
		default:
			ext = []string{filexten}
		case "", ".":
			switch strings.ToLower(mediaType) {
			case "application/octet-stream":
				ext = []string{".bin"}
			case "image/jpeg": // IMAGE
				ext = []string{".jpg"}
			case "image/png":
				ext = []string{".png"}
			case "image/gif":
				ext = []string{".gif"}
			case "audio/mpeg": // AUDIO
				ext = []string{".mp3"}
			case "audio/ogg": // VOICE
				ext = []string{".ogg"}
			default:
				// Resolve for MIME type ...
				ext, _ = mime.ExtensionsByType(mediaType)
			}
		}
		// Split: mediatype[/subtype]
		var subType string
		if slash := strings.IndexByte(mediaType, '/'); slash > 0 {
			subType = mediaType[slash+1:]
			mediaType = mediaType[0:slash]
		}
		if len(ext) == 0 { // != 1 {
			ext = strings.FieldsFunc(
				subType,
				func(c rune) bool {
					return !unicode.IsLetter(c)
				},
			)
			for n := len(ext) - 1; n >= 0; n-- {
				if ext[n] != "" {
					ext = []string{
						"." + ext[n],
					}
					break
				}
			}
		}
		if n := len(ext); n != 0 {
			filexten = ext[n-1] // last
		}
		if filename == "" {
			filename = strings.ToLower(mediaType)
			switch mediaType {
			case "image", "audio", "video":
			default:
				filename = "file"
			}
		}
	}
	// Build unique filename
	if filename != "" {
		filename += "_"
	}
	filename += filebase
	if filexten != "" {
		filename += filexten
	}
	// Populate unique filename
	media.Name = filename
	metadata, err := c.Gateway.UploadFile(context.TODO(), 4096, media.Mime, media.Name, uuid.Must(uuid.NewRandom()).String(), rsp.Body)
	if err != nil {
		return nil, err
	}

	media.Id = metadata.Id
	media.Url = metadata.Url
	media.Size = metadata.Size
	media.Malware = metadata.Malware

	return media, nil
}

// channel := notify.Chat
// contact := notify.User
func (c *App) SendNotify(ctx context.Context, notify *bot.Update) error {

	var (
		channel = notify.Chat
		message = notify.Message
		binding map[string]string //TODO
	)

	bind := func(key, value string) {
		if binding == nil {
			binding = make(map[string]string)
		}
		binding[key] = value
	}

	// Resolve Chat Conversation Thread !
	chatID := channel.ChatID // [TO] Contact
	fromID := c.number       // [FROM] Sender (Default)
	switch props := channel.Properties.(type) {
	case map[string]string:
		// WhatsApp Business Phone Number IDentification
		fromID, _ = props[paramWhatsAppNumber]
	}

	// Prepare Send API Request
	sendRequest := SendRequest{
		From: fromID,
		To:   chatID,
	}

	var sendContent SendContent
	sentMessage := notify.Message

	switch sentMessage.Type { // notify.Event {
	case "text":
	case "file":

	// // case "edit":
	// // case "send":

	// // case "read":
	// // case "seen":

	// // case "kicked":
	// case "joined": // ACK: ChatService.JoinConversation()
	// case "left":   // ACK: ChatService.LeaveConversation()

	// // case "typing":
	// // case "upload":

	// // case "invite":
	// case "closed":
	default:
		c.Gateway.Log.Warn("INFOBIP: SEND",
			slog.String("content", sentMessage.Type),
			slog.String("error", "send: reaction not implemented"),
		)
		return nil // IGNORE
	}

	// Content: TEXT -or- FILE
	buttons := message.Buttons
	if buttons == nil {
		// FIXME: Flow "menu" application does NOT process .Inline buttons =(
		buttons = message.Inline
	}
	if replies := quickReplies(buttons); len(replies) != 0 {
		message := &InteractiveButtonsMessage{}
		if doc := sentMessage.File; doc != nil {
			header := &InteractiveHeader{
				// Type:     "DOCUMENT", // TEXT, IMAGE, VIDEO, DOCUMENT
				Type:     mediaType(doc.Mime),
				MediaURL: doc.Url,
			}
			message.Header = header
		}
		message.Action.Buttons = replies
		message.Body.Text = trimChars(sentMessage.Text, 1024)
		if message.Body.Text == "" {
			c.Gateway.Log.Error("INFOBIP: SEND",
				slog.String("error", "content: interactive.body.text required but missing"),
			)
			return nil
		}
		sendContent = message
	} else if doc := sentMessage.File; doc != nil {
		message := &SendMediaMessage{
			MediaURL:  doc.Url, // <= 2048
			MediaType: doc.Mime,
		}
		switch mediaType(doc.Mime) {
		case MediaImage, MediaVideo:
			message.Caption = trimChars(sentMessage.Text, 3000)
		case MediaAudio, "STICKER":
			// content.Caption = ""
		case MediaFile:
			message.Filename = trimChars(doc.Name, 240)
			message.Caption = trimChars(sentMessage.Text, 3000)
		}
		sendContent = message
	} else if text := sentMessage.Text; text != "" {
		message := &SendTextMessage{
			Text:       trimChars(text, 4096),
			PreviewURL: false,
		}
		sendContent = message
	}

	if sendContent == nil {
		c.Gateway.Log.Warn("INFOBIP: SEND",
			slog.String("error", "send: no content"),
		)
		return nil // IGNORE
	}
	sendRequest.Content = sendContent

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	err := enc.Encode(sendRequest)

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: SEND",
			slog.Any("error", err),
		)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, // "POST",
		c.baseURL+sendRequest.Content.endpoint(),
		buf, // strings.NewReader(jsonBody),
	)

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: SEND",
			slog.Any("error", err),
			slog.String("stage", "http.NewRequest()"),
		)
		return err
	}

	req.Header.Set("Authorization", "App "+c.apiToken)
	req.Header.Set("Content-Type", "application/json; chatset=utf-8")
	req.Header.Set("Accept", "application/json")

	// HTTP Client
	client := c.Client
	// if client == nil {
	// 	client = http.DefaultClient
	// }
	// PERFORM: Call Send API
	resp, err := client.Do(req)

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: SEND",
			slog.Any("error", err),
			slog.String("stage", "http.Client.Do()"),
		)
		return err
	}

	defer resp.Body.Close()
	var res SendResponse

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		c.Gateway.Log.Error("INFOBIP: SEND",
			slog.Any("error", err),
			slog.String("stage", "http.sendResponse()"),
		)
		return err
	}

	if res.Error != nil {
		err = res.Error
	}

	if err != nil {
		c.Gateway.Log.Error("INFOBIP: SEND",
			slog.Any("error", err),
		)
		return err
	}

	// TARGET[chat_id]: MESSAGE[message_id]
	bind(chatID, res.MessageID)
	// attach sent message external bindings
	if message.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		message.Variables = binding
	}
	// +OK
	return nil

	// code := resp.StatusCode
	// switch {
	// case 200 <= code && code < 300:
	// 	// OK
	// default:
	// 	re, _ := ioutil.ReadAll(resp.Body)
	// 	c.Gateway.Log.Error().
	// 	Int("code", code).
	// 	Str("error", string(re)).
	// 	Msg("INFOBIP: SEND")
	// }

	// body, err := ioutil.ReadAll(resp.Body)

	// if err != nil {
	// 	c.Gateway.Log.Err(err).Msg("INFOBIP: SEND")
	// 	return err
	// }

	// fmt.Print(string(body))

	// return nil
}

const (
	// WhatsApp Bot's Conversation [TO] Business Phone Number
	paramWhatsAppNumber = "whatsapp.number"
)

// WebHook callback http.Handler
//
// // bot := BotProvider(agent *Gateway)
// ...
// recv := Update{/* decode from notice.Body */}
// err = c.Gateway.Read(notice.Context(), recv)
//
//	if err != nil {
//		http.Error(res, "Failed to deliver .Update notification", http.StatusBadGateway)
//		return // 502 Bad Gateway
//	}
//
// reply.WriteHeader(http.StatusOK)
func (c *App) WebHook(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	// case http.MethodGet:
	// Not Implemented (!)
	case http.MethodPost:
		// Handle Updates below
	default:
		http.Error(w,
			"(405) Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
		return // (405) Method Not Allowed
	}

	// POST Webhook
	defer r.Body.Close()
	// https://www.infobip.com/docs/api#channels/whatsapp/receive-whatsapp-inbound-messages

	var (
		req = Updates{}
		ctx = r.Context()
		dec = json.NewDecoder(r.Body)
	)

	if err := dec.Decode(&req); err != nil {
		// switch e := err.(type) {
		// case *json.InvalidUTF8Error:
		// }
		c.Gateway.Log.Error("INFOBIP: UPDATE",
			slog.String("error", "decode: "+err.Error()),
		)
		// REDUCE [RE]DELIVERIES
		return // (200) OK
	}

	for _, recvUpdate := range req.Results {

		// Subscription(s):
		//
		// - DELIVERY
		// - CLICK
		// - IDENTITY_CHANGE
		// - ACCOUNT_REGISTRATION
		// - SEEN
		// - BILLING
		// - TEMPLATE_UPDATE
		// - PAYMENT_STATUS
		// - INBOUND_MESSAGE
		// - MARKETING_SUBSCRIPTION_UPDATE
		// - ACCOUNT_UPDATE_NOTIFICATION
		//

		// We reacts ON message(s) ONLY !
		if recvUpdate.Message == nil {
			continue // skip any other update(s) ...
		}

		sender := bot.Account{
			// Contact internal IDentifier
			ID: 0, // LOOKUP
			// Number which sent the message.
			Contact: recvUpdate.From, // chatID
			// // End user's phone number.
			// Contact:   recvUpdate.Message.Context.From, // chatID
			Channel: strings.ToLower(recvUpdate.Integration), // "whatsapp" // "infobip_whatsapp",
			// Information about recipient.
			FirstName: recvUpdate.Contact.Name,
		}
		// Find Contact's Chat Thread
		channel, err := c.Gateway.GetChannel(
			ctx, sender.Contact, &sender,
		)

		if err != nil {
			// Failed locate chat channel !
			re := errors.FromError(err)
			if re.Code == 0 {
				re.Code = (int32)(http.StatusBadGateway)
			}
			http.Error(w, re.Detail, (int)(re.Code))
			return // (503) Bad Gateway
		}

		var (
			recvMessage = recvUpdate.Message
			sendUpdate  = bot.Update{
				Title: channel.Title,
				Chat:  channel,
				User:  &sender,
			}
			sendMessage = &chat.Message{}
		)

		// WhatsApp Business SENT Mapping !
		props := map[string]string{
			// ChatID: MessageID
			sender.Contact: recvUpdate.MessageID,
		}
		// WhatsApp Chat Bindings ...
		if channel.IsNew() {
			// BIND Channel START properties !
			props[paramWhatsAppNumber] = recvUpdate.To
			channel.Properties = props
		} // else { // BIND Message SENT properties ! }
		sendMessage.Variables = props

		trimKeyword := func(text string) string {
			// KEYWORD: It is the first word that appears in the message before the blank space,
			// and that end user is asked to include to the message
			// they’re sending out to a company or institution.
			keyword := c.keyword
			if keyword == "" {
				return text
			}
			text = strings.TrimSpace(text)
			if text == "" {
				return text
			}
			if sp := strings.IndexFunc(text, unicode.IsSpace); sp > 0 {
				// caseIgnoreMatchPrefix()
				if caseIgnoreMatchN(text, keyword, len(keyword)) {
					text = strings.TrimLeftFunc(text[len(keyword):], unicode.IsSpace)
				}
			}
			return text
		}

		messageType := strings.ToUpper(recvMessage.Type)
		switch messageType {
		case "TEXT":

			sendMessage.Type = "text"
			sendMessage.Text = trimKeyword(recvMessage.Text)

		case "IMAGE",
			"AUDIO",
			"VOICE",
			"VIDEO",
			"STICKER",
			"DOCUMENT":

			sendMessage.Type = "file"
			// Types: [DOCUMENT]
			sendMessage.Text = trimKeyword(recvMessage.Caption)
			// sendMessage.File = &chat.File{
			// 	Url: recvMessage.URL,
			// }
			doc := &chat.File{
				Id:   0,
				Mime: "",
				// Name: sendMessage.Text, // .Caption (parsed above)
				Url:  recvMessage.URL,
				Size: 0,
			}

			doc.Name = strings.ToLower(messageType)
			switch messageType {
			// case "IMAGE":
			// case "AUDIO":
			// case "VOICE":
			// case "VIDEO":
			// case "STICKER":
			case "DOCUMENT":
				doc.Name = sendMessage.Text // .Caption (parsed above)
				sendMessage.Text = ""       // Clear; This is the document filename !
			}
			_, err = c.forwardFile(doc, channel)
			if err != nil {
				c.Gateway.Log.Error("INFOBIP: MEDIA",
					slog.Any("error", err),
				)
				sendMessage.Type = "text"
				sendMessage.Text = "[failed] get media content ; " + err.Error()
			} else {
				sendMessage.File = doc
			}
			// // Normalize filename
			// sendMessage.Text = doc.Name

		// case "CONTACT":
		case "LOCATION":
			// recvMessage.Longitude*
			// recvMessage.Latitude*
			// recvMessage.Location
			// recvMessage.Address
			// recvMessage.URL
			sendMessage.Type = "text"
			sendMessage.Text = fmt.Sprintf(
				"https://www.google.com/maps/place/%f,%f",
				recvMessage.Latitude, recvMessage.Longitude,
			)

		// case "BUTTON":
		// 	// recvMessage.Text
		// 	// recvMessage.Payload
		case "INTERACTIVE_BUTTON_REPLY":
			// recvMessage.CallbackData // buttonId
			// recvMessage.CallbackTitle // buttonTitle
			sendMessage.Type = "text"
			sendMessage.Text = recvMessage.CallbackData

		// case "INTERACTIVE_LIST_REPLY":

		case "CONTACT":
			// Convert given .Contacts to
			// human-readable .Text message
			buf := bytes.NewBuffer(nil)
			err := contactInfo.Execute(
				buf, recvMessage.Contacts,
			)
			if err != nil {
				buf.Reset()
				_, _ = buf.WriteString(err.Error())
			}
			// Resolve the first ContactInfo from the list
			sentContact := recvMessage.Contacts[0]
			sendContact := &chat.Account{
				// Id:        0,
				FirstName: sentContact.Name.FirstName,
				LastName: strings.Join([]string{
					sentContact.Name.MiddleName, sentContact.Name.LastName,
				}, " "),
			}

			if len(sentContact.Phones) != 0 {
				sendContact.Channel = "phone"
				sendContact.Contact = sentContact.Phones[0].Phone
			} else if len(sentContact.Emails) != 0 {
				sendContact.Channel = "email"
				sendContact.Contact = sentContact.Emails[0].Email
			} else {
				sendContact.Channel = "name"
				sendContact.Contact = sentContact.Name.FormattedName
			}

			sendMessage.Type = "contact" // "text"
			sendMessage.Text = buf.String()
			sendMessage.Contact = sendContact

		default:

			c.Gateway.Log.Warn("INFOBIP: RECV",
				slog.String("error", "message type not supported"),
				slog.String("to", recvUpdate.To),
				slog.String("from", recvUpdate.From),
				slog.String("type", strings.ToLower(recvMessage.Type)),
			)

			continue
		}

		sendUpdate.Message = sendMessage
		err = c.Gateway.Read(ctx, &sendUpdate)

		if err != nil {
			c.Gateway.Log.Error("INFOBIP: FORWARD",
				slog.Any("error", err),
			)
			http.Error(w,
				"Failed to deliver Message",
				http.StatusBadGateway,
			)
			return // (502) Bad Gateway
		}
	}

	// REDUCE [RE]DELIVERIES
	w.WriteHeader(http.StatusOK)
	// return // (200) OK
}

// Register webhook callback URI
// You need to setup callback URI to yours WhatsApp Number messages forwarding by yourself
func (c *App) Register(ctx context.Context, uri string) error {
	return nil
}

// Deregister webhook callback URI
func (c *App) Deregister(ctx context.Context) error {
	return nil
}

// Close shuts down bot and all it's running session(s)
func (c *App) Close() error {
	return nil
}

func init() {
	// Register Infobip Application WhatsApp provider
	bot.Register(providerType, New)
}
