package main

import (
	"context"
	"fmt"
	"net/http"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type ChatServer interface {
	TelegramWebhookHandler(w http.ResponseWriter, r *http.Request)
	SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error
	AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error
	DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error
	StartWebhookServer() error
	StopWebhookServer() error
}

type botService struct {
	log           *zerolog.Logger
	client        pbchat.ChatService
	router        *mux.Router
	telegramBots  map[int64]*tgbotapi.BotAPI
	infobipWABots map[int64]*infobipWAClient
	botMap        map[int64]string
}

func NewBotService(
	log *zerolog.Logger,
	client pbchat.ChatService,
	router *mux.Router,
) *botService {
	b := &botService{
		log:    log,
		client: client,
		router: router,
	}
	b.botMap = make(map[int64]string)
	b.telegramBots = make(map[int64]*tgbotapi.BotAPI)
	b.infobipWABots = make(map[int64]*infobipWAClient)

	b.router.HandleFunc("/telegram/{profile_id}", b.TelegramWebhookHandler).
		Methods("POST")
	b.router.HandleFunc("/infobip/whatsapp/{profile_id}", b.InfobipWAWebhookHandler).
		Methods("POST")

	res, err := b.client.GetProfiles(context.Background(), &pbchat.GetProfilesRequest{Size: 100})
	if err != nil || res == nil {
		b.log.Fatal().Msg(err.Error())
		return nil
	}

	for _, profile := range res.Items {
		switch profile.Type {
		case "telegram":
			{
				b.botMap[profile.Id] = "telegram"
				b.telegramBots[profile.Id] = b.configureTelegram(profile)
			}
		case "infobip-whatsapp":
			{
				b.botMap[profile.Id] = "infobip-whatsapp"
				b.infobipWABots[profile.Id] = b.configureInfobipWA(profile)
			}
		default:
			b.log.Warn().
				Int64("id", profile.Id).
				Str("type", profile.Type).
				Str("name", profile.Name).
				Int64("domain_id", profile.DomainId).
				Msg("wrong profile type")
		}
	}

	return b
}

func (b *botService) StartWebhookServer() error {
	b.log.Info().
		Int("port", cfg.AppPort).
		Msg("webhook started listening on port")
	return http.ListenAndServe(fmt.Sprintf(":%v", cfg.AppPort), b.router) // srv.ListenAndServeTLS(cfg.CertPath, cfg.KeyPath)
}

func (b *botService) StopWebhookServer() error {
	b.log.Info().
		Msg("removing webhooks")
	for k := range b.telegramBots {
		if _, err := b.telegramBots[k].RemoveWebhook(); err != nil {
			b.log.Error().Msg(err.Error())
		}
		delete(b.telegramBots, k)
	}
	return nil
}

func (b *botService) SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error {
	b.log.Debug().
		Int64("profile_id", req.GetProfileId()).
		Str("type", req.GetMessage().GetType()).
		Str("user_id", req.GetExternalUserId()).
		Msg("send message")
	switch b.botMap[req.ProfileId] {
	case "telegram":
		{
			if err := b.sendMessageTelegram(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	case "infobip-whatsapp":
		{
			if err := b.sendMessageInfobipWA(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	}
	return nil
}

func (b *botService) AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error {
	b.log.Info().
		Int64("id", req.GetProfile().GetId()).
		Str("type", req.GetProfile().GetType()).
		Str("name", req.GetProfile().GetName()).
		Int64("domain_id", req.GetProfile().GetDomainId()).
		Msg("add profile")
	switch req.Profile.Type {
	case "telegram":
		{
			if err := b.addProfileTelegram(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	case "infobip-whatsapp":
		{
			if err := b.addProfileInfobipWA(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	}
	return nil
}

func (b *botService) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error {
	b.log.Info().
		Int64("id", req.GetId()).
		Msg("delete profile")
	switch b.botMap[req.Id] {
	case "telegram":
		{
			if err := b.deleteProfileTelegram(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	case "infobip-whatsapp":
		{
			if err := b.deleteProfileInfobipWA(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	}
	return nil
}
