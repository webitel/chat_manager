package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	// "net/http"

	"github.com/rs/zerolog"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/errors"
	"github.com/micro/micro/v3/util/selector"
	"github.com/micro/micro/v3/util/selector/random"

	chat "github.com/webitel/chat_manager/api/proto/chat"
	bot "github.com/webitel/chat_manager/api/proto/workflow"
	"github.com/webitel/chat_manager/app"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	strategy "github.com/webitel/chat_manager/internal/selector"
)

// Channel [FROM] chat.srv [TO] workflow
// CHAT communication channel; chat@bot
type Channel struct {
	// Client
	Log *zerolog.Logger
	// Host that routine .this chat@workflow channel
	Host  string // preffered: "workflow" service-node-id
	Agent bot.FlowChatServerService
	Store store.CacheRepository
	// Session
	ID string // Chat.ConversationID
	// Chat reflects unique originator's chat@webitel.chat.bot channel
	Chat store.Channel // embedded: originator (sender)
	// User *chat.User // TODO: flow schema @bot info
	ProfileID int64 // Disclose profile.schema.id
	// DomainID int64 // Chat.DomainID

	Invite  string // SESSION ID
	Pending string // .WaitMessage(token)

	// Created int64
	// Updated int64
	// Started int64
	// Joined int64
	// Closed int64

	// // LATEST
	// Update *chat.Message

	Variables map[string]string
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
/*func (c *Channel) lookup(services []*registry.Service) selector.Next {

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
		// return strategy.PrefferedHost("10.9.8.111")(services)
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
			Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
			Int64("pdc", c.DomainID()). // channel: primary domain component id
			Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
			Str("channel", "chatflow").
			Str("host", c.Host).   // WANTED
			Str("peek", "random"). // SELECT
			Str("error", "node: not found").
			Msg(perform)

		return selector.Random(services)
		// return strategy.PrefferedHost("10.9.8.111")(services)
	}

	var event *zerolog.Event

	if perform == "LOCATE" {
		event = c.Log.Info()
	} else {
		event = c.Log.Trace()
	}

	event.
		Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
		Int64("pdc", c.DomainID()). // channel: primary domain component id
		Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
		Str("channel", "chatflow").
		Str("host", c.Host).       // WANTED
		Str("addr", peer.Address). // FOUND
		Msg(perform)

	return func() (*registry.Node, error) {

		return peer, nil
	}
}*/

// call implements client.CallWrapper to keep tracking channel @workflow service node
func (c *Channel) callWrap(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {

		c.ID = c.ChatID() // resolve channel's chat-id ! early binding

		// doRequest
		err := next(ctx, addr, req, rsp, opts)
		//
		if err != nil {

			if c.Host != "" {
				c.Log.Warn().
					Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
					Int64("pdc", c.DomainID()). // channel: primary domain component id
					Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
					Str("channel", "chatflow").
					Str("seed", c.Host). // WANTED
					Str("peer", addr).   // REQUESTED
					Msg("LOST")
			}
			c.Host = ""

			re := errors.FromError(err)
			if re.Id == "go.micro.client" {
				if strings.HasPrefix(re.Detail, "service ") {
					if strings.HasSuffix(re.Detail, ": "+selector.ErrNoneAvailable.Error()) {
						// "{\"id\":\"go.micro.client\",\"code\":500,\"detail\":\"service workflow: not found\",\"status\":\"Internal Server Error\"}"
					}
				}
			}

			return err
		}

		if c.Host == "" {
			// NEW! Hosted!
			c.Host = addr
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Info().
				Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
				Int64("pdc", c.DomainID()). // channel: primary domain component id
				Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
				Str("channel", "chatflow").
				Str("peer", c.Host). // == addr
				Msg("HOSTED")

		} else if addr != c.Host {
			// Hosted! But JUST Served elsewhere ...
			var seed string             // WANTED
			seed, c.Host = c.Host, addr // RESET
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Info().
				Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
				Int64("pdc", c.DomainID()). // channel: primary domain component id
				Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
				Str("channel", "chatflow").
				Str("seed", seed). // WANTED
				Str("peer", addr). // SERVED
				Msg("RE-HOST")

			// c.Host = addr
		}

		return err
	}
}

// CallOption specific for this kind of channel(s)
func (c *Channel) callOpts(opts *client.CallOptions) {
	// apply .call options within .this channel ...
	for _, setup := range []client.CallOption{
		client.WithSelector(chatFlowSelector{c}),
		client.WithCallWrapper(c.callWrap),
	} {
		setup(opts)
	}
}

//
type chatFlowSelector struct {
	*Channel
}

var _ selector.Selector = chatFlowSelector{nil}

var randomize = random.NewSelector()

// Select a route from the pool using the strategy
func (c chatFlowSelector) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {
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
		// // return selector.Random(services)
		// return randomize.Select(hosts, opts...)
		return strategy.PrefferedHost("10.9.8.111")(hosts, opts...) // workflow
	}

	var peer string
	for _, host := range hosts {
		if host == c.Host {
			peer = host
			break
		}
	}

	if peer == "" {

		c.Log.Warn().
			Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
			Int64("pdc", c.DomainID()). // channel: primary domain component id
			Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
			Str("channel", "chatflow").
			Str("host", c.Host).   // WANTED
			Str("peek", "random"). // SELECT
			Str("error", "node: not found").
			Msg(perform)

		return randomize.Select(hosts, opts...)
		// return strategy.PrefferedHost("10.9.8.111")(services)
	}

	var event *zerolog.Event

	if perform == "LOCATE" {
		event = c.Log.Info()
	} else {
		event = c.Log.Trace()
	}

	event.
		Int64("pid", c.UserID()).   // channel: schema@bot.profile (external)
		Int64("pdc", c.DomainID()). // channel: primary domain component id
		Str("chat-id", c.ChatID()). // channel: chat@workflow.schema.bot (internal)
		Str("channel", "chatflow").
		Str("peer", c.Host). // WANTED & FOUND
		Msg(perform)

	return func() string {
		return peer
	}, nil
}

