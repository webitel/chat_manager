package graph

// An Error is a Graph API Error
// https://developers.facebook.com/docs/graph-api/guides/error-handling#handling-errors
type Error struct {
	// An error code.
	Code int `json:"code,omitempty"`
	// An error type.
	Type string `json:"type,omitempty"`
	// A human-readable description of the error.
	Message string `json:"message,omitempty"`
	// Additional information about the error.
	SubCode int `json:"error_subcode,omitempty"`
	// The title of the dialog, if shown.
	// The language of the message is based on the locale of the API request.
	UserTitle string `json:"error_user_title,omitempty"`
	// The message to display to the user.
	// The language of the message is based on the locale of the API request.
	UserMessage string `json:"error_user_msg,omitempty"`
	// Internal support identifier. When reporting a bug related to a Graph API call,
	// include the fbtrace_id to help us find log data for debugging.
	FBTraceID string `json:"fbtrace_id,omitempty"`
}

// Error message
func (err *Error) Error() string {
	return err.Message
}