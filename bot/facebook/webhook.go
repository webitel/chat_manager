package facebook

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"github.com/webitel/chat_manager/bot/facebook/messenger"
	"github.com/webitel/chat_manager/bot/facebook/webhooks"
	"github.com/webitel/chat_manager/internal/util"
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

			c.Log.Info("WEBHOOK: VERIFIED",
				slog.String("uri", uri),
			)
			return // 200 OK
		}

		http.Error(rsp,
			"webhook: verify token is invalid or missing",
			http.StatusForbidden,
		)

		c.Log.Error("WEBHOOK: NOT VERIFIED",
			slog.String("uri", uri),
			slog.String("error", "verify token is invalid or missing"),
		)
		return // 403 Forbidden

		// default:
		// fallthrough
	}

	http.Error(rsp,
		"webhook: setup mode is invalid or missing",
		http.StatusBadRequest,
	)

	c.Log.Error("WEBHOOK: NOT VERIFIED",
		slog.String("uri", uri),
		slog.String("error", "setup mode is invalid or missing"),
	)
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
		c.Log.Error("WEBHOOK",
			slog.Any("error", err),
		)
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
		c.Log.Error("WEBHOOK: EVEN",
			slog.Any("error", err),
		)
		return // (400) Bad Request
	}

	// X-Hub-Signature: Verification !
	if err = content.Close(); err != nil {
		http.Error(rsp, err.Error(), http.StatusBadRequest)
		c.Log.Error("WEBHOOK: INVALID SIGNATURE",
			slog.Any("error", err),
		)
		return // (400) Bad Request ! IGNORE !
	} // else {
	// 	c.Log.Debug().Msg("WEBHOOK: SIGNATURE VERIFIED")
	// }

	switch event.Object {
	case "page", "instagram":
		// Note that entry is an array and may contain multiple objects,
		// so ensure your code iterates over it to process all events.
		// https://developers.facebook.com/docs/messenger-platform/webhook#format
		var batch []*messenger.Entry
		if err = json.Unmarshal(data, &batch); err != nil {
			// 200 OK / IGNORE REDELIVERY
			rsp.WriteHeader(http.StatusOK)
			c.Log.Error("WEBHOOK: PAG",
				slog.Any("error", err),
			)
			// FIXME: Invalid subscribed object's field API version ?
			return
		}

		if event.Object == "page" {
			c.WebhookPage(batch)
		} else if event.Object == "instagram" {
			c.WebhookInstagram(batch)
		}

	// 1. Create Instagram Professional Or Business Account
	// 2. Connect Facebook Page to the created Instagram Account
	// 3. Setup and Subscribe Instagram Account(s) and it's connected Facebook Page(s)
	// 4. https://developers.facebook.com/docs/messenger-platform/instagram/get-started#connected-tools-toggle
	// case "instagram":

	// case "user":
	// case "permissions":

	// https://developers.facebook.com/docs/whatsapp/cloud-api/guides/set-up-webhooks
	// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components
	case "whatsapp_business_account":

		var batch []*webhooks.Entry
		if err = json.Unmarshal(data, &batch); err != nil {
			//
			c.Log.Error("meta.onWhatsAppBusinessAccount",
				slog.Any("error", err),
			)
			// 200 OK / IGNORE REDELIVERY
			rsp.WriteHeader(http.StatusOK)
			return
		}

		for _, entry := range batch {
			c.whatsAppOnUpdates(req.Context(), entry)
		}

	default:
		c.Log.Warn("messenger.onUpdate",
			slog.String("object", event.Object),
			slog.String("error", "update: object not supported"),
		)
	}

	// 200 OK / IGNORE [RE]DELIVERY !
	rsp.WriteHeader(http.StatusOK)
}

// func (c *Client) WebhookPermissions(batch []*webhooks.Entry) {
//
// }

