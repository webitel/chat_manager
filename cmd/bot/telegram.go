package main

import (

	// "fmt"
	"sync"
	"net/http"
	"strconv"
	"strings"
	"context"
	"encoding/json"

	"github.com/rs/zerolog"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	
)

// TelegramBot profile runtime
type TelegramBot struct {
	Profile *pbchat.Profile
	API     *tgbotapi.BotAPI    // Telegram-server-side bot API
	client   pbchat.ChatService // Webitel-internal-chat service API
	log      zerolog.Logger
	// this bot tracking active channels
	chatMx   sync.RWMutex
	chat     map[int64]*tgbotapi.Update
}

// // TelegramChat represents single user-bot chat channel
// type TelegramChat struct {
// 	Bot *TelegramBot         // To: internal (Webitel)
// 	Chat *tgbotapi.Chat      // From: external (Telegram)
// 	Current *tgbotapi.Update // Current: latest update
// }

func NewTelegramBot(profile *pbchat.Profile, client pbchat.ChatService, log *zerolog.Logger) ChatBot {
	token, ok := profile.Variables["token"]
	if !ok {
		log.Fatal().Msg("token not found")
		return nil
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		// log.Fatal().Msg(err.Error())
		log.Error().Err(err).
			Int64("pid", profile.Id).
			Str("gate", "telegram").
			Str("bot", profile.Name).
			Str("uri", "/" + profile.UrlId).
			Msg("Failed to init gateway")
		return nil
	}
	// webhookInfo := tgbotapi.NewWebhookWithCert(fmt.Sprintf("%s/telegram/%v", cfg.TgWebhook, profile.Id), cfg.CertPath)
	webhook := tgbotapi.NewWebhook(
		strings.TrimRight(cfg.SiteURL, "/") +"/"+ profile.UrlId,
	)
	_, err = bot.SetWebhook(webhook)
	if err != nil {
		log.Fatal().Msg(err.Error())
		return nil
	}
	return &TelegramBot{
		Profile: profile,
		API:     bot,
		client:  client,
		log:     log.With().

			Int64("pid", profile.Id).
			Str("gate", "telegram").
			Str("bot", bot.Self.UserName).
			Str("uri", "/" + profile.UrlId).
			
		Logger(),
		// cache: default size
		chat:    make(map[int64]*tgbotapi.Update, 64),
	}
}

func (bot *TelegramBot) DeleteProfile() error {
	// if _, err := bot.API.RemoveWebhook(); err != nil {
	// 	return err
	// }
	return nil
}

func (bot *TelegramBot) SendMessage(req *pb.SendMessageRequest) error {
	chatID, err := strconv.ParseInt(req.ExternalUserId, 10, 64)
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(chatID, req.GetMessage().GetText())
	// msg.ReplyToMessageID = update.Message.MessageID
	_, err = bot.API.Send(msg)
	if err != nil {
		bot.log.Error().Err(err).Msg("Failed to send message")
		return err
	}
	bot.log.Debug().

		Int64("chat-id", chatID).
		Str("type", "text").
		Str("text", req.GetMessage().GetText()).

	Msg("SENT Update")
	return nil
}

// Handler receives new Update message from telegram server
func (bot *TelegramBot) Handler(w http.ResponseWriter, r *http.Request) {
	// Contact: Chat-BOT profile unique ID represents contact string
	contact := strconv.FormatInt(bot.Profile.Id, 10)

	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		bot.log.Error().Err(err).Msg("Failed to decode update request")
		// FIXME: notify telegram about an error; guess: will try again to resend .this later !
		http.Error(w, "Failed to decode update request", http.StatusBadRequest) // 400 
		return
	}

	recvMessage := update.Message
	if recvMessage == nil {
		recvMessage = update.EditedMessage
	}

	if recvMessage != update.Message {
		
		bot.log.Warn().

			Int(  "telegram-id", recvMessage.From.ID).
			Str(  "username",    recvMessage.From.UserName).
			Int64("chat-id",     recvMessage.Chat.ID).
			// Str("first_name", message.From.FirstName).
			// Str("last_name",  message.From.LastName)

		Msg("IGNORE Update; NOT Message")
		
		return // 200 IGNORE
	}

	bot.log.Debug().

		Int(  "telegram-id", recvMessage.From.ID).
		Str(  "username",    recvMessage.From.UserName).
		Int64("chat-id",     recvMessage.Chat.ID).
		// Str("first_name", recvMessage.From.FirstName).
		// Str("last_name",  recvMessage.From.LastName).
		Str(  "text",        recvMessage.Text).

	Msg("RECV Update")

	// region: cache latest chat update
	// bot.set(&update)
	// endregion

	strChatID := strconv.FormatInt(recvMessage.Chat.ID, 10)

	username := recvMessage.From.FirstName
	if username != "" && recvMessage.From.LastName != "" {
		username += " " + recvMessage.From.LastName
	}

	if username == "" {
		username = recvMessage.From.UserName
	}
	
	check := &pbchat.CheckSessionRequest{
		ExternalId: strChatID,
		ProfileId:  bot.Profile.Id,
		Username:   username,
	}
	resCheck, err := bot.client.CheckSession(context.Background(), check)
	if err != nil {
		bot.log.Error().Err(err).Msg("Failed to get channel")
		return
	}
	bot.log.Debug().
		
		Str("channel_id", resCheck.ChannelId).
		Int64("contact_id", resCheck.ClientId).
		Bool("new", !resCheck.Exists).
		Msg("CHAT-Channel")

	if !resCheck.Exists {
		start := &pbchat.StartConversationRequest{
			DomainId: bot.Profile.DomainId,
			Username: check.Username,
			User: &pbchat.User{
				UserId:     resCheck.ClientId,
				Type:       "telegram",
				Connection: contact, // telegram: specific contact uri
				Internal:   false,
			},
			Message: &pbchat.Message{
				Type: "text",
				Value: &pbchat.Message_Text{
					Text: recvMessage.Text,
				},
				Variables: nil, // map[string]string{},
			},
		}
		_, err := bot.client.StartConversation(context.Background(), start)
		if err != nil {
			bot.log.Error().Err(err).Msg("Failed to start new chat")
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
				Text: recvMessage.Text,
			},
		}
		message.Message = textMessage
		// }

		_, err := bot.client.SendMessage(context.TODO(), message)
		if err != nil {
			bot.log.Error().Err(err).Msg("Failed to route message")
		}
	}
}

/*

// chat returns cached (internaly stored) active tracking chat communication channel
func (bot *TelegramBot) get(id int64) *tgbotapi.Update {
	
	bot.chatMx.RLock()
	state := bot.chat[id]
	bot.chatMx.RUnlock()

	return state
}

// cache stores (runtime only) tracking chat communication channel
func (bot *TelegramBot) set(state *tgbotapi.Update) {
	
	id := state.Message.Chat.ID

	bot.chatMx.Lock()
	bot.chat[id] = state
	bot.chatMx.Unlock()


}

*/