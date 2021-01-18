package app

import (
	// "context"
	"github.com/google/uuid"
)


type UUID uuid.UUID

type Channel struct {

	*Chat // embedded(!)
	 User *User     `json:"user,omitempty"` // this chat channel owner
	 DomainID int64 `json:"domainId,omitempty"`
	 // The member's status in the chat.
	 // Can be “creator”, “administrator”, “member”, “restricted”, “left” or “kicked”
	 Status string  `json:"status,omitempty"`
	 Provider interface {
		 // Send(ctx context.Context, notify *Update) error
	 }              `json:"provider,omitempty"`
	 
	 // timing
	 Created int64  `json:"created,omitempty"`
	 Updated int64  `json:"updated,omitempty"`
	 Joined  int64  `json:"joined,omitempty"`
	 Closed  int64  `json:"closed,omitempty"`
}

func (c *Channel) IsClosed() bool {
	return c == nil || c.Closed != 0
}
