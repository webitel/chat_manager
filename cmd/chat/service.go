package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	wlog "github.com/webitel/chat_manager/log"

	"github.com/jmoiron/sqlx"
	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"

	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/pkg/events"

	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	pbmessages "github.com/webitel/chat_manager/api/proto/chat/messages"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	"github.com/webitel/chat_manager/internal/keyboard"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/util"
)

type Service interface {
	GetConversations(ctx context.Context, req *pbchat.GetConversationsRequest, res *pbchat.GetConversationsResponse) error
	GetConversationByID(ctx context.Context, req *pbchat.GetConversationByIDRequest, res *pbchat.GetConversationByIDResponse) error
	GetHistoryMessages(ctx context.Context, req *pbchat.GetHistoryMessagesRequest, res *pbchat.GetHistoryMessagesResponse) error

	SendMessage(ctx context.Context, req *pbchat.SendMessageRequest, res *pbchat.SendMessageResponse) error
	// [WTEL-4695]: duct tape, please make me normal when chats will be rewrited (agent join message knows only webitel.chat.bot)
	SaveAgentJoinMessage(context.Context, *pbchat.SaveAgentJoinMessageRequest, *pbchat.SaveAgentJoinMessageResponse) error
	DeleteMessage(ctx context.Context, req *pbchat.DeleteMessageRequest, res *pbchat.HistoryMessage) error
	StartConversation(ctx context.Context, req *pbchat.StartConversationRequest, res *pbchat.StartConversationResponse) error
	CloseConversation(ctx context.Context, req *pbchat.CloseConversationRequest, res *pbchat.CloseConversationResponse) error
	JoinConversation(ctx context.Context, req *pbchat.JoinConversationRequest, res *pbchat.JoinConversationResponse) error
	LeaveConversation(ctx context.Context, req *pbchat.LeaveConversationRequest, res *pbchat.LeaveConversationResponse) error
	InviteToConversation(ctx context.Context, req *pbchat.InviteToConversationRequest, res *pbchat.InviteToConversationResponse) error
	DeclineInvitation(ctx context.Context, req *pbchat.DeclineInvitationRequest, res *pbchat.DeclineInvitationResponse) error
	WaitMessage(ctx context.Context, req *pbchat.WaitMessageRequest, res *pbchat.WaitMessageResponse) error
	CheckSession(ctx context.Context, req *pbchat.CheckSessionRequest, res *pbchat.CheckSessionResponse) error
	UpdateChannel(ctx context.Context, req *pbchat.UpdateChannelRequest, res *pbchat.UpdateChannelResponse) error
	GetChannelByPeer(ctx context.Context, request *pbchat.GetChannelByPeerRequest, res *pbchat.Channel) error

	SetVariables(ctx context.Context, in *pbchat.SetVariablesRequest, out *pbchat.ChatVariablesResponse) error
	BlindTransfer(ctx context.Context, in *pbchat.ChatTransferRequest, out *pbchat.ChatTransferResponse) error

	SendUserAction(ctx context.Context, in *pbchat.SendUserActionRequest, out *pbchat.SendUserActionResponse) error
	BroadcastMessage(ctx context.Context, in *pbmessages.BroadcastMessageRequest, out *pbmessages.BroadcastMessageResponse) error
	BroadcastMessageNA(ctx context.Context, in *pbmessages.BroadcastMessageRequest, out *pbmessages.BroadcastMessageResponse) error
}

type chatService struct {
	repo          pg.Repository
	log           *slog.Logger
	flowClient    flow.Client
	authClient    auth.Client
	botClient     pbbot.BotsService
	storageClient pbstorage.FileService
	eventRouter   event.Router
}

var _ pbchat.ChatServiceHandler = (*chatService)(nil)

