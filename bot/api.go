package bot

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/micro/go-micro/v2/errors"
	"github.com/rs/zerolog/log"

	"github.com/webitel/chat_manager/api/proto/bot"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
)

// implements ...
var _ bot.BotsHandler = (*Service)(nil)

const objclassBots = "chat_bots"

// Search returns list of bots, posibly filtered out with search conditions
func (srv *Service) SearchBot(ctx context.Context, req *bot.SearchBotRequest, rsp *bot.SearchBotResponse) error {
	
	authN, err := app.GetContext(ctx,
		app.AuthorizationRequire(srv.Auth.GetAuthorization),
	)

	if err != nil {
		return err
	}

	scope := authN.HasObjclass(objclassBots)
	if scope == nil {
		// ERR: Has NO license product GRANTED !
		return errors.Forbidden(
			"chat.bot.access.denied",
			"chatbot: objclass access DENIED !",
		)
	}

	const mode = auth.READ
	if !authN.CanAccess(scope, mode) {
		// ERR: Has NO access to objclass been GRANTED !
		return errors.Forbidden(
			"chat.bot.access.forbidden",
			"chatbot: objclass READ privilege NOT GRANTED !",
		)
	}

	// Normalize FIELDS request
	fields := app.FieldsFunc(
		req.GetFields(),
		app.SelectFields(
			// application: default
			[]string{
				"id", // "dc",
				"name", "uri",
				"enabled", "flow",
				"provider", // "metadata",
				// "created_at", // "created_by",
				// "updated_at", "updated_by",
			},
			// operational
			[]string{
				"dc",
				"metadata",
				"created_at", "created_by",
				"updated_at", "updated_by",
			},
		),
	)

	if scope.Rbac {
		// NOTE: ObjClass "bots" [R]ecord [b]ased [A]ccess [C]ontrol Policy ENABLED !
	}

	search := app.SearchOptions{
		// Operation Context
		Context: *(authN),
	
		ID:   req.GetId(),
		Term: req.GetQ(),
		Filter: map[string]interface{}{
			// "": nil,
		},
		Access: mode, // READ

		Fields: fields,
		Order:  req.GetSort(),
		Size:   int(req.GetSize()),
		Page:   int(req.GetPage()),
	}

	// size := search.GetSize() // normalized
	list, err := srv.store.Search(&search)

	if err != nil {
		return err
	}
	// Prepare results page
	var (

		size = len(list)
		limit = search.GetSize()
	)
	
	// normalized
	rsp.Page = int32(search.GetPage())
	rsp.Next = 0 < limit && limit < size // returned MORE than LIMIT requested

	if rsp.Next {
		size = limit
	}

	rsp.Items = list[0:size]

	return nil
	
	panic("not implemented") // TODO: Implement
}

// Select returns a single bot profile by unique identifier
func (srv *Service) SelectBot(ctx context.Context, req *bot.SelectBotRequest, rsp *bot.Bot) error {

	// Quick Validation(!)
	var (

		oid = req.GetId()
		uri = req.GetUri()
	)

	if oid == 0 && uri == "" {
		return errors.BadRequest(
			"chat.bot.select.cond.missing",
			"chatbot: select condition is missing",
		)
	}

	// GetAuthorization(!)
	authN, err := app.GetContext(ctx,
		app.AuthorizationRequire(
			srv.Auth.GetAuthorization,
		),
	)

	// Normalize FIELDS request
	fields := app.FieldsFunc(
		req.GetFields(),
		app.SelectFields(
			// application: default
			[]string{
				"id", // "dc",
				"name", "uri",
				"enabled", "flow",
				"provider", "metadata",
				"created_at", "created_by",
				"updated_at", "updated_by",
			},
			// operational
			[]string{
				"dc",
			},
		),
	)

	// Prepare Request
	lookup := app.SearchOptions{
		// Operational context
		Context: *(authN),
		
		Fields: fields,
		Access: 0, // READ

		Size:   1,
		Page:   1,
	}

	if oid != 0 {
		lookup.ID = []int64{oid}
	}

	if uri != "" {
		lookup.FilterAND("uri", uri)
	}

	// Perform
	list, err := srv.store.Search(&lookup)

	if err != nil {
		return err
	}

	if len(list) > 1 {
		return errors.Conflict(
			"chat.bot.select.conflict",
			"chatbot: too much records found; please provide more specific filter condition(s)",
		)
	}

	var obj *Bot
	if len(list) == 1 {
		obj = list[0]
	}

	if obj == nil {
		var expr string
		if oid != 0 {
			expr = fmt.Sprintf(".id=%d", oid)
		}
		if uri != "" {
			if expr != "" {
				expr += " and "
			}
			expr += fmt.Sprintf(".uri=%s", uri)
		}
		return errors.NotFound(
			"chat.bot.select.not_found",
			"chatbot: lookup query %s; not found",
			 expr,
		)
	}

	// *(rsp) = *(obj)
	app.MergeProto(rsp, obj, lookup.Fields...)

	return nil
	
	// panic("not implemented") // TODO: Implement
}

