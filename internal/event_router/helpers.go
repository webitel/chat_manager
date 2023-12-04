package event_router

import (
	"context"
	"fmt"

	// "strings"
	"database/sql"
	"strconv"

	"github.com/pkg/errors"

	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/internal/contact"

	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"

	"github.com/micro/micro/v3/service/broker"
	"github.com/webitel/chat_manager/service/broker/rabbitmq"
	// "github.com/micro/go-micro/v2/client"
	// selector "github.com/micro/go-micro/v2/client/selector"
	// strategy "github.com/webitel/chat_manager/internal/selector"
)

func (c *eventRouter) sendEventToWebitelUser(from *store.Channel, to *store.Channel, event string, data []byte) error {

	err := c.broker.Publish(
		// routing key
		fmt.Sprintf("event.%s.%d.%d",
			event, to.DomainID, to.UserID,
		),
		// delivery
		&broker.Message{
			Header: map[string]string{
				"content_type": "text/json",
			},
			Body: data,
		},
		rabbitmq.ContentType("text/json"),
		rabbitmq.ContentEncoding("charset=utf-8"),
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *eventRouter) sendMessageToBotUser(from *store.Channel, to *store.Channel, message *chat.Message) error {

	switch to.Type {
	case "portal":
		return c.portalSendUpdate(from, to, message)
	}

	// // profile[@svhost]
	// profileID, serviceHost, err := ContactProfileNode(to.Connection.String)

	// if err != nil {
	// 	// invalid <pid[@node]> contact string
	// 	return errors.Wrapf(err, "%s: invalid channel contact %q;", to.Type, to.Connection.String)
	// }

	// var callOpts []client.CallOption

	// if serviceHost != "" {
	// 	callOpts = append(callOpts, client.WithSelectOption(
	// 		selector.WithStrategy(strategy.PrefferedNode(
	// 			"webitel.chat.bot-" + serviceHost,
	// 		)),
	// 	))
	// }

	profileID, err := strconv.ParseInt(to.Connection.String, 10, 64)
	if err != nil {
		return err
	}

	client, err := c.repo.GetClientByID(context.TODO(), to.UserID)
	if err != nil {
		return err
	}
	if client == nil || !client.ExternalID.Valid {
		return fmt.Errorf("client not found. id: %v", to.UserID)
	}

	sendBinding := message.GetVariables()

	sendMessage := gate.SendMessageRequest{
		ProfileId:      profileID,
		ExternalUserId: client.ExternalID.String,
		Message:        message,
	}

	// // if _, err := e.botClient.SendMessage(context.Background(), botMessage, callOpts...); err != nil {
	// if _, err := c.botClient.SendMessage(context.Background(), &sendMessage); err != nil {
	// 	return err
	// }
	// return nil

	recepient := channel{to, c.log}
	requestNode := recepient.Hostname()
	sentMessage, err := c.botClient.SendMessage(

		context.TODO(), &sendMessage,
		// callOptions ...
		recepient.callOpts,
	)

	if err != nil {
		// FIXME: clear running .host ? got an error !
		return err
	}

	sentBinding := sentMessage.GetBindings()
	if sentBinding != nil {
		delete(sentBinding, "")
		if len(sentBinding) != 0 {
			// TODO: merge (!)
			if message.Id == 0 {
				// NOTE: there was a service-level message notification
				//       result bindings applies to target channel, not message !
				if _, err := c.repo.BindChannel(
					context.TODO(), to.ID, sentBinding,
				); err != nil {

					c.log.Error().Err(err).

						// Str("chat-id", target.User.Contact). // client.ExternalID.String).
						Str("channel-id", to.ID). // client.ExternalID.String).

						Msg("FAILED To bind channel properties")
				}

			} else {

				if sendBinding == nil {
					sendBinding = sentBinding
				} else {
					for key, value := range sentBinding {
						if _, ok := sendBinding[key]; ok {
							// FIXME: key(s) must be unique within recepients ? What if not ?
						}
						// reset|override (!)
						sendBinding[key] = value
					}
				}
				// TODO: update chat.message set valiables = :sendBindings where id = :message.Id;
			}
		}
	}

	respondNode := recepient.Hostname()
	if requestNode != respondNode {
		// RE-HOSTED! TODO: update DB channel state .host
		err := c.repo.UpdateChannelHost(context.TODO(), recepient.ID, respondNode)
		if err != nil {
			c.log.Error().Err(err).
				Str("chat-id", client.ExternalID.String).
				Str("channel-id", client.ExternalID.String).
				Msg("RELOCATE")
			// panic(err)
		}
	}

	return nil
}

func (c *eventRouter) SendMessageToGateway(sender, target *app.Channel, message *chat.Message) error {

	switch target.User.Channel {
	case "portal":
		return c.portalSendMessage(sender, target, message)
		// default:
	}

	// profile[@svhost]
	profileID, serviceHost, err := contact.ContactObjectNode(target.Contact)

	if err != nil {
		// invalid <pid[@node]> contact string
		return errors.Wrapf(err, "%s: invalid channel contact %q;", target.Chat.Channel, target.Contact)
	}

	if profileID == 0 {
		return errors.New("send: TO profile <zero> ID")
	}

	// var callOpts []client.CallOption

	// if serviceHost != "" {
	// 	callOpts = append(callOpts, client.WithSelectOption(
	// 		selector.WithStrategy(strategy.PrefferedNode(
	// 			"webitel.chat.bot-" + serviceHost,
	// 		)),
	// 	))
	// }

	// profileID, err := strconv.ParseInt(to.Connection.String, 10, 64)
	// if err != nil {
	// 	return err
	// }

	// client, err := c.repo.GetClientByID(context.TODO(), to.UserID)
	// if err != nil {
	// 	return err
	// }
	// if client == nil || !client.ExternalID.Valid {
	// 	return fmt.Errorf("client not found. id: %v", to.UserID)
	// }

	sendMessage := gate.SendMessageRequest{
		ExternalUserId: target.User.Contact,
		ProfileId:      profileID,
		Message:        message,
	}

	// // if _, err := e.botClient.SendMessage(context.Background(), botMessage, callOpts...); err != nil {
	// if _, err := c.botClient.SendMessage(context.Background(), &sendMessage); err != nil {
	// 	return err
	// }
	// return nil

	recepient := channel{
		trace: c.log,
		// simple transform to store.Channel
		Channel: &store.Channel{
			ID:             target.Chat.ID,
			Type:           target.Chat.Channel,
			ConversationID: target.Chat.Invite,
			UserID:         target.User.ID,
			// Connection: sql.NullString{
			// 	String: strconv.FormatInt(profileID),
			// 	Valid:  true,
			// },
			ServiceHost: sql.NullString{
				String: serviceHost,
				Valid:  serviceHost != "",
			},
			// CreatedAt: time.Time{},
			// Internal:  false,
			// ClosedAt: sql.NullTime{
			// 	Time:  time.Time{},
			// 	Valid: false,
			// },
			// UpdatedAt:  time.Time{},
			DomainID: target.DomainID,
			// FlowBridge: false,
			// Name:       target.User.DisplayName(),
			// ClosedCause: sql.NullString{
			// 	String: "",
			// 	Valid:  false,
			// },
			// JoinedAt: sql.NullTime{
			// 	Time:  time.Time{},
			// 	Valid: false,
			// },
			// Properties: map[string]string{
			// 	"": "",
			// },
		},
	}

	requestNode := recepient.Hostname()
	sendBinding := message.GetVariables() // latest binding(s)

	sentMessage, err := c.botClient.SendMessage(

		context.TODO(), &sendMessage,
		// callOptions ...
		recepient.callOpts,
	)

	if err != nil {
		// FIXME: clear running .host ? got an error !
		return err
	}

	// renewed binding(s) processed this message
	sentBinding := sentMessage.GetBindings()
	if sentBinding != nil {
		delete(sentBinding, "")
		if len(sentBinding) != 0 {
			// TODO: merge (!)
			if message.Id == 0 {
				// NOTE: there was a service-level message notification
				//       result bindings applies to target channel, not message !
				if _, err := c.repo.BindChannel(
					context.TODO(), target.ID, sentBinding,
				); err != nil {

					c.log.Error().Err(err).
						Str("chat-id", target.User.Contact). // client.ExternalID.String).
						Str("channel-id", target.Chat.ID).   // client.ExternalID.String).

						Msg("FAILED To bind channel properties")
				}

			} else {

				if sendBinding == nil {
					sendBinding = sentBinding
				} else {
					for key, newValue := range sentBinding {
						if oldValue, ok := sendBinding[key]; ok && newValue != oldValue {
							// FIXME: key(s) must be unique within recepients ? What if not ?
						}
						// reset|override (!)
						sendBinding[key] = newValue
					}
				}
				// TODO: update chat.message set valiables = :sendBindings where id = :message.Id;
				message.Variables = sendBinding
			}
		}
	}

	respondNode := recepient.Hostname()
	if requestNode != respondNode {
		// RE-HOSTED! TODO: update DB channel state .host
		err := c.repo.UpdateChannelHost(context.TODO(), recepient.ID, respondNode)
		if err != nil {
			c.log.Error().Err(err).
				Str("chat-id", target.User.Contact). // client.ExternalID.String).
				Str("channel-id", target.Chat.ID).   // client.ExternalID.String).

				Msg("RELOCATE")
			// panic(err)
		}
	}

	return nil
}

func (c *eventRouter) SendUserActionToGateway(target *app.Channel, sender *chat.SendUserActionRequest) (bool, error) {

	// profile[@svhost]
	pid, host, err := contact.ContactObjectNode(target.Contact)

	if err != nil {
		// invalid <pid[@node]> contact string
		err = errors.Wrapf(err, "%s: invalid channel contact %q;", target.Chat.Channel, target.Contact)
		return false, err
	}

	if pid == 0 {
		err = errors.New("send: TO profile <zero> ID")
		return false, err
	}

	sendMessageAction := gate.SendUserActionRequest{
		// [FROM] Sender peer.
		ChannelId: sender.GetChannelId(),
		Action:    sender.GetAction(),
		// [TO] Target peer.
		ProfileId:      pid,
		ExternalUserId: target.User.Contact,
	}

	recepient := channel{
		trace: c.log,
		// simple transform to store.Channel
		Channel: &store.Channel{
			ID:             target.Chat.ID,
			Type:           target.Chat.Channel,
			ConversationID: target.Chat.Invite,
			UserID:         target.User.ID,
			// Connection: sql.NullString{
			// 	String: strconv.FormatInt(profileID),
			// 	Valid:  true,
			// },
			ServiceHost: sql.NullString{
				String: host,
				Valid:  host != "",
			},
			// CreatedAt: time.Time{},
			// Internal:  false,
			// ClosedAt: sql.NullTime{
			// 	Time:  time.Time{},
			// 	Valid: false,
			// },
			// UpdatedAt:  time.Time{},
			DomainID: target.DomainID,
			// FlowBridge: false,
			// Name:       target.User.DisplayName(),
			// ClosedCause: sql.NullString{
			// 	String: "",
			// 	Valid:  false,
			// },
			// JoinedAt: sql.NullTime{
			// 	Time:  time.Time{},
			// 	Valid: false,
			// },
			// Properties: map[string]string{
			// 	"": "",
			// },
		},
	}

	requestNode := recepient.Hostname()
	affected, err := c.botClient.SendUserAction(
		context.TODO(), &sendMessageAction,
		// callOptions ...
		recepient.callOpts,
	)

	if err != nil {
		// FIXME: clear running .host ? got an error !
		return false, err
	}

	respondNode := recepient.Hostname()
	if requestNode != respondNode {
		// RE-HOSTED! TODO: update DB channel state .host
		err := c.repo.UpdateChannelHost(context.TODO(), recepient.ID, respondNode)
		if err != nil {
			c.log.Error().Err(err).
				Str("chat-id", target.User.Contact). // client.ExternalID.String).
				Str("channel-id", target.Chat.ID).   // client.ExternalID.String).

				Msg("RELOCATE")
			// panic(err)
		}
	}

	return affected.GetOk(), nil
}
