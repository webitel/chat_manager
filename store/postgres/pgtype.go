package postgres

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/webitel/chat_manager/app"
)

type (
	TextDecoder = pgtype.TextDecoder
	DecodeText  func(src []byte) error
)

var (
	_ TextDecoder = DecodeText(nil)
)

func (fn DecodeText) Scan(src any) error {
	if src == nil {
		return fn.DecodeText(nil, nil)
	}

	switch data := src.(type) {
	case string:
		return fn.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return fn.DecodeText(nil, text)
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, fn)
}

func (fn DecodeText) DecodeText(_ *pgtype.ConnInfo, src []byte) error {
	if fn != nil {
		return fn(src)
	}
	// IGNORE
	return nil
}

// pgtype:int8 ~ golang:int64
type BoolValue struct {
	Value *bool
}

func (e BoolValue) Scan(src any) error {
	if src == nil {
		*(e.Value) = false // NULL
		return nil
	}

	switch data := src.(type) {
	case bool:
		*(e.Value) = data
		return nil
	case string:
		return e.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return e.DecodeText(nil, text)
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, e.Value)
}

func (e BoolValue) DecodeText(_ *pgtype.ConnInfo, text []byte) error {

	if text == nil {
		*(e.Value) = false // NULL
		return nil
	}

	if len(text) != 1 {
		return fmt.Errorf("invalid length for bool: %v", len(text))
	}

	*(e.Value) = (text[0] == 't')
	return nil
}

// pgtype:int8 ~ golang:int64
type Int8 struct {
	Value *int64
}

func (e Int8) Scan(src any) error {
	if src == nil {
		*(e.Value) = 0 // NULL
		return nil
	}

	switch data := src.(type) {
	case int64:
		*(e.Value) = data
		return nil
	case string:
		return e.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return e.DecodeText(nil, text)
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, e.Value)
}

func (e Int8) DecodeText(_ *pgtype.ConnInfo, text []byte) error {

	if len(text) == 0 {
		*(e.Value) = 0 // NULL
		return nil
	}

	n, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}

	*(e.Value) = n
	return nil
}

// pgtype:int4 ~ golang:int32
type Int4 struct {
	Value *int32
}

func (e Int4) Scan(src any) error {
	if src == nil {
		*(e.Value) = 0 // NULL
		return nil
	}

	switch data := src.(type) {
	case int64:
		if data < math.MinInt32 {
			return fmt.Errorf("pgstore: %d is less than the minimum value for Int4", data)
		}
		if data > math.MaxInt32 {
			return fmt.Errorf("pgstore: %d is greater than maximum value for Int4", data)
		}
		*(e.Value) = int32(data)
		return nil
	case string:
		return e.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return e.DecodeText(nil, text)
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, e.Value)
}

func (e Int4) DecodeText(_ *pgtype.ConnInfo, text []byte) error {

	if len(text) == 0 {
		*(e.Value) = 0 // NULL
		return nil
	}

	n, err := strconv.ParseInt(string(text), 10, 32)
	if err != nil {
		return err
	}

	*(e.Value) = int32(n)
	return nil
}

// pgtype:text ~ golang:string
type Text struct {
	Value *string
}

func (e Text) Scan(src any) error {
	if src == nil {
		*(e.Value) = "" // NULL
		return nil
	}

	switch data := src.(type) {
	case string:
		return e.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return e.DecodeText(nil, text)
	case int64:
		*(e.Value) = strconv.FormatInt(data, 10)
		return nil
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, e.Value)
}

func (e Text) DecodeText(_ *pgtype.ConnInfo, text []byte) error {

	if len(text) == 0 {
		*(e.Value) = "" // NULL
		return nil
	}

	*(e.Value) = string(text)
	return nil
}

const (
	pgTimestampFormat         = "2006-01-02 15:04:05.999999999"
	pgTimestamptzHourFormat   = "2006-01-02 15:04:05.999999999Z07"
	pgTimestamptzMinuteFormat = "2006-01-02 15:04:05.999999999Z07:00"
	pgTimestamptzSecondFormat = "2006-01-02 15:04:05.999999999Z07:00:00"
)

// pgtype:timestamp ~ golang:int64
type Epochtime struct {
	Value     *int64 // *time.Time
	Precision time.Duration
}

func (e Epochtime) precision() time.Duration {
	precision := e.Precision
	if precision > 0 {
		return precision
	}
	// default
	return app.TimePrecision
}

func (e Epochtime) Scan(src any) error {
	if src == nil {
		*(e.Value) = 0 // NULL
		return nil
	}

	switch data := src.(type) {
	case string:
		return e.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return e.DecodeText(nil, text)
	case time.Time:
		*(e.Value) = app.DateEpochtime(data, e.precision())
		return nil
	}

	return fmt.Errorf("pgstore: cannot scan %T value %[1]v into %T", src, e.Value)
}

func (e Epochtime) DecodeText(_ *pgtype.ConnInfo, data []byte) (err error) {
	if len(data) == 0 {
		*(e.Value) = 0 // NULL
		return nil
	}

	var (
		date time.Time
		text = string(data)
	)
	switch text {
	// case "infinity":
	// 	*dst = Timestamp{Status: Present, InfinityModifier: Infinity}
	// case "-infinity":
	// 	*dst = Timestamp{Status: Present, InfinityModifier: -Infinity}
	case "infinity", "-infinity":
		return fmt.Errorf("pgstore: cannot scan %T::timestamp value %[1]v into %T", text, e.Value)
	default:
		if strings.HasSuffix(text, " BC") {
			date, err = time.Parse(
				pgTimestampFormat, strings.TrimRight(text, " BC"),
			)
			date = time.Date(
				1-date.Year(), date.Month(), date.Day(),
				date.Hour(), date.Minute(), date.Second(),
				date.Nanosecond(),
				date.Location(),
			)
			if err != nil {
				return err
			}
			*(e.Value) = app.DateEpochtime(date, e.precision())
			return nil
		}
		var (
			length = len(text)
			format = pgTimestampFormat // timestamp
		)
		// timestamptz ?
	autodetect:
		for _, e := range []struct {
			offset int
			format string
		}{
			{9, pgTimestamptzSecondFormat},
			{6, pgTimestamptzMinuteFormat},
			{3, pgTimestamptzHourFormat},
		} {
			if length < e.offset {
				continue
			}
			switch text[length-e.offset] {
			case '-', '+': // TZ
				format = e.format
				break autodetect
			}
		}
		// if len(text) >= 9 && (text[len(text)-9] == '-' || text[len(text)-9] == '+') {
		// 	format = pgTimestamptzSecondFormat
		// } else if len(text) >= 6 && (text[len(text)-6] == '-' || text[len(text)-6] == '+') {
		// 	format = pgTimestamptzMinuteFormat
		// } else {
		// 	format = pgTimestamptzHourFormat
		// }
		date, err = time.Parse(format, text)
		if err != nil {
			return err
		}
		*(e.Value) = app.DateEpochtime(date, e.precision())
	}

	return nil
}
