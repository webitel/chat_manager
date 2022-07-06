package app

import "strings"

type Session struct {
	// Basic; Me; Owner
	*Channel            // embedded(!)
	Members  []*Channel `json:"members"`
	History  []*Message `json:"history,omitempty"`

	//  // timing
	//  Created int64 `json:"-"`
	//  Updated int64 `json:"-"`
	//  Closed  int64 `json:"-"`
}

// GetMember trying to locate member's channel by given id.
func (c *Session) GetMember(id string) (chat *Channel) {
	equal := strings.EqualFold
	if equal(id, c.Channel.ID) {
		return c.Channel
	}
	for _, sub := range c.Members {
		if equal(id, sub.ID) {
			return sub
		}
	}
	return nil
}
