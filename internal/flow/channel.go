package flow

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	// "net/http"

	"github.com/golang/protobuf/proto"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/errors"
	"github.com/micro/micro/v3/util/selector"
	"github.com/micro/micro/v3/util/selector/random"

	chat "github.com/webitel/chat_manager/api/proto/chat"
	bot "github.com/webitel/chat_manager/api/proto/workflow"
	"github.com/webitel/chat_manager/app"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	wlog "github.com/webitel/chat_manager/log"
)

// Channel [FROM] chat.srv [TO] workflow
// CHAT communication channel; chat@bot
type Channel struct {
	// Client
	Log *slog.Logger
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
	Invite string // SESSION ID

	mx      sync.Mutex
	pending string // .WaitMessage(token)

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

	log *slog.Logger,
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
				c.Log.Warn("[ CHAT::FLOW ] LOST",
					slog.Int64("pid", c.UserID()),   // channel: schema@bot.profile (external)
					slog.Int64("pdc", c.DomainID()), // channel: primary domain component id
					// slog.String("channel", "chatflow"),
					slog.String("chat-id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
					slog.String("conversation_id", c.ChatID()),
					slog.String("host", c.Host), // WANTED
					slog.String("addr", addr),   // REQUESTED
					slog.Any("error", err),
				)
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

			c.Log.Info("[ CHAT::FLOW ] HOSTED",
				slog.Int64("pid", c.UserID()),      // channel: schema@bot.profile (external)
				slog.Int64("pdc", c.DomainID()),    // channel: primary domain component id
				slog.String("chat-id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
				// slog.String("channel", "chatflow"),
				slog.String("conversation_id", c.ChatID()),
				slog.String("host", c.Host), // == addr
			)

		} else if addr != c.Host {
			// Hosted! But JUST Served elsewhere ...
			var seed string             // WANTED
			seed, c.Host = c.Host, addr // RESET
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Info("[ CHAT::FLOW ] REHOST",
				slog.Int64("pid", c.UserID()),      // channel: schema@bot.profile (external)
				slog.Int64("pdc", c.DomainID()),    // channel: primary domain component id
				slog.String("chat-id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
				// slog.String("channel", "chatflow"),
				slog.String("conversation_id", c.ChatID()),
				slog.String("lost", seed), // WANTED
				slog.String("host", addr), // SERVED
			)

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

type chatFlowSelector struct {
	*Channel
}

var _ selector.Selector = chatFlowSelector{nil}

var randomize = random.NewSelector()

// Select a route from the pool using the strategy
func (c chatFlowSelector) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {
	lookup := client.DefaultClient.Options().Selector
	if lookup == nil {
		lookup = randomize
	}

	perform := "LOOKUP"
	// region: recover .this channel@workflow service node
	if c.Host == "lookup" {
		c.Host = "" // RESET
	} else if c.Host == "" && c.ChatID() != "" {

		node, err := c.Store.ReadConversationNode(c.ID)

		if err != nil {
			c.Log.Error("RECOVER Failed lookup store for chat@workflow channel host",
				slog.String("chat-id", c.ID), // channel: chat@workflow.schema.bot (internal)
				slog.String("channel", "chatflow"),
				slog.String("conversation_id", c.ID),
			)

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
		// return selector.Random(services)
		return lookup.Select(hosts, opts...)
		// return strategy.PrefferedHost("127.0.0.1")(hosts, opts...) // workflow
	}

	var peer string
	for _, host := range hosts {
		if host == c.Host {
			peer = host
			break
		}
	}

	if peer == "" {

		c.Log.Warn("[ CHAT::FLOW ] "+perform,
			slog.Int64("pid", c.UserID()),      // channel: schema@bot.profile (external)
			slog.Int64("pdc", c.DomainID()),    // channel: primary domain component id
			slog.String("chat-id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
			// slog.String("channel", "chatflow"),
			slog.String("conversation_id", c.ChatID()),
			slog.String("host", c.Host),   // WANTED
			slog.String("next", "random"), // SELECT
			slog.String("error", "node: not found"),
		)

		return lookup.Select(hosts, opts...)
		// return strategy.PrefferedHost("10.9.8.111")(services)
	}

	l := c.Log.With(
		slog.Int64("pid", c.UserID()),      // channel: schema@bot.profile (external)
		slog.Int64("pdc", c.DomainID()),    // channel: primary domain component id
		slog.String("chat-id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
		slog.String("channel", "chatflow"),
		slog.String("conversation_id", c.ChatID()), // channel: chat@workflow.schema.bot (internal)
		slog.String("host", c.Host),                // WANTED & FOUND
	)

	if perform == "LOCATE" {
		l.Info("[ CHAT::FLOW ] " + perform)
	} // else {
	// 	l.Debug(perform)
	// }

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

func (c *Channel) getPending() string {
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.pending
}

func (c *Channel) setPending(nextToken, lastToken string) bool {
	c.mx.Lock()
	defer c.mx.Unlock()
	if lastToken != "*" && c.pending != lastToken {
		return false
	}
	c.pending = nextToken
	return true
}

func (c *Channel) delPending(usedToken string) {
	c.setPending("", usedToken)
}

// expose postback.code as a message.text if specified
func flowMessage(message *chat.Message) *chat.Message {
	postback := message.Postback
	if postback.GetCode() == "" {
		return message // original
	}
	if message.Text == postback.Code {
		return message // nothing to change
	}
	// Here we need to send postback.code as a message.text
	// due to flow.schema(bot) might react on code(s), not "text" !
	message = proto.Clone(message).(*chat.Message)
	message.Text = postback.Code
	return message
}

// Send @workflow.ConfirmationMessage() or restart @workflow.Start()
func (c *Channel) Send(message *chat.Message) (err error) {

	// pending := c.Pending // token
	pending := c.getPending()
	if pending == "" {
		pending, err = c.Store.ReadConfirmation(c.ID)

		if err != nil {
			c.Log.Error("Failed to get {flow.recvMessage.token} from store",
				"chat-id", c.ID, // channel: chat@workflow.schema.bot (internal)
				"error", err,
			)
			return err
		}

		// c.Pending = pending
		// c.setPending(pending, "")
	}
	// Flow.WaitMessage()
	if pending == "" {
		// FIXME: NO confirmation found for chat - means that we are not in {waitMessage} block ?
		c.Log.Debug("[ CHAT::FLOW ] IDLE",
			"chat-id", c.ID, // channel: chat@workflow.schema.bot (internal)
			"conversation_id", c.ID,
		)
		return nil
	}

	c.Log.Debug("[ CHAT::FLOW ] Delivery",
		"conversation_id", c.ID, // channel: chat@workflow.schema.bot (internal)
		"confirmation_id", pending,
		"msg.type", message.Type,
		"msg.id", message.Id,
	)

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
		Messages: []*chat.Message{
			flowMessage(message),
		},
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
			// c.Pending = ""
			c.setPending("", "*") // c.delPending("*")

			_ = c.Store.DeleteConfirmation(c.ID)
			_ = c.Store.DeleteConversationNode(c.ID)

			c.Log.Warn(">>> RE-START! <<<",
				slog.Int64("pid", c.UserID()), // recepient: schema@bot.profile (internal)
				slog.Int64("pdc", c.DomainID()),
				slog.String("channel-id", c.ChatID()), // sender: originator user@bot.profile (external)
				slog.String("cause", "grpc.chat.conversation.not_found"),
			)

			// TODO: ensure this.(ID|ProfileID|DomainID)
			err = c.Start(message)

			// if err != nil {
			// 	c.Log.Error().Err(err).Str("channel-id", c.ID).Msg("RE-START Failed")
			// } else {
			// 	c.Log.Info().Str("channel-id", c.ID).Msg("RE-START!")
			// }

			return err // break

		default:

			c.Log.Error("SEND chat@bot", // TO: workflow
				slog.Int64("pdc", c.DomainID()),
				slog.Int64("pid", c.UserID()),         // recepient: schema@bot.profile (internal)
				slog.String("channel-id", c.ChatID()), // sender: originator user@bot.profile (external)
				slog.String("error", re.Detail),
			)
		}

		return re // errors.New(re.Error.Message)
	}

	// USED(!) remove ...
	// c.Pending = ""
	c.delPending(pending)
	_ = c.Store.DeleteConfirmation(c.ChatID())

	return nil
}

// Start @workflow.Start(!) chat channel routine
func (c *Channel) Start(message *chat.Message) error {

	// c.Log.Debug("START Conversation",
	// 	slog.String("conversation_id", c.ChatID()),
	// 	slog.Int64("profile_id", c.UserID()),
	// 	slog.Int64("domain_id", c.DomainID()),
	// 	slog.Any("metadata", c.Variables),
	// 	slog.Any("message", message),
	// )

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

	level := slog.LevelDebug
	debugCtx := []any{
		"msg", wlog.DeferValue(func() slog.Value {
			msg := []slog.Attr{
				slog.Int64("id", message.Id),
				slog.String("type", message.Type),
			}
			// if message.Text != "" || message.Type == "text" {
			// 	msg = append(msg,
			// 		slog.String("text", message.Text),
			// 	)
			// }
			// if message.File != nil {
			// 	msg = append(msg,
			// 		slog.String("file", wlog.JsonValue(message.File)),
			// 	)
			// }
			return slog.GroupValue(msg...)
		}),
		"from", wlog.DeferValue(func() slog.Value {
			fromChat := &c.Chat
			fromUser := message.From
			return slog.GroupValue(
				slog.String("id", fromChat.ID),
				slog.String("via", fmt.Sprintf("%d@%s", c.ProfileID, fromChat.ServiceHost.String)),
				slog.String("user", fromUser.Channel+":"+fromUser.Contact),
				slog.String("title", fromUser.FirstName),
			)
		}),
		"chat", wlog.DeferValue(func() slog.Value {
			return slog.GroupValue(
				slog.String("id", c.ChatID()),
				slog.Int64("dc", c.DomainID()),
				slog.String("user", "bot:"+strconv.FormatInt(schemaId, 10)),
				// slog.String("title", schemaName),
				slog.String("thread.id", c.ChatID()), // conversation_id
				slog.String("metadata", wlog.JsonValue(c.Variables)),
			)
		}),
		// copy of [TO] chat.thread.id
		"conversation_id", c.ChatID(),
	}

	c.Log.Log(
		context.TODO(), level,
		"[ CHAT::FLOW ] Setup",
		debugCtx...,
	)

	start := &bot.StartRequest{

		ConversationId: c.ChatID(),
		DomainId:       c.DomainID(),
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId: c.UserID(),
		SchemaId:  int32(schemaId),
		Message:   flowMessage(message),
		// Message: &bot.Message{
		// 	Id:   message.GetId(),
		// 	Type: message.GetType(),
		// 	Value: &bot.Message_Text{
		// 		Text: messageText, // req.GetMessage().GetTextMessage().GetText(),
		// 	},
		// },

		Variables: c.Variables, // message.GetVariables(),
	}

	c.setPending("", "*") // clear: wait.token; affects on XFER start NEW schema

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

		c.Log.Log(
			context.TODO(), slog.LevelError,
			"[ CHAT::FLOW ] START error",
			append(debugCtx, "error", err)...,
		)

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

	c.Log.Log(
		context.TODO(), level,
		"[ CHAT::FLOW ] START",
		append(debugCtx, "chat.host", "workflow@"+c.Host)...,
	)

	return nil
}

func (c *Channel) startUser(message *chat.Message, userToID int64) error {

	c.Log.Debug("START Conversation",
		slog.String("conversation_id", c.ChatID()),
		slog.Int64("user_id", userToID),
		slog.Int64("domain_id", c.DomainID()),
		slog.Any("metadata", c.Variables),
		slog.Any("message", message),
	)

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
		c.Log.Error("Failed to /start chat@bot routine",
			slog.Any("error", err),
		)

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

	c.Log.Info(">>>>> START <<<<<",
		slog.Int64("pdc", c.DomainID()),
		slog.Int64("pid", c.UserID()),
		slog.Int64("user-id", userToID),
		slog.String("host", c.Host),
		slog.String("channel-id", c.ID),
	)

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

	// Clear schema.(recvMessage).token of stopped channel !
	c.delPending("*")
	_ = c.Store.DeleteConfirmation(c.ChatID())

	c.Log.Warn("[ CHAT::FLOW ] STOP",
		slog.Int64("pdc", c.DomainID()),
		slog.Int64("pid", c.UserID()),
		slog.String("host", c.Host),
		slog.String("channel-id", c.ChatID()),
		slog.String("conversation_id", c.ChatID()),
	)

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
			Cause:          cause.String(), // strings.ToLower(),
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

	c.Log.Warn("[ CHAT::FLOW ] BREAK",
		slog.Int64("pdc", c.DomainID()),
		slog.Int64("pid", c.UserID()),
		slog.String("host", c.Host),
		slog.String("channel-id", c.ChatID()),
		slog.String("conversation_id", c.ChatID()),
	)

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

	// Save/Update chat.(conversation).variables changes ...
	if e := c.Store.Setvar(c.ID, map[string]string{
		"xfer": c.Variables["xfer"],
	}); e != nil {
		// [WARN] Failed to persist chat.(conversation).variables
		c.Log.Warn("Failed to update chat variables",
			"var.xfer", xferFull,
			"chat.id", c.ID, // NEW name
			"conversation_id", c.ID, // OLD name
			"error", e,
		)
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

	c.Log.Info(">>>>> TRANSFERED <<<<<",
		slog.Int64("pdc", c.DomainID()),
		slog.Int64("pid", c.UserID()),
		slog.String("host", c.Host),
		slog.String("flow-chat-id", c.ChatID()),
		slog.String("from-chat-id", chatFromID),
		slog.Int64("to-user-id", userToID),
	)

	return nil
}

// c => Conversation Channel (Schema:BOT)
// originator => Who(Operator!) started the XFER action (disconnected)
// schemaToID => NEW schema to run
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
	schemaNextID := strconv.FormatInt(schemaToID, 10)
	c.Variables["flow"] = schemaNextID
	// Start NEW workflow schema routine within this c channel.
	user := originator.User
	err = c.Start(&chat.Message{
		Id:   0,
		Type: "xfer",
		Text: "transfer",
		Variables: map[string]string{
			"flow": schemaNextID,
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

	// Save/Update chat.(conversation).variables changes ...
	if e := c.Store.Setvar(c.ID, map[string]string{
		"flow": schemaNextID,
		"xfer": xferFull,
	}); e != nil {
		// [WARN] Failed to persist chat.(conversation).variables
		c.Log.Warn("Failed to update chat variables",
			"var.flow", schemaToID,
			"var.xfer", xferFull,
			"chat.id", c.ID, // NEW name
			"conversation_id", c.ID, // OLD name
			"error", e,
		)
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

	c.Log.Info(">>>>> TRANSFERED <<<<<",
		slog.Int64("pdc", c.DomainID()),
		slog.Int64("pid", c.UserID()),
		slog.String("host", c.Host),
		slog.String("flow-chat-id", c.ChatID()),
		slog.String("from-chat-id", chatFromID),
		slog.Int64("to-schema-id", schemaToID),
	)

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
