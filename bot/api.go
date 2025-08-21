package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	audit "github.com/webitel/chat_manager/logger"
	"google.golang.org/grpc/metadata"

	// "github.com/golang/protobuf/proto"
	"github.com/micro/micro/v3/service/errors"
	"google.golang.org/protobuf/proto"

	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
)

// implements ...
var _ pbbot.BotsHandler = (*Service)(nil)

const objclassBots = "chat_bots"

// Search returns list of bots, posibly filtered out with search conditions
func (srv *Service) SearchBot(ctx context.Context, req *pbbot.SearchBotRequest, rsp *pbbot.SearchBotResponse) error {

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
				"metadata", "updates",
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

		ID:     req.GetId(),
		Term:   req.GetQ(),
		Filter: map[string]interface{}{
			// "": nil,
		},
		Access: mode, // READ

		Fields: fields,
		Order:  req.GetSort(),
		Size:   int(req.GetSize()),
		Page:   int(req.GetPage()),
	}

	if vs := req.GetUri(); vs != "" {
		search.FilterAND("uri", vs)
	}
	if vs := req.GetName(); vs != "" {
		search.FilterAND("name", vs)
	}
	switch vs := req.GetProvider(); len(vs) {
	case 0:
	case 1:
		search.FilterAND("provider", vs[0])
	default:
		search.FilterAND("provider", vs)
	}

	// size := search.GetSize() // normalized
	list, err := srv.store.Search(&search)

	if err != nil {
		return err
	}
	// Prepare results page
	var (
		size  = len(list)
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
}

