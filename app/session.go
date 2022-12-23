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
	if c != nil {
		equal := strings.EqualFold
		if c.Channel != nil && equal(id, c.Channel.ID) {
			return c.Channel
		}
		for _, sub := range c.Members {
			if sub != nil && equal(id, sub.ID) {
				return sub
			}
		}
	}
	return nil
}

// GetUser lookup member's channel by given User ID.
func (c *Session) GetUser(id int64) (chat *Channel) {
	if c != nil {
		equal := func(a, b int64) bool { return a != 0 && a == b }
		if c.Channel != nil && equal(id, c.Channel.User.ID) {
			return c.Channel
		}
		for _, sub := range c.Members {
			if sub != nil && equal(id, sub.User.ID) {
				return sub
			}
		}
	}
	return nil
}