func NewChatService(
	repo pg.Repository,
	log *slog.Logger,
	flowClient flow.Client,
	authClient auth.Client,
	botClient pbbot.BotsService,
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

func (s *chatService) GetChannelByPeer(ctx context.Context, req *pbchat.GetChannelByPeerRequest, res *pbchat.Channel) error {
	fromId := strconv.FormatInt(req.GetFromId(), 10)
	channel, err := s.repo.GetChannelByPeer(ctx, req.GetPeerId(), fromId)
	if err != nil {
		return err
	}
	res.Id = channel.ID
	res.Internal = channel.Internal
	res.Connection = channel.Connection.String
	res.Type = channel.Type

	md, err := json.Marshal(channel.Variables)
	if err != nil {
		return err
	}
	res.Props = string(md)
	return nil
}

func (s *chatService) UpdateChannel(
	ctx context.Context,
	req *pbchat.UpdateChannelRequest,
	res *pbchat.UpdateChannelResponse,
) error {

	var (
		channelChatID = req.GetChannelId()
		channelFromID = req.GetAuthUserId()

		messageAt = req.GetReadUntil() // Implies last seen message.created_at date
		localtime = app.CurrentTime()
		readUntil = localtime // default: ALL
	)
	wlog.TraceLog(s.log, "UPDATE Channel",
		slog.String("channel_id", channelChatID),  // TODO fields diff
		slog.Int64("auth_user_id", channelFromID), // TODO fields diff
		slog.Int64("read_until", messageAt),
	)

	// PERFORM find sender channel
	channel, err := s.repo.CheckUserChannel(
		ctx, channelChatID, channelFromID,
	)

	if err != nil {

		s.log.Error("FAILED Lookup Channel",
			slog.Any("error", err),
			slog.String("chat-id", channelChatID),   // TODO fields diff
			slog.Int64("contact-id", channelFromID), // TODO fields diff
		)

		return err
	}

	if channel == nil {
		s.log.Warn("Channel NOT Found",
			slog.String("chat-id", channelChatID),   // TODO fields diff
			slog.Int64("contact-id", channelFromID), // TODO fields diff
		)

		return errors.BadRequest(
			"chat.channel.not_found",
			"chat: channel ID=%s not found",
			channelChatID,
		)
	}

	if messageAt != 0 {
		// FIXME: const -or- app.TimePrecision ?
		const divergence = time.Millisecond

		readUntil = app.TimestampDate(messageAt)
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
	req *pbchat.SendMessageRequest,
	res *pbchat.SendMessageResponse,
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

	// log := s.log.With(
	// 	slog.String("channel_id", senderChatID),
	// 	slog.String("conversation_id", targetChatID),
	// 	slog.Int64("auth_user_id", senderFromID),
	// 	slog.String("type", sendMessage.GetType()),
	// 	slog.String("text", sendMessage.GetText()),
	// 	slog.Any("file", sendMessage.GetFile()))

	// log.Debug("SEND Message")

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
			"send: FROM channel ID=%s user ID=%d mismatch",
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
		// log.Error("FAILED Sending Message",
		// 	slog.Any("error", err),
		// )
		return err
	}

	// NOTE: normalized during .saveMessage() function
	sentMessage := sendMessage
	res.Message = sentMessage

	return nil
}

func (s *chatService) SaveAgentJoinMessage(ctx context.Context, req *pbchat.SaveAgentJoinMessageRequest, rsp *pbchat.SaveAgentJoinMessageResponse) error {

	var (
		sendMessage   *pbchat.Message
		webitelUserID int64
	)
	if req.GetMessage() == nil {
		return errors.BadRequest("chat.service.save_agent_join_message.check_args.message", "message required")
	}
	sendMessage = req.GetMessage()

	if sendMessage.GetFrom() == nil {
		return errors.BadRequest("chat.service.save_agent_join_message.check_args.message.from", "message.from required")
	}

	webitelUserID = sendMessage.From.GetId()
	if webitelUserID == 0 {
		return errors.BadRequest("chat.service.save_agent_join_message.check_args.message.from.id", "message.from.id required")
	}

	log := s.log.With(
		slog.Int64("webitel_user", webitelUserID),
		slog.String("type", sendMessage.GetType()),
		slog.String("text", sendMessage.GetText()),
		slog.Any("file", sendMessage.GetFile()),
	)

	log.Debug("SEND Message")

	// region: lookup target chat session by unique webitel user id
	chat, err := s.repo.GetSessionByInternalUserId(ctx, webitelUserID, req.GetReceiver())

	if err != nil {
		// lookup operation error
		return err
	}

	if chat == nil {
		// sender channel ID not found
		return errors.BadRequest(
			"chat.save_agent_join_message.channel.from.not_found",
			"send: FROM user ID=%d sender not found or been closed",
			webitelUserID,
		)
	}

	//if senderFromID != 0 && chat.User.ID != senderFromID {
	//	// mismatch sender contact ID
	//	return errors.BadRequest(
	//		"chat.save_agent_join_message.user.mismatch",
	//		"send: FROM channel ID=%s user ID=%d mismatch",
	//		senderChatID, senderFromID,
	//	)
	//}

	if chat.IsClosed() {
		// sender channel is already closed !
		return errors.BadRequest(
			"chat.save_agent_join_message.channel.from.closed",
			"send: FROM chat channel ID=%s is closed",
			chat.ID,
		)
	}

	sender := chat.Channel

	// save message from the sender perspective (remove me or do from the flow in the future)
	_, err = s.saveMessage(ctx, nil, sender, sendMessage)

	if err != nil {
		// Failed to store message or validation error !
		return err
	}

	// PERFORM message publish|broadcast
	_, err = s.notifyAgentJoinToAllMembers(ctx, chat, sendMessage)

	if err != nil {
		log.Error("FAILED Notify Websocket",
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

func (s *chatService) DeleteMessage(
	ctx context.Context,
	req *pbchat.DeleteMessageRequest,
	res *pbchat.HistoryMessage,
) error {

	var (
		dialogChatID = req.GetConversationId() // TO: Dialog.ID
		senderChatID = req.GetChannelId()      // FROM: Chat.ID
		senderFromID = req.GetAuthUserId()     // FROM: User.ID
	)

	log := s.log.With(
		slog.String("conversation_id", dialogChatID),
		slog.String("channel_id", senderChatID),
		slog.Int64("auth_user_id", senderFromID),
	)
	log.Debug("DEL Message")

	msg, err := s.repo.GetMessage(
		ctx, req.Id,
		senderChatID, dialogChatID,
		req.GetVariables(),
	)

	if err != nil {
		return err
	}

	lookupMsg := func() (fmt string) {
		if req.Id != 0 {
			fmt += " mid:" + strconv.FormatInt(req.Id, 10) + ";"
		}
		if req.AuthUserId != 0 {
			fmt += " user:" + strconv.FormatInt(req.AuthUserId, 10) + ";"
		}
		if req.ChannelId != "" {
			fmt += " from:" + req.ChannelId + ";"
		}
		if req.ConversationId != "" {
			fmt += " chat:" + req.ConversationId + ";"
		}
		for key, val := range req.Variables {
			fmt += " " + key + ":" + val + ";"
		}
		return // fmt
	}

	if msg == nil || (req.Id != 0 && msg.ID != req.Id) {
		return errors.BadRequest(
			"chat.message.not_found",
			"message: not found; %s",
			lookupMsg(),
		)
	}

	if dialogChatID != "" && msg.ConversationID != dialogChatID {
		// sender dialog ID NOT MATCH !
		return errors.BadRequest(
			"chat.message.delete.forbidden",
			"delete: invalid dialog; message:%s",
			lookupMsg(),
		)
	}
	dialogChatID = msg.ConversationID

	if senderChatID != "" && msg.ChannelID != senderChatID {
		// sender channel ID NOT MATCH !
		return errors.BadRequest(
			"chat.message.delete.forbidden",
			"delete: sender required; message:%s",
			lookupMsg(),
		)
	}
	senderChatID = msg.ChannelID

	// region: lookup target chat session by unique sender chat channel id
	dialog, err := s.repo.GetSession(ctx, senderChatID) // by: sender

	if err != nil {
		// lookup operation error
		return err
	}

	sender := dialog.GetMember(senderChatID)
	if sender == nil {
		return errors.BadRequest(
			"chat.message.delete.forbidden",
			"delete: sender not sound; message:%s",
			lookupMsg(),
		)
	}

	if senderFromID != 0 && sender.User.ID != senderFromID {
		return errors.BadRequest(
			"chat.message.delete.forbidden",
			"delete: sender required; message:%s",
			lookupMsg(),
		)
	}
	// Sender(Owner): APPROVED !
	req.Id = msg.ID // Disclose mid= in error(s) from now on ...
	n, err := s.repo.DeleteMessages(ctx, msg.ID)
	if err == nil && n != 1 {
		err = errors.InternalServerError(
			"chat.message.delete.none",
			"message: none; message:%s",
			lookupMsg(),
		)
	}
	if err != nil {
		return err
	}

	// TODO: Notify ALL dialog's members ...
	deleted := transformMessageFromRepoModel(msg)
	_ = s.eventRouter.RouteMessageDeleted(
		dialog, deleted,
	)

	*(res) = *(deleted)
	return nil
}

// StartConversation starts NEW chat@bot(workflow/schema) session
// ON one side there will be req.Username with the start req.Message channel as initiator (leg: A)
// ON other side there will be flow_manager.schema (chat@bot) channel to communicate with
func (s *chatService) StartConversation(
	ctx context.Context,
	req *pbchat.StartConversationRequest,
	res *pbchat.StartConversationResponse,
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
		serviceNodeID   = md["Micro-From-Host"]    // md["Micro-From-Id"]      // provider channel host !
	)

	// FIXME:
	if serviceProvider != "webitel.chat.bot" {
		// LOG: this is the only case expected for now !..
		// "go.webitel.portal" !!!
	}

	metadata := req.GetProperties()
	if len(metadata) != 0 {
		// Clear invalid (empty) key !
		delete(metadata, "")
	}

	log := s.log.With(
		slog.Int64("domain.id", req.GetDomainId()),
		slog.String("user.contact", user.GetConnection()),
		slog.String("user.type", user.GetType()),
		slog.Int64("user.id", user.GetUserId()),
		slog.String("user.name", title),
		slog.Bool("user.internal", user.GetInternal()),
	)

	log.Debug("START Conversation")

	// ORIGINATOR: CHAT channel, sender
	channel := pg.Channel{

		Type:   req.GetUser().GetType(),
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
		Variables: metadata, // req.GetMessage().GetVariables(), // req.GetProperties(),
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
		// Metadata chaining ...
		Variables: metadata,
	}

	// CHAT start message
	startMessage := req.GetMessage()
	if startMessage == nil {
		// FIXME: imit service /start command message
		startMessage = &pbchat.Message{
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
		case "contact":

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
				ID:      channel.ID,
				Title:   channel.Name,
				Channel: channel.Type,
				// Contact:   "",
				// Username:  "",
				// FirstName: "",
				// LastName:  "",
				Invite: conversation.ID,
			},
			User: &app.User{
				ID:      channel.UserID,
				Channel: channel.Type,
				// Contact:   "",
				FirstName: title,
				// LastName:  "",
				// UserName:  "",
				// Language:  "",
			},
			DomainID: channel.DomainID,
			// Status:   "",
			// Provider: nil,
			Created: app.DateTimestamp(channel.CreatedAt),
			Updated: app.DateTimestamp(channel.UpdatedAt),
			// Joined:   0,
			// Closed:   0,
			Variables: channel.Variables,
		}

		// Save historical START conversation message ...
		if _, err := s.saveMessage(ctx, tx, &sender, startMessage); err != nil {
			return err
		}

		res.ConversationId = conversation.ID
		res.ChannelId = channel.ID
		// sentMessage := startMessage
		res.Message = startMessage
		// TODO: return error from s.flowClient.Init(..) !!!!!!!!!!!!!!!
		//       to be able to ROLLBACK DB changes
		//       when got "go.micro.client; service: not found" error
		return nil
	}); err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
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

// CloseConversation used by the flow or the client to entirely close whole conversation
func (s *chatService) CloseConversation(
	ctx context.Context,
	req *pbchat.CloseConversationRequest,
	res *pbchat.CloseConversationResponse,
) error {
	cause := req.GetCause().String()
	log := s.log.With(
		slog.String("conversation_id", req.GetConversationId()),
		slog.String("closer_channel_id", req.GetCloserChannelId()),
		slog.String("cause", cause),
	)
	log.Debug("CLOSE Conversation")

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

	_, err = s.sendChatClosed(ctx, chat, cause)

	if err != nil {
		log.Error("FAILED Notify Chat Members",
			slog.Any("error", err),
		)
		// return err
	}

	// Mark ALL chat members as CLOSED !
	// NOTE: - delete: chat.confirmation; - delete: chat.flow.node
	//
	// close conversation should close all the derived channels with given reason
	// possible reasons
	// user_timeout - setting defined by queue violated (max client response time)
	// agent_timeout - setting defined by queue violated (max agent response time)
	// flow_end - given scheme ended
	// flow_err - given scheme returned error
	// client_leave - client wrote /close
	// TODO: i think the good idea will be to use reasons here to duplicate to the conversation (or not) actually there are two ways of fully ending conversation
	// TODO: flow_end, flow_err or client_leave
	err = s.closeConversation(ctx, &targetChatID, cause)
	if err != nil {
		log.Error("Failed to close chat channels",
			slog.Any("error", err),
		)
		return err
	}

	// +OK
	return nil
}

/*func (s *chatService) CloseConversation(
	ctx context.Context,
	req *pbchat.CloseConversationRequest,
	res *pbchat.CloseConversationResponse,
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
	req *pbchat.JoinConversationRequest,
	res *pbchat.JoinConversationResponse,
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

	log := s.log.With(
		slog.Int64("user_id", from),
		slog.String("invite_id", token),
	)

	log.Debug("JOIN Conversation")

	invite, err := s.repo.GetInviteByID(ctx, token)

	if err != nil {
		log.Error("FAILED Lookup INVITE token",
			slog.Any("error", err),
		)
		return err
	}

	found := invite != nil
	found = found && invite.ID == token            // req.InviteId
	found = found && invite.UserID == from         // req.AuthUserId
	found = found && invite.ClosedAt.Time.IsZero() // NOT Closed !

	if !found {
		// s.log.Warn().Msg("invitation not found")
		return errors.NotFound(
			"chat.invite.not_found",
			"join: invite token %s is invalid or already used",
			token,
		)
	}

	user, err := s.repo.GetWebitelUserByID(ctx, from, invite.DomainID)

	if err != nil {
		log.Error("FAILED Lookup Chat User",
			slog.Any("error", err),
			slog.Int64("user_id", invite.UserID),
			slog.Int64("domain_id", invite.DomainID),
		)
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

	timestamp := app.CurrentTime().UTC()

	channel := &pg.Channel{
		ID:             invite.ID, // FROM: INVITE token !
		Type:           "webitel",
		Internal:       true,
		ConversationID: invite.ConversationID,
		UserID:         invite.UserID,
		DomainID:       invite.DomainID,
		// Name:           user.Name,
		Name: user.ChatName,
		PublicName: sql.NullString{
			String: user.ChatName,
			Valid:  user.ChatName != "",
		},
		CreatedAt: invite.CreatedAt,
		UpdatedAt: timestamp,
		JoinedAt: sql.NullTime{
			Time:  timestamp,
			Valid: true,
		},
		Variables: invite.Variables,
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
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}

	// NOTIFY Related Chat Members !
	err = s.eventRouter.RouteJoinConversation(
		channel, &invite.ConversationID,
	)

	if err != nil {
		s.log.Error("FAILED Notify Chat Members",
			slog.Any("error", err),
			slog.String("event", "new_chat_member"),
			slog.String("invite_id", invite.ID), // TODO: same as NEW channel.ID
			slog.String("conversation_id", invite.ConversationID),
			slog.Int64("user_id", invite.UserID),
		)
		// return err // NON Fatal !
	}

	return nil
}

func (s *chatService) leaveChat(ctx context.Context, req *pbchat.LeaveConversationRequest, breakBridge flow.BreakBridgeCause) error {

	var (
		channelChatID  = req.GetChannelId()
		channelFromID  = req.GetAuthUserId()
		conversationID = req.GetConversationId()
		leaveCause     = req.GetCause()
	)

	log := s.log.With(
		slog.String("channel_id", channelChatID),
		slog.Int64("auth_user_id", channelFromID),
		slog.String("conversation_id", conversationID),
	)

	log.Debug("LEAVE Conversation")

	sender, err := s.repo.CheckUserChannel(
		ctx, channelChatID, channelFromID,
	)

	if err != nil {
		log.Error("FAILED Lookup CHAT Channel",
			slog.Any("error", err),
		)
		return err
	}

	found := sender != nil

	found = found && strings.EqualFold(sender.ID, channelChatID)
	found = found && sender.UserID == channelFromID
	// found = found && sender.ClosedAt.Time.IsZero() // NOT Closed yet !
	found = found && (conversationID == "" || strings.EqualFold(sender.ConversationID, conversationID))

	if !found {
		return errors.NotFound(
			"chat.leave.channel.from.not_found",
			"chat: leave FROM channel ID=%s user ID=%d not found or been closed",
			channelChatID, channelFromID,
		)
	}

	// if conversationID != "" {
	// 	if conversationID != sender.ConversationID {
	// 		s.log.Warn().Msg("channel.conversation_id mismatch")
	// 		return errors.BadRequest("channel.conversation_id mismatch", "")
	// 	}
	// }

	// ----- PERFORM ---------------------------------
	// 1. Mark given .channel.id as "closed" !
	closed, err := s.repo.CloseChannel(ctx, sender.ID, leaveCause.String()) // channelChatID)
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}

	if closed == nil {
		// NOTE: NOT FOUND -or- already been CLOSED
		// Loyal and idempotent !
		return nil // OK
	}

	// SYNC // NOT PARALLEL
	await := make(chan error, 2)
	for _, async := range []func(){
		func() {

			// await <- s.flowClient.BreakBridge(
			// 	sender.ConversationID, breakBridge,
			// )
			err := s.flowClient.BreakBridge(
				sender.ConversationID, breakBridge,
			)
			if re := errors.FromError(err); re != nil {
				if re.Detail == "bridge not found" {
					err = nil // Acceptable; Ignore !
				}
			}
			await <- err // DONE: workflow.BreakBridge();
			// },
			// func() {
			// omitted ? populate breakBridge cause
			var leaveNotify string
			if breakBridge != flow.LeaveConversationCause {
				leaveNotify = string(breakBridge)
			}

			// NOTIFY: All related CHAT member(s) !
			await <- s.eventRouter.RouteLeaveConversation(
				closed, &sender.ConversationID, leaveNotify,
			)

		},
	} {
		go async()
	}

	for i := 0; i < 2; i++ {
		if err = <-await; err != nil {
			s.log.Error("FAILED Notify Chat Members",
				slog.String("event", "left_chat_member"),
				slog.String("channel_id", sender.ID),
				slog.Int64("user_id", sender.UserID),
				slog.String("conversation_id", sender.ConversationID),
			)
			// return err // NON Fatal !
		}
	}
	close(await)

	/*/ parallel
	resErrorsChan := make(chan error, 2)
	go func() {
		if closed.FlowBridge {
			if err := s.flowClient.BreakBridge(conversationID, flow.LeaveConversationCause); err != nil {
				resErrorsChan <- err
				return
			}
		}
		resErrorsChan <- nil
	}()
	go func() {
		if err := s.eventRouter.RouteLeaveConversation(closed, &conversationID); err != nil {
			resErrorsChan <- err
		} else {
			resErrorsChan <- nil
		}
	}()
	for i := 0; i < 2; i++ {
		if err := <-resErrorsChan; err != nil {
			s.log.Error().Msg(err.Error())
			// return err
		}
	}*/
	return nil
}

// LeaveConversation means the agent leaved conversation and the initiator channel must be closed (cause reason agent_leave)
func (s *chatService) LeaveConversation(
	ctx context.Context,
	req *pbchat.LeaveConversationRequest,
	res *pbchat.LeaveConversationResponse,
) error {
	// use the incoming kick reason (from engine or call_center) in database
	return s.leaveChat(ctx, req, flow.LeaveConversationCause)
}

func (s *chatService) InviteToConversation(
	ctx context.Context,
	req *pbchat.InviteToConversationRequest,
	res *pbchat.InviteToConversationResponse,
) error {
	// _, err := s.repo.GetChannelByID(ctx, req.InviterChannelId)
	// if err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return err
	// }

	metadata := req.GetVariables()
	if len(metadata) != 0 {
		// Remove invalid (empty) key !
		delete(metadata, "")
	}

	log := s.log.With(
		slog.String("conversation_id", req.GetConversationId()),
		slog.String("user.connection", req.GetUser().GetConnection()),
		slog.String("user.type", req.GetUser().GetType()),
		slog.Bool("user.internal", req.GetUser().GetInternal()),
		slog.String("inviter_channel_id", req.GetInviterChannelId()),
		slog.Int64("domain_id", req.GetDomainId()),
		slog.Int64("timeout_sec", req.GetTimeoutSec()),
		slog.Int64("auth_user_id", req.GetAuthUserId()),
		slog.Any("variables", metadata),
	)

	log.Debug("INVITE TO Conversation")

	servName := s.authClient.GetServiceName(&ctx)
	if servName != "workflow" &&
		(req.GetInviterChannelId() == "" || req.GetAuthUserId() == 0) {
		log.Error("failed auth")
		return errors.BadRequest("failed auth", "")
	}

	domainID := req.GetDomainId()
	invite := &pg.Invite{

		UserID:         req.GetUser().GetUserId(),
		DomainID:       domainID,
		TimeoutSec:     req.GetTimeoutSec(),
		ConversationID: req.GetConversationId(),

		Variables: metadata,
	}
	if title := req.GetTitle(); title != "" {
		invite.Title = sql.NullString{
			String: title, Valid: true,
		}
	}
	if req.GetInviterChannelId() != "" {
		channel, err := s.repo.CheckUserChannel(ctx, req.GetInviterChannelId(), req.GetAuthUserId())
		if err != nil {
			log.Error(err.Error(),
				slog.Any("error", err),
			)
			return err
		}
		if channel == nil {
			log.Warn("channel not found")
			return errors.BadRequest("channel not found", "")
		}
		invite.InviterChannelID = sql.NullString{
			String: req.GetInviterChannelId(), Valid: true,
		}
	}
	if err := s.repo.CreateInvite(ctx, invite); err != nil {
		log.Error("FAILED Create INVITE Token",
			slog.Any("error", err),
		)
		return err
	}
	conversation, err := s.repo.GetConversations(ctx, req.GetConversationId(), 0, 0, nil, nil, 0, false, 0, 0)
	if err != nil {
		log.Error("FAILED Lookup Conversation",
			slog.Any("error", err),
		)
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
	for _, async := range []func(){
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
			log.Error("FAILED Notify Chat Members",
				slog.Any("error", err),
			)
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

			ilog := s.log.With(
				slog.String("invite_id", invite.ID),
				slog.Int64("user_id", invite.UserID),
				slog.String("conversation_id", invite.ConversationID),
			)

			if err != nil {
				ilog.Error("FAILED Closing INVITE",
					slog.Any("error", err),
				)
				return
			}

			if !closed {
				// NOTE: invalid invite_id, already closed or joined !
				return
			}
			// NOTE: closed !
			ilog.Warn("INVITE Timeout")

			if req.InviterChannelId == "" { // FROM: workflow !

				err = s.flowClient.BreakBridge(
					req.ConversationId, flow.TimeoutCause,
				)

				if err != nil {
					ilog.Error(err.Error(),
						slog.Any("error", err),
					)
				}
			}
			// NOTIFY: timed out !
			err = s.eventRouter.SendDeclineInviteToWebitelUser(
				&domainID, &invite.ConversationID, &invite.UserID,
				&invite.ID, string(flow.TimeoutCause),
			)

			if err != nil {
				ilog.Error("FAILED Notify User INVITE Timeout",
					slog.Any("error", err),
				)
			}

		}()
	}
	res.InviteId = invite.ID
	return nil
}

func (s *chatService) DeclineInvitation(
	ctx context.Context,
	req *pbchat.DeclineInvitationRequest,
	res *pbchat.DeclineInvitationResponse,
) error {

	userID := req.GetAuthUserId()
	conversationID := req.GetConversationId()

	log := s.log.With(
		slog.String("invite_id", req.GetInviteId()),
		slog.String("conversation_id", conversationID),
		slog.Int64("auth_user_id", userID),
	)

	log.Debug("DECLINE Invitation")

	invite, err := s.repo.GetInviteByID(ctx, req.GetInviteId())

	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
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
	for _, async := range []func(){
		func() {
			// NOTIFY: All related Chat members !
			await <- s.eventRouter.RouteDeclineInvite(
				&invite.UserID, &invite.ConversationID,
			)
		},
		func() {
			// NOTIFY: Invited User session(s) !
			await <- s.eventRouter.SendDeclineInviteToWebitelUser(
				&invite.DomainID, &invite.ConversationID, &invite.UserID,
				&invite.ID, req.GetCause(), // optional: custom
			)
		},
	} {
		go async()
	}

	for i := 0; i < 2; i++ {
		if err = <-await; err != nil {
			s.log.Error("FAILED Notify Chat Members",
				slog.Any("error", err),
				slog.String("event", "declined"),
				slog.String("invite_id", invite.ID),
				slog.String("conversation_id", invite.ConversationID),
				slog.Int64("user_id", invite.UserID),
			)
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

func (s *chatService) WaitMessage(ctx context.Context, req *pbchat.WaitMessageRequest, res *pbchat.WaitMessageResponse) error {

	s.log.Debug(
		"[ CHAT::FLOW ] WAIT message",
		"conversation_id", req.GetConversationId(),
		"confirmation_id", req.GetConfirmationId(),
	)
	// cachedMessages, err := s.chatCache.ReadCachedMessages(req.GetConversationId())
	// if err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return err
	// }
	// if cachedMessages != nil {
	// 	messages := make([]*pbchat.Message, 0, len(cachedMessages))
	// 	var tmp *pbchat.Message
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
	err := s.flowClient.WaitMessage(req.GetConversationId(), req.GetConfirmationId())
	if err != nil {
		s.log.Error(
			"[ CHAT::FLOW ] WAIT message error",
			"conversation_id", req.GetConversationId(),
			"confirmation_id", req.GetConfirmationId(),
			"error", err,
		)
		return err
	}

	// if err := s.repo.WriteConfirmation(req.GetConversationId(), req.GetConfirmationId()); err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return err
	// }
	res.TimeoutSec = int64(timeout)
	return nil
}

// CheckSession performs:
//   - Locate OR Create client contact
//   - Identify whether exists channel for
//     requested chat-bot gateway profile.id
func (s *chatService) CheckSession(ctx context.Context, req *pbchat.CheckSessionRequest, res *pbchat.CheckSessionResponse) error {

	log := s.log.With(
		slog.String("external_id", req.GetExternalId()),
		slog.Int64("profile_id", req.GetProfileId()),
	)
	log.Debug("check session")

	contact, err := s.repo.GetClientByExternalID(ctx, req.GetExternalId())
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}

	if contact == nil {
		contact, err = s.createClient(ctx, req)
		if err != nil {
			log.Error(err.Error(),
				slog.Any("error", err),
			)
			return err
		}
		res.ClientId = contact.ID
		res.Account = &pbchat.Account{}
		res.Exists = false
		return nil
	}

	// Update contact fields: name
	if req.Username != "" && req.Username != contact.Name.String {
		contact.Name = sql.NullString{
			String: req.Username,
			Valid:  true,
		}

		err := s.updateClient(ctx, contact)
		if err != nil {
			// Log the error but do not interrupt execution
			log.Error("Failed to update client.name changes",
				slog.String("name", req.Username),
				slog.Any("error", err),
			)
		}
	}

	// profileStr := strconv.Itoa(int(req.GetProfileId()))
	var profileOf *string
	if oid := req.GetProfileId(); oid > 0 {
		profileId := strconv.FormatInt(oid, 10)
		if err != nil {
			log.Error(err.Error(),
				slog.Any("error", err),
			)
			return err
		}
		profileOf = &profileId
	}

	externalBool := false
	active := true
	channels, err := s.repo.GetChannels(ctx, &contact.ID, nil, profileOf, &externalBool, nil, &active)
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}

	if len(channels) != 0 {
		channel := channels[0]
		res.ClientId = contact.ID
		res.Account = &pbchat.Account{
			Id:        contact.ID,
			Channel:   channel.Type,
			Contact:   contact.ExternalID.String,
			FirstName: contact.Name.String,
			LastName:  "",
			Username:  "",
		}
		res.ChannelId = channel.ID
		res.Exists = channel.ID != ""
		res.Properties = channel.Variables
	} else {
		res.ClientId = contact.ID
		res.Account = &pbchat.Account{
			Id:        contact.ID,
			Channel:   "", // unknown
			Contact:   contact.ExternalID.String,
			FirstName: contact.Name.String,
			LastName:  "",
			Username:  "",
		}
		res.Exists = false
	}

	return nil
}

func (s *chatService) GetConversations(ctx context.Context, req *pbchat.GetConversationsRequest, res *pbchat.GetConversationsResponse) error {

	log := s.log.With(
		slog.String("conversation_id", req.GetId()),
	)
	log.Debug("get conversations")

	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
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
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}
	res.Items = transformConversationsFromRepoModel(conversations)
	return nil
}

func (s *chatService) GetConversationByID(ctx context.Context, req *pbchat.GetConversationByIDRequest, res *pbchat.GetConversationByIDResponse) error {
	log := s.log.With(
		slog.String("conversation_id", req.GetId()),
	)
	log.Debug("get conversation by id")

	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}
	conversation, err := s.repo.GetConversations(ctx, req.GetId(), 0, 0, nil, nil, user.DomainID, false, 0, 0)
	//conversation, err := s.repo.GetConversationByID(ctx, req.GetId())
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}
	if conversation == nil {
		return nil
	}
	res.Item = transformConversationFromRepoModel(conversation[0])
	return nil
}

