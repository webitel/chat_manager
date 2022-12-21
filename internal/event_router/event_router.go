package event_router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/micro/micro/v3/service/broker"
	"github.com/rs/zerolog"

	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/pkg/events"

	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
)

type eventRouter struct {
	botClient gate.BotsService
	// flowClient flow.Client
	broker broker.Broker
	repo   store.Repository
	log    *zerolog.Logger
}

type Router interface {
	RouteCloseConversation(channel *store.Channel, cause string) error
	RouteCloseConversationFromFlow(conversationID *string, cause string) error
	RouteDeclineInvite(userID *int64, conversationID *string) error
	RouteInvite(conversationID *string, userID *int64) error
	RouteJoinConversation(channel *store.Channel, conversationID *string) error
	RouteLeaveConversation(channel *store.Channel, conversationID *string, cause string) error
	RouteMessage(channel *store.Channel, message *chat.Message) (bool, error)
	RouteMessageFromFlow(conversationID *string, message *chat.Message) error
	SendInviteToWebitelUser(conversation *chat.Conversation, invite *store.Invite) error
	SendDeclineInviteToWebitelUser(domainID *int64, conversationID *string, userID *int64, inviteID *string, cause string) error
	SendUpdateChannel(channel *store.Channel, updated_at int64) error
	// Override
	SendMessageToGateway(target *app.Channel, message *chat.Message) error
}

