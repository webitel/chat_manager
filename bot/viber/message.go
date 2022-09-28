package viber

const (
	mediaURL      = "url"
	mediaText     = "text"
	mediaFile     = "file"
	mediaVideo    = "video"
	mediaImage    = "picture"
	mediaSticker  = "sticker"
	mediaContact  = "contact"
	mediaLocation = "location"
	// https://developers.viber.com/docs/api/rest-bot-api/#rich-media-message--carousel-content-message
	mediaRichData = "rich_media" // not supported yet
)

// Contact info
type Contact struct {
	// The Contact’s username.
	Name string `json:"name,omitempty"`
	// The Contact’s phone number.
	Phone string `json:"phone_number,omitempty"`
	// The Contact’s avatar URL.
	Avatar string `json:"avatar,omitempty"`
}

// Location coordinates
type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// Message content
type Message struct {
	// Message type:
	// REQUIRED. Available message types:
	// - url
	// - text
	// - file
	// - video
	// - picture
	// - sticker
	// - contact
	// - location
	Type string `json:"type"`
	// The text of the message.
	// Relevant for `text` type messages
	// Max length 7,000 characters
	Text string `json:"text,omitempty"`
	// URL of the message media - can be `image`, `video`, `file` and `url`.
	// Image/Video/File URLs will have a TTL of 1 hour.
	MediaURL string `json:"media,omitempty"`
	// Location coordinates
	// Relevant for `location` type messages
	Location *Location `json:"location,omitempty"`
	// Contact info shared
	// Relevant for `contact` type messages
	Contact *Contact `json:"contact,omitempty"`
	// The filename.
	// Relevant for `file` type messages
	FileName string `json:"file_name,omitempty"`
	// The file size, in bytes.
	// Relevant for `file` type messages
	FileSize int64 `json:"file_size,omitempty"`
	// Video length in seconds.
	// Relevant for `video` type messages
	Duration int `json:"duration,omitempty"`
	// Unique Viber sticker ID.
	// Relevant for `sticker` type messages
	StickerId int64 `json:"sticker_id,omitempty"`
	// URL of a reduced size image (JPEG, PNG, GIF)
	// Relevant for `picture`, `video` type messages
	// OPTIONAL. Recommended: 400x400. Max size: 100kb.
	Thumbnail string `json:"thumbnail,omitempty"`
	// Tracking data sent with the last message to the user.
	// Allow the account to track messages and user’s replies.
	// Sent value will be passed back with user’s reply.
	// OPTIONAL. Max 4000 characters.
	TrackData string `json:"tracking_data,omitempty"`
	// Thumbnail string `json:"thumbnail,omitempty"`
}
