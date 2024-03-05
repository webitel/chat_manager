package bot

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"net/http/pprof"

	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	audit "github.com/webitel/chat_manager/logger"
)

// Service intercomunnication proxy
type Service struct {
	// cmd/bot.Service
	// Options
	// Public site URL to connect to .this service
	URL string

	// Address to listen HTTP callbacks (webhook) requests
	Addr    string
	WebRoot string

	Log    zerolog.Logger
	Auth   *auth.Client
	Client chat.ChatService
	exit   chan chan error

	// persistent store
	store Store
	// protects the load of the Gateway(s) queries
	loadMx sync.Mutex
	// local cache store
	indexMx  sync.RWMutex
	gateways map[string]int64   // map[URI]profile.id
	profiles map[int64]*Gateway // map[profile.id]gateway
	audit    *audit.Client
}

func NewService(
	store Store,
	logger *zerolog.Logger,
	client chat.ChatService,
	auditClient *audit.Client,
	// router *mux.Router,
) *Service {
	return &Service{

		Log:    *(logger),
		Client: client, // chat.NewChatService("webitel.chat.server"),

		exit: make(chan chan error),

		store: store,

		gateways: make(map[string]int64),
		profiles: make(map[int64]*Gateway),
		audit:    auditClient,
	}
}

func (srv *Service) onStart() {

	srv.Log.Info().Msg("Server [bots] Connecting recipient subscriptions . . .")

	ctx := context.TODO()
	lookup := app.SearchOptions{
		Context: app.Context{
			// Date:    time.Time{},
			// Error:   nil,
			Context: ctx,
			Authorization: auth.Authorization{
				Service: "webitel.chat.bot",
				Method:  "internal",
				Token:   "webitel.chat.bot",
				// Creds: &auth.Userinfo{
				// 	Dc:                0,
				// 	Domain:            "",
				// 	UserId:            0,
				// 	Name:              "",
				// 	Username:          "",
				// 	PreferredUsername: "",
				// 	Extension:         "",
				// 	Scope:             nil,
				// 	Roles:             nil,
				// 	License:           nil,
				// 	Permissions:       nil,
				// 	UpdatedAt:         0,
				// 	ExpiresAt:         0,
				// },
			},
		},
		// ID:   nil,
		// Term: "",
		Filter: map[string]interface{}{
			// LoadAndStart providers which must init their client connections
			"provider": []string{"gotd"},
		},
		// Access: 0,
		Fields: []string{
			"dc", "id",
			"name", "uri",
			"enabled", "flow",
			"updates",
			"provider", "metadata",
			"created_at", "created_by",
			"updated_at", "updated_by",
		},
		// Order:  nil,
		Size: -1,
		// Page:   0,
	}

	bots, err := srv.store.Search(&lookup)
	if err != nil {
		srv.Log.Err(err).Msg("service.onStart")
	}

	for _, profile := range bots {
		// TODO: Check profile.domain license activity; validity boundaries
		pid := profile.GetId()
		gate, err := srv.setup(profile)
		if err != nil {
			continue
		}
		force := false // REGISTER WebHook(!)
		err = gate.Register(ctx, force)
		if err != nil {
			// return nil, err
			srv.Log.Err(err).Int64("pid", pid).Msg("service.onStart.bot.register")
		}
	}
}

