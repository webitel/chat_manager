package event_router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/pkg/events"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"

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

// RouteCloseConversation broadcasts the last "Conversation closed"
// message to all related chat channels.
//
// `channel` represents close process initiator.
// `cause` overrides default "Conversation closed" message text
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
		case "webitel": // internal chat-bot leg
			{
				err = e.sendEventToWebitelUser(channel, item, events.CloseConversationEventType, body)
			}
		default: // "telegram", "infobip-whatsapp" ... // external chat-bot leg
			{
				reqMessage := &pb.Message{
					Type: "text",
					Value: &pb.Message_Text{
						Text: text,
					},
				}
				err = e.sendMessageToBotUser(channel, item, reqMessage)
			}
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

// RouteCloseConversationFromFlow same as RouteCloseConversation
// FIXME: except of thing that `flow_manager` service has already
//        closed all `webitel` (internal) related chat channels
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
		default: // "telegram", "infobip-whatsapp":

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
				ChannelID: item.ChannelId,
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
				Text:      conversation.Messages[0].Text,
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

		default:
		}
	}
	return nil
}

func (e *eventRouter) RouteMessage(sender *pg.Channel, message *pb.Message) (bool, error) {
	members, err := e.repo.GetChannels(context.TODO(), nil, &sender.ConversationID, nil, nil, nil) //&channel.ID)
	if err != nil {
		return false, err
	}
	if len(members) == 0 {
		// if !channel.Internal {
		// 	return e.flowClient.SendMessage(channel.ConversationID, reqMessage)
		// }
		return false, nil
	}
	msg :=events.MessageEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: sender.ConversationID,
			Timestamp:      time.Now().Unix() * 1000,
		},
		Message: events.Message{
			ChannelID: sender.ID,
			ID:        message.GetId(),
			Type:      message.GetType(),
			CreatedAt: time.Now().Unix() * 1000,
			UpdatedAt: time.Now().Unix() * 1000,
		},
	}

	switch message.Value.(type){
	 case *pb.Message_Text:
		msg.Text = message.GetText()

	case *pb.Message_File_:
		msg.File = &events.File{
			ID:     message.GetFile().Id,
			Mime :  message.GetFile().Mime,
			Name :  message.GetFile().Name,
			Size :  message.GetFile().Size,
		}

	default:

	}

	body, _ := json.Marshal(msg)

	flag := false
	for _, member := range members {
		var err error
		switch member.Type {
		case "webitel":
			{
				flag = true
				err = e.sendEventToWebitelUser(sender, member, events.MessageEventType, body)
			}
		default: // "telegram", "infobip-whatsapp"

			if sender.ID == member.ID {
				continue
			}
			err = e.sendMessageToBotUser(sender, member, message)

		}
		if err != nil {
			e.log.Warn().
				Str("channel_id", member.ID).
				Bool("internal", member.Internal).
				Int64("user_id", member.UserID).
				Str("conversation_id", member.ConversationID).
				Str("type", member.Type).
				Str("connection", member.Connection.String).
				Msg("failed to send message to channel")
		}
	}
	return flag, nil
}

// conversationID unifies [chat@bot] channel identification
// so, conversationID - unique chat channel sender ID (routine@workflow)
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
		default: // "telegram", "infobip-whatsapp"

			err = e.sendMessageToBotUser(nil, item, message)

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
