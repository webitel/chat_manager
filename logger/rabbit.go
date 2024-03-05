package logger

import (
	"context"
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	Url string
}

type Action string

func (a Action) String() string {
	return string(a)
}

const (
	CREATE_ACTION Action = "create"
	UPDATE_ACTION Action = "update"
	DELETE_ACTION Action = "delete"
	READ_ACTION   Action = "read"
)

type RabbitClient interface {
	Open() error
	SendContext(ctx context.Context, message *Message) error
	Close()
	IsOpened() bool
}

type Record struct {
	Id       int64  `json:"id,omitempty"`
	NewState []byte `json:"newState,omitempty"`
}

type rabbitClient struct {
	config   *Config
	conn     *amqp.Connection
	channel  *amqp.Channel
	client   *Client
	isOpened bool
}

type RequiredFields struct {
	UserId     int    `json:"userId,omitempty"`
	UserIp     string `json:"userIp,omitempty"`
	Action     string `json:"action,omitempty"`
	Date       int64  `json:"date,omitempty"`
	DomainId   int64
	ObjectName string
	//RecordId int    `json:"recordId,omitempty"`
}

func NewRabbitClient(url string, client *Client) RabbitClient {
	return &rabbitClient{config: &Config{Url: url}, client: client}
}

func (c *rabbitClient) Open() error {
	conn, err := amqp.Dial(c.config.Url)
	if err != nil {
		return err
	}
	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	err = channel.ExchangeDeclare(
		"logger", // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return err
	}
	_, err = channel.QueueDeclare(
		"logger.service",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}
	c.channel = channel
	c.conn = conn
	c.isOpened = true
	return nil
}

func (c *rabbitClient) Close() {
	c.conn.Close()
	c.channel = nil
	c.conn = nil
	c.isOpened = false
}

func (c *rabbitClient) IsOpened() bool {
	return c.isOpened
}

func (c *rabbitClient) SendContext(ctx context.Context, message *Message) error {
	if !c.IsOpened() {
		return fmt.Errorf("connection not opened")
	}
	enabled, err := c.client.Grpc().Config().CheckIsActive(ctx, message.RequiredFields.DomainId, message.RequiredFields.ObjectName)
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

	err = c.channel.PublishWithContext(
		ctx,
		"logger",
		fmt.Sprintf("logger.%d.%s", message.RequiredFields.DomainId, message.RequiredFields.ObjectName),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        result,
		},
	)
	if err != nil {
		return err
	}
	return nil
}
