package facebook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook.v12/messenger"
	"github.com/webitel/chat_manager/bot/facebook.v12/webhooks"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	providerType = "messenger" // "facebook"

	PageInboxApplicationID = "263902037430900"

	// Messenger Bot's Conversation WITH Facebook Page ID
	paramMessengerPage = "messenger_page"
	// Messenger Bot's Conversation WITH Facebook Page Name
	paramMessengerName = "messenger_name"
	// Messenger Bot's Conversation WITH Instagram Page ID
	paramInstagramPage = "instagram_page"
	// Messenger Bot's Conversation WITH Instagram Username
	paramInstagramName = "instagram_name"
)

func init() {
	// Register Facebook Messenger Application provider
	bot.Register(providerType, NewV2) // New)
}

// Implementation
var (
	_ bot.Sender   = (*Client)(nil)
	_ bot.Receiver = (*Client)(nil)
	_ bot.Provider = (*Client)(nil)
)

func NewV2(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {

	// agent: NEW config (to validate provider settings integrity)
	// state: RUN config (current to grab internal state if needed)
	// current, _ := state.(*Client)

	metadata := agent.Bot.Metadata
	if len(metadata) == 0 {
		return nil, fmt.Errorf("messenger: bot setup metadata is missing")
	}

	client := *http.DefaultClient
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	client.Transport = &bot.TransportDump{
		Transport: client.Transport,
		WithBody:  true,
	}
	client.Timeout = time.Second * 15

	const version = "v13.0"

	app := &Client{

		Gateway: agent,
		Client:  &client,
		Version: version,

		Config: oauth2.Config{
			ClientID:     metadata["client_id"],
			ClientSecret: metadata["client_secret"],
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://www.facebook.com" + path.Join("/", version, "/dialog/oauth"),
				TokenURL:  "https://graph.facebook.com" + path.Join("/", version, "/oauth/access_token"),
				AuthStyle: oauth2.AuthStyleInParams,
			},
			RedirectURL: agent.CallbackURL(),
			Scopes:      []string{"public_profile"},
		},

		webhook: webhooks.WebHook{
			URL:   agent.CallbackURL(), // "https://dev.webitel.com" + path.Join("/chat/ws8/messenger"),
			Token: metadata["verify_token"],
		},
	}

	creds := clientcredentials.Config{
		ClientID:       app.Config.ClientID,
		ClientSecret:   app.Config.ClientSecret,
		TokenURL:       app.Config.Endpoint.TokenURL,
		Scopes:         nil, // []string{"public_profile"},
		EndpointParams: nil, // url.Values{},
		AuthStyle:      app.Config.Endpoint.AuthStyle,
	}

	app.creds = creds.TokenSource(context.WithValue(
		context.Background(), oauth2.HTTPClient, &client,
	))

	if current, _ := state.(*Client); current != nil {

		app.chatMx = current.chatMx
		app.chats = current.chats

		app.pages = current.pages
		// app.facebook  = current.facebook
		app.instagram = current.instagram

		if app.Config.ClientSecret == current.Config.ClientSecret {
			app.proofMx = current.proofMx
			app.proofs = current.proofs
		}

	} else { // INIT

		app.chatMx = new(sync.RWMutex)
		app.chats = make(map[string]Chat)

		app.pages = &messengerPages{
			pages: make(map[string]*Page),
		}
		app.instagram = &messengerPages{
			pages: make(map[string]*Page),
		}
		// backwards capability
		if ds, legacy := metadata["accounts"]; legacy {
			if _, latest := metadata["fb"]; !latest {
				metadata["fb"] = ds // POPULATE
			}
			// delete(metadata, "accounts")
			// OVERRIDE OR DELETE
			_ = agent.SetMetadata(
				context.TODO(), map[string]string{
					"fb":       metadata["fb"],
					"accounts": "",
				},
			)
			// PULL Updats !
			metadata = agent.Bot.Metadata
		}
		if s := metadata["fb"]; s != "" {
			encoding := base64.RawURLEncoding
			data, err := encoding.DecodeString(s)
			if err == nil {
				err = app.pages.restore(data)
			}
			if err != nil {
				app.Log.Err(err).Msg("MESSENGER: ACCOUNTS")
			}
		}
		if s := metadata["ig"]; s != "" {
			encoding := base64.RawURLEncoding
			data, err := encoding.DecodeString(s)
			if err == nil {
				err = app.instagram.restore(data)
			}
			if err != nil {
				app.Log.Err(err).Msg("INSTAGRAM: ACCOUNTS")
			}
		}
	}

	if app.proofs == nil {
		app.proofMx = new(sync.Mutex)
		app.proofs = make(map[string]string)
	}

	return app, nil
}

