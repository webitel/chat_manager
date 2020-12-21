package main

import (
	
	"fmt"
	"time"
	"strconv"
	"context"
	"database/sql"
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/jmoiron/sqlx"

	"github.com/micro/go-micro/v2/errors"
	"github.com/micro/go-micro/v2/metadata"

	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"
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
	eventRouter   event.Router
}

func NewChatService(
	repo pg.Repository,
	log *zerolog.Logger,
	flowClient flow.Client,
	authClient auth.Client,
	botClient pbbot.BotService,
	storageClient pbstorage.FileService,
	eventRouter event.Router,
) Service {
	return &chatService{
		repo,
		log,
		flowClient,
		authClient,
		botClient,
		storageClient,
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
		// Bool("from_flow", req.GetFromFlow()).
		Int64("auth_user_id", req.GetAuthUserId()).
		Msg("send message")

	// FROM: INTERNAL (?)
	servName := s.authClient.GetServiceName(&ctx)
	if servName == "workflow" {
		conversationID := req.GetConversationId()
		message := &pg.Message{
			ConversationID: conversationID,
		}

		if req.Message.File != nil{
			message.Type = "file"
				message.Text = sql.NullString{
					req.GetMessage().GetFile().Url,
					true,
				}

				body, err := json.Marshal(req.Message.GetFile())
				if err != nil {
					s.log.Error().Msg(err.Error())
				}

				err = message.Variables.Scan(body)
				if err!=nil{
					s.log.Error().Msg(err.Error())
				}
		}else{
			message.Type = "text"
			message.Text = sql.NullString{
				req.GetMessage().GetText(),
				true,
			}
		}

		// s.repo.CreateMessage(ctx, message)
		// s.eventRouter.RouteMessageFromFlow(&conversationID, req.GetMessage())

		// ----- PERFORM ---------------------------------
		// 1. Save historical .Message delivery
		err := s.repo.CreateMessage(ctx, message)
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to save message to history")
			return err
		}
		// 2. Broadcast given .Message to all related external chat members
		//    on behalf of internal, workflow service, channel request
		err = s.eventRouter.RouteMessageFromFlow(&conversationID, req.GetMessage())
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to broadcast /send message")
			return err
		}

		return nil

		// resErrorsChan := make(chan error, 2)
		// go func() {
		// 	if err := s.repo.CreateMessage(ctx, message); err != nil {
		// 		resErrorsChan <- err
		// 	} else {
		// 		resErrorsChan <- nil
		// 	}
		// }()
		// go func() {
		// 	if err := s.eventRouter.RouteMessageFromFlow(&conversationID, req.GetMessage()); err != nil {
		// 		if err := s.flowClient.CloseConversation(conversationID); err != nil {
		// 			s.log.Error().Msg(err.Error())
		// 		}
		// 		resErrorsChan <- err
		// 	} else {
		// 		resErrorsChan <- nil
		// 	}
		// }()
		// for i := 0; i < 2; i++ {
		// 	if err := <-resErrorsChan; err != nil {
		// 		s.log.Error().Msg(err.Error())
		// 		return err
		// 	}
		// }
		// return nil
	}

	// FROM: EXTERNAL (!)
	sender, err := s.repo.CheckUserChannel(ctx, req.GetChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if sender == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}
	// original message to be re- send
	recvMessage := req.Message
	// historical saved store.Message
	sendMessage := pg.Message{
		ChannelID: sql.NullString{
			sender.ID,
			true,
		},
		ConversationID: sender.ConversationID,
	}

	if recvMessage.File != nil{
		if !sender.Internal {
			fileMessaged := &pbstorage.UploadFileUrlRequest{
				DomainId: sender.DomainID,
				Name:     req.Message.GetFile().Name,
				Url:      req.Message.GetFile().Url,
				Uuid:     sender.ConversationID,
				Mime:     req.Message.GetFile().Mime,
			}
	
			res, err := s.storageClient.UploadFileUrl(context.Background(), fileMessaged)
			if err != nil {
				s.log.Error().Err(err).Msg("Failed sand message to UploadFileUrl")
				return err
			}
			
			recvMessage.File.Url = res.Url
			recvMessage.File.Id = res.Id
			recvMessage.File.Mime = res.Mime
			recvMessage.File.Size = res.Size
		}

		// shallowcopy
		saveFile := *(recvMessage.File)
		saveFile.Url = "" // sanitize .url path

		body, err := json.Marshal(saveFile)
		if err != nil {
			s.log.Error().Msg(err.Error())
		}

		sendMessage.Type = "file"
		err = sendMessage.Variables.Scan(body)
		if err!=nil{
			s.log.Error().Msg(err.Error())
		}
	} else {
		sendMessage.Type = "text"
		sendMessage.Text = sql.NullString{
			req.GetMessage().GetText(),
			true,
		}
	}

	if recvMessage.Variables != nil {
		sendMessage.Variables.Scan(recvMessage.Variables)
	}
	if err := s.repo.CreateMessage(ctx, &sendMessage); err != nil {
		s.log.Error().Err(err).Msg("Failed to store .SendMessage() history")
		return err
	}
	recvMessage.Id = sendMessage.ID
	// populate normalized value(s)
	// recvMessage = &pb.Message{
	// 	Id:   sendMessage.ID,
	// 	Type: sendMessage.Type,
	// 	Value: &pb.Message_Text{
	// 		Text: sendMessage.Text.String,
	// 	},
	// }
	// Broadcast text message to every other channel in the room, from channel as a sender !
	sent, err := s.eventRouter.RouteMessage(sender, recvMessage)
	if err != nil {
		s.log.Warn().Msg(err.Error())
		return err
	}
	// Otherwise, if NO-ONE in the room - route message to the chat-flow !
	if !sender.Internal && !sent {
		// err = s.flowClient.SendMessage(channel.ConversationID, reqMessage)
		err = s.flowClient.SendMessage(sender, recvMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartConversation starts NEW chat@bot(workflow/schema) session
// ON one side there will be req.Username with the start req.Message channel as initiator (leg: A)
// ON other side there will be flow_manager.schema (chat@bot) channel to communicate with
func (s *chatService) StartConversation(
	ctx context.Context,
	req *pb.StartConversationRequest,
	res *pb.StartConversationResponse,
) error {

	// TODO: keep track .sender.host to be able to respond to
	//       the same .sender service node for .this unique chat channel
	//       that will be created !

	// // FIXME: this is always invoked by webitel.chat.bot service ?
	// // Gathering metadata to identify start req.Message sender NEW channel !...
	md, _ := metadata.FromContext(ctx)
	senderHostname := md["Micro-From-Id"] // provider channel host !
	senderProvider := md["Micro-From-Service"] // provider channel type !
	if senderProvider != "webitel.chat.bot" {
		// LOG: this is the only case expected for now !..
	}

	s.log.Trace().
		Int64("domain_id", req.GetDomainId()).
		Str("user.connection", req.GetUser().GetConnection()).
		Str("user.type", req.GetUser().GetType()).
		Int64("user.id", req.GetUser().GetUserId()).
		Str("username", req.GetUsername()).
		Bool("user.internal", req.GetUser().GetInternal()).
		Msg("start conversation")

	channel := pg.Channel{
		Type: req.GetUser().GetType(),
		// ConversationID: conversation.ID,
		UserID: req.GetUser().GetUserId(),
		ServiceHost: sql.NullString{
			// senderProvider +"-"+ senderHostname,
			senderHostname, // contact/from: node-id
			true,
		},
		Connection: sql.NullString{
			req.GetUser().GetConnection(),
			true,
		},
		Internal: req.GetUser().GetInternal(),
		DomainID: req.GetDomainId(),
		Name:     req.GetUsername(),

		Properties: req.GetMessage().GetVariables(),
	}

	conversation := &pg.Conversation{
		DomainID: req.GetDomainId(),
	}

	if err := s.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := s.repo.CreateConversationTx(ctx, tx, conversation); err != nil {
			return err
		}
		channel.ConversationID = conversation.ID
		if err := s.repo.CreateChannelTx(ctx, tx, &channel); err != nil {
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
		// // // profileID, providerNode, err :=
		// // profileID, _, err := event.ContactProfileNode(req.GetUser().GetConnection())
		// // if err != nil {
		// // 	return err
		// // }
		// profileID, err := strconv.ParseInt(req.GetUser().GetConnection(), 10, 64)
		// if err != nil {
		// 	return err
		// }
		// err = s.flowClient.Init(conversation.ID, profileID, req.GetDomainId(), req.GetMessage())
		err := s.flowClient.Init(&channel, req.GetMessage())
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
	
	// FROM: INTERNAL (?)
	servName := s.authClient.GetServiceName(&ctx)
	if servName == "workflow" {
		// s.chatCache.DeleteCachedMessages(conversationID)
		if conversationID == "" {
			return errors.BadRequest("conversation_id not found", "")
		}

		// s.repo.DeleteConfirmation(conversationID)
		// s.repo.DeleteConversationNode(conversationID)
		// s.eventRouter.RouteCloseConversationFromFlow(&conversationID, req.GetCause())
		// s.closeConversation(ctx, &conversationID)

		// ----- PERFORM ---------------------------------
		// 1. Broadcast latest "Conversation Close" message
		//    on behalf of internal, workflow service, channel request
		err := s.eventRouter.RouteCloseConversationFromFlow(&conversationID, req.GetCause())
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to broadcast /close message")
			return err
		}
		// 2. Mark .conversation and all its related .channel members as "closed" !
		// NOTE: - delete: chat.confirmation; - delete: chat.flow.node
		err = s.closeConversation(ctx, &conversationID)
		if err != nil {
			s.log.Error().Err(err).Msg("Failed to close chat channels")
			return err
		}

		return nil
	}
	// EXTERNAL
	closerChannel, err := s.repo.CheckUserChannel(ctx, req.GetCloserChannelId(), req.GetAuthUserId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	// ensure: op-init channel FOUND(?)
	if closerChannel == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}
	// ensure: channel.converaation_id MATCH(?)
	if conversationID != "" {
		if conversationID != closerChannel.ConversationID {
			s.log.Warn().Str("error", "mismatch: channel.conversation_id").Msg("channel not found")
			return errors.BadRequest("channel not found", "")
		}
	}
	conversationID = closerChannel.ConversationID // resolved from DB
	
	// ----- PERFORM ---------------------------------
	// 1. Broadcast latest "Conversation Close" message
	err = s.eventRouter.RouteCloseConversation(closerChannel, req.GetCause())
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to broadcast /close message")
		return err
	}
	// 2. Send workflow channel .Break() message to stop chat.flow routine ...
	// FIXME: - delete: chat.confirmation; - delete: chat.flow.node
	err = s.flowClient.CloseConversation(conversationID)
	// err = s.flowClient.CloseConversation(closerChannel)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to break chat.flow routine")
		return err
	}
	// 3. Mark .conversation and all its related .channel members as "closed" !
	// NOTE: - delete: chat.confirmation; - delete: chat.flow.node
	err = s.closeConversation(ctx, &conversationID)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to close chat channels")
		return err
	}

	// +OK
	return nil
}

