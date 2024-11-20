package bot

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/bot"
)

type (
	Bot         = bot.Bot
	Refer       = bot.Refer
	ChatUpdates = bot.ChatUpdates
)

func IsNew(e *Bot) bool {
	return 0 == e.GetId()
}

func Validate(e *Bot) error {

	// if add.GetDc().GetId() == 0 {
	// 	return errors.BadRequest(
	// 		"chat.gateway.add.profile.domain.required",
	// 		"gateway: add profile.domain_id is missing",
	// 	)
	// }

	rawURI := e.GetUri()
	if rawURI == "" {
		return errors.BadRequest(
			"chat.bot.uri.required",
			"chatbot: service relative URI required",
		)
	}

	if strings.HasPrefix(rawURI, "///") {
		// FIXME: that is local file path ?
		// e.g. valid url without //[host]/ component ?
	}

	if !strings.HasPrefix(rawURI, "/") {
		// FIXME: Force rooted !
		rawURI = "/" + rawURI
	}

	// Try to parse as relative URI
	botURI, err := url.Parse(rawURI)

	if err != nil {
		return errors.BadRequest(
			"chat.bot.uri.invalid",
			"chatbot: "+err.Error(),
		)
	}

	for _, check := range []struct {
		name  string
		valid bool
		error string
	}{
		{"relative", (!botURI.IsAbs() && botURI.Host == ""), "expect relative URI, not absolute"},
		{"hostport", (botURI.User == nil), "relative URI must not include :authority component"},
		{"query", (!botURI.ForceQuery && botURI.RawQuery == ""), "relative URI must not include ?query component"},
		{"fragment", (botURI.Fragment == ""), "relative URI must not include #fragment component"},
	} {
		if !check.valid {
			return errors.BadRequest(
				"chat.bot.uri.invalid",
				"chatbot: "+check.error,
			)
		}
	}
	// Normalize: escape path
	e.Uri = botURI.String()

	if e.GetEnabled() && e.GetFlow().GetId() == 0 {
		return errors.BadRequest(
			"chat.bot.flow.required",
			"chatbot: flow schema required to be enabled",
		)
	}

	if provider := e.GetProvider(); provider == "" {
		return errors.BadRequest(
			"chat.bot.provider.required",
			"chatbot: underlying provider required",
		)
	} else if GetProvider(provider) == nil {
		return errors.BadRequest(
			"chat.bot.provider.invalid",
			"chatbot: provider %s not supported",
			provider,
		)
	}

	return nil
}

// Setup validates and configures the gateway
// driver according to this bot's profile settings
func (srv *Service) setup(add *Bot) (*Gateway, error) {

	// Model validation(s) !
	err := Validate(add)

	if err != nil {
		return nil, err
	}

	log := srv.Log.With(
		slog.Int64("pid", add.GetId()),
		slog.Int64("pdc", add.GetDc().GetId()),
		slog.Int64("bot", add.GetFlow().GetId()),
		slog.String("uri", add.GetUri()),
		slog.String("title", add.GetName()),
		slog.String("channel", add.GetProvider()),
	)

	// Find provider implementation by code name
	setup := GetProvider(add.GetProvider())

	if setup == nil {
		log.Warn("PROVIDER: NOT SUPPORTED")
		// Client Request Error !
		return nil, errors.BadRequest(
			"chat.bot.provider.invalid",
			"chatbot: invalid %s provider; not implemented",
			add.Provider,
		)
	}

	srv.indexMx.Lock() // -RW
	run, ok := srv.profiles[add.GetId()]
	srv.indexMx.Unlock() // +RW

	// CHECK: Provider specific options are well formed !
	agent := &Gateway{

		Log:      log,
		Bot:      add,
		Internal: srv,
		// Template: NewTemplate(add.Provider),
		// // CACHE Store
		// RWMutex:  new(sync.RWMutex),
		// internal: make(map[int64]*Channel), // map[internal.user.id]
		// external: make(map[string]*Channel), // map[provider.user.id]
	}

	// NOTE: We oblige providers to independently manage templates for updates.
	// For some of them, support is not yet implemented, some allow the use of auxiliary functions, etc ...

	// tmpls := add.GetUpdates()
	// // Compile .Updates template(s); if provided
	// err = agent.Template.FromProto(tmpls)
	// if err != nil {
	// 	log.Err(err).Msg("TMPL")
	// 	return nil, err
	// }

	var state Provider
	if ok && run != nil {
		// share the load of the channel(s) mutex
		agent.loadMx = run.loadMx
		// share internal:cache store state
		run.Lock() // +RW
		agent.RWMutex = run.RWMutex
		agent.internal = run.internal
		agent.external = run.external
		run.Unlock() // -RW
		state = run.External
	} else {
		// mutex to protect the load of the channel(s)
		agent.loadMx = new(sync.Mutex)
		// NEW CACHE Store init
		agent.RWMutex = new(sync.RWMutex)
		agent.internal = make(map[int64]*Channel)  // map[internal.user.id]
		agent.external = make(map[string]*Channel) // map[provider.user.id]
	}

	// PERFORM ChatBot provider's driver setup
	agent.External, err = setup(agent, state)

	if err != nil {

		agent.External = nil // NULL -ify
		re := errors.FromError(err)

		if re.Code == 0 {
			// NOTE: is NOT err.(*errors.Error)
			code := http.StatusBadRequest // FIXME: 400 ?
			re.Id = "chat.bot." + add.Provider + ".setup.error"
			// re.Detail = err.Error()
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}

		log.Error("SETUP",
			slog.String("error", re.Detail),
		)

		return nil, re
	}
	// Template: default empty
	if agent.Template == nil {
		agent.Template = NewTemplate(
			agent.Bot.Provider,
		)
		// CHECH: User specified ? Not empty ?
		spec := agent.Bot.Updates
		if is := (spec != nil); is {
			for _, tmpl := range []string{
				spec.Title, spec.Close,
				spec.Join, spec.Left,
			} {
				tmpl = strings.TrimSpace(tmpl)
				if is = (tmpl != ""); is {

					re := errors.BadRequest(
						"chat.bot."+add.Provider+".updates.not_implemented",
						"updates: this provider does not implement templates yet",
					).(*errors.Error)

					log.Error("SETUP",
						slog.String("error", re.Detail),
					)
					return nil, re
				}
			}
		}
	}

	// if !add.GetEnabled() {
	// 	return agent, nil
	// }

	// err = agent.Register(ctx, force)

	// if err != nil {

	// 	re := errors.FromError(err)

	// 	if re.Code == 0 {
	// 		// NOTE: is NOT err.(*errors.Error)
	// 		code := http.StatusBadGateway
	// 		re.Id = "chat.bot."+ add.Provider +".register.error"
	// 		// re.Detail = err.Error()
	// 		re.Code = (int32)(code)
	// 		re.Status = http.StatusText(code)
	// 	}

	// 	log.Error().Str("error", re.Detail).Msg("REGISTER")

	// 	return re
	// }

	return agent, nil
}
