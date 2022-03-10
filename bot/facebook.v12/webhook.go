package facebook

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/micro/go-micro/v2/errors"
	"github.com/rs/zerolog/log"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"github.com/webitel/chat_manager/bot/facebook.v12/messenger"
	"github.com/webitel/chat_manager/bot/facebook.v12/webhooks"
)

// RandomBase64String of given n characters length
func RandomBase64String(n int) string {

	encoding := base64.RawURLEncoding
	buf := make([]byte, encoding.DecodedLen(n))
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic(err)
	}
	text := encoding.EncodeToString(buf)
	return text[:n]
}

func IsWebhookVerification(req url.Values) bool {

	return "subscribe" == req.Get("hub.mode")

	// if req.Method != http.MethodGet {
	// 	return false
	// }

	// query := req.URL.Query()

	// if query.Get("hub.mode") != "subscribe" {
	// 	return false
	// }

	// return query.Get("hub.verify_token") != ""
}

// https://developers.facebook.com/docs/messenger-platform/webhook#setup
func (c *Client) WebhookVerification(rsp http.ResponseWriter, req *http.Request) {

	if err := req.ParseForm(); err != nil {
		http.Error(rsp, err.Error(), http.StatusBadRequest)
		return
	}

	// LOG: Request URL without ?query= values
	uri := req.URL.Opaque
	if uri == "" {
		uri = req.URL.EscapedPath()
		if uri == "" {
			uri = "/"
		}
	} else {
		if strings.HasPrefix(uri, "//") {
			uri = req.URL.Scheme + ":" + uri
		}
	}

	switch req.Form.Get("hub.mode") {
	case "subscribe":
		
		hook := &c.webhook
		// TESTS PURPOSE !!! Uncomment for production !
		if req.Form.Get("hub.verify_token") == hook.Token {
			// SUCCEED
			hook.Verified = req.Form.Get("hub.challenge")
			rsp.WriteHeader(http.StatusOK)
			_, _ = rsp.Write([]byte(hook.Verified))

			c.Log.Info().
				Str("uri", uri).
				Msg("WEBHOOK: VERIFIED")
			return // 200 OK
		}

		http.Error(rsp,
			"webhook: verify token is invalid or missing",
			 http.StatusForbidden,
		)

		c.Log.Error().
			Str("uri", uri).
			Str("error", "verify token is invalid or missing").
			Msg("WEBHOOK: NOT VERIFIED")
		return // 403 Forbidden

	// default:
		// fallthrough
	}

	http.Error(rsp,
		"webhook: setup mode is invalid or missing",
		 http.StatusBadRequest,
	)

	c.Log.Error().
		Str("uri", uri).
		Str("error", "setup mode is invalid or missing").
		Msg("WEBHOOK: NOT VERIFIED")
	// return // 400 Bad Request
}

// WebhookEvent is the main Webhook update event Handler function
func (c *Client) WebhookEvent(rsp http.ResponseWriter, req *http.Request) {

	// defer func() {
		
	// 	req.Body.Close()
	// 	// (200) OK
	// 	code := http.StatusOK
	// 	rsp.WriteHeader(code)

	// } ()

	defer req.Body.Close()
	// Facebook-API-Version: v12.0
	content, err := webhooks.EventReader(
		[]byte(c.Config.ClientSecret), req,
	)

	if err != nil {
		http.Error(rsp, err.Error(), http.StatusBadRequest)
		c.Log.Err(err).Msg("WEBHOOK")
		return // (400) Bad Request
	}

	var (
		// Contents Payload
		data json.RawMessage
		// Update Event model
		event = webhooks.Event{
			Entry: &data,
		}
	)

	err = json.NewDecoder(content).Decode(&event)
	
	if err != nil {
		// TODO: FIXME: Broken model due to API version ?
		http.Error(rsp, "Failed to decode webhook event", http.StatusBadRequest)
		c.Log.Err(err).Msg("WEBHOOK: EVENT")
		return // (400) Bad Request
	}

	// X-Hub-Signature: Verification !
	if err = content.Close(); err != nil {
		http.Error(rsp, err.Error(), http.StatusBadRequest)
		c.Log.Err(err).Msg("WEBHOOK: INVALID SIGNATURE")
		return // (400) Bad Request ! IGNORE !
	} // else {
	// 	c.Log.Debug().Msg("WEBHOOK: SIGNATURE VERIFIED")
	// }

	switch event.Object {
	case "page":
		// Note that entry is an array and may contain multiple objects,
		// so ensure your code iterates over it to process all events.
		// https://developers.facebook.com/docs/messenger-platform/webhook#format
		var batch []*messenger.Entry
		if err = json.Unmarshal(data, &batch); err != nil {
			// 200 OK / IGNORE REDELIVERY
			rsp.WriteHeader(http.StatusOK)
			c.Log.Err(err).Msg("WEBHOOK: PAGE")
			// FIXME: Invalid subscribed object's field API version ?
			return
		}

		c.WebhookPage(batch)

	// case "user":
	// case "permissions":
	default:
		c.Log.Warn().Str("object", event.Object).Msg("WEBHOOK: NOT SUPPORTED")
	}

	// 200 OK / IGNORE [RE]DELIVERY !
	rsp.WriteHeader(http.StatusOK)
}

