package graph

// https://developers.facebook.com/docs/graph-api/reference/profile-picture-source/
type ProfilePicture struct {

	// A key to identify the profile picture for the purpose of invalidating the image cache
	CacheKey string `json:"cache_key,omitempty"`
	
	// True if the profile picture is the default 'silhouette' picture
	// Default
	IsSilhouette bool `json:"is_silhouette,omitempty"`
	
	// // Deprecated
	// Top uint32 `json:"top,omitempty"`
	
	// // Deprecated
	// Left uint32 `json:"left,omitempty"`
	
	// // Deprecated
	// Right uint32 `json:"right,omitempty"`
	
	// // Deprecated
	// Bottom uint32 `json:"bottom,omitempty"`

	// Picture width in pixels. Only returned when specified as a modifier
	// Default
	Width uint32 `json:"width,omitempty"`

	// Picture height in pixels. Only returned when specified as a modifier
	// Default
	Height uint32 `json:"height,omitempty"`

	// URL of the profile picture. The URL will expire.
	// Default
	URL string `json:"url,omitempty"`
}

type CoverPhoto struct {
	// The ID of the cover photo. Default
	ID string `json:"id,omitempty"`

	// Deprecated. Please use the id field instead. Default
	// CoverID string `json:"cover_id,omitempty"`

	// When greater than 0% but less than 100%, the cover photo overflows horizontally. The value represents the horizontal manual offset (the amount the user dragged the photo horizontally to show the part of interest) as a percentage of the offset necessary to make the photo fit the space. Default
	OffsetX float64 `json:"offset_x,omitempty"`

	// When greater than 0% but less than 100%, the cover photo overflows vertically. The value represents the vertical manual offset (the amount the user dragged the photo vertically to show the part of interest) as a percentage of the offset necessary to make the photo fit the space. Default
	OffsetY float64 `json:"offset_y,omitempty"`

	// Direct URL for the person's cover photo image. Default
	Source string `json:"source,omitempty"`
}