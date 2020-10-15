package flow

import (
	"context"
	"errors"

	pb "github.com/matvoy/chat_server/api/proto/chat"
	pbmanager "github.com/matvoy/chat_server/api/proto/flow_manager"
	cache "github.com/matvoy/chat_server/internal/chat_cache"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"
	"github.com/rs/zerolog"
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
	chatCache cache.ChatCache
}

func NewClient(
	log *zerolog.Logger,
	client pbmanager.FlowChatServerService,
	chatCache cache.ChatCache,
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
		return err
	}
	if confirmationID == nil {
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
		ConfirmationId: string(confirmationID),
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
			selector.WithFilter(
				FilterNodes(string(nodeID)),
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
		ConversationId: conversationID,
		ProfileId:      profileID,
		DomainId:       domainID,
		Message: &pbmanager.Message{
			Id:   message.GetId(),
			Type: message.GetType(),
			Value: &pbmanager.Message_Text{
				Text: "start", //req.GetMessage().GetTextMessage().GetText(),
			},
		},
	}
	if res, err := s.client.Start(
		context.Background(),
		start,
		client.WithCallWrapper(
			s.initCallWrapper(conversationID),
		),
	); err != nil ||
		res.Error != nil {
		if res != nil {
			s.log.Error().Msg(res.Error.Message)
		} else {
			s.log.Error().Msg(err.Error())
		}
		return nil
	}
	return nil
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
			selector.WithFilter(
				FilterNodes(string(nodeID)),
			),
		),
	); err != nil {
		return err
	} else if res != nil && res.Error != nil {
		return errors.New(res.Error.Message)
	}
	s.chatCache.DeleteCachedMessages(conversationID)
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
			selector.WithFilter(
				FilterNodes(string(nodeID)),
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
			if err := s.chatCache.WriteConversationNode(conversationID, []byte(node.Id)); err != nil {
				// s.log.Error().Msg(err.Error())
				return err
			}
			return nil
		}
	}
}

func FilterNodes(id string) selector.Filter {
	return func(old []*registry.Service) []*registry.Service {
		var services []*registry.Service

		for _, service := range old {
			if service.Name != "workflow" {
				continue
			}

			serv := new(registry.Service)
			var nodes []*registry.Node

			for _, node := range service.Nodes {
				if node.Id == id {
					nodes = append(nodes, node)
					break
				}
			}

			// only add service if there's some nodes
			if len(nodes) > 0 {
				// copy
				*serv = *service
				serv.Nodes = nodes
				services = append(services, serv)
			}
		}

		return services
	}
}
