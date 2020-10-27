package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/internal/auth"
	cache "github.com/webitel/chat_manager/internal/chat_cache"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	pbbot "github.com/webitel/protos/bot"
	pb "github.com/webitel/protos/chat"

	"github.com/jmoiron/sqlx"
	"github.com/micro/go-micro/v2/errors"
	"github.com/rs/zerolog"
)

type Service interface {
	GetConversationByID(ctx context.Context, req *pb.GetConversationByIDRequest, res *pb.GetConversationByIDResponse) error
	GetConversations(ctx context.Context, req *pb.GetConversationsRequest, res *pb.GetConversationsResponse) error
	GetProfileByID(ctx context.Context, req *pb.GetProfileByIDRequest, res *pb.GetProfileByIDResponse) error
	GetProfiles(ctx context.Context, req *pb.GetProfilesRequest, res *pb.GetProfilesResponse) error
	CreateProfile(ctx context.Context, req *pb.CreateProfileRequest, res *pb.CreateProfileResponse) error
	UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest, res *pb.UpdateProfileResponse) error
	DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest, res *pb.DeleteProfileResponse) error
	GetHistoryMessages(ctx context.Context, req *pb.GetHistoryMessagesRequest, res *pb.GetHistoryMessagesResponse) error

	SendMessage(ctx context.Context, req *pb.SendMessageRequest, res *pb.SendMessageResponse) error
	StartConversation(ctx context.Context, req *pb.StartConversationRequest, res *pb.StartConversationResponse) error
	CloseConversation(ctx context.Context, req *pb.CloseConversationRequest, res *pb.CloseConversationResponse) error
	JoinConversation(ctx context.Context, req *pb.JoinConversationRequest, res *pb.JoinConversationResponse) error
	LeaveConversation(ctx context.Context, req *pb.LeaveConversationRequest, res *pb.LeaveConversationResponse) error
	InviteToConversation(ctx context.Context, req *pb.InviteToConversationRequest, res *pb.InviteToConversationResponse) error
	DeclineInvitation(ctx context.Context, req *pb.DeclineInvitationRequest, res *pb.DeclineInvitationResponse) error
	WaitMessage(ctx context.Context, req *pb.WaitMessageRequest, res *pb.WaitMessageResponse) error
	CheckSession(ctx context.Context, req *pb.CheckSessionRequest, res *pb.CheckSessionResponse) error
	UpdateChannel(ctx context.Context, req *pb.UpdateChannelRequest, res *pb.UpdateChannelResponse) error
}

type chatService struct {
	repo          pg.Repository
	log           *zerolog.Logger
	flowClient    flow.Client
	authClient    auth.Client
	botClient     pbbot.BotService
	storageClient pbstorage.FileService
	chatCache     cache.ChatCache
	eventRouter   event.Router
}

func NewChatService(
	repo pg.Repository,
	log *zerolog.Logger,
	flowClient flow.Client,
	authClient auth.Client,
	botClient pbbot.BotService,
	storageClient pbstorage.FileService,
	chatCache cache.ChatCache,
	eventRouter event.Router,
) Service {
	return &chatService{
		repo,
		log,
		flowClient,
		authClient,
		botClient,
		storageClient,
		chatCache,
		eventRouter,
	}
}

func (s *chatService) UpdateChannel(
	ctx context.Context,
	req *pb.UpdateChannelRequest,
	res *pb.UpdateChannelResponse,
) error {
	channel, err := s.repo.CheckUserChannel(ctx, req.GetChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if channel == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}
	updatedAt, err := s.repo.UpdateChannel(ctx, req.GetChannelId())
	if err != nil {
		return err
	}
	if err := s.eventRouter.SendUpdateChannel(channel, updatedAt); err != nil {
		return err
	}
	return nil
}

