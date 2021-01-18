package app

type Session struct {
	// Basic; Me; Owner
	*Channel // embedded(!)
	 Members []*Channel `json:"members"`
	 History []*Message `json:"history,omitempty"`
	 
	//  // timing
	//  Created int64 `json:"-"`
	//  Updated int64 `json:"-"`
	//  Closed  int64 `json:"-"`
}