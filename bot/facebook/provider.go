package facebook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/micro/micro/v3/service/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook/messenger"
	"github.com/webitel/chat_manager/bot/facebook/webhooks"
	"github.com/webitel/chat_manager/bot/facebook/whatsapp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	providerType = "messenger" // "facebook"

	PageInboxApplicationID = "263902037430900"

	// Messenger Bot's Conversation WITH Facebook Page ID
	paramFacebookPage = "facebook.page"
	// Messenger Bot's Conversation WITH Facebook Page Name
	paramFacebookName = "facebook.name"
	// Messenger Bot's Conversation WITH Instagram Page ID
	paramInstagramPage = "instagram.page"
	// Messenger Bot's Conversation WITH Instagram Username
	paramInstagramUser = "instagram.user"
	// Instagram::mention(s)
	paramIGMentionText = "instagram.mention"
	paramIGMentionLink = "instagram.mention.link"
	// Instagram::comment(s)
	paramIGCommentText = "instagram.comment"
	paramIGCommentLink = "instagram.comment.link"
	// Story::mention(s)
	paramStoryMentionCDN = "instagram.story.cdn"
	// paramStoryMentionText = "instagram.story.mention"
	// paramStoryMentionLink = "instagram.story.mention.link"
)

func init() {
	// Register Facebook Messenger Application provider
	bot.Register(providerType, New)
}

// Implementation
var (
	_ bot.Sender   = (*Client)(nil)
	_ bot.Receiver = (*Client)(nil)
	_ bot.Provider = (*Client)(nil)
)

