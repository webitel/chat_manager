package util

import (
	"strings"
	"unicode"
)

// ParseFullName splits the full name into first and last names
func ParseFullName(fullName string) (firstName, lastName string) {
	trimSpace := unicode.IsSpace
	firstName = strings.TrimFunc(fullName, trimSpace)
	if sp := strings.LastIndexFunc(firstName, trimSpace); sp > 1 {
		lastName, firstName =
			strings.TrimLeftFunc(firstName[sp:], trimSpace),
			strings.TrimRightFunc(firstName[0:sp], trimSpace)
	}
	return
}
