package main

import (
	
	"path/filepath"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"context"
	"strconv"
	"strings"
	"net/url"
	"bytes"
	"sync"
	"fmt"
	
	"github.com/rs/zerolog/log"
	"github.com/micro/go-micro/v2/errors"

	chat "github.com/webitel/chat_manager/api/proto/chat"
)

type facebookBot struct {
	accessToken string
	verifyToken string
	url         string
	Gateway     *Gateway
	clients     map[int64]*userProfile
	sync.RWMutex
}

// facebookReqBody struct used for sending text messages to messenger
type facebookReqBody struct {
	Message       messageContent  `json:"message"`
	Recipient     recipient       `json:"recipient"`
	MessagingType string          `json:"messaging_type,omitempty"`
}

type recipient struct {
	ID int64 `json:"id,string"`
}

type userProfile struct {
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	ID           string    `json:"id"`
}

type messageContent struct {
	Text         string         `json:"text,omitempty"`
	QuickReplies []quickReplies `json:"quick_replies,omitempty"`
	Attachment   *attachment    `json:"attachment,omitempty"`
}

type quickReplies struct {
	ContentType string `json:"content_type,omitempty"`
	Title       string `json:"title,omitempty"`
	Payload     string `json:"payload,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

type attachment struct {
	Type    string  `json:"type,omitempty"`
	Payload payload `json:"payload,omitempty"`
	Title   string  `json:"title,omitempty"`
	URL     string  `json:"URL,omitempty"`
}

type payload struct {
	TemplateType string   `json:"template_type,omitempty"`
	Text         string   `json:"text,omitempty"`
	Buttons      []buttons `json:"buttons,omitempty"`
	URL          string   `json:"url,omitempty"`
	IsReusable   *bool    `json:"is_reusable,omitempty"`
}

type buttons struct {
	Type    string `json:"type"`
	URL     string `json:"url,omitempty"`
	Title   string `json:"title"`
	Payload string `json:"payload,omitempty"`
}

// FacebookRequest received from Facebook server on webhook, contains messages, delivery reports and/or postbacks
type FacebookRequest struct {
	Entry []struct {
		ID        string      `json:"id"`
		Messaging []messaging `json:"messaging"`
		Time      int         `json:"time"`
	} `json:"entry"`
	Object string `json:"object"`
}

type messaging struct {
	Recipient struct {
		ID int64 `json:"id,string"`
	} `json:"recipient"`
	Sender struct {
		ID int64 `json:"id,string"`
	} `json:"sender"`
	Timestamp int               `json:"timestamp"`
	Message   *FacebookMessage  `json:"message,omitempty"`
	Delivery  *FacebookDelivery `json:"delivery"`
	Postback  *FacebookPostback `json:"postback"`
}

// received error response from Facebook
type errorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int64  `json:"code"`
		ErrorSubcode int64  `json:"error_subcode"`
		FbtraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

// FacebookMessage struct for text messaged received from facebook server as part of FacebookRequest struct
type FacebookMessage struct {
	Mid         string       `json:"mid"`
	Text        string       `json:"text"`
	Attachments []attachment `json:"attachments"`
}

// FacebookDelivery struct for delivery reports received from Facebook server as part of FacebookRequest struct
type FacebookDelivery struct {
	Mids      []string `json:"mids"`
	Watermark int      `json:"watermark"`
}

// FacebookPostback struct for postbacks received from Facebook server  as part of FacebookRequest struct
type FacebookPostback struct {
	Payload string `json:"payload"`
}

func init() {
	// NewProvider(facebook)
	Register("facebook", NewFacebookBot)
}

// NewFacebookBot initialize new agent.profile service provider
func NewFacebookBot(agent *Gateway) (Provider, error) {

	accessToken, ok := agent.Profile.Variables["AccessToken"]
	if !ok {
		log.Error().Msg("AccessToken not found")
		return nil, errors.BadRequest(
			"chat.gateway.facebook.AccessToken.required",
			"facebook: bot API AccessToken required",
		)
	}

	verifyToken, ok := agent.Profile.Variables["VerifyToken"]
	if !ok {
		log.Error().Msg("VerifyToken not found")
		return nil, errors.BadRequest(
			"chat.gateway.facebook.VerifyToken.required",
			"facebook: bot API VerifyToken required",
		)
	}

	url, ok := agent.Profile.Variables["url"]
	if !ok {
		log.Error().Msg("url not found")
		return nil, errors.BadRequest(
			"chat.gateway.facebook.url.required",
			"facebook: bot API url required",
		)
	}

	url += accessToken

	return &facebookBot {
		accessToken: accessToken,
		verifyToken: verifyToken,
		url:         url,
		Gateway: 	 agent,
		clients:     make(map[int64]*userProfile),
	}, nil
}

func (_ *facebookBot) String() string {
	return "facebook"
}

func (b *facebookBot) Register(ctx context.Context, linkURL string) error {
	return nil
}

func (b *facebookBot) Deregister(ctx context.Context) error {
	return nil
}

func (b *facebookBot) SendNotify(ctx context.Context, notify *Update) error {
	var (
		channel = notify.Chat

		message = notify.Message

		//binding map[string]string  //TODO
	)

	chatID, err := strconv.ParseInt(channel.ChatID, 10, 64)
	if err != nil {
		return err
	}

	reqBody := facebookReqBody {
		MessagingType: "RESPONSE",
		Recipient: recipient {
			ID: chatID,
		},
	}

	switch message.Type {
		
		case "text":

			if message.Buttons != nil {
				
				reqBody.Message.Text = message.Text
				newReplyKeyboardFb(message.GetButtons(), &reqBody.Message);

			} else if message.Inline != nil {

				newInlineboardFb(message.GetInline(), &reqBody.Message, message.Text);

			} else {

				reqBody.Message.Text = message.Text
			}

		case "file":
			newFileMessageFb(message.GetFile(), &reqBody.Message)

		case "closed":
			reqBody.Message.Text = message.GetText()

			
		default:

	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	facebookReq, err := http.NewRequest(http.MethodPost, b.url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	facebookReq.Header.Set("Content-Type", "application/json")

	facebookRes, err := http.DefaultClient.Do(facebookReq)
	if err != nil {
		return err
	}

	bodyBytes, err := ioutil.ReadAll(facebookRes.Body)

	bodyString := string(bodyBytes)
	log.Debug().
		Int("StatusCode", facebookRes.StatusCode).
		Str("bodyString", bodyString).
		Msg("SendMessage facebook Response")

	if facebookRes.StatusCode == 400 {

		var fbErr errorResponse

		if err := json.Unmarshal(bodyBytes, &fbErr); err != nil {
			log.Error().Err(err).Msg("Failed to decode response")
			return err
		}

		if fbErr.Error.Code == 551 && fbErr.Error.ErrorSubcode == 1545041 { // client turned off messages
			b.userUnsubscribed(notify.Chat.ChatID)
		}
	}
	return nil
}

func (b *facebookBot) userUnsubscribed(userID string) {

	contact := &Account {
		ID:        0,
		Channel:   "facebook",
		Contact:   userID,
	}

	channel, err := b.Gateway.GetChannel(
		context.TODO(), userID, contact,
	)
	if err != nil {
		return //200 IGNORE
	}

	// TODO: break flow execution !
	if channel.IsNew() {

		channel.Log.Warn().Msg("CLOSE Request NO Channel; IGNORE")
		return // TODO: NOTHING !
	}

	channel.Log.Info().Msg("CLOSE External request; PERFORM")

	// DO: .CloseConversation(!)
	// cause := commandCloseRecvDisposiotion
	_ = channel.Close() // (cause) // default: /close request
	
	return
	
}

func (b *facebookBot) WebHook(reply http.ResponseWriter, notice *http.Request) {
	b.verifyWebhook(reply, notice)

	var fbRequest FacebookRequest

	if err := json.NewDecoder(notice.Body).Decode(&fbRequest); err != nil {
		log.Error().Err(err).Msg("Failed to decode update request")
		http.Error(reply, "Failed to decode update request", http.StatusBadRequest) // 400
		return
	}

	for _, entry := range fbRequest.Entry {

		for _, msg := range entry.Messaging {

			switch {
				case msg.Message != nil:

					b.RLock()   // +R
					client, ok := b.clients[msg.Sender.ID]
					b.RUnlock() // -R
					
					if !ok {
						client, _ = b.getProfileInfo(fmt.Sprint(msg.Sender.ID))
						
						b.Lock()   // +RW
						b.clients[msg.Sender.ID] = client
						b.Unlock() // -RW
					}

					contact := &Account {
						FirstName:  client.FirstName,
						LastName:   client.LastName,
						ID:         0, // LOOKUP
						Channel:    "facebook",
						Contact:    fmt.Sprint(msg.Sender.ID),
					}
					// endregion
					
					// region: channel
					channel, err := b.Gateway.GetChannel(
						notice.Context(), fmt.Sprint(msg.Sender.ID), contact,
					)
					if err != nil {
						// Failed locate chat channel !
						re := errors.FromError(err); if re.Code == 0 {
							re.Code = (int32)(http.StatusBadGateway)
						}
						http.Error(reply, re.Detail, (int)(re.Code))
						return // 503 Bad Gateway
					}
		
					sendUpdate := Update {
						Title:   channel.Title,
						Chat:    channel,
						User:    contact,
						Message: &chat.Message{},
					}

					if msg.Message.Text != "" {
						sendUpdate.Message.Type = "text"
						sendUpdate.Message.Text = msg.Message.Text
						
						err = b.Gateway.Read(notice.Context(), &sendUpdate)
						if err != nil {
							http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
							return // 502 Bad Gateway
						}
					}
					if msg.Message.Attachments != nil {
						for _, item := range msg.Message.Attachments {
			
							url, err := url.Parse(item.Payload.URL)
							if err != nil {
								log.Error().Msg(err.Error())
								continue
							}

							path := filepath.Base(url.Path)

							sendUpdate.Message.Text = ""
							sendUpdate.Message.Type = "file"
							sendUpdate.Message.File = &chat.File {
								Name:     path,
								Url:      item.Payload.URL,
							}

							err = b.Gateway.Read(notice.Context(), &sendUpdate)
							if err != nil {
								http.Error(reply, "Failed to deliver facebook .Update message", http.StatusInternalServerError)
								return // 502 Bad Gateway
							}
						}
					}
				 case msg.Delivery != nil:

				 case msg.Postback != nil:
				
			}
		}
	}
}

func newInlineboardFb(data []*chat.Buttons, msg *messageContent, text string) {
	var rows = make([]buttons, 0)

	for _, v := range data {

		for _, button := range v.Button {

			if button.Type == "url" {
				rows = append(rows, newfbKeyboardButtonURL(button.Text, button.Url))

			}else if button.Type =="call" {
				rows = append(rows, newfbKeyboardButtonCall(button.Text, button.Code))
	
			}else if button.Type =="postback" {
				rows = append(rows, newfbKeyboardButtonData(button.Text, button.Code))
			}
		}
	}

	if len(rows) > 0 {
		msg.Attachment = &attachment {
			Type: "template",
			Payload: payload {
				TemplateType: "button",
				Buttons:      rows,
			},
		}
	}else {
		msg.Text = text
	}

	
}

func newReplyKeyboardFb(b []*chat.Buttons, msg *messageContent) {
	var quick = make([]quickReplies, 0)

	for _, v := range b {

		for _, b := range v.Button {

			if b.Type == "reply" {
				quick = append(quick, newfbKeyboardButtonReply(b.Text))
			}
		}
	}

	msg.QuickReplies= quick
}

func (b *facebookBot) getProfileInfo(psid string) (*userProfile, error) {
	var profile userProfile
	
	url := fmt.Sprintf("https://graph.facebook.com/%s?fields=first_name,last_name,profile_pic&access_token=%v", psid, b.accessToken)
	res, err := http.Get(url)

	if err!=nil {
		log.Error().Err(err).Msg("Get profile info")
		return &profile, err
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&profile); err != nil {
		log.Error().Err(err).Msg("Failed to decode profile response")
	}

	return &profile, err
}

func newfbKeyboardButtonURL(text string, url string)buttons {
	return buttons {
		Type: 	"web_url",
		URL:   	url,
		Title:	text,
	}
}

func newfbKeyboardButtonData(text string, code string)buttons {
	return buttons {
		Type: 		"postback",
		Title: 		text,
		Payload: 	code,
	}
}

func newfbKeyboardButtonCall(text string, code string)buttons {
	return buttons {
		Type: 		"phone_number",
		Title: 		text,
		Payload: 	code, //number
	}
}

func newfbKeyboardButtonReply(text string)quickReplies {
	return quickReplies {
		ContentType:  "text",
		Title: 		  text,
		Payload:      text,
	}
}

func newFileMessageFb(f *chat.File, msg *messageContent) {
	var attachmentType string
	
	switch {
		case strings.HasPrefix(f.Mime, "image"):
			attachmentType = "image"

		case strings.HasPrefix(f.Mime, "video"):
			attachmentType = "video"

		case strings.HasPrefix(f.Mime, "audio"):
			attachmentType = "audio"

		default:
			attachmentType = "file"
	}

	msg.Attachment =  &attachment {
		Type: attachmentType,
		Payload: payload {
			URL: f.Url,
		},
	}
	
}

func (b *facebookBot) verifyWebhook(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("hub.mode") == "subscribe" {
		if r.FormValue("hub.verify_token") == b.verifyToken {
			w.Write([]byte(r.FormValue("hub.challenge")))
			return
		}
	}
}
