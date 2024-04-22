package logger

import (
	"fmt"
	"github.com/webitel/chat_manager/app"
	"time"
)

type Message struct {
	//records        []*InputRecord
	Records        []*Record `json:"records,omitempty"`
	RequiredFields `json:"requiredFields"`
}

//type InputRecord struct {
//	Id     int64 `json:"id,omitempty"`
//	Object any   `json:"newState,omitempty"`
//}

type Record struct {
	Id       int64 `json:"id,omitempty"`
	NewState any   `json:"newState,omitempty"`
}

//
//func (r *InputRecord) TransformToOutput() (*Record, error) {
//	bytes, err := json.Marshal(r.Object)
//	if err != nil {
//		return nil, err
//	}
//	return &Record{
//		Id:       r.Id,
//		NewState: bytes,
//	}, nil
//}

func (c *Message) Many(records []*Record) *Message {
	if len(records) == 0 {
		return c
	}
	c.Records = append(c.Records, records...)
	return c
}

func (c *Message) checkRecordsValidity() error {
	if c.Records == nil {
		return fmt.Errorf("logger: no records data in message")
	}
	var canNil bool
	switch c.Action {
	case CREATE_ACTION.String(), UPDATE_ACTION.String():
		canNil = false
	case DELETE_ACTION.String():
		canNil = true
	}
	if !canNil {
		for _, record := range c.Records {
			if record.NewState == nil {
				return fmt.Errorf("logger: record has no data ( id: %d )", record.Id)
			}
		}
	}

	return nil
}

func (c *Message) One(record *Record) *Message {
	if record == nil {
		return c
	}
	c.Records = append(c.Records, record)
	return c
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

type RequiredFields struct {
	UserId     int    `json:"userId,omitempty"`
	UserIp     string `json:"userIp,omitempty"`
	Action     string `json:"action,omitempty"`
	Date       int64  `json:"date,omitempty"`
	DomainId   int64
	ObjectName string
}

func NewCreateMessage(session *app.Context, userIp string, objectName string) *Message {
	user := session.Creds
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     int(user.GetUserId()),
		UserIp:     userIp,
		Action:     string(CREATE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   user.GetDc(),
		ObjectName: objectName,
	}}
	return mess
}

func NewUpdateMessage(session *app.Context, userIp string, objectName string) *Message {
	user := session.Creds
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     int(user.GetUserId()),
		UserIp:     userIp,
		Action:     string(UPDATE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   user.GetDc(),
		ObjectName: objectName,
	}}
	return mess
}

func NewDeleteMessage(session *app.Context, userIp string, objectName string) *Message {
	user := session.Creds
	mess := &Message{RequiredFields: RequiredFields{
		UserId:     int(user.GetUserId()),
		UserIp:     userIp,
		Action:     string(DELETE_ACTION),
		Date:       time.Now().Unix(),
		DomainId:   user.GetDc(),
		ObjectName: objectName,
	}}
	return mess
}
