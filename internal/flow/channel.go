package flow

import (
	"strings"
	"strconv"

	"errors"
	"context"
	// "github.com/micro/go-micro/v2/errors"

	"github.com/rs/zerolog"

	// strategy "github.com/webitel/chat_manager/internal/selector"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	
	chat "github.com/webitel/chat_manager/api/proto/chat"
	bot "github.com/webitel/chat_manager/api/proto/workflow"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"
	
)


// Channel [FROM] chat.srv [TO] workflow
// CHAT communication channel; chat@bot
type Channel struct {
	// Client
	Log   *zerolog.Logger
	// Host that routine .this chat@workflow channel
	Host string // preffered: "workflow" service-node-id
	Agent bot.FlowChatServerService
	Store store.CacheRepository
	// Session
	ID string // Chat.ConversationID
	// Chat reflects unique originator's chat@webitel.chat.bot channel
	Chat store.Channel // embedded: originator (sender)
	// User *chat.User // TODO: flow schema @bot info
	ProfileID int64 // Disclose profile.schema.id
	// DomainID int64 // Chat.DomainID
	
	Invite string // SESSION ID
	Pending string // .WaitMessage(token)

	// Created int64
	// Updated int64
	// Started int64
	// Joined int64
	// Closed int64

	// // LATEST
	// Update *chat.Message

}

// NewChannel chat@workflow
func NewChannel(

	log *zerolog.Logger,
	store store.CacheRepository,
	agent bot.FlowChatServerService,

) *Channel {

	return &Channel{
		Log:   log,
		Store: store,
		Agent: agent,
	}
}

// ChatID unique chat channel id
// in front of @workflow service
func (c *Channel) ChatID() string {
	if c.ID == "" {
		c.ID = c.Chat.ConversationID
	}
	return c.ID
}

// DomainID that .this channel chat belongs to ...
func (c *Channel) DomainID() int64 {
	return c.Chat.DomainID
}

// UserID defines end-user for .this channel {.ProfileID}
// This MUST be the flow ${schema.id} which acts as a webitel @bot
func (c *Channel) UserID() int64 {

	if c.ProfileID == 0 {
		if originator := c.Chat.Connection.String; originator != "" {
			c.ProfileID, _ = strconv.ParseInt(originator, 10, 64)
		}
	}

	return c.ProfileID
}


// lookup is client.Selector.Strategy to peek preffered @workflow node,
// serving .this specific chat channel
func (c *Channel) lookup(services []*registry.Service) selector.Next {

	perform := "LOOKUP"
	// region: recover .this channel@workflow service node
	if c.Host == "lookup" {
		c.Host = "" // RESET
	} else if c.Host == "" && c.ChatID() != "" {
			
		node, err := c.Store.ReadConversationNode(c.ID)
	
		if err != nil {
			
			c.Log.Error().Err(err).
				Str("chat-id", c.ID).
				Str("channel", "chatflow").
				Msg("RECOVER Failed lookup store for chat@workflow channel host")
			
			c.Host = ""
		
		} else if node != "" {

			c.Host = node
			perform = "LOCATE"

			// c.Log.Info().
			// 	Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
			// 	Int64("pdc", c.DomainID()). // channel: primary domain component id
			// 	Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
			// 	Str("channel", "chatflow").
			// 	Str("host", c.Host).
			// 	Msg("RECOVERY")
		}
	
	} // else if c.Host != "" {
		
	// 	// c.Log.Debug().
	// 	// 	Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
	// 	// 	Int64("pdc", c.DomainID()). // channel: primary domain component id
	// 	// 	Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
	// 	// 	Str("channel", "chatflow").
	// 	// 	Str("host", c.Host).
	// 	// 	Msg("LOOKUP")
	// }
	// endregion
	
	if c.Host == "" {
		// START
		return selector.Random(services)
	}
	
	var peer *registry.Node
	
	lookup:
	for _, service := range services {
		for _, node := range service.Nodes {
			if strings.HasSuffix(node.Id, c.Host) {
				peer = node
				break lookup
			}
		}
	}

	if peer == nil {
		
		c.Log.Warn().
			Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
			Int64("pdc", c.DomainID()). // channel: primary domain component id
			Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
			Str("channel", "chatflow").
			Str("host", c.Host). // WANTED
			Str("peek", "random"). // SELECT
			Str("error", "node: not found").
			Msg(perform)

		return selector.Random(services)
	}

	var event *zerolog.Event

	if perform == "LOCATE" {
		event = c.Log.Info()
	} else {
		event = c.Log.Trace()
	}

	event.
		Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
		Int64("pdc", c.DomainID()). // channel: primary domain component id
		Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
		Str("channel", "chatflow").
		Str("host", c.Host). // WANTED
		Str("addr", peer.Address). // FOUND
		Msg(perform)
	
	return func() (*registry.Node, error) {

		return peer, nil
	}
}

