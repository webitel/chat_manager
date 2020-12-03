package main

import (
	"fmt"

	"net"
	"sync"
	"context"
	"strings"
	"net/http"

	"github.com/micro/go-micro/v2/errors"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"

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
	exit   chan chan error

	index sync.RWMutex       // index: RW locker
	hooks map[string]int64   // index: map[URI]PID
	gates map[int64]*chatBot // index: map[PID]BOT

	// mx sync.RWMutex
	// indexBot map[int64]*chatbot
	// indexUri map[string]int64
}

func NewBotService(
	log *zerolog.Logger,
	client pbchat.ChatService,
	router *mux.Router,
) *botService {

	s := &botService{

		log:    log,
		client: client,
		router: router,
		
		hooks: make(map[string]int64),
		gates: make(map[int64]*chatBot),
		exit:  make(chan chan error),
	}

	s.router.
		PathPrefix("/").
		Methods("POST", "GET").
		HandlerFunc(
			s.WebhookFunc,
		)

	// TODO:
	// - fetch .Registry.GetService(service.Options().Name)
	// - lookup DB for profiles, NOT in registered service nodes list; hosted!
	list, err := s.client.GetProfiles(
		context.TODO(),
		&pbchat.GetProfilesRequest{Size: 100},
	)

	if err != nil || list == nil {
		s.log.Fatal().Msg(err.Error())
		return nil
	}

	var (

		register pb.AddProfileRequest
		response pb.AddProfileResponse
	)

	for _, profile := range list.Items {
		
		register.Profile = profile
		
		_ = s.AddProfile(context.TODO(), &register, &response)
		
		
		// s.urlMap[profile.UrlId] = profile.Id
		// configure, ok := ConfigureBotsMap[profile.Type]
		// if !ok {
		// 	b.log.Warn().
		// 		Int64("id", profile.Id).
		// 		Str("type", profile.Type).
		// 		Str("name", profile.Name).
		// 		Int64("domain_id", profile.DomainId).
		// 		Msg("wrong profile type")
		// 	continue
		// }
		// s.bots[profile.Id] = &chatBot{
		// 	Profile: profile,
		// 	ChatBot: configure(profile, s.client, s.log),
		// }
		
		// s.log.Info().

		// 	Int64("pid", profile.Id).
		// 	Str("type", profile.Type).
		// 	Str("bot", profile.Name).
		// 	Str("uri", "/"+ profile.UrlId).

		// Msg("BOT Register")

	}

	return s
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

// gateway returns BOT profile runtime gateway instance
// if runtime not found, performs lazy startup preparations
func (s *botService) gateway(ctx context.Context, pid int64, uri string) (*chatBot, error) {

	if pid == 0 {

		if uri == "" {
			return nil, fmt.Errorf("gateway: lookup.id missing")
		}

		// lookup: resolve by webhook URI
		s.index.RLock()    // +R
		pid = s.hooks[uri] // resolve
		s.index.RUnlock()  // -R
	}

	if pid != 0 {
		// lookup: running ?
		s.index.RLock()   // +R
		gate, ok := s.gates[pid] // runtime
		s.index.RUnlock() // -R

		if ok && gate != nil {
			// CACHE: FOUND !
			return gate, nil
		}
	}

	// region: lookup persistant DB
	res, err := s.client.GetProfileByID(
		// bind to this request cancellation context
		ctx,
		// prepare request parameters
		&pbchat.GetProfileByIDRequest{
			Id:  pid, // LOOKUP .ID
			Uri: uri, // LOOKUP .URI
		},
		// callOpts ...
	)

	if err != nil {

		s.log.Error().Err(err).
			Int64("pid", pid).
			Msg("Failed lookup bot.profile")

		return nil, err
	}

	profile := res.GetItem()
	
	if profile == nil || profile.Id == 0 {
		// NOT FOUND !
		s.log.Warn().Int64("pid", pid).Str("uri", uri).
			Msg("PROFILE: NOT FOUND")
		
		return nil, errors.NotFound(
			"chat.bot.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			 pid, uri,
		)
	}

	if pid != 0 && pid != profile.Id {
		// NOT FOUND !
		s.log.Warn().Int64("pid", pid).
			Str("error", "mismatch profile.id requested").
			Msg("PROFILE: NOT FOUND")
		
		return nil, errors.NotFound(
			"chat.bot.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			 pid, uri,
		)
	}

	if uri != "" && uri != profile.UrlId {
		// NOT FOUND !
		s.log.Warn().Str("uri", uri).
			Str("error", "mismatch profile.uri requested").
			Msg("PROFILE: NOT FOUND")
		
		return nil, errors.NotFound(
			"chat.bot.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			 pid, uri,
		)
	}
	// endregion

	pid = profile.Id
	// region: start runtime; add cache
	err = s.AddProfile(
		// bind cancellation context
		ctx,
		// request
		&pb.AddProfileRequest{
			Profile: profile,
		},
		// response
		&pb.AddProfileResponse{
			// envelope expected; but we ignore result
		},
	)

	if err != nil {
		return nil, err
	}
	// endregion

	// region: ensure running
	s.index.RLock()   // +R
	gate, ok := s.gates[pid] // runtime
	s.index.RUnlock() // -R

	if !ok || gate == nil {
		return nil, fmt.Errorf("Failed startup profile gateway; something went wrong")
	}
	// endregion

	return gate, nil
}

func (s *botService) SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error {
	// b.log.Debug().
	// 	Int64("pid", req.GetProfileId()).
	// 	Str("type", req.GetMessage().GetType()).
	// 	Str("to-user", req.GetExternalUserId()).
	// 	Msg("SEND Update")

	pid := req.GetProfileId(); if pid == 0 {
		return errors.BadRequest(
			"chat.bot.send.profile_id.required",
			"gateway: send.profile_id required but missing",
		)
	}

	msg := req.GetMessage(); if msg == nil {
		return errors.BadRequest(
			"chat.bot.send.message.required",
			"gateway: send.message required but missing",
		)
	}

	gate, err := s.gateway(ctx, pid, "")

	if err != nil {
		return err
	}

	// perform
	const commandClose = "Conversation closed" // internal: from !
	// NOTE: sending the last conversation message
	closing := msg.GetText() == commandClose
	err = gate.SendMessage(req)
	
	if err != nil {
		
		s.log.Error().Err(err).
	
			Int64("pid", gate.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("Failed to send message")
		return err
	}
	
	if closing {
		
		s.log.Warn().
	
			Int64("pid", gate.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("SENT Close")
	
	} else {

		s.log.Debug().
		
			Int64("pid", gate.Profile.Id).
			Str("type", msg.GetType()).
			Str("chat-id", req.GetExternalUserId()).
			Str("text", msg.GetText()).
		
		Msg("SENT")
	}

	return err
}

func (s *botService) AddProfile(ctx context.Context, req *pb.AddProfileRequest, res *pb.AddProfileResponse) error {

	add := req.GetProfile()

	init, ok := ConfigureBotsMap[add.Type]
	
	if !ok || init == nil {
		
		s.log.Warn().
			
			Int64("pid", add.Id).
			Int64("pdc", add.DomainId).
			
			Str("uri", "/"+ add.UrlId).
			Str("type", add.Type).
			Str("name", add.Name).
			
			Msg("NOT SUPPORTED")
		
			return nil
	}

	srv := &chatBot{
		Profile: add,
		ChatBot: init(add, s.client, s.log),
	}

	pid := add.GetId()
	uri := add.GetUrlId()

	s.index.Lock()     // +RW
	s.gates[pid] = srv // register: cache entry
	s.hooks[uri] = pid // register: service URI
	s.index.Unlock()   // -RW
	
	s.log.Info().

		Int64("pid", srv.Profile.Id).
		Int64("pdc", srv.Profile.DomainId).
		
		Str("uri", "/" + srv.Profile.UrlId).
		Str("type", srv.Profile.Type).
		Str("name", srv.Profile.Name).

		Msg("REGISTER")

	return nil
}

// FIXME: performs DEREGISTER only ?
func (s *botService) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error {
	
	gate, err := s.gateway(ctx, req.GetId(), req.GetUrlId())
	
	if err != nil {
		return err
	}

	// DEREGISTER Webhook (!)
	err = gate.DeleteProfile()

	var event *zerolog.Event

	if err == nil {

		event = s.log.Info()

	} else {

		event = s.log.Error().Err(err)
	}

	event.

		Int64("pid", gate.Profile.Id).
		Int64("pdc", gate.Profile.DomainId).
		
		Str("uri", "/" + gate.Profile.UrlId).
		Str("type", gate.Profile.Type).
		Str("name", gate.Profile.Name).

		Msg("DEREGISTER Webhook")

	return err
	
	// b.log.Info().
	// 	Int64("pid", req.GetId()).
	// 	Msg("DEREGISTER")
	// delete(b.urlMap, req.UrlId)
	// if err := b.bots[req.Id].DeleteProfile(); err != nil {
	// 	b.log.Error().Msg(err.Error())
	// 	return err
	// }
	// return nil
}

// Face the gateway's hook result status.
// Mostly, do nothing, except that you will see failure status log events
type gatewayResponseWriter struct {
	http.ResponseWriter
	event zerolog.Logger
}

func (w *gatewayResponseWriter) WriteHeader(code int) {

	// switch {
	// case 200 <= code && code < 300: // 2XX - 
	// case 300 <= code && code < 400: // 3XX - 
	// case 400 <= code && code < 500: // 4XX - 
	// case 500 <= code && code < 600: // 5XX - 
	// default:
	// }

	if 0 <= code && code < 400 {
		w.event.Debug().Int("code", code).Str("status", http.StatusText(code)).Msg("WEBHOOK")
	} else {
		w.event.Error().Int("code", code).Str("status", http.StatusText(code)).Msg("WEBHOOK")
	}

	w.ResponseWriter.WriteHeader(code)
}

func (s *botService) WebhookFunc(res http.ResponseWriter, req *http.Request) {
	
	URI := strings.TrimPrefix(req.URL.Path, "/")
	
	if URI == "" {
		http.NotFound(res, req) // 404
		s.log.Error().Str("uri", "/").Int("code", 404).Str("status", "Not Found").Msg("WEBHOOK")
		return
	}

	gate, err := s.gateway(req.Context(), 0, URI)

	if err != nil {

		// re := errors.FromError(err)
		// switch re.Code {
		// // case 400 <= re.Code && re.Code < 500: // 4XX - Client Error
		// // case 500 <= re.Code && re.Code < 600: // 5XX - Server Error
		// case 404:
		// 	http.Error(res, err.Error(), http.StatusNotFound) // 404
		// default:
		// 	code := http.StatusBadGetaway // 502
		// 	if re.Code != 
		// }

		code := http.StatusBadGateway // 502
		http.Error(res, err.Error(), code)
		s.log.Error().Err(err).Str("uri", "/" + URI).Int("code", code).Str("status", http.StatusText(code)).Msg("WEBHOOK")
		return
	}

	res = &gatewayResponseWriter{
		
		ResponseWriter: res,
		event: s.log.With().

		Str("uri", "/" + URI).
		Int64("pid", gate.Profile.Id).
		Int64("pdc", gate.Profile.DomainId).

		Str("type", gate.Profile.Type).
		Str("name", gate.Profile.Name).

		Logger(),
	}

	// PASSTHRU
	gate.Handler(res, req)

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