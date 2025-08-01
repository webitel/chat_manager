package bot

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"strconv"
	"sync"
	"time"

	wlog "github.com/webitel/chat_manager/log"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/server"
	"github.com/micro/micro/v3/util/selector"

	"github.com/micro/micro/v3/service/context/metadata"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	strategy "github.com/webitel/chat_manager/internal/selector"
)

var FileUploadPolicyError = errors.New("this type of file is not allowed to upload")

type Channel struct {
	Host string // webitel.chat.server node-id serving .this channel

	Title   string  // chat channel title
	ChatID  string  // chat.external.id (provider:chat.id) .ExternalID  .AuthUserID
	Account Account // chat.external.user.contact

	ChannelID string // chat.channel.id  .ID
	SessionID string // chat.session.id  .ConverationID
	// Contact  *Account // chat.channel.user
	Properties interface{} // driver channel bindings

	// Title  string // chat channel title
	// ChatID string // chat.external.id (provider:chat.id) .ExternalID  .AuthUserID
	// Username string // chat channel username
	// ContactID int64   // internal.user.id

	// protects the method Recv() as a main onNewMessage handler
	recvMx sync.Mutex
	// Closed indicates that .this channel was previously closed at timestamp
	Closed int64

	Log     *slog.Logger
	Gateway *Gateway
}

func (c *Channel) IsNew() bool {
	return c == nil || c.ChannelID == ""
}

func (c *Channel) DomainID() int64 {
	return c.Gateway.DomainID()
}

func (c *Channel) ProfileID() int64 {
	return c.Gateway.Bot.GetId()
}

// func (c *Channel) AccountID() int64 {
// 	return c.Account.ID
// }

// BotID defines workflow.schema.id internal @bot end-user
func (c *Channel) BotID() int64 {
	return c.Gateway.Bot.GetId() // .GetFlow().GetId()
}

// func (c *Channel) ContactID() int64 {
// 	return c.ContactID
// }

func (c *Channel) Provider() string {
	return c.Gateway.External.String()
}

