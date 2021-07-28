package viber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/micro/go-micro/v2/errors"
	"github.com/rs/zerolog/log"

	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
)

type viberBot struct {
	Token      string
	Events     []string
	BotName    string
	Gateway    *bot.Gateway
	Client     *http.Client
}

// eventPayload received from Viber server on webhook
type eventPayload struct {
	Event        string  `json:"event"`
	Timestamp    int     `json:"timestamp"`
	ChatHostname string  `json:"chat_hostname"`
	MessageToken uint64  `json:"message_token"`
	Sender       sender  `json:"sender"`
	Message      message `json:"message"`
	Silent       bool    `json:"silent"`
	UserID       string  `json:"user_id,omitempty"`
}

type message struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Contact   struct {
		Name        string `json:"name"`
		PhoneNumber string `json:"phone_number"`
	}
	Location   struct {
		Latitude    float64 `json:"lat"`
		Longitude   float64 `json:"lon"`
	}
	Media     string `json:"media"`
	Thumbnail string `json:"thumbnail"`
	FileName  string `json:"file_name"`
}

type sender struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Language   string `json:"language,omitempty"`
	Country    string `json:"country,omitempty"`
	APIVersion int    `json:"api_version,omitempty"`
}

// viberReqBody struct used for sending text messages to messenger
type viberReqBody struct {
	Receiver      string    `json:"receiver",omitempty`
	Sender        sender    `json:"sender"`
	MinApiVersion int       `json:"min_api_version,omitempty"`
	Type          string    `json:"type"`
	Text          string  	`json:"text"`
	Media         string  	`json:"media,omitempty"`
	Size          int64    	`json:"size,omitempty"`
	FileName      string   	`json:"file_name,omitempty"`
	//BroadcastList []string 	`json:"broadcast_list,omitempty"`
	Keyboard      *keyboard `json:"keyboard,omitempty"`
}

type viberResponse struct {
	MessageToken int       `json:"message_token,omitempty"`
}

type keyboard struct {
	Type           string   `json:"Type,omitempty"`
	DefaultHeight  bool     `json:"DefaultHeight,omitempty"`
	Buttons        []button `json:"Buttons,omitempty"`
}

type button struct {
	ActionType string 	`json:"ActionType"`
	ActionBody string 	`json:"ActionBody"`
	Text       string 	`json:"Text"`
	Columns    int    	`json:"Columns"`
	BgColor    string 	`json:"BgColor,omitempty"`
}

// WebhookReq request
type WebhookReq struct {
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
}


func init() {
	// NewProvider(viber)
	bot.Register("viber", NewViberBot)
}