func New(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {

	// agent: NEW config (to validate provider settings integrity)
	// state: RUN config (current to grab internal state if needed)
	// current, _ := state.(*Client)

	// Parse and validate message templates
	var err error
	agent.Template = bot.NewTemplate(providerType)
	// // Populate messenger-specific markdown-escape helper funcs
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
			"chat.bot.messenger.updates.invalid",
			err.Error(),
		)
	}

	metadata := agent.Bot.Metadata
	if len(metadata) == 0 {
		return nil, fmt.Errorf("messenger: bot setup metadata is missing")
	}

	apiVersion, _ := metadata["version"]
	if apiVersion != "" && !IsVersion(apiVersion) {
		return nil, errors.BadRequest(
			"chat.bot.messenger.version.invalid",
			"( version: %s ) invalid syntax; default: %s",
			apiVersion, Latest,
		)
	}
	if apiVersion == "" {
		apiVersion = Latest
	}
	app := &Client{

		Gateway: agent,

		Version: apiVersion,
		Config: oauth2.Config{
			ClientID:     metadata["client_id"],
			ClientSecret: metadata["client_secret"],
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://www.facebook.com" + path.Join("/", apiVersion, "/dialog/oauth"),
				TokenURL:  "https://graph.facebook.com" + path.Join("/", apiVersion, "/oauth/access_token"),
				AuthStyle: oauth2.AuthStyleInParams,
			},
			RedirectURL: agent.CallbackURL(),
			Scopes:      []string{"public_profile"},
		},
		peerCache: *expirable.NewLRU[string, *chat.Channel](1000, nil, time.Hour*1),

		webhook: webhooks.WebHook{
			URL:   agent.CallbackURL(), // "https://dev.webitel.com" + path.Join("/chat/ws8/messenger"),
			Token: metadata["verify_token"],
		},
	}

	if app.ClientID == "" {
		return nil, fmt.Errorf("messenger: missing client_id parameter")
	}

	if app.ClientSecret == "" {
		return nil, fmt.Errorf("messenger: missing client_secret parameter")
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
	// HTTP Client Transport
	app.Client = &client

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
	// Verify Meta App client credentials
	_, err = app.creds.Token()
	if err != nil {
		if ret, is := err.(*oauth2.RetrieveError); is {
			var res struct {
				*graph.Error `json:"error"`
			}
			_ = json.Unmarshal(ret.Body, &res)
			if res.Error != nil {
				// ret.Body = append(ret.Body[:0], []byte(res.Error.Message)...)
				// err = res.Error
				err = errors.BadGateway(
					"chat.bot.messenger.oauth.error",
					"MetaApp: "+res.Error.Message,
				)
			}
		}
		// Invalid credentials provided
		return nil, err
	}

	if current, _ := state.(*Client); current != nil {

		app.chatMx = current.chatMx
		app.chats = current.chats

		if app.Config.ClientSecret == current.Config.ClientSecret {
			app.proofMx = current.proofMx
			app.proofs = current.proofs
		}

		if app.proofs == nil {
			app.proofMx = new(sync.Mutex)
			app.proofs = make(map[string]string)
		}

		app.pages = current.pages
		// app.facebook  = current.facebook
		app.instagram = current.instagram
		// WHATSAPP Business Manager
		app.whatsApp = current.whatsApp

	} else { // INIT

		app.chatMx = new(sync.RWMutex)
		app.chats = make(map[string]Chat)

		app.proofMx = new(sync.Mutex)
		app.proofs = make(map[string]string)

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
				app.Log.Error("FACEBOOK: ACCOUNTS",
					slog.Any("error", err),
				)
			}
		}
		if s := metadata["ig"]; s != "" {
			encoding := base64.RawURLEncoding
			data, err := encoding.DecodeString(s)
			if err == nil {
				err = app.instagram.restore(data)
			}
			if err != nil {
				app.Log.Error("INSTAGRAM: ACCOUNTS",
					slog.Any("error", err),
				)
			}
		}
	}
	// WHATSAPP: [Continue] Setup ...
	whatsAppToken := metadata["whatsapp_token"]
	if whatsAppToken != "" {
		if app.whatsApp == nil || whatsAppToken != app.whatsApp.AccessToken {
			// Verify `whatsapp_token` requirements
			err = app.whatsAppVerifyToken(whatsAppToken)
			if err != nil {
				return nil, err
			}
		}
	}
	// Refresh due to whatsapp_token provided
	refresh := (whatsAppToken != "")
	if app.whatsApp == nil {
		// WHATSAPP: INIT
		app.whatsApp = whatsapp.NewManager(
			// Meta Business System User's generated token WITH whatsapp_business_management, whatsapp_business_messaging scope GRANTED !
			whatsAppToken,
			// https://developers.facebook.com/docs/graph-api/webhooks/getting-started/webhooks-for-whatsapp#available-subscription-fields
			"messages",
		)
	} else {
		// NOTE: Manager is populated from current state
		refresh = (refresh && whatsAppToken != app.whatsApp.AccessToken)
		// Just setup NEWly provided WhatsApp access token !
		app.whatsApp.AccessToken = whatsAppToken
	}
	// Refresh needed ?
	if refresh {
		// Eliminate cached Accounts
		_ = app.whatsApp.Deregister(
			app.whatsApp.GetAccounts(),
		)
		// ERR: Log. Ignore invalid dataset ...
		_ = app.whatsAppRestoreAccounts()
	}

	var (
		on bool
		// err error
		set string
	)
	for object, fields := range map[string][]string{
		// "facebook": {
		// 	// "feed",
		// 	"comments",
		// 	"mentions",
		// },
		"instagram": {
			"story_mentions",
			"comments",
			"mentions",
		},
	} {
		for _, field := range fields {
			param := object + "_" + field
			if set, on = metadata[param]; on {
				if on, _ = strconv.ParseBool(set); on {
					// TRUE Specified !
					switch object {
					// case "facebook":
					case "instagram":
						switch field {
						// required: instagram_manage_messages
						case "story_mentions":
							app.hookIGStoryMention = app.onIGStoryMention
						// required: instagram_manage_comments
						case "comments":
							app.hookIGMediaComment = app.onIGMediaComment
						// required: instagram_manage_comments
						case "mentions":
							app.hookIGMediaMention = app.onIGMediaMention
						}
					}
				} // else if err == nil { // && !set {
				// 	// FALSE Specified !
				// } // else {
				// BOOL Invalid !
				// }
			}
		}
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
		// flags = make([]string, 0, 3)
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
		// flags = flags[0:0]
		// if unicode.IsPrint(r) {
		// 	flags = append(flags, "print")
		// }
		// if unicode.IsDigit(r) {
		// 	flags = append(flags, "digit")
		// }
		// if unicode.IsLetter(r) {
		// 	flags = append(flags, "letter")
		// }
		// if unicode.IsNumber(r) {
		// 	flags = append(flags, "number")
		// }
		// if unicode.IsSymbol(r) {
		// 	flags = append(flags, "symbol")
		// }
		// if unicode.IsPunct(r) {
		// 	flags = append(flags, "punct")
		// }
		// if unicode.IsSpace(r) {
		// 	flags = append(flags, "space")
		// }
		// fmt.Printf("[%c]: %s\n", r, strings.Join(flags, "|"))

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

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}

