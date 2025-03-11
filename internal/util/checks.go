package util

import (
	"net/url"
	"strconv"
)

func IsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// IsURL checks if the given string is a valid URL.
func IsURL(str string) bool {
	// Check for empty string
	if str == "" {
		return false
	}

	// Try to parse the string as a URL
	parsedURL, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}

	// Check if the URL has a valid scheme (e.g., "http", "https")
	// if parsedURL.Scheme == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https" && parsedURL.Scheme != "ftp") {
	// 	return false
	// }

	// Check if the URL has a valid host (e.g., "www.example.com")
	if parsedURL.Host == "" {
		return false
	}

	return true
}
