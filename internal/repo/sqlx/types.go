package sqlxrepo

import (
	"time"
	"fmt"
	"encoding/json"
	"database/sql"
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



func NullProperties(props map[string]string) []byte { // JSONB

	// if props == nil {
	// 	return nil
	// }
	
	// delete(props, "")
	if len(props) == 0 {
		return nil
	}

	jsonb, _ := json.Marshal(props)

	return jsonb
}

func ScanProperties(props *map[string]string) ScanFunc {

	_ = *(props) // early pointer value binding

	return func(src interface{}) error {

		if src == nil {
			*(props) = nil
			return nil
		}

		var jsonb []byte

		switch v := src.(type) {
		case map[string]string:
			*(props) = v
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
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T", src, props)
		}

		if len(jsonb) == 0 {
			*(props) = nil
			return nil
		}

		err := json.Unmarshal(jsonb, props)
		if err != nil {
			*(props) = nil
			return fmt.Errorf("postgres: convert %[1]T value %[1]v to type %[2]T; %[3]s", src, props, err)
		}

		return nil
	}
}


// Properties represents extra key:value variables
type Properties map[string]string

func (e Properties) Value() (interface{}, error) {
	return NullProperties(e), nil
}

func (e *Properties) Scan(src interface{}) error {
	return ScanProperties((*map[string]string)(e))(src)
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