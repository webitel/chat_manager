package main

import (
	"github.com/micro/go-micro/v2/errors"
	"context"
	"net"
	"net/http"
	"strings"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type Configure func(profile *pbchat.Profile, client pbchat.ChatService, log *zerolog.Logger) ChatBot

var ConfigureBotsMap = map[string]Configure{
	"telegram":         NewTelegramBot, // ConfigureTelegram,
	"infobip-whatsapp": ConfigureInfobipWA,
	"corezoid":         ConfigureCorezoid,
}

type ChatServer interface {
	WebhookFunc(w http.ResponseWriter, r *http.Request)
	SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error
	AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error
	DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error
	StartWebhookServer() error
	StopWebhookServer() error
}

type ChatBot interface {
	Handler(w http.ResponseWriter, r *http.Request)
	SendMessage(req *pb.SendMessageRequest) error
	DeleteProfile() error
	// // String type name e.g.: telegram, viber, facebook
	// String() string
}

type botService struct {
	log    *zerolog.Logger
	client pbchat.ChatService
	router *mux.Router
	urlMap map[string]int64
	bots   map[int64]ChatBot
	exit   chan chan error
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
		urlMap: make(map[string]int64),
		bots:   make(map[int64]ChatBot),
		exit:   make(chan chan error),
	}

	b.router.HandleFunc("/{url_id}", b.WebhookFunc).
		Methods("POST")

	res, err := b.client.GetProfiles(context.Background(), &pbchat.GetProfilesRequest{Size: 100})
	if err != nil || res == nil {
		b.log.Fatal().Msg(err.Error())
		return nil
	}

	for _, profile := range res.Items {
		b.urlMap[profile.UrlId] = profile.Id
		configure, ok := ConfigureBotsMap[profile.Type]
		if !ok {
			b.log.Warn().
				Int64("id", profile.Id).
				Str("type", profile.Type).
				Str("name", profile.Name).
				Int64("domain_id", profile.DomainId).
				Msg("wrong profile type")
			continue
		}
		b.bots[profile.Id] = configure(profile, b.client, b.log)
	}

	return b
}

func (b *botService) StartWebhookServer() error {

	srv, err := net.Listen("tcp", cfg.Address)

	if err != nil {
		return err
	}

	if log := b.log.Info(); log.Enabled() {
		log.Msgf("Server [http] Listening on %s", srv.Addr().String())
	}

	go func() {
		if err := http.Serve(srv, b.router); err != nil {
			// if err != http.ErrServerClosed {
			// 	if log := b.log.Error(); log.Enabled() {
			// 		log.Err(err).Msg("Server [http] Shutted down due to an error")
			// 	}
			// 	return
			// }
		}
		if log := b.log.Info(); log.Enabled() {
			log.Err(err).Msg("Server [http] Shutted down")
		}
	}()

	go func() {
		ch := <-b.exit
		ch <- srv.Close()
	}()

	return nil
}

func (b *botService) StopWebhookServer() error {

	ch := make(chan error)
	b.exit <- ch
	<-ch // await: http.ErrServerClosed, "use of closed network connection"

	b.log.Info().
		Msg("removing webhooks")
	for k := range b.bots {
		if err := b.bots[k].DeleteProfile(); err != nil {
			b.log.Error().Msg(err.Error())
		}
		delete(b.bots, k)
	}

	return nil
}

func (srv *botService) SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error {
	// b.log.Debug().
	// 	Int64("pid", req.GetProfileId()).
	// 	Str("type", req.GetMessage().GetType()).
	// 	Str("to-user", req.GetExternalUserId()).
	// 	Msg("SEND Update")

	bot := srv.bots[req.ProfileId]
	
	if bot == nil {
		// TODO: try to fetch from persistent store (db)
		srv.log.Error().
		
			Int64("pid", req.ProfileId).
			Str("err", "bot: not found").
			
		Msg("Failed to send message")

		return errors.NotFound(
			"webitel.chat.bot.not_found",
			"bot: profile %d not found",
			req.ProfileId,
		)
	}

	// perform
	err := bot.SendMessage(req)
	
	if err != nil {
		srv.log.Error().Err(err).Msg("Failed to send message")
		return err
	}

	srv.log.Debug().
	
		Int64("pid", req.GetProfileId()).
		Str("type", req.GetMessage().GetType()).
		Str("chat-id", req.GetExternalUserId()).
		Str("text", req.GetMessage().GetText()).
	
	Msg("SENT")

	return err
}

func (b *botService) AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error {
	b.log.Info().
		Int64("id", req.GetProfile().GetId()).
		Str("type", req.GetProfile().GetType()).
		Str("name", req.GetProfile().GetName()).
		Int64("domain_id", req.GetProfile().GetDomainId()).
		Msg("add profile")
	b.urlMap[req.Profile.UrlId] = req.Profile.Id
	configure, ok := ConfigureBotsMap[req.Profile.Type]
	if !ok {
		b.log.Warn().
			Int64("id", req.Profile.Id).
			Str("type", req.Profile.Type).
			Str("name", req.Profile.Name).
			Int64("domain_id", req.Profile.DomainId).
			Msg("wrong profile type")
		return nil
	}
	b.bots[req.Profile.Id] = configure(req.Profile, b.client, b.log)
	return nil
}

func (b *botService) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error {
	b.log.Info().
		Int64("id", req.GetId()).
		Msg("delete profile")
	delete(b.urlMap, req.UrlId)
	if err := b.bots[req.Id].DeleteProfile(); err != nil {
		b.log.Error().Msg(err.Error())
		return err
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
	b.bots[profileID].Handler(w, r)
}