// channel := notify.Chat
// contact := notify.User
func (c *Client) SendNotify(ctx context.Context, notify *bot.Update) error {

	var (
		channel = notify.Chat
		message = notify.Message
		binding map[string]string //TODO
		bind    = func(key, value string) {
			if binding == nil {
				binding = make(map[string]string)
			}
			binding[key] = value
		}
	)

	// Resolve VIA internal Account
	switch env := channel.Properties.(type) {
	case *Chat: // [Instagram] Facebook
		break
	case *whatsapp.WhatsAppPhoneNumber: // WhatsApp
		return c.whatsAppSendUpdate(ctx, notify)
	case map[string]string: // Recover
		if _, ok := env[paramWhatsAppNumberID]; ok {
			return c.whatsAppSendUpdate(ctx, notify)
		}
		// ASID, fb := env[paramFacebookPage]
	}

	// Resolve Facebook conversation (User+Page)
	// by channel.Account.Contact [P]age-[s]coped User [ID]
	chatID := channel.ChatID // channel.Account.Contact
	recipientUserPSID := chatID
	conversation, err := c.getExternalThread(channel)
	if err != nil {
		// re := errors.FromError(err)
		// switch re.Id {
		// case "bot.messenger.chat.page.missing":
		// 	// Facebook Page authentication is missing
		// 	// Guess: this MAY be the "whatsapp" channel
		// 	return c.whatsAppSendUpdate(ctx, notify)
		// } // default:
		defer channel.Close()
		return err
	}

	if conversation == nil || conversation.Page == nil {
		err := errors.NotFound(
			"bot.messenger.send.chat.not_found",
			"messenger: send TO.user=%s FROM.page=? not found",
			recipientUserPSID,
		)
		// return err
		c.Log.Error("messenger.sendMessage",
			slog.Any("error", err),
		)
		return nil
	}

	dialog := conversation
	platform := "facebook"
	facebook := dialog.Page // MUST: sender
	pageName := facebook.Name
	instagram := facebook.Instagram
	if instagram != nil {
		platform = "instagram"
		pageName = instagram.Name
		if pageName == "" {
			pageName = instagram.Username
		}
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
				buttons []*messenger.Button
				replies []*messenger.QuickReply
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
						if instagram != nil {
							continue
						} // NOT Supported !
						replies = append(replies, &messenger.QuickReply{
							Type: "user_email",
						})
					case "phone", "contact": // https://developers.facebook.com/docs/messenger-platform/send-messages/quick-replies#phone
						if instagram != nil {
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
				if instagram == nil {
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

	case "joined": // ACK: ChatService.JoinConversation()

		peer := contactPeer(message.NewChatMembers[0])
		updates := c.Gateway.Template
		text, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Error(platform+".updateChatMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// format new message to the engine for saving it in the DB as operator message [WTEL-4695]
		messageToSave := &chat.Message{
			Type:      "text",
			Text:      text,
			CreatedAt: time.Now().UnixMilli(),
			From:      peer,
		}
		if channel != nil && channel.ChannelID != "" {
			_, err = c.Gateway.Internal.Client.SaveAgentJoinMessage(ctx, &chat.SaveAgentJoinMessageRequest{Message: messageToSave, Receiver: channel.ChannelID})
			if err != nil {
				return err
			}
		}
		// Send Text
		sendMessage.Text = text

	case "left": // ACK: ChatService.LeaveConversation()

		peer := contactPeer(message.LeftChatMember)
		updates := c.Gateway.Template
		text, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Error(platform+".updateLeftMember",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// Send Text
		sendMessage.Text = text

	// case "typing":
	// case "upload":

	// case "invite":
	case "closed":

		updates := c.Gateway.Template
		text, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Error(platform+".updateChatClose",
				slog.Any("error", err),
				slog.String("update", message.Type),
			)
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// Send Text
		sendMessage.Text = text

	default:
		c.Log.Warn(platform+".sendMessage",
			slog.String("error", "send: content type="+message.Type+" not supported"),
		)
		return nil
	}

	messageID, err := c.Send(conversation.Page, &sendRequest)

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
//	if err != nil {
//		http.Error(res, "Failed to deliver .Update notification", http.StatusBadGateway)
//		return // 502 Bad Gateway
//	}
//
// reply.WriteHeader(http.StatusOK)
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
			case "fb": // Facebook Pages
				c.SetupMessengerPages(rsp, req)
				return
			case "ig": // Instagram Pages
				c.SetupInstagramPages(rsp, req)
				return
			case "wa": // WhatsApp Business Account(s)
				c.SetupWhatsAppBusinessAccounts(rsp, req)
				return
			default:
				_ = writeCompleteOAuthHTML(rsp,
					fmt.Errorf("state: invalid or missing"),
				)
				return
			}
		}

		// region: --- AdminAuthorization(!) ---
		// [D]e[M]illitary[Z]one(s) ...
		dmz := (query.Get("pages") == "setup" ||
			query.Get("instagram") == "setup" ||
			query.Get("whatsapp") == "setup")
		// Authorize (!)
		if !dmz && c.Gateway.AdminAuthorization(rsp, req) != nil {
			return // Authorization FAILED(!)
		}
		// endregion: --- AdminAuthorization(!) ---

		// Tab: Instagram section ...
		if _, is := query["instagram"]; is {

			switch op := query.Get("instagram"); op {
			case "setup":

				c.PromptSetup(
					rsp, req,
					c.instagramOAuth2Scope(), "ig", // "instagram"
					oauth2.SetAuthURLParam(
						"display", "popup",
					),
					// https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow#reaskperms
					oauth2.SetAuthURLParam(
						"auth_type", "rerequest",
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

		// Tab: WhatsApp section ...
		if _, is := query["whatsapp"]; is {

			switch op := query.Get("whatsapp"); op {
			case "setup":

				if c.whatsApp.AccessToken == "" {
					http.Error(rsp, "WhatsApp: integration access token is required but missing", http.StatusBadRequest)
					return // (400) Bad Request
				}

				c.PromptSetup(
					rsp, req,
					whatsAppOAuthScopes, "wa", // "whatsapp"
					// oauth2.SetAuthURLParam(
					// 	"response_type", "code,signed_request",
					// ),
					oauth2.SetAuthURLParam(
						"display", "popup",
					),
					// https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow#reaskperms
					oauth2.SetAuthURLParam(
						"auth_type", "rerequest",
					),
					// oauth2.SetAuthURLParam(
					// 	// https://developers.facebook.com/docs/whatsapp/embedded-signup/pre-filled-data
					// 	"extras", `{"feature":"whatsapp_embedded_signup","setup":{}}`,
					// 	// "extras", `{"feature":"whatsapp_embedded_signup"}`, // FIXME: NOT Working =((
					// ),
					// oauth2.AccessTypeOffline, // NO REACTION ! NO refresh_token returning
				)
				return // (302) Found

			case "remove":
				// DELETE /{WABAID}/subscribed_apps
				// delete(c.whatsApp.Businesses, id)
				c.handleWhatsAppRemoveAccounts(rsp, req)
				return // (200) OK ?

			case "subscribe":
				// POST /{WABAID}/subscribed_apps
				c.handleWhatsAppSubscribeAccounts(rsp, req)
				return // (200) OK ?

			case "unsubscribe":
				// DELETE /{WABAID}/subscribed_apps
				c.handleWhatsAppUnsubscribeAccounts(rsp, req)
				return // (200) OK ?

			case "search", "":

				c.handleWhatsAppSearchAccounts(rsp, req)
				return // (200) OK ?

			}

			http.Error(rsp, "whatsapp: operation not supported", http.StatusBadRequest)
			return // (400) Bad Request
		}

		// Tag: Facebook section ...
		switch op := query.Get("pages"); op {
		case "setup":

			c.PromptSetup(
				rsp, req,
				messengerFacebookScope, "fb", // "facebook"
				oauth2.SetAuthURLParam(
					"display", "popup",
				),
				// https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow#reaskperms
				oauth2.SetAuthURLParam(
					"auth_type", "rerequest",
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
					"mentions",
					"comments",
					// "standby",
					"messages",
					// "message_reactions",
					"messaging_postbacks",
					// "messaging_handovers",
					// "messaging_seen",
				},
			},
			{
				Object: "whatsapp_business_account",
				Fields: c.whatsApp.SubscribedFields,
				// Fields: []string{
				// 	"messages",
				// },
			},
			// {
			// 	Object: "permissions",
			// 	Fields: []string{
			// 		"connected",
			// 		"pages_show_list",
			// 		"pages_messaging",
			// 		"pages_messaging_subscriptions",
			// 		"pages_manage_metadata",
			// 	},
			// },
		}

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

func (c *Client) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {
	peers := req.GetPeer()
	text := req.GetMessage().GetText()
	var channel *chat.Channel
	for _, peer := range peers {
		// check CACHE !
		if v, ok := c.peerCache.Get(peer); !ok {
			resp, err := c.Gateway.Internal.Client.GetChannelByPeer(ctx, &chat.GetChannelByPeerRequest{PeerId: peer, FromId: req.GetFrom()})
			if err != nil {
				return err
			}

			c.peerCache.Add(peer, resp)
			channel = resp
		} else {
			channel = v
		}
		props := make(map[string]string)
		err := json.Unmarshal([]byte(channel.Props), &props)
		if err != nil {
			return errors.InternalServerError("facebook.broadcast.unmarshal_props.error", err.Error())
		}
		// PSID | IGSID | WAID required
		switch channel.Type {
		case "instagram":
			if p := c.instagram.getPage(props[paramInstagramPage]); p != nil {
				_, err := c.SendInstagramText(p, peer, text)
				if err != nil {
					return err // nil
				}
			} else {
				return errors.BadRequest("facebook.provider.broadcast_message.instagram.page_not_found.error", fmt.Sprintf("instagram page not found (IGSID - %s)", p.IGSID()))
			}
		case "facebook":
			v, ok := props[paramFacebookPage]
			if !ok {
				return errors.BadRequest("facebook.provider.broadcast_message.facebook.missing_args.error", "facebook page id not specified !")
			}
			_, err := c.SendText(v, peer, text)
			if err != nil {
				return err
			}
		case "whatsapp":
			v, ok := props[paramWhatsAppNumberID]
			if !ok {
				return errors.BadRequest("facebook.provider.broadcast_message.whatsapp.missing_args.error", "whatsapp page number id not specified !")
			}
			sendMsg := &whatsapp.SendMessage{
				MessagingProduct: "whatsapp",
				RecipientType:    "individual",
				Status:           "",
				TO:               peer,
				Text: &whatsapp.Text{
					Body: text,
				},
				Type: "text",
			}
			account := c.whatsApp.GetPhoneNumber(v)
			if account == nil {
				return errors.BadRequest("facebook.provider.broadcast_message.missing_args.error", "whatsapp page number not specified !")
			}
			_, err := c.whatsAppSendMessage(ctx, account, sendMsg)
			if err != nil {
				return err
			}
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

var (
	//
	facebookPageFields = []string{
		// "standby",
		"messages",
		// "message_reads",
		// "message_reactions",
		// "messaging_referrals",
		"messaging_postbacks",
		// "messaging_handovers",
		// "user_action",
	}

	instagramPageFields = []string{
		// FAKE subscription field to be able
		// to receive Instagram Business Account Inbox Messages
		// update event notifications
		"name",
	}
)

func intersectFields(fields, known []string) (inner []string) {
	if len(fields) == 0 || len(known) == 0 {
		return // nil
	}
	var (
		e, n = 0, len(fields)
	)
intersect:
	for _, field := range known {
		for e = 0; e < n && fields[e] != field; e++ {
			// Lookup for known `field` in given `fields` set
		}
		if e < n {
			if inner == nil {
				inner = make([]string, 0, len(known)) // max
			}
			inner = append(inner, field)
			continue intersect
		}
	}
	return // inner
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
		// item.SubscribedFields = page.SubscribedFields
		item.SubscribedFields = intersectFields(
			page.SubscribedFields, facebookPageFields,
		)

		_ = enc.Encode(item)
	}
	// JSON EndArray
	_, _ = rsp.Write([]byte("]"))
}

// Subscribe Facebook Page(s)
func (c *Client) SubscribePages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.pages.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.subscribePages(pages, facebookPageFields)

	if err != nil {
		return nil, err
	}

	return pages, nil
}

// Unsubscribe Facebook Page(s)
func (c *Client) UnsubscribePages(pageIds ...string) ([]*Page, error) {

	// Find ALL requested page(s)...
	pages, err := c.pages.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// Do subscribe for page(s) webhook updates
	err = c.unsubscribePages(pages) //, facebookPageFields)

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
