// package messenger
//
// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_handovers
package messenger

type (

	pass_thread_control struct {
		// TODO:
	}

	take_thread_control struct {
		// TODO:
	}

	// This callback will be sent to the Primary Receiver app
	// when a Secondary Receiver app calls the Request Thread Control API.
	// The Primary Receiver may then choose to honor the request and
	// pass thread control, or ignore the request.
	request_thread_control struct {
		// App ID of the Secondary Receiver that is requesting thread control.
		AppID string `json:"requested_owner_app_id"`
		// Custom string specified in the API request.
		Metadata string `json:"metadata,omitempty"`
	}

	// This callback will occur when a page admin changes the role of your application.
	// An app can be assigned the roles of primary_receiver or secondary_receiver.
	app_roles map[string][]string
)

