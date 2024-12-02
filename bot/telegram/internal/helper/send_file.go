package helper

import (
	"io"
	"net/http"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendFile contains information about an internal File to upload as a File to Telegram.
type SendFile struct {
	URL  string
	Name string
}

var _ telegram.RequestFileData = SendFile{}

// NeedsUpload shows if the file needs to be uploaded.
func (src SendFile) NeedsUpload() bool {
	return true
}

// UploadData gets the file name and an `io.Reader` for the file to be uploaded. This
// must only be called when the file needs to be uploaded.
func (src SendFile) UploadData() (string, io.Reader, error) {
	resp, err := http.Get(src.URL)
	if err != nil {
		return src.Name, nil, err
	}

	// defer res.Body.Close()
	return src.Name, ReadCloser{resp.Body}, nil
}

// SendData gets the file data to send when a file does not need to be uploaded. This
// must only be called when the file does not need to be uploaded.
func (src SendFile) SendData() string {
	panic("SendFile must be uploaded")
}

// NewSendFile constructor for SendFile struct
func NewSendFile(url, name string) *SendFile {
	return &SendFile{URL: url, Name: name}
}