func (c *Client) WebhookPage(batch []*messenger.Entry) {

	var (
		err error
		on  = "facebook.onUpdate"
	)
	for _, entry := range batch {
		if len(entry.Messaging) != 0 {
			// Array containing one messaging object.
			// Note that even though this is an array,
			// it will only contain one messaging object.
			// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#entry
			for _, event := range entry.Messaging {
				if event.Message != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
					on = "facebook.onMessage"
					err = c.WebhookMessage(event)
				} else if event.Postback != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
					on = "facebook.onPostback"
					err = c.WebhookPostback(event)
				} else if event.LinkRef != nil {
					//on = "facebook.onPostback"
					err = c.WebhookReferral(event)
				} // else {
				// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#event_list
				// }
			}

			// } else if len(entry.Standby) != 0 {
			// 	// Array of messages received in the standby channel.
			// 	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/standby
			// 	for _, event := range entry.Standby {
			//
			// 	}
			// }
		} else {
			on = "facebook.onUpdate"
			err = errors.BadRequest(
				"messenger.update.not_supported",
				"facebook: update event type not supported",
			)
		}

		if err != nil {
			re := errors.FromError(err)
			c.Gateway.Log.Error(on,
				slog.String("error", re.Detail),
			)
			err = nil
			// continue
		}
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
		pageASID, _ = props[paramFacebookPage]
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
		page = c.instagram.getPage(pageASID)
	}
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
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Id = "bot.messenger.chat.user.error"
			re.Code = (int32)(http.StatusBadGateway)
			re.Detail = "messenger: GET /" + userPSID + ".(*graph.User); " + re.Detail
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

func (c *Client) getPage(asid string) (page *Page) {
	page = c.pages.getPage(asid)
	if page == nil {
		page = c.instagram.getPage(asid)
	}
	return // page
}

func (c *Client) GetChannel(ctx context.Context, userPSID, pageASID string) (*bot.Channel, error) {

	page := c.getPage(pageASID)
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
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Id = "bot.messenger.chat.user.error"
			re.Code = (int32)(http.StatusBadGateway)
			re.Detail = "messenger: GET /" + userPSID + ".(*graph.User); " + re.Detail
		}
		// c.Log.Err(err).
		// Str("psid", userPSID).
		// Msg("MESSENGER: Get User Profile")
		return nil, re
	}

	sender := thread.User
	contact := bot.Account{
		ID:        0,           // LOOKUP
		FirstName: sender.Name, // sender.FirstName,
		// LastName:   sender.LastName,
		// NOTE: This is the [P]age-[S]coped User [ID]
		// For the same Facebook User, but different Pages
		// this value differs
		Channel: "facebook", // "messenger",
		Contact: sender.ID,
	}

	if pageASID == thread.Page.IGSID() {
		contact.Channel = "instagram"
	}

	// GET Chat
	chatID := userPSID // .sender.id
	channel, err := c.Gateway.GetChannel(
		ctx, chatID, &contact,
	)

	if err != nil {
		// Failed locate chat channel !
		re := errors.FromError(err)
		if re.Code == 0 {
			re.Code = (int32)(http.StatusBadGateway)
		}
		// http.Error(reply, re.Detail, (int)(re.Code))
		return nil, re // 503 Bad Gateway
	}

	// if channel.IsNew() {
	// 	channel.Properties = thread
	// 	// props := map[string]string{
	// 	// 	paramMessengerPage: page.ID,
	// 	// 	paramMessengerName: page.Name,
	// 	// }
	// 	// channel.Properties = props
	// }
	if dlg, ok := channel.Properties.(*Chat); !ok || dlg.User.ID != userPSID {
		channel.Properties = thread
	}

	return channel, nil
}

