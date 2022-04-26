package bot

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"

	chat "github.com/webitel/chat_manager/api/proto/chat"
	// strategy "github.com/webitel/chat_manager/internal/selector"
)

type Channel struct {

	Host string // webitel.chat.server node-id serving .this channel

	Title   string // chat channel title
	ChatID  string // chat.external.id (provider:chat.id) .ExternalID  .AuthUserID
	Account Account // chat.external.user.contact

	ChannelID string  // chat.channel.id  .ID
	SessionID string  // chat.session.id  .ConverationID
	// Contact  *Account // chat.channel.user
	Properties interface{} // driver channel bindings
	
	// Title  string // chat channel title
	// ChatID string // chat.external.id (provider:chat.id) .ExternalID  .AuthUserID
	// Username string // chat channel username
	// ContactID int64   // internal.user.id

	// Closed indicates that .this channel was previously closed at timestamp
	Closed int64

	Log zerolog.Logger
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
	bot.Lock()   // +RW
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
			} else if !bot.Enabled {
				bot.Log.Warn().Msg("DISABLED")
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
			c.Log.Warn().Msg(">>>>> CLOSED <<<<<") // CONFIRMED Chat State !
			return nil
		}

		// Mark SENT .CloseConversation(!)
		c.Closed = time.Now().Unix()
		// // complete command /close with reply text
		// // if cause == "" {
		// // 	// cause = commandCloseRecvDisposiotion // FROM: external, end-user request !
		// // 	// NOTE: default: "Conversation closed"; expected ...
		// // }
		// // cause := "" // ACK: "Conversation closed" expected !
		// if cause == "" {
			cause := commandCloseRecvDisposiotion // FROM: external, end-user request !
		// }
		// // switch cause {
		// // case "": // default: "Conversation closed"
		// // 	// cause = commandCloseSendDisposiotion
		// // case commandCloseRecvDisposiotion:
		// // 	cause = "closed" // 
		// // }

		c.Log.Warn().Str("cause", cause).Msg(">>>>> CLOSING >>>>>")
		// PREPARE: request parameters
		close := chat.CloseConversationRequest{
			ConversationId:  c.SessionID,
			CloserChannelId: c.ChannelID,
			AuthUserId:      c.Account.ID, // c.ContactID,
			Cause:           cause,
		}
		// PERFORM: close request !
		_, err = c.Gateway.Internal.Client.CloseConversation(
			// cancellation // request // callOptions
			context.TODO(), &close, c.sendOptions,
		)

		if err != nil {
			// FORCE: destroy runtime link
			bot := c.Gateway
			bot.Lock()   // +RW
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
					} else if !bot.Enabled {
						bot.Log.Warn().Msg("DISABLED")
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
			c.Log.Error().Err(err).Str("cause", cause).Msg(">>>>> CLOSING >>>>>")
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
		c.Log.Warn().Str("error", "ignore: start the conversation by editing the message").Msg("BOT: START")
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
	// Flow Schema unique IDentifier
	metadata["flow"] = strconv.FormatInt(c.Gateway.Bot.Flow.Id, 10)
	// Chat channel's provider type
	metadata["chat"] = c.Account.Channel
	// External User's (Contact) unique IDentifier; Chat's type- specific !
	metadata["user"] = c.Account.Contact
	// External User's (Contact) Full Name
	metadata["from"] = c.Account.DisplayName()

	start := chat.StartConversationRequest{
		DomainId: c.DomainID(),
		Username: c.Title, // title, // used: as channel title
		User: &chat.User{
			UserId:     c.Account.ID, // c.ContactID,
			Type:       c.Account.Channel, // c.Provider(),
			Connection: providerID, // contact: profile.ID
			Internal:   false,
		},
		Message: message, // start
		Properties: metadata,
	}

	agent := c.Gateway
	span := agent.Log.With().
	
		Str("chat-id", c.ChatID).
		Str("username", c.Title). // title).

		Logger()

	// PERFORM: /start external chat channel
	if c.Host == "" {
		c.Host = "lookup"
	}
	chat, err := c.Gateway.Internal.Client.
		StartConversation(ctx, &start, c.sendOptions)
	
	if err != nil {
		span.Error().Err(err).Msg(">>>>> START <<<<<")
		return err
	}

	c.Closed    = 0 // RE- NEW!
	// c.Username  = title
	c.ChannelID = chat.ChannelId
	c.SessionID = chat.ConversationId

	span = span.With().
	
		Str("session-id", c.SessionID).
		Str("channel-id", c.ChannelID).
		Str("host",       c.Host). // webitel.chat.server

		Logger()

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

	c.Gateway.Lock()   // +RW
	c.Gateway.external[c.ChatID] = c
	c.Gateway.internal[c.Account.ID] = c // [c.ContactID] = c
	c.Gateway.Unlock() // -RW

	c.Log.Info().Msg(">>>>> START <<<<<")

	return nil
}

// SendMessage [FROM] .provider [TO] flow-bot@chat.server
func (c *Channel) Recv(ctx context.Context, message *chat.Message) error {

	// region: closing ?
	// const commandClose = "/stop" // internal: from !
	// // // NOTE: sending the last conversation message
	messageText := message.GetText()
	// close := messageText == commandClose
	close := messageText == commandCloseRecvDisposiotion && message.Type == "text"
	// // if close {
	// // 	// received: /stop command from external
	// // 	// DO: .CloseConversation(!)
	// // 	return c.Close("") // (commandClose)
	// // }
	// // endregion

	if c.IsNew() {

		if close { // command: /close ?
			c.Log.Warn().Str("command", commandCloseRecvDisposiotion).
				Str("notice", "channel: chat not running").Msg("IGNORE")
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

			AuthUserId:     c.Account.ID, // senderFromID

			ChannelId:      c.ChannelID,  // senderChatID
			ConversationId: c.SessionID,  // targetChatID

			Message:        message,      // message
			// EDIT(?) 0 != message.UpdatedAt
		},
		// callOptions ...
		c.sendOptions,
	)

	var event *zerolog.Event

	if err == nil {
		event = c.Log.Debug()
		// TODO: Remove if clause !
		// For backwards capability only !
		if res.Message != nil {

			*(message) = *(res.Message)
		}
	} else {
		event = c.Log.Error().Err(err)
	}

	event.Str("text", messageText).Msg(">>>>> RECV >>>>>>")

	return err
}



// lookup is client.Selector.Strategy to peek preffered @workflow node,
// serving .this specific chat channel
func (c *Channel) peer(services []*registry.Service) selector.Next {

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

	selectNode := selector.Random
	// selectNode = strategy.PrefferedHost("localhost")
	
	if c.Host == "" {
		// START
		return selectNode(services)
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
			Str("peer", c.Host). // WANTED
			Str("peek", "random"). // SELECT
			Str("error", "host: service unavailable").
			Msg(perform)

		return selectNode(services)
	}

	var event *zerolog.Event
	if perform == "RECOVER" {
		event = c.Log.Info()
	} else {
		event = c.Log.Trace()
	}

	event.
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

		// PERFORM client.Call(!)
		err := next(ctx, node, req, rsp, opts)
		// 
		if err != nil {
			if c.Host != "" {
				c.Log.Warn().
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

			c.Log.Info().
				Str("host", c.Host).
				Str("addr", node.Address).
				Msg("HOSTED")
		
		} else if node.Id != c.Host {
			// Hosted! But JUST Served elsewhere ...
			c.Log.Info().
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
		selector.WithStrategy(c.peer),
	)(opts)

	client.WithCallWrapper(c.call)(opts)
}