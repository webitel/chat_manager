package app

const (

	ContactBot  = "bot"
	ContactUser = "user"

	ChannelBot  = "chatflow"
	ChannelUser = "websocket"

)

// User Account Info
type User struct {

	ID        int64  `json:"id"`
	Channel   string `json:"channel,omitempty"`
	Contact   string `json:"contact,omitempty"`

	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	UserName  string `json:"userName,omitempty"`
	Language  string `json:"language,omitempty"`
}

func (e *User) IsBot() bool {
	return e.Channel == ContactBot
}

func (e *User) IsUser() bool {
	return e.Channel == ContactUser
}