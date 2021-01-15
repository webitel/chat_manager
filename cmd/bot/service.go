package main

import (
	
	"net"
	// "net/url"
	
	"fmt"
	"context"
	"strings"

	"sync"
	"net/http"

	"github.com/rs/zerolog"
	// errs "github.com/pkg/errors"
	"github.com/micro/go-micro/v2/errors"
	
	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
)

// Service to communicate
// with external chat providers
type Service struct {

	// Public site URL to connect to .this service
	URL      string

	// Address to listen HTTP callbacks (webhook) requests
	Addr     string

	Log      zerolog.Logger
	Client   chat.ChatService
	exit     chan chan error

	indexMx  sync.RWMutex
	gateways map[string]int64 // map[URI]profile.id
	profiles map[int64]*Gateway // map[profile.id]gateway
}


func NewService(
	log *zerolog.Logger,
	client chat.ChatService,
	// router *mux.Router,
) *Service {

	return &Service{
		
		Log: *(log),
		Client: client, // chat.NewChatService("webitel.chat.server"),
		exit: make(chan chan error),

		gateways: make(map[string]int64),
		profiles: make(map[int64]*Gateway),
	}
}

// Start background http.Server to listen
// and serve external chat incoming notifications
func (srv *Service) Start() error {

	// region: force RE-REGISTER all profiles
	// // - fetch .Registry.GetService(service.Options().Name)
	// // - lookup DB for profiles, NOT in registered service nodes list; hosted!
	// list, err := srv.Client.GetProfiles(
	// 	context.TODO(), &chat.GetProfilesRequest{Size: 100},
	// )

	// if err != nil || list == nil {
	// 	srv.Log.Fatal().Err(err).Msg("Failed to list gateway profiles")
	// 	return nil
	// }

	// var (

	// 	register gate.AddProfileRequest
	// 	response gate.AddProfileResponse
	// )

	// for _, profile := range list.Items {

	// 	if !strings.HasPrefix(profile.UrlId, "chat/") {
	// 		continue
	// 	}
		
	// 	register.Profile = profile
	// 	_ = srv.AddProfile(context.TODO(), &register, &response)
	// }
	// endregion

	ln, err := net.Listen("tcp", srv.Addr)

	if err != nil {
		return err
	}

	if log := srv.Log.Info(); log.Enabled() {
		log.Msgf("Server [http] Listening on %s", ln.Addr().String())
	}

	// handler := srv
	handler := dumpMiddleware(srv)

	go func() {
		
		if err := http.Serve(ln, handler); err != nil {
			// if err != http.ErrServerClosed {
			// 	if log := b.log.Error(); log.Enabled() {
			// 		log.Err(err).Msg("Server [http] Shutted down due to an error")
			// 	}
			// 	return
			// }
		}

		if log := srv.Log.Warn(); log.Enabled() {
			
			log.Err(err).
				Str("addr", ln.Addr().String()).
				Msg("Server [http] Shutted down")
		}

	} ()

	go func() {
		ch := <-srv.exit
		ch <- ln.Close()
	}()

	return nil
}

