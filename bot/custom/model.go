package custom

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Event struct {
	Message *Message `json:"message"`
	Close   *Close   `json:"close"`
}

func (e *Event) Requestify(method string, url string, secret string) (*http.Request, error) {
	var (
		buf  bytes.Buffer
		copy bytes.Buffer
	)
	err := json.NewEncoder(&buf).Encode(e)
	if err != nil {
		return nil, err
	}
	copy.Write(buf.Bytes())
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Webitel-Sign", calculateHash(copy.Bytes(), secret))
	//req.Header.Set("Content-Type", "application/json")
	//req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	return req, nil
}

type File struct {
	Url  string `json:"url"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
	Name string `json:"name"`
}

type Message struct {
	// Message id
	Id string `json:"id"`
	// Chat id
	ChatId string `json:"chatId"`
	// Origin
	Sender *Sender `json:"sender"`
	// Date
	Date int64 `json:"date"`
	// Text of message (not required)
	Text string `json:"text"`
	// File if exists
	File *File `json:"file"`
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
	return nil
}

type Close struct {
	Sender *Sender `json:"sender,omitempty"`
	ChatId string  `json:"chatId"`
	Date   int64   `json:"date,omitempty"`
}

func (c *Close) Normalize() error {
	if c.Sender == nil {
		return errors.New("sender is empty")
	}
	err := c.Sender.Normalize()
	if err != nil {
		return err
	}
	if c.Date == 0 {
		c.Date = time.Now().Unix()
	}
	return nil
}

type Sender struct {
	Id       string            `json:"id"`
	Type     string            `json:"type"`
	Name     string            `json:"name"`
	Nickname string            `json:"nickname"`
	Metadata map[string]string `json:"metadata"`
}

func (s *Sender) Normalize() error {
	if s.Id == "" {
		return errors.New("sender id is empty")
	}
	if s.Name == "" {
		s.Name = "Anonymous"
	}
	return nil
}

type Response struct {
	Success bool   `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}
