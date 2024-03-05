package logger

import (
	proto "github.com/webitel/chat_manager/api/proto/logger"
	"time"
)

type Client struct {
	rabbit RabbitClient
	grpc   GrpcClient
}

func (c *Client) IsOpened() bool {
	return c.rabbit.IsOpened()
}

// ! NewClient creates new client for logger.
// * rabbitUrl - connection string to rabbit server
// * consulAddress - address to connect to consul server
func NewClient(brokerConn string, grpcClient proto.ConfigService) *Client {
	cli := &Client{grpc: NewGrpcClient(grpcClient)}
	rab := NewRabbitClient(brokerConn, cli)
	cli.rabbit = rab
	return cli
}

func (c *Client) Open() error {
	err := c.rabbit.Open()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() {
	c.rabbit.Close()
}

func (c *Client) Rabbit() RabbitClient {
	return c.rabbit
}

func (c *Client) Grpc() GrpcClient {
	return c.grpc
}

func (c *Client) CreateAction(domainId int64, objectName string, userId int, userIp string) *Message {
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     userId,
		UserIp:     userIp,
		Action:     string(CREATE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   domainId,
		ObjectName: objectName,
	}, client: c.Rabbit()}
	return mess
}

func (c *Client) UpdateAction(domainId int64, objectName string, userId int, userIp string) *Message {
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     userId,
		UserIp:     userIp,
		Action:     string(UPDATE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   domainId,
		ObjectName: objectName,
	}, client: c.Rabbit()}
	return mess
}

func (c *Client) DeleteAction(domainId int64, objectName string, userId int, userIp string) *Message {
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     userId,
		UserIp:     userIp,
		Action:     string(DELETE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   domainId,
		ObjectName: objectName,
	}, client: c.Rabbit()}
	return mess
}