// Create new bot profile
func (srv *Service) CreateBot(ctx context.Context, add *bot.Bot, obj *bot.Bot) error {


	// // region: Validation
	// err := Validate(add)

	// if err != nil {
	// 	return err
	// }

	// log := srv.Log.With().

	// 	Int64("pid", add.Id).
	// 	Int64("pdc", add.GetDc().GetId()).
	// 	Int64("bot", add.Flow.Id).
		
	// 	Str("uri", add.Uri).
		
	// 	Str("title", add.Name).
	// 	Str("channel", add.Provider).

	// 	Logger()

	// // Find provider implementation by code name
	// setup := GetProvider(add.GetProvider())

	// if setup == nil {
		
	// 	log.Warn().Msg("NOT SUPPORTED")
	// 	// Client Request Error !
	// 	return errors.BadRequest(
	// 		"chat.bot.provider.invalid",
	// 		"chatbot: invalid %s provider; not implemented",
	// 		 add.Provider,
	// 	)
	// }
	// endregion: Validation

	// region: Authentication
	authN, err :=
		app.GetContext(ctx,
			app.AuthorizationRequire(
				srv.Auth.GetAuthorization,
			),
		)

	if err != nil {
		return err
	}
	// endregion: Authentication
	
	// region: Authorization
	scope := authN.HasObjclass(objclassBots)
	if scope == nil {
		// ERR: NO Products !
		return errors.Forbidden(
			"chat.bot.access.denied",
			"chatbot: objclass access DENIED !",
		)
	}

	const mode = auth.ADD // CREATE
	if !authN.CanAccess(scope, mode) {
		// ERR: NOT GRANTED !
		return errors.Forbidden(
			"chat.bot.access.forbidden",
			"chatbot: objclass ADD privilege NOT GRANTED !",
		)
	}

	if scope.Rbac {
		// objclass: [R]ecord [b]ased [A]ccess [C]ontrol policy enabled !
	}
	// endregion: Authorization

	if add.Enabled {
		err = srv.constraintChatBotsLimit(authN, +1)
		if err != nil {
			// ERR: chat: gateway registration is limited to a maximum of active at a time
			return err
		}
	}

	// Preset values !
	add.Id = 0 // NEW !
	add.Dc = &Refer{
		Id:   authN.Creds.GetDc(),
		Name: authN.Creds.GetDomain(),
	}
	add.CreatedAt = authN.Timestamp()
	add.CreatedBy = &Refer{
		Id:   authN.Creds.GetUserId(),
		Name: authN.Creds.GetName(),
	}
	add.UpdatedAt = add.CreatedAt
	add.UpdatedBy = add.CreatedBy

	// CHECK: Provider specific options are well formed !
	// agent := &Gateway{

	// 	Log:     &log,
	// 	Bot:      add,
	// 	Internal: srv,
	// 	// CACHE Store
	// 	RWMutex:  new(sync.RWMutex),
	// 	internal: make(map[int64]*Channel), // map[internal.user.id]
	// 	external: make(map[string]*Channel), // map[provider.user.id]
	// }

	// Perform ChatBot Provider setup
	agent, err := srv.setup(add)
	// agent.External, err = setup(agent)

	if err != nil {
		return err
	}

	// Prepare Operation Context
	create := app.CreateOptions{
		Context: *(authN),
		Fields: []string{
			// "dc",         // normal: source .Creds
			"id",            // assigned from store !
			"flow",          // need display name !
			// "created_by", // normal: source .Creds
			// "updated_by", // normal: source .Creds
		},
	}

	// Perform Create Operation
	err = srv.store.Create(&create, add)

	if err == nil {
		// POST Create Validation(s) !
		if add.Id == 0 {
			err = errors.InternalServerError(
				"chat.bot.create.error",
				"chatbot: no unique ID assigned",
			)
		}
	}

	if err != nil {
		return err
	}

	// Enable NOW (?)
	if !add.Enabled {
		// TODO: nothing more ...
		agent.External = nil
		// Result: shallowcopy !
		*(obj) = *(add)
		// Sanitize
		obj.Dc = nil
		// Success
		return nil
	}

	// REGISTER ChatBot WebHook Callback URI (!)
	force := true // [RE-]REGISTER on provider side (?)
	err = agent.Register(ctx, force)
	
	if err != nil {

		re := errors.FromError(err)

		if re.Code == 0 {
			// NOTE: is NOT err.(*errors.Error)
			code := http.StatusBadGateway
			re.Id = "chat.bot."+ add.Provider +".register.error"
			// re.Detail = err.Error()
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}

		log.Error().Str("error", re.Detail).Msg("REGISTER")

		return re
	}

	// Prepare Result: shallowcopy !
	*(obj) = *(add)
	// Sanitize Result
	obj.Dc = nil

	return nil

	// panic("not implemented") // TODO: Implement
}