func (s *chatService) GetHistoryMessages(ctx context.Context, req *pbchat.GetHistoryMessagesRequest, res *pbchat.GetHistoryMessagesResponse) error {
	log := s.log.With(
		slog.String("conversation_id", req.GetConversationId()),
	)
	log.Debug("get history")

	user, err := s.authClient.MicroAuthentication(&ctx)
	if err != nil {
		log.Error(err.Error(),
			slog.Any("error", err),
		)
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
		log.Error(err.Error(),
			slog.Any("error", err),
		)
		return err
	}
	res.Items = transformMessagesFromRepoModel(messages)
	return nil
}

// func (c *chatService) saveMessage(ctx context.Context, dcx sqlx.ExtContext, senderChatID string, targetChatID string, notify *pb.Message) (saved *pg.Message, err error) {
func (c *chatService) saveMessage(ctx context.Context, dcx sqlx.ExtContext, sender *app.Channel, notify *pbchat.Message) (saved *pg.Message, err error) {

	var (
		sendMessage = notify

		senderChatID = sender.Chat.ID
		targetChatID = sender.Chat.Invite

		localtime = app.CurrentTime()
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
		forwardFromBinding   = sendMessage.ForwardFromVariables

		replyToMessageID = sendMessage.ReplyToMessageId
		replyToBinding   = sendMessage.ReplyToVariables

		// FORWARD operation purpose ?
		forward = forwardFromMessageID != 0 ||
			len(forwardFromBinding) != 0

		// REPLY operation purpose ?
		reply = replyToMessageID != 0 ||
			len(replyToBinding) != 0 // || sendMessage.Postback.GetMid() > 0

		// EDIT operation purpose ?
		edit = sendMessage.UpdatedAt != 0

		// Store (Saved) Message Model
		saveMessage *pg.Message
	)

	// Normalize lookup message bindings if provided
	for _, findBinding := range []*map[string]string{
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

	// if replyToMessageID == 0 {
	// 	replyToMessageID = sendMessage.Postback.GetMid()
	// }
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

		// Custom [message.type] classifier. Optional.
		saveMessage.Kind = strings.TrimSpace(sendMessage.Kind)
		// [RAW]: Message Content details
		saveMessage.Contact = notify.Contact
		// Quick Reply Button(s) ?
		saveMessage.Keyboard, _ = keyboard.MarkupV2(
			notify.Buttons,
		)
		// Disable `input` request ?
		if notify.NoInput && saveMessage.Keyboard != nil {
			// Can only be used with a set of `Buttons` !
			saveMessage.Keyboard.NoInput = len(saveMessage.Keyboard.Buttons) > 0
		}
		postback := notify.Postback
		if postback.GetCode() != "" {
			saveMessage.Postback = &pbmessages.Postback{
				Mid:  postback.Mid,
				Code: postback.Code,
				Text: postback.Text,
			}
			if saveMessage.Text == "" {
				saveMessage.Text = postback.Text
			}
		}
	}

	log := c.log
	if saveMessage != nil {
		log = log.With(
			slog.String("conversation_id", saveMessage.ConversationID),
			slog.String("channel_id", saveMessage.ChannelID),
			slog.String("type", saveMessage.Type),
			slog.String("kind", saveMessage.Kind),
		)
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
			var forwardFrom interface{} = forwardFromMessageID
			if forwardFromMessageID == 0 {
				forwardFrom = forwardFromBinding
			}
			err = errors.BadRequest(
				"chat.message.lookup.error",
				"forward: message %v lookup: %s",
				forwardFrom, err,
			)

			log.Warn("FORWARD[FROM]",
				slog.Any("sender", sender.Chat),
				slog.Any("error", err),
			)
			forwardMessage = nil
			err = nil // continue
			// return nil, errors.BadRequest(
			// 	"chat.message.lookup.error",
			// 	"forward: message ID=%d lookup: %s",
			// 	 forwardFromMessageID, err,
			// )
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
			var forwardFrom interface{} = forwardFromMessageID
			if forwardFromMessageID == 0 {
				forwardFrom = forwardFromBinding
			}
			// return nil, errors.BadRequest(
			// 	"chat.forward.message.not_found",
			// 	"forward: original message %v not found",
			// 	 forwardFrom,
			// )
			err = errors.BadRequest(
				"chat.forward.message.not_found",
				"forward: original message %v not found",
				forwardFrom,
			)
			log.Warn("FORWARD[FROM]",
				slog.Any("sender", sender.Chat),
				slog.Any("error", err),
			)
			err = nil // continue

		} else {

			// MARK message FORWARDED !
			saveMessage.ForwardFromMessageID = forwardMessage.ID
			// COPY Original Message Source !
			saveMessage.Type = forwardMessage.Type
			saveMessage.Text = forwardMessage.Text
			saveMessage.File = forwardMessage.File

			// Populate result message payload !
			sendMessage.ForwardFromMessageId = forwardMessage.ID
			sendMessage.ForwardFromChatId = forwardMessage.ConversationID
			// Forward Message Payload
			sendMessage.Type = forwardMessage.Type
			sendMessage.Text = forwardMessage.Text
			if doc := forwardMessage.File; doc != nil {
				sendMessage.File = &pbchat.File{
					Id:   doc.ID,
					Url:  "",
					Size: doc.Size,
					Mime: doc.Type,
					Name: doc.Name,
				}
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
				// return nil, errors.BadRequest(
				// 	"chat.message.lookup.error",
				// 	"reply: message ID=%d lookup: %s",
				// 	 replyToMessageID, err,
				// )
				var replyTo interface{} = replyToMessageID
				if replyToMessageID == 0 {
					replyTo = replyToBinding
				}
				err = errors.BadRequest(
					"chat.message.lookup.error",
					"reply: message %v lookup: %s",
					replyTo, err,
				)
				log.Warn("REPLY[TO]",
					slog.Any("sender", sender.Chat),
					slog.Any("error", err),
				)
				replyToMessage = nil
				err = nil // continue
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
				// return nil, errors.BadRequest(
				// 	"chat.reply.message.not_found",
				// 	"reply: original message ID=%d not found",
				// 	 replyToMessageID,
				// )
				var replyTo interface{} = replyToMessageID
				if replyToMessageID == 0 {
					replyTo = replyToBinding
				}
				err = errors.BadRequest(
					"chat.reply.message.not_found",
					"reply: original message %v not found",
					replyTo,
				)
				log.Warn("REPLY[TO]",
					slog.Any("sender", sender.Chat),
					slog.Any("error", err),
				)
				err = nil // continue

			} else {

				// MARK message as REPLY !
				saveMessage.ReplyToMessageID = replyToMessage.ID

				// Disclose operation details
				sendMessage.ReplyToMessageId = replyToMessage.ID

			}
		}
	}

	saveBinding := sendMessage.Variables
	// NOTE: Hide bindings from recepients, because this implies system request info !
	// sendMessage.Variables = nil

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
	// Save as message.variables["kind"]
	if saveMessage.Kind != "" {
		if saveMessage.Variables == nil {
			saveMessage.Variables = make(pg.Metadata)
		}
		saveMessage.Variables["kind"] = saveMessage.Kind
	}

	// endregion

	// region: POST- processing: validate result message

	messageType := sendMessage.Type
	messageType = strings.TrimSpace(messageType)
	messageType = strings.ToLower(messageType)
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
			// } else if sendMessage.Postback != nil {
			// 	sendMessage.Type = "postback"
		} else {
			sendMessage.Type = "text"
		}

	}

	switch sendMessage.Type {

	case "text":

		text := sendMessage.Text
		postback := sendMessage.Postback
		// coalesce(...)
		for _, vs := range []string{
			sendMessage.Text,
			postback.GetText(),
			postback.GetCode(),
		} {
			vs = strings.TrimSpace(vs)
			if vs != "" {
				text = vs
				break
			}
		}

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
		// Button click[ed] ?
		if postback.GetCode() != "" {
			saveMessage.Postback = &pbmessages.Postback{
				Mid:  postback.Mid,
				Code: postback.Code,
				Text: postback.Text,
			}
		}

	// case "buttons", "inline":

	// 	saveMessage.Type = "menu"

	case "contact":

		contact := sendMessage.GetContact()
		if contact == nil ||
			contact.Channel == "" ||
			contact.Contact == "" {
			return nil, errors.BadRequest(
				"chat.send.message.contact.missing",
				"send: contact data is missing",
			)
		}

		saveMessage.Type = "contact"
		saveMessage.Text = sendMessage.Text // contact.Contact
		if saveMessage.Text == "" {
			saveMessage.Text = contact.Contact
		}
		// FIXME: This MAY be NOT Contact Info of our Client (customer).
		// Customers (people) MAY share ANY Contact(s) from their own Contact Books !
		if contact.Id > 0 && sender.User != nil && contact.Id == sender.User.ID {
			var err error
			switch contact.Channel {
			case sender.Channel: // client.external_id changed !
				err = c.repo.UpdateClientChatID(ctx, sender.User.ID, contact.Contact)
			case "phone": // client.phone_number shared !
				err = c.repo.UpdateClientNumber(ctx, sender.User.ID, contact.Contact)
			}
			if err != nil {
				log.Error("Failed to persist Contact update",
					slog.Any("error", err),
					slog.Int64("client.id", sender.User.ID),
					slog.String(contact.Channel, contact.Contact),
				)
				return nil, err
			}
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
		// CHECK: document is internal file ?
		if doc.Id > 0 && (doc.Url == "" || doc.Name == "" || doc.Size == 0) {
			_, params, err := mime.ParseMediaType(doc.Mime)
			if err != nil {
				return nil, errors.BadRequest(
					"chat.send.document.file.mime.invalid",
					"send: document file mime is invalid",
				)
			}

			source := params["source"]
			if source == "" {
				// Use source = 'file' by default
				source = "file"
			}

			fileLink, err := c.storageClient.GenerateFileLink(ctx, &pbstorage.GenerateFileLinkRequest{
				DomainId: sender.DomainID,
				FileId:   doc.Id,
				Source:   source,
				Action:   "download",
				Metadata: true,
			})
			if err != nil {
				return nil, errors.BadRequest(
					"chat.send.document.file.error",
					"send: document file is not found",
				)
			}

			doc.Name = fileLink.Metadata.Name
			doc.Size = fileLink.Metadata.Size

			doc.Mime = mime.FormatMediaType(fileLink.Metadata.MimeType, map[string]string{
				"source": source,
			})

			doc.Url, err = util.JoinURL(fileLink.GetBaseUrl(), fileLink.GetUrl())
			if err != nil {
				return nil, errors.InternalServerError(
					"message.file.save",
					"broadcast: file( id: %d ); generate link error",
					doc.Id,
				)
			}
		}

		// CHECK: document URL specified ?
		if doc.Url == "" {
			return nil, errors.BadRequest(
				"chat.send.document.url.required",
				"send: document source URL required",
			)
		}
		// CHECK: provided URL is valid ?
		href, err := url.ParseRequestURI(doc.Url) // href

		if err != nil {
			return nil, errors.BadRequest(
				"chat.send.document.url.invalid",
				"send: document source URL invalid; %s", err,
			)
		}

		ok := href != nil

		ok = ok && href.IsAbs() // ok = ok && strings.HasPrefix(href.Scheme, "http")
		ok = ok && href.Host != ""

		if !ok {
			return nil, errors.BadRequest(
				"chat.send.document.url.invalid",
				"send: document source URL invalid;",
			)
		}

		// reset: normalized !
		doc.Url = href.String()

		// CHECK: filename !
		if doc.Name == "" {
			doc.Name = path.Base(href.Path)
			switch doc.Name {
			case "", ".", "/": // See: path.Base()
				return nil, errors.BadRequest(
					"chat.send.document.name.invalid",
					"send: document filename is missing or invalid",
				)
			}
		}

		// DETECT: MIME Content-Type by URL filename extension
		if doc.Mime == "" {
			doc.Mime = mime.TypeByExtension(
				path.Ext(doc.Name),
			)
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
				log.Error("Failed to UploadFileUrl",
					slog.Any("error", err),
				)
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
			// href, err := url.Parse(res.Url)

			// if err != nil {
			// 	return errors.InternalServerError(
			// 		"chat.send.document.url.invalid",
			// 		"send: uploaded document URL invalid; %s",
			// 		err,
			// 	)
			// }

			// reset: noramlized !
			doc.Id = res.Id
			doc.Size = res.Size
			// MIME: auto-detected while download ...

			doc.Url, err = util.JoinURL(res.Server, res.Url)
			if err != nil {
				return nil, errors.InternalServerError(
					"message.file.save",
					"broadcast: file( id: %d ); upload url error",
					doc.Id,
				)
			}

			// EXT: detect from MIME spec -if- missing
			filename := filepath.Base(doc.Name)
			filexten := filepath.Ext(filename)

			filename = filename[0 : len(filename)-len(filexten)]
			if mediaType := doc.Mime; mediaType != "" {
				// Get file extension for MIME type
				var ext []string
				switch filexten {
				default:
					ext = []string{filexten}
				case "", ".":
					switch strings.ToLower(mediaType) {
					case "application/octet-stream":
						ext = []string{".bin"}
					case "image/jpeg": // IMAGE
						ext = []string{".jpg"}
					case "image/png":
						ext = []string{".png"}
					case "image/gif":
						ext = []string{".gif"}
					case "audio/mpeg": // AUDIO
						ext = []string{".mp3"}
					case "audio/ogg": // VOICE
						ext = []string{".ogg"}
					default:
						// Resolve for MIME type ...
						ext, _ = mime.ExtensionsByType(mediaType)
					}
				}
				// Split: mediatype[/subtype]
				var subType string
				if slash := strings.IndexByte(mediaType, '/'); slash > 0 {
					subType = mediaType[slash+1:]
					mediaType = mediaType[0:slash]
				}
				if len(ext) == 0 { // != 1 {
					ext = strings.FieldsFunc(
						subType,
						func(c rune) bool {
							return !unicode.IsLetter(c)
						},
					)
					for n := len(ext) - 1; n >= 0; n-- {
						if ext[n] != "" {
							ext = []string{
								"." + ext[n],
							}
							break
						}
					}
				}
				if n := len(ext); n != 0 {
					filexten = ext[n-1] // last
				}
			}
			if filexten != "" {
				filename += filexten
			}
			// Populate unique filename
			doc.Name = filename

			// Determining mime by filename
			mediaType := mime.TypeByExtension(filexten)
			if mediaType != "" {
				mediaType = res.Mime
			}
			// File source = 'file' because we use FileService.UploadFileUrl for uploading
			doc.Mime = mime.FormatMediaType(mediaType, map[string]string{
				"source": "file",
			})

		} else if doc.Id < 0 {

			// DO NOT store/cache requested;
			// JUST rely original CDN media URL

			// doc.Url != "" // MUST
			doc.Id = 0 // NONE

			// TODO:
			// HEAD|GET URL
			// Content-Type: ?
			// Content-Length: ?
		}

		// Fill .Document
		saveMessage.Type = "file"
		saveMessage.File = &pg.Document{
			ID:   doc.Id,
			Size: doc.Size,
			Type: doc.Mime,
			Name: doc.Name,
		}
		if doc.Id == 0 {
			saveMessage.File.URL = doc.Url
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
		log.Error("Failed to store message",
			slog.Any("error", err),
		)
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
	from := sender.User
	sendMessage.From = &pbchat.Account{
		Id:        from.ID,
		Channel:   from.Channel,
		Contact:   from.Contact,
		FirstName: from.FirstName,
		LastName:  from.LastName,
		Username:  from.UserName,
	}
	// endregion

	return saveMessage, nil
}

// SendMessage publishes given message to all related recepients
// Override: event_router.RouteMessage()
func (c *chatService) sendMessage(ctx context.Context, chatRoom *app.Session, notify *pbchat.Message) (sent int, err error) {
	// FROM
	sender := chatRoom.Channel
	// TO
	if len(chatRoom.Members) == 0 {
		return 0, nil // NO ANY recepient(s) !
	}

	// publish
	var (
		// data   []byte // TO: websocket
		data = struct {
			textgate *pbchat.Message // messages-bot (text:gateway)
			workflow *pbchat.Message // flow_manager (bot:schema)
			wesocket []byte          // engine (agent:user)
		}{}
		header map[string]string

		rebind  bool
		binding = notify.GetVariables()
	)
	// Broadcast message to every member in the room,
	// in front of chaRoom.Channel as a sender !
	var (
		member  *app.Channel
		members = make([]*app.Channel, 1+len(chatRoom.Members))
	)

	members[0] = sender
	copy(members[1:], chatRoom.Members)

	debugCtx := []any{
		// event
		"msg", wlog.DeferValue(func() slog.Value {
			sentMsg := []slog.Attr{
				slog.Int64("id", notify.Id),
				slog.String("type", notify.Type),
				slog.String("kind", notify.Kind),
			}
			// if notify.Text != "" || notify.Type == "text" {
			// 	sentMsg = append(sentMsg,
			// 		slog.String("text", notify.Text),
			// 	)
			// }
			// if notify.File != nil {
			// 	sentMsg = append(sentMsg,
			// 		slog.String("file", wlog.JsonValue(notify.File)),
			// 	)
			// }
			return slog.GroupValue(sentMsg...)
		}),
		// sender
		"from", wlog.DeferValue(func() slog.Value {
			return debugLogChatValue(sender)
		}),
		// copy of [TO] chat.thread.id
		"conversation_id", sender.Chat.Invite,
	}
	// LOG: [kind/]type
	messageType := notify.Type
	if kind := notify.Kind; kind != "" {
		messageType = kind + "/" + messageType
	}
	debugText := fmt.Sprintf(
		"[ CHAT ] thread( %s ).message( %d; %s ).FROM( %s:%s )",
		sender.Chat.Invite, // conversation_id::chat.thread.id
		notify.Id, messageType,
		sender.User.Channel, sender.User.Contact,
	)
	// Start delivery ...
	c.log.Debug(
		debugText,
		debugCtx...,
	)
	deliveryLog := func(level slog.Level, msg string, args ...any) {
		params := append(
			// target participant
			debugCtx, "chat", wlog.DeferValue(func() slog.Value {
				return slog.GroupValue(debugLogChatGroup(member)...)
			}),
		)
		params = append(
			// extra arguments
			params, args...,
		)
		c.log.Log(
			context.TODO(), level,
			fmt.Sprintf(
				"%s.TO( %s:%s )%s",
				debugText,
				member.User.Channel, member.User.Contact,
				msg,
			),
			params...,
		)
	}

	var deliveryErr error

	for _, member = range members {

		if member.IsClosed() {
			continue // omit send TO channel: closed !
		}

		switch member.Channel {

		case "websocket": // TO: engine (internal)
			// NOTE: if sender is an internal chat@channel user (operator)
			//       we publish message for him (author) as a member too
			//       to be able to detect chat updates on other browser tabs ...
			if data.wesocket == nil {
				// basic
				timestamp := notify.UpdatedAt
				if timestamp == 0 {
					timestamp = notify.CreatedAt
				}
				notice := events.MessageEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: sender.Chat.Invite, // hidden channel.conversation_id
						Timestamp:      timestamp,          // millis
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
						URL:  doc.Url,
						Type: doc.Mime,
						Size: doc.Size,
						Name: doc.Name,
					}
				}
				// Postback. Button click[ed].
				// Webitel User (Agent) side.
				if btn := notify.Postback; btn != nil {
					notice.Postback = btn
					if notice.Text == "" {
						notice.Text = btn.Text
					}
				}
				// NOTE: Here, keyboard is not supported
				// because customer(s) can't send buttons

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
				data.wesocket, _ = json.Marshal(notice)
				header = map[string]string{
					"content_type": "text/json",
				}
			}
			deliveryLog(slog.LevelDebug, "")
			agent := broker.DefaultBroker // service.Options().Broker
			err = agent.Publish(fmt.Sprintf("event.%s.%d.%d",
				events.MessageEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: header,
				Body:   data.wesocket,
			})

		case "chatflow": // TO: workflow (internal)

			if member == sender {
				continue
			}

			if data.workflow == nil {
				// proto.Clone(notify).(*pbchat.Message)
				send := *(notify) // shallowcopy
				// Postback. Button click[ed].
				// Webitel Bot (Schema) side.
				if btn := notify.Postback; btn != nil {
					if code := btn.Code; code != "" {
						send.Text = code // reply_to
					}
				}
				data.workflow = &send
			}
			deliveryLog(slog.LevelDebug, "")
			err = c.flowClient.SendMessageV1(
				member, data.workflow,
			)

		default: // TO: webitel.chat.bot (external)
			if member == sender {
				continue
			}
			deliveryLog(slog.LevelDebug, "")
			err = c.eventRouter.SendMessageToGateway(sender, member, notify)
			if err != nil {
				deliveryErr = err
			}
			// Merge SENT message external binding (variables)
			if notify.Id == 0 {
				// NOTE: there was a service-level message notification
				//       so we omit message binding
				continue
			}

			for key, newValue := range notify.GetVariables() {
				if key == "" {
					continue
				}
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
		}

		(sent)++ // calc active recepients !

		if err != nil {
			// FIXME: just log failed attempt ?
			deliveryLog(
				slog.LevelError, "; error",
				"error", err,
			)
		}
	}

	if rebind {
		_ = c.repo.BindMessage(ctx, notify.Id, binding)
	}

	if sent == 0 {
		// ERR: unreachable code
		c.log.Error(
			debugText+"; no delivery",
			append(debugCtx, "error", "no delivery")...,
		)
	}

	return sent, deliveryErr
}

func (c *chatService) notifyAgentJoinToAllMembers(ctx context.Context, chatRoom *app.Session, notify *pbchat.Message) (sent int, err error) {
	// FROM
	sender := chatRoom.Channel
	// TO
	if len(chatRoom.Members) == 0 {
		return 0, nil // NO ANY recepient(s) !
	}

	// publish
	var (
		// data   []byte // TO: websocket
		data = struct {
			textgate *pbchat.Message // messages-bot (text:gateway)
			workflow *pbchat.Message // flow_manager (bot:schema)
			wesocket []byte          // engine (agent:user)
		}{}
		header map[string]string
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
			if data.wesocket == nil {
				// basic
				timestamp := notify.UpdatedAt
				if timestamp == 0 {
					timestamp = notify.CreatedAt
				}
				notice := events.MessageEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: sender.Chat.Invite, // hidden channel.conversation_id
						Timestamp:      timestamp,          // millis
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
						URL:  doc.Url,
						Type: doc.Mime,
						Size: doc.Size,
						Name: doc.Name,
					}
				}
				// Postback. Button click[ed].
				// Webitel User (Agent) side.
				if btn := notify.Postback; btn != nil {
					notice.Postback = btn
					if notice.Text == "" {
						notice.Text = btn.Text
					}
				}
				// NOTE: Here, keyboard is not supported
				// because customer(s) can't send buttons

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
				data.wesocket, _ = json.Marshal(notice)
				header = map[string]string{
					"content_type": "text/json",
				}
			}

			agent := broker.DefaultBroker // service.Options().Broker
			err = agent.Publish(fmt.Sprintf("event.%s.%d.%d",
				events.MessageEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: header,
				Body:   data.wesocket,
			})
		}
	}
	return sent, nil // err
}

