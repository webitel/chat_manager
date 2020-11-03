package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"net/http"
	"strconv"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

type telegramBot struct {
	profileID int64
	API       *tgbotapi.BotAPI
	log       *zerolog.Logger
	client    pbchat.ChatService
}

type telegramBody struct {
	Message struct {
		MessageID int64       `json:"message_id"`
		Text      string      `json:"text"`
		Photo     []PhotoSize `json:"photo"` // image/jpeg
		From      struct {
			Username  string `json:"username"`
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int64  `json:"width"`
	Height       int64  `json:"height"`
	FileSize     int64  `json:"file_size"`
}

func ConfigureTelegram(profile *pbchat.Profile, client pbchat.ChatService, log *zerolog.Logger) ChatBot {
	token, ok := profile.Variables["token"]
	if !ok {
		log.Fatal().Msg("token not found")
		return nil
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal().Msg(err.Error())
		return nil
	}
	// webhookInfo := tgbotapi.NewWebhookWithCert(fmt.Sprintf("%s/telegram/%v", cfg.TgWebhook, profile.Id), cfg.CertPath)
	webhookInfo := tgbotapi.NewWebhook(fmt.Sprintf("%s/%s", cfg.Webhook, profile.UrlId))
	_, err = bot.SetWebhook(webhookInfo)
	if err != nil {
		log.Fatal().Msg(err.Error())
		return nil
	}
	return &telegramBot{
		profile.Id,
		bot,
		log,
		client,
	}
}

func (b *telegramBot) DeleteProfile() error {
	if _, err := b.API.RemoveWebhook(); err != nil {
		return err
	}
	return nil
}

func (b *telegramBot) SendMessage(req *pb.SendMessageRequest) error {
	id, err := strconv.ParseInt(req.ExternalUserId, 10, 64)
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(id, req.GetMessage().GetText())
	// msg.ReplyToMessageID = update.Message.MessageID
	_, err = b.API.Send(msg)
	if err != nil {
		return err
	}
	return nil
}

func (b *telegramBot) Handler(r *http.Request) {
	p := strconv.Itoa(int(b.profileID))

	update := &telegramBody{}
	if err := json.NewDecoder(r.Body).Decode(update); err != nil {
		log.Error().Msgf("could not decode request body: %s", err)
		return
	}

	b.log.Debug().
		Int64("id", update.Message.From.ID).
		Str("username", update.Message.From.Username).
		Str("first_name", update.Message.From.FirstName).
		Str("last_name", update.Message.From.LastName).
		Str("text", update.Message.Text).
		Msg("receive message")

	strChatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	username := fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName)
	check := &pbchat.CheckSessionRequest{
		ExternalId: strChatID,
		ProfileId:  b.profileID,
		Username:   username,
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
		// if update.Message.Text != "/start" {
		// 	textMessage := &pbchat.Message{
		// 		Type: "text",
		// 		Value: &pbchat.Message_TextMessage_{
		// 			TextMessage: &pbchat.Message_TextMessage{
		// 				Text: update.Message.Text,
		// 			},
		// 		},
		// 	}
		// 	message := &pbchat.SendMessageRequest{
		// 		Message:   textMessage,
		// 		ChannelId: resStart.ChannelId,
		// 		FromFlow:  false,
		// 	}
		// 	_, err = b.client.SendMessage(context.Background(), message)
		// 	if err != nil {
		// 		b.log.Error().Msg(err.Error())
		// 	}
		// }
	} else {
		message := &pbchat.SendMessageRequest{
			// Message:   textMessage,
			AuthUserId: resCheck.ClientId,
			ChannelId:  resCheck.ChannelId,
		}
		// if update.Message.Photo != nil {
		// 	fileURL, err := b.telegramBots[profileID].GetFileDirectURL(
		// 		update.Message.Photo[len(update.Message.Photo)-1].FileID,
		// 	)
		// 	if err != nil {
		// 		log.Error().Msg(err.Error())
		// 		return
		// 	}
		// 	fileRes, err := http.Get(fileURL)
		// 	if err != nil {
		// 		log.Error().Msg(err.Error())
		// 		return
		// 	}
		// 	defer fileRes.Body.Close()
		// 	if fileRes.StatusCode != 200 {
		// 		log.Error().Int("status", fileRes.StatusCode).Msgf("failed to download image")
		// 		return
		// 	}
		// 	f, err := ioutil.ReadAll(fileRes.Body)
		// 	// m, _, err := image.Decode(fileRes.Body)
		// 	// if err != nil {
		// 	// 	log.Error().Msg(err.Error())
		// 	// 	return
		// 	// }
		// 	fileMessage := &pbchat.Message{
		// 		Type: "text",
		// 		Value: &pbchat.Message_Text{
		// 			Text: update.Message.Text,
		// 		},
		// 	}
		// } else {
		textMessage := &pbchat.Message{
			Type: "text",
			Value: &pbchat.Message_Text{
				Text: update.Message.Text,
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
