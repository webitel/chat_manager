package util

import "database/sql"

// ValidateNullStrings checks and updates the Valid field of each sql.NullString passed in.
// If the String field of a sql.NullString is non-empty, the Valid field is set to true;
// otherwise, it is set to false. If a nil pointer is encountered, it is skipped.
func ValidateNullStrings(strings ...*sql.NullString) {
	for _, str := range strings {
		if str == nil {
			continue // Skip nil
		}

		str.Valid = str.String != ""
	}
}