func (srv *Service) constraintChatBotsLimit(req *app.Context, delta int) error {

	tenant, err := srv.Auth.GetCustomer(
		req.Context, req.Authorization.Token,
	)

	if err != nil {
		return err
	}
	// Choose license.product(s).CHAT maximum .limit count
	var limitMax int32
	for _, grant := range tenant.GetLicense() {
		if grant.Product != "CHAT" {
			continue // Lookup CHAT only !
		}
		if len(grant.Status.Errors) != 0 {
			continue // Currently invalid
		}
		if limitMax < grant.Limit {
			limitMax = grant.Limit
		}
	}

	if limitMax == 0 {
		// FIXME: No CHAT product(s) issued !
		return errors.New(
			"bot.register.product.not_found",
			"bots: CHAT product required but missing",
			 http.StatusPreconditionFailed,
		)
	}

	n, err := srv.store.AnalyticsActiveBotsCount(
		req.Context, req.Authorization.Creds.GetDc(),
	)

	if err != nil {
		return err
	}

	if (int)(limitMax) < (n+delta) {
		return errors.New(
			"bot.register.limit.exhausted",
			"bots: gateway registration is limited; maximum number of active: "+ strconv.FormatInt((int64)(limitMax), 10),
			 http.StatusPreconditionFailed,
		)
	}

	return nil
}

