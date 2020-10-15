package event_router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pbbot "github.com/matvoy/chat_server/api/proto/bot"
	pb "github.com/matvoy/chat_server/api/proto/chat"
	pg "github.com/matvoy/chat_server/internal/repo/boiler"
	"github.com/matvoy/chat_server/models"
	"github.com/matvoy/chat_server/pkg/events"

	"github.com/micro/go-micro/v2/broker"
	"github.com/rs/zerolog"
)

type eventRouter struct {
	botClient pbbot.BotService
	// flowClient flow.Client
	broker broker.Broker
	repo   pg.Repository
	log    *zerolog.Logger
}

type Router interface {
	RouteCloseConversation(channel *models.Channel, cause string) error
	RouteCloseConversationFromFlow(conversationID *string, cause string) error
	RouteDeclineInvite(userID *int64, conversationID *string) error
	RouteInvite(conversationID *string, userID *int64) error
	RouteJoinConversation(channelID, conversationID *string) error
	RouteLeaveConversation(channelID, conversationID *string) error
	RouteMessage(channel *models.Channel, message *pb.Message) (bool, error)
	RouteMessageFromFlow(conversationID *string, message *pb.Message) error
	SendInviteToWebitelUser(conversation *pb.Conversation, domainID *int64, conversationID *string, userID *int64, inviteID *string) error
	SendDeclineInviteToWebitelUser(domainID *int64, conversationID *string, userID *int64, inviteID *string) error
}

func NewRouter(
	botClient pbbot.BotService,
	// flowClient flow.Client,
	broker broker.Broker,
	repo pg.Repository,
	log *zerolog.Logger,
) Router {
	return &eventRouter{
		botClient,
		// flowClient,
		broker,
		repo,
		log,
	}
}

func (e *eventRouter) RouteCloseConversation(channel *models.Channel, cause string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, &channel.ConversationID, nil, nil, nil) //&channel.ID)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		// if !channel.Internal {
		// 	return e.flowClient.CloseConversation(channel.ConversationID)
		// }
		return nil
	}
	body, _ := json.Marshal(events.CloseConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: channel.ConversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		FromChannelID: channel.ID,
		Cause:         cause,
	})
	for _, item := range otherChannels {
		var err error
		switch item.Type {
		case "webitel":
			{
				err = e.sendEventToWebitelUser(channel, item, events.CloseConversationEventType, body)
			}
		case "telegram", "infobip-whatsapp":
			{
				reqMessage := &pb.Message{
					Type: "text",
					Value: &pb.Message_Text{
						Text: cause,
					},
				}
				err = e.sendMessageToBotUser(channel, item, reqMessage)
			}
		default:
		}
		if err != nil {
			e.log.Warn().
				Str("channel_id", item.ID).
				Bool("internal", item.Internal).
				Int64("user_id", item.UserID).
				Str("conversation_id", item.ConversationID).
				Str("type", item.Type).
				Str("connection", item.Connection.String).
				Msg("failed to send close conversation event to channel")
		}
	}
	return nil
}

func (e *eventRouter) RouteCloseConversationFromFlow(conversationID *string, cause string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	for _, item := range otherChannels {
		switch item.Type {
		case "telegram", "infobip-whatsapp":
			{
				text := "Conversation closed"
				if cause != "" {
					text = cause
				}
				reqMessage := &pb.Message{
					Type: "text",
					Value: &pb.Message_Text{
						Text: text,
					},
				}
				if err := e.sendMessageToBotUser(nil, item, reqMessage); err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msg("failed to send close conversation event to channel")
				}
			}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteDeclineInvite(userID *int64, conversationID *string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}
	body, _ := json.Marshal(events.DeclineInvitationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		UserID: *userID,
	})
	// TO DO declineInvitationToFlow??
	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			{
				if err := e.sendEventToWebitelUser(nil, item, events.DeclineInvitationEventType, body); err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msg("failed to send invite conversation event to channel")
				}
			}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteInvite(conversationID *string, userID *int64) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}
	// if err := e.sendInviteToWebitelUser(&otherChannels[0].DomainID, conversationID, userID); err != nil {
	// 	return err
	// }
	body, _ := json.Marshal(events.InviteConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		UserID: *userID,
	})
	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			{
				if err := e.sendEventToWebitelUser(nil, item, events.InviteConversationEventType, body); err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msg("failed to send invite conversation event to channel")
				}
			}
		default:
		}
	}
	return nil
}

