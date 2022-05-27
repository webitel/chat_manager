package infobip_whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/micro/micro/v3/service/errors"
	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"

	"github.com/rs/zerolog/log"
)

const (
	scenarioRoute = "omni/1/scenarios"
	messageRoute  = "omni/1/advanced"
)

type infobipWABot struct {
	profileID   int64
	apiKey      string
	scenarioKey string
	number      string
	url         string
	*bot.Gateway
}

type CreateScenarioRequest struct {
	Name    string  `json:"name"`
	Flow    []*Flow `json:"flow"`
	Default bool    `json:"default"`
}

type Flow struct {
	From    string `json:"from"`
	Channel string `json:"channel"`
}

type CreateScenarioResponse struct {
	Key string `json:"key"`
}

type SendMessageWARequest struct {
	ScenarioKey  string           `json:"scenarioKey"`
	Destinations []*Destination   `json:"destinations"`
	WhatsApp     *WhatsAppMessage `json:"whatsApp"`
}

type WhatsAppMessage struct {
	Text     string `json:"text,omitempty"`
	FileURL  string `json:"fileUrl,omitempty"`
	VideoURL string `json:"videoUrl,omitempty"`
	AudioURL string `json:"audioUrl,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
}

type Destination struct {
	MessageId string             `json:"messageId,omitempty"`
	To        *NumberDestination `json:"to"`
}

type NumberDestination struct {
	PhoneNumber string `json:"phoneNumber"`
}

type InfobipWABody struct {
	Results             []*Result `json:"results"`
	MessageCount        int64     `json:"messageCount"`
	PendingMessageCount int64     `json:"pendingMessageCount"`
}

type Result struct {
	From            string `json:"from"`
	To              string `json:"to"`
	IntegrationType string `json:"integrationType"`
	ReceivedAt      string `json:"receivedAt"`
	MessageID       string `json:"messageId"`
	Message         `json:"message"`
	Contact         `json:"contact,omitempty"`
	Price           `json:"price"`
}

type Message struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	URL     string `json:"url"`
	Caption string `json:"caption"`
}

type Contact struct {
	Name string `json:"name"`
}

type Price struct {
	PricePerMessage float64 `json:"pricePerMessage"`
	Currency        string  `json:"currency"`
}

func init() {
	bot.Register("infobip_whatsapp", NewInfobipWABot)
}

// func NewInfobipWABot(agent *bot.Gateway) (bot.Provider, error) {
func NewInfobipWABot(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {
	profile := agent.Bot.GetMetadata()
	apiKey, ok := profile["api_key"]
	if !ok {
		return nil, errors.BadRequest(
			"chat.gateway.infobipWA.api_key.required",
			"infobipWA: bot API api_key required",
		)
	}

	number, ok := profile["number"]
	if !ok {
		return nil, errors.BadRequest(
			"chat.gateway.infobipWA.number.required",
			"infobipWA: bot API number required",
		)
	}

	url, ok := profile["url"]
	if !ok {
		return nil, errors.BadRequest(
			"chat.gateway.infobipWA.url.required",
			"infobipWA: bot API url required",
		)
	}

	scenarioKey, _ := profile["scenario_key"]
	if !ok {
		log.Debug().Msg("creating scenario")
		var err error
		scenarioKey, err = createWAScenario(apiKey, number, url)
		if err != nil {
			log.Error().Msg(err.Error())
			return nil, err
		}

		profile["scenario_key"] = scenarioKey
		// if _, err = agent.Internal.Client.UpdateProfile(context.TODO(), &chat.UpdateProfileRequest {
		// 	// Id:   profile.Id,
		// 	Item: agent.Profile,
		// }); err != nil {
		// 	log.Error().Msg(err.Error())
		// 	return nil, err
		// }
		err = agent.Internal.UpdateBot(
			context.TODO(),
			&gate.UpdateBotRequest{
				// Id:   profile.Id,
				Bot:    agent.Bot,
				Fields: []string{"metadata"},
			},
			agent.Bot,
		)

		if err != nil {
			log.Error().Msg(err.Error())
			return nil, err
		}
	}
	return &infobipWABot{
		agent.Bot.GetId(),
		apiKey,
		scenarioKey,
		number,
		url,
		agent,
	}, nil
}

func createWAScenario(apiKey, number, url string) (scenarioKey string, err error) {
	body, err := json.Marshal(CreateScenarioRequest{
		Name:    number,
		Default: true,
		Flow: []*Flow{
			{
				From:    number,
				Channel: "WHATSAPP",
			},
		},
	})
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", url, scenarioRoute), bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("App %s", apiKey))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	res.Body.Close()
	var scenario *CreateScenarioResponse
	err = json.Unmarshal(data, scenario)
	if err != nil {
		return
	}
	scenarioKey = scenario.Key
	return
}

func (_ *infobipWABot) Close() error {
	return nil
}

// String "infobip_whatsapp" provider's name
func (_ *infobipWABot) String() string {
	return "infobip_whatsapp"
}

func (b *infobipWABot) Register(ctx context.Context, linkURL string) error {
	return nil
}

// Deregister infobipWABot Bot Webhook endpoint URI
func (b *infobipWABot) Deregister(ctx context.Context) error {
	return nil
}

func (b *infobipWABot) SendNotify(ctx context.Context, notify *bot.Update) error {

	msg := SendMessageWARequest{
		ScenarioKey: b.scenarioKey,
		WhatsApp:    &WhatsAppMessage{},
		Destinations: []*Destination{
			{
				To: &NumberDestination{
					PhoneNumber: notify.Chat.ChatID,
				},
			},
		},
	}

	switch notify.Message.Type {

	case "text":
		msg.WhatsApp.Text = "webitel " + notify.Message.GetText()

	case "file":
		whatsappMessageFile(notify.Message.GetFile(), &msg)

	case "closed":
		msg.WhatsApp.Text = "webitel " + notify.Message.GetText()

	default:
		return nil // UNKNOWN Event
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	infobipReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", b.url, messageRoute), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	infobipReq.Header.Set("Content-Type", "application/json")
	infobipReq.Header.Set("Authorization", fmt.Sprintf("App %s", b.apiKey))

	infobipRes, err := http.DefaultClient.Do(infobipReq)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(infobipRes.Body)
	return err
}

func whatsappMessageFile(f *chat.File, m *SendMessageWARequest) {

	switch {

	case strings.HasPrefix(f.Mime, "image"):
		m.WhatsApp.ImageURL = f.Url

	case strings.HasPrefix(f.Mime, "video"):
		m.WhatsApp.VideoURL = f.Url

	case strings.HasPrefix(f.Mime, "audio"):
		m.WhatsApp.AudioURL = f.Url

	default:
		m.WhatsApp.FileURL = f.Url
		m.WhatsApp.Text = f.Name
	}
}

// WebHook implementes provider.Receiver interface for infobipWA
func (b *infobipWABot) WebHook(reply http.ResponseWriter, notice *http.Request) {

	update := &InfobipWABody{}

	if err := json.NewDecoder(notice.Body).Decode(update); err != nil {
		log.Error().Msgf("could not decode request body: %s", err)
		return
	}

	for _, msg := range update.Results {

		log.Debug().
			Str("from", msg.From).
			Str("username", msg.Contact.Name).
			Str("type", msg.Message.Type).
			Str("text", msg.Message.Text).
			Msg("receive message")

		contact := &bot.Account{
			ID:       0, // LOOKUP
			Username: msg.Contact.Name,
			Channel:  "infobip_whatsapp",
			Contact:  msg.From,
		}

		// endregion

		// region: channel
		chatID := msg.From
		channel, err := b.Gateway.GetChannel(
			notice.Context(), chatID, contact,
		)
		if err != nil {
			// Failed locate chat channel !
			re := errors.FromError(err)
			if re.Code == 0 {
				re.Code = (int32)(http.StatusBadGateway)
			}
			http.Error(reply, re.Detail, (int)(re.Code))
			return // 503 Bad Gateway
		}

		sendUpdate := bot.Update{
			Title: channel.Title,
			Chat:  channel,
			User:  contact,
		}

		switch msg.Message.Type {

		case "TEXT":
			sendUpdate.Message = &chat.Message{
				Type: "text",
				Text: strings.TrimPrefix(msg.Message.Text, "webitel "),
			}

		case "IMAGE", "VIDEO", "DOCUMENT":
			sendUpdate.Message = &chat.Message{
				Type: "file",
				File: &chat.File{
					Url:  msg.Message.URL,
					Name: msg.Message.Caption,
				},
			}

		case "AUDIO":
			sendUpdate.Message = &chat.Message{
				Type: "file",
				File: &chat.File{
					Url:  msg.Message.URL,
					Name: msg.Message.Caption,
				},
			}

		case "VOICE":
			sendUpdate.Message = &chat.Message{
				Type: "file",
				File: &chat.File{
					Url:  msg.Message.URL,
					Name: msg.Message.Caption,
				},
			}

		case "CONTACT":
			// TODO ....
			continue

		case "LOCATION":
			// TODO ....
			continue
		}

		err = b.Gateway.Read(notice.Context(), &sendUpdate)

		if err != nil {
			//http.Error(reply, "Failed to deliver infobip_whatsapp .Update message", http.StatusInternalServerError)
			//return // 502 Bad Gateway
		}
	}

	reply.WriteHeader(http.StatusOK)
	return

}