// Update single bot
func (srv *Service) UpdateBot(ctx context.Context, req *bot.UpdateBotRequest, rsp *bot.Bot) error {

	var (

		dst = req.GetBot() // NEW Source
		oid = dst.GetId()
	)

	if oid == 0 {
		return errors.BadRequest(
			"chat.bot.update.id.required",
			"chatbot: update .id required but missing",
		)
	}

	// Collapse metadata.*, created_by.*, ... fields !
	fields := app.FieldsMask(
		req.GetFields(), 1, // base: "."
	)
	if len(fields) == 0 {
		// default: PUT !
		fields = []string{
			// "id", "dc", "uri",
			"name", "flow", "enabled",
			// "provider",
			"metadata",
			// "created_at", // "created_by",
			// "updated_at", "updated_by",
		}
	}

	// Validate UPDATE fields !
	for _, att := range fields {
		switch att {
		// READONLY
		case "id", "dc", "uri", "provider",
			"created_at", "created_by",
			"updated_at", "updated_by":
			// FIXME: 
			return errors.BadRequest(
				"chat.bot.update.field.readonly",
				"chatbot: update .%s; attribute is readonly",
				 att,
			)
		// EDITABLE
		case "name", "enabled", "flow", "metadata":
		// INVALID
		default:
			return errors.BadRequest(
				"chat.bot.update.field.invalid",
				"chatbot: update .%s; attribute is unknown",
				 att,
			)
		}
	}
	// Set normalized !
	req.Fields = fields

	// Authentication(!)
	authN, err :=
		app.GetContext(ctx,
			app.AuthorizationRequire(
				srv.Auth.GetAuthorization,
			),
		)

	if err != nil {
		return err
	}

	// Authorization(!)
	scope := authN.HasObjclass(objclassBots)
	if scope == nil {
		// ERR: NO Products !
		return errors.Forbidden(
			"chat.bot.access.denied",
			"chatbot: objclass access DENIED !",
		)
	}

	const mode = auth.WRITE // UPDATE
	if !authN.CanAccess(scope, mode) {
		// ERR: NOT GRANTED !
		return errors.Forbidden(
			"chat.bot.access.forbidden",
			"chatbot: objclass WRITE privilege NOT GRANTED !",
		)
	}
	// Chain API Context
	ctx = authN.Context

	if scope.Rbac {
		// NOTE: ObjClass "bots" [R]ecord [b]ased [A]ccess [C]ontrol Policy ENABLED !
	}

	// Fetch source !
	var (

		src *bot.Bot // OLD Source
		lookup = app.SearchOptions{
			
			Context: *(authN),
			
			Fields: []string{"+"}, // ALL
			Access: mode,          // WRITE
			
			ID: []int64{oid},
			Size: 1,
		}
	)

	src, err = srv.LocateBot(&lookup)

	if err != nil {
		return err
	}

	if src.GetId() != oid {
		return errors.NotFound(
			"chat.bot.locate.not_found",
			"chatbot: update .id=%d; not found",
			 oid,
		)
	}

	// Prepare RESULT object !
	res := proto.Clone(src).(*bot.Bot) // NEW Target !
	// DO: Merge changes ...
	app.MergeProto(res, dst, fields...)

	// DO: REGISTER ?
	if (res.Enabled && !src.Enabled) {
		err = srv.constraintChatBotsLimit(authN, +1)
		if err != nil {
			// ERR: chat: gateway registration is limited to a maximum of active at a time
			return err
		}
	}
	
	// // TODO: Validate result object !
	// err = Validate(res)
	// TODO: check provider specific .metadata options !!!!!!!!!!!!!!!!!!!!!!!
	gate, err := srv.setup(res)
	
	if err != nil {
		return err // 400
	}

	// Save changes to persistent store ...
	modify := app.UpdateOptions{
		Context: *(authN),
		Fields:    fields, // partial
	}
	// Track operation details
	res.UpdatedAt = authN.Timestamp()
	res.UpdatedBy = &bot.Refer{
		Id: authN.Creds.GetUserId(),
		Name: authN.Creds.GetName(),
	}
	// Perform UPDATE
	err = srv.store.Update(&modify, res)

	if err != nil {
		return err
	}

	// // [RE]SET NEW runtime version
	// // if src.Enabled { // Currently enabled ?
	// srv.indexMx.Lock()   // +RW
	// if run, ok := srv.profiles[oid]; ok {
	// 	if run.Enabled = res.Enabled; run.Enabled {
	// 		// if len(this.external) == 0 {
	// 		// 	// NO active channels: RENEW
	// 		// 	srv.profiles[oid] = gate
	// 		// 	gate.Log.Info().Msg("UPGRADED")
	// 		// } else {
	// 		// 	// SET NEW runtime !
	// 		// 	this.next = gate
	// 		// 	gate.Log.Info().Msg("UPDATED")
	// 		// }
	// 		uri := res.Uri
	// 		if pid, ok := srv.gateways[uri]; !ok {
	// 			// NOTE: register NEW callback URI
	// 			// TODO: Need to [re-]register on provider's side !
	// 			srv.gateways[uri] = oid
	// 		} else if pid != oid {
	// 			panic("update: URI reserved")
	// 		}

	// 		// Populate active channels index to NEW bot revision !
	// 		// NOTE: Each channel is linked to specific *Gateway controller and .Bot revision !
	// 		// So running channels must be still served with previous *Gateway controller state !
	// 		// But, since now, NEW channels must be linked to NEW, updated *Gateway controller !
	// 		gate.RWMutex  = run.RWMutex
	// 		gate.external = run.external
	// 		gate.internal = run.internal
	// 		// TODO: need to link gate.External state, but with NEW .Gateway
	// 		// TODO: now channel.Close() must be unlinked in two places !..  =(
	// 		srv.profiles[oid] = gate
	// 		gate.Log.Info().Msg("UPGRADED")
	// 	}
	// } // else { WILL be lazy fetched }
	// srv.indexMx.Unlock() // -RW
	// // }

	// NOTE: (gate.Bot == res) !

	toggle := (gate.Bot.Enabled != src.Enabled)
	// REGISTER *Gateway UPDATED !
	err = gate.Register(ctx, gate.Bot.Enabled && toggle)
	// err = gate.Register(ctx, gate.Enabled && (toggle ||
	// 	(app.HasScope(fields, "uri") && gate.Uri != src.Uri)),
	// )

	if err != nil {
		gate.Bot.Enabled = false
		// TODO: Update persistent DB record
		return err
	}

	// // DISABLE *Gateway Callback URI !
	// gate.Lock()
	// if !gate.Enabled && len(gate.external) == 0 {
	// 	err = gate.Deregister(ctx)
	// }
	// gate.Unlock()

	// if err != nil {
	// 	return err
	// }

/*
	// DO: Apply changes to running bot !
	var post []func() error
	// dst - NEW source
	// src - OLD source
	// res - NEW source
	for _, att := range fields {
		switch att {
		// EDITABLE
		case "uri":
			thisURI := src.GetUri()
			nextURI := dst.GetUri()
			if thisURI != nextURI {
				// FIXME: URI changed !
				if !res.GetEnabled() {
					break
				}
				// APPLY: URI changed !
				post = append(post, func() error {

					srv.indexMx.Lock()   // +RW
					defer srv.indexMx.Unlock() // -RW

					pid, ok := srv.gateways[nextURI]
					if ok && pid != oid {
						return errors.Conflict(
							"chat.bot.uri.conflict",
							"chatbot: service URI reserved",
						)
					}

					pid, ok = srv.gateways[thisURI]
					if ok && pid == oid {
						delete(srv.gateways, thisURI)
						srv.gateways[nextURI] = oid
					}

					return nil
				})
			}
		// case "name":
		case "enabled":
			enable := res.GetEnabled()      // WANT !
			if src.GetEnabled() != enable { // TOGGLE !
				if !enable {                // SWITCH: OFF !
					post = append(post, func() error {

						var (
							ok bool
							bot *Gateway
						)

						srv.indexMx.Lock()   // +RW
						if bot, ok = srv.profiles[oid]; ok {
							delete(srv.gateways, src.GetUri())
							delete(srv.profiles, oid)
						}
						srv.indexMx.Unlock() // -RW
						// NOTE: we need to DEREGISTER the linked webhook URI
						// from provider to NOT receive NO MORE new updates !..
						if !ok {
							bot = gate
						}

						defer bot.External.Close()
						return bot.Deregister(ctx)
					})
				} else {                    // SWITCH: ON !
					// // FIXME: leave as is; will init on first callback ?
					// post = append(post, func() error {
					// 	return gate.Register(ctx, true)
					// })
				}
			} else if enable {              // UPGRADE: RUNNING !
				// FIXME: upgrade running bot changes ?
				post = append(post, func() error {
					
					srv.indexMx.Lock()   // +RW
					defer srv.indexMx.Unlock() // -RW

					run, ok := srv.profiles[oid]
					if ok {
						
						// TODO: NewProviderBot(runtime.(interface{}), gate.(*Gateway))
					}
				})
			}
		// case "flow":
			// FIXME: engine service responsibility
		case "metadata":
			if src.GetEnabled() && !bytes.Equal(
				metadataHash(src.GetMetadata()), // OLD
				metadataHash(res.GetMetadata()), // NEW
			) {
				// METADATA changes !
				post = append(post, func() error {
					// TODO: resync bot's running sessions
					srv.indexMx.Lock()   // +RW
					defer srv.indexMx.Unlock() // -RW

					run, ok := srv.profiles[oid]
					if ok {
						
						// TODO: NewProviderBot(runtime.(interface{}), gate.(*Gateway))
					}
				})
			}
		}
	}
*/

	// Show RESULT !
	// *(rsp) = *(obj)
	app.MergeProto(rsp, res) // ALL
	// Sanitize
	rsp.Dc = nil // == authN.Creds.GetDc()
	// Success
	return nil
}

