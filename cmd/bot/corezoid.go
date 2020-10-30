package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/rs/zerolog"
	"io/ioutil"
	"net/http"
	"strconv"

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

type corezoidBot struct {
	profileID int64
	url       string
	log       *zerolog.Logger
	client    pbchat.ChatService
}

//func NewCorezoidClient(url string) *corezoidClient {
//	return &corezoidClient{
//		//token,
//		url,
//	}
//}

func ConfigureCorezoid(profile *pbchat.Profile, client pbchat.ChatService, log *zerolog.Logger) ChatBot {
	//token, ok := profile.Variables["token"]
	//if !ok {
	//	b.log.Fatal().Msg("token not found")
	//	return nil
	//}
	url, ok := profile.Variables["url"]
	if !ok {
		log.Fatal().Msg("url not found")
		return nil
	}
	return &corezoidBot{
		profile.Id,
		url,
		log,
		client,
	}
}

func (b *corezoidBot) DeleteProfile() error {
	return nil
}

func (b *corezoidBot) SendMessage(req *pb.SendMessageRequest) error {
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
	corezoidReq, err := http.NewRequest(http.MethodPost, b.url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	corezoidReq.Header.Set("Content-Type", "application/json")
	//corezoidReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", profile.token))

	corezoidRes, err := http.DefaultClient.Do(corezoidReq)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(corezoidRes.Body)
	return err
}

func (b *corezoidBot) Handler(r *http.Request) {
	p := strconv.Itoa(int(b.profileID))

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
		ProfileId:  b.profileID,
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
