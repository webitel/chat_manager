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
