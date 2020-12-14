package sqlxrepo

import (
	"fmt"
	"encoding/json"
	"database/sql"
)

// ScanFunc is custom database/sql.Scanner.
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

func NullProperties(props map[string]string) interface{} {

	if props == nil {
		return nil
	}
	
	delete(props, "")

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

		if jsonb == nil {
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