package app

const (
	// send: event
	MessageText   = "text" // send: message text
	MessageFile   = "file" // send: file document
	MessageEdit   = "edit" // send: previously sent message edited
	MessageRead   = "read" // send: seen unread message(s); NOTE: implies online(?) presence status notification
	// notification
	MessageInvite = "invite" // send: invite new members
	MessageJoined = "joined" // send: new members joined the chat room
	
	MessageClosed = "closed" // send:  chat/channel closed closed

	MessageTyping = "typing" // send: user is typing message text right now
	MessageUpload = "upload" // send: user start uploading file documents

)

type Update struct {
	// Unique Update IDentifier
	 ID string
	*Message
}

// Message 
type Message struct {

	ID             int64          `json:"id"` // unique message id
	From           *User          `json:"from"` // [FROM] sender member contact
	Chat           *Chat          `json:"chat"` // [TO] chat channel, the message belongs to
	Date           int64          `json:"date"` // sent date; created
	Type           string         `json:"type"` // mime type
	// inline message event arguments; embedded
	MessageForwarded

	// Optional. For replies, the original message.
	// Note that the Message object in this field will not contain
	// further reply_to_message fields even if it itself is a reply.
	ReplyToMessage *Message       `json:"reply_to_message,omitempty"`

	// EditDate for 'edited' messages indicates when message was edited, in milliseconds
	EditDate        int64         `json:"edit_date,omitempty"` // updated
	// Text message
	Text            string        `json:"text,omitempty"`

	// Caption string  `json:"caption,omitempty"` // USE: .Text instead !
	File            *Document     `json:"file,omitempty"`

	NewChatMembers  []*ChatMember `json:"new_chat_members,omitempty"`
	LeftChatMembers []*ChatMember `json:"left_chat_members,omitempty"`

	// NewChatTitle string  `json:"new_chat_title,omitempty"`
	// NewChatPhoto *File   `json:"new_chat_photo,omitempty"`
	// DeleteChatPhoto bool `json:"delete_chat_photo,omitempty"`
}

// MessageForwarded event arguments
type MessageForwarded struct {
	 ForwardFrom *User                      `json:"forward_from,omitempty"`
	 ForwardFromChat *Chat                  `json:"forward_from_chat,omitempty"`
	 ForwardFromMessageID int64             `json:"forward_from_message_id,omitempty"`
	 ForwardFromVariables map[string]string `json:"forward_from_variables,omitempty"`
	 ForwardSenderName string               `json:"forward_sender_name,omitempty"`
	 ForwardDate int64                      `json:"forward_date,omitempty"`
}

// Document file info
type Document struct {
	 ID   int64  `json:"id"`
	 Size int64  `json:"size"`
	 Type string `json:"mime"`
	 Name string `json:"name"`
}