/*func (s *chatService) CloseConversation(
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
	servName := s.authClient.GetServiceName(&ctx)
	if servName == "workflow" {
		// s.chatCache.DeleteCachedMessages(conversationID)
		if conversationID == "" {
			return errors.BadRequest("conversation_id not found", "")
		}
		resErrorsChan := make(chan error, 4)
		go func() {
			if err := s.repo.DeleteConfirmation(conversationID); err != nil {
				resErrorsChan <- err
			} else {
				resErrorsChan <- nil
			}
		}()
		go func() {
			if err := s.repo.DeleteConversationNode(conversationID); err != nil {
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
	conversationID = closerChannel.ConversationID // resolved from DB
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
			if err := s.flowClient.CloseConversation(conversationID); err != nil {
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
}*/

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
	if user == nil || user.DomainID != invite.DomainID {
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
		JoinedAt:       sql.NullTime{
			Time:       time.Now().UTC(),
			Valid:      true,
		},
		Properties:     invite.Variables,
	}
	if !invite.InviterChannelID.Valid {
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
	
	userID := req.GetAuthUserId()
	channelID := req.GetChannelId()
	conversationID := req.GetConversationId()
	
	s.log.Trace().
		Int64("auth_user_id", userID).
		Str("channel_id", channelID).
		Str("conversation_id", conversationID).
		Msg("leave conversation")
	
	sender, err := s.repo.CheckUserChannel(ctx, channelID, userID)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	
	if sender == nil {
		s.log.Warn().Msg("channel not found")
		return errors.BadRequest("channel not found", "")
	}

	if conversationID != "" {
		if conversationID != sender.ConversationID {
			s.log.Warn().Msg("channel.conversation_id mismatch")
			return errors.BadRequest("channel.conversation_id mismatch", "")
		}
	}
	
	// ----- PERFORM ---------------------------------
	// 1. Mark given .channel.id as "closed" !
	ch, err := s.repo.CloseChannel(ctx, channelID)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	// parallel
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
		// Bool("from_flow", req.GetFromFlow()).
		Msg("invite to conversation")
	servName := s.authClient.GetServiceName(&ctx)
	if servName != "workflow" &&
		(req.GetInviterChannelId() == "" || req.GetAuthUserId() == 0) {
		s.log.Error().Msg("failed auth")
		return errors.BadRequest("failed auth", "")
	}
	domainID := req.GetDomainId()
	invite := &pg.Invite{
		UserID:         req.GetUser().GetUserId(),
		DomainID:       domainID,
		Variables:      req.GetVariables(),
		TimeoutSec:     req.GetTimeoutSec(),
		ConversationID: req.GetConversationId(),
	}
	if title := req.GetTitle(); title != "" {
		invite.Title = sql.NullString{
			String: title, Valid: true,
		}
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
	conversation, err := s.repo.GetConversations(ctx, req.GetConversationId(), 0, 0, nil, nil, 0, false, 0, 0)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if conversation == nil {
		s.log.Error().Msg("conversation not found")
		return errors.BadRequest("conversation not found", "")
	}
	resErrorsChan := make(chan error, 2)
	go func() {
		if err := s.eventRouter.SendInviteToWebitelUser(transformConversationFromRepoModel(conversation[0]), invite); err != nil {
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
	if invite == nil || invite.UserID != req.GetAuthUserId() {
		return errors.BadRequest("invite not found", "")
	}
	resErrorsChan := make(chan error, 3)
	go func() {
		if !invite.InviterChannelID.Valid {
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
	go func() {
		if err := s.eventRouter.SendDeclineInviteToWebitelUser(&invite.DomainID, &invite.ConversationID, &invite.UserID, &invite.ID); err != nil {
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
	if err := s.repo.WriteConfirmation(req.GetConversationId(), req.GetConfirmationId()); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.TimeoutSec = int64(timeout)
	return nil
}

// CheckSession performs:
// - Locate OR Create client contact
// - Identify whether exists channel for
//   requested chat-bot gateway profile.id
func (s *chatService) CheckSession(ctx context.Context, req *pb.CheckSessionRequest, res *pb.CheckSessionResponse) error {
	
	s.log.Trace().
		Str("external_id", req.GetExternalId()).
		Int64("profile_id", req.GetProfileId()).
		Msg("check session")
	
	contact, err := s.repo.GetClientByExternalID(ctx, req.GetExternalId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	
	if contact == nil {
		contact, err = s.createClient(ctx, req)
		if err != nil {
			s.log.Error().Msg(err.Error())
			return err
		}
		res.ClientId = contact.ID
		res.Exists = false
		return nil
	}
	
	// profileStr := strconv.Itoa(int(req.GetProfileId()))
	profileStr := strconv.FormatInt(req.GetProfileId(), 10)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	
	externalBool := false
	channels, err := s.repo.GetChannels(ctx, &contact.ID, nil, &profileStr, &externalBool, nil)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	
	if len(channels) != 0 {
		channel := channels[0]
		res.ClientId = contact.ID
		res.ChannelId = channel.ID
		res.Exists = channel.ID != ""
		res.Properties = channel.Properties
	} else {
		res.ClientId = contact.ID
		res.Exists = false
	}

	return nil
}

func (s *chatService) GetConversationByID(ctx context.Context, req *pb.GetConversationByIDRequest, res *pb.GetConversationByIDResponse) error {
	s.log.Trace().
		Str("conversation_id", req.GetId()).
		Msg("get conversation by id")
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	conversation, err := s.repo.GetConversations(ctx, req.GetId(), 0, 0, nil, nil, user.DomainID, false, 0, 0)
	//conversation, err := s.repo.GetConversationByID(ctx, req.GetId())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if conversation == nil {
		return nil
	}
	res.Item = transformConversationFromRepoModel(conversation[0])
	return nil
}

func (s *chatService) GetConversations(ctx context.Context, req *pb.GetConversationsRequest, res *pb.GetConversationsResponse) error {
	s.log.Trace().
		Str("conversation_id", req.GetId()).
		Msg("get conversations")
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
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
		user.DomainID, //req.GetDomainId(),
		req.GetActive(),
		req.GetUserId(),
		req.GetMessageSize(),
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
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if user.DomainID != req.GetItem().GetDomainId() {
		s.log.Error().Msg("invalid domain id")
		return errors.BadRequest("invalid domain id", "")
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
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profile, err := s.repo.GetProfileByID(ctx, req.GetId(), "")
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	} else if profile == nil || profile.DomainID != user.DomainID {
		return errors.BadRequest("profile not found", "")
	}
	if err := s.repo.DeleteProfile(ctx, req.GetId()); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	deleteProfileReq := &pbbot.DeleteProfileRequest{
		Id:    req.GetId(),
		UrlId: profile.UrlID,
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
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	profile, err := transformProfileToRepoModel(req.GetItem())
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	if profile.DomainID != user.DomainID {
		s.log.Error().Msg("invalid domain id")
		return errors.BadRequest("invalid domain id", "")
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
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	var domainID int64
	if user != nil {
		domainID = user.DomainID
	}
	profiles, err := s.repo.GetProfiles(
		ctx,
		req.GetId(),
		req.GetSize(),
		req.GetPage(),
		req.GetFields(),
		req.GetSort(),
		req.GetType(),
		domainID, //req.GetDomainId(),
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

	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	s.log.Trace().
		Int64("pid", req.GetId()).
		Str("uri", req.GetUri()).
		Msg("get profile")
	profile, err := s.repo.GetProfileByID(ctx, req.GetId(), req.GetUri())
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to get profile")
		return err
	}
	if profile == nil {

		s.log.Warn().Int64("pid", req.GetId()).Str("uri", req.GetUri()).
			Msg("Profile Not Found")

		return errors.NotFound(
			"chat.gateway.profile.not_found",
			"chat: gateway profile id=%d uri=/%s not found",
			req.GetId(), req.GetUri(),
		)
	}
	if user != nil && profile.DomainID != user.DomainID {
		s.log.Error().Msg("invalid domain id")
		return errors.BadRequest("invalid domain id", "")
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
	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
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
		user.DomainID,
		req.GetConversationId(),
	)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	res.Items = transformMessagesFromRepoModel(messages)
	return nil
}
