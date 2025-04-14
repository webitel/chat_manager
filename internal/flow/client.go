package flow

import (
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/internal/contact"
	"log/slog"

	"strconv"
	"sync"

	// "errors"
	// "context"
	// "github.com/micro/go-micro/v2/errors"

	// strategy "github.com/webitel/chat_manager/internal/selector"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	flow "github.com/webitel/chat_manager/api/proto/workflow"
	"github.com/webitel/chat_manager/app"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
)

type BreakBridgeCause string

const (
	DeclineInvitationCause = BreakBridgeCause("DECLINE_INVITATION")
	LeaveConversationCause = BreakBridgeCause("LEAVE_CONVERSATION")
	TransferCause          = BreakBridgeCause("TRANSFER")
	TimeoutCause           = BreakBridgeCause("TIMEOUT")
)

func (c BreakBridgeCause) String() string {
	return string(c)
}

type Client interface {
	// SendMessage(conversationID string, message *chat.Message) error
	SendMessage(sender *store.Channel, message *chat.Message) error
	// Init(conversationID string, profileID, domainID int64, message *chat.Message) error
	Init(sender *store.Channel, message *chat.Message) error
	BreakBridge(conversationID string, cause BreakBridgeCause) error
	CloseConversation(conversationID, cause string) error
	// BreakBridge(sender *store.Channel, cause BreakBridgeCause) error
	// CloseConversation(sender *store.Channel) error
	TransferTo(conversationID string, originator *app.Channel, schemaToID, userToID int64, setVars map[string]string) error

	SendMessageV1(target *app.Channel, message *chat.Message) error
	WaitMessage(conversationId, confirmationId string) error
}

// Agent "workflow" (internal: chat@bot) channel service provider
type Agent struct {
	Log    *slog.Logger
	Store  store.CacheRepository
	Client flow.FlowChatServerService
	// cache: memory
	sync.RWMutex // REFLOCK
	// map[conversation]workflow
	channel map[string]*Channel
}

func NewClient(

	log *slog.Logger,
	store store.CacheRepository,
	client flow.FlowChatServerService,

) *Agent {

	return &Agent{
		Log:     log,
		Store:   store,
		Client:  client,
		channel: make(map[string]*Channel),
	}
}

func (c *Agent) GetChannel(conversationID string) (*Channel, error) {

	c.RLock() // +R
	channel, ok := c.channel[conversationID]
	c.RUnlock() // -R

	if ok && channel.ID == conversationID {
		return channel, nil // CACHE: FOUND !
	}

	channel = &Channel{

		Log:   c.Log,
		Store: c.Store,
		Agent: c.Client,

		Host: "", // NEW
		ID:   conversationID,
	}

	if !ok {
		c.Lock() // +RW
		c.channel[conversationID] = channel
		c.Unlock() // -RW
	}

	return channel, nil
}

func (c *Agent) delChannel(conversationID string) (ok bool) {

	c.Lock() // +RW
	if _, ok = c.channel[conversationID]; ok {
		delete(c.channel, conversationID)
	}
	c.Unlock() // -RW

	return ok
}

// WaitMessage setup confirmationId token for the next conversationId message delivery
func (c *Agent) WaitMessage(conversationId, confirmationId string) error {
	// Fast setup
	sub, err := c.GetChannel(conversationId)
	if err != nil {
		return err
	}
	sub.setPending(confirmationId, "*")
	// sub.Pending = confirmationId
	// Perisist changes
	err = c.Store.WriteConfirmation(conversationId, confirmationId)
	if err != nil {
		return err
	}
	return nil
}

func (c *Agent) SendMessage(sender *store.Channel, message *chat.Message) error {

	channel, err := c.GetChannel(sender.ConversationID)
	if err != nil {
		return err
	}

	// region: RECOVERY state
	if channel.DomainID() == 0 {
		channel.Chat.DomainID = sender.DomainID
	}

	if channel.ProfileID == 0 {
		// RESTORE: recepient processing ...
		// NOTE: sender for now is webitel.chat.bot channel only !
		schemaProfileID, err := strconv.ParseInt(sender.Connection.String, 10, 64)
		if err != nil {
			return err
		}
		// preset: resolved !
		channel.ProfileID = schemaProfileID
	}
	// endregion

	// PERFORM !
	err = channel.Send(message)

	if err != nil {
		// NOTE: remove to be renewed next time ...
		_ = c.delChannel(channel.ID)
		return err
	}

	return nil
}

