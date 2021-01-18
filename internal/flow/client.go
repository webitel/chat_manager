package flow

import (
	"github.com/webitel/chat_manager/internal/contact"

	"sync"
	"strconv"

	// "errors"
	// "context"
	// "github.com/micro/go-micro/v2/errors"

	"github.com/rs/zerolog"

	// strategy "github.com/webitel/chat_manager/internal/selector"
	"github.com/webitel/chat_manager/app"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	flow "github.com/webitel/chat_manager/api/proto/workflow"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"

	// "github.com/micro/go-micro/v2/client"
	// "github.com/micro/go-micro/v2/client/selector"
	// "github.com/micro/go-micro/v2/registry"
	
)

type BreakBridgeCause int32

const (
	DeclineInvitationCause BreakBridgeCause = iota
	LeaveConversationCause
	TimeoutCause
)

func (c BreakBridgeCause) String() string {
	return [...]string{
		"DECLINE_INVITATION",
		"LEAVE_CONVERSATION",
		"TIMEOUT",
	}[c]
}

type Client interface {
	// SendMessage(conversationID string, message *chat.Message) error
	SendMessage(sender *store.Channel, message *chat.Message) error
	// Init(conversationID string, profileID, domainID int64, message *chat.Message) error
	Init(sender *store.Channel, message *chat.Message) error
	BreakBridge(conversationID string, cause BreakBridgeCause) error
	CloseConversation(conversationID string) error
	// BreakBridge(sender *store.Channel, cause BreakBridgeCause) error
	// CloseConversation(sender *store.Channel) error

	SendMessageV1(target *app.Channel, message *chat.Message) error
}

// Agent "workflow" (internal: chat@bot) channel service provider
type Agent struct {
	Log *zerolog.Logger
	Store store.CacheRepository
	Client flow.FlowChatServerService
	// cache: memory
	sync.RWMutex // REFLOCK
	// map[conversation]workflow
	channel map[string]*Channel
}

func NewClient(

	log *zerolog.Logger,
	store store.CacheRepository,
	client flow.FlowChatServerService,

) *Agent {
	
	return &Agent{
		Log: log,
		Store: store,
		Client: client,
		channel: make(map[string]*Channel),
	}
}

func (c *Agent) GetChannel(conversationID string) (*Channel, error) {

	c.RLock()   // +R
	channel, ok := c.channel[conversationID]
	c.RUnlock() // -R

	if ok && channel.ID == conversationID {
		return channel, nil // CACHE: FOUND ! 
	}

	// if !ok {

	// 	srv, err := c.Store.ReadConversationNode(conversationID)
			
	// 	if err != nil {
			
	// 		c.Log.Error().Err(err).
	// 		Str("chat-id", conversationID).
	// 		Str("channel", "workflow").
	// 		Msg("Looking for channel host")

	// 		return nil, err
		
	// 	}

	// 	node = srv
	// }

	channel = &Channel{

		Log:   c.Log,
		Store: c.Store,
		Agent: c.Client,

		Host:  "", // NEW
		ID: conversationID,
		// User: &chat.User{
		// 	UserId:     0, // flow.schema.id
		// 	Type:       "workflow",
		// 	Connection: "",
		// 	Internal:   true,
		// },
		// Chat: &sqlxrepo.Channel{
		// 	ID:             "", // FIXME
		// 	Type:           "workflow",
		// 	ConversationID: conversationID,
		// 	UserID:         0,
		// 	// Connection: sql.NullString{
		// 	// 	String: "workflow:bot@" + node,
		// 	// 	Valid:  false,
		// 	// },
		// 	// ServiceHost: sql.NullString{
		// 	// 	String: "",
		// 	// 	Valid:  false,
		// 	// },
		// 	// CreatedAt: time.Time{},
		// 	// Internal:  false,
		// 	// ClosedAt: sql.NullTime{
		// 	// 	Time:  time.Time{},
		// 	// 	Valid: false,
		// 	// },
		// 	// UpdatedAt:  time.Time{},
		// 	// DomainID:   0,
		// 	// FlowBridge: false,
		// 	// Name:       "",
		// 	// ClosedCause: sql.NullString{
		// 	// 	String: "",
		// 	// 	Valid:  false,
		// 	// },
		// 	// JoinedAt: sql.NullTime{
		// 	// 	Time:  time.Time{},
		// 	// 	Valid: false,
		// 	// },
		// },
		// Invite:  "",
		// Pending: "",
		// Created: 0,
		// Updated: 0,
		// Started: 0,
		// Joined:  0,
		// Closed:  0,
	}

	if !ok {
		c.Lock()   // +RW
		c.channel[conversationID] = channel
		c.Unlock() // -RW
	}

	return channel, nil
}

