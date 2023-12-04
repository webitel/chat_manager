package vk

type Photo struct {
	ID      int64 `json:"id,omitempty"`
	AlbumID int64 `json:"album_id,omitempty"`
	// ID of photo owner
	OwnerID int64  `json:"owner_id,omitempty"`
	Caption string `json:"text,omitempty"`
	// Date added (!UNIX)
	Date int64 `json:"date,omitempty"`
	// Array of photo copies with different sizes
	Sizes []ImageProps `json:"sizes,omitempty"`
}

type ImageProps struct {
	Type   string `json:"type,omitempty"`
	Url    string `json:"url,omitempty"`
	Width  int64  `json:"width,omitempty"`
	Height int64  `json:"height,omitempty"`
}

type Video struct {
	ID int64 `json:"id,omitempty"`
	// ID of video owner
	OwnerID     int64  `json:"owner_id,omitempty"`
	Description string `json:"description,omitempty"`
	Title       string `json:"title,omitempty"`
	// IN SECS!
	Duration int64 `json:"duration,omitempty"`
	// Date added (!UNIX)
	Date     int64  `json:"date,omitempty"`
	Url      string `json:"player,omitempty"`
	Platform string `json:"platform"`
}

type Audio struct {
	ID int64 `json:"id,omitempty"`
	// ID of audio owner
	OwnerID int64  `json:"owner_id,omitempty"`
	Title   string `json:"title,omitempty"`
	// IN SECS!
	Duration int64 `json:"duration,omitempty"`
	// Date added (!UNIX)
	Date int64  `json:"date,omitempty"`
	Url  string `json:"url,omitempty"`
}

type VoiceMessage struct {
	ID int64 `json:"id,omitempty"`
	// ID of audio owner
	OwnerID int64 `json:"owner_id,omitempty"`
	// IN SECS!
	Duration int64  `json:"duration,omitempty"`
	Url      string `json:"url,omitempty"`
	LinkMP3  string `json:"link_mp3"`
	LinkOGG  string `json:"link_ogg"`
}

type Document struct {
	ID int64 `json:"id,omitempty"`
	// ID of photo owner
	OwnerID int64  `json:"owner_id,omitempty"`
	Title   string `json:"title,omitempty"`
	// IN SECS!
	Size int64 `json:"size,omitempty"`

	//1 - text
	//2 - archive
	//3 - gif
	//4 - image
	//5 - audio
	//6 - video
	//7 - e-book
	//8 - unknown
	Type      int    `json:"type,omitempty"`
	Extension string `json:"ext,omitempty"`
	// Date added (!UNIX)
	Date int64  `json:"date,omitempty"`
	Url  string `json:"url,omitempty"`
}

type Sticker struct {
	Images       []ImageProps `json:"images,omitempty"`
	AnimationUrl string       `json:"animation_url,omitempty"`
	IsAllowed    bool         `json:"is_allowed,omitempty"`
}
