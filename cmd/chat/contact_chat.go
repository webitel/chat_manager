package chat

import (
	"context"
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
	//endregion: ----- Authentication -----

	// region: ----- Authorization -----
	//var contactsAccess bool
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	if !authN.Authorization.CanAccess(scope, auth.READ) {
		return errors.Forbidden(
			"contact_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	scope = authN.Authorization.HasObjclass(scopeContacts)
	if scope == nil {
		return errors.Forbidden(
			"contact_chat.objclass.access.denied",
			"denied: require r:contacts access but not granted",
		) // (403) Forbidden
	}
	if !authN.Authorization.CanAccess(scope, auth.READ) {
		return errors.Forbidden(
			"contact_chat.objclass.access.denied",
			"denied: require r:contacts access but not granted",
		) // (403) Forbidden
	}

	// endregion

	// region: ----- Validation -----
	// required!
	if req.GetContactId() == "" {
		return errors.BadRequest(
			"chat.contact_chat.get_contact_chat_history.contact_id.required",
			"chat.history( contact.id: string! ); input: required",
		)
	}
	// endregion: ----- Validation -----

	// ------- Filter(s) ------- //
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		Filter: map[string]any{
			"contact.id": req.GetContactId(),
		},
		Access: auth.READ,
		Fields: req.Fields,
		Size:   int(req.GetSize()),
		Page:   int(req.GetPage()),
	}

	if chatId := req.GetChatId(); chatId != "" {
		search.FilterAND("chat.id", chatId)
	}

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

	return nil
}