// Close background http.Server
func (srv *Service) Close() error {

	ch := make(chan error)
	srv.exit <- ch
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

// ServeHTTP handler to deal with external chat channel notifications
func (srv *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	srv.Log.Debug().
		Str("uri", r.URL.Path).
		Str("method", r.Method).
		Msg("<<<<< WEBHOOK <<<<<")

	uri := strings.TrimLeft(r.URL.Path, "/")

	srv.indexMx.RLock()   // +R
	pid, ok := srv.gateways[uri]
	srv.indexMx.RUnlock() // -R

	if !ok {

		res, err := srv.Client.GetProfileByID(
			
			r.Context(),
			&chat.GetProfileByIDRequest{
				Uri: uri,
			},
		)

		if err != nil {
			// Failed to lookup profile by URI
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		profile := res.GetItem()
		if profile == nil || profile.UrlId != uri {
			// Profile NOT FOUND !
			http.NotFound(w, r)
			return
		}
		// STARTUP !
		pid = profile.GetId()
		// FIXME: omit profile.Register(!) operation
		err = srv.AddProfile(
			r.Context(),
			&gate.AddProfileRequest{
				Profile: profile,
			},
			&gate.AddProfileResponse{},
		)

		if err != nil {
			re := errors.FromError(err)
			http.Error(w, re.Detail, (int)(re.Code))
			return
		}

		// provider := GetProvider(profile.Type)
		// if provider == nil {
		// 	http.Error(w, "gateway: provider "+ profile.Type +" not implemented", http.StatusNotImplemented)
		// 	return
		// }

		// logs := srv.Log.With().
		// 	Int64("pid", profile.Id).
		// 	Str("gate", profile.Type).
		// 	Str("uri", "/"+ profile.UrlId).
		// 	Logger()

		// gateway := &Gateway{

		// 	Log:     &logs,
		// 	Profile:  profile,
		// 	Internal: srv,
		// 	External: nil, // TOBE: init(!)
		// 	// RWMutex:  sync.RWMutex{},
		// 	internal: make(map[int64]*Channel),
		// 	external: make(map[string]*Channel),
		// }

		// gateway.External = provider(gateway)

		// srv.indexMx.Lock()   // +RW
		// srv.gateways[uri] = gateway.Profile.Id
		// srv.profiles[gateway.Profile.Id] = gateway
		// srv.indexMx.Unlock() // -RW
	}

	// ensure running !
	srv.indexMx.RLock()   // +R
	gateway, ok := srv.profiles[pid]
	srv.indexMx.RUnlock() // -R

	if !ok {
		http.NotFound(w, r)
		return
	}

	gateway.WebHook(w, r)

	return // 200
}

// Gateway returns profile's runtime gateway instance
// If profile exists but not yet running, performs lazy startup process
func (srv *Service) Gateway(ctx context.Context, pid int64, uri string) (*Gateway, error) {

	// if uri != "" {
	// 	// make relative !
	// 	// if !strings.IndexByte(uri, '/') != 0 {
	// 	// 	uri = "/" + uri
	// 	// }
	// 	link, err := url.Parse(uri)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	if link.IsAbs() && !strings.HasPrefix(uri, srv.URL) {
	// 		return nil, errors.NotFound(
	// 			"chat.gateway.url.not_found",
	// 			"gateway: url %s not found",
	// 			 uri,
	// 		)
	// 	}
	// 	uri = link.String() // renormalized !
	// }
	
	if pid == 0 {

		if uri == "" {
			return nil, errors.BadRequest(
				"chat.gateway.lookup.missing",
				"gateway: lookup .id and .uri missing",
			)
		}

		// lookup: resolve by webhook URI
		srv.indexMx.RLock()    // +R
		pid = srv.gateways[uri] // resolve
		srv.indexMx.RUnlock()  // -R
	}

	if pid != 0 {
		// lookup: running ?
		srv.indexMx.RLock()   // +R
		gate, ok := srv.profiles[pid] // runtime
		srv.indexMx.RUnlock() // -R

		if ok && gate != nil {
			// CACHE: FOUND !
			return gate, nil
		}
	}

	// region: lookup persistant DB
	res, err := srv.Client.GetProfileByID(
		// bind to this request cancellation context
		ctx,
		// prepare request parameters
		&chat.GetProfileByIDRequest{
			Id:  pid, // LOOKUP .ID
			Uri: uri, // LOOKUP .URI
		},
		// callOpts ...
	)

	if err != nil {

		srv.Log.Error().Err(err).
			Int64("pid", pid).
			Msg("Failed lookup bot.profile")

		return nil, err
	}

	profile := res.GetItem()
	
	if profile == nil || profile.Id == 0 {
		// NOT FOUND !
		srv.Log.Warn().Int64("pid", pid).Str("uri", uri).
			Msg("PROFILE: NOT FOUND")
		
		return nil, errors.NotFound(
			"chat.gateway.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			 pid, uri,
		)
	}

	if pid != 0 && pid != profile.Id {
		// NOT FOUND !
		srv.Log.Warn().Int64("pid", pid).
			Str("error", "mismatch profile.id requested").
			Msg("PROFILE: NOT FOUND")
		
		return nil, errors.NotFound(
			"chat.bot.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			 pid, uri,
		)
	}

	// validate: relative URI !
	if profile.UrlId == "" {
		return nil, errors.New(
			"chat.gateway.profile.url.missing",
			"gateway: profile URI is missing",
			http.StatusBadGateway,
		)
	}
	// if strings.IndexByte(profile.UrlId, '/') != 0 {
	// 	profile.UrlId = "/" + profile.UrlId
	// }
	if uri != "" && uri != profile.UrlId {
		// NOT FOUND !
		srv.Log.Warn().Str("uri", uri).
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
	err = srv.AddProfile(
		// bind cancellation context
		ctx,
		// request
		&gate.AddProfileRequest{
			Profile: profile,
		},
		// response
		&gate.AddProfileResponse{
			// envelope expected; but we ignore result
		},
	)

	if err != nil {
		return nil, err
	}
	// endregion

	// region: ensure running
	srv.indexMx.RLock()   // +R
	gate, ok := srv.profiles[pid] // runtime
	srv.indexMx.RUnlock() // -R

	if !ok || gate == nil {
		return nil, fmt.Errorf("Failed startup profile gateway; something went wrong")
	}
	// endregion

	return gate, nil
}

// implements ...
var _ gate.BotServiceHandler = (*Service)(nil)

// SendMessage to external chat end-user (contact) side
func (srv *Service) SendMessage(ctx context.Context, req *gate.SendMessageRequest, rsp *gate.SendMessageResponse) error {
	
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

	// FIXME: guess here context chaining passthru original
	//        Micro-From-Service: webitel.chat.server
	//        Micro-From-Id: xxxxxxxx-xxxx-xxxx-xxxxxxxxxxxx
	c, err := srv.Gateway(ctx, pid, "")

	if err != nil {
		return err
	}

	// perform
	err = c.Send(ctx, req)
	
	if err != nil {
		
		// srv.Log.Error().Err(err).
	
		// 	Int64("pid", gate.Profile.Id).
		// 	Str("type", msg.GetType()).
		// 	Str("chat-id", req.GetExternalUserId()).
		// 	Str("text", msg.GetText()).
		
		// Msg("Failed to send message")
		return err
	}

	// sentBinding := req.GetMessage().GetVariables()
	// if sentBinding != nil {
	// 	delete(sentBinding, "")
	// 	if len(sentBinding) != 0 {
	// 		// populate SENT message external bindings
	// 		rsp.Bindings = sentBinding
	// 	}
	// }
	// // +OK
	return nil
	
	// if closing {
		
	// 	srv.Log.Warn().
	
	// 		Int64("pid", gate.Profile.Id).
	// 		Str("type", msg.GetType()).
	// 		Str("chat-id", req.GetExternalUserId()).
	// 		Str("text", msg.GetText()).
		
	// 	Msg("SENT Close")
	
	// } else {

	// 	srv.Log.Debug().
		
	// 		Int64("pid", gate.Profile.Id).
	// 		Str("type", msg.GetType()).
	// 		Str("chat-id", req.GetExternalUserId()).
	// 		Str("text", msg.GetText()).
		
	// 	Msg("SENT")
	// }

	// return err
	
	// panic("not implemented") // TODO: Implement
}

// AddProfile register new profile gateway
func (srv *Service) AddProfile(ctx context.Context, req *gate.AddProfileRequest, res *gate.AddProfileResponse) error {


	add := req.GetProfile()

	// region: validate profile
	if add == nil {
		return errors.BadRequest(
			"chat.gateway.add.profile.required",
			"gateway: profile to add is missing",
		)
	}
	if add.Id == 0 {
		return errors.BadRequest(
			"chat.gateway.add.profile.id.required",
			"gateway: add profile.id is missing",
		)
	}
	if add.Type == "" {
		return errors.BadRequest(
			"chat.gateway.add.profile.type.required",
			"gateway: add profile.type is missing",
		)
	}
	if add.DomainId == 0 {
		return errors.BadRequest(
			"chat.gateway.add.profile.domain.required",
			"gateway: add profile.domain_id is missing",
		)
	}
	if add.SchemaId == 0 {
		return errors.BadRequest(
			"chat.gateway.add.profile.schema.required",
			"gateway: add profile.schema_id is missing",
		)
	}
	if add.UrlId == "" {
		return errors.BadRequest(
			"chat.gateway.add.profile.url.required",
			"gateway: add profile.url is missing",
		)
	}
	// endregion

	log := srv.Log.With().

		Int64("pid", add.Id).
		Int64("pdc", add.DomainId).
		Int64("bot", add.SchemaId).
		
		Str("uri", "/" + add.UrlId).
		
		Str("title", add.Name).
		Str("channel", add.Type).

		Logger()

	// Find provider by code name
	start := GetProvider(add.Type)

	if start == nil {
		
		log.Warn().Msg("NOT SUPPORTED")
		
		return errors.New(
			"chat.gateway.provider.not_supported",
			"gateway: provider "+ add.Type +" not supported",
			 http.StatusNotImplemented,
		)
	}

	agent := &Gateway{

		Log:     &log,
		Profile:  add,
		Internal: srv,

		internal: make(map[int64]*Channel), // map[internal.user.id]
	 	external: make(map[string]*Channel), // map[provider.user.id]
	}

	var err error
	
	agent.External, err = start(agent)

	if err != nil {
		
		agent.External = nil
		re := errors.FromError(err)
		
		if re.Code == 0 {
			// NOTE: is NOT err.(*errors.Error)
			code := http.StatusInternalServerError
			re.Id = "chat.gateway."+ add.Type +".start.error"
			// re.Detail = err.Error()
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}

		log.Error().Str("error", re.Detail).Msg("STARTUP")
		
		return re
	}

	force := true // REGISTER WebHook(!)
	err = agent.Register(ctx, force)
	
	if err != nil {

		re := errors.FromError(err)

		if re.Code == 0 {
			// NOTE: is NOT err.(*errors.Error)
			code := http.StatusBadGateway
			re.Id = "chat.gateway."+ add.Type +".register.error"
			// re.Detail = err.Error()
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}

		log.Error().Str("error", re.Detail).Msg("REGISTER")

		return re
	}

	return nil
}

// DeleteProfile deregister profile gateway
func (srv *Service) DeleteProfile(ctx context.Context, req *gate.DeleteProfileRequest, res *gate.DeleteProfileResponse) error {
	
	pid := req.GetId()
	uri := req.GetUrlId()
	
	gate, err := srv.Gateway(ctx, pid, uri)
	
	if err != nil {
		return err
	}

	pid = gate.Profile.Id
	// uri = gate.Profile.UrlId

	// DEREGISTER Webhook (!)
	err = gate.Deregister(ctx)

	if err != nil {
		return err
	}

	// REMOVE FROM CACHE (!)
	if !gate.Remove() {
		return errors.BadRequest(
			"chat.gateway.not_running",
			"gateway: profile id=%d not running",
			 pid,
		)
	}

	return nil
}



