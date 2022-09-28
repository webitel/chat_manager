package viber

// setWebhook request options
// https://developers.viber.com/docs/api/rest-bot-api/#setting-a-webhook
type setWebhook struct {
	// Account webhook URL to receive callbacks & messages from users.
	// Validation: Webhook URL must use SSL.
	// Note: Viber doesn’t support self signed certificates
	CallbackURL string `json:"url"`
	// Indicates the types of Viber events that the account owner would like to be notified about.
	// Don’t include this parameter in your request to get all events
	EventTypes []string `json:"event_types,omitempty"`

	// Indicates whether or not the bot should receive the user name.
	SendName bool `json:"send_name,omitempty"`
	// Indicates whether or not the bot should receive the user photo.
	SendPhoto bool `json:"send_photo,omitempty"`
}

func (setWebhook) method() string {
	return "set_webhook"
}
