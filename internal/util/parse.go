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

// ParseMediaType extracts the media type (the part before '/') from a given MIME type string.
// It trims any leading or trailing spaces, converts the string to lowercase,
// and returns the media type. If the '/' character is not found, the entire string is returned.
func ParseMediaType(mimeType string) string {
	mediaType := strings.TrimSpace(mimeType)
	mediaType = strings.ToLower(mediaType)
	subt := strings.IndexByte(mediaType, '/')
	if subt > 0 {
		return mediaType[:subt]
	}
	return mediaType
}
