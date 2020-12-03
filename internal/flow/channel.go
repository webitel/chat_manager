package flow

import (

	"errors"
	"context"
	// "github.com/micro/go-micro/v2/errors"

	"github.com/rs/zerolog"

	strategy "github.com/webitel/chat_manager/internal/selector"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	
	chat "github.com/webitel/chat_manager/api/proto/chat"
	bot "github.com/webitel/chat_manager/api/proto/workflow"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"
	
)


// Channel [FROM] chat.srv [TO] workflow
// CHAT communication channel
type Channel struct {
	// Client
	Log   *zerolog.Logger

	Host string // preffered: "workflow" service-node-id
	Agent bot.FlowChatServerService
	Store sqlxrepo.CacheRepository
	// Session
	
	
	// Chat *sqlxrepo.Channel // Conversation
	ID string
	User *chat.User // TODO: flow schema @bot info
	
	DomainID int64
	ProfileID int64 // Disclose profile.schema.id
	
	Invite string // SESSION ID
	Pending string // .WaitMessage(token)

	Created int64
	Updated int64
	Started int64
	Joined int64
	Closed int64

	// // LATEST
	// Update *chat.Message

}

func NewChannel(

	log *zerolog.Logger,
	store sqlxrepo.CacheRepository,
	agent bot.FlowChatServerService,

) *Channel {

	return &Channel{
		Log:   log,
		Store: store,
		Agent: agent,
	}
}

func (c *Channel) peer() selector.SelectOption {
	
	if c.Host == "" && c.ID != "" {
			
		node, err := c.Store.ReadConversationNode(c.ID)
	
		if err != nil {
			
			c.Log.Error().Err(err).
			Str("chat-id", c.ID).
			Str("channel", "chatflow").
			Msg("Looking for channel host")
			
			c.Host = ""
		
		} else {

			c.Host = node
		}
	}

	// if c.Host != "" {
		return selector.WithStrategy(
			strategy.PrefferedNode(c.Host),
		)
	// }
	// return selector.WithStrategy(selector.Random)
}

func (c *Channel) call(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
		
		// doRequest
		err := next(ctx, node, req, rsp, opts)
		// 
		if err != nil {
			if c.Host != "" {
				c.Log.Debug().
					Str("host", node.Id).
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

			c.Log.Debug().
				Str("host", c.Host).
				Msg("HOSTED")
		
		} else if node.Id != c.Host {
			// Hosted! But JUST Served elsewhere ...
			re := c.Store.WriteConversationNode(c.ID, c.Host)
			if err = re; err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}

			c.Log.Debug().
				Str("peer", c.Host).
				Str("host", node.Id).
				Msg("RE-HOST")

			c.Host = node.Id
		}

		return err
	}
}