// Start background http.Server to listen
// and serve external chat incoming notifications
func (srv *Service) Start() error {

	// Validate TLS Config
	var secure *tls.Config

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
	// Normalize address
	srv.Addr = ln.Addr().String()
	if log := srv.Log.Info(); log.Enabled() {
		log.Msgf("Server [http] Listening on %s", srv.Addr)
	}
	// Normalize Reverse URL
	var hostURL *url.URL
	if srv.URL == "" {
		hostURL = &url.URL{
			Scheme: "http",
			Host:   srv.Addr,
		}
		if secure != nil {
			hostURL.Scheme += "s"
		}

	} else {
		hostURL, err = url.ParseRequestURI(srv.URL)
		if err != nil {
			// Invalid --site-url parameter spec
			_ = ln.Close()
			return err
		}
	}
	srv.URL = hostURL.String()
	if log := srv.Log.Info(); log.Enabled() {
		log.Msgf("Server [http] Reverse Host %s", srv.URL)
	}

	rmux := http.NewServeMux()
	rmux.HandleFunc("/debug/pprof/", pprof.Index)
	rmux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	rmux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	rmux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	rmux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	rmux.Handle("/", srv)

	// handler := srv
	// handler := dumpMiddleware(srv)
	handler := dumpMiddleware(rmux)
	// handler := rmux

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

	}()

	go func() {
		ch := <-srv.exit
		ch <- ln.Close()
	}()

	// TODO: onStartup()
	srv.onStart()

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

	for _, gate := range srv.profiles {
		_ = gate.Remove()
		if re := gate.External.Close(); re != nil {
			gate.Log.Err(re).Msg("STOP")
		}
	}

	return nil
}

func (srv *Service) HostURL() string {
	return srv.URL
}