// Select returns a single bot profile by unique identifier
func (srv *Service) SelectBot(ctx context.Context, req *pbbot.SelectBotRequest, rsp *pbbot.Bot) error {

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
	if err != nil {
		return err
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
				"provider", "metadata",
				"updates",
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

		Size: 1,
		Page: 1,
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
func (srv *Service) CreateBot(ctx context.Context, add *pbbot.Bot, obj *pbbot.Bot) error {

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
	agent, err := srv.setup(add, srv.fileService)
	// agent.External, err = setup(agent)

	if err != nil {
		return err
	}

	// Prepare Operation Context
	create := app.CreateOptions{
		Context: *(authN),
		Fields: []string{
			// "dc",         // normal: source .Creds
			"id",   // assigned from store !
			"flow", // need display name !
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
			re.Id = "chat.bot." + add.Provider + ".register.error"
			// re.Detail = err.Error()
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
		}
		srv.Log.Error("REGISTER",
			slog.String("error", re.Detail),
		)

		return re
	}

	// Prepare Result: shallowcopy !
	*(obj) = *(add)

	srv.LogAction(ctx, audit.NewCreateMessage(authN, getClientIp(ctx), objclassBots).One(&audit.Record{Id: obj.Id, NewState: obj}))
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
	const ChatProductName = "CHAT"
	for _, grant := range tenant.GetLicense() {
		if grant.Product != ChatProductName {
			continue // Lookup CHAT only !
		}
		if errs := grant.Status.Errors; len(errs) != 0 {
			// Also, ignore single 'product exhausted' (remain < 1) error
			// as we do not consider product user assignments here ...
			if !(len(errs) == 1 && errs[0] == "product exhausted") {
				continue // Currently invalid
			}
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

	if (int)(limitMax) < (n + delta) {
		return errors.New(
			"bot.register.limit.exhausted",
			"bots: gateway registration is limited; maximum number of active: "+strconv.FormatInt((int64)(limitMax), 10),
			http.StatusPreconditionFailed,
		)
	}

	return nil
}

// Update single bot
func (srv *Service) UpdateBot(ctx context.Context, req *pbbot.UpdateBotRequest, rsp *pbbot.Bot) error {

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
			"metadata", "updates",
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
		case "name", "flow", "enabled", "updates", "metadata":
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
		src    *pbbot.Bot // OLD Source
		lookup = app.SearchOptions{

			Context: *(authN),

			Fields: []string{"+"}, // ALL
			Access: mode,          // WRITE

			ID:   []int64{oid},
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
	res := proto.Clone(src).(*pbbot.Bot) // NEW Target !
	// DO: Merge changes ...
	app.MergeProto(res, dst, fields...)

	// DO: REGISTER ?
	if res.Enabled && !src.Enabled {
		err = srv.constraintChatBotsLimit(authN, +1)
		if err != nil {
			// ERR: chat: gateway registration is limited to a maximum of active at a time
			return err
		}
	}

	// // TODO: Validate result object !
	// err = Validate(res)
	// TODO: check provider specific .metadata options !!!!!!!!!!!!!!!!!!!!!!!
	gate, err := srv.setup(res, srv.fileService)

	if err != nil {
		return err // 400
	}

	// Save changes to persistent store ...
	modify := app.UpdateOptions{
		Context: *(authN),
		Fields:  fields, // partial
	}
	// Track operation details
	res.UpdatedAt = authN.Timestamp()
	res.UpdatedBy = &pbbot.Refer{
		Id:   authN.Creds.GetUserId(),
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
	srv.LogAction(ctx, audit.NewUpdateMessage(authN, getClientIp(ctx), objclassBots).One(&audit.Record{Id: rsp.Id, NewState: rsp}))
	// Success
	return nil
}

// Delete bot(s) selection
func (srv *Service) DeleteBot(ctx context.Context, req *pbbot.SearchBotRequest, rsp *pbbot.SearchBotResponse) error {

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
		Fields: []string{
			"id", "uri",
			"enabled",
		},
		Access: mode, // DELETE
		Size:   len(ids),
		ID:     ids, // Find ALL to be deleted
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
		ID:      ids,
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
			run.Lock() // +RW
			run.Enabled = false
			run.deleted = true
			if len(run.external) == 0 {
				_ = run.Deregister(context.TODO())
				_ = run.Remove()
				_ = run.External.Close()
			}
			run.Unlock() // -RW
		}
	}
	// srv.indexMx.Unlock() // -RW
	var records []*audit.Record
	for _, item := range rsp.Items {
		records = append(records, &audit.Record{Id: item.Id})
	}
	srv.LogAction(ctx, audit.NewDeleteMessage(authN, getClientIp(ctx), objclassBots).Many(records))
	return nil

	// panic("not implemented") // TODO: Implement
}

// SendMessage to external chat end-user (contact) side
func (srv *Service) SendMessage(ctx context.Context, req *pbbot.SendMessageRequest, rsp *pbbot.SendMessageResponse) error {

	pid := req.GetProfileId()
	if pid == 0 {
		return errors.BadRequest(
			"chat.bot.send.profile_id.required",
			"gateway: send.profile_id required but missing",
		)
	}

	msg := req.GetMessage()
	if msg == nil {
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

// func metadataHash(md map[string]string) []byte {

// 	n := len(md)
// 	if n == 0 {
// 		return nil
// 	}

// 	keys := make([]string, 0, n)
// 	for key, _ := range md {
// 		keys = append(keys, key)
// 	}

// 	sort.Strings(keys)

// 	hash := md5.New()
// 	for _, key := range keys {
// 		hash.Write([]byte(key))
// 		hash.Write([]byte{':'})
// 		hash.Write([]byte(md[key]))
// 		hash.Write([]byte{';'})
// 	}

// 	return hash.Sum(nil)
// }

func (srv *Service) SendUserAction(ctx context.Context, req *pbbot.SendUserActionRequest, rsp *pbchat.SendUserActionResponse) error {

	// Lookup running profile by id !
	pid := req.GetProfileId()
	via, err := srv.Gateway(ctx, pid, "")

	if err != nil {
		return err
	}

	if via == nil || via.GetId() != pid {
		return errors.BadRequest(
			"chat.action.via.not_found",
			"sendChatAction: via profile.id=%d not found",
			pid,
		)
	}

	// Does provider support .SendUserAction method ?
	provider := via.External
	sender, is := provider.(interface {
		SendUserAction(ctx context.Context, chatId string, action pbchat.UserAction) (bool, error)
	})

	if !is {
		// Not implemented
		rsp.Ok = false
		return nil
		// return errors.BadRequest(
		// 	"chat.action.via.not_supported",
		// 	"sendChatAction: via profile.type=%s not supported",
		// 	via.External.String(),
		// )
	}

	ok, err := sender.SendUserAction(ctx, req.GetExternalUserId(), req.GetAction())
	if err != nil {
		return err
	}
	rsp.Ok = ok
	return nil
}

// Broadcast message [from] bot profile [to] multiple recipients
func (srv *Service) BroadcastMessage(ctx context.Context, req *pbbot.BroadcastMessageRequest, rsp *pbbot.BroadcastMessageResponse) error {

	/*

		+------------------------------------------------------+
		|                    Сonstraints                       |
		|------------------------------------------------------|
		| Topic       | Viber | Telegram | WhatsApp | Facebook |
		|-------------|-------|----------|----------|----------|
		| Text len    | 4096  | 4096     | 4096     | 2000     |
		| File size   | 1-3MB | 5-50MB   | 5-100MB  | 25MB     |
		| Caption len | 768   | 1024     | 1024     | NONE     |
		+------------------------------------------------------+

		+------------------------------------------------------------------------------------------------------------------------+
		|                                                  Documents                                                             |
		|------------------------------------------------------------------------------------------------------------------------|
		| Viber    | Text len    - https://developers.viber.com/docs/api/rest-bot-api/#send-message                              |
		|          | File size   - https://developers.viber.com/docs/api/rest-bot-api/#send-message                              |
		|          | Caption len - https://developers.viber.com/docs/api/rest-bot-api/#send-message                              |
		|----------|-------------------------------------------------------------------------------------------------------------|
		| Telegram | Text len    - https://core.telegram.org/bots/api#sendmessage                                                |
		|          | File size   - https://core.telegram.org/bots/api#sending-files                                              |
		|          | Caption len - https://core.telegram.org/bots/api#sendphoto                                                  |
		|----------|-------------------------------------------------------------------------------------------------------------|
		| WhatsApp | Text len    - https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#text-object        |
		|          | File size   - https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#supported-media-types |
		|          | Caption len - https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#media-object       |
		|----------|-------------------------------------------------------------------------------------------------------------|
		| Facebook | Text len    - https://developers.facebook.com/docs/messenger-platform/reference/send-api/#properties        |
		|          | File size   - https://developers.facebook.com/docs/messenger-platform/reference/send-api/#properties        |
		|          | Caption len - NONE                                                                                          |
		+------------------------------------------------------------------------------------------------------------------------+

	*/

	// Get message params from request
	message := req.GetMessage()
	if message == nil {
		return errors.BadRequest(
			"chat.broadcast.message.required",
			"broadcast: message required but missing",
		)
	}

	// Data normalization
	message.Type = strings.ToLower(strings.TrimSpace(message.Type))
	if message.Type == "" {
		message.Type = "text"

		if message.File != nil {
			message.Type = "file"
		}
	}

	// Lookup bot profile by from.id !
	fromId := req.From
	from, err := srv.Gateway(ctx, fromId, "")
	if err != nil {
		return err
	}

	if from == nil || from.Bot.GetId() != fromId {
		return errors.BadRequest(
			"chat.broadcast.from.not_found",
			"broadcast: from.id=%d bot not found",
			fromId,
		)
	}

	if !from.Bot.GetEnabled() {
		return errors.BadRequest(
			"chat.broadcast.from.disabled",
			"broadcast: from.id=%d bot disabled",
			fromId,
		)
	}

	// Does provider support .Broadcast interface ?
	provider := from.External
	sender, is := provider.(interface {
		BroadcastMessage(ctx context.Context, req *pbbot.BroadcastMessageRequest, res *pbbot.BroadcastMessageResponse) error
	})

	if !is {
		return errors.BadRequest(
			"chat.broadcast.from.not_supported",
			"broadcast: from.type=%s not supported",
			from.External.String(),
		)
	}

	switch message.Type {
	case "text":
		text := message.Text
		if text == "" {
			return errors.BadRequest(
				"chat.broadcast.message.text.required",
				"broadcast: message.text required but missing",
			)
		}

		if message.File != nil {
			return errors.BadRequest(
				"chat.broadcast.message.file.invalid",
				"broadcast: type 'text' does not accept files",
			)
		}

	case "file":
		file := message.File

		if file == nil {
			return errors.BadRequest(
				"chat.broadcast.message.file.required",
				"broadcast: message.file required but missing",
			)
		}

		file.Url = strings.TrimSpace(file.Url)
		if file.GetUrl() == "" {
			return errors.BadRequest(
				"chat.broadcast.message.file.url.required",
				"broadcast: message.file.url required but missing",
			)
		}

		file.Name = strings.TrimSpace(file.Name)
		if file.Name == "" {
			return errors.BadRequest(
				"chat.broadcast.message.file.name.required",
				"broadcast: message.file.name required but missing",
			)
		}

		file.Mime = strings.TrimSpace(file.Mime)
		if file.Mime == "" {
			return errors.BadRequest(
				"chat.broadcast.message.file.mime.required",
				"broadcast: message.file.mime required but missing",
			)
		}

	default:
		return errors.BadRequest(
			"chat.broadcast.message.type.invalid",
			"broadcast: message( type: %s ); not supported",
			message.Type,
		)
	}

	return sender.BroadcastMessage(ctx, req, rsp)
}

func getClientIp(ctx context.Context) string {
	v := ctx.Value("grpc_ctx")
	info, ok := v.(metadata.MD)
	if !ok {
		info, ok = metadata.FromIncomingContext(ctx)
	}
	if !ok {
		return ""
	}
	ip := strings.Join(info.Get("x-real-ip"), ",")
	if ip == "" {
		ip = strings.Join(info.Get("x-forwarded-for"), ",")
	}

	return ip
}
