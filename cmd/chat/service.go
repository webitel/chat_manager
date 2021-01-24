package main

import (
	

	"fmt"
	"time"
	"path"
	"strconv"
	"strings"
	"context"
	"net/url"
	"net/http"
	"database/sql"
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/jmoiron/sqlx"

	// errs "github.com/pkg/errors"
	"github.com/micro/go-micro/v2/errors"
	"github.com/micro/go-micro/v2/broker"
	"github.com/micro/go-micro/v2/metadata"

	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/pkg/events"

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

	var (

		channelChatID = req.GetChannelId()
		channelFromID = req.GetAuthUserId()

		messageAt = req.GetReadUntil() // Implies last seen message.created_at date
		localtime = app.CurrentTime()
		readUntil = localtime // default: ALL
	)

	s.log.Trace().

		Str("channel_id",     channelChatID).
		Int64("auth_user_id", channelFromID).
		Int64("read_until",   messageAt).

		Msg("UPDATE Channel")

	// PERFORM find sender channel
	channel, err := s.repo.CheckUserChannel(
		ctx, channelChatID, channelFromID,
	)
	
	if err != nil {

		s.log.Error().
		
			Err(err).
			Str("chat-id", channelChatID).
			Int64("contact-id", channelFromID).

			Msg("FAILED Lookup Channel")

		return err
	}

	if channel == nil {

		s.log.Warn().

			Str("chat-id", channelChatID).
			Int64("contact-id", channelFromID).

			Msg("Channel NOT Found")

		return errors.BadRequest(
			"chat.channel.not_found",
			"chat: channel ID=%s not found",
			 channelChatID,
		)
	}

	if messageAt != 0 {
		// FIXME: const -or- app.TimePrecision ?
		const divergence = time.Millisecond

		readUntil  = app.TimestampDate(messageAt)
		currentMs := localtime.Truncate(divergence)
		messageMs := readUntil.Truncate(divergence)
		updatedMs := channel.UpdatedAt.Truncate(divergence)

		if messageMs.Before(updatedMs) {
			return errors.BadRequest(
				"chat.read.date.invalid",
				"read: update %s date is beyond latest %s",
				 messageMs.Format(app.TimeStamp),
				 updatedMs.Format(app.TimeStamp),
			)
		}
		
		if messageMs.After(currentMs) {
			return errors.BadRequest(
				"chat.read.date.invalid",
				"read: update %s date is beyond local %s",
				 messageMs.Format(app.TimeStamp),
				 currentMs.Format(app.TimeStamp),
			)
		}
	}

	// updatedAt, err := s.repo.UpdateChannel(ctx, req.GetChannelId())
	err = s.repo.UpdateChannel(ctx, channelChatID, &readUntil)
	if err != nil {
		return err
	}

	err = s.eventRouter.SendUpdateChannel(channel, app.DateTimestamp(readUntil))
	// err = s.eventRouter.SendUpdateChannel(channel, updatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (s *chatService) SendMessage(
	ctx context.Context,
	req *pb.SendMessageRequest,
	res *pb.SendMessageResponse,
) error {

	// const (
		
	// 	precision = (int64)(time.Millisecond) // milli: 1e6
	// )

	var (

		// localtime = time.Now()
		// // timestamp = localtime.Unix() // seconds
		// // epochtime = localtime.UnixNano()/precision

		sendMessage = req.GetMessage()
		// recvMessage0 = pg.Message { // saveMessage
		// 	CreatedAt: localtime.UTC(),
		// 	// UpdatedAt: time.IsZero(!) // NOT edited yet !
		// }

		// edit = (sendMessage.UpdatedAt != 0)

		senderFromID = req.GetAuthUserId()
		senderChatID = req.GetChannelId()
		
		targetChatID = req.GetConversationId()
	)

	s.log.Debug().

		Str("channel_id",      senderChatID).
		Str("conversation_id", targetChatID).
		Int64("auth_user_id",  senderFromID).

		Str("type",  sendMessage.GetType()).
		Str("text",  sendMessage.GetText()).
		Bool("file", sendMessage.GetFile() != nil).

		Msg("SEND Message")

	if senderChatID == "" {
		senderChatID = targetChatID
		if senderChatID == "" {
			return errors.BadRequest(
				"chat.send.channel.from.required",
				"send: message sender chat ID required",
			)
		}
	}

	// region: lookup target chat session by unique sender chat channel id
	chat, err := s.repo.GetSession(ctx, senderChatID)

	if err != nil {
		// lookup operation error
		return err
	}

	if chat == nil || chat.ID != senderChatID {
		// sender channel ID not found
		return errors.BadRequest(
			"chat.send.channel.from.not_found",
			"send: FROM channel ID=%s sender not found or been closed",
			 senderChatID,
		)
	}

	if senderFromID != 0 && chat.User.ID != senderFromID {
		// mismatch sender contact ID
		return errors.BadRequest(
			"chat.send.channel.user.mismatch",
			"send: FROM channel_id=%s user_id=%d mismatch",
			 senderChatID, senderFromID,
		)
	}

	if chat.IsClosed() {
		// sender channel is already closed !
		return errors.BadRequest(
			"chat.send.channel.from.closed",
			"send: FROM chat channel ID=%s is closed",
			 senderChatID,
		)
	}

	sender := chat.Channel

	// Validate and normalize message to send
	// Mostly also stores non-service-level message to persistent DB
	_, err = s.saveMessage(ctx, nil, sender, sendMessage)
	
	if err != nil {
		// Failed to store message or validation error !
		return err
	}

	// // show chat room state
	// data, _ := json.MarshalIndent(chat, "", "  ")
	// s.log.Debug().Msg(string(data))

	// PERFORM message publish|broadcast
	_, err = s.sendMessage(ctx, chat, sendMessage)
	
	if err != nil {
		s.log.Error().Err(err).
			Msg("FAILED Sending Message")
		return err
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

	var (
		// TODO: keep track .sender.host to be able to respond to
		//       the same .sender service node for .this unique chat channel
		//       that will be created !
		localtime = app.CurrentTime()

		user  = req.GetUser()
		title = req.GetUsername()

		// // FIXME: this is always invoked by webitel.chat.bot service ?
		// // Gathering metadata to identify start req.Message sender NEW channel !...
		md, _ = metadata.FromContext(ctx)

		serviceProvider = md["Micro-From-Service"] // provider channel type !
		serviceNodeID   = md["Micro-From-Id"]      // provider channel host !
	)

	// FIXME:
	if serviceProvider != "webitel.chat.bot" {
		// LOG: this is the only case expected for now !..
	}

	s.log.Trace().

		Int64("domain.id",     req.GetDomainId()).
		Str("user.contact",    user.GetConnection()).
		Str("user.type",       user.GetType()).
		Int64("user.id",       user.GetUserId()).
		Str("user.name",       title).
		Bool("user.internal",  user.GetInternal()).
		Msg("START Conversation")

	// ORIGINATOR: CHAT channel, sender
	channel := pg.Channel{

		Type: req.GetUser().GetType(),
		UserID: req.GetUser().GetUserId(),
		
		CreatedAt: localtime,
		UpdatedAt: localtime,
		
		// ConversationID: conversation.ID,
		ServiceHost: sql.NullString{
			// senderProvider +"-"+ senderHostname,
			// contact/from: node-id
			String: serviceNodeID,
			Valid:  serviceNodeID != "",
		},
		Connection: sql.NullString{
			String: user.GetConnection(),
			Valid:  user.GetConnection() != "",
		},
		Internal: user.GetInternal(),
		DomainID: req.GetDomainId(),
		Name:     title,

		// NOTE: for now this endpoint if called by
		Properties: req.GetMessage().GetVariables(), // req.GetProperties(),
	}
	// NOTE: sender CHAT channel represents A leg, so must be created earlier
	// than, target CHAT channel represents B leg, the first recepient, chat@bot channel
	startDate := localtime.Add(time.Millisecond)
	// ORIGINATEE: CHAT channel, target: chat@bot
	conversation := &pg.Conversation{

		CreatedAt: startDate,
		UpdatedAt: startDate,
		
		DomainID: req.GetDomainId(),
		Title: sql.NullString{
			String: title, Valid: title != "",
		},
	}
	// CHAT start message
	startMessage := req.GetMessage()
	if startMessage == nil {
		// FIXME: imit service /start command message
		startMessage = &pb.Message{
			Type: "text",
			Text: "/start",
		}

	} else {
		// Validate START message type !
		messageType := startMessage.Type
		messageType = strings.TrimSpace(messageType)
		messageType = strings.ToLower(messageType)
		// reset: normalized !
		startMessage.Type = messageType

		switch startMessage.Type {

		case "":
			// TODO: support forward message !
			// NOTE: for externaly forwarded message(s), 
			//       providers copy original message source to result message to send
			//       so, I guess, we must never get this case: startMessage.Type == ""
			// FIXME: but what about internaly forwarded message(s) ?
			forward := startMessage.ForwardFromMessageId != 0 ||
				len(startMessage.ForwardFromVariables) != 0

			if !forward {
				if startMessage.File != nil {
					startMessage.Type = "file"
				} else {
					startMessage.Type = "text"
				}
			}

		case "text":
		case "file":

		default:
			// FIXME: any other message event types are disallowed !
			return errors.BadRequest(
				"chat.start.message.invalid",
				"start: message type=%s is invalid",
				 startMessage.Type,
			)
		}

		if edit := startMessage.UpdatedAt != 0; edit {
			// NOTE: implies EDIT message; disallowed !
			return errors.BadRequest(
				"chat.start.message.type.invalid",
				"start: message type=edit is invalid",
			)
		}
	}

	// // NOTE: now we are using .message.variables as NEW channel start environment
	// // TODO: separated attribute for start channel environment !!!
	// startMessage.CreatedAt = localtime
	// // TODO: validate and save start message !!!

	if err := s.repo.WithTransaction(func(tx *sqlx.Tx) error {
		// Create target CHAT room conversation ...
		if err := s.repo.CreateConversationTx(ctx, tx, conversation); err != nil {
			return err
		}
		// Create sender CHAT channel ...
		channel.ConversationID = conversation.ID
		if err := s.repo.CreateChannelTx(ctx, tx, &channel); err != nil {
			return err
		}
		// Transform channel OLD model to NEW one !
		sender := app.Channel{
			Chat: &app.Chat{
				ID:        channel.ID,
				Title:     channel.Name,
				Channel:   channel.Type,
				// Contact:   "",
				// Username:  "",
				// FirstName: "",
				// LastName:  "",
				Invite:    conversation.ID,
			},
			User: &app.User{
				ID:        channel.UserID,
				Channel:   channel.Type,
				// Contact:   "",
				FirstName: title,
				// LastName:  "",
				// UserName:  "",
				// Language:  "",
			},
			DomainID: channel.DomainID,
			// Status:   "",
			// Provider: nil,
			Created:  app.DateTimestamp(channel.CreatedAt),
			Updated:  app.DateTimestamp(channel.UpdatedAt),
			// Joined:   0,
			// Closed:   0,
		}
		// Save historical START conversation message ...
		if _, err := s.saveMessage(ctx, tx, &sender, startMessage); err != nil {
			return err
		}
		res.ConversationId = conversation.ID
		res.ChannelId = channel.ID
		// TODO: return error from s.flowClient.Init(..) !!!!!!!!!!!!!!!
		//       to be able to ROLLBACK DB changes
		//       when got "go.micro.client; service: not found" error
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
		
		// // Hide external provider message binding
		// // but setup with channel start properties
		// startMessage.Variables = req.GetProperties()
		err := s.flowClient.Init(&channel, startMessage)
		if err != nil {
			return err
		}
	}
	// else { FIXME: what todo ? }

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
}*/

func (s *chatService) CloseConversation(
	ctx context.Context,
	req *pb.CloseConversationRequest,
	res *pb.CloseConversationResponse,
) error {
	
	s.log.Trace().
		Str("conversation_id", req.GetConversationId()).
		Str("cause", req.GetCause()).
		Str("closer_channel_id", req.GetCloserChannelId()).
		Msg("CLOSE Conversation")

	var (

		// localtime = app.CurrentTime()

		senderFromID = req.GetAuthUserId()
		senderChatID = req.GetCloserChannelId()
		
		targetChatID = req.GetConversationId()
	)

	if senderChatID == "" {
		senderChatID = targetChatID
		if senderChatID == "" {
			return errors.BadRequest(
				"chat.close.channel.from.required",
				"close: disposition channel ID required",
			)
		}
	}

	// region: lookup target chat session by unique sender chat channel id
	chat, err := s.repo.GetSession(ctx, senderChatID)

	if err != nil {
		// lookup operation error
		return err
	}

	if chat == nil || chat.ID != senderChatID {
		// sender channel ID not found
		return errors.BadRequest(
			"chat.close.channel.from.not_found",
			"close: channel ID=%s not found or been closed",
			 senderChatID,
		)
	}

	if senderFromID != 0 && chat.User.ID != senderFromID {
		// mismatch sender contact ID
		return errors.BadRequest(
			"chat.close.channel.user.invalid",
			"close: channel ID=%s FROM user ID=%d is invalid",
			 senderChatID, senderFromID,
		)
	}

	if chat.IsClosed() {
		// // sender channel is already closed !
		// return errors.BadRequest(
		// 	"chat.close.channel.from.closed",
		// 	"close: FROM channel ID=%s is already closed",
		// 	 senderChatID,
		// )

		// Make idempotent !
		return nil
	}

	if targetChatID == "" {
		targetChatID = chat.Invite

	} else if !strings.EqualFold(chat.Invite, targetChatID) {
		// invalid target CHAT conversation ID
		return errors.BadRequest(
			"chat.close.conversation.invalid",
			"close: conversation ID=%s FROM channel ID=%s is invalid",
			 targetChatID, senderChatID,
		)
	}

	// sender := chat.Channel
	// endregion

	text := req.GetCause()

	_, err = s.sendChatClosed(ctx, chat, text)

	if err != nil {
		s.log.Error().Err(err).
			Msg("FAILED Notify Chat Members")
		// return err
	}

	// Mark ALL chat members as CLOSED !
	// NOTE: - delete: chat.confirmation; - delete: chat.flow.node
	err = s.closeConversation(ctx, &targetChatID)
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

	from := req.GetAuthUserId() // FROM
	token := req.GetInviteId()  // AUTH

	if from == 0 {
		return errors.BadRequest(
			"chat.join.user.required",
			"join: user authentication required",
		)
	}
	
	if token == "" {
		return errors.BadRequest(
			"chat.join.invite.required",
			"join: invite token required but missing",
		)
	}
	
	s.log.Trace().
		Int64("user_id", from).
		Str("invite_id", token).
		Msg("JOIN Conversation")
	
	invite, err := s.repo.GetInviteByID(ctx, token)
	
	if err != nil {
		s.log.Error().Err(err).
			Str("invite_id", token).
			Msg("FAILED Lookup INVITE token")
		return err
	}

	found := invite != nil
	found = found && invite.ID == token // req.InviteId
	found = found && invite.UserID == from // req.AuthUserId
	found = found && invite.ClosedAt.Time.IsZero() // NOT Closed !

	if !found {
		// s.log.Warn().Msg("invitation not found")
		return errors.NotFound(
			"chat.invite.not_found",
			"join: invite token %s is invalid or already used",
			 token,
		)
	}

	user, err := s.repo.GetWebitelUserByID(ctx, from)

	if err != nil {
		s.log.Error().Err(err).
			Int64("user_id", invite.UserID).
			Int64("domain_id", invite.DomainID).
			Msg("FAILED Lookup Chat User")
		return err
	}

	if user == nil || user.DomainID != invite.DomainID {
		// s.log.Warn().Msg("user not found")
		return errors.NotFound(
			"chat.user.not_found",
			"join: user ID=%d not found",
			 from,
		)
	}

	channel := &pg.Channel{
		ID:             invite.ID, // FROM: INVITE token !
		Type:           "webitel",
		Internal:       true,
		ConversationID: invite.ConversationID,
		UserID:         invite.UserID,
		DomainID:       invite.DomainID,
		Name:           user.Name,
		JoinedAt:       sql.NullTime{
			Time:       app.CurrentTime().UTC(),
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
		if _, err := s.repo.CloseInviteTx(ctx, tx, req.GetInviteId()); err != nil {
			return err
		}
		res.ChannelId = channel.ID
		return nil
	}); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}

	// NOTIFY Related Chat Members !
	err = s.eventRouter.RouteJoinConversation(
		channel, &invite.ConversationID,
	)

	if err != nil {
		s.log.Error().Err(err).
			Str("event", "joined").
			Str("invite_id", invite.ID). // TODO: same as NEW channel.ID
			Str("conversation_id", invite.ConversationID).
			Int64("user_id", invite.UserID).
			Msg("FAILED Notify Chat Members")
		// return err // NON Fatal !
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
		Msg("LEAVE Conversation")
	
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
		Msg("INVITE TO Conversation")
	
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
			String: req.GetInviterChannelId(), Valid: true,
		}
	}
	if err := s.repo.CreateInvite(ctx, invite); err != nil {
		s.log.Error().Err(err).Msg("FAILED Create INVITE Token")
		return err
	}
	conversation, err := s.repo.GetConversations(ctx, req.GetConversationId(), 0, 0, nil, nil, 0, false, 0, 0)
	if err != nil {
		s.log.Error().Err(err).
			Str("id", req.ConversationId).
			Msg("FAILED Lookup Conversation")
		return err
	}
	if conversation == nil {
		// s.log.Error().Msg("conversation not found")
		return errors.NotFound(
			"chat.conversation.not_found",
			"chat: conversation ID=%s not found",
			 req.ConversationId,
		)
	}

	await := make(chan error, 2)
	for _, async := range []func() {
		func() {
			// 1. NOTIFY: Invited User session(s) !
			await <- s.eventRouter.SendInviteToWebitelUser(
				transformConversationFromRepoModel(conversation[0]), invite,
			)
		},
		func() {
			// 2. NOTIFY: All related Chat members !
			await <- s.eventRouter.RouteInvite(
				&invite.ConversationID, &invite.UserID,
			)
		},
	} {
		go async()
	}

	for i := 0; i < 2; i++ {
		err = <-await
		if err != nil {
			s.log.Error().Err(err).Msg("FAILED Notify Chat Members")
			return err
		}
	}
	close(await)


	// // 1. NOTIFY: Invited User session(s) !
	// err = s.eventRouter.SendInviteToWebitelUser(
	// 	transformConversationFromRepoModel(conversation[0]), invite,
	// )
	// // 2. NOTIFY: All related Chat members !
	// err = s.eventRouter.RouteInvite(
	// 	&invite.ConversationID, &invite.UserID,
	// )
	
	/*resErrorsChan := make(chan error, 2)
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
	}*/
	if ttl := req.GetTimeoutSec(); ttl > 0 {
		go func() {
			time.Sleep(time.Duration(ttl) * time.Second) // seconds
			/*if val, err := s.repo.GetInviteByID(context.Background(), invite.ID); err != nil {
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
			}*/
			closed, err := s.repo.CloseInvite(context.Background(), invite.ID)

			if err != nil {
				s.log.Error().Err(err).
					Str("invite_id", invite.ID).
					Int64("user_id", invite.UserID).
					Str("conversation_id", invite.ConversationID).
					Msg("FAILED Closing INVITE")
				return
			}
			
			if !closed {
				// NOTE: invalid invite_id, already closed or joined !
				return
			}
			// NOTE: closed !
			s.log.Warn().

				Str("invite_id", invite.ID).
				Int64("user_id", invite.UserID).
				Str("conversation_id", invite.ConversationID).

				Msg("INVITE Timeout")
			
			if req.InviterChannelId == "" { // FROM: workflow !
				
				err = s.flowClient.BreakBridge(
					req.ConversationId, flow.TimeoutCause,
				)
				
				if err != nil {
					s.log.Error().Msg(err.Error())
				}
			}
			// NOTIFY: timed out !
			err = s.eventRouter.SendDeclineInviteToWebitelUser(
				&domainID, &invite.ConversationID, &invite.UserID, &invite.ID,
			)

			if err != nil {
				s.log.Error().Err(err).
					Msg("FAILED Notify User INVITE Timeout")
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
		Msg("DECLINE Invitation")
	
	invite, err := s.repo.GetInviteByID(ctx, req.GetInviteId())

	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}

	found := invite != nil

	found = found && invite.ID == req.InviteId
	found = found && invite.UserID == req.AuthUserId
	found = found && invite.ClosedAt.Time.IsZero() // NOT Closed yet !

	if !found {
		// return errors.BadRequest(
		// 	"chat.decline.token.invalid",
		// 	"decline: invite %s token invalid or been closed",
		// 	 req.InviteId,
		// )
		// Be loyal and idempotent !
		return nil
	}

	// PERFORM: Mark invite token as 'closed' !
	closed, err := s.repo.CloseInvite(ctx, invite.ID)

	if err != nil {
		re := errors.FromError(err)
		if re.Id == "" {
			code := http.StatusInternalServerError
			re.Id = "chat.invite.decline.error"
			re.Code = (int32)(code)
			re.Status = http.StatusText(code)
			// re.Detail = err.Error() // Something went wrong !
		}
		return re
	}

	if !closed {
		// NOTE: Not found or already closed !
		// Be loyal and idempotent !
		return nil // OK !
	}
	// INVITED FROM: workflow ?
	if !invite.InviterChannelID.Valid {
		_ = s.flowClient.BreakBridge(
			invite.ConversationID, flow.DeclineInvitationCause,
		)
		// if err != nil {
		// 	// LOG: itself !
		// }
	}
	// parallel
	await := make(chan error, 2)
	for _, async := range []func() {
		func() {
			// NOTIFY: All related Chat members !
			await <- s.eventRouter.RouteDeclineInvite(
				&invite.UserID, &invite.ConversationID,
			)
		},
		func() {
			// NOTIFY: Invited User session(s) !
			await <- s.eventRouter.SendDeclineInviteToWebitelUser(
				&invite.DomainID, &invite.ConversationID, &invite.UserID, &invite.ID,
			)
		},
	} {
		go async()
	}

	for i := 0; i < 2; i++ {
		if err = <-await; err != nil {
			s.log.Error().Err(err).
				Str("event", "declined").
				Str("invite_id", invite.ID).
				Str("conversation_id", invite.ConversationID).
				Int64("user_id", invite.UserID).
				Msg("FAILED Notify Chat Members")
			// return err // NON Fatal !
		}
	}
	close(await)



	// // NOTE: guess, this method publishes {decline_invite} for all chat related members
	// //       but events are not appointed to specific channel(s), they all are the same !..
	
	// // NOTIFY: Related Chat Members !
	// err = s.eventRouter.RouteDeclineInvite(
	// 	&invite.UserID, &invite.ConversationID,
	// )

	// // if err != nil {
	// // 	// LOG: itself !
	// // }

	// // NOTIFY: Invited User Session(s) !
	// err = s.eventRouter.SendDeclineInviteToWebitelUser(
	// 	&invite.DomainID, &invite.ConversationID, &invite.UserID, &invite.ID,
	// )

	// if err != nil {

	// 	s.log.Error().Err(err).
	// 		Str("event", "decline").
	// 		Str("invite_id", invite.ID).
	// 		Str("conversation_id", invite.ConversationID).
	// 		Int64("user_id", invite.UserID).
	// 		Msg("FAILED Notify User")
	// }

	// Be loyal and idempotent !
	return nil // OK !



	/*resErrorsChan := make(chan error, 3)
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
		if _, err := s.repo.CloseInvite(ctx, req.GetInviteId()); err != nil {
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
	return nil*/
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

	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	s.log.Trace().
		Str("name", req.GetItem().GetName()).
		Str("type", req.GetItem().GetType()).
		Int64("domain_id", user.DomainID). //req.GetItem().GetDomainId()).
		Int64("schema_id", req.GetItem().GetSchemaId()).
		Str("variables", fmt.Sprintf("%v", req.GetItem().GetVariables())).
		Msg("create profile")

	// if user.DomainID != req.GetItem().GetDomainId() {
	// 	s.log.Error().Msg("invalid domain id")
	// 	return errors.BadRequest("invalid domain id", "")
	// }

	req.Item.DomainId = user.DomainID

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



// func (c *chatService) saveMessage(ctx context.Context, dcx sqlx.ExtContext, senderChatID string, targetChatID string, notify *pb.Message) (saved *pg.Message, err error) {
func (c *chatService) saveMessage(ctx context.Context, dcx sqlx.ExtContext, sender *app.Channel, notify *pb.Message) (saved *pg.Message, err error) {

	var (

		sendMessage  = notify

		senderChatID = sender.Chat.ID
		targetChatID = sender.Chat.Invite

		localtime    = app.CurrentTime()
	)

	// region: PRE- processing: fetch related messages ...
	
	if sendMessage == nil {
		return nil, errors.BadRequest(
			"chat.message.required",
			"chat: message required but missing",
		)
	}

	if senderChatID == "" {
		senderChatID = targetChatID
		if senderChatID == "" {
			return nil, errors.BadRequest(
				"chat.message.channel.required",
				"chat: message sender chat ID required",
			)
		}
	}

	// TODO:
	// 1. Fetch message for 'edit' or 'forward' request;
	// 2. For founded 'forward' message copy source to sendMessage
	// 3. Validate sendMesage integrity
	// 4. Save non-service message to persistent DB

	// Detecting underlaying operation purpose ...
	var (

		forwardFromMessageID = sendMessage.ForwardFromMessageId
		forwardFromBinding = sendMessage.ForwardFromVariables

		replyToMessageID = sendMessage.ReplyToMessageId
		replyToBinding = sendMessage.ReplyToVariables

		// FORWARD operation purpose ?
		forward =
			forwardFromMessageID != 0 ||
			len(forwardFromBinding) != 0

		// REPLY operation purpose ?
		reply =
			replyToMessageID != 0 ||
			len(replyToBinding) != 0

		// EDIT operation purpose ?
		edit = sendMessage.UpdatedAt != 0

		// Store (Saved) Message Model
		saveMessage *pg.Message
	)

	// Normalize lookup message bindings if provided
	for _, findBinding := range []*map[string]string {
		&forwardFromBinding, &replyToBinding,
	} {
		if *(findBinding) == nil {
			continue
		}
		delete(*(findBinding), "")
		if len(*(findBinding)) == 0 {
			*(findBinding) = nil
		}
	}

	if forward && forwardFromMessageID == 0 && len(forwardFromBinding) == 0 {
		// NOTE: we detected 'forward' operation purpose
		//       but after message's binding normalization
		//       they resulted in empty properties map
		return nil, errors.BadRequest(
			"chat.forward.message.binding.required",
			"chat: forward message binding is missing",
		)
	}

	if reply && replyToMessageID == 0 && len(replyToBinding) == 0 {
		// NOTE: the same for 'reply' operation request ...
		return nil, errors.BadRequest(
			"chat.reply.message.binding.required",
			"chat: reply to message binding is missing",
		)
	}

	// Hide lookup message bindings from result message SENT
	sendMessage.ForwardFromVariables = nil
	sendMessage.ReplyToVariables = nil

	if edit {
		// FIXME: Can we edit forwarded message ?
		if forward {
			return nil, errors.BadRequest(
				"chat.edit.message.forwarded",
				"chat: edit forwarded messages not allowed",
			)
		}

		// NOTE: External chat providers does NOT provide internal message.ID to be EDITED
		//       They JUST provide original message external identifier(s), called binding(s),
		//       so be aware of (edit == true && sendMessage.ID == 0)
		// NOTE: In that case sendMessage.Variables acts as a search filter
		//       that MUST locate unique, single message to edit !
		findBinding := sendMessage.Variables
		// lookup for original message to edit
		saveMessage, err = c.repo.GetMessage(
			ctx, sendMessage.Id, senderChatID, targetChatID, findBinding,
		)

		if err != nil {
			return nil, errors.BadRequest(
				"chat.message.lookup.error",
				"edit: message ID=%d lookup: %s",
				 sendMessage.Id, err,
			)
		}

		// CHECK: original message found ?
		found := (saveMessage != nil)
		// CHECK: original message match requested ID ?
		found = found && (sendMessage.Id == 0 || saveMessage.ID == sendMessage.Id)
		// CHECK: original message match requested bindings ?
		if found && len(findBinding) != 0 {
			for key, value := range findBinding {
				bound, ok := saveMessage.Variables[key]
				if !ok || bound != value {
					// Found message does not match partial bindings !
					found = false
					break
				}
			}
		}
		
		if !found {
			return nil, errors.BadRequest(
				"chat.edit.message.not_found",
				"edit: message ID=%d from chat ID=%s not found",
				 sendMessage.Id, senderChatID,
			)
		}

		if !strings.EqualFold(senderChatID, saveMessage.ChannelID) {
			return nil, errors.Forbidden(
				"chat.edit.message.forbidden",
				"chat: message ID=%d editor chat ID=%s is not the author",
				 sendMessage.Id, senderChatID,
			)
		}

		if saveMessage.ForwardFromMessageID != 0 {
			return nil, errors.BadRequest(
				"chat.edit.message.forwarded",
				"chat: edit forwarded messages not allowed",
			)
		}

		// Mark message to be EDITED !
		saveMessage.UpdatedAt = localtime // .UTC().Truncate(app.TimePrecision)

	} else {
		// Allocate NEW message to be saved !
		saveMessage = &pg.Message{

			CreatedAt: localtime, // .UTC().Truncate(app.TimePrecision),
			// UpdatedAt: time.Time{}.IsZero(!) // MUST: NOT EDITED !
			
			// [FROM]: ChatID
			ChannelID: senderChatID,

			// [TO]: ChatID
			ConversationID: targetChatID,
		}
	}

	if forward {
		
		forwardFromChatID := sendMessage.ForwardFromChatId
		if forwardFromChatID == "" {
			forwardFromChatID = targetChatID
		}

		forwardMessage, err := c.repo.GetMessage(ctx,
			forwardFromMessageID, "", forwardFromChatID, forwardFromBinding,
		)

		if err != nil {
			return nil, errors.BadRequest(
				"chat.message.lookup.error",
				"forward: message ID=%d lookup: %s",
				 forwardFromMessageID, err,
			)
		}

		// CHECK: original message found ?
		found := (forwardMessage != nil)
		// CHECK: original message match requested ID ?
		found = found && (forwardFromMessageID == 0 || forwardFromMessageID == forwardMessage.ID)
		// CHECK: original message match requested bindings ?
		if found && len(forwardFromBinding) != 0 {
			for key, value := range forwardFromBinding {
				bound, ok := forwardMessage.Variables[key]
				if !ok || bound != value {
					// Found message does not match partial bindings !
					found = false
					break
				}
			}
		}

		if !found {
			return nil, errors.BadRequest(
				"chat.forward.message.not_found",
				"forward: original message ID=%d not found",
				 forwardFromMessageID,
			)
		}

		// MARK message FORWARDED !
		saveMessage.ForwardFromMessageID = forwardMessage.ID
		// COPY Original Message Source !
		saveMessage.Type = forwardMessage.Type
		saveMessage.Text = forwardMessage.Text
		saveMessage.File = forwardMessage.File

		// Populate result message payload !
		sendMessage.ForwardFromMessageId = forwardMessage.ID
		sendMessage.ForwardFromChatId    = forwardMessage.ConversationID
		// Forward Message Payload
		sendMessage.Type = forwardMessage.Type
		sendMessage.Text = forwardMessage.Text
		if doc := forwardMessage.File; doc != nil {
			sendMessage.File = &pb.File{
				Id:   doc.ID,
				Url:  "",
				Size: doc.Size,
				Mime: doc.Type,
				Name: doc.Name,
			}
		}
	
	} else if reply {
		// Omit recheck for EDIT message with the same value !
		if saveMessage.ReplyToMessageID == 0 || (replyToMessageID != 0 &&
			saveMessage.ReplyToMessageID != replyToMessageID) {
			// TODO: find message by internal id or external sent-bindings
			replyToMessage, err := c.repo.GetMessage(ctx,
				replyToMessageID, "", targetChatID, replyToBinding,
			)

			if err != nil {
				return nil, errors.BadRequest(
					"chat.message.lookup.error",
					"reply: message ID=%d lookup: %s",
					 replyToMessageID, err,
				)
			}

			// CHECK: original message found ?
			found := (replyToMessage != nil)
			// CHECK: original message match requested ID ?
			found = found && (replyToMessageID == 0 || replyToMessage.ID == replyToMessageID)
			// CHECK: original message match requested bindings ?
			if found && len(replyToBinding) != 0 {
				for key, value := range replyToBinding {
					bound, ok := replyToMessage.Variables[key]
					if !ok || bound != value {
						// Found message does not match partial bindings !
						found = false
						break
					}
				}
			}

			if !found {
				return nil, errors.BadRequest(
					"chat.reply.message.not_found",
					"reply: original message ID=%d not found",
					 replyToMessageID,
				)
			}

			// MARK message as REPLY !
			saveMessage.ReplyToMessageID = replyToMessage.ID

			// Disclose operation details
			sendMessage.ReplyToMessageId = replyToMessage.ID
		}
	}

	saveBinding := sendMessage.Variables
	// NOTE: Hide bindings from recepients, because this implies system request info !
	sendMessage.Variables = nil

	if saveBinding != nil {
		delete(saveBinding, "")
		if len(saveBinding) != 0 {
			
			// data, err := json.Marshal(saveBinding)
			// if err != nil {
			// 	// Failed to store message variables !
			// 	return nil, errors.BadRequest(
			// 		"chat.message.variables.error",
			// 		"send: failed to encode message variables; %s",
			// 		 err,
			// 	)
			// }
			// // populate to be saved !
			// saveMessage.Variables = data
			saveMessage.Variables = saveBinding
		
		} // else {
		// 	// cleanup broken set: {"": ?}
		// 	sendMessage.Variables = nil
		// }
	}

	// endregion

	// region: POST- processing: validate result message

	messageType := sendMessage.Type
	messageType  = strings.TrimSpace(messageType)
	messageType  = strings.ToLower(messageType)
	// reset: normalized !
	sendMessage.Type = messageType

	if sendMessage.Type == "" {
		// NOTE: if sendMessage.Type is blank that means that
		//       type is omitted, so we need to look into payload
		if sendMessage.File != nil {
			sendMessage.Type = "file"
		// } else {
		// 	sendMessage.Type = "text"
		// }
		} else if sendMessage.Contact != nil {
			sendMessage.Type = "contact"
		} else {
			sendMessage.Type = "text"
		}

	}
	
	switch sendMessage.Type {

	case "text":

		text := sendMessage.GetText()
		text = strings.TrimSpace(text)
		
		if text == "" {
			return nil, errors.BadRequest(
				"chat.send.message.text.missing",
				"send: message text is missing",
			)
		}
		// reset: normalized !
		sendMessage.Text = text
		// TOBE: saved !
		saveMessage.Type = "text"
		saveMessage.Text = text

	case "buttons", "inline":

		saveMessage.Type = "menu"

	case "contact":

		contact := sendMessage.GetContact()
		
		saveMessage.Type = "contact"
		saveMessage.Text = contact.Contact

		err := c.repo.UpdateClientNumber(ctx, sender.User.ID, contact.Contact)
		if err != nil {
			c.log.Error().Err(err).Msg("Failed to store Client number")
			return nil, err
		}

	case "file":

		// CHECK: document specified ?
		doc := sendMessage.GetFile()
		if doc == nil {
			return nil, errors.BadRequest(
				"chat.send.document.file.missing",
				"send: document file is missing",
			)
		}
		// // CHECK: document is internal file ?
		// if doc.ID == 0 {
		// 	// TODO: /storage/MediaFileService.ReadMediaFile(id: , domain_id: )
		// }

		// CHECK: document URL specified ?
		if doc.Url == "" {
			return nil, errors.BadRequest(
				"chat.send.document.url.required",
				"send: document source URL required",
			)
		}
		// CHECK: provided URL is valid ?
		src, err := url.Parse(doc.Url)
		
		if err != nil {
			return nil, errors.BadRequest(
				"chat.send.document.url.invalid",
				"send: document source URL invalid; %s", err,
			)
		}
		// reset: normalized !
		doc.Url = src.String()

		// CHECK: filename !
		if doc.Name == "" {
			doc.Name = path.Base(src.Path)
			switch doc.Name {
			case "", ".", "/": // See: path.Base()
				return nil, errors.BadRequest(
					"chat.send.document.name.missing",
					"send: document filename is missing or invalid",
				)
			}
		}

		// .Caption
		caption := sendMessage.GetText()
		caption = strings.TrimSpace(caption)
		// reset: normalized !
		sendMessage.Text = caption

		// CHECK: document uploaded ?
		if doc.Id == 0 {
			// Upload ! // TODO: Background, async ..
			res, err := c.storageClient.UploadFileUrl(
				context.TODO(),
				&pbstorage.UploadFileUrlRequest{
					DomainId: sender.DomainID,
					Uuid:     sender.Chat.Invite, // sender.ConversationID, // FIXME: is this required ?
					Name:     doc.Name,
					Mime:     doc.Mime,
					Url:      doc.Url,
				},
			)

			if err != nil {
				c.log.Error().Err(err).Msg("Failed to UploadFileUrl")
				return nil, errors.InternalServerError(
					"chat.upload.document.error",
					"upload: %s", err.Error(),
				)
			}

			// CHECK: finally(!) response document data
			if res.Id == 0 {
				return nil, errors.InternalServerError(
					"chat.upload.document.error",
					"upload: returned <zero> document ID",
				)
			}

			// // CHECK: uploaded file URL returned ?
			// if doc.Url == "" {
			// 	return errors.InternalServerError(
			// 		"chat.send.document.url.missing",
			// 		"send: uploaded document URL is missing",
			// 	)
			// }

			// // CHECK: download URL is still valid ?
			// src, err := url.Parse(res.Url)
			
			// if err != nil {
			// 	return errors.InternalServerError(
			// 		"chat.send.document.url.invalid",
			// 		"send: uploaded document URL invalid; %s",
			// 		err,
			// 	)
			// }

			// reset: noramlized !
			doc.Id   = res.Id
			doc.Url  = res.Url // src.String()
			doc.Size = res.Size
			doc.Mime = res.Mime
			// doc.Name = res.Name // Normalized ABOVE !
		}

		// Fill .Document
		saveMessage.Type = "file"
		saveMessage.File = &pg.Document{
			ID:   doc.Id,
			Size: doc.Size,
			Name: doc.Name,
			Type: doc.Mime,
		}
		// Fill .Caption
		saveMessage.Text = caption

	case "read":

		// TODO: DO NOT save to persistent DB; this is the service-level-message !

		readMessageAll := localtime.UTC().Truncate(app.TimePrecision)
		readMessageTill := readMessageAll
		
		if date := sendMessage.UpdatedAt; date != 0 {
			readMessageTill = app.TimestampDate(date)
			if readMessageTill.After(readMessageAll) {
				return nil, errors.BadRequest(
					"chat.read.message.date.invalid",
					"read: message date %s is future; hint: leave it blank to read all messages",
					 readMessageTill.Format(app.TimeStamp),
				)
			}
			readMessageLast := app.TimestampDate(sender.Updated)
			if readMessageTill.Before(readMessageLast) {
				return nil, errors.BadRequest(
					"chat.read.message.date.invalid",
					"read: messages till %s already read; hint: leave it blank to read all messages",
					 readMessageLast.Format(app.TimeStamp),
				)
			}
		}

		// TODO: update chat.channel set updated_at = ${saveMessage.UpdatedAt} where id = ${senderChat.ID}
		err = c.repo.UpdateChannel(ctx, sender.Chat.ID, &readMessageTill)
		
		if err != nil {
			return nil, err
		}

		// NOTE: this is the service level message,
		//       so we dont need to store it ...
		return nil, nil // SUCCESS

	// // sendStatus
	// case "upload": // uploading file document; service message: DO NOT store !
	// 	// FIXME: do not store; just broadcast to sender's chat members
	// case "typing": // typing message text; service message: DO NOT store !
	// 	// FIXME: do not store; just broadcast to sender's chat members
	// case "closed":
	default:
		
		return nil, errors.BadRequest(
			"chat.send.message.type.invalid",
			"send: message '%s' is invalid",
			 messageType,
		)
	}

	// endregion

	// region: save historical message to persistent DB

	// NOTE: Need to be saved to persistent storage ! Is "text" -or- "file"
	if tx, ok := dcx.(*sqlx.Tx); ok {
		err = c.repo.CreateMessageTx(ctx, tx, saveMessage)
	} else {
		err = c.repo.SaveMessage(ctx, saveMessage)
	}

	if err != nil {
		c.log.Error().Err(err).Msg("Failed to store message")
		return nil, err
	}

	// if err = c.repo.SaveMessage(ctx, saveMessage); err != nil {
	// 	c.log.Error().Err(err).Msg("Failed to store message")
	// 	return nil, err
	// }

	// Populate saved message ID
	sendMessage.Id = saveMessage.ID
	// Populate saved message timing ...
	sendMessage.CreatedAt = app.DateTimestamp(saveMessage.CreatedAt)
	// if !saveMessage.UpdatedAt.IsZero() {
	sendMessage.UpdatedAt = app.DateTimestamp(saveMessage.UpdatedAt)
	// }
	// endregion

	return saveMessage, nil
}

// SendMessage publishes given message to all related recepients
// Override: event_router.RouteMessage()
func (c *chatService) sendMessage(ctx context.Context, chatRoom *app.Session, notify *pb.Message) (sent int, err error) {
	// FROM
	sender := chatRoom.Channel
	// TO
	if len(chatRoom.Members) == 0 {
		return 0, nil // NO ANY recepient(s) !
	}

	// publish
	var (

		data []byte
		header map[string]string

		rebind bool
		binding = notify.GetVariables()
		// default: workflow chat@bot channel -if- no any member(s)
		chatflow *app.Channel
	)
	// Broadcast message to every member in the room,
	// in front of chaRoom.Channel as a sender !
	members := make([]*app.Channel, 1+len(chatRoom.Members))
	
	members[0] = sender
	copy(members[1:], chatRoom.Members)
	
	for _, member := range members { // chatRoom.Members {
		
		if member.IsClosed() {
			continue // omit send TO channel: closed !
		}

		switch member.Channel {

		case "websocket": // TO: engine (internal)
			// s.eventRouter.sendEventToWebitelUser()
			// NOTE: if sender is an internal chat@channel user (operator)
			//       we publish message for him (author) as a member too
			//       to be able to detect chat updates on other browser tabs ...
			if data == nil {
				// basic
				timestamp := notify.UpdatedAt
				if timestamp == 0 {
					timestamp = notify.CreatedAt
				}
				notice := events.MessageEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: sender.Chat.Invite, // hidden channel.conversation_id
						Timestamp:      timestamp, // millis
					},
					Message: events.Message{
						ID:        notify.Id,
						ChannelID: sender.Chat.ID,
						Type:      notify.Type,
						Text:      notify.Text,
						// File:   notify.File,
						CreatedAt: notify.CreatedAt, // NEW
						UpdatedAt: notify.UpdatedAt, // EDITED
						
						ReplyToMessageID: notify.ReplyToMessageId,
						MessageForwarded: events.MessageForwarded{
							// original message/sender details ...
							ForwardFromChatID:    notify.ForwardFromChatId,
							ForwardFromMessageID: notify.ForwardFromMessageId,
							ForwardSenderName:    "",
							ForwardDate:          0,
						},
					},
				}
				// File
				if doc := notify.File; doc != nil {
					notice.File = &events.File{
						ID:   doc.Id,
						Size: doc.Size,
						Type: doc.Mime,
						Name: doc.Name,
					}
				}
				
				// Contact
				if contact := notify.Contact; contact != nil {
					notice.Contact = &events.Contact{
						ID:        contact.Id,
						FirstName: contact.FirstName,
						LastName:  contact.LastName,
						Phone:     contact.Contact,
					}
				}
				// init once
				data, _ = json.Marshal(notice)
				header = map[string]string {
					"content_type": "text/json",
				}
			}

			agent := service.Options().Broker
			err = agent.Publish(fmt.Sprintf("event.%s.%d.%d",
				events.MessageEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: header,
				Body:   data,
			})

		case "chatflow":  // TO: workflow (internal)
			// NOTE: we do not send messages to chat@bot channel
			// until there is not a private (one-to-one) chat room
			if member == sender { // e == 0
				continue
			}
			chatflow = member
			continue
			// err = c.flowClient.SendMessageV1(member, notify)

		default:          // TO: webitel.chat.bot (external)
			// s.eventRouter.sendMessageToBotUser()
			if member == sender { // e == 0
				continue
			}
			err = c.eventRouter.SendMessageToGateway(member, notify)
			// Merge SENT message external binding variables
			for key, newValue := range notify.GetVariables() {
				if key == "" { continue }
				oldValue, exists := binding[key]
				rebind = rebind || !exists || newValue != oldValue
				if exists && newValue != oldValue {
					// FIXME: key(s) must be unique within recepients ? What if not ?
				}
				// reset|override (!)
				if binding == nil {
					binding = make(map[string]string)
				}
				binding[key] = newValue
			}
			// Merged !
			notify.Variables = binding
			// user := member.User
			// // "user": {
			// // 	"id": 59,
			// // 	"channel": "telegram",
			// // 	"contact": "520924760",
			// // 	"firstName": "srgdemon"
			// // },
			// req := gate.SendMessageRequest{
			// 	ProfileId:      14, // profileID,
			// 	ExternalUserId: user.Contact, // client.ExternalID.String,
			// 	Message:        notify,
			// }
		}

		(sent)++ // calc active recepients !

		var trace *zerolog.Event
		
		if err != nil {
			// FIXME: just log failed attempt ?
			trace = c.log.Error().Err(err)
		} else {
			trace = c.log.Trace()
		}

		trace.

			Str("chat-id", member.Chat.ID).
			Str("channel", member.Chat.Channel).
			Str("TO",      member.User.FirstName).

			Msg("SENT")
	}
	// Otherwise, if NO-ONE in the room - route message to the chat-flow !
	if sent == 0 && chatflow != nil {
		// MUST: (chatflow != nil)
		err = c.flowClient.SendMessageV1(chatflow, notify)

		if err != nil {
			c.log.Error().Err(err).Str("chat-id", chatflow.Chat.ID).Msg("SEND TO chat@flow")
		}
	
	} else if rebind {

		_ = c.repo.BindMessage(ctx, notify.Id, binding)
	}

	return sent, nil // err
}

// sendClosed publishes final message to all related members
// Override: event_router.RouteCloseConversation[FromFlow]()
func (c *chatService) sendChatClosed(ctx context.Context, chatRoom *app.Session, text string) (sent int, err error) {

	localtime := app.CurrentTime()
	// FROM
	sender := chatRoom.Channel
	// // TO
	// if len(chatRoom.Members) == 0 {
	// 	return 0, nil // NO ANY recepient(s) !
	// }

	if text == "" {
		text = "Conversation closed"
	}

	// publish
	var (

		data []byte
		header map[string]string

		notice *pb.Message
	)
	// Broadcast message to every member in the room,
	// in front of chaRoom.Channel as a sender !
	members := make([]*app.Channel, 1+len(chatRoom.Members))
	
	members[0] = sender
	copy(members[1:], chatRoom.Members)
	
	for _, member := range members { // chatRoom.Members {
		
		if member.IsClosed() {
			continue // omit send TO channel: closed !
		}

		switch member.Channel {

		case "websocket": // TO: engine (internal)
			// s.eventRouter.sendEventToWebitelUser()
			// NOTE: if sender is an internal chat@channel user (operator)
			//       we publish message for him (author) as a member too
			//       to be able to detect chat updates on other browser tabs ...
			if data == nil {
				// basic
				// timestamp := notify.UpdatedAt
				// if timestamp == 0 {
				// 	timestamp = notify.CreatedAt
				// }
				notice := events.CloseConversationEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: sender.Chat.Invite, // hidden channel.conversation_id
						// Timestamp:      timestamp, // millis
						Timestamp: app.DateTimestamp(localtime),
					},
					FromChannelID: sender.Chat.ID,
					Cause:         text,
				}
				// init once
				data, _ = json.Marshal(notice)
				header = map[string]string {
					"content_type": "text/json",
				}
			}

			agent := service.Options().Broker
			err = agent.Publish(fmt.Sprintf("event.%s.%d.%d",
				events.CloseConversationEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: header,
				Body:   data,
			})

		case "chatflow":  // TO: workflow (internal)
			// NOTE: we do not send messages to chat@bot channel
			// until there is not a private (one-to-one) chat room
			if member == sender { // e == 0
				continue
			}
			// Send workflow channel .Break() message to stop chat.flow routine ...
			// FIXME: - delete: chat.confirmation; - delete: chat.flow.node
			err = c.flowClient.CloseConversation(member.Chat.ID) // .ConversationID
			
			// if err != nil {
			// 	c.log.Error().Err(err).
			// 		Msg("FAILED Break chat@flow routine")
			// 	// return err
			// }

		default:          // TO: webitel.chat.bot (external)
			// s.eventRouter.sendMessageToBotUser()
			
			// if member == sender { // e == 0
			// 	continue
			// }

			if notice == nil {
				notice = &pb.Message{

					Id:    0,

					Type: "closed", // "text",
					Text:  text,
					
					CreatedAt: app.DateTimestamp(localtime),
				}
			}
			
			err = c.eventRouter.SendMessageToGateway(member, notice)
		}

		(sent)++ // calc active recepients !

		var trace *zerolog.Event
		
		if err != nil {
			// FIXME: just log failed attempt ?
			trace = c.log.Error().Err(err)
		} else {
			trace = c.log.Trace()
		}

		trace.

			Str("chat-id", member.Chat.ID).
			Str("channel", member.Chat.Channel).
			Str("TO",      member.User.FirstName).

			Msg("CLOSED")
	}
	// // Otherwise, if NO-ONE in the room - route message to the chat-flow !
	// if sent == 0 && chatflow != nil {
	// 	// MUST: (chatflow != nil)
	// 	err = c.flowClient.SendMessageV1(chatflow, notify)

	// 	if err != nil {
	// 		c.log.Error().Err(err).Str("chat-id", chatflow.Chat.ID).Msg("SEND TO chat@flow")
	// 	}
	
	// } else if rebind {

	// 	_ = c.repo.BindMessage(ctx, notify.Id, binding)
	// }

	return sent, nil // err
}