// Delete bot(s) selection
func (srv *Service) DeleteBot(ctx context.Context, req *bot.SearchBotRequest, rsp *bot.SearchBotResponse) error {
	
	var (

		ids = req.GetId()
	)

	if len(ids) == 0 || ids[0] == 0 {
		return errors.BadRequest(
			"chat.bot.delete.id.required",
			"chatbot: delete .id required but missing",
		)
	}

	// Authentication(!)
	authN, err :=
		app.GetContext(ctx,
			app.AuthorizationRequire(
				srv.Auth.GetAuthorization,
			),
		)

	if err != nil {
		return err
	}

	// Authorization(!)
	scope := authN.HasObjclass(objclassBots)
	if scope == nil {
		// ERR: NO Products !
		return errors.Forbidden(
			"chat.bot.access.denied",
			"chatbot: objclass access DENIED !",
		)
	}

	const mode = auth.DELETE // DELETE
	if !authN.CanAccess(scope, mode) {
		// ERR: NOT GRANTED !
		return errors.Forbidden(
			"chat.bot.access.forbidden",
			"chatbot: objclass DELETE privilege NOT GRANTED !",
		)
	}

	if scope.Rbac {
		// NOTE: ObjClass "bots" [R]ecord [b]ased [A]ccess [C]ontrol Policy ENABLED !
	}

	// TODO: select ALL by ids
	search := app.SearchOptions{
		Context: *(authN),
		Fields:  []string{
			"id", "uri",
			"enabled",
		},
		Access:  mode, // DELETE
		Size:    len(ids),
		ID:      ids, // Find ALL to be deleted
	}

	// PERFORM SELECT !
	list, err := srv.store.Search(&search)

	if err != nil {
		// SEARCH Error !
		return err
	}

	// TODO: Ensure ALL requested selected FOR DELETE !
	var (
		no []int64 // index: records ID(s) NOT FOUND !
	)
	next:
	for _, id := range ids {
		for _, obj := range list {
			if obj.GetId() == id {
				continue next
			}
		}
		no = append(no, id)
	}

	if len(no) != 0 {
		// NOTE: NOT ALL requested ID(s) were selected !
		// NOTE: User may have NOT enough rights
		//       to perform this operation on some objects
		return errors.BadRequest(
			"chat.bot.delete.not_found",
			"chatbot: lookup id=%v; not found",
			 no,
		)
	}

	delete := app.DeleteOptions{
		Context: *(authN),
		ID:        ids,
	}

	// PERFORM DELETE !
	_, err = srv.store.Delete(&delete)

	if err != nil {
		// DELETE Error !
		return err
	}

	// TODO: Stop enabled bot(s) service !
	// TODO: Remove deleted bot(s) gateways !
	// srv.indexMx.Lock()   // +RW
	for _, pid := range ids {
		run, ok := srv.profiles[pid]
		if ok && run != nil {
			run.Lock()   // +RW
			run.Enabled = false
			run.deleted = true
			if len(run.external) == 0 {
				_ = run.Deregister(context.TODO())
				_ = run.Remove()
			}
			run.Unlock() // -RW
		}
	}
	// srv.indexMx.Unlock() // -RW

	return nil
	
	// panic("not implemented") // TODO: Implement
}




