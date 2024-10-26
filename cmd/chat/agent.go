package chat

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	oauth "github.com/webitel/chat_manager/api/proto/auth"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
)

type AgentChatsService struct {
	logs         *zerolog.Logger
	authN        *auth.Client
	catalogStore store.CatalogStore
}

type AgentChatServiceOption func(srv *AgentChatsService) error

func AgentChatServiceLogs(logs *zerolog.Logger) AgentChatServiceOption {
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

func AgentChatServiceConversationStore(store store.CatalogStore) AgentChatServiceOption {
	return func(srv *AgentChatsService) error {
		srv.catalogStore = store
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
			"chat.objclass.access.denied",
			"denied: require r:chats access but not granted",
		) // (403) Forbidden
	}
	// endregion

	// Prepare SELECT request
	search := app.SearchOptions{
		Context: *(authN),
		Term:    req.Q,
		Access:  auth.READ,
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
					"close_cause",
					"queue",
					"context",
				},
				// extra
				[]string{
					"members",
				},
			),
		),
		Size: int(req.GetSize()),
		Page: int(req.GetPage()),
	}
	// SELF related ONLY (!)
	search.FilterAND("self", authN.Creds.GetUserId())
	// Only closed
	onlyClosed := req.GetOnlyClosed()
	search.FilterAND("online", !onlyClosed)
	// Only for current day
	currentTime := app.CurrentTime()
	year, month, day := currentTime.Date()
	location := currentTime.Location()
	startOfTheDay := time.Date(year, month, day, 0, 0, 0, 0, location)
	search.FilterAND("date", &pb.Timerange{Since: startOfTheDay.UnixMilli(), Until: currentTime.UnixMilli()})
	//userId := authN.Creds.GetUserId()
	resultingChats := pb.ChatDialogs{}
	err = srv.catalogStore.GetDialogs(&search, &resultingChats)
	if err != nil {
		return err
	}

	for _, conv := range resultingChats.Data {
		var unprocessedClose bool
		if v, ok := conv.Context[store.ChatNeedsProcessingVariable]; ok {
			raw, err := strconv.ParseBool(v)
			if err == nil {
				unprocessedClose = raw
			}
		}
		agentChat := &pb.AgentChat{
			Id:               conv.Id,
			Title:            conv.Title,
			StartedAt:        conv.Started,
			ClosedAt:         conv.Closed,
			CloseReason:      conv.ClosedCause,
			Gateway:          conv.Via,
			LastMessage:      conv.Message,
			Queue:            conv.Queue,
			UnprocessedClose: unprocessedClose,
		}
		res.Items = append(res.Items, agentChat)
	}
	res.Page = resultingChats.Page
	res.Next = resultingChats.Next
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
	affected, err := srv.catalogStore.MarkChatAsProcessed(ctx, request.ChatId, authN.Creds.GetUserId())
	if err != nil {
		return errors.New("chat.agent.mark_chat_processed.storage.error", err.Error(), http.StatusInternalServerError)
	}
	if affected == 0 {
		return errors.New("chat.agent.mark_chat_processed.no_rows_affected.error", "user didn't take action in the given conversation or wrong conversation id", http.StatusInternalServerError)
	}
	return nil
}
