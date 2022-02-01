package graph

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// type text []byte

// func (b text) String() string {
// 	return string(b)
// }
// func (b *text) UnmarshalText(text []byte) error {

// }
// func (b *text) MarshalText() (text []byte, err error) {

// }
// func (b *text) Read(p []byte) (n int, err error) {

// }
// func (b *text) Write(p []byte) (n int, err error) {

// }

type BatchRequest struct {
	
	// Name MUST be unique across batch request(s)
	Name        string `json:"name,omitempty"`
	// GET | PUT | POST | DELETE
	Method      string `json:"method"`
	// GET | DELETE [?query=]
	RelativeURL string `json:"relative_url"`
	Header      Header `json:"headers,omitempty"`
	// PUT | POST
	Body        string `json:"body,omitempty"`
}



type BatchResult struct {
	Code        int    `json:"code"`
	Header      Header `json:"headers,omitempty"`
	Body        string `json:"body,omitempty"`
	// Body        []byte `json:"body,string,omitempty"`
}

type (
	
	Header http.Header
	header struct {
		Name  string   `json:"name"`
		Value string   `json:"value,omitempty"`
	}
)

func (h Header) MarshalJSON() ([]byte, error) {

	var n int
	for _, vs := range h {
		n += len(vs)
	}

	kvs := make([]header, n)
	
	n = 0
	for key, vals := range h {
		for _, val := range vals {
			e := &kvs[n]
			e.Name = key
			e.Value = val
			(n)++
		}
		
	}

	return json.Marshal(kvs)
}

func (h Header) UnmarshalJSON(data []byte) error {

	var src []*header
	err := json.Unmarshal(data, &src)
	if err != nil {
		return err
	}

	dst := http.Header(h)
	for _, e := range src {
		dst.Set(e.Name, e.Value)
	}

	return nil
}

// FormBatchRequest encodes GraphAPI Batch Request(s)
func FormBatchRequest(form url.Values, batch ...*BatchRequest) (url.Values, error) {

	jsonb, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}

	form.Set("batch", string(jsonb))
	return form, nil
}

// ScanBatchResults decodes GrapthAPI Batch Result(s)
func ScanBatchResults(body io.Reader) ([]*BatchResult, error) {

	var batch []*BatchResult
	err := json.NewDecoder(body).Decode(&batch)
	
	if err != nil {
		return nil, err
	}

	return batch, nil
}