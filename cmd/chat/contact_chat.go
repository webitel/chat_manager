package chat

import (
	"context"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	oauth "github.com/webitel/chat_manager/api/proto/auth"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/protos/gateway/contacts"
	"strconv"
)

type ContactChatHistoryService struct {
	logs          *zerolog.Logger
	authN         *auth.Client
	store         store.CatalogStore
	contactClient contacts.ContactsService
}

type ContactChatHistoryServiceOption func(srv *ContactChatHistoryService) error

func NewContactChatHistoryService(opts ...ContactChatHistoryServiceOption) *ContactChatHistoryService {
	srv := &ContactChatHistoryService{}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

func ContactChatHistoryServiceLogs(logs *zerolog.Logger) ContactChatHistoryServiceOption {
	return func(srv *ContactChatHistoryService) error {
		srv.logs = logs
		return nil
	}
}

func ContactChatHistoryServiceAuthN(client *auth.Client) ContactChatHistoryServiceOption {
	return func(srv *ContactChatHistoryService) error {
		srv.authN = client
		return nil
	}
}

func ContactChatHistoryServiceStore(store store.CatalogStore) ContactChatHistoryServiceOption {
	return func(srv *ContactChatHistoryService) error {
		srv.store = store
		return nil
	}
}
func ContactChatHistoryServiceContactClient(client contacts.ContactsService) ContactChatHistoryServiceOption {
	return func(srv *ContactChatHistoryService) error {
		srv.contactClient = client
		return nil
	}
}

func (srv *ContactChatHistoryService) bindNativeClient(ctx *app.Context) error {
	authZ := &ctx.Authorization
	if authZ.Creds == nil && authZ.Native != nil {
		md, _ := metadata.FromContext(
			ctx.Context,
		)
		dc, _ := strconv.ParseInt(
			md["X-Webitel-Domain"], 10, 64,
		)
		authZ.Creds = &oauth.Userinfo{
			Dc: dc,
			Permissions: []*oauth.Permission{
				&auth.PermissionSelectAny,
			},
			Scope: []*oauth.Objclass{{
				Class:  scopeChats,
				Access: "r",
			}},
		}
	}
	return nil
}

// Query of the chat history
func (srv *ContactChatHistoryService) GetContactChatHistory(ctx context.Context, req *pb.GetContactChatHistoryRequest, res *pb.GetContactChatHistoryResponse) error {
	// region: ----- Authentication -----
	authN, err := app.GetContext(
		ctx, app.AuthorizationRequire(
			srv.authN.GetAuthorization,
		),
		srv.bindNativeClient,
	)
	if err != nil {
		return err // 401
	}
	// endregion: ----- Authentication -----

	// region: ----- Validation -----
	var peer *pb.Peer // mandatory(!)

	if req.ChatId != "" {
		chatId, err := uuid.Parse(req.ChatId)
		if err != nil { // || chatId.IsZero() {
			return errors.BadRequest(
				"chat.contact_chat.get_contact_chat_history.chat_id.input",
				"chat.history( chat: %s ); input: invalid id",
				req.ChatId,
			)
		}
		peer = &pb.Peer{
			Type: "chat",
			Id:   hex.EncodeToString(chatId[:]),
		}
	}

	if peer.GetId() == "" {
		return errors.BadRequest(
			"chat.contact_chat.get_contact_chat_history.chat_id.required",
			"chat.history( chat.id: string! ); input: required",
		)
	}

	if peer.GetType() == "" {
		return errors.BadRequest(
			"chat.contact_chat.get_contact_chat_history.peer_type.required",
			"chat.history( chat.type: string! ); input: required",
		)
	}

	if req.GetContactId() == "" {
		return errors.BadRequest(
			"chat.contact_chat.get_contact_chat_history.contact_id.required",
			"chat.history( contact.id: string! ); input: required",
		)
	}
	// endregion: ----- Validation -----

	// region: ----- Check contact access -----

	_, err = srv.contactClient.SearchContacts(ctx, &contacts.SearchContactsRequest{Id: []string{req.ContactId}})
	if err != nil {
		return err
	}
	// endregion: ----- Check contact access -----

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
		Size: int(req.GetLimit()),
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
	if vs := req.Group; len(vs) > 0 {
		if delete(vs, ""); len(vs) > 0 {
			search.FilterAND("group", vs)
		}
	}

	list, re := srv.store.GetContactChatHistory(&search)
	if err = re; err != nil {
		return err
	}

	res.Messages = list.Messages
	res.Chats = list.Chats
	res.Peers = list.Peers
	res.Page = list.Page
	res.Next = list.Next

	// TODO: Output sanitizer ...

	return nil
}
