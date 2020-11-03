package event_router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/pkg/events"
	pbbot "github.com/webitel/protos/bot"
	pb "github.com/webitel/protos/chat"

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
	RouteCloseConversation(channel *pg.Channel, cause string) error
	RouteCloseConversationFromFlow(conversationID *string, cause string) error
	RouteDeclineInvite(userID *int64, conversationID *string) error
	RouteInvite(conversationID *string, userID *int64) error
	RouteJoinConversation(channel *pg.Channel, conversationID *string) error
	RouteLeaveConversation(channel *pg.Channel, conversationID *string) error
	RouteMessage(channel *pg.Channel, message *pb.Message) (bool, error)
	RouteMessageFromFlow(conversationID *string, message *pb.Message) error
	SendInviteToWebitelUser(conversation *pb.Conversation, invite *pg.Invite) error
	SendDeclineInviteToWebitelUser(domainID *int64, conversationID *string, userID *int64, inviteID *string) error
	SendUpdateChannel(channel *pg.Channel, updated_at int64) error
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

func (e *eventRouter) RouteCloseConversation(channel *pg.Channel, cause string) error {
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
	text := "Conversation closed"
	if cause != "" {
		text = cause
	}
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
						Text: text,
					},
				}
				err = e.sendMessageToBotUser(channel, item, reqMessage)
			}
		//case "corezoid":
		//	{
		//		reqMessage := &pb.Message{
		//			Type: "text",
		//			Value: &pb.Message_Text{
		//				Text: text,
		//			},
		//			Variables: map[string]string{
		//				"operator_name": channel.Name,
		//				"action":        "close",
		//				"channel":       "viber",
		//			},
		//		}
		//		err = e.sendMessageToBotUser(channel, item, reqMessage)
		//	}
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
	text := "Conversation closed"
	if cause != "" {
		text = cause
	}
	for _, item := range otherChannels {
		switch item.Type {
		case "telegram", "infobip-whatsapp":
			{

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
		//case "corezoid":
		//	{
		//		reqMessage := &pb.Message{
		//			Type: "text",
		//			Value: &pb.Message_Text{
		//				Text: text,
		//			},
		//			Variables: map[string]string{
		//				"operator_name": "bot",
		//				"action":        "close",
		//				"channel":       "viber",
		//			},
		//		}
		//		err = e.sendMessageToBotUser(nil, item, reqMessage)
		//	}
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

func (e *eventRouter) SendInviteToWebitelUser(conversation *pb.Conversation, invite *pg.Invite) error {
	mes := events.UserInvitationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: conversation.Id,
			Timestamp:      time.Now().Unix() * 1000,
		},
		InviteID: invite.ID,
		Title:    invite.Title.String,
		Conversation: events.Conversation{
			ID: conversation.Id,
			//DomainID:  conversation.DomainId,
			CreatedAt: conversation.CreatedAt,
			UpdatedAt: conversation.UpdatedAt,
			//ClosedAt:  conversation.ClosedAt,
			Title: conversation.Title,
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
				// ChannelID: item.ChannelId,
				UserID:    item.UserId,
				Username:  item.Username,
				Type:      item.Type,
				Internal:  item.Internal,
				UpdatedAt: item.UpdatedAt,
				// Firstname: item.Firstname,
				// Lastname:  item.Lastname,
			})
		}
	}
	if len(conversation.Messages) > 0 {
		mes.Messages = []*events.Message{
			{
				ID:        conversation.Messages[0].Id,
				ChannelID: conversation.Messages[0].ChannelId,
				Type:      conversation.Messages[0].Type,
				Value:     conversation.Messages[0].Text,
				CreatedAt: conversation.Messages[0].CreatedAt,
				UpdatedAt: conversation.Messages[0].UpdatedAt,
			},
		}
	}
	body, _ := json.Marshal(mes)
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := e.broker.Publish(fmt.Sprintf("event.%s.%v.%v", events.UserInvitationEventType, invite.DomainID, invite.UserID), msg); err != nil {
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

func (e *eventRouter) SendUpdateChannel(channel *pg.Channel, updated_at int64) error {
	body, _ := json.Marshal(events.UpdateChannelEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: channel.ConversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		UpdatedAt: updated_at,
		ChannelID: channel.ID,
	})
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := e.broker.Publish(fmt.Sprintf("event.%s.%v.%v", events.UpdateChannelEventType, channel.DomainID, channel.UserID), msg); err != nil {
		return err
	}
	return nil
}