// Gateway returns profile's runtime gateway instance
// If profile exists but not yet running, performs lazy startup process
func (srv *Service) Gateway(ctx context.Context, pid int64, uri string) (*Gateway, error) {

	srv.loadMx.Lock()
	defer srv.loadMx.Unlock()

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
				"chat.bot.lookup.missing",
				"chat: lookup for a bot without any conditions",
			)
		}

		// lookup: resolve by webhook URI
		srv.indexMx.RLock()     // +R
		pid = srv.gateways[uri] // resolve
		srv.indexMx.RUnlock()   // -R
	}

	if pid != 0 {
		// lookup: running ?
		srv.indexMx.RLock()           // +R
		gate, ok := srv.profiles[pid] // runtime
		srv.indexMx.RUnlock()         // -R

		if ok && gate != nil {
			// CACHE: FOUND !
			return gate, nil
		}
	}

	// region: lookup persistant DB
	lookup := app.SearchOptions{
		Context: app.Context{
			// Date:    time.Time{},
			// Error:   nil,
			Context: ctx,
			Authorization: auth.Authorization{
				Service: "webitel.chat.bot",
				Method:  "internal",
				Token:   "webitel.chat.bot",
				// Creds: &auth.Userinfo{
				// 	Dc:                0,
				// 	Domain:            "",
				// 	UserId:            0,
				// 	Name:              "",
				// 	Username:          "",
				// 	PreferredUsername: "",
				// 	Extension:         "",
				// 	Scope:             nil,
				// 	Roles:             nil,
				// 	License:           nil,
				// 	Permissions:       nil,
				// 	UpdatedAt:         0,
				// 	ExpiresAt:         0,
				// },
			},
		},
		// ID:   nil,
		// Term: "",
		// Filter: map[string]interface{}{
		// 	"": nil,
		// },
		// Access: 0,
		Fields: []string{"+"},
		// Order:  nil,
		Size: 1,
		// Page:   0,
	}

	if pid != 0 {
		lookup.ID = []int64{pid}
	}

	if uri != "" {
		lookup.FilterAND("uri", uri)
	}

	res, err := srv.LocateBot(&lookup)

	// res, err := srv.Client.GetProfileByID(
	// 	// bind to this request cancellation context
	// 	ctx,
	// 	// prepare request parameters
	// 	&chat.GetProfileByIDRequest{
	// 		Id:  pid, // LOOKUP .ID
	// 		Uri: uri, // LOOKUP .URI
	// 	},
	// 	// callOpts ...
	// )

	if err != nil {

		srv.Log.Error().Err(err).
			Int64("pid", pid).
			Msg("Failed lookup bot.profile")

		return nil, err
	}

	profile := res // res.GetItem()

	if profile.GetId() == 0 {
		// NOT FOUND !
		srv.Log.Warn().Int64("pid", pid).Str("uri", uri).
			Msg("PROFILE: NOT FOUND")

		return nil, errors.NotFound(
			"chat.gateway.profile.not_found",
			"gateway: profile {id=%d, uri=%s} not found",
			pid, uri,
		)
	}

	if pid != 0 && pid != profile.GetId() {
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
	if profile.GetUri() == "" {
		return nil, errors.New(
			"chat.gateway.profile.url.missing",
			"gateway: profile URI is missing",
			http.StatusBadGateway,
		)
	}
	// if strings.IndexByte(profile.UrlId, '/') != 0 {
	// 	profile.UrlId = "/" + profile.UrlId
	// }
	if uri != "" && uri != profile.GetUri() {
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

	// if !profile.GetEnabled() {
	// 	// NOT FOUND !
	// 	srv.Log.Warn().
	// 		Int64("pid", profile.GetId()).
	// 		Str("uri", profile.GetUri()).
	// 		Str("error", "chat: bot is disabled").
	// 		Msg("PROFILE: DISABLED")

	// 	return nil, errors.NotFound(
	// 		"chat.bot.channel.disabled",
	// 		"chat: bot is disabled",
	// 	)
	// }
	// endregion

	pid = profile.GetId()
	agent, err := srv.setup(profile)

	if err != nil {
		return nil, err
	}

	force := false // REGISTER WebHook(!)
	err = agent.Register(ctx, force)

	if err != nil {
		return nil, err
	}

	// pid = profile.Id
	// // region: start runtime; add cache
	// err = srv.AddProfile(
	// 	// bind cancellation context
	// 	ctx,
	// 	// request
	// 	&gate.AddProfileRequest{
	// 		Profile: profile,
	// 	},
	// 	// response
	// 	&gate.AddProfileResponse{
	// 		// envelope expected; but we ignore result
	// 	},
	// )

	// if err != nil {
	// 	return nil, err
	// }
	// endregion

	// region: ensure running
	srv.indexMx.RLock()           // +R
	gate, ok := srv.profiles[pid] // runtime
	srv.indexMx.RUnlock()         // -R

	if !ok || gate == nil {
		return nil, fmt.Errorf("Failed startup bot's profile; something went wrong")
	}
	// endregion

	return gate, nil
}

/*func (srv *Service) Gateway(ctx context.Context, pid int64, uri string) (*Gateway, error) {

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
				"chat.bot.lookup.missing",
				"chat: lookup for a bot without any conditions",
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
}*/

var (
	hdrOrigin = http.CanonicalHeaderKey("Origin")
)

// ServeHTTP handler to deal with external chat channel notifications
func (srv *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// TODO: Allow GET or POST Methods ONLY !
	switch r.Method {
	case http.MethodOptions:
		// TODO: Access-Control-Request-Headers: x-xsrf-token, X-Requested-With
		fallthrough
	case http.MethodGet:
		header := w.Header()
		header.Set("Access-Control-Allow-Credentials", "true")
		header.Set("Access-Control-Allow-Methods", "OPTIONS, GET, POST")
		header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Webitel-Access, Cookie, "+
			"Connection, Upgrade, Sec-Websocket-Version, Sec-Websocket-Extensions, Sec-Websocket-Key, Sec-Websocket-Protocol, "+
			"X-XSRF-Token, "+ // Axios frontend
			"X-Requested-With",
		)
		origin := r.Header.Get(hdrOrigin)
		if origin == "" {
			origin = "*"
		}
		header.Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return // 200 OK
		}
		// File exists ? static asset !
		name := filepath.Join(srv.WebRoot, r.URL.Path)
		if file, re := os.Stat(name); re == nil && !file.IsDir() {
			http.ServeFile(w, r, name)
			return
		}
	case http.MethodPost:
		// Receive Update Event !
	default:
		http.Error(w, "(405) Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	srv.Log.Debug().
		Str("uri", r.URL.Path).
		Str("method", r.Method).
		Msg("<<<<< WEBHOOK <<<<<")

	uri := r.URL.Path // strings.TrimLeft(r.URL.Path, "/")

	if "/favicon.ico" == uri {
		http.NotFound(w, r)
		return
	}

	gate, err := srv.Gateway(r.Context(), 0, uri)

	if err != nil {
		re := errors.FromError(err)
		switch re.Code {
		case 404: // NOT FOUND
			// Failed to lookup profile by URI
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			// Failed to lookup profile by URI
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
		return
	}

	// // Inspect known URIs index
	// srv.indexMx.RLock()   // +R
	// pid, ok := srv.gateways[uri]
	// srv.indexMx.RUnlock() // -R

	// // Discover -IF- no runtime found !
	// if !ok {

	// 	bot, err := srv.LocateBot(
	// 		&app.SearchOptions{
	// 			// TODO: Service Authentication !
	// 			Context: app.Context{
	// 				Context: r.Context(),
	// 			},
	// 			Filter: map[string]interface{}{
	// 				"uri": uri, // TODO: !!!
	// 			},
	// 			Fields: []string{"+"},
	// 			Size: 1,
	// 		},
	// 	)

	// 	if err != nil {
	// 		re := errors.FromError(err)
	// 		switch re.Code {
	// 		case 404: // NOT FOUND
	// 			// Failed to lookup profile by URI
	// 			http.Error(w, err.Error(), http.StatusNotFound)
	// 		default:
	// 			// Failed to lookup profile by URI
	// 			http.Error(w, err.Error(), http.StatusBadGateway)
	// 		}
	// 		return
	// 	}

	// 	pid = bot.GetId()
	// 	if pid == 0 || bot.GetUri() != uri {
	// 		// Profile NOT FOUND !
	// 		http.NotFound(w, r)
	// 		return
	// 	}

	// 	if !bot.GetEnabled() {
	// 		// ERROR: BOT is disabled !
	// 		http.Error(w,
	// 			"chat: bot is disabled", // 503
	// 			 http.StatusServiceUnavailable,
	// 		)
	// 		return
	// 	}

	// 	// SETUP
	// 	gate, err := srv.setup(bot)

	// 	if err != nil {
	// 		// ERROR: Profile misconfigured !
	// 		http.Error(w, err.Error(), http.StatusBadGateway)
	// 		return
	// 	}

	// 	// NOTE: bot.Enabled IS !
	// 	force := false
	// 	// Definitely, do not RE-Register callback URI
	// 	// Because we already receiving notification
	// 	// so I guess provider knows which URI to contact
	// 	err = gate.Register(r.Context(), force)

	// 	if err != nil {
	// 		// ERROR: Provider register error !
	// 		http.Error(w, err.Error(), http.StatusBadGateway)
	// 		return
	// 	}

	// 	// // FIXME: omit profile.Register(!) operation
	// 	// err = srv.AddProfile(
	// 	// 	r.Context(),
	// 	// 	&gate.AddProfileRequest{
	// 	// 		Profile: profile,
	// 	// 	},
	// 	// 	&gate.AddProfileResponse{},
	// 	// )

	// 	// if err != nil {
	// 	// 	re := errors.FromError(err)
	// 	// 	http.Error(w, re.Detail, (int)(re.Code))
	// 	// 	return
	// 	// }
	// }

	// // ensure running !
	// srv.indexMx.RLock()   // +R
	// gate, ok := srv.profiles[pid]
	// srv.indexMx.RUnlock() // -R

	// if !ok || gate == nil {
	// 	http.NotFound(w, r)
	// 	return
	// }

	if !gate.GetEnabled() {

		// NOTE: If we got disabled cache - guess
		// bot still has active channels, need to be gracefully closed !
		// So pass thru this request. We will deal with NEW channel later ...

		// // ERROR: BOT is disabled !
		// http.Error(w,
		// 	"chat: bot is disabled", // 503
		// 	 http.StatusServiceUnavailable,
		// )
		// return
	}

	// Invoke bot's gateway callback handler !
	gate.WebHook(w, r)

	return // 200
}