func NewRouter(
	botClient gate.BotsService,
	// flowClient flow.Client,
	broker broker.Broker,
	repo store.Repository,
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
func (e *eventRouter) RouteCloseConversation(channel *store.Channel, cause string) error {
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
				reqMessage := &chat.Message{
					Type: "closed", // "text",
					Text: text,
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
//
//	closed all `webitel` (internal) related chat channels.
//
// NOTE:  that is NOT the truth !  =(
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
		case "webitel":
			{
				body, _ := json.Marshal(events.CloseConversationEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: *conversationID, // channel.ConversationID,
						Timestamp:      time.Now().Unix() * 1000,
					},
					FromChannelID: *conversationID, // channel.ID,
					Cause:         cause,
				})
				err = e.sendEventToWebitelUser(nil, item, events.CloseConversationEventType, body)
				if err != nil {
					e.log.Warn().
						Str("channel_id", item.ID).
						Bool("internal", item.Internal).
						Int64("user_id", item.UserID).
						Str("conversation_id", item.ConversationID).
						Str("type", item.Type).
						Str("connection", item.Connection.String).
						Msg("failed to send close conversation event to 'webitel' channel")
				}
			}
		default: // "telegram", "infobip-whatsapp":

			reqMessage := &chat.Message{
				Type: "closed", // "text",
				Text: text,
			}
			if err := e.sendMessageToBotUser(nil, item, reqMessage); err != nil {
				e.log.Warn().
					Str("channel_id", item.ID).
					Bool("internal", item.Internal).
					Int64("user_id", item.UserID).
					Str("conversation_id", item.ConversationID).
					Str("type", item.Type).
					Str("connection", item.Connection.String).
					Msg("failed to send close conversation event to 'gateway' channel")
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

func (e *eventRouter) SendInviteToWebitelUser(conversation *chat.Conversation, invite *store.Invite) error {

	// const precision = (int64)(time.Millisecond)

	mes := events.UserInvitationEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: conversation.Id,
			Timestamp:      app.DateTimestamp(invite.CreatedAt), // .UnixNano()/precision, // time.Now().Unix() * 1000,
		},
		InviteID:   invite.ID,
		Title:      invite.Title.String,
		TimeoutSec: invite.TimeoutSec,
		Variables:  invite.Variables,
		Conversation: events.Conversation{
			ID:    conversation.Id,
			Title: conversation.Title,
			//DomainID:  conversation.DomainId,
			CreatedAt: conversation.CreatedAt,
			UpdatedAt: conversation.UpdatedAt,
			//ClosedAt:  conversation.ClosedAt,
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
	if size := len(conversation.Members); size != 0 {

		page := make([]events.Member, size)
		list := make([]*events.Member, size)

		for e, src := range conversation.Members {

			dst := &page[e]

			dst.Type = src.Type
			dst.Internal = src.Internal
			dst.ChannelID = src.ChannelId

			dst.UserID = src.UserId
			dst.Username = src.Username
			dst.ExternalId = src.ExternalId
			// dst.Firstname = src.Firstname,
			// dst.Lastname  = src.Lastname,

			dst.UpdatedAt = src.UpdatedAt

			list[e] = dst
		}

		mes.Members = list
	}

	if size := len(conversation.Messages); size != 0 {

		// size = 1

		page := make([]events.Message, size)
		list := make([]*events.Message, 0, size)

		// for e, src := range conversation.Messages {
		for e := size - 1; e >= 0; e-- {

			src := conversation.Messages[e]
			dst := &page[e]

			dst.ID = src.Id
			dst.ChannelID = src.ChannelId
			dst.CreatedAt = src.CreatedAt
			dst.UpdatedAt = src.UpdatedAt
			dst.Type = src.Type
			dst.Text = src.Text

			if doc := src.File; doc != nil {
				dst.File = &events.File{
					ID:   doc.Id,
					URL:  doc.Url,
					Size: doc.Size,
					Type: doc.Mime,
					Name: doc.Name,
				}
			}

			// list[e] = dst
			list = append(list, dst)
			// // NOTE: latest ONE !
			// break
		}

		mes.Messages = list
	}

	data, _ := json.Marshal(mes)
	notify := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: data,
	}

	err := e.broker.Publish(fmt.Sprintf("event.%s.%d.%d",
		events.UserInvitationEventType, invite.DomainID, invite.UserID,
	), notify)

	if err != nil {
		return err
	}

	return nil
}

func (e *eventRouter) SendDeclineInviteToWebitelUser(domainID *int64, conversationID *string, userID *int64, inviteID *string, cause string) error {

	data, err := json.Marshal(
		events.DeclineInvitationEvent{
			BaseEvent: events.BaseEvent{
				ConversationID: *conversationID,
				Timestamp:      time.Now().Unix() * 1000,
			},
			InviteID: *inviteID,
			UserID:   *userID,
			Cause:    cause,
		},
	)

	if err != nil {
		// Encode error !
		return err
	}

	err = e.broker.Publish(fmt.Sprintf("event.%s.%d.%d",
		events.DeclineInvitationEventType, *domainID, *userID,
	), &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: data,
	})

	if err != nil {
		// Publish error !
		return err
	}

	return nil
}

func (e *eventRouter) SendUpdateChannel(channel *store.Channel, updated_at int64) error {

	data, _ := json.Marshal(
		events.UpdateChannelEvent{
			BaseEvent: events.BaseEvent{
				ConversationID: channel.ConversationID,
				Timestamp:      time.Now().Unix() * 1000,
			},
			UpdatedAt: updated_at,
			ChannelID: channel.ID,
		},
	)

	send := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: data,
	}

	err := e.broker.Publish(fmt.Sprintf("event.%s.%d.%d",
		events.UpdateChannelEventType, channel.DomainID, channel.UserID,
	), send)

	if err != nil {
		return err
	}

	return nil
}

