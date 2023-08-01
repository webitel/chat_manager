package chat

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
)

type Catalog struct {
	logs  *zerolog.Logger
	authN *auth.Client
	store store.CatalogStore
}

type CatalogOption func(srv *Catalog) error

func CatalogLogs(logs *zerolog.Logger) CatalogOption {
	return func(srv *Catalog) error {
		srv.logs = logs
		return nil
	}
}

func CatalogAuthN(client *auth.Client) CatalogOption {
	return func(srv *Catalog) error {
		srv.authN = client
		return nil
	}
}

func CatalogStore(store store.CatalogStore) CatalogOption {
	return func(srv *Catalog) error {
		srv.store = store
		return nil
	}
}

func NewCatalog(opts ...CatalogOption) *Catalog {
	srv := &Catalog{}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

var _ pb.CatalogHandler = (*Catalog)(nil)

const scopeChats = "chats"

// Query of external chat customers
func (srv *Catalog) GetCustomers(ctx context.Context, req *pb.ChatCustomersRequest, res *pb.ChatCustomers) error {

	// region: ----- Authentication -----
	authN, err := app.GetContext(
		ctx, app.AuthorizationRequire(
			srv.authN.GetAuthorization,
		),
	)

	if err != nil {
		return err // 401
	}
	// wrapped
	// ctx = authN.Context
	// endregion: ----- Authentication -----

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	// Prepare SELECT request
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		// Filter: map[string]interface{}{
		// 	"": nil,
		// },
		Access: auth.READ,
		Fields: app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// default
				[]string{
					"id",
					"type",
					"name",
				},
				// extra
				[]string{
					"via", // text gateway(s) mentioned in res.peers
				},
			),
		),
		Order: app.FieldsFunc(
			req.Sort, app.InlineFields,
		),
		Size: int(req.GetSize()),
		Page: int(req.GetPage()),
	}
	// Can SELECT ANY object(s) ?
	super := &auth.PermissionSelectAny
	if !authN.HasPermission(super.Id) {
		// SELF related ONLY (!)
		search.FilterAND("self", authN.Creds.GetUserId())
	}
	// endregion: ----- Authorization -----

	// ------- Filter(s) ------- //
	if vs := req.Id; len(vs) > 0 {
		search.FilterAND("id", vs)
	}
	if vs := req.Via; vs != nil {
		search.FilterAND("via", vs)
	}
	if vs := req.Type; vs != "" {
		search.FilterAND("type", vs)
	}
	// PERFORM
	err = srv.store.GetCustomers(&search, res)

	if err != nil {
		return err
	}

	// TODO: Output sanitizer ...

	return nil
}

// Query of chat conversations
func (srv *Catalog) GetDialogs(ctx context.Context, req *pb.ChatDialogsRequest, res *pb.ChatDialogs) error {

	// region: ----- Authentication -----
	authN, err := app.GetContext(
		ctx, app.AuthorizationRequire(
			srv.authN.GetAuthorization,
		),
	)

	if err != nil {
		return err // 401
	}
	// ctx = authN.Context // wrapped
	// endregion: ----- Authentication -----

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	// Prepare SELECT request
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		// Filter: map[string]interface{}{
		// 	"": nil,
		// },
		Access: auth.READ,
		Fields: app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// default
				[]string{
					"id",
					"via",
					"from",
					"date",
					"title",
					"closed",
					"started",
					"message",
				},
				// extra
				[]string{
					"members",
					"context",
				},
			),
		),
		Order: app.FieldsFunc(
			req.Sort, app.InlineFields,
		),
		Size: int(req.GetSize()),
		Page: int(req.GetPage()),
	}
	// Can SELECT ANY object(s) ?
	super := &auth.PermissionSelectAny
	if !authN.HasPermission(super.Id) {
		// SELF related ONLY (!)
		search.FilterAND("self", authN.Creds.GetUserId())
	}
	// endregion: ----- Authorization -----

	// ------- Filter(s) ------- //
	if vs := req.Id; len(vs) > 0 {
		search.FilterAND("id", vs)
	}
	if vs := req.Via; vs != nil {
		search.FilterAND("via", vs)
	}
	if vs := req.Date; vs != nil {
		search.FilterAND("date", vs)
	}
	if vs := req.Peer; vs != nil {
		search.FilterAND("peer", vs)
	}
	if vs := req.Online; vs != nil {
		online := vs.GetValue()
		search.FilterAND("online", &online)
	}
	// PERFORM
	err = srv.store.GetDialogs(&search, res)

	if err != nil {
		return err
	}

	// TODO: Output sanitizer ...

	return nil
}