func (s *chatService) SendMessage(
	ctx context.Context,
	req *pb.SendMessageRequest,
	res *pb.SendMessageResponse,
) error {
	s.log.Trace().
		Str("channel_id", req.GetChannelId()).
		Str("conversation_id", req.GetConversationId()).
		Bool("from_flow", req.GetFromFlow()).
		Int64("auth_user_id", req.GetAuthUserId()).
		Msg("send message")
	if req.GetFromFlow() {
		conversationID := req.GetConversationId()
		message := &pg.Message{
			Type:           "text",
			ConversationID: conversationID,
			Text: sql.NullString{
				req.GetMessage().GetText(),
				true,
			},
		}
		resErrorsChan := make(chan error, 2)
		go func() {
			if err := s.repo.CreateMessage(ctx, message); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		go func() {
			if err := s.eventRouter.RouteMessageFromFlow(&conversationID, req.GetMessage()); err != nil {
				if err := s.flowClient.CloseConversation(conversationID); err != nil {
					s.log.Error().Msg(err.Error())
				}
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		for i := 0; i < 2; i++ {
			if err := <-resErrorsChan; err != nil {
				s.log.Error().Msg(err.Error())
				return err
			}
		}
		return nil
	}

	channel, err := s.repo.CheckUserChannel(ctx, req.GetChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if channel == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}

	message := &pg.Message{
		Type: "text",
		ChannelID: sql.NullString{
			channel.ID,
			true,
		},
		ConversationID: channel.ConversationID,
		Text: sql.NullString{
			req.GetMessage().GetText(),
			true,
		},
	}
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	reqMessage := &pb.Message{
		Id:   message.ID,
		Type: message.Type,
		Value: &pb.Message_Text{
			Text: message.Text.String,
		},
	}

	sent, err := s.eventRouter.RouteMessage(channel, reqMessage)
	if err != nil {
		s.log.Warn().Msg(err.Error())
		return err
	}
	if !channel.Internal && !sent {
		if s.flowClient.SendMessage(channel.ConversationID, reqMessage); err != nil {
			return err
		}
	}
	return nil
}

func (s *chatService) StartConversation(
	ctx context.Context,
	req *pb.StartConversationRequest,
	res *pb.StartConversationResponse,
) error {
	s.log.Trace().
		Int64("domain_id", req.GetDomainId()).
		Str("user.connection", req.GetUser().GetConnection()).
		Str("user.type", req.GetUser().GetType()).
		Int64("user.id", req.GetUser().GetUserId()).
		Str("username", req.GetUsername()).
		Bool("user.internal", req.GetUser().GetInternal()).
		Msg("start conversation")
	channel := &pg.Channel{
		Type: req.GetUser().GetType(),
		// ConversationID: conversation.ID,
		UserID: req.GetUser().GetUserId(),
		Connection: sql.NullString{
			req.GetUser().GetConnection(),
			true,
		},
		Internal: req.GetUser().GetInternal(),
		DomainID: req.GetDomainId(),
		Name:     req.GetUsername(),
	}
	conversation := &pg.Conversation{
		DomainID: req.GetDomainId(),
	}
	if err := s.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := s.repo.CreateConversationTx(ctx, tx, conversation); err != nil {
			return err
		}
		channel.ConversationID = conversation.ID
		if err := s.repo.CreateChannelTx(ctx, tx, channel); err != nil {
			return err
		}
		res.ConversationId = conversation.ID
		res.ChannelId = channel.ID
		return nil
	}); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if !req.GetUser().GetInternal() {
		profileID, err := strconv.ParseInt(req.GetUser().GetConnection(), 10, 64)
		if err != nil {
			return err
		}
		err = s.flowClient.Init(conversation.ID, profileID, req.GetDomainId(), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *chatService) CloseConversation(
	ctx context.Context,
	req *pb.CloseConversationRequest,
	res *pb.CloseConversationResponse,
) error {
	s.log.Trace().
		Str("conversation_id", req.GetConversationId()).
		Str("cause", req.GetCause()).
		Str("closer_channel_id", req.GetCloserChannelId()).
		Msg("close conversation")
	conversationID := req.GetConversationId()
	if conversationID == "" {
		return errors.BadRequest("conversation_id not found", "")
	}
	if req.FromFlow {
		// s.chatCache.DeleteCachedMessages(conversationID)
		resErrorsChan := make(chan error, 4)
		go func() {
			if err := s.chatCache.DeleteConfirmation(conversationID); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		go func() {
			if err := s.chatCache.DeleteConversationNode(conversationID); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		go func() {
			if err := s.eventRouter.RouteCloseConversationFromFlow(&conversationID, req.GetCause()); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		go func() {
			if err := s.closeConversation(ctx, &conversationID); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		for i := 0; i < 4; i++ {
			if err := <-resErrorsChan; err != nil {
				s.log.Error().Msg(err.Error())
				return err
			}
		}
		return nil
	}
	closerChannel, err := s.repo.CheckUserChannel(ctx, req.GetCloserChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if closerChannel == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}
	resErrorsChan := make(chan error, 3)
	go func() {
		if err := s.eventRouter.RouteCloseConversation(closerChannel, req.GetCause()); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	go func() {
		if !closerChannel.Internal || closerChannel.FlowBridge {
			if err := s.flowClient.CloseConversation(closerChannel.ConversationID); err != nil {
				resErrorsChan <- err
				return
			}
		}
		resErrorsChan <- nil
	}()
	go func() {
		if err := s.closeConversation(ctx, &conversationID); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	for i := 0; i < 3; i++ {
		if err := <-resErrorsChan; err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
	}
	return nil
}

func (s *chatService) JoinConversation(
	ctx context.Context,
	req *pb.JoinConversationRequest,
	res *pb.JoinConversationResponse,
) error {
	s.log.Trace().
		Str("invite_id", req.GetInviteId()).
		Msg("join conversation")
	invite, err := s.repo.GetInviteByID(ctx, req.GetInviteId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if invite == nil || invite.UserID != req.GetAuthUserId() {
		s.log.Warn().Msg("invitation not found")
		return errors.BadRequest("invitation not found", "")
	}
	user, err := s.repo.GetWebitelUserByID(ctx, invite.UserID)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if user == nil {
		s.log.Warn().Msg("user not found")
		return errors.BadRequest("user not found", "")
	}
	channel := &pg.Channel{
		Type:           "webitel",
		Internal:       true,
		ConversationID: invite.ConversationID,
		UserID:         invite.UserID,
		DomainID:       invite.DomainID,
		Name:           user.Name,
	}
	if invite.InviterChannelID == (sql.NullString{}) {
		channel.FlowBridge = true
	}
	if err := s.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := s.repo.CreateChannelTx(ctx, tx, channel); err != nil {
			return err
		}
		if err := s.repo.CloseInviteTx(ctx, tx, req.GetInviteId()); err != nil {
			return err
		}
		res.ChannelId = channel.ID
		return nil
	}); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if err := s.eventRouter.RouteJoinConversation(channel, &invite.ConversationID); err != nil {
		s.log.Warn().Msg(err.Error())
		return err
	}
	return nil
}

func (s *chatService) LeaveConversation(
	ctx context.Context,
	req *pb.LeaveConversationRequest,
	res *pb.LeaveConversationResponse,
) error {
	channelID := req.GetChannelId()
	conversationID := req.GetConversationId()
	s.log.Trace().
		Str("channel_id", channelID).
		Str("conversation_id", conversationID).
		Int64("auth_user_id", req.GetAuthUserId()).
		Msg("leave conversation")
	channel, err := s.repo.CheckUserChannel(ctx, req.GetChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if channel == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}
	ch, err := s.repo.CloseChannel(ctx, channelID)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	resErrorsChan := make(chan error, 2)
	go func() {
		if ch.FlowBridge {
			if err := s.flowClient.BreakBridge(conversationID, flow.LeaveConversationCause); err != nil {
				resErrorsChan <- err
				return
			}
		}
		resErrorsChan <- nil
	}()
	go func() {
		if err := s.eventRouter.RouteLeaveConversation(ch, &conversationID); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	for i := 0; i < 2; i++ {
		if err := <-resErrorsChan; err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
	}
	return nil
}

func (s *chatService) InviteToConversation(
	ctx context.Context,
	req *pb.InviteToConversationRequest,
	res *pb.InviteToConversationResponse,
) error {
	// _, err := s.repo.GetChannelByID(ctx, req.InviterChannelId)
	// if err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return err
	// }
	s.log.Trace().
		Str("user.connection", req.GetUser().GetConnection()).
		Str("user.type", req.GetUser().GetType()).
		Int64("user.id", req.GetUser().GetUserId()).
		Bool("user.internal", req.GetUser().GetInternal()).
		Str("conversation_id", req.GetConversationId()).
		Str("inviter_channel_id", req.GetInviterChannelId()).
		Int64("domain_id", req.GetDomainId()).
		Int64("timeout_sec", req.GetTimeoutSec()).
		Int64("auth_user_id", req.GetAuthUserId()).
		Bool("from_flow", req.GetFromFlow()).
		Msg("invite to conversation")
	if !req.GetFromFlow() &&
		(req.GetInviterChannelId() == "" || req.GetAuthUserId() == 0) {
		s.log.Error().Msg("failed auth")
		return errors.BadRequest("failed auth", "")
	}
	domainID := req.GetDomainId()
	invite := &pg.Invite{
		ConversationID: req.GetConversationId(),
		UserID:         req.GetUser().GetUserId(),
		TimeoutSec:     req.GetTimeoutSec(),
		DomainID:       domainID,
	}
	if req.GetInviterChannelId() != "" {
		channel, err := s.repo.CheckUserChannel(ctx, req.GetInviterChannelId(), req.GetAuthUserId())
		if err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
		if channel == nil {
			s.log.Warn().Msg("channel not found")
			return errors.BadRequest("channel not found", "")
		}
		invite.InviterChannelID = sql.NullString{
			req.GetInviterChannelId(),
			true,
		}
	}
	if err := s.repo.CreateInvite(ctx, invite); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	conversation, err := s.repo.GetConversationByID(ctx, req.GetConversationId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	resErrorsChan := make(chan error, 2)
	go func() {
		if err := s.eventRouter.SendInviteToWebitelUser(transformConversationFromRepoModel(conversation), invite); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	go func() {
		if err := s.eventRouter.RouteInvite(&invite.ConversationID, &invite.UserID); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	for i := 0; i < 2; i++ {
		if err := <-resErrorsChan; err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
	}
	if req.GetTimeoutSec() != 0 {
		go func() {
			time.Sleep(time.Second * time.Duration(req.GetTimeoutSec()))
			if val, err := s.repo.GetInviteByID(context.Background(), invite.ID); err != nil {
				s.log.Error().Msg(err.Error())
			} else if val != nil {
				s.log.Trace().
					Str("invite_id", invite.ID).
					Int64("user_id", invite.UserID).
					Str("conversation_id", invite.ConversationID).
					Msg("autodecline invitation")
				if req.GetInviterChannelId() == "" {
					if err := s.flowClient.BreakBridge(req.GetConversationId(), flow.TimeoutCause); err != nil {
						s.log.Error().Msg(err.Error())
					}
				}
				if err := s.eventRouter.SendDeclineInviteToWebitelUser(&domainID, &invite.ConversationID, &invite.UserID, &invite.ID); err != nil {
					s.log.Error().Msg(err.Error())
				}
				if err := s.repo.CloseInvite(context.Background(), val.ID); err != nil {
					s.log.Error().Msg(err.Error())
				}
			}

		}()
	}
	res.InviteId = invite.ID
	return nil
}

func (s *chatService) DeclineInvitation(
	ctx context.Context,
	req *pb.DeclineInvitationRequest,
	res *pb.DeclineInvitationResponse,
) error {
	userID := req.GetAuthUserId()
	conversationID := req.GetConversationId()
	s.log.Trace().
		Str("invite_id", req.GetInviteId()).
		Str("conversation_id", conversationID).
		Int64("auth_user_id", userID).
		Msg("decline invitation")
	invite, err := s.repo.GetInviteByID(ctx, req.GetInviteId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if invite == nil && invite.UserID != req.GetAuthUserId() {
		return errors.BadRequest("invite not found", "")
	}
	resErrorsChan := make(chan error, 3)
	go func() {
		if invite.InviterChannelID == (sql.NullString{}) {
			if err := s.flowClient.BreakBridge(invite.ConversationID, flow.DeclineInvitationCause); err != nil {
				resErrorsChan <- err
				return
			}
		}
		resErrorsChan <- nil
	}()
	go func() {
		if err := s.repo.CloseInvite(ctx, req.GetInviteId()); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	go func() {
		if err := s.eventRouter.RouteDeclineInvite(&invite.UserID, &invite.ConversationID); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	for i := 0; i < 3; i++ {
		if err := <-resErrorsChan; err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
	}
	return nil
}

func (s *chatService) WaitMessage(ctx context.Context, req *pb.WaitMessageRequest, res *pb.WaitMessageResponse) error {
	s.log.Debug().
		Str("conversation_id", req.GetConversationId()).
		Str("confirmation_id", req.GetConfirmationId()).
		Msg("accept confirmation")
	// cachedMessages, err := s.chatCache.ReadCachedMessages(req.GetConversationId())
	// if err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return err
	// }
	// if cachedMessages != nil {
	// 	messages := make([]*pb.Message, 0, len(cachedMessages))
	// 	var tmp *pb.Message
	// 	var err error
	// 	s.log.Info().Msg("send cached messages")
	// 	for _, m := range cachedMessages {
	// 		err = proto.Unmarshal(m.Value, tmp)
	// 		if err != nil {
	// 			s.log.Error().Msg(err.Error())
	// 			return err
	// 		}
	// 		messages = append(messages, tmp)
	// 		s.chatCache.DeleteCachedMessage(m.Key)
	// 	}
	// 	res.Messages = messages
	// 	s.chatCache.DeleteConfirmation(req.GetConversationId())
	// 	res.TimeoutSec = int64(timeout)
	// 	return nil
	// }
	if err := s.chatCache.WriteConfirmation(req.GetConversationId(), []byte(req.GetConfirmationId())); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.TimeoutSec = int64(timeout)
	return nil
}

func (s *chatService) CheckSession(ctx context.Context, req *pb.CheckSessionRequest, res *pb.CheckSessionResponse) error {
	s.log.Trace().
		Str("external_id", req.GetExternalId()).
		Int64("profile_id", req.GetProfileId()).
		Msg("check session")
	client, err := s.repo.GetClientByExternalID(ctx, req.GetExternalId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if client == nil {
		client, err = s.createClient(ctx, req)
		if err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
		res.ClientId = client.ID
		res.Exists = false
		return nil
	}
	profileStr := strconv.Itoa(int(req.GetProfileId()))
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	externalBool := false
	channel, err := s.repo.GetChannels(ctx, &client.ID, nil, &profileStr, &externalBool, nil)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if len(channel) > 0 {
		res.Exists = true
		res.ChannelId = channel[0].ID
		res.ClientId = client.ID
	} else {
		res.Exists = false
		res.ClientId = client.ID
	}
	return nil
}

func (s *chatService) GetConversationByID(ctx context.Context, req *pb.GetConversationByIDRequest, res *pb.GetConversationByIDResponse) error {
	s.log.Trace().
		Str("conversation_id", req.GetId()).
		Msg("get conversation by id")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	conversation, err := s.repo.GetConversationByID(ctx, req.GetId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if conversation == nil {
		return nil
	}
	res.Item = transformConversationFromRepoModel(conversation)
	return nil
}

func (s *chatService) GetConversations(ctx context.Context, req *pb.GetConversationsRequest, res *pb.GetConversationsResponse) error {
	s.log.Trace().
		Str("conversation_id", req.GetId()).
		Msg("get conversations")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	conversations, err := s.repo.GetConversations(
		ctx,
		req.GetId(),
		req.GetSize(),
		req.GetPage(),
		req.GetFields(),
		req.GetSort(),
		req.GetDomainId(),
		req.GetActive(),
		req.GetUserId(),
	)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Items = transformConversationsFromRepoModel(conversations)
	return nil
}

func (s *chatService) CreateProfile(
	ctx context.Context,
	req *pb.CreateProfileRequest,
	res *pb.CreateProfileResponse) error {
	s.log.Trace().
		Str("name", req.GetItem().GetName()).
		Str("type", req.GetItem().GetType()).
		Int64("domain_id", req.GetItem().GetDomainId()).
		Int64("schema_id", req.GetItem().GetSchemaId()).
		Str("variables", fmt.Sprintf("%v", req.GetItem().GetVariables())).
		Msg("create profile")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	result, err := transformProfileToRepoModel(req.GetItem())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if err := s.repo.CreateProfile(ctx, result); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Item = req.Item
	res.Item.Id = result.ID

	addProfileReq := &pbbot.AddProfileRequest{
		Profile: res.Item,
	}
	if _, err := s.botClient.AddProfile(ctx, addProfileReq); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	return nil
}

func (s *chatService) DeleteProfile(
	ctx context.Context,
	req *pb.DeleteProfileRequest,
	res *pb.DeleteProfileResponse) error {
	s.log.Trace().
		Int64("profile_id", req.GetId()).
		Msg("delete profile")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profile, err := s.repo.GetProfileByID(ctx, req.GetId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	} else if profile == nil {
		return errors.BadRequest("profile not found", "")
	}
	if err := s.repo.DeleteProfile(ctx, req.GetId()); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	deleteProfileReq := &pbbot.DeleteProfileRequest{
		Id: req.GetId(),
	}
	if _, err := s.botClient.DeleteProfile(ctx, deleteProfileReq); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Item, err = transformProfileFromRepoModel(profile)
	return err
}

func (s *chatService) UpdateProfile(
	ctx context.Context,
	req *pb.UpdateProfileRequest,
	res *pb.UpdateProfileResponse) error {
	s.log.Trace().
		Str("update", "profile").
		Msgf("%v", req.GetItem())
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profile, err := transformProfileToRepoModel(req.GetItem())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if err := s.repo.UpdateProfile(ctx, profile); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Item, err = transformProfileFromRepoModel(profile)
	return err
}

func (s *chatService) GetProfiles(ctx context.Context, req *pb.GetProfilesRequest, res *pb.GetProfilesResponse) error {
	s.log.Trace().
		Str("type", req.GetType()).
		Int64("domain_id", req.GetDomainId()).
		Msg("get profiles")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profiles, err := s.repo.GetProfiles(
		ctx,
		req.GetId(),
		req.GetSize(),
		req.GetPage(),
		req.GetFields(),
		req.GetSort(),
		req.GetType(),
		req.GetDomainId(),
	)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	result, err := transformProfilesFromRepoModel(profiles)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Items = result
	return nil
}

func (s *chatService) GetProfileByID(ctx context.Context, req *pb.GetProfileByIDRequest, res *pb.GetProfileByIDResponse) error {
	s.log.Trace().
		Int64("profile_id", req.GetId()).
		Msg("get profile by id")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profile, err := s.repo.GetProfileByID(ctx, req.GetId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	result, err := transformProfileFromRepoModel(profile)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Item = result
	return nil
}

func (s *chatService) GetHistoryMessages(ctx context.Context, req *pb.GetHistoryMessagesRequest, res *pb.GetHistoryMessagesResponse) error {
	s.log.Trace().
		Str("conversation_id", req.GetConversationId()).
		Msg("get history")
	if err := s.authClient.MicroAuthentication(&ctx); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	messages, err := s.repo.GetMessages(
		ctx,
		req.GetId(),
		req.GetSize(),
		req.GetPage(),
		req.GetFields(),
		req.GetSort(),
		req.GetConversationId(),
	)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Items = transformMessagesFromRepoModel(messages)
	return nil
}
