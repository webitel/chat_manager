package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/micro/micro/v3/service/broker"
	proto "github.com/webitel/chat_manager/api/proto/logger"
)

type Client struct {
	rabbit broker.Broker
	grpc   GrpcClient
}

// ! NewClient creates new client for logger.
// * rabbitUrl - connection string to rabbit server
// * consulAddress - address to connect to consul server
func NewClient(rabbit broker.Broker, grpc proto.ConfigService) *Client {
	cli := &Client{grpc: NewGrpcClient(grpc), rabbit: rabbit}
	return cli
}

func (c *Client) Rabbit() broker.Broker {
	return c.rabbit
}

func (c *Client) Grpc() GrpcClient {
	return c.grpc
}

func (c *Client) SendContext(ctx context.Context, message *Message) error {
	//if !c.IsOpened() {
	//	return fmt.Errorf("connection not opened")
	//}
	enabled, err := c.Grpc().Config().CheckIsActive(ctx, message.RequiredFields.DomainId, message.RequiredFields.ObjectName)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	if err := message.checkRecordsValidity(); err != nil {
		return err
	}

	result, err := json.Marshal(message)
	if err != nil {
		return err
	}

	err = c.rabbit.Publish(fmt.Sprintf("logger.%d.%s", message.RequiredFields.DomainId, message.RequiredFields.ObjectName), &broker.Message{Body: result}, broker.PublishContext(ctx))
	if err != nil {
		return err
	}
	return nil
}
