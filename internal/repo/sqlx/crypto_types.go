package sqlxrepo

import (
	"database/sql"
	"encoding/json"
	"fmt"

	dbx "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/crypto/cryptostore/schema"
)

func decryptJSONB(dst any, table, column string) sql.Scanner {
	return dbx.ScanFunc(func(src any) error {
		if src == nil {
			return nil // NULL
		}
		var jsonb json.RawMessage
		switch data := src.(type) {
		case json.RawMessage:
			jsonb = data
		case []byte:
			jsonb = data
		default:
			return fmt.Errorf("database: convert %T into %T type", src, dst)
		}
		if len(jsonb) == 0 {
			return nil // NULL
		}
		codec := Crypto().Base()
		jsonb, err := schema.DecryptJSONB(codec, jsonb)
		if err != nil {
			return fmt.Errorf("cryptostore: decrypt %s.%s ; %w", table, column, err)
		}
		return json.Unmarshal(jsonb, dst)
	})
}

