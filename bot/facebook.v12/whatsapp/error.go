package whatsapp

// Error information when a message failed
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#value-object
type MessageError struct {
	// Error code.
	// Build your app's error handling around error codes instead of subcodes or HTTP response status codes.
	Code int `json:"code"`
	// Combination of the error code and its title.
	Title string `json:"title,omitempty"`
}

func (e *MessageError) IsCode(code int) (is bool) {
	if e != nil {
		is = (e.Code == code)
	}
	return // is
}

func (e *MessageError) Error() string {
	if e != nil {
		return e.Title
	}
	return ""
}