func (e *eventRouter) RouteJoinConversation(channel *pg.Channel, conversationID *string) error {
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}
	member := events.Member{
		ChannelID: channel.ID,
		UserID:    channel.UserID,
		Username:  channel.Name,
		Type:      channel.Type,
		Internal:  channel.Internal,
		UpdatedAt: channel.UpdatedAt.Unix() * 1000,
	}
	req := events.JoinConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		//JoinedUserID:  channel.UserID,
		Member: member,
		//SelfChannelID: channel.ID,
	}
	//selfBody, _ := json.Marshal(selfEvent)
	//if err := e.sendEventToWebitelUser(nil, channel, events.JoinConversationEventType, selfBody); err != nil {
	//	e.log.Error().
	//		Str("channel_id", channel.ID).
	//		Bool("internal", channel.Internal).
	//		Int64("user_id", channel.UserID).
	//		Str("conversation_id", channel.ConversationID).
	//		Str("type", channel.Type).
	//		Str("connection", channel.Connection.String).
	//		Msgf("failed to send join conversation event to channel: %s", err.Error())
	//	return err
	//}
	// selfEvent.SelfChannelID = ""
	body, _ := json.Marshal(req)
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
		//case "corezoid":
		//	{
		//		reqMessage := &pb.Message{
		//			Type: "text",
		//			Value: &pb.Message_Text{
		//				Text: "Operator joined",
		//			},
		//			Variables: map[string]string{
		//				"operator_name": channel.Name,
		//				"action":        "join",
		//				"channel":       "viber",
		//			},
		//		}
		//		if err := e.sendMessageToBotUser(channel, item, reqMessage); err != nil {
		//			e.log.Warn().
		//				Str("channel_id", item.ID).
		//				Bool("internal", item.Internal).
		//				Int64("user_id", item.UserID).
		//				Str("conversation_id", item.ConversationID).
		//				Str("type", item.Type).
		//				Str("connection", item.Connection.String).
		//				Msgf("failed to send join conversation event to channel: %s", err.Error())
		//		}
		//	}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteLeaveConversation(channel *pg.Channel, conversationID *string) error {
	body, _ := json.Marshal(events.LeaveConversationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: *conversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		LeavedChannelID: channel.ID,
	})
	if err := e.sendEventToWebitelUser(nil, channel, events.LeaveConversationEventType, body); err != nil {
		e.log.Warn().
			Str("channel_id", channel.ID).
			Bool("internal", channel.Internal).
			Int64("user_id", channel.UserID).
			Str("conversation_id", channel.ConversationID).
			Str("type", channel.Type).
			Str("connection", channel.Connection.String).
			Msg("failed to send leave conversation event to channel")
	}
	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil) //channelID)
	if err != nil {
		return err
	}
	if len(otherChannels) == 0 {
		return nil
	}
	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			{
				if err := e.sendEventToWebitelUser(nil, item, events.LeaveConversationEventType, body); err != nil {
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
		//case "corezoid":
		//	{
		//		reqMessage := &pb.Message{
		//			Type: "text",
		//			Value: &pb.Message_Text{
		//				Text: "Operator left",
		//			},
		//			Variables: map[string]string{
		//				"operator_name": channel.Name,
		//				"action":        "leave",
		//				"channel":       "viber",
		//			},
		//		}
		//		err = e.sendMessageToBotUser(channel, item, reqMessage)
		//	}
		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteMessage(channel *pg.Channel, message *pb.Message) (bool, error) {
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
		Message: events.Message{
			ChannelID: channel.ID,
			ID:        message.GetId(),
			Type:      message.GetType(),
			Value:     message.GetText(),
			//CreatedAt: 0,
			//UpdatedAt: 0,
		},
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
		//case "corezoid":
		//	{
		//		reqMessage := *message
		//		reqMessage.Variables = map[string]string{
		//			"operator_name": channel.Name,
		//			"action":        "message",
		//			"channel":       "viber",
		//		}
		//		err = e.sendMessageToBotUser(channel, item, &reqMessage)
		//	}
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
		case "corezoid":
			{
				m, err := e.repo.GetLastMessage(*conversationID)
				variableBytes, err := m.Variables.MarshalJSON()
				variables := make(map[string]string)
				err = json.Unmarshal(variableBytes, &variables)
				variables["text"] = m.Text.String
				if err == nil {
					reqMessage := *message
					reqMessage.Variables = variables
					err = e.sendMessageToBotUser(nil, item, &reqMessage)
				}

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
