package sqlxrepo

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


// NullString sql.Valuer for native integer values
func NullInteger(i int64) *int64 {
	if i != 0 {
		return &i
	}
	return nil
}

func ScanInteger(dst *int64) ScanFunc {
	return func(src interface{}) error {
		if src == nil {
			return nil
		}
		switch v := src.(type) {
		case int64:
			*dst = v
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, dst)
		}
		return nil
	}
}


// NullString sql.Valuer for native string values
func NullString(s string) *string {
	if len(s) != 0 {
		return &s
	}
	return nil
}

func ScanString(s *string) ScanFunc {
	return func(src interface{}) error {
		if src == nil {
			return nil
		}
		switch v := src.(type) {
		case string:
			*s = v
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, s)
		}
		return nil
	}
}

func ScanJSON(dst interface{}) ScanFunc {
	return func(src interface{}) error {
		
		if src == nil {
			return nil
		}

		var data []byte

		switch v := src.(type) {
		case []byte:
			if len(v) == 0 {
				return nil
			}
			data = v
		case string:
			if len(v) == 0 {
				return nil
			}
			data = []byte(v)
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, dst)
		}

		err := json.Unmarshal(data, dst)

		return err
	}
}

var (

	jsonNull = []byte("null")
	// jsonNullArray = []byte("[]")
	jsonNullObject = []byte("{}")
)

func NullMetadata(md map[string]string) []byte { // JSONB

	// if props == nil {
	// 	return nil
	// }
	
	// delete(props, "")
	if len(md) == 0 {
		return nil
	}

	jsonb, _ := json.Marshal(md)
	for _, null := range [][]byte{
		jsonNull, jsonNullObject, // jsonNullArray,
	} {
		if bytes.Equal(jsonb, null) {
			jsonb = nil
			break
		}
	}

	return jsonb
}

func ScanMetadata(md *map[string]string) ScanFunc {

	_ = *(md) // early pointer value binding

	return func(src interface{}) error {

		if src == nil {
			*(md) = nil
			return nil
		}

		var jsonb []byte

		switch v := src.(type) {
		case map[string]string:
			*(md) = v
			return nil
		case json.RawMessage:
			jsonb = v
		case []byte:
			jsonb = v
		case string:
			if v != "" {
				jsonb = []byte(v)
			}
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, md)
		}

		if len(jsonb) == 0 {
			*(md) = nil
			return nil
		}

		err := json.Unmarshal(jsonb, md)
		if err != nil {
			*(md) = nil
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T; %[3]s", src, md, err)
		}

		return nil
	}
}


// Properties represents extra key:value variables
type Metadata map[string]string

func (e Metadata) Value() (interface{}, error) {
	return NullMetadata(e), nil
}

func (e *Metadata) Scan(src interface{}) error {
	return ScanMetadata((*map[string]string)(e))(src)
}



func NullDatetime(date *time.Time) *time.Time {
	if date == nil || date.IsZero() {
		return nil
	}
	return date
}

// ScanDatetime returns database/sql.Scanner
// bound to given date as a target value
// and able to deal with NULL values
func ScanDatetime(date *time.Time) ScanFunc {
	return func(src interface{}) error {

		if src == nil {
			*(date) = time.Time{} // time.IsZero(!)
			return nil
		}

		switch v := src.(type) {
		case time.Time:
			*(date) = v
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, date)
		}

		return nil
	}
}

func ScanTimestamp(sec *int64) ScanFunc {

	_ = *(sec) // early pointer value binding

	return func(src interface{}) error {

		if src == nil {
			*(sec) = 0
			return nil
		}

		switch v := src.(type) {
		case time.Time:
			*(sec) = v.Unix()
		case int64:
			*(sec) = v
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, sec)
		}

		return nil
	}
}

/*
func ScanEpochtime(date *int64, precision time.Duration) ScanFunc {

	_ = *(date) // early pointer value binding

	return func(src interface{}) error {

		if src == nil {
			*(date) = 0
			return nil
		}

		switch v := src.(type) {
		case time.Time:
			*(date) = v.Unix()
		case int64:
			*(date) = v
		default:
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, date)
		}

		return nil
	}
}*/