// Record the error returned from a route to inform future selection
func (c chatFlowSelector) Record(host string, err error) error {
	if err != nil {
		// TODO: Resolve error type & change node if needed !
	}
	return nil
}

// Reset the selector
func (chatFlowSelector) Reset() error {
	return nil
}

// String returns the name of the selector
func (chatFlowSelector) String() string {
	return "chatflow"
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
		c.Log.Warn().Str("chat-id", c.ID).Msg("CHAT Flow is NOT waiting for message(s); DO NOTHING MORE!")
		return nil
	}

	c.Log.Debug().
		Str("conversation_id", c.ID).
		Str("confirmation_id", string(pending)).
		Msg("send confirmed messages")

		// messages := []*bot.Message{
		// 	{
		// 		Id:   message.GetId(),
		// 		Type: message.GetType(),
		// 		Value: &bot.Message_Text{
		// 			Text: message.GetText(),
		// 		},
		// 	},
		// }
	sendMessage := &bot.ConfirmationMessageRequest{
		ConversationId: c.ID,
		ConfirmationId: pending,
		// Messages:       messages,
		Messages: []*chat.Message{message},
	}
	// PERFORM
	_, err = c.Agent.
		ConfirmationMessage(
			// canellation context
			context.TODO(),
			// request params
			sendMessage,
			// callOptions ...
			c.callOpts,
		)

	if err != nil {

		re := errors.FromError(err)

		switch re.Id {
		// "Chat: grpc.chat.conversation.not_found, Conversation %!d(string=0d882ad8-523a-4ed1-b36c-8c3f046e218e) not found"
		case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
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
				Int64("pid", c.UserID()).      // recepient: schema@bot.profile (internal)
				Str("channel-id", c.ChatID()). // sender: originator user@bot.profile (external)
				Str("error", re.Detail).
				Msg("SEND chat@bot") // TO: workflow
		}

		return re // errors.New(re.Error.Message)
	}

	// USED(!) remove ...
	c.Pending = ""
	_ = c.Store.DeleteConfirmation(c.ChatID())

	return nil
}

