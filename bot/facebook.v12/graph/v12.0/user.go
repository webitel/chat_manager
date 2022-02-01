package graph

type User = UserProfile

// UserProfile
// https://developers.facebook.com/docs/messenger-platform/identity/user-profile#fields
// https://developers.facebook.com/docs/graph-api/reference/user/#default-public-profile-fields
type UserProfile struct {
	
	// The app user's App-Scoped User ID. This ID is unique to the app and cannot be used by other apps.
	ID string `json:"id,omitempty"`

	// The person's first name
	FirstName string `json:"first_name,omitempty"`
	
	// The person's middle name
	MiddleName string `json:"middle_name,omitempty"`

	// The person's last name
	LastName string `json:"last_name,omitempty"`

	// The person's full name
	Name string `json:"name,omitempty"`

	// The person's name formatted to correctly handle Chinese, Japanese, or Korean ordering
	NameFormat string `json:"name_format,omitempty"`

	// Shortened, locale-aware name for the person
	ShortName string `json:"short_name,omitempty"`

	// The person's profile picture
	Picture *UserPicture `json:"picture,omitempty"`

	// URL to the Profile picture. The URL will expire.
	PictureURL string `json:"profile_pic,omitempty"`

	// Locale of the user on Facebook. For supported locale codes, see Supported Locales.
	// pages_user_locale permission
	Locale string `json:"locale,omitempty"`

	// Timeone, number relative to GMT
	// pages_user_timezone permission
	Timezone int `json:"timezone,omitempty"`

	// Gender
	// pages_user_gender permission
	Gender string `json:"gender,omitempty"`
}

// https://developers.facebook.com/docs/graph-api/reference/user/picture/#fields
type UserPicture struct {
	// A single ProfilePictureSource node.
	Data *ProfilePicture `json:"data,omitempty"`
	// For more details about pagination, see the Graph API guide.
	Paging *Paging       `json:"paging,omitempty"`
}