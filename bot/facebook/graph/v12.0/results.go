package graph

// Data is an envelope for any result structures
type Data interface{}

// A cursor refers to a random string of characters which marks a specific item in a list of data.
// The cursor will always point to the item, however it will be invalidated if the item is deleted or removed.
// Therefore, your app shouldn't store cursors or assume that they will be valid in the future.
type Cursors struct {
	// This is the cursor that points to the start of the page of data that has been returned.
	Before string `json:"before,omitempty"`
	// This is the cursor that points to the end of the page of data that has been returned.
	After  string `json:"after,omitempty"`
}


type Paging struct {
	// Cursors
	Cursors *Cursors `json:"cursors,omitempty"`
	// This offsets the start of each page by the number specified.
	Offset   int32   `json:"offset,omitempty"`
	// This is the maximum number of objects that may be returned.
	// A query may return fewer than the value of limit due to filtering.
	// Do not depend on the number of results being fewer than the limit value to indicate
	// that your query reached the end of the list of data, use the absence of next instead as described below.
	// For example, if you set limit to 10 and 9 results are returned, there may be more data available,
	// but one item was removed due to privacy filtering.
	// Some edges may also have a maximum on the limit value for performance reasons.
	// In all cases, the API returns the correct pagination links.
	Limit    int32   `json:"limit,omitempty"`
	// The Graph API endpoint that will return the next page of data.
	// If not included, this is the last page of data.
	// Due to how pagination works with visibility and privacy, it is possible
	// that a page may be empty but contain a next paging link.
	// Stop paging when the next link no longer appears.
	Next     string  `json:"next,omitempty"`
	// The Graph API endpoint that will return the previous page of data.
	// If not included, this is the first page of data.
	Previous string  `json:"previous,omitempty"`
}

func (c *Paging) More() bool {
	return c != nil && c.Next != ""
}

// type PagedResult struct {
// 	Data interface{} `json:"data,omitempty"`
// 	Error *Error     `json:"error,omitempty"`
// 	Paging *Paging   `json:"paging,omitempty"`
// }

// func (res *PagedResult) More() bool {
// 	return res.Paging != nil && res.Paging.Next != ""
// }


// type Result struct {
// 	Success bool        `json:"success,omitempty"`
// 	Error   *Error      `json:"error,omitempty"`
// 	Data    interface{} `json:"data,omitempty"`
// 	Paging  *Paging     `json:"paging,omitempty"`
// }

// Success result envelope
type Success struct {
	 Ok bool `json:"success,omitempty"`
}

// API Result structure
type Result struct {
	 // Data envelope
	 Data `json:",omitempty"` // Anonymous field
	 Error *Error `json:"error,omitempty"`
}

// PagedResult envelope
type PagedResult struct {
	 // Data envelope; Mostly this is an array
	 Data           `json:"data,omitempty"`
	 Paging *Paging `json:"paging,omitempty"`
}
