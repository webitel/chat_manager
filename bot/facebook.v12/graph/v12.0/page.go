package graph

// Page represents a Facebook Page.
type Page struct {
	// The ID representing a Facebook Page. (numeric string)
	ID string `json:"id,omitempty"`
	// The name of the Page. CoreDefault
	Name string `json:"name,omitempty"`
	// Information about the page's cover photo
	Cover *CoverPhoto `json:"cover,omitempty"`
	// This Page's profile picture
	Picture *PagePicture `json:"picture,omitempty"`
	// Instagram Professional or Business Account associated with this page
	Instagram *InstagramUser `json:"instagram,omitempty"`
	// The Page's access token. Only returned if the User making the request
	// has a role (other than Live Contributor) on the Page.
	// If your business requires two-factor authentication,
	// the User must also be authenticated
	AccessToken string `json:"access_token,omitempty"`
	
}

// https://developers.facebook.com/docs/graph-api/reference/page/picture/#fields
type PagePicture struct {
	// A single ProfilePictureSource node.
	Data *ProfilePicture `json:"data,omitempty"`
	// For more details about pagination, see the Graph API guide.
	Paging *Paging       `json:"paging,omitempty"`
}

// IG User Represents an Instagram Business Account or an Instagram Creator Account.
// https://developers.facebook.com/docs/instagram-api/reference/ig-user/#fields
type InstagramUser struct {
	// App-scoped User ID. <IGSID>
	// IGSID is an Instagram (IG)-scoped ID assigned to each user that messages an Instagram Professional account.
	// IGSID is used throughout Messenger API support for Instagram to enable you to identify the user associated with sent and received messages.
	// IGSID is a unique mapping/hash between the Instagram user ID and the Instagram Professional account.
	// You should use IGSID as a primary identifier in your backend system instead of username because an IG username can be changed by the user.
	ID string `json:"id"`

	// Instagram User ID. Used with Legacy Instagram API, now deprecated. Use id instead.
	// ig_id int64 `json:"ig_id,omitempty"`

	// Profile name.
	Name string `json:"name,omitempty"`

	// Profile username.
	// Public
	Username string `json:"username"`

	// Profile picture URL.
	PictureURL string `json:"profile_picture_url,omitempty"`

	// Total number of Instagram users following the user.
	// Public
	FollowersCount int `json:"followers_count,omitempty"` 

	// Profile website URL.
	// Public
	Website string `json:"website,omitempty"`

}