func (c *Channel) Send(message *chat.Message) (err error) {
	
	pending := c.Pending // token
	if pending == "" {
		
		pending, err = c.Store.ReadConfirmation(c.ID)
		
		if err != nil {
			c.Log.Error().Err(err).Str("chat-id", c.ID).Msg("Failed to get {chat.recvMessage.token} from store")
			return err
		}
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
	messageReq := &bot.ConfirmationMessageRequest{
		ConversationId: c.ID,
		ConfirmationId: pending,
		Messages:       messages,
	}

	/*if res, err := c.Agent.ConfirmationMessage(
		context.TODO(), messageReq,
		client.WithCallWrapper(c.call),
		client.WithSelectOption(c.peer()),
	); err != nil || res.Error != nil {
		if res != nil {
			return errors.New(res.Error.Message)
		}
		return err
	}
	// s.chatCache.DeleteConfirmation(conversationID)
	c.Pending = ""
	c.Store.DeleteConfirmation(c.ID)
	return nil*/

	re, err := c.Agent.
	ConfirmationMessage(
		context.TODO(), messageReq,
		client.WithCallWrapper(c.call),
		client.WithSelectOption(c.peer()),
	)
		
	if err != nil {
		return err
	}
	
	if re.Error != nil {

		c.Log.Error().
			Str("error", re.Error.Message).
			Str("channel-id", c.ID).
			Msg("Failed to delivery chat@bot messages")

		switch re.Error.Id {
		case "grpc.chat.conversation.not_found": // Conversation xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx not found
			// // FIXME: RE-start chat@bot routine (?)
			// // RESET
			// c.Host = ""
			// c.Pending = ""
			// TODO: ensure this.(ID|ProfileID|DomainID)
			// err = c.Start(message)
			
			// if err != nil {
			// 	c.Log.Error().Err(err).Str("channel-id", c.ID).Msg("RE-START Failed")
			// } else {
			// 	c.Log.Info().Str("channel-id", c.ID).Msg("RE-START!")
			// }

			// return err

		}
		// default:
		return errors.New(re.Error.Message)
	}
	// s.chatCache.DeleteConfirmation(conversationID)
	c.Pending = ""
	c.Store.DeleteConfirmation(c.ID)
	return nil

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
}

/*func (c *Channel) SendMessage(conversationID string, message *chat.Message) (err error) {
	
	pending := c.Pending // token
	if pending == "" {
		
		pending, err = c.Store.ReadConfirmation(conversationID)
		
		if err != nil {
			c.Log.Error().Err(err).Str("chat-id", conversationID).Msg("Failed to get {chat.recvMessage.token} from store")
			return err
		}
	}
	// Flow.WaitMessage() 
	if pending == "" {
		// FIXME: NO confirmation found for chat - means that we are not in {waitMessage} block ?
		c.Log.Warn().Str("chat-id", conversationID).Msg("CHAT Flow is NOT waiting for text message(s); DO NOTHING MORE!")
		return nil
	}
	
	c.Log.Debug().
		Str("conversation_id", conversationID).
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
	messageReq := &bot.ConfirmationMessageRequest{
		ConversationId: conversationID,
		ConfirmationId: pending,
		Messages:       messages,
	}

	if res, err := c.Agent.ConfirmationMessage(
		context.TODO(), messageReq,
		client.WithCallWrapper(c.call),
		client.WithSelectOption(c.peer()),
	); err != nil || res.Error != nil {
		if res != nil {
			return errors.New(res.Error.Message)
		}
		return err
	}
	// s.chatCache.DeleteConfirmation(conversationID)
	c.Pending = ""
	c.Store.DeleteConfirmation(conversationID)
	return nil

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

func (c *Channel) Start(message *chat.Message) error {
	
	c.Log.Debug().
		Str("conversation_id", c.ID).
		Int64("profile_id", c.ProfileID).
		Int64("domain_id", c.DomainID).
		Msg("init conversation")
	
	const commandStart = "start"
	messageText := commandStart
	start := &bot.StartRequest{
		
		DomainId:       c.DomainID,
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId:      c.ProfileID,
		ConversationId: c.ID,

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

			messageText = "File"
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
	c.Host = "" // NEW: selector.Random
	
	res, err := c.Agent.Start(
		// channel context
		context.Background(), start,
		// callOptions
		client.WithCallWrapper(c.call),
		// client.WithSelectOption(selector.Random),
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
	Int64("pdc", c.DomainID).
	Int64("pid", c.ProfileID).
	Str("host", c.Host).
	Str("channel-id", c.ID).
	Msg(">>>>> START <<<<<")

	return nil

	/* ; err != nil || res.Error != nil { // WTF: (0_o) (?)
		if err == nil && res.Error != nil {
			err = 
		}
		
		if res != nil { // GUESS: it will never be empty !
			s.log.Error().Msg(res.Error.Message)
		} else {
			s.log.Error().Err(err).Msg("Failed to start chat-flow routine")
		}
		return nil
	}
	return nil
	*/
}