// NewViberBot initialize new agent.profile service provider
// func NewViberBot(agent *bot.Gateway) (bot.Provider, error) {
func NewViberBot(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {
	profile := agent.Bot.GetMetadata()
	appToken, ok := profile["token"]
	if !ok {
		log.Error().Msg("AppToken not found")
		return nil, errors.BadRequest(
			"chat.gateway.viber.token.required",
			"viber: bot API token required",
		)
	}

	name, ok := profile["botName"]
	if !ok {
		name = "bot"
		log.Error().Msg("botName not found")
	}

	eventTypes, _ := profile["eventTypes"]
	types := strings.Split(eventTypes, ",")

	return &viberBot {
		Events:     types,
		Token:      appToken,
		BotName:    name,
		Gateway:    agent,
	}, nil
}

func (_ *viberBot) Close() error {
	return nil
}

func (_ *viberBot) String() string {
	return "viber"
}

// Register Viber Bot Webhook endpoint URI
func (v *viberBot) Register(ctx context.Context, linkURL string) error {
	req := WebhookReq {
		URL:        linkURL,
		EventTypes: v.Events,
	}

	_, err := v.PostData("https://chatapi.viber.com/pa/set_webhook", req)

	if err != nil {
		v.Gateway.Log.Error().Err(err).Msg("Failed to .Register webhook")
		return err
	}
	return nil
}


// Deregister viber Bot Webhook endpoint URI
func (v *viberBot) Deregister(ctx context.Context) error {
	req := WebhookReq {
		URL: "",
	}

	_, err := v.PostData("https://chatapi.viber.com/pa/set_webhook", req)

	if err != nil {
		return err
	}
	return nil
}

func (v *viberBot) SendNotify(ctx context.Context, notify *bot.Update) error {
	
	var (
		// notify.Chat
		channel = notify.Chat
		//notify.Message
		message = notify.Message
	)


	msg := viberReqBody {
		Receiver: channel.ChatID,
		Sender: sender {
			Name: v.BotName,
		},
	}

	switch message.Type {
		
		case "text":

			msg.Type = "text"

			msg.Text = message.GetText()

			if message.Buttons != nil {
			
				if len(message.Buttons) > 0 {

					msg.MinApiVersion = 6
					v.viberMessageMenu(message.GetButtons(), &msg)
				}
			}

		case "file":
			viberMessageFile(message.GetFile(), &msg)

		case "closed":
			msg.Type = "text"
			msg.Text = message.GetText()

		default:
			return nil // UNKNOWN Event
	}

	_, err := v.PostData("https://chatapi.viber.com/pa/send_message", msg)
	if err != nil {
		return err
	}

	return nil
}

func viberMessageFile(f *chat.File, m *viberReqBody) {
	const (
		// 30 Mb = 1024 Kb * 1024 b
		imageSizeMax = 30 * 1024 * 1024
		// 26 Mb = 1024 Kb * 1024 b
		videoSizeMax = 26 * 1024 * 1024
	)

	switch {

		case strings.HasPrefix(f.Mime, "image") && f.Size < imageSizeMax: 
			m.Type = "picture"
			m.Media = f.Url

		case strings.HasPrefix(f.Mime, "video") && f.Size < videoSizeMax:
			m.Type = "video"
			m.Media = f.Url
			m.Size = f.Size	

		default:
			m.Type = "file"
			m.FileName = f.Name
			m.Media = f.Url
			m.Size = f.Size
	}
}

func (v *viberBot) viberMessageMenu(buttons []*chat.Buttons, m *viberReqBody) {

	var rows = make([]button, 0)

	layout := newLayout()
	
	for _, line := range buttons {

		rowlayout,ok :=layout[len(line.Button)]

		if !ok {
			v.Gateway.Log.Error().Int("Value", len(line.Button)).Str("Err", "line layout NOT Found. Possible values 1 - 6")
			continue
		}

		for i, b := range line.Button {

			if b.Type == "url" {
				rows = append(rows, newKeyboardButtonURL(b.Text, b.Url, rowlayout[i]))

			}else if b.Type == "contact" {
				rows = append(rows, newKeyboardButtonContact(b.Text, rowlayout[i]))

			}else if b.Type == "location" {
				rows = append(rows, newKeyboardButtonLocation(b.Text, rowlayout[i]))

			}else if b.Type == "reply" {
				rows = append(rows, newKeyboardButtonReply(b.Text, rowlayout[i]))

			}else if b.Text != "" {
				rows = append(rows, newKeyboardNoneButton(b.Text, rowlayout[i]))
			}
		}
	}

	m.Keyboard = &keyboard {
		DefaultHeight: 	true,
		Type: 		 	"keyboard",
		Buttons:		 rows,
	}
}

func (v *viberBot) PostData(url string, i interface{}) (*viberResponse, error) {
	
	body, err := json.Marshal(i)
	
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Add("X-Viber-Auth-Token", v.Token)
	req.Close = true

	viberRes, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer viberRes.Body.Close()
	
	var res viberResponse
	 
	if err := json.NewDecoder(viberRes.Body).Decode(&res); err != nil {

		v.Gateway.Log.Error().Err(err).Msg("Failed to decode viber response")

		return nil, err
	}

	return &res, nil
}

func newLayout()map[int][]int {
	return map[int][]int {
		1: {6},
		2: {3,3},
		3: {2,2,2},
		4: {2,2,1,1},
		5: {2,1,1,1,1},
		6: {1,1,1,1,1,1},
	}
}

func newKeyboardButtonURL(text string, url string, col int)button {
	return button {
		ActionType:  "open-url",
		BgColor:     "#c3e0e0",
		ActionBody:  url,	
		Text: 		 text,	
		Columns:     col,
	}
}

func newKeyboardButtonContact(text string, col int)button {
	return button {
		ActionType:  "share-phone",
		ActionBody:  "reply",
		BgColor:     "#c3e0e0",
		Text:        text,
		Columns:     col,
	}
}

func newKeyboardButtonLocation(text string, col int)button {
	return button {
		ActionType:  "location-picker",
		BgColor:     "#c3e0e0",
		Text: 		 text,		
		Columns:     col,
	}
}

func newKeyboardButtonReply(text string, col int)button {
	return button {
		ActionType:  "reply",
		BgColor:     "#c3e0e0",
		ActionBody:  text,	
		Text: 		 text,
		Columns:     col,	
	}
}

func newKeyboardNoneButton(text string, col int)button {
	return button {
		ActionType:  "none",
		BgColor:     "#c3e0e0",
		Text: 		 text,	
		Columns:     col,
		
	}
}

// WebHook implementes provider.Receiver interface for viber
func (v *viberBot) WebHook(reply http.ResponseWriter, notice *http.Request) {
	var req *eventPayload

	if err := json.NewDecoder(notice.Body).Decode(&req); err != nil {
		v.Gateway.Log.Error().Err(err).Msg("Failed to decode update request")
		http.Error(reply, "Failed to decode update request", http.StatusBadRequest) // 400
		return
	}
	
	switch req.Event {
		case "message":
			contact := &bot.Account {
				ID:        0, // LOOKUP
				Username:  req.Sender.Name,
				Channel:   "viber",
				Contact:   req.Sender.ID,
			}
			// endregion
			
			// region: channel
			chatID := req.Sender.ID
			channel, err := v.Gateway.GetChannel(
				notice.Context(), chatID, contact,
			)
			if err != nil {
				// Failed locate chat channel !
				re := errors.FromError(err); if re.Code == 0 {
					re.Code = (int32)(http.StatusBadGateway)
				}
				http.Error(reply, re.Detail, (int)(re.Code))
				return // 503 Bad Gateway
			}

			sendUpdate := bot.Update {
				Title:   channel.Title,
				Chat:    channel,
				User:    contact,
			}

			switch req.Message.Type {

				case "text":
					sendUpdate.Message = &chat.Message {
						Type: "text",
						Text: req.Message.Text,
					}
			
				case "url":
					sendUpdate.Message = &chat.Message {
						Type: "text",
						Text: req.Message.Media,
					}
			
				case "picture":
					sendUpdate.Message = &chat.Message {
						Type: "file",
						File: &chat.File {
							Url:    req.Message.Media,
							Name:   req.Message.FileName,
						},
					}
					
				case "video", "sticker", "file":
					sendUpdate.Message = &chat.Message {
						Type: "file",
						File: &chat.File {
							Url:    req.Message.Media,
							Name:   req.Message.FileName,
						},
					}

				case "contact":
					sendUpdate.Message = &chat.Message {
						Type: "contact",
						Contact: &chat.Account {
							Contact: req.Message.Contact.PhoneNumber,
						},
					}
			
				default:
					//http.Error(reply, "Unknown type message", http.StatusBadRequest) // 400 
					return // IGNORE
			}

			err = v.Gateway.Read(notice.Context(), &sendUpdate)
		
			if err != nil {
				http.Error(reply, "Failed to deliver viber .Update message", http.StatusInternalServerError)
				return // 502 Bad Gateway
			}
		
			reply.WriteHeader(http.StatusOK)
			return 
			
		case "unsubscribed":
			v.userUnsubscribed(req)

		case "delivered":
			// TODO...

		case "seen":
			// TODO...

		default:

	}
}

func (v viberBot) userUnsubscribed(msg *eventPayload) {

	contact := &bot.Account{
		ID:        0,
		Channel:   "viber",
		Contact:   msg.UserID,
	}

	channel, err := v.Gateway.GetChannel(
		context.TODO(), msg.UserID, contact,
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