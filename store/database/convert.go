package database

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ScanFunc is custom database/sql.Scanner
type ScanFunc func(src interface{}) error

// Implements sql.Scanner interface
var _ sql.Scanner = ScanFunc(nil)

// Scan implements sql.Scanner interface
func (fn ScanFunc) Scan(src interface{}) error {
	if fn != nil {
		return fn(src)
	}
	// IGNORE
	return nil
}

func NullTimestamp(tsec int64) *time.Time {
	
	if tsec == 0 {
		return nil
	}

	date := TimestampDate(tsec).UTC()
	return &date
}

func ScanTimestamp(tsec *int64) ScanFunc {
	return func(src interface{}) error {
		if src == nil {
			return nil
		}
		switch data := src.(type) {
		case int64:
			(*tsec) = data
		case time.Time:
			if !data.IsZero() {
				// precision := (int64)(1e6) // time.Millisecond) // ms
				(*tsec) = DateTimestamp(data) // v.UnixNano() / precision
			}
		default:
			return fmt.Errorf("database: convert %[1]T value %[1]v to %[2]T type", src, tsec)
		}
		return nil
	}
}

var jsonNull = []byte("null")

func NullJSONBytes(src interface{}) []byte {

	if src == nil {
		return nil
	}

	data, err := json.Marshal(src)

	if err != nil {
		panic(fmt.Errorf(
			"database: convert %T value to JSON bytes",
			 src,
		))
	}

	if bytes.EqualFold(data, jsonNull) {
		return nil
	}

	return data
}

func ScanJSONBytes(dst interface{}) ScanFunc {
	return func(src interface{}) error {
		if src == nil {
			return nil
		}
		switch data := src.(type) {
		case []byte:
			if len(data) == 0 {
				return nil
			}
			return json.Unmarshal(data, dst)
		default:
			return fmt.Errorf("database: convert %[1]T value %[1]v to %[2]T type", src, dst)
		}
		panic("unreachable code")
	}
}