// String provider's code name
func (c *Client) String() string {
	return providerType
}

func scanTextPlain(s string, max int) string {
	var (
		d, c int
		rs   []byte
		n    = max
	)
	for i, r := range s {

		// switch {
		// case unicode.IsPrint(r):
		// case unicode.IsDigit(r):
		// case unicode.IsLetter(r):
		// case unicode.IsNumber(r):
		// case unicode.IsPunct(r):
		// case unicode.IsSpace(r):
		// default:
		// }
		if !unicode.IsSymbol(r) && unicode.IsPrint(r) && (n < max || !unicode.IsSpace(r)) {
			if n--; n < 0 {
				if d != 0 {
					rs = rs[0 : i-d]
				} else {
					s = s[0 : i+utf8.RuneLen(r)]
				}
				break // limit exceeded
			}
			continue
		}
		// remove invalid character
		if rs == nil {
			rs = []byte(s)
		}
		c = utf8.RuneLen(r)
		rs = append(rs[0:i-d], rs[i-d+c:]...)
		d += c
	}
	if rs != nil {
		s = string(rs)
	}
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

// channel := notify.Chat
// contact := notify.User
func (c *Client) SendNotify(ctx context.Context, notify *bot.Update) error {

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
	// Resolve Facebook conversation (User+Page)
	// by channel.Account.Contact [P]age-[s]coped User [ID]
	chatID := channel.ChatID // channel.Account.Contact
	recipientUserPSID := chatID
	chat, err := c.getExternalThread(channel)
	if err != nil {
		defer channel.Close()
		return err
	}

	if chat == nil || chat.Page == nil {
		err := errors.NotFound(
			"bot.messenger.send.chat.not_found",
			"messenger: send TO.user=%s FROM.page=? not found",
			recipientUserPSID,
		)
		// return err
		c.Log.Err(err).Msg("MESSENGER: SEND")
		return nil
	}

	// Prepare SendAPI Request
	sendRequest := messenger.SendRequest{
		// https://developers.facebook.com/docs/messenger-platform/send-messages/#messaging_types
		Type: "RESPONSE",
		Recipient: &messenger.SendRecipient{
			ID: recipientUserPSID,
		},
		Message: new(messenger.SendMessage),
		// Notify: "REGULAR",
		// Tag: "",
	}

	sendMessage := sendRequest.Message

	coalesce := func(s ...string) string {
		for _, v := range s {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
		return ""
	}

	// Transform from internal to external message structure
	switch message.Type {
	case "text", "":
		// Text Message !
		sendMessage.Text = message.Text

		menu := message.Buttons
		if menu == nil {
			// FIXME: Flow "menu" application does NOT process .Inline buttons =(
			menu = message.Inline
		}
		// if menu := message.Buttons; menu != nil { // len(menu) != 0 {
		// 	// newReplyKeyboardFb(message.GetButtons(), &reqBody.Message);
		// 	// See https://developers.facebook.com/docs/messenger-platform/send-messages/buttons

		// }
		if /*menu := message.Inline;*/ len(menu) != 0 {
			// newInlineboardFb(message.GetInline(), &reqBody.Message, message.Text);
			// See https://developers.facebook.com/docs/messenger-platform/reference/buttons/quick-replies#quick_reply
			var (
				buttons   []*messenger.Button
				replies   []*messenger.QuickReply
				instagram = (chat.Page.IGSID() != "")
			)
			for _, row := range menu {
				for _, src := range row.Button {
					// Caption string
					// Text    string
					// Type    string
					// Code    string
					// Url     string
					switch strings.ToLower(src.Type) {
					case "email", "mail": // https://developers.facebook.com/docs/messenger-platform/send-messages/quick-replies#email
						if instagram {
							continue
						} // NOT Supported !
						replies = append(replies, &messenger.QuickReply{
							Type: "user_email",
						})
					case "phone", "contact": // https://developers.facebook.com/docs/messenger-platform/send-messages/quick-replies#phone
						if instagram {
							continue
						} // NOT Supported !
						replies = append(replies, &messenger.QuickReply{
							Type: "user_phone_number",
						})
					case "location": // https://developers.facebook.com/docs/messenger-platform/send-messages/quick-replies#locations
						// March 16, 2022
						// Error: (#100) Location Quick Reply is now deprecated on API 4.0. Please refer to our Developer Documentation for more info.
						// https://developers.facebook.com/docs/messenger-platform/changelog/#20190610
						//
						// June 10, 2019 (Changes)
						// - Location quick reply which allows people to send their location in the Messenger thread will no longer be rendered.
						// We recommend businesses ask for zip code and address information within the thread.
						// While we are sunsetting the existing version of Share Location,
						// in the coming months we will be introducing new ways for people to communicate their location to businesses in more valuable ways.

						// replies = append(replies, &messenger.QuickReply{
						// 	Type: "location",
						// })
					case "postback": // https://developers.facebook.com/docs/messenger-platform/send-messages/buttons#postback
						// Buttons !
						buttons = append(buttons, &messenger.Button{
							Type:    "postback",
							Title:   scanTextPlain(coalesce(src.Caption, src.Text), 21),
							Payload: scanTextPlain(coalesce(src.Code, src.Text), 1000),
						})
					case "url": // https://developers.facebook.com/docs/messenger-platform/send-messages/buttons#button-format
						buttons = append(buttons, &messenger.Button{
							Type:  "web_url",
							Title: scanTextPlain(coalesce(src.Caption, src.Text), 21),
							URL:   src.GetUrl(),
						})
					default: // https://developers.facebook.com/docs/messenger-platform/send-messages/quick-replies#text
						// case "reply", "text":
						// [Instagram] See: https://developers.facebook.com/docs/messenger-platform/instagram/features/quick-replies
						// A maximum of 13 quick replies are supported and each quick reply allows up to 20 characters before being truncated.
						// Quick replies only support plain text.
						replies = append(replies, &messenger.QuickReply{
							Type: "text",
							// Required if content_type is 'text'.
							// The text to display on the quick reply button.
							// 20 character limit.
							Title: scanTextPlain(coalesce(src.Caption, src.Text), 21),
							// Required if content_type is 'text'.
							// 1000 character limit.
							Payload: scanTextPlain(coalesce(src.Code, src.Text), 1000),
							// Required if title is an empty string. Image should be a minimum of 24px x 24px.
							ImageURL: src.Url,
						})
					}
				}
			}
			// (#100) Only one of the text, attachment, and dynamic_text fields can be specified
			if len(replies) != 0 { // A maximum of 13 quick replies are supported
				sendMessage.QuickReplies = replies
			}

			if len(buttons) != 0 {
				if !instagram {
					// Facebook(!)
					// (#100) Only one of the text, attachment, and dynamic_text fields can be specified
					sendMessage.Text = "" // NULLify !
					sendMessage.Attachment = &messenger.SendAttachment{
						Type: "template",
						Payload: &messenger.TemplateAttachment{
							TemplateType: "button",
							ButtonTemplate: &messenger.ButtonTemplate{
								Text:    coalesce(message.Text, "Де текст ?"),
								Buttons: buttons,
							},
						},
					}
				} else {
					// Instagram(!)
					sendMessage.Text = "" // NULLify !
					sendMessage.QuickReplies = nil
					sendMessage.Attachment = &messenger.SendAttachment{
						Type: "template",
						Payload: &messenger.TemplateAttachment{
							TemplateType: "generic",
							GenericTemplate: &messenger.GenericTemplate{
								Elements: []*messenger.GenericElement{
									{
										Title:   coalesce(message.Text, "Де текст ?"),
										Buttons: buttons,
									},
								},
							},
						},
					}
				}
			}
		}

	case "file":
		// newFileMessageFb(message.GetFile(), &reqBody.Message)
		// mime.ParseMediaType()
		sendAttachment := &messenger.SendAttachment{
			Type: "file", // default
		}
		sentAttachment := message.File
		for _, mediaType := range []string{
			"image", "audio", "video",
		} {
			if strings.HasPrefix(sentAttachment.Mime, mediaType) {
				sendAttachment.Type = mediaType
				break
			}
		}

		sendAttachment.Payload = messenger.FileAttachment{
			URL:        sentAttachment.Url,
			IsReusable: false,
		}

		sendMessage.Attachment = sendAttachment

	// case "send":
	// case "edit":
	// case "read":
	// case "seen":

	// https://developers.facebook.com/docs/messenger-platform/send-messages/personas
	// case "joined": // NEW Member(s) joined the conversation
	// newChatMember := message.NewChatMembers[0]
	// persona := graph.Persona{
	// 	Name: newChatMember.GetFullName(),
	// 	PictureURL: "",
	// }
	// // https://developers.facebook.com/docs/messenger-platform/send-messages/personas#create
	// // POST https://graph.facebook.com/me/personas?access_token=<PAGE_ACCESS_TOKEN>
	// // {
	// // 	"name": "John Mathew",
	// // 	"profile_picture_url": "https://facebook.com/john_image.jpg",
	// // }
	// // ----------------------------------------------------------------------------
	// // {
	// // 	"id": "<PERSONA_ID>"
	// // }
	// // ----------------------------------------------------------------------------
	// // Note: persona_id is a optional property.
	// // If persona_id is not included, the message will be sent normally.
	// sendRequest.PersonaID = string

	// case "left": // Someone left the conversation thread !

	case "closed":
		sendMessage.Text = message.GetText()

	default:
		c.Log.Warn().
			// Str("type", message.Type).
			Str("error", "message: type="+message.Type+" not implemented yet").
			Msg("MESSENGER: SEND")
		return nil
	}

	messageID, err := c.Send(chat.Page, &sendRequest)

	if err != nil {
		return err // nil
	}

	// TARGET[chat_id]: MESSAGE[message_id]
	bind(chatID, messageID)
	// sentBindings := map[string]string {
	// 	"chat_id":    channel.ChatID,
	// 	"message_id": strconv.Itoa(sentMessage.MessageID),
	// }
	// attach sent message external bindings
	if message.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		message.Variables = binding
	}
	// +OK
	return nil
}

// WebHook callback http.Handler
//
// // bot := BotProvider(agent *Gateway)
// ...
// recv := Update{/* decode from notice.Body */}
// err = c.Gateway.Read(notice.Context(), recv)
//
// if err != nil {
// 	http.Error(res, "Failed to deliver .Update notification", http.StatusBadGateway)
// 	return // 502 Bad Gateway
// }
//
// reply.WriteHeader(http.StatusOK)
//
func (c *Client) WebHook(rsp http.ResponseWriter, req *http.Request) {
	// panic("not implemented") // TODO: Implement

	switch req.Method {
	case http.MethodGet:
		// Request URL ?query=
		query := req.URL.Query()
		// Webhook Verification !
		// https://developers.facebook.com/docs/messenger-platform/getting-started/webhook-setup#steps (4) !
		if IsWebhookVerification(query) {
			c.WebhookVerification(rsp, req)
			return
		}

		// TODO: Check for ?code=|error= OAuth 2.0 flow stage
		if state, is := IsOAuthCallback(query); is {
			switch state {
			case "fb": // Messenger Pages
				c.SetupMessengerPages(rsp, req)
				return
			case "ig": // Instagram Pages
				c.SetupInstagramPages(rsp, req)
				return
			default:
				_ = writeCompleteOAuthHTML(rsp,
					fmt.Errorf("state: invalid or missing"),
				)
				return
			}
		}

		// Tab: Instagram section ...
		if _, is := query["instagram"]; is {

			switch op := query.Get("instagram"); op {
			case "setup":

				c.PromptSetup(
					rsp, req,
					messengerInstagramScope, "ig", // "instagram"
					oauth2.SetAuthURLParam(
						"display", "popup",
					),
				)
				return // (302) Found

			case "remove",
				"subscribe",
				"unsubscribe":

				var (
					err error
					res []*Page
					ids = Fields(query["id"]...)
				)

				if op == "remove" {
					// DELETE /{PAGE-ID}/subscribed_apps
					// delete(c.pages, id)
					// res, err = c.RemovePages(ids...)
				} else if op == "subscribe" {
					// POST /{PAGE-ID}/subscribed_apps
					res, err = c.SubscribeInstagramPages(ids...)
				} else if op == "unsubscribe" {
					// DELETE /{PAGE-ID}/subscribed_apps
					res, err = c.UnsubscribeInstagramPages(ids...)
				}

				if err != nil {
					http.Error(rsp, err.Error(), http.StatusBadRequest)
					return // Error
				}

				header := rsp.Header()
				header.Set("Pragma", "no-cache")
				header.Set("Cache-Control", "no-cache")
				header.Set("Content-Type", "application/json; charset=utf-8")
				rsp.WriteHeader(http.StatusOK)

				enc := json.NewEncoder(rsp)
				enc.SetIndent("", "  ")
				_ = enc.Encode(res)

				return // (200) OK

			// case "remove":
			case "search", "":

				c.GetInstagramPages(rsp, req)
				return // (200) OK
			}

			http.Error(rsp, "instagram: operation not supported", http.StatusBadRequest)
			return // (400) Bad Request
		}

		switch op := query.Get("pages"); op {
		case "setup":

			c.PromptSetup(
				rsp, req,
				messengerFacebookScope, "fb", // "facebook"
				oauth2.SetAuthURLParam(
					"display", "popup",
				),
			)
			return // (302) Found

		case "remove",
			"subscribe",
			"unsubscribe":

			var (
				err error
				res []*Page
				ids = Fields(query["id"]...)
			)

			if op == "remove" {
				// DELETE /{PAGE-ID}/subscribed_apps
				// delete(c.pages, id)
				res, err = c.RemovePages(ids...)
			} else if op == "subscribe" {
				// POST /{PAGE-ID}/subscribed_apps
				res, err = c.SubscribePages(ids...)
			} else if op == "unsubscribe" {
				// DELETE /{PAGE-ID}/subscribed_apps
				res, err = c.UnsubscribePages(ids...)
			}

			if err != nil {
				http.Error(rsp, err.Error(), http.StatusBadRequest)
				return // Error
			}

			header := rsp.Header()
			header.Set("Pragma", "no-cache")
			header.Set("Cache-Control", "no-cache")
			header.Set("Content-Type", "application/json; charset=utf-8")
			rsp.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(rsp)
			enc.SetIndent("", "  ")
			_ = enc.Encode(res)

			return // (200) OK

		case "search", "":

			c.MessengerPages(rsp, req)
			return // (200) OK

		default:

			http.Error(rsp, "pages: operation not supported", http.StatusBadRequest)
			return // (400) Bad Request
		}

	case http.MethodPost:

		// Deauthorize Request ?
		if rs := req.FormValue("signed_request"); rs != "" {
			err := c.Deauthorize(rs)
			if err != nil {
				http.Error(rsp, err.Error(), http.StatusOK)
			}
			break // 200 OK
		}

		// POST Webhook event !
		c.WebhookEvent(rsp, req)

	default:

		http.Error(rsp, "(405) Method Not Allowed", http.StatusMethodNotAllowed)
	}

	// return
}

// Subscribe Webhook Callback URI to Facebook well-known Object(s).Fields...
func (c *Client) SubscribeObjects(ctx context.Context, uri string) error {

	// https://developers.facebook.com/docs/graph-api/reference/app/subscriptions#publish

	token, err := c.creds.Token()
	if err != nil {
		// switch re := err.(type) {
		// case *oauth2.RetrieveError:
		// }
		return err
	}

	// Generate random Verify Token string !
	webhook := &c.webhook
	webhook.URL = uri
	webhook.Token = RandomBase64String(64)
	webhook.Verified = ""

	var (
		// Request Template
		// https://developers.facebook.com/docs/graph-api/reference/app/subscriptions#publishingfields
		form = url.Values{
			// // Indicates the object type that this subscription applies to.
			// // enum{user, page, permissions, payments}
			// "object": {"page"},
			// // One or more of the set of valid fields in this object to subscribe to.
			// "fields": {strings.Join([]string{
			// 	// "standby",
			// 	"messages",
			// 	"messaging_postbacks",
			// 	// "messaging_handovers",
			// 	// "user_action",
			// }, ",")},
			// Indicates if change notifications should include the new values.
			"include_values": {"true"},
			// The URL that will receive the POST request when an update is triggered, and a GET request when attempting this publish operation. See our guide to constructing a callback URL page.
			"callback_url": {webhook.URL},
			// An arbitrary string that can be used to confirm to your server that the request is valid.
			"verify_token": {webhook.Token},
		}
		// Object(s) Subscription(s)
		subs = []webhooks.Subscription{
			{
				Object: "page",
				// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events
				Fields: []string{
					// "standby",
					"messages",
					// "message_reads",
					// "message_reactions",
					// "messaging_referrals",
					"messaging_postbacks",
					// "messaging_handovers",
					// "user_action",
				},
			},
			{
				Object: "instagram",
				Fields: []string{
					// "standby",
					"messages",
					// "message_reactions",
					"messaging_postbacks",
					// "messaging_handovers",
					// "messaging_seen",
				},
				// },
				// {
				// 	Object: "permissions",
				// 	Fields: []string{
				// 		"connected",
				// 		"pages_show_list",
				// 		"pages_messaging",
				// 		"pages_messaging_subscriptions",
				// 		"pages_manage_metadata",
				// 	},
			}}

		res = struct {
			graph.Success              // Embedded (Anonymous)
			Error         *graph.Error `json:"error,omitempty"`
		}{
			// Alloc
		}
	)

	// [RE]Authorize Each Request
	form = c.requestForm(form, token.AccessToken)
	for _, sub := range subs {

		form.Set("object", sub.Object)
		form.Set("fields", strings.Join(sub.Fields, ","))

		// SWITCH ON Webhook subscription !
		req, err := http.NewRequestWithContext(
			ctx, http.MethodPost,
			"https://graph.facebook.com"+path.Join(
				"/", c.Version, c.Config.ClientID, "subscriptions",
			),
			strings.NewReader(form.Encode()),
		)

		if err != nil {
			return err
		}

		rsp, err := c.Client.Do(req)

		if err != nil {
			return err
		}

		// NULLify
		res.Ok = false
		// Decode Response
		err = json.NewDecoder(rsp.Body).Decode(&res)
		// Close Response
		rsp.Body.Close()

		if err != nil {
			return err
		}

		if res.Error != nil {
			return res.Error
		}

		if !res.Ok {
			return fmt.Errorf("subscribe: object=%s; success not confirmed", sub.Object)
		}
	}

	return nil
}

// Register webhook callback URI
func (c *Client) Register(ctx context.Context, uri string) error {
	return c.SubscribeObjects(ctx, uri)
}

// Deregister webhook callback URI
func (c *Client) Deregister(ctx context.Context) error {

	// https://developers.facebook.com/docs/graph-api/reference/app/subscriptions#delete

	token, err := c.creds.Token()
	if err != nil {
		// switch re := err.(type) {
		// case *oauth2.RetrieveError:
		// }
		return err
	}

	form := url.Values{
		// // A specific object type to remove subscriptions for.
		// If this optional field is not included, all subscriptions for this app will be removed.
		// // enum{ user, page, permissions, payments }
		// "object": {"page"},
		// // One or more of the set of valid fields in this object to unsubscribe from.
		// "fields": {strings.Join([]string{
		// 	"standby",
		// 	"messages",
		// 	"messaging_postbacks",
		// 	"messaging_handovers",
		// 	// "user_action",
		// }, ",")},
	}

	form = c.requestForm(form, token.AccessToken)
	// SWITCH ON Webhook subscription !
	// https://developers.facebook.com/docs/graph-api/reference/app/subscriptions#delete
	req, err := http.NewRequest(http.MethodDelete,
		"https://graph.facebook.com"+path.Join(
			"/", c.Version, c.Config.ClientID, "subscriptions",
		),
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)

	if err != nil {
		return err
	}

	defer rsp.Body.Close()

	var (
		res = struct {
			graph.Success              // Embedded (Anonymous)
			Error         *graph.Error `json:"error,omitempty"`
		}{
			// Alloc
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err != nil {
		return err
	}

	if res.Error != nil {
		return res.Error
	}

	if !res.Ok {
		return fmt.Errorf("unsubscribe: success not confirmed")
	}

	// NULLify settings
	webhook := &c.webhook
	webhook.URL = ""
	webhook.Token = ""
	webhook.Verified = ""

	return nil
}

// Close shuts down bot and all it's running session(s)
func (c *Client) Close() error {
	// panic("not implemented") // TODO: Implement
	return nil
}

func (c *Client) MessengerPages(rsp http.ResponseWriter, req *http.Request) {

	// TODO: Authorization Required

	query := req.URL.Query()
	pageId := Fields(query["id"]...)

	pages, err := c.pages.getPages(pageId...)

	if err != nil {
		http.Error(rsp, err.Error(), http.StatusNotFound)
		return
	}

	sort.SliceStable(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })

	header := rsp.Header()
	header.Set("Pragma", "no-cache")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "close")
	header.Set("Content-Type", "application/json; charset=utf-8") // res.Header.Get("Content-Type"))

	indent := "  "
	enc := json.NewEncoder(rsp)
	enc.SetIndent(indent, indent)

	// _ = enc.Encode(pages)

	// JSON StartArray
	_, _ = rsp.Write([]byte("[\n" + indent))

	// Result View
	var (
		n    int
		item = Page{
			Page: &graph.Page{
				// Envelope: Sanitized View
			},
		}
	)
	// Sanitize fields
	for i, page := range pages {
		// JSON ArrayItem
		if i != 0 {
			_, _ = rsp.Write([]byte(", ")) // (",\n"+indent))
		}

		n = len(page.Accounts)
		if n == 0 {
			continue // DO NOT Show !
		}

		item.Page.ID = page.ID
		item.Page.Name = page.Name
		// item.Page.Picture     = page.Picture
		// item.Page.AccessToken = page.GetAccessToken()

		item.Accounts = page.Accounts
		item.SubscribedFields = page.SubscribedFields

		_ = enc.Encode(item)
	}
	// JSON EndArray
	_, _ = rsp.Write([]byte("]"))
}

func (c *Client) SubscribePages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.pages.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.subscribePages(pages)

	if err != nil {
		return nil, err
	}

	return pages, nil
}

