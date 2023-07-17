package graph_test

import (
	"encoding/json"

	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
)

// Example of PagedResults usage
func ExampleResult_success() {

	const jsonb = `{"success":true}`

	var (
		res graph.Success
		ret = graph.Result{
			Data: &res,
		}
	)

	err := json.Unmarshal([]byte(jsonb), &ret)

	if err != nil {
		// Failed to decode result structure
	}

	if ret.Error != nil {
		// ERR: API Error result !
	}

	if res.Ok {
		// SUCCEED !
	}
}

// Example of PagedResults usage
func ExampleResult_pagedResult() {

	const jsonb = `{"data":[{"id":"12345678","name":"Full name to display"},{"id":"87654321","name":"Another name to display"}]}`

	var (
		items []interface{}
		res   = graph.PagedResult{
			Data: &items,
		}
		ret = graph.Result{
			Data: &res,
		}
	)

	err := json.Unmarshal([]byte(jsonb), &ret)

	if err != nil {
		// Failed to decode result structure
	}

	if ret.Error != nil {
		// ERR: API Error result !
	}

	for i := 0; i < len(items); i++ {
		// item := items[i]
	}

	if res.Paging.Next != "" {

	}
}