// func (c *Channel) Close(cause string) (err error) {
func (c *Channel) Close() (err error) {

	bot := c.Gateway
	bot.Lock() // +RW
	e, ok := bot.external[c.ChatID]
	if ok = (ok && e == c); ok && c.Closed != 0 {
		delete(bot.external, c.ChatID)
		delete(bot.internal, c.Account.ID) // delete(c.Gateway.internal, c.ContactID)
		if len(bot.external) == 0 {
			// if !bot.Enabled {
			// 	// NOTE: Removes srv.gateways[uri] index
			// 	_ = bot.Deregister(context.TODO())
			// }
			if bot.deleted {
				// NOTE: Removes srv.gateways[uri] index
				_ = bot.Deregister(context.TODO())
				// NOTE: Removes srv.profiles[oid] index
				_ = bot.Remove()
				_ = bot.External.Close()
			} else if !bot.Enabled {
				c.Log.Warn("[ CHAT::CLOSE ]", "error", "gateway is disabled")
			}
		}
		// if len(bot.external) == 0 {
		// 	// NOTE: We just destroy the last active channel link
		// 	if !bot.Enabled {
		// 		// DISABLED !
		// 		_ = bot.Remove()
		// 	} // else if next := bot.next; next != nil {
		// }
		// 		// UPGRADE !
		// 		// We have NEW agent revision: close THIS and start NEXT !
		// 		srv := bot.Internal
		// 		srv.indexMx.Lock()   // +RW
		// 		srv.profiles[bot.Id] = next
		// 		srv.indexMx.Unlock() // +RW
		// 		// TODO: Dispose(bot) !
		// 		next.Log.Info().Msg("UPGRADED")
		// 	}
		// }
	}
	bot.Unlock() // -RW

	if !ok {
		// panic("channel: not running !")
		return nil // make: idempotent !
	}

	// if ok && c.Closed != 0 {
	// 	// NOTE: we are done ! Close confirmed by server sent final .message.text "Conversation closed" !
	// 	c.Log.Warn().Str("cause", cause).Msg(">>>>> CLOSED <<<<<")
	// 	return nil
	// }

	if ok { // && !c.IsNew()

		if c.Closed != 0 {
			// NOTE: we are done ! Close confirmed by server sent final .message.text "Conversation closed" !
			c.Log.Warn("[ CHAT::CLOSED ]") // CONFIRMED Chat State !
			return nil
		}
		cause := chat.CloseConversationCause_client_leave

		// Mark SENT .CloseConversation(!)
		c.Closed = time.Now().Unix()

		c.Log.Warn("[ CHAT::CLOSE ]", // Start request
			slog.String("cause", cause.String()),
		)
		// PREPARE: request parameters
		close := chat.CloseConversationRequest{
			ConversationId:  c.SessionID,
			CloserChannelId: c.ChannelID,
			AuthUserId:      c.Account.ID,
			Cause:           cause,
		}
		// PERFORM: close request !
		_, err = c.Gateway.Internal.Client.CloseConversation(
			// cancellation // request // callOptions
			context.TODO(), &close, c.callOpts,
		)

		if err != nil {
			// FORCE: destroy runtime link
			bot := c.Gateway
			bot.Lock() // +RW
			e, ok := bot.external[c.ChatID]
			if ok = (ok && e == c); ok {
				delete(bot.external, c.ChatID)
				delete(bot.internal, c.Account.ID) // c.ContactID)
				if len(bot.external) == 0 {
					// if !bot.Enabled {
					// 	// NOTE: Removes srv.gateways[uri] index
					// 	_ = bot.Deregister(context.TODO())
					// }
					if bot.deleted {
						// NOTE: Removes srv.gateways[uri] index
						_ = bot.Deregister(context.TODO())
						// NOTE: Removes srv.profiles[oid] index
						_ = bot.Remove()
						_ = bot.External.Close()
					} else if !bot.Enabled {
						c.Log.Warn("[ CHAT::CLOSE ]", "error", "gateway is disabled")
					}
				}
				// if len(bot.external) == 0 {
				// 	// NOTE: We just destroy the last active channel link
				// 	if !bot.Enabled {
				// 		// DISABLED !
				// 		_ = bot.Remove()
				// 	} // else if next := bot.next; next != nil {
				// }
				// 		// UPGRADED !
				// 		// We have NEW agent revision: close THIS and start NEXT !
				// 		srv := bot.Internal
				// 		srv.indexMx.Lock()   // +RW
				// 		srv.profiles[bot.Id] = next
				// 		srv.indexMx.Unlock() // +RW
				// 		// TODO: Dispose(bot) !
				// 		next.Log.Info().Msg("UPGRADED")
				// 	}
				// }
			}
			bot.Unlock() // -RW

			// TODO: defer c.Send("error: %s", err)
			c.Log.Error("[ CHAT::CLOSE ]", // Start request
				slog.String("cause", cause.String()),
				slog.Any("error", err),
			)
		}

		// var event *zerolog.Event

		// if err == nil {
		// 	event = c.Log.Warn()
		// } else {
		// 	event = c.Log.Error().Err(err)
		// 	// FIXME: force delete from cache ?
		// }

		// event.Str("cause", cause).Msg(">>>>> CLOSING >>>>>")
		// // }
	}

	return err
}

