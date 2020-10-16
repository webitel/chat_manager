package event_router

import (
	"context"
	"fmt"
	"strconv"

	pbbot "github.com/matvoy/chat_server/api/proto/bot"
	pb "github.com/matvoy/chat_server/api/proto/chat"
	pg "github.com/matvoy/chat_server/internal/repo/sqlx"

	"github.com/micro/go-micro/v2/broker"
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
	profileID, err := strconv.ParseInt(to.Connection.String, 10, 64)
	if err != nil {
		return err
	}
	client, err := e.repo.GetClientByID(context.Background(), to.UserID)
	if err != nil {
		return err
	}
	if client == nil || client.ExternalID.Valid == false {
		return fmt.Errorf("client not found. id: %v", to.UserID)
	}

	botMessage := &pbbot.SendMessageRequest{
		ProfileId:      profileID,
		ExternalUserId: client.ExternalID.String,
		Message:        message,
	}
	if _, err := e.botClient.SendMessage(context.Background(), botMessage); err != nil {
		return err
	}
	return nil
}