func (e *eventRouter) RouteJoinConversation(channel *store.Channel, conversationID *string) error {

	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)
	if err != nil {
		return err
	}
	if otherChannels == nil {
		return nil
	}

	var (
		// encoded JSON message for internal chat@channel notification !
		data []byte
		// prepared *Message for external chat@gateway notification !
		notice *chat.Message
	)

	for _, item := range otherChannels {
		switch item.Type {
		case "webitel":
			// encode message event once !
			if len(data) == 0 {
				event := events.JoinConversationEvent{
					BaseEvent: events.BaseEvent{
						ConversationID: *conversationID,
						Timestamp:      time.Now().Unix() * 1000,
					},
					Member: events.Member{
						ChannelID: channel.ID,
						UserID:    channel.UserID,
						Username:  channel.Name,
						Type:      channel.Type,
						Internal:  channel.Internal,
						UpdatedAt: channel.UpdatedAt.Unix() * 1000,
					},
				}
				data, _ = json.Marshal(event)
			}

			if err := e.sendEventToWebitelUser(nil, item, events.JoinConversationEventType, data); err != nil {
				e.log.Warn().Err(err).
					Str("notify", "new_chat_member").
					Str("channel_id", item.ID).
					Int64("user_id", item.UserID).
					Str("conversation_id", item.ConversationID).
					Str("channel_type", item.Type).
					Msg("FAILED To NOTIFY Channel")
			}
		default: // TO: webitel.chat.bot (gateway)
			// TODO: notify message.new_chat_members
			// prepare message event once !
			if notice == nil {
				notice = &chat.Message{
					Id:   0,        // SERVICE MESSAGE !
					Type: "joined", // "event/joined",
					NewChatMembers: []*chat.Account{
						{
							Id:        channel.UserID,
							Channel:   "user",
							Contact:   "",
							FirstName: channel.Name,
							LastName:  "",
							Username:  "",
						},
					},
				}
			}

			err = e.sendMessageToBotUser(channel, item, notice)

			if err != nil {
				e.log.Warn().Err(err).
					Str("notify", "new_chat_member").
					Str("channel_id", item.ID).
					Int64("user_id", item.UserID).
					Str("conversation_id", item.ConversationID).
					Str("channel_type", item.Type).
					Str("gateway_id", item.Connection.String).
					Msg("FAILED To NOTIFY Gateway")
			}
		}
	}
	return nil
}

func (e *eventRouter) RouteLeaveConversation(channel *store.Channel, conversationID *string, cause string) error {
	// TO: @broker  (engine, callcenter, etc.)
	internalM, _ := json.Marshal(
		events.LeaveConversationEvent{
			BaseEvent: events.BaseEvent{
				ConversationID: *conversationID,
				Timestamp:      time.Now().Unix() * 1000,
			},
			LeavedChannelID: channel.ID,
			Cause:           cause,
		},
	)

	err := e.sendEventToWebitelUser(
		nil, channel, events.LeaveConversationEventType, internalM,
	)

	if err != nil {
		e.log.Warn().Err(err).
			Str("notify", "left_chat_member").
			Str("channel_id", channel.ID).
			Int64("user_id", channel.UserID).
			Str("conversation_id", channel.ConversationID).
			Str("channel_type", channel.Type).
			Msg("FAILED To NOTIFY Channel")
	}
	// Get CHAT related member(s) TO notify ...
	members, err := e.repo.GetChannels(
		context.Background(), nil, conversationID, nil, nil, nil,
	)

	if err != nil {
		return err
	}

	if len(members) == 0 {
		return nil
	}

	var (
		externalM *chat.Message // TO: @gateway (webitel.chat.bot)
	)

	for _, member := range members {
		switch member.Type {
		case "webitel":

			err = e.sendEventToWebitelUser(nil, member,
				events.LeaveConversationEventType, internalM,
			)

			if err != nil {
				e.log.Warn().Err(err).
					Str("notify", "left_chat_member").
					Str("channel_id", member.ID).
					Int64("user_id", member.UserID).
					Str("conversation_id", member.ConversationID).
					Str("channel_type", member.Type).
					Msg("FAILED To NOTIFY Channel")
			}

		default: // TO: webitel.chat.bot (gateway)
			// TODO: notify message.left_chat_member
			// prepare message event once !
			if externalM == nil {
				externalM = &chat.Message{
					Id:   0,      // SERVICE MESSAGE !
					Type: "left", // "event/left_chat_member",
					LeftChatMember: &chat.Account{
						Id:        channel.UserID,
						Channel:   "user",
						Contact:   "",
						FirstName: channel.Name,
						LastName:  "",
						Username:  "",
					},
				}
			}

			err = e.sendMessageToBotUser(channel, member, externalM)

			if err != nil {
				e.log.Warn().Err(err).
					Str("notify", "left_chat_member").
					Str("channel_id", member.ID).
					Bool("internal", member.Internal).
					Int64("user_id", member.UserID).
					Str("conversation_id", member.ConversationID).
					Str("channel_type", member.Type).
					Str("gateway_id", member.Connection.String).
					Msg("FAILED To NOTIFY Gateway")
			}
		}
	}
	return nil
}

