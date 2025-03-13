package bot

import (
	"strings"

	chat "github.com/webitel/chat_manager/api/proto/chat"
)

// Account contact info
type Account struct {
	ID        int64  `json:"id,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	Channel   string `json:"channel,omitempty"` // channel: communication type name, e.g.: user, flow, telephone, telegram, viber
	Contact   string `json:"contact,omitempty"` // channel: contact string value
}

func (e *Account) IsBot() bool {
	return e.Channel == "bot" // "flow"
}

func (e *Account) IsUser() bool {
	return e.Channel == "user"
}

// func (e *Account) GetUsername() string {

// 	e.Username = strings.TrimSpace(e.Username)
// 	return e.Username
// }

func (e *Account) DisplayName() string {

	names := []string{e.FirstName, e.LastName, e.Username}
	validNames := make([]string, 0, 3)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			validNames = append(validNames, name)
		}
	}

	displayName := strings.Join(validNames, " ")
	if displayName == "" {
		displayName = "noname"
	}

	return displayName
}

// Update represents unified message eventArgs
type Update struct {
	// Unique ID
	ID int64
	// Chat that this message belongs to
	Chat *Channel
	// User channel contact
	User *Account //
	// Title for .this chat
	Title string
	// // Action, e.g.: text, file, edit, send, read, joined, typing, kicked etc
	// Event string
	// Message envelope
	Message *chat.Message // Message; embedded
	// // Edited message details
	// Edited int64 // date; if non-zero ~ .Event="edited"
	// // For edited message update, this is identifier of the original message
	// EditedMessageID int64
	// // joined
	// JoinMembers []*Account
	// // kicked
	// KickMembers []*Account
}

const (
	// chat.Close() command dispositions
	commandCloseRecvDisposition = "/close"              // by external: end-user request
	commandCloseSendDisposition = "Conversation closed" // by internal: .chat.server channel
)

// IsCommandClose indicates whether
// given update represents the final:
// chat.closed channel notification text
func (e *Update) IsCommandClose() bool {

	if e.Message != nil {

		switch e.Message.Type {
		case "closed":
			return true
		case "text":
			// if e.Message.UpdatedAt == 0 {
			text := e.Message.GetText()
			return IsCommandClose(text)
			// }
		}
	}

	return false
}

// IsCommandClose indicates whether
// given text represents the chat.closed
// channel notification or command text
func IsCommandClose(text string) bool {
	return text == commandCloseSendDisposition ||
		text == commandCloseRecvDisposition
}
