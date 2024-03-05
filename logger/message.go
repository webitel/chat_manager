package logger

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/micro/micro/v3/service/logger"
)

type Message struct {
	records        []*InputRecord
	Records        []*Record `json:"records,omitempty"`
	RequiredFields `json:"requiredFields"`
	client         RabbitClient
}

type InputRecord struct {
	Id     int64
	Object any
}

func (r *InputRecord) TransformToOutput() (*Record, error) {
	bytes, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	return &Record{
		Id:       r.Id,
		NewState: bytes,
	}, nil
}

func (c *Message) Many(records []*InputRecord) *Message {
	if len(records) == 0 {
		return c
	}
	c.records = records
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
			if record.NewState == nil || len(record.NewState) == 0 {
				return fmt.Errorf("logger: record has no data ( id: %s )", record.Id)
			}
		}
	}

	return nil
}

func (c *Message) One(record *InputRecord) *Message {
	if record == nil {
		return c
	}
	c.records = append(c.records, record)
	return c
}

func (c *Message) SendContext(ctx context.Context) {
	//if err := c.checkRecordsValidity(); err != nil {
	//	return err
	//}
	for _, record := range c.records {
		res, err := record.TransformToOutput()
		if err != nil {
			log.Debugf("[LOGGER] object=%s, recordId = %s, error=%s", c.ObjectName, record.Id, err.Error())
		}
		c.Records = append(c.Records, res)
	}
	err := c.client.SendContext(ctx, c)
	if err != nil {
		log.Debugf("[LOGGER] object=%s, error=%s", c.ObjectName, err.Error())
	}
}