func (e *eventRouter) RouteMessage(sender *store.Channel, message *chat.Message) (bool, error) {

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

	var (
		data []byte
		flag = false
	)

	for _, member := range members {

		switch member.Type {
		case "webitel":
			{
				// NOTE: Encode update event data once (!)
				// due to NO target member channel reference is needed ...
				if len(data) == 0 {

					timestamp := message.UpdatedAt
					if timestamp == 0 {
						timestamp = message.CreatedAt
						if timestamp == 0 {
							// time.Now().UnixNano()/(int64)(time.Millisecond) // epochtime: milliseconds
							timestamp = app.DateTimestamp(
								app.CurrentTime(),
							)
						}
					}

					notify := events.MessageEvent{
						BaseEvent: events.BaseEvent{
							ConversationID: sender.ConversationID,
							Timestamp:      timestamp, // time.Now().Unix() * 1000,
						},
						Message: events.Message{
							ChannelID: sender.ID,
							ID:        message.GetId(),
							Type:      message.GetType(),
							Text:      message.GetText(),

							CreatedAt: message.CreatedAt, // time.Now().Unix() * 1000,
							UpdatedAt: message.UpdatedAt, // time.Now().Unix() * 1000,
						},
					}

					if doc := message.File; doc != nil {
						notify.File = &events.File{
							ID:   doc.Id,
							URL:  doc.Url,
							Size: doc.Size,
							Type: doc.Mime,
							Name: doc.Name,
						}
					}

					data, _ = json.Marshal(notify)
				}

				flag = true
				err = e.sendEventToWebitelUser(sender, member, events.MessageEventType, data)
			}

		default: // "telegram", "infobip-whatsapp"

			if sender.ID == member.ID {
				continue
			}

			err = e.sendMessageToBotUser(sender, member, message)

		}
		if err != nil {
			e.log.Warn().Err(err).
				Str("channel_id", member.ID).
				Bool("internal", member.Internal).
				Int64("user_id", member.UserID).
				Str("conversation_id", member.ConversationID).
				Str("type", member.Type).
				Str("connection", member.Connection.String).
				Msg("FAILED Sending message TO channel")
		}
	}

	return flag, nil
}

