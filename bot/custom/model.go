package custom

import (
	"errors"
	"fmt"
	"time"
)

// region Send types

type SendEvent struct {
	Message   *Message       `json:"message,omitempty"`
	Close     *SendClose     `json:"close,omitempty"`
	Broadcast *SendBroadcast `json:"broadcast,omitempty"`
}

// File represents file in message
type File struct {
	Url  string `json:"url,omitempty"`
	Mime string `json:"mime,omitempty"`
	Size int64  `json:"size,omitempty"`
	Name string `json:"name,omitempty"`
}

// SendBroadcast type for sending broadcast event [TO] the customer webhook
type SendBroadcast struct {
	// Message id
	EventId string `json:"eventId"`
	// Chat id
	Recipients []*Lookup `json:"recipients,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`

	Text string `json:"text,omitempty"`
}

func (c *SendBroadcast) Normalize() error {
	if c.Text == "" {
		return errors.New("empty text message")
	}
	if len(c.Recipients) == 0 {
		return errors.New("empty recipients")
	}
	for i, recipient := range c.Recipients {
		if recipient.Id == "" {
			return errors.New(fmt.Sprintf("recipient [%d] has no id", i))
		}
	}

	return nil
}

func (c *SendClose) Normalize() error {
	if c.ChatId == "" {
		return errors.New("chat id is empty")
	}
	return nil
}

type SendClose struct {
	ChatId string `json:"chatId"`
}

// endregion

//region Receive types

type ReceiveEvent struct {
	Message   *Message          `json:"message,omitempty"`
	Close     *ReceiveClose     `json:"close,omitempty"`
	Broadcast *ReceiveBroadcast `json:"broadcast,omitempty"`
}

// ReceiveBroadcast type for receiving broadcast event [FROM] the customer webhook
type ReceiveBroadcast struct {
	// Message id
	EventId string `json:"eventId"`
	// Chat id
	FailedReceivers []*FailedRecipient `json:"recipients,omitempty"`
}

type FailedRecipient struct {
	Id    string `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Error string `json:"error,omitempty"`
}

func (c *ReceiveClose) Normalize() error {
	if c.ChatId == "" {
		return errors.New("chat id is empty")
	}
	return nil
}

type ReceiveClose struct {
	ChatId string `json:"chatId"`
}

// endregion

// region General

type Lookup struct {
	Id   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

func (c *Message) Normalize() error {
	if c.Sender == nil {
		return errors.New("sender is empty")
	}
	err := c.Sender.Normalize()
	if err != nil {
		return err
	}
	if c.Text == "" && c.File == nil {
		return errors.New("message with no payload")
	}
	if c.Date == 0 {
		c.Date = time.Now().Unix()
	}
	// combine sender type and id to recognize in future
	if c.Sender.Type != "" {
		c.Sender.Id = FormatSenderId(c.Sender.Type, c.Sender.Id)
	}
	return nil
}

func FormatSenderId(senderType string, originSenderId string ) string {
	return fmt.Sprintf("%s|%s", senderType, originSenderId)
}

// Message identifies message FROM/TO customer
type Message struct {
	// Message id
	Id string `json:"id,omitempty"`
	// Chat id
	ChatId string `json:"chatId,omitempty"`
	// Origin
	Sender *Sender `json:"sender,omitempty"`
	// Date
	Date int64 `json:"date,omitempty"`
	// Text of message (not required)
	Text string `json:"text,omitempty"`
	// File if exists
	File *File `json:"file,omitempty"`
	// Variables (set only on first message of new chat)
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Sender struct {
	Id       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

func (s *Sender) Normalize() error {
	if s.Id == "" {
		return errors.New("sender id is empty")
	}
	return nil
}

type Response struct {
	Success bool   `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}

// endregion