// Start NEW external chat channel
func (c *Channel) Start(ctx context.Context, message *chat.Message) error {

	// title := c.Username
	// if title == "" {
	// 	title = c.Title
	// }

	if message.UpdatedAt != 0 {
		c.Log.Warn("[ CHAT::START ]",
			slog.Any("error", "ignore: start the conversation by editing the message"),
		)
		return nil
	}

	if c.Title == "" {
		c.Title = c.Account.DisplayName()
	}

	providerID := strconv.FormatInt(c.ProfileID(), 10)

	metadata, _ := c.Properties.(map[string]string)
	if metadata == nil {
		metadata = make(map[string]string, 4)
	}
	// External User's (Contact) Internal IDentifier: DB(chat.client.id)
	metadata["cid"] = strconv.FormatInt(c.Account.ID, 10)
	// Chat channel's provider type
	metadata["chat"] = c.Account.Channel
	// External User's (Contact) unique IDentifier; Chat's type- specific !
	metadata["user"] = c.Account.Contact
	// External User's (Contact) Full Name
	metadata["from"] = c.Account.DisplayName()
	// Flow Schema unique IDentifier
	metadata["flow"] = strconv.FormatInt(c.Gateway.Bot.Flow.Id, 10)

	// autobind
	if c.Properties == nil {
		c.Properties = metadata
	}

	start := chat.StartConversationRequest{
		DomainId: c.DomainID(),
		Username: c.Title, // title, // used: as channel title
		User: &chat.User{
			UserId:     c.Account.ID,      // c.ContactID,
			Type:       c.Account.Channel, // c.Provider(),
			Connection: providerID,        // contact: profile.ID
			Internal:   false,
		},
		Message:    message, // start
		Properties: metadata,
	}

	agent := c.Gateway
	span := agent.Log.With(
		"chat", slog.GroupValue(
			slog.String("user", c.Account.Channel+":"+c.ChatID),
			slog.String("title", c.Title),
			slog.Int64("uid", c.Account.ID),
		),
	)

	// PERFORM: /start external chat channel
	if c.Host == "" {
		c.Host = "lookup"
	}
	chat, err := c.Gateway.Internal.Client.
		StartConversation(ctx, &start, c.callOpts)

	if err != nil {
		span.Error("[ CHAT::START ]",
			slog.Any("error", err),
		)
		return err
	}

	c.Closed = 0 // RE- NEW!
	// c.Username  = title
	c.ChannelID = chat.ChannelId
	c.SessionID = chat.ConversationId

	span = span.With(
		"chat", slog.GroupValue(
			slog.String("user", c.Account.Channel+":"+c.ChatID),
			slog.String("title", c.Title),
			slog.Int64("uid", c.Account.ID),
			slog.String("id", c.ChannelID),
			slog.String("thread.id", c.SessionID),
			slog.String("host", c.Host), // webitel.chat.server
		),
	)

	c.Log = span

	// channel := &Channel{

	// 	ChatID:     externalID,
	// 	Title:      username,

	// 	Username:   username,
	// 	ContactID:  contactID,

	// 	ChannelID:  chat.ChannelId,
	// 	SessionID:  chat.ConversationId,

	// 	Gateway:    c,
	// }

	c.Gateway.Lock() // +RW
	c.Gateway.external[c.ChatID] = c
	c.Gateway.internal[c.Account.ID] = c // [c.ContactID] = c
	c.Gateway.Unlock()                   // -RW

	c.Log.Info("[ CHAT::START ]")

	return nil
}

// SendMessage [FROM] .provider [TO] flow-bot@chat.server
func (c *Channel) Recv(ctx context.Context, message *chat.Message) error {

	// region: closing ?
	// const commandClose = "/stop" // internal: from !
	// // // NOTE: sending the last conversation message
	messageText := message.GetText()
	// close := messageText == commandClose
	close := messageText == commandCloseRecvDisposition && message.Type == "text"
	// // if close {
	// // 	// received: /stop command from external
	// // 	// DO: .CloseConversation(!)
	// // 	return c.Close("") // (commandClose)
	// // }
	// // endregion

	c.recvMx.Lock()
	defer c.recvMx.Unlock()

	if c.IsNew() { // || c.Closed != 0 {

		if c.Closed != 0 {
			c.Log.Warn("RESTART")
		}

		if close { // command: /close ?
			c.Log.Warn("IGNORE",
				slog.String("command", commandCloseRecvDisposition),
				slog.String("notice", "channel: chat not running"),
			)
			return nil
		}

		return c.Start(ctx, message)
	}

	if close {
		// command: /close !
		return c.Close()
	}

	// PERFORM resend to internal chat service provider
	res, err := c.Gateway.Internal.Client.SendMessage(
		ctx, // operation cancellation context
		&chat.SendMessageRequest{

			AuthUserId: c.Account.ID, // senderFromID

			ChannelId:      c.ChannelID, // senderChatID
			ConversationId: c.SessionID, // targetChatID

			Message: message, // message
			// EDIT(?) 0 != message.UpdatedAt
		},
		// callOptions ...
		c.callOpts,
	)

	log := c.Log.With(
		slog.String("text", messageText),
	)

	if err == nil {
		// TODO: Remove if clause !
		// For backwards capability only !
		if res.Message != nil {

			*(message) = *(res.Message)
		}
		log.Debug("<<<<< RECV <<<<<")
	} else {
		log.Error("<<<<< RECV <<<<<",
			slog.Any("error", err),
		)
		if st, ok := status.FromError(err); ok && message.File != nil {
			switch st.Code() {
			case codes.FailedPrecondition: // storage file policy violation
				return FileUploadPolicyError
			}
		}
	}

	return err
}

