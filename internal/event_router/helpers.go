package event_router

import (
	// "github.com/micro/go-micro/v2/client"
	// "strings"
	// "github.com/pkg/errors"
	"context"
	"fmt"
	"strconv"

	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"

	"github.com/micro/go-micro/v2/broker"
	// selector "github.com/micro/go-micro/v2/client/selector"
	// strategy "github.com/webitel/chat_manager/internal/selector"
)

func (c *eventRouter) sendEventToWebitelUser(from *store.Channel, to *store.Channel, event string, body []byte) error {
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := c.broker.Publish(fmt.Sprintf("event.%s.%v.%v", event, to.DomainID, to.UserID), msg); err != nil {
		return err
	}
	return nil
}

func (c *eventRouter) sendMessageToBotUser(from *store.Channel, to *store.Channel, message *chat.Message) error {
	
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
	_, err = c.botClient.SendMessage(

		context.TODO(), &sendMessage,
		// callOptions ...
		recepient.sendOptions,
	)

	if err != nil {
		// FIXME: clear running .host ? got an error !
		return err
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