// SendMessage to external chat end-user (contact) side
func (srv *Service) SendMessage(ctx context.Context, req *bot.SendMessageRequest, rsp *bot.SendMessageResponse) error {
	
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

	sentBinding := req.GetMessage().GetVariables()
	if sentBinding != nil {
		delete(sentBinding, "")
		if len(sentBinding) != 0 {
			// populate SENT message external bindings
			rsp.Bindings = sentBinding
		}
	}
	// +OK
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

/*/ AddProfile register new profile gateway
func (srv *Service) AddProfile(ctx context.Context, req *bot.AddProfileRequest, res *bot.AddProfileResponse) error {


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

		Log: &log,
		Bot: &Bot{
			Id: add.GetId(),
			Dc: &Refer{
				Id:   add.GetDomainId(),
				Name: "",
			},
			Uri:  add.GetUrlId(),
			Name: add.GetName(),
			Flow: &Refer{
				Id:   add.GetSchemaId(),
				Name: "",
			},
			Enabled:  true,
			Provider: add.GetType(),
			Metadata: add.GetVariables(),
			CreatedAt: 0,
			CreatedBy: nil,
			UpdatedAt: 0,
			UpdatedBy: nil,
		},
		Internal: srv,
		// CACHE Store
		RWMutex:  new(sync.RWMutex),
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

func (srv *Service) AddProfile(ctx context.Context, req *bot.AddProfileRequest, res *bot.AddProfileResponse) error {

	panic("not implemented")

}

// DeleteProfile deregister profile gateway
func (srv *Service) DeleteProfile(ctx context.Context, req *bot.DeleteProfileRequest, res *bot.DeleteProfileResponse) error {
	
	pid := req.GetId()
	uri := req.GetUrlId()
	
	gate, err := srv.Gateway(ctx, pid, uri)
	
	if err != nil {
		return err
	}

	pid = gate.Bot.Id
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
}*/


// func (srv *Service) register() error {}

// func (srv *Service) deregister() error {}

func metadataHash(md map[string]string) []byte {

	n := len(md)
	if n == 0 {
		return nil
	}

	keys := make([]string, 0, n)
	for key, _ := range md {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	hash := md5.New()
	for _, key := range keys {
		hash.Write([]byte(key))
		hash.Write([]byte{':'})
		hash.Write([]byte(md[key]))
		hash.Write([]byte{';'})
	}

	return hash.Sum(nil)
}