func debugLogChatValue(side *app.Channel) slog.Value {
	return slog.GroupValue(debugLogChatGroup(side)...)
}

func debugLogChatGroup(side *app.Channel) []slog.Attr {
	chat := side.Chat
	user := side.User
	args := []slog.Attr{
		slog.String("id", chat.ID),
		slog.String("via", chat.Contact),
		slog.String("user", user.Channel+":"+user.Contact),
		slog.String("title", user.FirstName),
		// slog.String("thread.id", chat.Invite), // conversation_id
	}
	return args
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
		data   []byte
		header map[string]string

		notice *pbchat.Message
	)
	// Broadcast message to every member in the room,
	// in front of chaRoom.Channel as a sender !
	var (
		member  *app.Channel // current: in iteration with ...
		members = make([]*app.Channel, 1+len(chatRoom.Members))
	)

	members[0] = sender
	copy(members[1:], chatRoom.Members)

	debugCtx := []any{
		// sender
		"from", wlog.DeferValue(func() slog.Value {
			return debugLogChatValue(sender)
		}),
		// event
		"msg.type", "closed",
		// "msg.text", text,
		// copy of [TO] chat.thread.id
		"conversation_id", sender.Chat.Invite,
	}
	debugText := fmt.Sprintf(
		"[ CHAT ] thread( %s ).message( closed ).FROM( %s:%s )",
		sender.Chat.Invite, // conversation_id::chat.thread.id
		sender.User.Channel, sender.User.Contact,
	)
	// Start delivery ...
	c.log.Debug(
		debugText,
		debugCtx...,
	)
	deliveryLog := func(level slog.Level, msg string, args ...any) {
		params := append(
			// target participant
			debugCtx, "chat", wlog.DeferValue(func() slog.Value {
				return slog.GroupValue(debugLogChatGroup(member)...)
			}),
		)
		params = append(
			// extra arguments
			params, args...,
		)
		c.log.Log(
			context.TODO(), level,
			fmt.Sprintf(
				"%s.TO( %s:%s )%s",
				debugText,
				member.User.Channel, member.User.Contact,
				msg,
			),
			params...,
		)
	}

	for _, member = range members { // chatRoom.Members {

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
				header = map[string]string{
					"content_type": "text/json",
				}
			}

			deliveryLog(slog.LevelDebug, "")
			agent := broker.DefaultBroker // service.Options().Broker
			err = agent.Publish(fmt.Sprintf("event.%s.%d.%d",
				events.CloseConversationEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: header,
				Body:   data,
			})

		case "chatflow": // TO: workflow (internal)
			// NOTE: we do not send messages to chat@bot channel
			// until there is not a private (one-to-one) chat room
			if member == sender { // e == 0
				continue
			}
			deliveryLog(slog.LevelDebug, "")
			// Send workflow channel .Break() message to stop chat.flow routine ...
			// FIXME: - delete: chat.confirmation; - delete: chat.flow.node
			err = c.flowClient.CloseConversation(member.Chat.ID, "") // .ConversationID

			// if err != nil {
			// 	c.log.Error().Err(err).
			// 		Msg("FAILED Break chat@flow routine")
			// 	// return err
			// }

		default: // TO: webitel.chat.bot (external)
			// s.eventRouter.sendMessageToBotUser()

			// if member == sender { // e == 0
			// 	continue
			// }

			if notice == nil {
				notice = &pbchat.Message{

					Id: 0, // SERVICE MESSAGE !

					Type: "closed", // "text",
					Text: text,

					CreatedAt: app.DateTimestamp(localtime),
				}
			}
			deliveryLog(slog.LevelDebug, "")
			err = c.eventRouter.SendMessageToGateway(sender, member, notice)
		}

		(sent)++ // calc active recepients !

		if err != nil {
			// FIXME: just log failed attempt ?
			deliveryLog(
				slog.LevelError, "; error",
				"error", err,
			)
		}
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

