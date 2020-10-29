package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type ChatServer interface {
	WebhookFunc(w http.ResponseWriter, r *http.Request)
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
	corezoidBots  map[int64]*corezoidClient
	botMap        map[int64]string
	urlMap        map[string]int64
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
	b.urlMap = make(map[string]int64)
	b.telegramBots = make(map[int64]*tgbotapi.BotAPI)
	b.infobipWABots = make(map[int64]*infobipWAClient)
	b.corezoidBots = make(map[int64]*corezoidClient)

	b.router.HandleFunc("/{url_id}", b.WebhookFunc).
		Methods("POST")
	//b.router.HandleFunc("/telegram/{profile_id}", b.TelegramWebhookHandler).
	//	Methods("POST")
	//b.router.HandleFunc("/infobip/whatsapp/{profile_id}", b.InfobipWAWebhookHandler).
	//	Methods("POST")
	//b.router.HandleFunc("/corezoid/{profile_id}", b.CorezoidWebhookHandler).
	//	Methods("POST")

	res, err := b.client.GetProfiles(context.Background(), &pbchat.GetProfilesRequest{Size: 100})
	if err != nil || res == nil {
		b.log.Fatal().Msg(err.Error())
		return nil
	}

	for _, profile := range res.Items {
		b.urlMap[profile.UrlId] = profile.Id
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
		case "corezoid":
			{
				b.botMap[profile.Id] = "corezoid"
				b.corezoidBots[profile.Id] = b.configureCorezoid(profile)
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
	case "corezoid":
		{
			if err := b.sendMessageCorezoid(req); err != nil {
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
	b.urlMap[req.Profile.UrlId] = req.Profile.Id
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
	case "corezoid":
		{
			if err := b.addProfileCorezoid(req); err != nil {
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
	delete(b.urlMap, req.UrlId)
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
	case "corezoid":
		{
			if err := b.deleteProfileCorezoid(req); err != nil {
				b.log.Error().Msg(err.Error())
				return err
			}
		}
	}
	return nil
}

func (b *botService) WebhookFunc(w http.ResponseWriter, r *http.Request) {
	urlID := strings.TrimPrefix(r.URL.Path, "/")
	if urlID == "" {
		b.log.Error().Msg("url id doesn't exist")
		return
	}
	profileID, ok := b.urlMap[urlID]
	if !ok {
		b.log.Error().Msg("profile id not found")
		return
	}
	switch b.botMap[profileID] {
	case "telegram":
		{
			b.telegramHandler(profileID, r)
		}
	case "infobip-whatsapp":
		{
			b.infobipWAHandler(profileID, r)
		}
	case "corezoid":
		{

		}
	}
}
