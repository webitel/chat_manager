package chat

import (
	"context"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
	oauth "github.com/webitel/chat_manager/api/proto/auth"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/protos/gateway/contacts"
	"log/slog"
	"strconv"
)

const scopeCases = "cases"

type CaseChatHistoryService struct {
	logs          *slog.Logger
	authN         *auth.Client
	store         store.CatalogStore
	contactClient contacts.ContactsService
}

type CaseChatHistoryServiceOption func(srv *CaseChatHistoryService) error

func NewCaseChatHistoryService(opts ...CaseChatHistoryServiceOption) *CaseChatHistoryService {
	srv := &CaseChatHistoryService{}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

func CaseChatHistoryServiceLogs(logs *slog.Logger) CaseChatHistoryServiceOption {
	return func(srv *CaseChatHistoryService) error {
		srv.logs = logs
		return nil
	}
}

func CaseChatHistoryServiceAuthN(client *auth.Client) CaseChatHistoryServiceOption {
	return func(srv *CaseChatHistoryService) error {
		srv.authN = client
		return nil
	}
}

func CaseChatHistoryServiceStore(store store.CatalogStore) CaseChatHistoryServiceOption {
	return func(srv *CaseChatHistoryService) error {
		srv.store = store
		return nil
	}
}

// TODO

func (srv *CaseChatHistoryService) bindNativeClient(ctx *app.Context) error {
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
func (srv *CaseChatHistoryService) GetCaseChatHistory(ctx context.Context, req *pb.GetCaseChatHistoryRequest, res *pb.ChatMessages) error {
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
			"case_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	scope = authN.Authorization.HasObjclass(scopeCases)
	if scope == nil {
		return errors.Forbidden(
			"case_chat.objclass.access.denied",
			"denied: require r:contacts access but not granted",
		) // (403) Forbidden
	}
	if !authN.Authorization.CanAccess(scope, auth.READ) {
		return errors.Forbidden(
			"case_chat.objclass.access.denied",
			"denied: require r:contacts access but not granted",
		) // (403) Forbidden
	}

	// endregion

	// region: ----- Validation -----
	// required!
	if req.GetCaseId() == "" {
		return errors.BadRequest(
			"chat.case_chat.get_case_chat_history.case_id.required",
			"chat.history( case.id: string! ); input: required",
		)
	}
	// endregion: ----- Validation -----

	// ------- Filter(s) ------- //
	search := app.SearchOptions{
		Context: *(authN),
		// ID:   []int64{},
		Term: req.Q,
		Filter: map[string]any{
			"case.id": req.GetCaseId(),
		},
		Access: auth.READ,
		Fields: req.Fields,
		Size:   -1,
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

	list, re := srv.store.GetHistory(&search)
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
