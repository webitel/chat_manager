package chat

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
	oauth "github.com/webitel/chat_manager/api/proto/auth"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/protos/gateway/contacts"
)

type AgentChatsService struct {
	logs          *slog.Logger
	authN         *auth.Client
	store         store.AgentChatStore
	contactClient contacts.ContactsService
}

type AgentChatServiceOption func(srv *AgentChatsService) error

func AgentChatServiceLogs(logs *slog.Logger) AgentChatServiceOption {
	return func(srv *AgentChatsService) error {
		srv.logs = logs
		return nil
	}
}

func AgentChatServiceAuthN(client *auth.Client) AgentChatServiceOption {
	return func(srv *AgentChatsService) error {
		srv.authN = client
		return nil
	}
}

func AgentChatServiceConversationStore(store store.AgentChatStore) AgentChatServiceOption {
	return func(srv *AgentChatsService) error {
		srv.store = store
		return nil
	}
}

func AgentChatServiceContactClient(client contacts.ContactsService) AgentChatServiceOption {
	return func(srv *AgentChatsService) error {
		srv.contactClient = client
		return nil
	}
}

func NewAgentChatService(opts ...AgentChatServiceOption) *AgentChatsService {
	srv := &AgentChatsService{}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

//const scopeContacts = "contacts"

func (srv *AgentChatsService) bindNativeClient(ctx *app.Context) error {
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

func (srv *AgentChatsService) GetAgentChats(ctx context.Context, req *pb.GetAgentChatsRequest, res *pb.GetAgentChatsResponse) error {

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
	// wrapped
	// ctx = authN.Context
	// endregion: ----- Authentication -----

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"agent_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	if !authN.Authorization.CanAccess(scope, auth.READ) {
		return errors.Forbidden(
			"agent_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	var contactAccess bool
	scope = authN.Authorization.HasObjclass(scopeContacts)
	if scope != nil {
		if authN.Authorization.CanAccess(scope, auth.READ) {
			contactAccess = true
		}
	}
	// endregion

	fields := app.FieldsFunc(
		req.Fields,
		app.SelectFields(
			// default
			[]string{
				"id",
				"title",
				"gateway",
				"last_message",
				"created_at",
				"closed_at",
				"closed_cause",
				"needs_processing",
				"queue",
			},
			nil,
		),
	)
	if contactAccess {
		fields = append(fields, "contact")
	}
	search := app.SearchOptions{
		Context: *(authN),
		Term:    req.Q,
		Access:  auth.READ,
		Fields:  fields,
		Size:    int(req.GetSize()),
		Page:    int(req.GetPage()),
	}
	// only with current agent
	search.FilterAND("agent", authN.Creds.GetUserId())
	// Only closed
	onlyClosed := req.GetOnlyClosed()
	search.FilterAND("closed", &onlyClosed)
	onlyUnprocessed := req.GetOnlyUnprocessed()
	search.FilterAND("unprocessed", &onlyUnprocessed)
	// Only for current day
	currentTime := app.CurrentTime()
	year, month, day := currentTime.Date()
	location := currentTime.Location()
	startOfTheDay := time.Date(year, month, day, 0, 0, 0, 0, location)
	search.FilterAND("date", &pb.Timerange{Since: startOfTheDay.UnixMilli(), Until: currentTime.UnixMilli()})
	err = srv.store.GetAgentChats(&search, res)
	if err != nil {
		return err
	}
	if contactAccess {
		srv.fillContactsEtag(ctx, res)
	}
	if res.Page != 0 {
		res.Page = int32(search.GetPage())
	}
	return nil
}

// fillContactsEtag sets contact.etag on each chat by fetching it from the
// contacts service in a single batch 
func (srv *AgentChatsService) fillContactsEtag(ctx context.Context, res *pb.GetAgentChatsResponse) {
	if srv.contactClient == nil {
		return
	}
	// unique contact ids present on the page
	ids := make([]string, 0, len(res.GetItems()))
	seen := make(map[string]struct{}, len(res.GetItems()))
	for _, item := range res.GetItems() {
		id := item.GetContact().GetId()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	// SearchContacts caps the result page at 32; query in chunks.
	const chunkSize = 30
	etags := make(map[string]string, len(ids))
	for off := 0; off < len(ids); off += chunkSize {
		end := off + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[off:end]
		list, err := srv.contactClient.SearchContacts(ctx, &contacts.SearchContactsRequest{
			Id:     chunk,
			Fields: []string{"id", "etag"},
			Size:   int32(len(chunk)),
		})
		if err != nil {
			srv.logs.Warn("agent_chats: fetch contacts etag",
				slog.Any("error", err),
			)
			return // graceful: leave etag empty
		}
		for _, c := range list.GetData() {
			etags[c.GetId()] = c.GetEtag()
		}
	}
	for _, item := range res.GetItems() {
		if c := item.GetContact(); c != nil {
			if etag, ok := etags[c.GetId()]; ok {
				c.Etag = etag
			}
		}
	}
}

func (srv *AgentChatsService) GetAgentChatsCounter(ctx context.Context, req *pb.GetAgentChatsCounterRequest, res *pb.GetAgentChatsCounterResponse) error {

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

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"agent_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	if !authN.Authorization.CanAccess(scope, auth.READ) {
		return errors.Forbidden(
			"agent_chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	// endregion
	search := app.SearchOptions{
		Context: *(authN),
		Access:  auth.READ,
	}
	// only with current agent
	// required for API security
	search.FilterAND("agent", authN.Creds.GetUserId())

	// Only closed
	onlyClosed := req.GetOnlyClosed()
	search.FilterAND("closed", &onlyClosed)

	onlyUnprocessed := req.GetOnlyUnprocessed()
	search.FilterAND("unprocessed", &onlyUnprocessed)

	// Only for current day
	currentTime := app.CurrentTime()
	year, month, day := currentTime.Date()
	location := currentTime.Location()
	startOfTheDay := time.Date(year, month, day, 0, 0, 0, 0, location)
	search.FilterAND("date", &pb.Timerange{Since: startOfTheDay.UnixMilli(), Until: currentTime.UnixMilli()})

	count, err := srv.store.GetAgentChatsCounter(&search)
	if err != nil {
		return err
	}

	res.Count = int32(count)

	return nil
}

func (srv *AgentChatsService) MarkChatProcessed(ctx context.Context, request *pb.MarkChatProcessedRequest, response *pb.MarkChatProcessedResponse) error {
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

	// region: ----- Authorization -----
	scope := authN.Authorization.HasObjclass(scopeChats)
	if scope == nil {
		return errors.Forbidden(
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	// endregion

	// database logic
	affected, err := srv.store.MarkChatAsProcessed(ctx, request.ChatId, authN.Creds.GetUserId())
	if err != nil {
		return errors.New("chat.agent.mark_chat_processed.storage.error", err.Error(), http.StatusInternalServerError)
	}
	if affected == 0 {
		return errors.New("chat.agent.mark_chat_processed.no_rows_affected.error", "user didn't take action in the given conversation or wrong conversation id", http.StatusInternalServerError)
	}
	return nil
}
