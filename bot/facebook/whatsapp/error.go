package whatsapp

import "fmt"

// ErrorData describes the error
type ErrorData struct {
	// Describes the error. Example:
	// -----------------------------
	// Message failed to send because there were too many messages
	// sent from this phone number in a short period of time.
	Details string `json:"details"`
}

// Error information when a message failed
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#value-object
type MessageError struct {

	// ===== -v15.0 ===== //

	// Error code.
	// Build your app's error handling around error codes instead of subcodes or HTTP response status codes.
	Code int `json:"code"`
	// Combination of the error code and its title.
	Title string `json:"title,omitempty"`

	// ===== v16.0+ ===== //

	// Error code message.
	// This value is the same as the title value.
	// For example: Rate limit hit.
	// Note that the message property in API error response payloads
	// pre-pends this value with the a # symbol and the error code in parenthesis.
	// For example: (#130429) Rate limit hit.
	Message string `json:"message,omitempty"`
	// An error data object.
	*ErrorData `json:"error_data,omitempty"`
}

func (e *MessageError) IsCode(code int) (is bool) {
	if e != nil {
		is = (e.Code == code)
	}
	return // is
}

func (e *MessageError) Error() string {
	if e == nil {
		return "!ERR<nil>"
	}
	err := e.Message // DOES NOT pre-pend (#code)
	// if err == "" {
	err = fmt.Sprintf("(#%d) %s", e.Code, e.Title)
	// }
	more := e.ErrorData
	if more != nil && more.Details != "" {
		err += "; " + more.Details
	}
	return err
}