// Init chat => flow chat-channel communication state
/*func (c *Channel) Init(conversationID string, profileID, domainID int64, message *chat.Message) error {
	
	c.Log.Debug().
		Str("conversation_id", conversationID).
		Int64("profile_id", profileID).
		Int64("domain_id", domainID).
		Msg("init conversation")
	
	start := &bot.StartRequest{
		
		DomainId:       domainID,
		// FIXME: why flow_manager need to know about some external chat-bot profile identity ?
		ProfileId:      profileID,
		ConversationId: conversationID,

		Message: &bot.Message{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &bot.Message_Text{
				Text: "start", //req.GetMessage().GetTextMessage().GetText(),
			},
		},

		Variables: message.GetVariables(),
	}

	if message != nil {
		
		switch e := message.GetValue().(type) {
		case *chat.Message_Text: // TEXT
			
			messageText := e.Text
			if messageText == "" {
				messageText = "/start" // default!
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
	}

	// Request to start flow-routine for NEW-chat incoming message !
	c.Host = "" // NEW: balacing
	res, err := c.Agent.Start(
		context.Background(), start,
		client.WithCallWrapper(
			// SELECT: use default
			c.start(conversationID),
		),
	)
	
	if err != nil {
		
		c.Log.Error().Err(err).
			Msg("Failed to start chat-flow routine")
		
		return err

	} else if re := res.GetError(); re != nil {

		c.Log.Error().
			Str("errno", re.GetId()).
			Str("error", re.GetMessage()).
			Msg("Failed to start chat-flow routine")

		// return errors.New(
		// 	re.GetId(),
		// 	re.GetMessage(),
		// 	502, // 502 Bad Gateway
		// 	// The server, while acting as a gateway or proxy,
		// 	// received an invalid response from the upstream server it accessed
		// 	// in attempting to fulfill the request.
		// )
	}

	return nil

	// ; err != nil || res.Error != nil { // WTF: (0_o) (?)
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
	//
}*/

func (c *Channel) Close() error {
	// nodeID, err := c.Store.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	if res, err := c.Agent.Break(
		context.TODO(), &bot.BreakRequest{
			ConversationId: c.ID,
		},
		client.WithSelectOption(c.peer()),
		client.WithCallWrapper(c.call),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}

	c.Log.Warn().
	Int64("pdc", c.DomainID).
	Int64("pid", c.ProfileID).
	Str("host", c.Host).
	Str("channel-id", c.ID).
	Msg("<<<<< CLOSE >>>>>")
	// //s.chatCache.DeleteCachedMessages(conversationID)
	// s.chatCache.DeleteConfirmation(conversationID)
	// s.chatCache.DeleteConversationNode(conversationID)
	return nil
}

/*func (c *Channel) CloseConversation(conversationID string) error {
	// nodeID, err := c.Store.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	if res, err := c.Agent.Break(
		context.TODO(), &bot.BreakRequest{
			ConversationId: conversationID,
		},
		client.WithSelectOption(c.peer()),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}
	// //s.chatCache.DeleteCachedMessages(conversationID)
	// s.chatCache.DeleteConfirmation(conversationID)
	// s.chatCache.DeleteConversationNode(conversationID)
	return nil
}*/

func (c *Channel) BreakBridge(cause BreakBridgeCause) error {
	// nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	if res, err := c.Agent.BreakBridge(
		context.TODO(),
		&bot.BreakBridgeRequest{
			ConversationId: c.ID,
			Cause:          cause.String(),
		},
		client.WithSelectOption(c.peer()),
		client.WithCallWrapper(c.call),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}

	c.Log.Warn().
	Int64("pdc", c.DomainID).
	Int64("pid", c.ProfileID).
	Str("host", c.Host).
	Str("channel-id", c.ID).
	Msg("<<<<< BREAK >>>>>")
	return nil
}

/*func (c *Channel) BreakBridge(conversationID string, cause BreakBridgeCause) error {
	// nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	// if err != nil {
	// 	return err
	// }
	if res, err := c.Agent.BreakBridge(
		context.Background(),
		&bot.BreakBridgeRequest{
			ConversationId: conversationID,
			Cause:          cause.String(),
		},
		client.WithSelectOption(c.peer()),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}
	return nil
}*/

/*func (c *Channel) start(conversationID string) func(client.CallFunc) client.CallFunc {
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
			
			c.Log.Trace().
				Str("id", node.Id).
				Str("address", node.Address).
				Msg("send request to node")
			
			err := next(ctx, node, req, rsp, opts)
			if err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}
			c.Host = node.Id
			if err := c.Store.WriteConversationNode(conversationID, node.Id); err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}
			return nil
		}
	}
}*/