// Query of chat participants
func (srv *Catalog) GetMembers(ctx context.Context, req *pb.ChatMembersRequest, res *pb.ChatMembers) error {
	// region: ----- Validation -----
	if req.GetChatId() == "" {
		return errors.BadRequest(
			"catalog.members.chat.id.required",
			"members( chat: id! ); required",
		)
	}
	// endregion: ----- Validation -----

	// region: ----- Authentication -----
	authN, err := app.GetContext(
		ctx, app.AuthorizationRequire(
			srv.authN.GetAuthorization,
		),
	)
	if err != nil {
		return err // 401
	}
	// ctx = authN.Context // wrapped
	// endregion: ----- Authentication -----

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		// Filter: map[string]interface{}{
		// 	"": nil,
		// },
		Access: auth.READ,
		Fields: app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// default
				[]string{
					"id",
					"via",
					"peer",
					"left",
					"join",
					"invite",
				},
				// extra
				[]string{
					"context",
				},
			),
		),
		Order: app.FieldsFunc(
			req.Sort, app.InlineFields,
		),
		Size: int(req.GetSize()),
		Page: int(req.GetPage()),
	}
	// Can SELECT ANY object(s) ?
	super := &auth.PermissionSelectAny
	if !authN.HasPermission(super.Id) {
		// SELF related ONLY (!)
		search.FilterAND("self", authN.Creds.GetUserId())
	}
	// endregion: ----- Authorization -----

	// ------- Filter(s) ------- //
	// mandatory: chat_id AS thread.id
	search.FilterAND("thread.id", req.ChatId)
	// chat( id: [id!] )
	if vs := req.GetId(); len(vs) > 0 {
		search.FilterAND("member.id", vs)
	}
	// chat( via: peer )
	if vs := req.GetVia(); vs != nil {
		if vs.Id != "" || vs.Type != "" || vs.Name != "" {
			search.FilterAND("via", vs)
		}
	}
	// chat( peer: peer )
	if vs := req.GetPeer(); vs != nil {
		if vs.Id != "" || vs.Type != "" || vs.Name != "" {
			search.FilterAND("peer", vs)
		}
	}
	// chat( date: timerange )
	if vs := req.GetDate(); vs != nil {
		if vs.Since > 0 || vs.Until > 0 {
			search.FilterAND("date", vs)
		}
	}
	// chat( online: bool )
	if vs := req.GetOnline(); vs != nil {
		search.FilterAND("online", vs)
	}
	// chat( joined: bool )
	if vs := req.GetJoined(); vs != nil {
		search.FilterAND("joined", vs)
	}
	// PERFORM
	list, re := srv.store.GetMembers(&search)
	if err = re; err != nil {
		return err
	}

	// *(res) = *(list)
	res.Data = list.Data
	// res.Users = list.Users
	res.Page = list.Page
	res.Next = list.Next

	// TODO: Output sanitizer ...

	return nil
}

// Query of chat messages history
func (srv *Catalog) GetHistory(ctx context.Context, req *pb.ChatMessagesRequest, res *pb.ChatMessages) error {
	// region: ----- Validation -----
	var peer *pb.Peer // mandatory(!)
	switch input := req.GetChat().(type) {
	case *pb.ChatMessagesRequest_Peer:
		{
			peer = input.Peer
		}
	case *pb.ChatMessagesRequest_ChatId:
		{
			if input.ChatId != "" {
				chatId, err := uuid.Parse(input.ChatId)
				if err != nil { // || chatId.IsZero() {
					return errors.BadRequest(
						"messages.query.chat.id.input",
						"messages( chat: %s ); input: invalid id",
						input.ChatId,
					)
				}
				peer = &pb.Peer{
					Type: "chat",
					Id:   hex.EncodeToString(chatId[:]),
				}
			}
		}
	}

	if peer.GetId() == "" {
		return errors.BadRequest(
			"messages.query.peer.id.required",
			"messages( peer.id: string! ); input: required",
		)
	}

	if peer.GetType() == "" {
		return errors.BadRequest(
			"messages.query.peer.type.required",
			"messages( peer.type: string! ); input: required",
		)
	}
	// endregion: ----- Validation -----

	// region: ----- Authentication -----
	authN, err := app.GetContext(
		ctx, app.AuthorizationRequire(
			srv.authN.GetAuthorization,
		),
	)
	if err != nil {
		return err // 401
	}
	// ctx = authN.Context // wrapped
	// endregion: ----- Authentication -----

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		Filter: map[string]any{
			"peer": peer, // mandatory(!)
		},
		Access: auth.READ,
		Fields: app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// default
				[]string{
					"id",
					"from", // sender; user
					"date",
					"edit",
					"text",
					"file",
				},
				// extra
				[]string{
					"chat",   // chat dialog, that this message belongs to ..
					"sender", // chat member, on behalf of the "chat" (dialog)
					"context",
				},
			),
		),
		// Order: app.FieldsFunc(
		// 	req.Sort, app.InlineFields,
		// ),
		// Size: int(req.GetSize()),
		// Page: int(req.GetPage()),
		Size: int(req.GetLimit()),
	}
	// Can SELECT ANY object(s) ?
	super := &auth.PermissionSelectAny
	if !authN.HasPermission(super.Id) {
		// SELF related ONLY (!)
		search.FilterAND("self", authN.Creds.GetUserId())
	}
	indexField := func(name string) int {
		var e, n = 0, len(search.Fields)
		for ; e < n && search.Fields[e] != name; e++ {
			// lookup: field specified ?
		}
		if e < n {
			return e // FOUND !
		}
		return -1 // NOT FOUND !
	}
	switch peer.Type {
	case "chat":
		// Hide: { chat }; Input given, will be the same for all messages !
		e := indexField("chat")
		for e >= 0 {
			search.Fields = append(
				search.Fields[0:e], search.Fields[e+1:]...,
			)
			e = indexField("chat")
		}
	default:
		// [ bot, user, viber, telegram, ... ]
		// Query: { chat }; To be able to distinguish individual chat dialogs
		if indexField("chat") < 0 {
			search.Fields = append(search.Fields, "chat")
		}
	}
	// endregion: ----- Authorization -----

	// ------- Filter(s) ------- //
	if vs := req.Offset; vs != nil {
		search.FilterAND("offset", vs)
	}

	list, re := srv.store.GetHistory(&search)
	if err = re; err != nil {
		return err
	}

	// *(res) = *(list)
	res.Messages = list.Messages
	res.Chats = list.Chats
	res.Peers = list.Peers
	res.Page = list.Page
	res.Next = list.Next

	// TODO: Output sanitizer ...

	return nil
}