func (e *eventRouter) SendInviteToWebitelUser(conversation *pb.Conversation, domainID *int64, conversationID *string, userID *int64, inviteID *string) error {
	mes := events.UserInvitationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		InviteID: *inviteID,
		Conversation: events.Conversation{
			ID:        conversation.Id,
			DomainID:  conversation.DomainId,
			CreatedAt: conversation.CreatedAt,
			UpdatedAt: conversation.UpdatedAt,
			ClosedAt:  conversation.ClosedAt,
			Title:     conversation.Title,
		},
	}
	// if conversation.CreatedAt != 0 {
	// 	mes.CreatedAt = conversation.CreatedAt.Unix() * 1000
	// }
	// if conversation.ClosedAt != nil {
	// 	mes.ClosedAt = conversation.ClosedAt.Unix() * 1000
	// }
	// if conversation.Title != nil {
	// 	mes.Title = *conversation.Title
	// }
	if memLen := len(conversation.Members); memLen > 0 {
		mes.Members = make([]*events.Member, 0, memLen)
		for _, item := range conversation.Members {
			mes.Members = append(mes.Members, &events.Member{
				ChannelID: item.ChannelId,
				UserID:    item.UserId,
				Username:  item.Username,
				Type:      item.Type,
				Internal:  item.Internal,
				Firstname: item.Firstname,
				Lastname:  item.Lastname,
			})
		}
	}
	body, _ := json.Marshal(mes)
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := e.broker.Publish(fmt.Sprintf("event.%s.%v.%v", events.UserInvitationEventType, *domainID, *userID), msg); err != nil {
		return err
	}
	return nil
}

func (e *eventRouter) SendDeclineInviteToWebitelUser(domainID *int64, conversationID *string, userID *int64, inviteID *string) error {
	body, _ := json.Marshal(events.DeclineInvitationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		InviteID: *inviteID,
		UserID:   *userID,
	})
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := e.broker.Publish(fmt.Sprintf("event.%s.%v.%v", events.DeclineInvitationEventType, *domainID, *userID), msg); err != nil {
		return err
	}
	return nil
}

func (e *eventRouter) RouteJoinConversation(channelID, conversationID *string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil) //channelID)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}
	body, _ := json.Marshal(events.JoinConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		JoinedChannelID: *channelID,
	})
	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			{
				if err := e.sendEventToWebitelUser(nil, item, events.JoinConversationEventType, body); err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msgf("failed to send join conversation event to channel: %s", err.Error())
				}
			}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteLeaveConversation(channelID, conversationID *string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil) //channelID)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}
	body, _ := json.Marshal(events.LeaveConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		LeavedChannelID: *channelID,
	})
	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			{
				if err := e.sendEventToWebitelUser(nil, item, events.JoinConversationEventType, body); err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msg("failed to send leave conversation event to channel")
				}
			}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteMessage(channel *models.Channel, message *pb.Message) (bool, error) {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, &channel.ConversationID, nil, nil, nil) //&channel.ID)
	if err != nil {
		return false, err
	}
	if otherChannels == nil {
		// if !channel.Internal {
		// 	return e.flowClient.SendMessage(channel.ConversationID, reqMessage)
		// }
		return false, nil
	}
	body, _ := json.Marshal(events.MessageEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: channel.ConversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		FromChannelID: channel.ID,
		// ToChannelID:    item.ID,
		MessageID: message.Id,
		Type:      message.Type,
		Value:     message.GetText(),
	})
	flag := false
	for _, item := range otherChannels {
		var err error
		switch item.Type {
		case "webitel":
			{
				flag = true
				err = e.sendEventToWebitelUser(channel, item, events.MessageEventType, body)
			}
		case "telegram", "infobip-whatsapp":
			{
				if channel.ID == item.ID {
					continue
				}
				err = e.sendMessageToBotUser(channel, item, message)
			}
		default:
		}
		if err != nil {
			e.log.Warn().
				Str("channel_id", item.ID).
				Bool("internal", item.Internal).
				Int64("user_id", item.UserID).
				Str("conversation_id", item.ConversationID).
				Str("type", item.Type).
				Str("connection", item.Connection.String).
				Msg("failed to send message to channel")
		}
	}
	return flag, nil
}

func (e *eventRouter) RouteMessageFromFlow(conversationID *string, message *pb.Message) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	for _, item := range otherChannels {
		var err error
		switch item.Type {
		// case "webitel":
		// 	{
		// 		e.sendToWebitelUser(channel, item, reqMessage)
		// 	}
		case "telegram", "infobip-whatsapp":
			{
				err = e.sendMessageToBotUser(nil, item, message)
			}
		default:
		}
		if err != nil {
			e.log.Error().
				Str("channel_id", item.ID).
				Bool("internal", item.Internal).
				Int64("user_id", item.UserID).
				Str("conversation_id", item.ConversationID).
				Str("type", item.Type).
				Str("connection", item.Connection.String).
				Msg(err.Error())
		}
	}
	return nil
}