func (c *Client) mediaHead(media *chat.File) error {

	const method = http.MethodHead // HEAD
	req, err := http.NewRequestWithContext(
		context.TODO(), method,
		media.GetUrl(), nil,
	)

	if err != nil {
		return err
	}

	res, err := c.Client.Do(req)

	if err != nil {
		return err
	}
	// defer res.Body.Close()
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %d %s",
			method, res.StatusCode, res.Status,
		)
	}

	var (
		mediaType string
		header    = res.Header
	)
	mediaType, _, err = mime.ParseMediaType(
		header.Get("Content-Type"),
	)
	if err != nil {
		return err
	}
	if mediaType != "" {
		media.Mime = mediaType
	}
	if n := res.ContentLength; n > 0 {
		media.Size = n
	}

	return nil
}

// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
func (c *Client) WebhookMessage(event *messenger.Messaging) error {

	message := event.Message
	pageASID := event.Recipient.ID // [A]pp-[s]coped [ID] -or- [I]nsta[G]ram-[s]coped [ID]
	userPSID := event.Sender.ID    // [P]age-[s]coped [ID]
	// Ignore @self publication(s) !
	if message.IsEcho {
		// sender: event.Sender.ID; ECHO from out page publication
		fromId := userPSID
		sender := c.getPage(fromId)
		if sender == nil || sender.Page == nil {
			c.Gateway.Log.Warn("messenger.onMessage",
				slog.String("error", "account: page.id="+fromId+" not found"),
			)
		}
		platform := "facebook"
		pageName := sender.Name
		if sender.IGSID() == fromId {
			platform = "instagram"
			pageName = sender.Instagram.Name
			if pageName == "" {
				pageName = sender.Instagram.Username
			}
		}

		c.Gateway.Log.Warn(platform+".onMessage",
			slog.String("asid", fromId),
			slog.String(platform, pageName),
			slog.String("echo", "ignore: @self publication echo"),
		)
		// IGNORE // (200) OK
		return nil
	}

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
	dialog, _ := channel.Properties.(*Chat)
	platform := "facebook"
	facebook := dialog.Page // MUST: recipient
	pageName := facebook.Name
	instagram := facebook.Instagram
	if instagram != nil {
		platform = "instagram"
		pageName = instagram.Name
		if pageName == "" {
			pageName = instagram.Username
		}
	}

	// Facebook Message SENT Mapping !
	sentMsg := event.Message
	props := map[string]string{
		// ChatID: MessageID
		chatID: sentMsg.ID,
	}

	// update := bot.Update {
	// 	Title:   channel.Title,
	// 	Chat:    channel,
	// 	User:    &channel.Account,
	// 	Message: new(chat.Message),
	// }
	if message.IsDeleted {
		err = c.Gateway.DeleteMessage(
			context.TODO(), &bot.Update{
				Title: channel.Title,
				Chat:  channel,
				User:  &channel.Account,
				Message: &chat.Message{
					Id:        0,     // Internal MID: unknown;
					Variables: props, // Lookup: on external binding(s)
				},
			},
		)
		if err != nil {
			c.Gateway.Log.Warn(platform+".onMessageDelete",
				slog.Any("error", err),
				slog.String("asid", pageASID),
				slog.String(platform, pageName),
				slog.String("psid", userPSID),
				slog.String("from", channel.Account.DisplayName()),
			)
		}
		// return nil // (200) OK
		return err
	}

	// Spread multiple attachments into separate internal messages ...
	n := len(event.Message.Attachments)
	if n == 0 {
		n = 1
	}
	messages := make([]chat.Message, 1, n)
	// sendMsg := update.Message
	sendMsg := &messages[0]

	// Facebook Chat Bindings ...
	if channel.IsNew() {
		// BIND Channel START properties !
		thread, _ := channel.Properties.(*Chat)
		props[paramFacebookPage] = thread.Page.ID
		props[paramFacebookName] = thread.Page.Name
		var p = make(map[string]string)
		p[paramFacebookPage] = thread.Page.ID
		p[paramFacebookName] = thread.Page.Name
		p["facebook.psid"] = userPSID
		channel.Properties = p
		if instagram := thread.Page.Instagram; instagram != nil {
			props[paramInstagramPage] = instagram.ID
			props[paramInstagramUser] = instagram.Username
			p[paramInstagramPage] = instagram.ID
			p[paramInstagramUser] = instagram.Username
			channel.Properties = p
		}
	} // else { // BIND Message SENT properties ! }
	sendMsg.Variables = props

	// Defaults ...
	sendMsg.Type = "text"
	sendMsg.Text = sentMsg.Text
	//if event.Postback != nil && event.Postback.Referral != nil {
	//	ref := event.Postback.Referral
	//	switch ref.Source {
	//	case "SHORTLINK", "ADS":
	//		sendMsg.Text = ref.Ref
	//	}
	//}

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
		// Variables binding
		var vs map[string]string
		if re := replyTo.MessageID; re != "" {
			vs = map[string]string{
				// ChatID: MessageID
				chatID: re,
			}
		} else if re := replyTo.Story; re != nil {
			// ReplyTo [Instagram] Story
			vs = map[string]string{
				// ChatID: MessageID
				"story.id":  re.ID,
				"story.url": re.URL,
			}
		}
		sendMsg.ReplyToVariables = vs
	}
	// TODO: Separate each Attachment to individual internal message
	for _, doc := range sentMsg.Attachments {
		switch data := doc.Payload; doc.Type {
		// INSTAGRAM: Story @mention for oneof @yours connected page(s)
		// https://developers.facebook.com/docs/messenger-platform/instagram/features/webhook#webhook-events
		// See `messages` notification cases !
		case "story_mention":
			hook := c.hookIGStoryMention
			if hook == nil {
				c.Gateway.Log.Warn("instagram.onStoryMention",
					slog.String("error", "update: instagram{story_mentions} is disabled"),
					slog.String("asid", pageASID),
					slog.String(platform, pageName),
				)
				continue
			}
			// mention := IGStoryMention{
			// 	ID: sentMsg.ID,
			// 	Mention: StoryMention{
			// 		Link: data.URL,
			// 	},
			// }
			// // Build Story permalink !
			// dialog, _ := channel.Properties.(*Chat)
			// account := dialog.Page // @mention[ed]
			// story, err := c.fetchStoryMention(
			// 	context.TODO(), account,
			// 	// sentMsg.ID, data.URL,
			// 	&mention,
			// )
			// if err != nil {
			// 	c.Gateway.Log.Warn().
			// 		Str("error", "getMentionedStory: "+err.Error()).
			// 		Msg("instagram.onStoryMention")
			// 	// continue
			// }
			sendMsg.Type = "text"
			sendMsg.Text = "[@story]: " + data.URL
			// sendMsg.Type = "file"
			// sendMsg.Text = "#story_mention"
			// FIXME: How to GET mentioned Story permalink ?
			props[paramStoryMentionCDN] = data.URL
			// sendMsg.File = &chat.File{
			// 	Id:  -1, // DO NOT download VIA chat_manager service !
			// 	Url: data.URL,
			// }
			// // HEAD URL
			// // Mime: "image/jpeg",
			// // Name: "cdn_media_story.jpg",
			// // Size: 153403,``
			// err = c.mediaHead(sendMsg.File)
			// if err != nil {
			// 	c.Gateway.Log.Error().
			// 		Str("asid", pageASID).
			// 		Str(platform, pageName).
			// 		Str("error", "media: no definition; "+err.Error()).
			// 		Msg("instagram.onStoryMention")
			// }
			break

			// if story != nil {
			// 	doc.Type = strings.ToLower(story.MediaType)
			// 	props[paramStoryMentionText] = story.Caption
			// 	props[paramStoryMentionLink] = story.GetPermaLink()
			// }
			// props["knowledge_base"] = data.URL
			fallthrough // send story @mention as an inbox media message

		case "audio", "file", "video", "image":
			if doc.Type == "image" && data.StickerID != 0 {
				// Applicable to attachment type: image only if a sticker is sent.
				// - attach.StickerID
			}
			if data.URL == "" {
				// FIXME: This is the Sticker ?
				continue
			}

			// Check URL is valid else ignore that
			if !util.IsURL(data.URL) {
				// Invalid Attachment URL !
				c.Log.Error("ATTACHMENT: INVALID",
					slog.Any("error", err),
				)
				continue // NEXT !
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

			// Set file type and file struct to send message
			sendMsg.Type = "file"
			sendMsg.File = &chat.File{
				Id:  0, // TO Be downloaded ON chat_manager service !
				Url: data.URL,
			}

			// Fetch file details from URL or headers
			sendMsg.File.Mime, sendMsg.File.Size, err = fetchFileDetails(c.Client, data.URL)
			if err != nil {
				c.Log.Error("ATTACHMENT: INVALID; CANNOT FETCH FILE DETAILS",
					slog.Any("error", err),
				)
			}

			// If mimetype is still empty, use the document's type as default
			if sendMsg.File.Mime == "" {
				// doc.Type: [ "audio", "file", "video", "image" ]
				sendMsg.File.Mime = doc.Type
			}

			// If filename is still empty, use the default name 'file'
			sendMsg.File.Name = doc.Type + time.Now().UTC().Format("_2006-01-02_15-04-05")
			ext, _ := mime.ExtensionsByType(sendMsg.File.Mime)
			if n := len(ext); n > 0 {
				sendMsg.File.Name += ext[n-1]
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

	update := bot.Update{
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: nil, // new(chat.Message),
	}

	var re error
	n = len(messages)
	for i := 0; i < n; i++ {
		// Populate next prepared message
		sendMsg = &messages[i]
		switch sendMsg.Type {
		case "text", "":
			if sendMsg.Text == "" {
				continue // NO Message; IGNORE
			}
		case "file":
			if sendMsg.File == nil {
				continue // NO Media; IGNORE
			}
		}
		update.Message = sendMsg
		// Forward Bot received Message !
		err = c.Gateway.Read(ctx, &update)

		if err != nil {
			c.Gateway.Log.Error(platform+".onMessage",
				slog.Any("error", err),
				slog.String("asid", pageASID),
				slog.String(platform, pageName),
			)
			// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
			// return err // 502 Bad Gateway
			if re == nil {
				re = err // The First ONE !
			}
		}
	}

	return nil // re // First, if any occured !
}

// returns valid path.Base(rawpath) filename or none
// func getfilename(rawpath string) (filename string) {
// 	filename = path.Base(rawpath)
// 	ext := path.Ext(filename)
// 	if len(ext) < 2 { // ".+"
// 		// No .ext ; invalidate !
// 		return ""
// 	}
// 	name, _ := strings.CutSuffix(filename, ext)
// 	if name == "" || name[0] == '.' {
// 		// Hidden ; invalidate !
// 		return ""
// 	}
// 	return // filename
// }

// fetchFileDetails fetches file data by URL and returns filename, mimeType, size, and error
func fetchFileDetails(client *http.Client, link string) (mimetype string, size int64, err error) {
	if client == nil {
		client = http.DefaultClient
	}

	// Create new HEAD request to resource
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, link, nil)
	if err != nil {
		return
	}

	// Fetch file size and headers
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Get file MIME type from headers
	mimetype = resp.Header.Get("Content-Type")
	// mimetype, _, _ = mime.ParseMediaType(mimetype)
	// if mimetype == "" {
	// 	var filename string

	// 	// Get filename from Content-Disposition header
	// 	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
	// 		if _, params, err := mime.ParseMediaType(disposition); err == nil {
	// 			if filename = params["filename"]; filename != "" {
	// 				// RFC 7578, Section 4.2 requires that if a filename is provided, the
	// 				// directory path information must not be used.
	// 				switch filename = path.Base(filename); filename {
	// 				case ".", "/":
	// 					// invalid
	// 					filename = ""
	// 				default:
	// 					// OK
	// 				}
	// 			}
	// 		}
	// 	}

	// 	// If filename is not correctly extracted, try URL's path
	// 	if filename == "" {
	// 		// Parse the URL
	// 		var parsedURL *url.URL
	// 		parsedURL, err = url.Parse(link)
	// 		if err != nil {
	// 			return
	// 		}

	// 		// Get the file name from URL path
	// 		filename = path.Base(parsedURL.Path)

	// 		if !isFilename(filename) {
	// 			filename = ""
	// 		}
	// 	}

	// 	if ext := path.Ext(filename); ext != "" {
	// 		mimetype = mime.TypeByExtension(ext)
	// 	}
	// }

	// Get file size from headers
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err = strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return
		}
	}

	return
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
	update := bot.Update{
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
	if ref := sentMsg.Referral; ref != nil { // referral links come first https://developers.facebook.com/docs/messenger-platform/discovery/m-me-links
		switch ref.Source {
		case "SHORTLINK", "ADS":
			sendMsg.Text = ref.Ref
		}
	} else if sentMsg.Payload != "" {
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
		props[paramFacebookPage] = thread.Page.ID
		props[paramFacebookName] = thread.Page.Name
		var p = make(map[string]string)
		p[paramFacebookPage] = thread.Page.ID
		p[paramFacebookName] = thread.Page.Name
		p["facebook.psid"] = userPSID
		channel.Properties = p
		if instagram := thread.Page.Instagram; instagram != nil {
			props[paramInstagramPage] = instagram.ID
			props[paramInstagramUser] = instagram.Username
			p[paramInstagramPage] = instagram.ID
			p[paramInstagramUser] = instagram.Username
			channel.Properties = p
		}
	} // else { // BIND Message SENT properties ! }
	sendMsg.Variables = props

	// Forward Bot received Message !
	err = c.Gateway.Read(ctx, &update)

	if err != nil {
		c.Gateway.Log.Error(err.Error(),
			slog.Any("error", err),
		)
		// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
		return err // 502 Bad Gateway
	}

	return nil
}

func (c *Client) WebhookReferral(event *messenger.Messaging) error {

	userPSID := event.Sender.ID    // [P]age-[s]coped [ID]
	pageASID := event.Recipient.ID // [A]pp-[s]coped [ID]

	ctx := context.TODO()
	channel, err := c.getInternalThread(
		ctx, pageASID, userPSID,
	)

	if err != nil {
		return err
	}

	//chatID := userPSID
	update := bot.Update{
		Title:   channel.Title,
		Chat:    channel,
		User:    &channel.Account,
		Message: new(chat.Message),
	}

	sentMsg := event.Referral
	sendMsg := update.Message
	// Defaults ...
	sendMsg.Type = "text"
	sendMsg.Text = sentMsg.Ref
	// Facebook Message SENT Mapping !
	props := map[string]string{
		// ChatID: MessageID
		//chatID: event.,
	}

	// Facebook Chat Bindings ...
	if channel.IsNew() {
		// BIND Channel START properties !
		thread, _ := channel.Properties.(*Chat)
		props[paramFacebookPage] = thread.Page.ID
		props[paramFacebookName] = thread.Page.Name
		var p = make(map[string]string)
		p[paramFacebookPage] = thread.Page.ID
		p[paramFacebookName] = thread.Page.Name
		p["facebook.psid"] = userPSID
		channel.Properties = p
		if instagram := thread.Page.Instagram; instagram != nil {
			props[paramInstagramPage] = instagram.ID
			props[paramInstagramUser] = instagram.Username
			p[paramInstagramPage] = instagram.ID
			p[paramInstagramUser] = instagram.Username
			channel.Properties = p
		}
	} // else { // BIND Message SENT properties ! }
	sendMsg.Variables = props

	// Forward Bot received Message !
	err = c.Gateway.Read(ctx, &update)

	if err != nil {
		c.Gateway.Log.Error(err.Error(),
			slog.Any("error", err.Error()),
		)
		// http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
		return err // 502 Bad Gateway
	}

	return nil
}
