package whatsapp

import (
	"strconv"
	"time"
)

type Timestamp time.Time

func (ts *Timestamp) Time() (tm time.Time) {
	if ts != nil {
		tm = (time.Time)(*ts)
	}
	return // tm
}

func (ts *Timestamp) IsZero() bool {
	return ts.Time().IsZero()
}

func (ts *Timestamp) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	tsec, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*(ts) = Timestamp(time.Unix(tsec, 0))
	return nil

	tm, err := time.Parse("2006-01-02T15:04:05-0700", string(data))
	if err != nil {
		return err
	}
	*(ts) = Timestamp(tm)
	return nil
}