func (c *Agent) SendMessageV1(target *app.Channel, message *chat.Message) error {

	// channel, err := c.GetChannel(sender.ConversationID)
	channel, err := c.GetChannel(target.Chat.ID) // NOTE: target.Chat.ID == target.Chat.Invite
	if err != nil {
		return err
	}

	// region: RECOVERY state
	if channel.DomainID() == 0 {
		channel.Chat.DomainID = target.DomainID
	}

	if channel.ProfileID == 0 {
		// RESTORE: recepient processing ...
		channel.ProfileID, channel.Host, _ =
			contact.ContactObjectNode(target.Contact)
		// [NOW] Except webitel.chat.bot there is webitel.chat.portal with no such profile !
		// // NOTE: sender for now is webitel.chat.bot channel only !
	}
	// endregion

	// PERFORM !
	err = channel.Send(message)

	if err != nil {
		// NOTE: remove to be renewed next time ...
		_ = c.delChannel(channel.ID)
		return err
	}

	return nil
}

// Init chat => flow chat-channel communication state
func (c *Agent) Init(sender *store.Channel, message *chat.Message) error {

	// now := time.Now()
	// date := now.UTC().Unix()

	// resolve end-user for .this NEW channel: flow.schema.id
	botGatewayID, err := strconv.ParseInt(
		sender.Connection.String, 10, 64,
	)

	if err != nil {
		// MUST: be valid profile@webitel.chat.bot channel as a sender !
		return err
	}

	channel := &Channel{

		Log:   c.Log,
		Host:  "", // PEEK
		Agent: c.Client,
		Store: c.Store,
		// ChannelID: reflects .start channel member.id
		ID:   sender.ConversationID,
		Chat: *(sender),
		// User: &chat.User{
		// 	UserId:     0, // profile.schema.id
		// 	Type:       "chatflow",
		// 	Connection: "",
		// 	Internal:   true,
		// },

		// DomainID: sender.DomainID,
		ProfileID: botGatewayID,

		Invite:  "", // .Invite(!) token
		pending: "", // .WaitMessage(!) token

		// Created: date,
		// Updated: date,
		// Started: 0,
		// Joined:  0,
		// Closed:  0,

		Variables: sender.Variables,
	}

	// c.Log.Debug().
	// 	Str("conversation_id", channel.ChatID()).
	// 	Int64("profile_id", channel.UserID()).
	// 	Int64("domain_id", channel.DomainID()).
	// 	Msg("init conversation")

	err = channel.Start(message)
	if err != nil {
		return err
	}

	c.Lock() // +RW
	c.channel[channel.ID] = channel
	c.Unlock() // -RW

	return nil
}

/*func (c *Agent) Init(conversationID string, profileID, domainID int64, message *chat.Message) error {

	c.Log.Debug().
		Str("conversation_id", conversationID).
		Int64("profile_id", profileID).
		Int64("domain_id", domainID).
		Msg("init conversation")

	// *start := &bot.StartRequest{

		DomainId:       domainID,
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId:      profileID,
		ConversationId: conversationID,

		Variables: message.GetVariables(),

		Message: &bot.Message{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &bot.Message_Text{
				Text: "start", //req.GetMessage().GetTextMessage().GetText(),
			},
		},
	}

	if message != nil {

		switch e := message.GetValue().(type) {
		case *chat.Message_Text: // TEXT

			messageText := e.Text
			if messageText == "" {
				messageText = "start" // default!
			}

			start.Message.Value =
				&bot.Message_Text{
					Text: messageText,
				}

		case *chat.Message_File_: // FILE

			start.Message.Value =
				&bot.Message_File_{
					File: &bot.Message_File{
						Id:       e.File.GetId(),
						Url:      e.File.GetUrl(),
						MimeType: e.File.GetMimeType(),
					},
				}
		}
	}* //

	now := time.Now()
	date := now.UTC().Unix()

	channel := &Channel {

		Log:   c.Log,
		Host:  "", // PEEK
		Agent: c.Client,
		Store: c.Store,
		// ChannelID: reflects .start channel member.id
		ID: conversationID,
		User: &chat.User{
			UserId:     0, // profile.schema.id
			Type:       "chatflow",
			Connection: "",
			Internal:   true,
		},

		DomainID: domainID,
		ProfileID: profileID,

		Invite:  "", // .Invite(!) token
		Pending: "", // .WaitMessage(!) token

		Created: date,
		Updated: date,
		Started: 0,
		Joined:  0,
		Closed:  0,
	}

	err := channel.Start(message)
	if err != nil {
		return err
	}

	c.Lock()   // +RW
	c.channel[channel.ID] = channel
	c.Unlock() // -RW

	return nil

	// // Request to start flow-routine for NEW-chat incoming message !
	// res, err := c.Agent.Start(
	// 	context.TODO(), start,
	// 	client.WithCallWrapper(
	// 		s.initCallWrapper(conversationID),
	// 	),
	// )

	// if err != nil {

	// 	s.log.Error().Err(err).
	// 		Msg("Failed to start chat-flow routine")

	// 	return err

	// } else if re := res.GetError(); re != nil {

	// 	s.log.Error().
	// 		Str("errno", re.GetId()).
	// 		Str("error", re.GetMessage()).
	// 		Msg("Failed to start chat-flow routine")

	// 	// return errors.New(
	// 	// 	re.GetId(),
	// 	// 	re.GetMessage(),
	// 	// 	502, // 502 Bad Gateway
	// 	// 	// The server, while acting as a gateway or proxy,
	// 	// 	// received an invalid response from the upstream server it accessed
	// 	// 	// in attempting to fulfill the request.
	// 	// )
	// }

	// return nil

	// / ; err != nil || res.Error != nil { // WTF: (0_o) (?)
	// 	if err == nil && res.Error != nil {
	// 		err =
	// 	}

	// 	if res != nil { // GUESS: it will never be empty !
	// 		s.log.Error().Msg(res.Error.Message)
	// 	} else {
	// 		s.log.Error().Err(err).Msg("Failed to start chat-flow routine")
	// 	}
	// 	return nil
	// }
	// return nil
	// /
}*/