// call implements client.CallWrapper to keep tracking channel @workflow service node
func (c *Channel) call(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
		
		c.ID = c.ChatID() // resolve channel's chat-id ! early binding

		// doRequest
		err := next(ctx, node, req, rsp, opts)
		// 
		if err != nil {
			if c.Host != "" {
				c.Log.Warn().
					Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
					Int64("pdc", c.DomainID()). // channel: primary domain component id
					Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
					Str("channel", "chatflow").
					Str("peer", c.Host). // WANTED
					Str("host", node.Id). // REQUESTED
					Str("addr", node.Address).
					Msg("LOST")
			}
			c.Host = ""
			return err
		}

		if c.Host == "" {
			// NEW! Hosted!
			c.Host = node.Id
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Info().
				Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
				Int64("pdc", c.DomainID()). // channel: primary domain component id
				Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
				Str("channel", "chatflow").
				Str("host", c.Host).
				Str("addr", node.Address).
				Msg("HOSTED")
		
		} else if node.Id != c.Host {
			// Hosted! But JUST Served elsewhere ...
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Info().
				Int64("pid", c.UserID()). // channel: schema@bot.profile (external)
				Int64("pdc", c.DomainID()). // channel: primary domain component id
				Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
				Str("channel", "chatflow").
				Str("peer", c.Host). // WANTED
				Str("host", node.Id). // SERVED
				Str("addr", node.Address).
				Msg("RE-HOST")

			c.Host = node.Id
		}

		return err
	}
}

// CallOption specific for this kind of channel(s)
func (c *Channel) sendOptions(opts *client.CallOptions) {
	// apply .call options for .this channel ...
	client.WithSelectOption(
		selector.WithStrategy(c.lookup),
	)(opts)
	client.WithCallWrapper(c.call)(opts)
}

// Send @workflow.ConfirmationMessage() or restart @workflow.Start()
func (c *Channel) Send(message *chat.Message) (err error) {
	
	pending := c.Pending // token
	if pending == "" {
		pending, err = c.Store.ReadConfirmation(c.ID)
		
		if err != nil {
			c.Log.Error().Err(err).Str("chat-id", c.ID).Msg("Failed to get {chat.recvMessage.token} from store")
			return err
		}

		c.Pending = pending
	}
	// Flow.WaitMessage() 
	if pending == "" {
		// FIXME: NO confirmation found for chat - means that we are not in {waitMessage} block ?
		c.Log.Warn().Str("chat-id", c.ID).Msg("CHAT Flow is NOT waiting for text message(s); DO NOTHING MORE!")
		return nil
	}
	
	c.Log.Debug().
		Str("conversation_id", c.ID).
		Str("confirmation_id", string(pending)).
		Msg("send confirmed messages")
	
		messages := []*bot.Message{
		{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &bot.Message_Text{
				Text: message.GetText(),
			},
		},
	}
	sendMessage := &bot.ConfirmationMessageRequest{
		ConversationId: c.ID,
		ConfirmationId: pending,
		Messages:       messages,
	}
	// PERFORM
	re, err := c.Agent.
	ConfirmationMessage(
		// canellation context
		context.TODO(),
		// request params
		sendMessage,
		// callOptions ...
		c.sendOptions,
	)

	if err != nil {
		return err
	}
	
	if re.Error != nil {

		switch re.Error.Id {
		// "Chat: grpc.chat.conversation.not_found, Conversation %!d(string=0d882ad8-523a-4ed1-b36c-8c3f046e218e) not found"
		case "grpc.chat.conversation.not_found": // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
			// FIXME: RE-start chat@bot routine (?)
			// RESET
			c.Host = ""
			c.Pending = ""

			_ = c.Store.DeleteConfirmation(c.ID)
			_ = c.Store.DeleteConversationNode(c.ID)
			
			c.Log.Warn().
				Int64("pid", c.UserID()). // recepient: schema@bot.profile (internal)
				Int64("pdc", c.DomainID()).
				Str("channel-id", c.ChatID()). // sender: originator user@bot.profile (external)
				Str("cause", "grpc.chat.conversation.not_found").
				Msg(">>> RE-START! <<<")
			
			// TODO: ensure this.(ID|ProfileID|DomainID)
			err = c.Start(message)
			
			// if err != nil {
			// 	c.Log.Error().Err(err).Str("channel-id", c.ID).Msg("RE-START Failed")
			// } else {
			// 	c.Log.Info().Str("channel-id", c.ID).Msg("RE-START!")
			// }

			return err // break

		default:

			c.Log.Error().
				Int64("pdc", c.DomainID()).
				Int64("pid", c.UserID()). // recepient: schema@bot.profile (internal)
				Str("channel-id", c.ChatID()). // sender: originator user@bot.profile (external)
				Str("error", re.Error.Message).
				Msg("SEND chat@bot") // TO: workflow
		}
		// default:
		return errors.New(re.Error.Message)
	}
	// USED(!) remove ...
	c.Pending = ""
	_ = c.Store.DeleteConfirmation(c.ChatID())

	return nil
}

