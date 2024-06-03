package chat

import (
	"context"
	"strconv"

	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	oauth "github.com/webitel/chat_manager/api/proto/auth"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/protos/gateway/contacts"
)

type ContactLinkingService struct {
	logs                  *zerolog.Logger
	authN                 *auth.Client
	channelStore          store.ChannelRepository
	clientStore           store.ClientRepository
	contactClient         contacts.ContactsService
	contactIMClientClient contacts.IMClientsService
}

type ContactsLinkingServiceOption func(srv *ContactLinkingService) error

func ContactLinkingServiceLogs(logs *zerolog.Logger) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.logs = logs
		return nil
	}
}

func ContactLinkingServiceAuthN(client *auth.Client) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.authN = client
		return nil
	}
}

func ContactLinkingServiceChannelStore(store store.ChannelRepository) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.channelStore = store
		return nil
	}
}
func ContactLinkingServiceClientStore(store store.ClientRepository) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.clientStore = store
		return nil
	}
}

func ContactsLinkingServiceContactClient(client contacts.ContactsService) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.contactClient = client
		return nil
	}
}

func ContactsLinkingServiceIMClient(client contacts.IMClientsService) ContactsLinkingServiceOption {
	return func(srv *ContactLinkingService) error {
		srv.contactIMClientClient = client
		return nil
	}
}

func NewContactLinkingService(opts ...ContactsLinkingServiceOption) *ContactLinkingService {
	srv := &ContactLinkingService{}
	for _, opt := range opts {
		opt(srv)
	}
	return srv
}

const scopeContacts = "contacts"

func (srv *ContactLinkingService) bindNativeClient(ctx *app.Context) error {
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

func (srv *ContactLinkingService) LinkContactToClient(ctx context.Context, req *pb.LinkContactToClientRequest, res *pb.EmptyResponse) error {

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
	scope = authN.Authorization.HasObjclass(scopeContacts)
	if scope == nil {
		return errors.Forbidden(
			"contacts.objclass.access.denied",
			"denied: require r:contacts access but not granted",
		) // (403) Forbidden
	}
	internal := false
	domainId := authN.Authorization.Creds.Dc

	// PERFORM
	channels, err := srv.channelStore.GetChannels(ctx, nil, &req.ConversationId, nil, &internal, nil, nil)
	if err != nil {
		return err
	}

	if len(channels) <= 0 {
		return errors.BadRequest("cmd.chat.link_contact_to_client.get_channel.no_channel", "no such conversation")
	}

	_, err = srv.contactIMClientClient.UpsertIMClients(ctx, &contacts.UpsertIMClientsRequest{
		ContactId: req.ContactId,
		DomainId:  domainId,
		Input: []*contacts.InputIMClient{
			{
				CreatedBy:    strconv.FormatInt(authN.Authorization.Creds.UserId, 10),
				ExternalUser: strconv.FormatInt(channels[0].UserID, 10),
				GatewayId:    channels[0].Connection.String,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (srv *ContactLinkingService) LinkContactToClientNA(ctx context.Context, req *pb.LinkContactToClientNARequest, res *pb.LinkContactToClientNAResponse) error {

	internal := false

	// PERFORM
	channels, err := srv.channelStore.GetChannels(ctx, nil, &req.ConversationId, nil, &internal, nil, nil)
	if err != nil {
		return err
	}

	if len(channels) <= 0 {
		return errors.BadRequest("cmd.chat.link_contact_to_client_no_auth.get_channel.no_channel", "no such conversation")
	}

	_, err = srv.contactIMClientClient.UpsertIMClients(ctx, &contacts.UpsertIMClientsRequest{
		ContactId: req.ContactId,
		DomainId:  channels[0].DomainID,
		Input: []*contacts.InputIMClient{
			{
				ExternalUser: strconv.FormatInt(channels[0].UserID, 10),
				GatewayId:    channels[0].Connection.String,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (srv *ContactLinkingService) CreateContactFromConversation(ctx context.Context, req *pb.CreateContactFromConversationRequest, res *pb.Lookup) error {
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
	// endregion: ----- Authorization -----
	internal := false
	domainId := authN.Authorization.Creds.Dc
	var (
		externalUserId int64
	)

	var (
		convertLookup = func(input *pb.Lookup) *contacts.Lookup {
			if input == nil {
				return nil
			}
			return &contacts.Lookup{
				Id:   strconv.FormatInt(input.Id, 10),
				Name: input.Name,
			}
		}
		coalesce = func(strings ...string) string {
			for _, s := range strings {
				if s != "" {
					return s
				}
			}
			return ""
		}
	)

	// region: ----- Perform -----
	active := true
	channels, err := srv.channelStore.GetChannels(ctx, nil, &req.ConversationId, nil, &internal, nil, &active)
	if err != nil {
		return err
	}

	if len(channels) <= 0 {
		return errors.BadRequest("cmd.chat.create_contact_from_conversation.get_channel.no_channel", "no channels found")
	}
	channel := channels[0]
	externalUserId = channels[0].UserID

	inputContact := &contacts.InputContact{
		Name: &contacts.InputName{
			GivenName: req.Name,
		},
		Timezones: []*contacts.InputTimezone{{
			Timezone: convertLookup(req.Timezone),
		}},
		Managers: []*contacts.InputManager{{
			User: convertLookup(req.Owner),
		}},
		About: req.Description,
	}
	for _, s := range req.Label {
		inputContact.Labels = append(inputContact.Labels, &contacts.InputLabel{Label: s})
	}

	// TODO: Everytime Api called contact will be created!
	creationResp, err := srv.contactClient.CreateContact(ctx, &contacts.InputContactRequest{
		Input: inputContact,
	})
	if err != nil {
		return err
	}

	_, err = srv.contactIMClientClient.UpsertIMClients(ctx, &contacts.UpsertIMClientsRequest{
		ContactId: creationResp.Id,
		DomainId:  domainId,
		Input: []*contacts.InputIMClient{
			{
				CreatedBy:    strconv.FormatInt(authN.Authorization.Creds.UserId, 10),
				ExternalUser: strconv.FormatInt(externalUserId, 10),
				GatewayId:    channel.Connection.String,
				Protocol:     channel.Type,
			},
		},
	})
	if err != nil {
		srv.contactClient.DeleteContact(ctx, &contacts.DeleteContactRequest{
			Etag: creationResp.Id,
		})
		return err
	}
	id, err := strconv.ParseInt(creationResp.Id, 10, 64)
	if err != nil {
		return err
	}

	res.Id = id
	res.Name = coalesce(creationResp.Name.CommonName, creationResp.Name.GivenName, creationResp.Name.FamilyName)

	return nil
}
