package flow

import (

	"errors"
	"context"
	// "github.com/micro/go-micro/v2/errors"

	"github.com/rs/zerolog"

	strategy "github.com/webitel/chat_manager/internal/selector"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	pb "github.com/webitel/protos/chat"
	pbmanager "github.com/webitel/protos/workflow"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"
	
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
	SendMessage(conversationID string, message *pb.Message) error
	Init(conversationID string, profileID, domainID int64, message *pb.Message) error
	BreakBridge(conversationID string, cause BreakBridgeCause) error
	CloseConversation(conversationID string) error
}

type flowClient struct {
	log       *zerolog.Logger
	client    pbmanager.FlowChatServerService
	chatCache sqlxrepo.CacheRepository
}

func NewClient(
	log *zerolog.Logger,
	client pbmanager.FlowChatServerService,
	chatCache sqlxrepo.CacheRepository,
) *flowClient {
	return &flowClient{
		log,
		client,
		chatCache,
	}
}

func (s *flowClient) SendMessage(conversationID string, message *pb.Message) error {
	confirmationID, err := s.chatCache.ReadConfirmation(conversationID)
	if err != nil {
		s.log.Error().Err(err).Str("chat-id", conversationID).Msg("Failed to get {chat.recvMessage.token} from store")
		return err
	}
	if confirmationID == "" {
		// FIXME: NO confirmation found for chat - means that we are not in {waitMessage} block ?
		s.log.Warn().Str("chat-id", conversationID).Msg("CHAT Flow is NOT waiting for text message(s); DO NOTHING MORE!")
		return nil
	}
	s.log.Debug().
		Str("conversation_id", conversationID).
		Str("confirmation_id", string(confirmationID)).
		Msg("send confirmed messages")
	messages := []*pbmanager.Message{
		{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &pbmanager.Message_Text{
				Text: message.GetText(),
			},
		},
	}
	messageReq := &pbmanager.ConfirmationMessageRequest{
		ConversationId: conversationID,
		ConfirmationId: confirmationID,
		Messages:       messages,
	}
	nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	if err != nil {
		return err
	}
	if res, err := s.client.ConfirmationMessage(
		context.Background(),
		messageReq,
		client.WithSelectOption(
			selector.WithStrategy(
				strategy.PrefferedNode(nodeID),
			),
		),
	); err != nil || res.Error != nil {
		if res != nil {
			return errors.New(res.Error.Message)
		}
		return err
	}
	s.chatCache.DeleteConfirmation(conversationID)
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

func (s *flowClient) Init(conversationID string, profileID, domainID int64, message *pb.Message) error {
	
	s.log.Debug().
		Str("conversation_id", conversationID).
		Int64("profile_id", profileID).
		Int64("domain_id", domainID).
		Msg("init conversation")
	
	start := &pbmanager.StartRequest{
		
		DomainId:       domainID,
		ProfileId:      profileID,
		ConversationId: conversationID,

		Message: &pbmanager.Message{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &pbmanager.Message_Text{
				Text: "start", //req.GetMessage().GetTextMessage().GetText(),
			},
		},

		Variables: message.GetVariables(),
	}

	if message != nil {
		
		switch v := message.GetValue().(type) {
		case *pb.Message_Text: // TEXT
			
			messageText := v.Text
			if messageText == "" {
				messageText = "start" // default!
			}

			start.Message.Value =
				&pbmanager.Message_Text{
					Text: messageText,
				}

		case *pb.Message_File_: // FILE

			start.Message.Value =
				&pbmanager.Message_File_{
					File: &pbmanager.Message_File{
						Id:       v.File.GetId(),
						Url:      v.File.GetUrl(),
						MimeType: v.File.GetMimeType(),
					},
				}
		}
	}

	// Request to start flow-routine for NEW-chat incoming message !
	res, err := s.client.Start(
		context.Background(), start,
		client.WithCallWrapper(
			s.initCallWrapper(conversationID),
		),
	)
	
	if err != nil {
		
		s.log.Error().Err(err).
			Msg("Failed to start chat-flow routine")
		
		return err

	} else if re := res.GetError(); re != nil {

		s.log.Error().
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

func (s *flowClient) CloseConversation(conversationID string) error {
	nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	if err != nil {
		return err
	}
	if res, err := s.client.Break(
		context.Background(),
		&pbmanager.BreakRequest{
			ConversationId: conversationID,
		},
		client.WithSelectOption(
			selector.WithStrategy(
				strategy.PrefferedNode(nodeID),
			),
		),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}
	//s.chatCache.DeleteCachedMessages(conversationID)
	s.chatCache.DeleteConfirmation(conversationID)
	s.chatCache.DeleteConversationNode(conversationID)
	return nil
}

func (s *flowClient) BreakBridge(conversationID string, cause BreakBridgeCause) error {
	nodeID, err := s.chatCache.ReadConversationNode(conversationID)
	if err != nil {
		return err
	}
	if res, err := s.client.BreakBridge(
		context.Background(),
		&pbmanager.BreakBridgeRequest{
			ConversationId: conversationID,
			Cause:          cause.String(),
		},
		client.WithSelectOption(
			selector.WithStrategy(
				strategy.PrefferedNode(nodeID),
			),
		),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}
	return nil
}

func (s *flowClient) initCallWrapper(conversationID string) func(client.CallFunc) client.CallFunc {
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
}
