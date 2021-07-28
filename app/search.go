package app

// SearchOptions
type SearchOptions struct {
	// Operation Context
	Context

	// Selection BY Unique IDentifier(s)
	ID []int64
	// Selection BY term of search
	Term string
	// Selection BY filter(s) AND condition
	Filter map[string]interface{}
	// Selection BY access modificator granted
	Access uint8

	// Fields to be returned for entries
	Fields []string
	// Order fields to sort result entries
	Order []string
	// Size number of entries to be returned
	Size int
	// Page number of selection to be returned
	Page int
}

var DefaultSearchSize = 16

// GetSize returns the normalized number of records to be fetched; LIMIT
func (rpc *SearchOptions) GetSize() int {
	if rpc == nil {
		return DefaultSearchSize
	}
	switch {
	case rpc.Size < 0:
		return -1 // ALL: NO LIMIT !
	case rpc.Size > 0:
		// CHECK for too big values !
		return rpc.Size
	case rpc.Size == 0:
		return DefaultSearchSize
	}
	panic("unreachable code")
}

// GetPage returns the normalized number of result page(s)
func (rpc *SearchOptions) GetPage() int {

	if rpc.GetSize() < 0 {
		return 0 // ALL: NO LIMIT !
	}
	// Valid ?page= specified ?
	if rpc != nil && rpc.Page > 0 {
		return rpc.Page
	}
	// default: always the first one !
	return 1
}

// FilterAND grab assertion condition with filter code name
func (rpc *SearchOptions) FilterAND(name string, cond interface{}) {

	if rpc.Filter == nil {
		rpc.Filter = make(map[string]interface{})
	}

	rpc.Filter[name] = cond
}
