package viber

// The Viber Userinfo
// https://developers.viber.com/docs/api/rest-bot-api/#get-user-details
type User struct {
	// Unique Viber user id
	ID string `json:"id,omitempty"`
	// User’s Viber name
	Name string `json:"name"`
	// URL of user’s avatar
	Avatar string `json:"avatar,omitempty"`
	// User’s 2 letter country code
	Country string `json:"country,omitempty"`
	// User’s phone language. Will be returned according to the device language
	Language string `json:"language,omitempty"`
	// The maximal Viber version that is supported by all of the user’s devices
	MaxVersion int `json:"api_version,omitempty"`
}