// func (c *Client) WebhookPermissions(batch []*webhooks.Entry) {
// 	
// }

func (c *Client) WebhookPage(batch []*messenger.Entry) {

	for _, entry := range batch {
		if len(entry.Messaging) != 0 {
			// Array containing one messaging object.
			// Note that even though this is an array,
			// it will only contain one messaging object.
			// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#entry
			for _, event := range entry.Messaging {
				if event.Message != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
					_ = c.WebhookMessage(event)
				} else if event.Postback != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
					_ = c.WebhookPostback(event)
				} // else {
				// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#event_list
				// }
			}

		} // else if len(entry.Standby) != 0 {
		// 	// Array of messages received in the standby channel.
		// 	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/standby
		// 	for _, event := range entry.Standby {
		// 		
		// 	}
		// }
	}
}

// Gets internal *bot.Channel to external *Chat(Page+User) mapping
// Mostly used for bot.SendMessage(SEND)
func (c *Client) getExternalThread(chat *bot.Channel) (*Chat, error) {

	var userPSID = chat.ChatID // chat.Account.Contact
	thread, ok := chat.Properties.(*Chat)
	if ok && thread != nil { // && thread.User.ID == userPSID {
		return thread, nil // Resolved & Attached !
	}
	// Resolve page ASID from given chat channel
	var pageASID string
	switch props := chat.Properties.(type) {
	case map[string]string:
		pageASID, _ = props[paramMessengerPage]
		// pageName, _ := props[paramMessengerName]
	}

	if pageASID == "" {
		// NOTE: We cannot determine Facebook conversation side(s)
		// It all starts from Messenger Page identification !..
		err := errors.BadRequest(
			"bot.messenger.chat.page.missing",
			"messenger: missing .page= reference for .user=%s conversation",
			 userPSID,
		)
		return nil, err
	}
	// Find the Messenger Page by [A]pp-[s]coped unique ID
	page := c.pages.getPage(pageASID)
	if page == nil {
		err := errors.NotFound(
			"bot.messenger.chat.page.not_found",
			"messenger: conversation .user=%s .page=%s not found",
			 userPSID, pageASID,
		)
		return nil, err
	}

	// GET Sender's Facebook User Profile
	thread, err := c.getChat(page, userPSID)
	
	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err); if re.Code == 0 {
			re.Id = "bot.messenger.chat.user.error"
			re.Code = (int32)(http.StatusBadGateway)
			re.Detail = "messenger: GET /"+userPSID+".(*graph.User); "+ re.Detail
		}
		// c.Log.Err(err).
		// Str("psid", userPSID).
		// Msg("MESSENGER: Get User Profile")
		return nil, re
	}

	chat.Properties = thread
	// Resolved & Attached
	return thread, nil
}

// Gets external *Chat(Page+User) to internal *bot.Channel mapping
// Mostly used for bot.Webhook(RECV)
func (c *Client) getInternalThread(ctx context.Context, pageASID, userPSID string) (*bot.Channel, error) {
	return c.GetChannel(ctx, userPSID, pageASID)
}


