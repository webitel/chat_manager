package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	"github.com/rs/zerolog/log"
)

const (
	scenarioRoute = "omni/1/scenarios"
	messageRoute  = "omni/1/advanced"
)

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
	Text string `json:"text"`
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
	Contact         `json:"contact"`
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

type infobipWAClient struct {
	apiKey      string
	scenarioKey string
	number      string
	url         string
}

func NewInfobipWAClient(apiKey, scenarioKey, number, url string) *infobipWAClient {
	return &infobipWAClient{
		apiKey,
		scenarioKey,
		number,
		url,
	}
}

func (b *botService) configureInfobipWA(profile *pbchat.Profile) *infobipWAClient {
	apiKey, ok := profile.Variables["api_key"]
	if !ok {
		b.log.Fatal().Msg("api key not found")
		return nil
	}
	number, ok := profile.Variables["number"]
	if !ok {
		b.log.Fatal().Msg("api key not found")
		return nil
	}
	url, ok := profile.Variables["url"]
	if !ok {
		b.log.Fatal().Msg("api key not found")
		return nil
	}
	scenarioKey, _ := profile.Variables["scenario_key"]
	if !ok {
		b.log.Debug().Msg("creating scenario")
		var err error
		scenarioKey, err = b.createWAScenario(apiKey, number, url)
		if err != nil {
			b.log.Fatal().Msg(err.Error())
			return nil
		}
		profile.Variables["scenario_key"] = scenarioKey
		if _, err := b.client.UpdateProfile(context.Background(), &pbchat.UpdateProfileRequest{
			// Id:   profile.Id,
			Item: profile,
		}); err != nil {
			b.log.Fatal().Msg(err.Error())
			return nil
		}
	}
	return NewInfobipWAClient(apiKey, scenarioKey, number, url)
}

func (b *botService) createWAScenario(apiKey, number, url string) (scenarioKey string, err error) {
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

func (b *botService) addProfileInfobipWA(req *pb.AddProfileRequest) error {
	bot := b.configureInfobipWA(req.Profile)
	b.infobipWABots[req.Profile.Id] = bot
	b.botMap[req.Profile.Id] = "infobip-whatsapp"
	return nil
}

func (b *botService) deleteProfileInfobipWA(req *pb.DeleteProfileRequest) error {
	delete(b.infobipWABots, req.Id)
	delete(b.botMap, req.Id)
	return nil
}

func (b *botService) sendMessageInfobipWA(req *pb.SendMessageRequest) error {
	profile := b.infobipWABots[req.ProfileId]
	body, err := json.Marshal(SendMessageWARequest{
		ScenarioKey: profile.scenarioKey,
		WhatsApp: &WhatsAppMessage{
			Text: "webitel " + req.GetMessage().GetText(),
		},
		Destinations: []*Destination{{
			To: &NumberDestination{
				PhoneNumber: req.ExternalUserId,
			},
		}},
	})
	if err != nil {
		return err
	}
	infobipReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", profile.url, messageRoute), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	infobipReq.Header.Set("Content-Type", "application/json")
	infobipReq.Header.Set("Authorization", fmt.Sprintf("App %s", profile.apiKey))

	infobipRes, err := http.DefaultClient.Do(infobipReq)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(infobipRes.Body)
	return err
}

func (b *botService) infobipWAHandler(profileID int64, r *http.Request) {
	p := strconv.Itoa(int(profileID))

	update := &InfobipWABody{}
	if err := json.NewDecoder(r.Body).Decode(update); err != nil {
		log.Error().Msgf("could not decode request body: %s", err)
		return
	}
	if len(update.Results) == 0 ||
		(Message{}) == update.Results[0].Message {
		log.Warn().Msg("no data")
		return
	}
	if update.Results[0].Message.Text == "" {
		return
	}
	b.log.Debug().
		Str("from", update.Results[0].From).
		Str("username", update.Results[0].Contact.Name).
		Str("text", update.Results[0].Message.Text).
		Msg("receive message")

	check := &pbchat.CheckSessionRequest{
		ExternalId: update.Results[0].From,
		ProfileId:  profileID,
		Username:   update.Results[0].Contact.Name,
	}
	resCheck, err := b.client.CheckSession(context.Background(), check)
	if err != nil {
		b.log.Error().Msg(err.Error())
		return
	}
	b.log.Debug().
		Bool("exists", resCheck.Exists).
		Str("channel_id", resCheck.ChannelId).
		Int64("client_id", resCheck.ClientId).
		Msg("check user")

	if !resCheck.Exists {
		start := &pbchat.StartConversationRequest{
			User: &pbchat.User{
				UserId:     resCheck.ClientId,
				Type:       "infobip-whatsapp",
				Connection: p,
				Internal:   false,
			},
			Username: check.Username,
			DomainId: 1,
		}
		_, err := b.client.StartConversation(context.Background(), start)
		if err != nil {
			b.log.Error().Msg(err.Error())
			return
		}
	} else {
		textMessage := &pbchat.Message{
			Type: strings.ToLower(update.Results[0].Message.Type),
			Value: &pbchat.Message_Text{
				Text: strings.TrimPrefix(update.Results[0].Message.Text, "webitel "),
			},
		}
		message := &pbchat.SendMessageRequest{
			AuthUserId: resCheck.ClientId,
			Message:    textMessage,
			ChannelId:  resCheck.ChannelId,
			FromFlow:   false,
		}
		_, err := b.client.SendMessage(context.Background(), message)
		if err != nil {
			b.log.Error().Msg(err.Error())
		}
	}
}
