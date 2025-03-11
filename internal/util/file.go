package util

import (
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+\.[a-zA-Z0-9]+$`)
)

// FetchFileDetails fetches file data by URL and returns filename, mimeType, extension, size, and error
func FetchFileDetails(link string) (filename, mimetype, extension string, size int64, err error) {
	// Parse the URL
	parsedURL, err := url.Parse(link)
	if err != nil {
		return
	}

	// Get the file name from URL path
	filename = path.Base(parsedURL.Path)

	// Fetch file size and headers
	resp, err := http.Head(link)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Get file size
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err = strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return
		}
	}

	// Get file extension
	extension = strings.ToLower(path.Ext(filename))

	// Get file MIME type from file extension
	mimetype = mime.TypeByExtension(extension)
	if mimetype == "" {
		// If MIME type is not found, fallback to the Content-Type header
		mimetype = resp.Header.Get("Content-Type")
	}

	// If filename is not correctly extracted, try Content-Disposition header
	if filename == "" || !filenameRegex.MatchString(filename) {
		contentDisposition := resp.Header.Get("Content-Disposition")
		if contentDisposition != "" {
			for _, params := range strings.Split(contentDisposition, ";") {
				pair := strings.SplitN(params, "=", 2)
				if len(pair) == 2 && strings.TrimSpace(pair[0]) == "filename" {
					filename = strings.Trim(pair[1], "\"")
					break
				}
			}
		}
	}

	// Add the extension if the filename doesn't already have one
	if filename != "" && path.Ext(filename) == "" && extension != "" {
		filename = filename + extension
	}

	return
}