// Start @workflow.Start(!) chat channel routine
func (c *Channel) Start(message *chat.Message) error {
	
	c.Log.Debug().
		Str("conversation_id", c.ChatID()).
		Int64("profile_id", c.UserID()).
		Int64("domain_id", c.DomainID()).
		Msg("init conversation")
	
	const commandStart = "start"
	messageText := commandStart
	start := &bot.StartRequest{
		
		DomainId:       c.DomainID(),
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId:      c.UserID(),
		ConversationId: c.ChatID(),

		Message: &bot.Message{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &bot.Message_Text{
				Text: messageText, // req.GetMessage().GetTextMessage().GetText(),
			},
		},

		Variables: message.GetVariables(),
	}


	if message != nil {
		
		switch e := message.GetValue().(type) {
		case *chat.Message_Text: // TEXT
			
			if e.Text != "" {
				messageText = e.Text
			}
			
			start.Message.Value =
				&bot.Message_Text{
					Text: messageText,
				}

		case *chat.Message_File_: // FILE

			// messageText = "File"
			start.Message.Value =
				&bot.Message_File_{
					File: &bot.Message_File{
						Id:       e.File.GetId(),
						Url:      e.File.GetUrl(),
						MimeType: e.File.GetMimeType(),
					},
				}
		}
	}

	// Request to start flow-routine for NEW-chat incoming message !
	c.Host = "lookup" // NEW: selector.Random
	
	res, err := c.Agent.Start(
		// channel context
		context.Background(), start,
		// callOptions
		c.sendOptions,
	)

	// event := zerolog.Dict().
	// Int64("pdc", c.DomainID).
	// Int64("pid", c.ProfileID).
	// Str("channel-id", c.ID)

	if err != nil {
		
		c.Log.Error().Err(err).
			Msg("Failed to /start chat@bot routine")
		
		return err

	}
	
	if re := res.GetError(); re != nil {

		c.Log.Error().
			Str("errno", re.GetId()).
			Str("error", re.GetMessage()).
			Msg("Failed to /start chat@bot routine")

		// return errors.New(
		// 	re.GetId(),
		// 	re.GetMessage(),
		// 	502, // 502 Bad Gateway
		// 	// The server, while acting as a gateway or proxy,
		// 	// received an invalid response from the upstream server it accessed
		// 	// in attempting to fulfill the request.
		// )
	}

	c.Log.Info().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("channel-id", c.ID).
		Msg(">>>>> START <<<<<")

	return nil
}

// Close .this channel @workflow.Break(!)
func (c *Channel) Close() error {

	if res, err := c.Agent.Break(
		// cancellation context
		context.TODO(),
		// request
		&bot.BreakRequest{
			ConversationId: c.ID,
		},
		// callOptions ...
		c.sendOptions,

	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}

	c.Log.Warn().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("channel-id", c.ChatID()).
		Msg("<<<<< CLOSE >>>>>")
	// //s.chatCache.DeleteCachedMessages(conversationID)
	// s.chatCache.DeleteConfirmation(conversationID)
	// s.chatCache.DeleteConversationNode(conversationID)
	return nil
}

// BreakBridge .this channel @workflow.BreakBridge(!)
func (c *Channel) BreakBridge(cause BreakBridgeCause) error {

	if res, err := c.Agent.BreakBridge(
		// cancellation context
		context.TODO(),
		// request
		&bot.BreakBridgeRequest{
			ConversationId: c.ChatID(),
			Cause:          cause.String(),
		},
		// callOptions
		c.sendOptions,

	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}

	c.Log.Warn().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("channel-id", c.ChatID()).
		Msg("<<<<< BREAK >>>>>")

	return nil
}