func (c *Agent) CloseConversation(conversationID, cause string) error {

	channel, err := c.GetChannel(conversationID)

	if err != nil {
		return err
	}

	err = channel.Close(cause)

	if err != nil {
		return err
	}

	// c.Lock()   // +RW
	// delete(c.channel, channel.ID)
	// c.Unlock() // -RW
	c.delChannel(channel.ID)

	return nil

	// nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	// if res, err := s.client.Break(
	// 	context.Background(),
	// 	&pbmanager.BreakRequest{
	// 		ConversationId: conversationID,
	// 	},
	// 	client.WithSelectOption(
	// 		selector.WithStrategy(
	// 			strategy.PrefferedNode(nodeID),
	// 		),
	// 	),
	// ); err != nil {
	// 	return err
	// } else if res != nil && res.Error != nil {
	// 	return errors.New(res.Error.Message)
	// }
	// //s.chatCache.DeleteCachedMessages(conversationID)
	// s.chatCache.DeleteConfirmation(conversationID)
	// s.chatCache.DeleteConversationNode(conversationID)
	// return nil
}

func (c *Agent) BreakBridge(conversationID string, cause BreakBridgeCause) error {

	channel, err := c.GetChannel(conversationID)

	if err != nil {
		return err
	}

	err = channel.BreakBridge(cause)

	if err != nil {
		// NOTE: ignore "grpc.chat.conversation.not_found" !
		return err
	}

	return nil

	// nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	// if res, err := s.client.BreakBridge(
	// 	context.Background(),
	// 	&pbmanager.BreakBridgeRequest{
	// 		ConversationId: conversationID,
	// 		Cause:          cause.String(),
	// 	},
	// 	client.WithSelectOption(
	// 		selector.WithStrategy(
	// 			strategy.PrefferedNode(nodeID),
	// 		),
	// 	),
	// ); err != nil {
	// 	return err
	// } else if res != nil && res.Error != nil {
	// 	return errors.New(res.Error.Message)
	// }
	// return nil
}

/*func (c *Agent) initCallWrapper(conversationID string) func(client.CallFunc) client.CallFunc {
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
			s.log.Trace().
				Str("id", node.Id).
				Str("address", node.Address).Msg("send request to node")
			err := next(ctx, node, req, rsp, opts)
			if err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}
			if err := s.chatCache.WriteConversationNode(conversationID, node.Id); err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}
			return nil
		}
	}
}*/

func (c *Agent) TransferTo(conversationID string, originator *app.Channel, schemaToID, userToID int64, setVars map[string]string) error {

	channel, err := c.GetChannel(conversationID)

	if err != nil {
		return err
	}

	// merge channel.variables latest state
	if setVars != nil {
		delete(setVars, "")
		if n := len(setVars); n != 0 {
			chatVars := channel.Variables
			if chatVars == nil {
				chatVars = make(map[string]string, n)
			}
			for key, val := range setVars {
				// switch key {
				// case "xfer", "flow", "chat", "user", "from": // system; DO NOT reassign !
				// default:
				// }
				chatVars[key] = val
			}
			channel.Variables = chatVars
		}
	}

	if schemaToID != 0 {
		err = channel.TransferToSchema(originator, schemaToID)
	} else if userToID != 0 {
		err = channel.TransferToUser(originator, userToID)
	} else {
		err = errors.BadRequest(
			"chat.transfer.target.required",
			"chat: transfer:to target(.schema_id|.user_id) required but missing",
		)
	}

	if err != nil {
		// NOTE: ignore "grpc.chat.conversation.not_found" !
		return err
	}

	return nil
}
