package util

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Converts the size in bytes to a readable format
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// JoinURL connects the base URL to the path, correctly processing query parameters
func JoinURL(base string, paths ...string) (string, error) {
	parsedBase, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}

	var newPathSegments []string
	queryValues := url.Values{}

	for _, p := range paths {
		trimmed := strings.Trim(p, "/")
		if strings.Contains(trimmed, "?") {
			parts := strings.SplitN(trimmed, "?", 2)
			newPathSegments = append(newPathSegments, parts[0])
			queryPart, err := url.ParseQuery(parts[1])
			if err != nil {
				return "", err
			}
			for key, values := range queryPart {
				for _, value := range values {
					queryValues.Add(key, value)
				}
			}
		} else {
			newPathSegments = append(newPathSegments, trimmed)
		}
	}

	parsedBase.Path = path.Join(append([]string{strings.Trim(parsedBase.Path, "/")}, newPathSegments...)...)
	if len(queryValues) > 0 {
		parsedBase.RawQuery = queryValues.Encode()
	}

	return parsedBase.String(), nil
}
