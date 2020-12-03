package event_router

import (
	// "github.com/micro/go-micro/v2/client"
	// "strings"
	// "github.com/pkg/errors"
	"context"
	"fmt"
	"strconv"

	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"

	"github.com/micro/go-micro/v2/broker"
	// selector "github.com/micro/go-micro/v2/client/selector"
	// strategy "github.com/webitel/chat_manager/internal/selector"
)

func (e *eventRouter) sendEventToWebitelUser(from *pg.Channel, to *pg.Channel, eventType string, body []byte) error {
	msg := &broker.Message{
		Header: map[string]string{
			"content_type": "text/json",
		},
		Body: body,
	}
	if err := e.broker.Publish(fmt.Sprintf("event.%s.%v.%v", eventType, to.DomainID, to.UserID), msg); err != nil {
		return err
	}
	return nil
}

func (e *eventRouter) sendMessageToBotUser(from *pg.Channel, to *pg.Channel, message *pb.Message) error {
	
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

	client, err := e.repo.GetClientByID(context.Background(), to.UserID)
	if err != nil {
		return err
	}
	if client == nil || !client.ExternalID.Valid {
		return fmt.Errorf("client not found. id: %v", to.UserID)
	}

	botMessage := &pbbot.SendMessageRequest{
		ProfileId:      profileID,
		ExternalUserId: client.ExternalID.String,
		Message:        message,
	}
	// if _, err := e.botClient.SendMessage(context.Background(), botMessage, callOpts...); err != nil {
	if _, err := e.botClient.SendMessage(context.Background(), botMessage); err != nil {
		return err
	}
	return nil
}
