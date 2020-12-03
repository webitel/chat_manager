package sqlxrepo

import (
	"time"
	"context"
)

type Operation struct {
	 ID string // RequestID
	 time.Time // Started! Local!
	 context.Context // Bindings
}

var DefaultSearchSize = 16

type SearchOptions struct {
	
	Operation
	// filters: extra
	Params map[string]interface{}
	
	Fields []string
	Sort []string
	
	Page int
	Size int

}

func (rpc *SearchOptions) GetSize() int {
	if rpc == nil {
		return DefaultSearchSize
	}
	switch {
	case rpc.Size < 0:
		return -1
	case rpc.Size > 0:
		// CHECK for too big values !
		return rpc.Size
	case rpc.Size == 0:
		return DefaultSearchSize
	}
	panic("unreachable code")
}

func (rpc *SearchOptions) GetPage() int {
	if rpc != nil {
		// Limited ? either: manual -or- default !
		if rpc.GetSize() > 0 {
			// Valid ?page= specified ?
			if rpc.Page > 0 {
				return rpc.Page
			}
			// default: always the first one !
			return 1
		}
	}
	// <nop> -or- <nolimit>
	return 0
}

func (q *SearchOptions) TermOfSearch() string {
	s, _ := q.Params["q"].(string)
	return s
}