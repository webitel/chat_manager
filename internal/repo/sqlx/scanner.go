package sqlxrepo

import (
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