// Start @workflow.Start(!) chat channel routine
func (c *Channel) Start(message *chat.Message) error {

	c.Log.Debug().
		Interface("metadata", c.Variables).
		Str("conversation_id", c.ChatID()).
		Int64("profile_id", c.UserID()).
		Int64("domain_id", c.DomainID()).
		Interface("message", message).
		Msg("START Conversation")

	// const commandStart = "start"
	//messageText := commandStart

	// region: TEST PURPOSE ONLY !
	var schemaId int64
	if envars := c.Variables; len(envars) != 0 {
		schemaId, _ = strconv.ParseInt(c.Variables["flow"], 10, 64)
		delete(c.Variables, "flow")
		// REMOVE FOR PRODUCTION !
		metadata := message.Variables
		if metadata == nil {
			metadata = make(map[string]string, len(envars))
		}
		for key, val := range envars {
			metadata[key] = val
		}
		message.Variables = metadata
	}
	// endregion: TEST PURPOSE ONLY !

	start := &bot.StartRequest{

		ConversationId: c.ChatID(),
		DomainId:       c.DomainID(),
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId: c.UserID(),
		SchemaId:  int32(schemaId),
		Message:   message,
		// Message: &bot.Message{
		// 	Id:   message.GetId(),
		// 	Type: message.GetType(),
		// 	Value: &bot.Message_Text{
		// 		Text: messageText, // req.GetMessage().GetTextMessage().GetText(),
		// 	},
		// },

		Variables: c.Variables, // message.GetVariables(),
	}

	// if message != nil {

	// 	if message.File != nil{
	// 		start.Message.Value =
	// 			&bot.Message_File_{
	// 				File: &bot.Message_File{
	// 					Id:       message.File.GetId(),
	// 					Url:      message.File.GetUrl(),
	// 					MimeType: message.File.GetMime(),
	// 				},
	// 			}
	// 	}else{
	// 		if message.Text != "" {
	// 			messageText = message.Text
	// 		}

	// 		start.Message.Value = &bot.Message_Text{
	// 			Text: messageText,
	// 		}
	// 	}
	// }

	// Request to start flow-routine for NEW-chat incoming message !
	c.Host = "lookup" // NEW: selector.Random

	_, err := c.Agent.Start(
		// channel context
		context.TODO(), start,
		// callOptions
		c.callOpts,
	)

	if err != nil {

		c.Log.Error().Err(err).
			Msg("Failed to /start chat@bot routine")

		return err

	}

	// var re *errors.Error

	// if err != nil {
	// 	re = errors.FromError(err)
	// } else {
	// 	re = chatFlowError(res.GetError())
	// }

	// if re := res.GetError(); re != nil {

	// 	c.Log.Error().
	// 		Str("errno", re.GetId()).
	// 		Str("error", re.GetMessage()).
	// 		Msg("Failed to /start chat@bot routine")

	// 	// return errors.New(
	// 	// 	re.GetId(),
	// 	// 	re.GetMessage(),
	// 	// 	502, // 502 Bad Gateway
	// 	// 	// The server, while acting as a gateway or proxy,
	// 	// 	// received an invalid response from the upstream server it accessed
	// 	// 	// in attempting to fulfill the request.
	// 	// )
	// }

	c.Log.Info().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Int64("schema-id", schemaId).
		Str("host", c.Host).
		Str("channel-id", c.ID).
		Msg(">>>>> START <<<<<")

	return nil
}

