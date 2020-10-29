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

type corezoidReqBody struct {
	ID      string `json:"id,omitempty"`
	Text    string `json:"text,omitempty"`
	Action  string `json:"action,omitempty"`
	Channel string `json:"channel,omitempty"`
	Type    string `json:"type,omitempty"`
}

type corezoidResBody struct {
	ID           string `json:"id,omitempty"`
	Text         string `json:"text,omitempty"`
	Action       string `json:"action,omitempty"`
	Channel      string `json:"channel,omitempty"`
	OperatorName string `json:"operator_name,omitempty"`
	Type         string `json:"type,omitempty"`
}

type corezoidClient struct {
	token string
	url   string
}

func NewCorezoidClient(token, url string) *corezoidClient {
	return &corezoidClient{
		token,
		url,
	}
}

func (b *botService) configureCorezoid(profile *pbchat.Profile) *corezoidClient {
	token, ok := profile.Variables["token"]
	if !ok {
		b.log.Fatal().Msg("token not found")
		return nil
	}
	url, ok := profile.Variables["url"]
	if !ok {
		b.log.Fatal().Msg("url not found")
		return nil
	}
	return NewCorezoidClient(token, url)
}

func (b *botService) addProfileCorezoid(req *pb.AddProfileRequest) error {
	bot := b.configureCorezoid(req.Profile)
	b.corezoidBots[req.Profile.Id] = bot
	b.botMap[req.Profile.Id] = "corezoid"
	return nil
}

func (b *botService) deleteProfileCorezoid(req *pb.DeleteProfileRequest) error {
	delete(b.corezoidBots, req.Id)
	delete(b.botMap, req.Id)
	return nil
}

func (b *botService) sendMessageCorezoid(req *pb.SendMessageRequest) error {
	profile := b.corezoidBots[req.ProfileId]
	body, err := json.Marshal(corezoidResBody{
		ID:           req.GetExternalUserId(),
		Text:         req.GetMessage().GetText(),
		Action:       req.GetMessage().GetVariables()["action"],
		Channel:      req.GetMessage().GetVariables()["channel"],
		OperatorName: req.GetMessage().GetVariables()["operator_name"],
		Type:         req.GetMessage().GetType(),
	})
	if err != nil {
		return err
	}
	corezoidReq, err := http.NewRequest(http.MethodPost, profile.url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	corezoidReq.Header.Set("Content-Type", "application/json")
	corezoidReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", profile.token))

	corezoidRes, err := http.DefaultClient.Do(corezoidReq)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(corezoidRes.Body)
	return err
}

func (b *botService) CorezoidWebhookHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/corezoid/")
	profileID, err := strconv.ParseInt(p, 10, 64)
	if err != nil {
		b.log.Error().Msg(err.Error())
		return
	}
	update := &corezoidReqBody{}
	if err := json.NewDecoder(r.Body).Decode(update); err != nil {
		log.Error().Msgf("could not decode request body: %s", err)
		return
	}

	//b.log.Debug().
	//	Int64("id", update.ID).
	//	Str("username", update.Message.From.Username).
	//	Str("first_name", update.Message.From.FirstName).
	//	Str("last_name", update.Message.From.LastName).
	//	Str("text", update.Message.Text).
	//	Msg("receive message")

	strChatID := update.ID //strconv.FormatInt(update.ID, 10)

	check := &pbchat.CheckSessionRequest{
		ExternalId: strChatID,
		ProfileId:  profileID,
		//Username:   update.Message.From.Username,
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
				Type:       "telegram",
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
		message := &pbchat.SendMessageRequest{
			// Message:   textMessage,
			AuthUserId: resCheck.ClientId,
			ChannelId:  resCheck.ChannelId,
			FromFlow:   false,
		}
		textMessage := &pbchat.Message{
			Type: "text",
			Value: &pbchat.Message_Text{
				Text: update.Text,
			},
		}
		message.Message = textMessage
		// }

		_, err := b.client.SendMessage(context.Background(), message)
		if err != nil {
			b.log.Error().Msg(err.Error())
		}
	}
}