func (c *Client) UnsubscribePages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.pages.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.unsubscribePages(pages)

	if err != nil {
		return nil, err
	}

	return pages, nil
}

// Fields returns set of normalized variant ?fields=a,b&fields=c spec
// Example ?id=a,b&id=c&id=&id=c will result in [a,b,c]
func Fields(list ...string) []string {

	// Normalize ?id=a,b&id=c
	for i := 0; i < len(list); i++ {
		// if id[i] == "" {
		// 	id = append(id[0:i], id[i+1:]...)
		// 	i--; continue
		// }
		more := strings.FieldsFunc(list[i], func(c rune) bool {
			return !unicode.IsNumber(c) && !unicode.IsLetter(c)
		})
		switch m := len(more); m {
		case 0:
			list = append(list[0:i], list[i+1:]...)
			i--
			continue
		case 1:
			list[i] = more[0]
		default:
			// extend
			// in  ["a,b","c"]
			// out ["a","b","c"]
			n := len(list)
			// grow(more ex fields)
			list = append(list, more[1:]...)
			// move(rest to the end)
			copy(list[i+m:], list[i+1:n])
			// push(ex field on it's place)
			copy(list[i:i+m], more)
			// iter(move cursor)
			i += m - 1 // -1 next iter
		}
	}

	return Unique(list)
}

// Unique returns set of unique values from list
func Unique(list []string) []string {

	var e int // index duplicate
	for i := 1; i < len(list); i++ {
		for e = i - 1; e >= 0 && list[i] != list[e]; e-- {
			// lookup for duplicate; backwards
		}
		if e >= 0 {
			// duplicate: found; drop !
			list = append(list[:i], list[i+1:]...)
			(i)--
			continue
		}
	}

	return list
}