func (c *chatService) SetVariables(ctx context.Context, req *pbchat.SetVariablesRequest, res *pbchat.ChatVariablesResponse) error {

	channelId := req.GetChannelId()
	if channelId == "" {
		return errors.BadRequest(
			"chat.channel.id.required",
			"chat: channel.id required but missing",
		)
	}

	changes := req.GetVariables()
	if len(changes) != 0 {
		// Remove invalid (empty) key !
		delete(changes, "")
	}

	if len(changes) == 0 {
		return errors.BadRequest(
			"chat.channel.vars.required",
			"chat: channel.vars required but missing",
		)
	}

	// // region: lookup target chat session by unique sender chat channel id
	// chat, err := c.repo.GetSession(ctx, channelId)

	// if err != nil {
	// 	// lookup operation error
	// 	return err
	// }

	// if chat == nil || chat.ID != channelId {
	// 	// sender channel ID not found
	// 	return errors.BadRequest(
	// 		"chat.send.channel.from.not_found",
	// 		"send: FROM channel ID=%s sender not found or been closed",
	// 		 channelId,
	// 	)
	// }

	// channel := chat.Channel
	// channel.MergeVars(req.GetVariables())
	envars, err := c.repo.BindChannel(ctx, channelId, changes)

	if err != nil {
		return err
	}

	res.ChannelId = channelId
	res.Variables = envars
	return nil
}