func (c *Agent) delChannel(conversationID string) (ok bool) {
	
	c.Lock()   // +RW
	if _, ok = c.channel[conversationID]; ok {
		delete(c.channel, conversationID)
	}
	c.Unlock() // -RW

	return ok
}

/*func (c *Agent) SendMessage(conversationID string, message *chat.Message) error {
	
	channel, err := c.GetChannel(conversationID)
	if err != nil {
		return err
	}

	err = channel.Send(message)

	if err != nil {
		return err
	}

	return nil

	// confirmationID, err := s.chatCache.ReadConfirmation(conversationID)
	// if err != nil {
	// 	s.log.Error().Err(err).Str("chat-id", conversationID).Msg("Failed to get {chat.recvMessage.token} from store")
	// 	return err
	// }
	// if confirmationID == "" {
	// 	// FIXME: NO confirmation found for chat - means that we are not in {waitMessage} block ?
	// 	s.log.Warn().Str("chat-id", conversationID).Msg("CHAT Flow is NOT waiting for text message(s); DO NOTHING MORE!")
	// 	return nil
	// }
	// s.log.Debug().
	// 	Str("conversation_id", conversationID).
	// 	Str("confirmation_id", string(confirmationID)).
	// 	Msg("send confirmed messages")
	// messages := []*pbmanager.Message{
	// 	{
	// 		Id:   message.GetId(),
	// 		Type: message.GetType(),
	// 		Value: &pbmanager.Message_Text{
	// 			Text: message.GetText(),
	// 		},
	// 	},
	// }
	// messageReq := &pbmanager.ConfirmationMessageRequest{
	// 	ConversationId: conversationID,
	// 	ConfirmationId: confirmationID,
	// 	Messages:       messages,
	// }
	// nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	// if res, err := s.client.ConfirmationMessage(
	// 	context.Background(),
	// 	messageReq,
	// 	client.WithSelectOption(
	// 		selector.WithStrategy(
	// 			strategy.PrefferedNode(nodeID),
	// 		),
	// 	),
	// ); err != nil || res.Error != nil {
	// 	if res != nil {
	// 		return errors.New(res.Error.Message)
	// 	}
	// 	return err
	// }
	// s.chatCache.DeleteConfirmation(conversationID)
	// return nil

	// s.log.Debug().
	// 	Int64("conversation_id", conversationID).
	// 	Msg("cache messages for confirmation")
	// cacheMessage := &pb.Message{
	// 	Id:   message.GetId(),
	// 	Type: message.GetType(),
	// 	Value: &pb.Message_TextMessage_{
	// 		TextMessage: &pb.Message_TextMessage{
	// 			Text: message.GetTextMessage().GetText(),
	// 		},
	// 	},
	// }
	// messageBytes, err := proto.Marshal(cacheMessage)
	// if err != nil {
	// 	s.log.Error().Msg(err.Error())
	// 	return nil
	// }
	// if err := s.chatCache.WriteCachedMessage(conversationID, message.GetId(), messageBytes); err != nil {
	// 	s.log.Error().Msg(err.Error())
	// }
	// return nil
}*/

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
		// NOTE: sender for now is webitel.chat.bot channel only !
		schemaProfileID, serviceNode, err := contact.ContactObjectNode(target.Contact)
		// schemaProfileID, err := strconv.ParseInt(sender.Connection.String, 10, 64)
		if err != nil {
			return err
		}
		// preset: resolved !
		channel.ProfileID = schemaProfileID
		channel.Host      = serviceNode
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
	schemaProfileID, err := strconv.ParseInt(
		sender.Connection.String, 10, 64,
	)

	if err != nil {
		// MUST: be valid profile@webitel.chat.bot channel as a sender !
		return err
	}

	channel := &Channel {
		
		Log:   c.Log,
		Host:  "", // PEEK
		Agent: c.Client,
		Store: c.Store,
		// ChannelID: reflects .start channel member.id
		ID: sender.ConversationID,
		Chat: *(sender),
		// User: &chat.User{
		// 	UserId:     0, // profile.schema.id
		// 	Type:       "chatflow",
		// 	Connection: "",
		// 	Internal:   true,
		// },
		
		// DomainID: sender.DomainID,
		ProfileID: schemaProfileID, 

		Invite:  "", // .Invite(!) token
		Pending: "", // .WaitMessage(!) token
		
		// Created: date,
		// Updated: date,
		// Started: 0,
		// Joined:  0,
		// Closed:  0,
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

	c.Lock()   // +RW
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

func (c *Agent) CloseConversation(conversationID string) error {
	
	channel, err := c.GetChannel(conversationID)
	
	if err != nil {
		return err
	}

	err = channel.Close()

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
