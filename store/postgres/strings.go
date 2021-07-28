package postgres

import (
	"strings"
)

// Substrings represents [I]LIKE pattern value
func Substring(pattern []string) string {
	if len(pattern) == 0 {
		return ""
	}
	// TODO: escape(%)
	s := append([]string(nil), pattern...)
	const ESC = "\\" // https://postgrespro.ru/docs/postgresql/12/functions-matching#FUNCTIONS-LIKE
	for i := 0; i < len(s) && len(s[i]) != 0; i++ {
		s[i] = strings.ReplaceAll(s[i], "_", ESC+"_") // escape control '_' (single char entry)
		s[i] = strings.ReplaceAll(s[i], "?", "_")     // propagate '?' char for PostgreSQL purpose
		s[i] = strings.ReplaceAll(s[i], "%", ESC+"%") // escape control '%' (any char(s) or none)
	}
	return strings.Join(s, "%")
}