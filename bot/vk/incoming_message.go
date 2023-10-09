package vk

type VKEvent struct {
	// Id of group messaged in
	GroupId int `json:"group_id,omitempty"`
	// type of event ( see docs )
	Type string `json:"type,omitempty"`
	// API version
	Version string `json:"v,omitempty"`
	// Object ( ! DEPENDS ON WHAT EVENT IS IT )
	Object *Object `json:"object,omitempty"`
}

type Message struct {
	// Time sent
	Date int `json:"date,omitempty"`
	// ID of user
	FromId int64 `json:"from_id,omitempty"`
	// ID of message ( !GENERAL )
	Id int `json:"id,omitempty"`
	// Direction:
	// 0 - received;
	// 1 - sent;
	Out int `json:"out,omitempty"`
	// Attachments of incoming message
	Attachments []map[string]any `json:"attachments,omitempty"`
	// ID of message in dialogue ( !LOCAL ) ( AUTOINCREMENT )
	ConversationMessageId int64 `json:"conversation_message_id,omitempty"`
	// Array of forwarded messages ( !UNSUPPORTED)
	FwdMessages []interface{} `json:"fwd_messages,omitempty"`
	// Is message checked as important ( !UNSUPPORTED )
	Important bool `json:"important,omitempty"`
	//IsHidden     bool          `json:"is_hidden,omitempty"`
	// ID of peer that messaged
	PeerId int64 `json:"peer_id,omitempty"`
	// Unique number that used to know the uniqueness of message ( only for sent messages )
	RandomId int    `json:"random_id,omitempty"`
	Text     string `json:"text,omitempty"`
	// Message replied to
	ReplyMessage *Message `json:"reply_message,omitempty"`

	Geo *Geolocation `json:"geo,omitempty"`
}

// Describes opportunities of the client
type ClientInfo struct {
	ButtonActions  []string `json:"button_actions,omitempty"`
	Keyboard       bool     `json:"keyboard,omitempty"`
	InlineKeyboard bool     `json:"inline_keyboard,omitempty"`
	Carousel       bool     `json:"carousel,omitempty"`
	LangId         int      `json:"lang_id,omitempty"`
}

type Object struct {
	*Message    `json:"message,omitempty"`
	*ClientInfo `json:"client_info,omitempty"`
}

type Geolocation struct {
	Type        string       `json:"type,omitempty"`
	Coordinates *Coordinates `json:"coordinates,omitempty"`
}

type Coordinates struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

//type VKPlace struct {
//	latitude   float64 `json:"latitude,omitempty"`
//	longtitude float64 `json:"longtitude,omitempty"`
//}
