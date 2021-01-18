package app

type Account struct {

	ID string
	FirstName string
	LastName  string
	UserName  string
	Channel   string
	Contact   string
	Language  string
}

type Chat struct {

	ID        string `json:"chatId"` // Unique Chat-Channel IDentifier
	Title     string `json:"title,omitempty"` // Optional. Title, for supergroups, channels and group chats
	Channel   string `json:"channel,omitempty"` // type: telegram, facebook, chatflow, websocket etc
	Contact   string `json:"contact,omitempty"` 
	Username  string `json:"userName,omitempty"` // Optional. Username, for private chats, supergroups and channels if available
	FirstName string `json:"firstName,omitempty"` // Optional. First name of the other party in a private chat
	LastName  string `json:"lastName,omitempty"` // Optional. Last name of the other party in a private chat

	Invite    string `json:"roomId,omitempty"` // Invite link. Session ID

	// Photo  File{}
}

type ChatMember struct {
	*User // embedded
	 Seen int64
	 Status string
}