// SendMessage [FROM] .provider [TO] flow-bot@chat.server
func (c *Channel) DeleteMessage(ctx context.Context, message *chat.Message) error {

	// PERFORM resend to internal chat service provider
	msg, err := c.Gateway.Internal.Client.DeleteMessage(
		ctx, // operation cancellation context
		&chat.DeleteMessageRequest{

			AuthUserId: c.Account.ID, // senderFromID

			ChannelId:      c.ChannelID, // senderChatID
			ConversationId: c.SessionID, // targetChatID

			Id:        message.Id, // message
			Variables: message.Variables,
			// EDIT(?) 0 != message.UpdatedAt
		},
		// callOptions ...
		c.callOpts,
	)

	log := c.Log.With(
		slog.Any("deleted", wlog.SlogObject(msg)),
	)

	if err == nil {
		// TODO: Remove if clause !
		// For backwards capability only !
		if msg != nil {
			// message.DeletedAt = time.Now()
		}
		log.Debug("***** DEL *****")
	} else {
		log.Error("***** DEL *****",
			slog.Any("error", err),
		)
	}

	return err
}

// call implements client.CallWrapper to keep tracking channel @workflow service node
func (c *Channel) callWrap(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {
		// Micro-From-Host: Origin
		ctx = metadata.MergeContext(ctx, metadata.Metadata{"Micro-From-Host": server.DefaultServer.Options().Address}, false)
		// PERFORM client.Call(!)
		err := next(ctx, addr, req, rsp, opts)
		//
		if err != nil {
			if c.Host != "" {
				c.Log.Warn("[ CHAT::LOST ]",
					"chat", slog.GroupValue(
						slog.String("lost", c.Host), // WANTED
						slog.String("host", addr),   // REQUESTED
					),
				)
			}
			c.Host = ""
			return err
		}

		if c.Host == "" {
			// NEW! Hosted!
			c.Host = addr

			c.Log.Info("[ CHAT::HOSTED ]",
				slog.String("chat.host", c.Host),
			)

		} else if addr != c.Host {
			// Hosted! But JUST Served elsewhere ...
			c.Log.Info("[ CHAT::REHOST ]",
				"chat", slog.GroupValue(
					slog.String("lost", c.Host), // WANTED
					slog.String("host", addr),   // SERVED
				),
			)

			c.Host = addr
		}

		return err
	}
}

// CallOption specific for this kind of channel(s)
func (c *Channel) callOpts(opts *client.CallOptions) {
	// apply .call options for .this channel ...
	for _, setup := range []client.CallOption{
		client.WithSelector(channelLookup{c}),
		client.WithCallWrapper(c.callWrap),
	} {
		setup(opts)
	}
}

type channelLookup struct {
	*Channel
}

var _ selector.Selector = channelLookup{nil}

// Select a route from the pool using the strategy
func (c channelLookup) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {
	perform := "LOOKUP"
	// region: recover .this channel@workflow service node
	if c.Host == "lookup" {
		c.Host = "" // RESET
	} else if c.Host != "" {

		// c.Log.Debug().
		// 	Str("peer", c.Host).
		// 	Msg("LOOKUP")
	}
	// endregion

	// Lookup: [webitel.chat.server]
	lookup := strategy.RoundRobin.Select
	// lookup := strategy.PrefferedHost("127.0.0.1")

	if c.Host == "" {
		// START
		return lookup(hosts, opts...)
	}

	var peer string
	for _, host := range hosts {
		if host == c.Host {
			peer = host
			break
		}
	}

	if peer == "" {

		c.Log.Warn("[ HOST::LOOKUP ]",
			"conn", wlog.DeferValue(func() slog.Value {
				return slog.GroupValue(
					slog.String("addr", c.Host),   // WANTED
					slog.String("next", "random"), // SELECT
				)
			}),
			"error", "host: service unavailable",
		)

		return lookup(hosts, opts...)
	}

	emit := c.Log.With(
		"conn", slog.GroupValue(
			slog.String("last", c.Host), // WANTED
			slog.String("addr", peer),   // FOUND
		),
	)

	if perform == "RECOVER" { // TODO  is always 'false'
		emit.Info("[ HOST::RECOVER ]")
	} else {
		emit.Debug("[ HOST::LOOKUP ]")
	}

	return func() string {
		return peer
	}, nil
}

// Record the error returned from a route to inform future selection
func (c channelLookup) Record(host string, err error) error {
	if err != nil {

	}
	return nil
}

// Reset the selector
func (c channelLookup) Reset() error {
	return nil
}

// String returns the name of the selector
func (c channelLookup) String() string {
	return "webitel.chat.srv"
}