func (c *Channel) startUser(message *chat.Message, userToID int64) error {

	c.Log.Debug().
		Interface("metadata", c.Variables).
		Str("conversation_id", c.ChatID()).
		Int64("user_id", userToID).
		Int64("domain_id", c.DomainID()).
		Interface("message", message).
		Msg("START Conversation")

	// const commandStart = "start"
	//messageText := commandStart

	start := &bot.StartRequest{

		ConversationId: c.ChatID(),
		DomainId:       c.DomainID(),
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId: c.UserID(),
		Message:   message,
		UserId:    userToID,
		// Message: &bot.Message{
		// 	Id:   message.GetId(),
		// 	Type: message.GetType(),
		// 	Value: &bot.Message_Text{
		// 		Text: messageText, // req.GetMessage().GetTextMessage().GetText(),
		// 	},
		// },

		Variables: c.Variables, // message.GetVariables(),
	}

	// if message != nil {

	// 	if message.File != nil{
	// 		start.Message.Value =
	// 			&bot.Message_File_{
	// 				File: &bot.Message_File{
	// 					Id:       message.File.GetId(),
	// 					Url:      message.File.GetUrl(),
	// 					MimeType: message.File.GetMime(),
	// 				},
	// 			}
	// 	}else{
	// 		if message.Text != "" {
	// 			messageText = message.Text
	// 		}

	// 		start.Message.Value = &bot.Message_Text{
	// 			Text: messageText,
	// 		}
	// 	}
	// }

	// Request to start flow-routine for NEW-chat incoming message !
	c.Host = "lookup" // NEW: selector.Random

	_, err := c.Agent.Start(
		// channel context
		context.TODO(), start,
		// callOptions
		c.callOpts,
	)

	if err != nil {

		c.Log.Error().Err(err).
			Msg("Failed to /start chat@bot routine")

		return err

	}

	// var re *errors.Error

	// if err != nil {
	// 	re = errors.FromError(err)
	// } else {
	// 	re = chatFlowError(res.GetError())
	// }

	// if re := res.GetError(); re != nil {

	// 	c.Log.Error().
	// 		Str("errno", re.GetId()).
	// 		Str("error", re.GetMessage()).
	// 		Msg("Failed to /start chat@bot routine")

	// 	// return errors.New(
	// 	// 	re.GetId(),
	// 	// 	re.GetMessage(),
	// 	// 	502, // 502 Bad Gateway
	// 	// 	// The server, while acting as a gateway or proxy,
	// 	// 	// received an invalid response from the upstream server it accessed
	// 	// 	// in attempting to fulfill the request.
	// 	// )
	// }

	c.Log.Info().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Int64("user-id", userToID).
		Str("host", c.Host).
		Str("channel-id", c.ID).
		Msg(">>>>> START <<<<<")

	return nil
}

// Close .this channel @workflow.Break(!)
func (c *Channel) Close(cause string) error {

	_, err := c.Agent.Break(
		// cancellation context
		context.TODO(),
		// request
		&bot.BreakRequest{
			ConversationId: c.ID,
			Cause:          cause,
		},
		// callOptions ...
		c.callOpts,
	)

	// var re *errors.Error

	if err != nil {
		re := errors.FromError(err)
		switch re.Id {
		case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
			// NOTE: got Not Found ! make idempotent !
			err = nil // break
			// return nil // no matter !

		default:

			return re // Failure !
		}

	} // else {
	// 	re = chatFlowError(res.GetError())
	// }

	// re := chatFlowError(res.GetError())

	// if re != nil {

	// 	switch re.Id {
	// 	case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
	// 		// NOTE: got Not Found ! make idempotent !
	// 		return nil // no matter !

	// 	default:

	// 		return re // Failure !
	// 	}
	// }

	c.Log.Warn().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("channel-id", c.ChatID()).
		Msg("<<<<< STOP >>>>>")
	// //s.chatCache.DeleteCachedMessages(conversationID)
	// s.chatCache.DeleteConfirmation(conversationID)
	// s.chatCache.DeleteConversationNode(conversationID)
	return nil
}