// conversationID unifies [chat@bot] channel identification
// so, conversationID - unique chat channel sender ID (routine@workflow)
func (e *eventRouter) RouteMessageFromFlow(conversationID *string, message *chat.Message) error {

	otherChannels, err := e.repo.GetChannels(context.Background(), nil, conversationID, nil, nil, nil)

	if err != nil {
		return err
	}

	var (
		// encoded message event
		data []byte
	)

	for _, item := range otherChannels {
		var err error
		switch item.Type {
		case "webitel":
			{
				// e.sendEventToWebitelUser(channel, item, reqMessage)
				// NOTE: Encode update event data once (!)
				// due to NO target item channel reference is needed ...
				if len(data) == 0 {

					timestamp := message.UpdatedAt
					if timestamp == 0 {
						timestamp = message.CreatedAt
						if timestamp == 0 {
							// time.Now().UnixNano()/(int64)(time.Millisecond) // epochtime: milliseconds
							timestamp = app.DateTimestamp(
								app.CurrentTime(),
							)
						}
					}

					notify := events.MessageEvent{
						BaseEvent: events.BaseEvent{
							ConversationID: *conversationID, // sender.ConversationID,
							Timestamp:      timestamp,       // time.Now().Unix() * 1000,
						},
						Message: events.Message{
							ChannelID: *conversationID, // sender.ID,
							ID:        message.GetId(),
							Type:      message.GetType(),
							Text:      message.GetText(),

							CreatedAt: message.CreatedAt, // time.Now().Unix() * 1000,
							UpdatedAt: message.UpdatedAt, // time.Now().Unix() * 1000,
						},
					}

					if doc := message.File; doc != nil {
						notify.File = &events.File{
							ID:   doc.Id,
							URL:  doc.Url,
							Size: doc.Size,
							Type: doc.Mime,
							Name: doc.Name,
						}
					}

					data, _ = json.Marshal(notify)
				}

				err = e.sendEventToWebitelUser(nil, item, events.MessageEventType, data)
			}
		default: // "telegram", "infobip-whatsapp"

			err = e.sendMessageToBotUser(nil, item, message)

		}

		if err != nil {
			e.log.Error().Err(err).
				Str("channel_id", item.ID).
				Bool("internal", item.Internal).
				Int64("user_id", item.UserID).
				Str("conversation_id", item.ConversationID).
				Str("type", item.Type).
				Str("connection", item.Connection.String).
				Msg("FAILED Sending message TO channel")
		}
	}

	return nil
}

/*
func (c *eventRouter) SendMessage(chatRoom *app.Session, notify *chat.Message) (sent int, err error) {
	// FROM
	sender := chatRoom.Channel
	// TO
	if len(chatRoom.Members) == 0 {
		return 0, nil // NO ANY recepient(s) !
	}
	// basic
	notice := events.MessageEvent{
		BaseEvent: events.BaseEvent{
			ConversationID: sender.Invite,
			Timestamp:      notify.CreatedAt, // millis
		},
		Message: events.Message{
			ID:        notify.Id,
			ChannelID: sender.Chat.ID,
			Type:      notify.Type,
			Text:      notify.Text,
			// File:   notify.File,
			CreatedAt: notify.CreatedAt,
			UpdatedAt: notify.UpdatedAt,
		},
	}
	// file
	if doc := notify.File; doc != nil {
		notice.File = &events.File{
			ID:   doc.Id,
			Size: doc.Size,
			Mime: doc.Mime,
			Name: doc.Name,
		}
	}
	// content
	data, _ := json.Marshal(notice)

	// publish
	var (

		head = map[string]string {
			"content_type": "text/json",
		}
	)

	for _, member := range chatRoom.Members {

		if member.IsClosed() {
			continue // omit send TO channel: closed !
		}

		switch member.Channel {
		case "websocket": // TO: engine (internal)
			// s.eventRouter.sendEventToWebitelUser()
			err = c.broker.Publish(fmt.Sprintf("event.%s.%v.%v",
				events.MessageEventType, member.DomainID, member.User.ID,
			), &broker.Message{
				Header: head,
				Body:   data,
			})

		case "chatflow":  // TO: workflow (internal)
			// s.flowClient.SendMessage(sender, sendMessage)


		default:          // TO: webitel.chat.bot (external)
			// s.eventRouter.sendMessageToBotUser()
			err = c.sendMessageToGateway(member, notify)
			// user := member.User
			// // "user": {
			// // 	"id": 59,
			// // 	"channel": "telegram",
			// // 	"contact": "520924760",
			// // 	"firstName": "srgdemon"
			// // },
			// req := gate.SendMessageRequest{
			// 	ProfileId:      14, // profileID,
			// 	ExternalUserId: user.Contact, // client.ExternalID.String,
			// 	Message:        notify,
			// }
		}

		(sent)++ // calc active recepients !

		if err != nil {
			// FIXME: just log failed attempt ?
		}
	}

	return sent, err
}
*/
