package webhooks

type WebHook struct {
	// Callback URL
	URL string
	// Verify token string
	Token string
	// Subscribe challenge token
	Verified string

	Subscriptions
}