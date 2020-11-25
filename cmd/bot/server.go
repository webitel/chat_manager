package main

import (

	"net"
	// "sync"
	"context"
	"strings"
	"net/http"

	"github.com/micro/go-micro/v2/errors"

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
	// Handler implements .Receiver interface
	Handler(w http.ResponseWriter, r *http.Request)
	// SendMessage implements .Sender interface
	SendMessage(req *pb.SendMessageRequest) error
	// DeleteProfile
	DeleteProfile() error
	// // String type name e.g.: telegram, viber, facebook
	// String() string
}

type chatBot struct {
	*pbchat.Profile // options
	 ChatBot        // adapter
}

type botService struct {
	log    *zerolog.Logger
	client pbchat.ChatService // webitel.chat.server: client
	router *mux.Router
	urlMap map[string]int64
	bots   map[int64]*chatBot
	exit   chan chan error

	// mx sync.RWMutex
	// indexBot map[int64]*chatbot
	// indexUri map[string]int64
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
		bots:   make(map[int64]*chatBot),
		exit:   make(chan chan error),
	}

	b.router.
		Path("/{url_id}").
		Methods("GET", "POST").
		HandlerFunc(
			b.WebhookFunc,
		)

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
		b.bots[profile.Id] = &chatBot{
			Profile: profile,
			ChatBot: configure(profile, b.client, b.log),
		}
		
		b.log.Info().

			Int64("pid", profile.Id).
			Str("type", profile.Type).
			Str("bot", profile.Name).
			Str("uri", "/"+ profile.UrlId).

		Msg("BOT Register")

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
		if log := b.log.Warn(); log.Enabled() {
			
			log.Err(err).
				Str("addr", srv.Addr().String()).
				Msg("Server [http] Shutted down")
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

	// b.log.Info().
	// 	Msg("removing webhooks")
	// for k := range b.bots {
	// 	if err := b.bots[k].DeleteProfile(); err != nil {
	// 		b.log.Error().Msg(err.Error())
	// 	}
	// 	delete(b.bots, k)
	// }

	return nil
}

func (s *botService) SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error {
	// b.log.Debug().
	// 	Int64("pid", req.GetProfileId()).
	// 	Str("type", req.GetMessage().GetType()).
	// 	Str("to-user", req.GetExternalUserId()).
	// 	Msg("SEND Update")

	msg := req.GetMessage()

	c := s.bots[req.ProfileId]
	
	if c == nil {
		// TODO: try to fetch from persistent store (db)
		s.log.Error().
		
			Int64("pid", req.ProfileId).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
			Str("err", "bot: profile %d not running").
			
		Msg("Failed to send message")

		return errors.NotFound(
			"webitel.chat.bot.not_found",
			"bot: profile %d not running",
			req.ProfileId,
		)
	}

	// perform
	const commandClose = "Conversation closed" // internal: from !
	// NOTE: sending the last conversation message
	closing := msg.GetText() == commandClose
	err := c.SendMessage(req)
	
	if err != nil {
		s.log.Error().Err(err).
	
			Int64("pid", c.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("Failed to send message")
		return err
	}
	
	if closing {
		
		s.log.Warn().
	
			Int64("pid", c.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("SENT Close")
	
	} else {

		s.log.Debug().
		
			Int64("pid", c.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("SENT")
	}

	return err
}

func (b *botService) AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error {

	opts := req.GetProfile()

	init, ok := ConfigureBotsMap[opts.Type]
	if !ok {
		b.log.Warn().
			
			Int64("pid", opts.Id).
			Int64("pdc", opts.DomainId).
			
			Str("uri", "/"+opts.UrlId).
			Str("type", opts.Type).
			Str("name", opts.Name).
			
			Msg("wrong profile type")
		
			return nil
	}

	bot := &chatBot{
		Profile: opts,
		ChatBot: init(opts, b.client, b.log),
	}

	b.bots[bot.Profile.Id] = bot // register: cache entry
	b.urlMap[bot.Profile.UrlId] = bot.Profile.Id // register: service URI
	
	b.log.Info().

		Int64("pid", req.GetProfile().GetId()).
		Int64("pdc", req.GetProfile().GetDomainId()).
		Str("uri", "/"+ req.GetProfile().GetUrlId()).
		Str("type", req.GetProfile().GetType()).
		Str("name", req.GetProfile().GetName()).
		
		Msg("add profile")

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
	
	uri := strings.TrimPrefix(r.URL.Path, "/")
	if uri == "" {
		b.log.Error().Msg("missing /uri")
		http.NotFound(w, r)
		return
	}
	pid, ok := b.urlMap[uri]
	if !ok {
		
		// region: lookup persistant DB; lazy start
		res, err := b.client.GetProfileByID(
			// bind to this request cancelation context
			r.Context(),
			// prepare request parameters
			&pbchat.GetProfileByIDRequest{
				Id:  0,   // unknown
				Uri: uri, // known
			},
			// go-micro client.CallOption(s) ...
		)

		if err != nil {
			http.Error(w, "chat-bot: get profile: "+ err.Error(), http.StatusInternalServerError)
			return
		}

		bot := res.Item
		if bot.GetUrlId() != uri {
			http.Error(w, "chat-bot: profile not found", http.StatusNotFound)
			return
		}
		// endregion

		// region: runtime cache
		err = b.AddProfile(
			// bind cancellation context
			r.Context(),
			// request
			&pb.AddProfileRequest{
				Profile: bot,
			},
			// response
			&pb.AddProfileResponse{
				// results expected
			},
		)

		if err != nil {
			http.Error(w, "chat-bot: "+ err.Error(), http.StatusInternalServerError)
			return
		}
		// resolve profileID
		pid = bot.Id
		// endregion

		// b.log.Error().Msg("profile id not found")
		// return
	}
	b.bots[pid].Handler(w, r)
}

// type Receiver interface {
// 	 RecvUpdate(http.ResponseWriter, *http.Request)
// }

// type Sender interface {
// 	 SendUpdate(context.Context, *pbchat.SendMessageRequest) error
// }

// // chatbot runtime
// type chatbot struct {
// 	// Options
// 	*pbchat.Profile
// 	// Send/Recv interface
// 	 ChatBot
// }



// func (s *botService) getBotByURI(ctx context.Context, uri string) (*chatbot, error) {

// 	s.mx.RLock()
// 	e, ok := s.indexUri[uri]
// 	s.mx.RUnlock()
// }