func (c *Client) GetChannel(ctx context.Context, userPSID, pageASID string) (*bot.Channel, error) {

	// page := c.getPageAccount(pageASID, "messages")
	page := c.pages.getPage(pageASID)
	if page == nil {
		err := errors.BadRequest(
			"bot.messenger.page.not_found",
			"messenger: account page=%s not found",
			 pageASID,
		)
		// c.Log.Warn().
		// Str("error", "page: not found").
		// Str("page-id", pageID).
		// Msg("MESSENGER: Get Page Account")
		return nil, err
	}

	// GET Sender Facebook User Profile
	thread, err := c.getChat(page, userPSID)
	
	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err); if re.Code == 0 {
			re.Id = "bot.messenger.chat.user.error"
			re.Code = (int32)(http.StatusBadGateway)
			re.Detail = "messenger: GET /"+userPSID+".(*graph.User); "+ re.Detail
		}
		// c.Log.Err(err).
		// Str("psid", userPSID).
		// Msg("MESSENGER: Get User Profile")
		return nil, re
	}

	sender := thread.User
	contact := bot.Account {
		ID:         0, // LOOKUP
		FirstName:  sender.Name, // sender.FirstName,
		// LastName:   sender.LastName,
		// NOTE: This is the [P]age-[S]coped User [ID]
		// For the same Facebook User, but different Pages
		// this value differs
		Channel:    "messenger",
		Contact:    sender.ID,
	}

	// GET Chat
	chatID := userPSID // .sender.id
	channel, err := c.Gateway.GetChannel(
		ctx, chatID, &contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err); if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		// http.Error(reply, re.Detail, (int)(re.Code))
		return nil, re // 503 Bad Gateway
	}

	if channel.IsNew() {
		channel.Properties = thread
		// props := map[string]string{
		// 	paramMessengerPage: page.ID,
		// 	paramMessengerName: page.Name,
		// }
		// channel.Properties = props
	}
	
	return channel, nil
}

// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
func (c *Client) WebhookMessage(event *messenger.Messaging) error {

	userPSID := event.Sender.ID    // [P]age-[s]coped [ID]
	pageASID := event.Recipient.ID // [A]pp-[s]coped [ID]

	ctx := context.TODO()
	channel, err := c.getInternalThread(
		ctx, pageASID, userPSID,
	)

	if err != nil {
		// TODO: Auto-respond with this error message
		// to the given chat message event !
		re := errors.FromError(err)
		_, _ = c.SendText(pageASID, userPSID, re.Detail)
		return err
	}

	chatID := userPSID
	// update := bot.Update {
	// 	Title:   channel.Title,
	// 	Chat:    channel,
	// 	User:    &channel.Account,
	// 	Message: new(chat.Message),
	// }

	// Spread multiple attachments into separate internal messages ...
	n := len(event.Message.Attachments)
	if n == 0 {
		n = 1
	}
	messages := make([]chat.Message, 1, n)

	sentMsg := event.Message
	// sendMsg := update.Message
	sendMsg := &messages[0]

	// Facebook Message SENT Mapping !
	props := map[string]string{
		// ChatID: MessageID
		chatID: sentMsg.ID,
	}
	// Facebook Chat Bindings ...
	if channel.IsNew() {
		// BIND Channel START properties !
		thread, _ := channel.Properties.(*Chat)
		props[paramMessengerPage] = thread.Page.ID
		props[paramMessengerName] = thread.Page.Name
	} // else { // BIND Message SENT properties ! }
	sendMsg.Variables = props

	// Defaults ...
	sendMsg.Type = "text"
	sendMsg.Text = sentMsg.Text

	// A quick_reply payload is only provided with a text message
	// when the user tap on a Quick Replies button.
	quickReply := sentMsg.QuickReply
	if quickReply != nil && quickReply.Payload != "" {
		// "user_phone_number": "message": {"text":"+380XXXXXXXXX","quick_reply":{"payload":"+380XXXXXXXXX"}
		// "user_email": "message": {"text":"box\u0040mx.example.com","quick_reply":{"payload":"box\u0040mx.example.com"}
		sendMsg.Text = quickReply.Payload
		// FIXME: Resolve value format, like: phone, email ?
		// And reinit message as type=contact ?
	}
	// MARK As a reply TO message
	replyTo := sentMsg.ReplyTo
	if replyTo != nil {
		bind := map[string]string{
			// ChatID: MessageID
			chatID: replyTo.MessageID,
		}
		sendMsg.ReplyToVariables = bind
	}
	// TODO: Separate each Attachment to individual internal message
	for _, doc := range sentMsg.Attachments {
		switch data := doc.Payload; doc.Type {
		case "audio", "file", "video", "image":
			if doc.Type == "image" && data.StickerID != 0 {
				// Applicable to attachment type: image only if a sticker is sent.
				// - attach.StickerID
			}
			if data.URL == "" {
				// FIXME: This is the Sticker ?
				continue
			}
			// Download: doc.URL
			url, err := url.Parse(data.URL)
			if err != nil {
				// Invalid Attachment URL !
				c.Log.Err(err).Msg("ATTACHMENT: INVALID")
				continue // NEXT !
			}

			name := path.Base(url.Path)
			switch name {
			case "/", ".":
				name = ""
			}

			if sendMsg.File != nil {
				// NOTE: This is the second or more files attached
				// Need to send as a separate internal messages ...
				n := len(messages)
				messages = append(messages, chat.Message{
					// INIT Defaults here ...
					Variables: props, // BIND The same initial message
				})
				sendMsg = &messages[n]
				// break // NOT Applicable yet !
			}

			sendMsg.Type = "file"
			sendMsg.File = &chat.File{
				Id:   0, // TO Be downloaded ON chat_manager service !
				Url:  data.URL,
				Mime: doc.Type,
				Name: name,
				Size: 0, // Unknown !
			}
			
		case "location":
			// Applicable to attachment type: location
			// - attach.Coordinates
		case "fallback":
			// Applicable to attachment type: fallback
			// - attach.Title 
		case "template":
		default:
			// UNKNOWN !
		}
	}

	update := bot.Update {
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: nil, // new(chat.Message),
	}

	var re error
	n = len(messages)
	for i := 0; i < n; i++ {
		// Populate next prepared message
		update.Message = &messages[i]
		// Forward Bot received Message !
		err = c.Gateway.Read(ctx, &update)
		
		if err != nil {
			log.Err(err).Msg("MESSENGER: FORWARD")
			// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
			// return err // 502 Bad Gateway
			if re == nil {
				re = err // The First ONE !
			}
		}
	}

	return nil // re // First, if any occured !
}

// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
func (c *Client) WebhookPostback(event *messenger.Messaging) error {

	userPSID := event.Sender.ID    // [P]age-[s]coped [ID]
	pageASID := event.Recipient.ID // [A]pp-[s]coped [ID]

	ctx := context.TODO()
	channel, err := c.getInternalThread(
		ctx, pageASID, userPSID,
	)

	if err != nil {
		return err
	}

	chatID := userPSID
	update := bot.Update {
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: new(chat.Message),
	}

	sentMsg := event.Postback
	sendMsg := update.Message
	// Defaults ...
	sendMsg.Type = "text"
	sendMsg.Text = sentMsg.Title
	if sentMsg.Payload != "" {
		sendMsg.Text = sentMsg.Payload
	}

	// Facebook Message SENT Mapping !
	props := map[string]string{
		// ChatID: MessageID
		chatID: sentMsg.MessageID,
	}

	// Facebook Chat Bindings ...
	if channel.IsNew() {
		// BIND Channel START properties !
		thread, _ := channel.Properties.(*Chat)
		props[paramMessengerPage] = thread.Page.ID
		props[paramMessengerName] = thread.Page.Name
	} // else { // BIND Message SENT properties ! }
	sendMsg.Variables = props

	// Forward Bot received Message !
	err = c.Gateway.Read(ctx, &update)
	
	if err != nil {
		log.Error().Msg(err.Error())
		// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
		return err // 502 Bad Gateway
	}

	return nil
}