func (c *chatService) BlindTransfer(ctx context.Context, req *pbchat.ChatTransferRequest, res *pbchat.ChatTransferResponse) error {

	var (
		userToID   = req.GetUserId()
		schemaToID = req.GetSchemaId()
		chatFromID = req.GetChannelId()
		chatFlowID = req.GetConversationId()
	)

	c.log.Debug("TRANSFER Conversation",
		slog.String("conversation_id", chatFlowID),
		slog.String("channel_id", chatFromID),
		slog.Int64("schema_id", schemaToID),
		slog.Int64("user_id", userToID),
	)

	if chatFlowID == "" && chatFromID == "" {
		return errors.BadRequest(
			"chat.transfer.conversation.required",
			"transfer: chat .conversation_id or sender .channel_id required but missing",
		)
	}

	// if schemaToID == 0 {
	// 	return errors.BadRequest(
	// 		"chat.transfer.flow.schema_id.required",
	// 		"chat: transfer:to schema_id required but missing",
	// 	)
	// }

	if schemaToID == 0 && userToID == 0 {
		return errors.BadRequest(
			"chat.transfer.target.required",
			"chat: transfer:to target(.schema_id|.user_id) required but missing",
		)
	} else if schemaToID != 0 && userToID != 0 {
		return errors.BadRequest(
			"chat.transfer.target.ambiguous",
			"chat: transfer:to target(.schema_id&.user_id) is ambiguous",
		)
	}

	coalesce := func(s ...string) string {
		for _, v := range s {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
		return ""
	}

	chat, err := c.repo.GetSession(
		ctx, coalesce(chatFromID, chatFlowID),
	)

	if err != nil {
		return err
	}

	if chat == nil || (chatFlowID != "" && chat.Channel.Chat.Invite != chatFlowID) {
		// The Conversation (chat@flow) channel not found
		return errors.BadRequest(
			"chat.transfer.conversation.not_found",
			"transfer: conversation ID=%s not found or been closed",
			chatFlowID,
		)
	}
	// Resolve .conversationId -from- .originatorId -if- omitted
	chatFlowID = chat.Channel.Chat.Invite

	if chatFromID != "" && chat.ID != chatFromID {
		// The Originator (user@webitel) channel not found
		return errors.BadRequest(
			"chat.transfer.channel.not_found",
			"transfer: origin channel ID=%s not found or been closed",
			chatFromID,
		)
	}
	chatFromID = chat.ID

	originator := chat.Channel                 // Mostly: call-center operator (channelId)
	conversation := chat.GetMember(chatFlowID) // MUST: schema@workflow (conversationId)
	_ = conversation.ID                        // NOTNULL

	/*var userToID int64 = 72
	if userToID != 0 {
		var res pbchat.InviteToConversationResponse
		err = c.InviteToConversation(ctx,
			&pbchat.InviteToConversationRequest{
				User: &pbchat.User{
					UserId:     userToID,
					Type:       "",
					Connection: "",
					Internal:   true,
				},
				ConversationId:   originator.Chat.Invite, // chatFlowID,
				InviterChannelId: originator.Chat.ID, // chatFromID,
				AuthUserId:       originator.User.ID,
				DomainId:         originator.DomainID,
				Title:            originator.Title,
				TimeoutSec:       16,
				AppId:            "",
				Variables:        req.GetVariables(),
			},
			&res,
		)
		if err != nil {
			return err
		}
		// return err // err | <nil>
	}*/

	switch originator.Channel {
	case "websocket":

		if chatFromID == chatFlowID {
			// Transfer request from Flow schema itself !
			// DO NOTHING More !..
			break
		}
		// if originator.FlowBridge {
		// 	// LeaveConversation()
		// } else {
		// 	// DeclineInvitation()
		// }
		// In case of NON-Accepted Invite Request !
		var decline pbchat.DeclineInvitationResponse
		// NOTE: Ignore errors; Calling this just to be sure that originator's channel is kicked !
		_ = c.DeclineInvitation(ctx,
			&pbchat.DeclineInvitationRequest{
				ConversationId: chatFlowID,
				InviteId:       chatFromID,
				AuthUserId:     originator.User.ID,
			}, &decline,
		)
		// In case of Agent (Originator) Bridge Application running !
		// var leave pbchat.LeaveConversationResponse
		// NOTE: Ignore errors; Calling this just to be sure that originator's channel is kicked !
		// Cause of the originator kick is clearly defined here
		_ = c.leaveChat(ctx, &pbchat.LeaveConversationRequest{
			ConversationId: chatFlowID,
			AuthUserId:     originator.User.ID,
			ChannelId:      chatFromID,
			Cause:          pbchat.LeaveConversationCause_transfer,
		}, flow.TransferCause)

	case "chatflow":
		// FIXME:
	}

	/*/ COMPLETE: Transfer TO User ?
	if userToID != 0 {
		// res.* ?
		return nil
	}*/

	// PERFORM: SWITCH Flow runtime schema(s) !
	err = c.flowClient.TransferTo(
		chatFlowID, originator,
		schemaToID, userToID,
		// merge channel.variables latest state
		req.Variables,
	)

	if err != nil {
		return err
	}
	// res.* ?
	return nil
}

func (c *chatService) SendUserAction(ctx context.Context, req *pbchat.SendUserActionRequest, res *pbchat.SendUserActionResponse) error {

	senderChatID := req.GetChannelId()
	if senderChatID == "" {
		return errors.BadRequest(
			"chat.send.action.from.required",
			"send: action sender chat ID required",
		)
	}

	// region: lookup target chat session by unique sender chat channel id
	chat, err := c.repo.GetSession(ctx, senderChatID)

	if err != nil {
		// lookup operation error
		return err
	}

	if chat == nil || chat.ID != senderChatID {
		// sender channel ID not found
		return errors.BadRequest(
			"chat.send.action.from.not_found",
			"send: FROM channel ID=%s sender not found or been closed",
			senderChatID,
		)
	}

	if chat.IsClosed() {
		// sender channel is already closed !
		return errors.BadRequest(
			"chat.send.action.from.closed",
			"send: FROM chat channel ID=%s is closed",
			senderChatID,
		)
	}
	// FROM
	// sender := chat.Channel
	// TO
	if len(chat.Members) == 0 {
		// NO group partners
		res.Ok = false
		return nil
	}

	for _, member := range chat.Members {

		if member.IsClosed() {
			continue // omit send TO channel: closed !
		}

		switch member.Channel {
		case "websocket": // TO: engine (internal)
		case "chatflow": // TO: workflow (internal)
		default: // TO: webitel.chat.bot (external)
			{
				ok, err := c.eventRouter.SendUserActionToGateway(member, req)
				if err != nil {
					c.log.Warn("ACTION [TO]",
						slog.Any("error", err),
						slog.String("channel_id", req.ChannelId),
					)
					continue
				}
				res.Ok = (res.Ok || ok)
			}
		}
	}

	return nil
}

func (c *chatService) BroadcastMessage(ctx context.Context, req *pbmessages.BroadcastMessageRequest, resp *pbmessages.BroadcastMessageResponse) error {
	// NOTE: Authentication
	authUser, err := c.authClient.MicroAuthentication(&ctx)
	if err != nil {
		return err
	}

	// NOTE: Validate input
	validator := newBroadcastValidator(req)

	err = validator.validateMessage()
	if err != nil {
		return err
	}

	req.Peers = validator.validatePeers() // ignore invalid peers
	resp.Failure = append(resp.Failure, validator.getErrors()...)

	return c.executeBroadcast(ctx, authUser, req, resp)
}

func (c *chatService) BroadcastMessageNA(ctx context.Context, req *pbmessages.BroadcastMessageRequest, resp *pbmessages.BroadcastMessageResponse) error {
	// NOTE: Validate input
	validator := newBroadcastValidator(req)

	err := validator.validateMessage()
	if err != nil {
		return err
	}

	req.Peers = validator.validatePeers() // ignore invalid peers
	resp.Failure = append(resp.Failure, validator.getErrors()...)

	return c.executeBroadcast(ctx, nil, req, resp)
}