// BreakBridge .this channel @workflow.BreakBridge(!)
func (c *Channel) BreakBridge(cause BreakBridgeCause) error {

	_, err := c.Agent.BreakBridge(
		// cancellation context
		context.TODO(),
		// request
		&bot.BreakBridgeRequest{
			ConversationId: c.ChatID(),
			Cause:          cause.String(),
		},
		// callOptions
		c.callOpts,
	)

	if err != nil {
		re := errors.FromError(err)
		switch re.Id {
		case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
			// NOTE: got Not Found ! make idempotent !
			return nil // FIXME: no matter !
			// Affected ON .LeaveConversation, after .workflow service,
			// that run .this chat session - was stopped !

		default:

			return re // Failure !
		}

	} // else {
	// 	re = chatFlowError(res.GetError())
	// }

	// re := chatFlowError(res.GetError())

	// if re != nil {

	// 	switch re.Id {
	// 	case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
	// 		// NOTE: got Not Found ! make idempotent !
	// 		return nil // no matter !

	// 	default:

	// 		return re // Failure !
	// 	}
	// }

	c.Log.Warn().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("channel-id", c.ChatID()).
		Msg("<<<<< BREAK >>>>>")

	return nil
}

func (c *Channel) TransferToUser(originator *app.Channel, userToID int64) error {

	chatFromID := originator.Chat.ID
	// Stop current workflow schema routine on this c channel.
	err := c.Close("transfer") // workflow@Break()
	if err != nil {
		return err
	}

	// Format: [;]date:from:to
	// from: channel_id
	// to: user_id
	date := app.DateTimestamp(
		time.Now().UTC(),
	)
	xferNext := fmt.Sprintf("%d:%s:user:%d",
		date, chatFromID, userToID,
	)
	xferFull := xferNext
	xferThis := c.Variables["xfer"]
	if xferThis != "" {
		xferFull = strings.Join(
			// FIXME: PUSH ? OR APPEND ?
			// QUEUE ? OR STACK ?
			[]string{xferThis, xferNext}, ";",
		)
	}
	if c.Variables == nil {
		c.Variables = make(map[string]string)
	}
	c.Variables["xfer"] = xferFull
	// schemaThisID := c.Variables["flow"]
	// c.Variables["flow"] = strconv.FormatInt(schemaToID, 10)
	// Start NEW workflow schema routine within this c channel.
	user := originator.User
	err = c.startUser(&chat.Message{
		Id:   0,
		Type: "xfer",
		Text: "transfer",
		Variables: map[string]string{
			// "flow": strconv.FormatInt(schemaToID, 10),
			"xfer": xferNext,
		},
		CreatedAt: date,
		// originator.Chat.User
		From: &chat.Account{
			Id:        user.ID,
			Channel:   user.Channel,
			Contact:   user.Contact,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.UserName,
		},
	}, userToID,
	)

	if err != nil {
		// Restore current state
		c.Variables["xfer"] = xferThis
		// c.Variables["flow"] = schemaThisID
		return err
	}

	// _, err := c.Agent.TransferChatPlan(
	// 	// cancellation context
	// 	context.TODO(),
	// 	// request
	// 	&bot.TransferChatPlanRequest{
	// 		ConversationId: c.ChatID(),
	// 		PlanId: int32(schemaToID),
	// 	},
	// 	// callOptions
	// 	c.sendOptions,

	// )

	if err != nil {
		// re := errors.FromError(err)
		// switch re.Id {
		// case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
		// 	// NOTE: got Not Found ! make idempotent !
		// 	return nil // FIXME: no matter !
		// 	// Affected ON .LeaveConversation, after .workflow service,
		// 	// that run .this chat session - was stopped !

		// default:

		// 	return re // Failure !
		// }
		return err
	}

	c.Log.Info().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("flow-chat-id", c.ChatID()).
		Str("from-chat-id", chatFromID).
		Int64("to-user-id", userToID).
		Msg(">>>>> TRANSFERED <<<<<")

	return nil
}

func (c *Channel) TransferToSchema(originator *app.Channel, schemaToID int64) error {

	chatFromID := originator.Chat.ID
	// Stop current workflow schema routine on this c channel.
	err := c.Close("transfer") // workflow@Break()
	if err != nil {
		return err
	}

	// Format: [;]date:from:to
	// from: channel_id
	// to: schema_id
	date := app.DateTimestamp(
		time.Now().UTC(),
	)
	xferNext := fmt.Sprintf("%d:%s:schema:%d",
		date, chatFromID, schemaToID,
	)
	xferFull := xferNext
	xferThis := c.Variables["xfer"]
	if xferThis != "" {
		xferFull = strings.Join(
			// FIXME: PUSH ? OR APPEND ?
			// QUEUE ? OR STACK ?
			[]string{xferThis, xferNext}, ";",
		)
	}
	if c.Variables == nil {
		c.Variables = make(map[string]string)
	}
	c.Variables["xfer"] = xferFull
	schemaThisID := c.Variables["flow"]
	c.Variables["flow"] = strconv.FormatInt(schemaToID, 10)
	// Start NEW workflow schema routine within this c channel.
	user := originator.User
	err = c.Start(&chat.Message{
		Id:   0,
		Type: "xfer",
		Text: "transfer",
		Variables: map[string]string{
			"flow": strconv.FormatInt(schemaToID, 10),
			"xfer": xferNext,
		},
		CreatedAt: date,
		// originator.Chat.User
		From: &chat.Account{
			Id:        user.ID,
			Channel:   user.Channel,
			Contact:   user.Contact,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.UserName,
		},
	})

	if err != nil {
		// Restore current state
		c.Variables["xfer"] = xferThis
		c.Variables["flow"] = schemaThisID
		return err
	}

	// _, err := c.Agent.TransferChatPlan(
	// 	// cancellation context
	// 	context.TODO(),
	// 	// request
	// 	&bot.TransferChatPlanRequest{
	// 		ConversationId: c.ChatID(),
	// 		PlanId: int32(schemaToID),
	// 	},
	// 	// callOptions
	// 	c.sendOptions,

	// )

	if err != nil {
		// re := errors.FromError(err)
		// switch re.Id {
		// case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
		// 	// NOTE: got Not Found ! make idempotent !
		// 	return nil // FIXME: no matter !
		// 	// Affected ON .LeaveConversation, after .workflow service,
		// 	// that run .this chat session - was stopped !

		// default:

		// 	return re // Failure !
		// }
		return err
	}

	c.Log.Info().
		Int64("pdc", c.DomainID()).
		Int64("pid", c.UserID()).
		Str("host", c.Host).
		Str("flow-chat-id", c.ChatID()).
		Str("from-chat-id", chatFromID).
		Int64("to-schema-id", schemaToID).
		Msg(">>>>> TRANSFERED <<<<<")

	return nil
}

const (
	errnoSessionNotFound = "grpc.chat.conversation.not_found"
)

// func chatFlowError(err *chat.Error) *errors.Error {

// 	if err == nil || (err.Id == "" && err.Message == "") {
// 		return nil
// 	}

// 	switch err.GetId() {
// 	// "Chat: grpc.chat.conversation.not_found, Conversation %!d(string=0d882ad8-523a-4ed1-b36c-8c3f046e218e) not found"
// 	case errnoSessionNotFound: // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found

// 		code := http.StatusNotFound

// 		return &errors.Error{
// 			Id:     err.Id,
// 			Code:   (int32)(code),
// 			Detail: err.Message,
// 			Status: http.StatusText(code),
// 		}

// 	// default:
// 	}

// 	code := http.StatusInternalServerError

// 	return &errors.Error{
// 		Id:     err.Id,
// 		Code:   (int32)(code),
// 		Detail: err.Message,
// 		Status: http.StatusText(code),
// 	}
// }
