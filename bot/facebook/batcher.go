package facebook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/micro/micro/v3/service/errors"
)

type Batcher interface {
	Do(ctx context.Context, req *BatchRequest, accessToken string) ([]*BatchResponse, error)
	DoParallel(ctx context.Context, req *BatchRequest, accessToken string) ([]*BatchResponse, error)
}

const (
	BaseFacebookURL = "https://graph.facebook.com"
	MaxBatchSize    = 50
)

var (
	ErrEmptyBatchRequest = errors.BadRequest("facebook.batch.empty", "received empty graph batch request")
)

type SubRequest struct {
	Method      string `json:"method"`
	RelativeURL string `json:"relative_url"`
	Body        string `json:"body,omitempty"`
}

func NewSubRequest(method, relativeURL, body string) *SubRequest {
	return &SubRequest{
		Method:      method,
		RelativeURL: relativeURL,
		Body:        body,
	}
}

type BatchRequest struct {
	Requests       []*SubRequest
	IncludeHeaders bool
}

func NewBatchRequest(includeHeaders bool, requests ...*SubRequest) *BatchRequest {
	return &BatchRequest{
		Requests:       requests,
		IncludeHeaders: includeHeaders,
	}
}

func (r *BatchRequest) Validate() error {
	if r == nil || len(r.Requests) == 0 {
		return ErrEmptyBatchRequest
	}

	for i := range r.Requests {
		if r.Requests[i].Method == "" {
			return errors.BadRequest("facebook.batch.invalid_subrequest", "sub-request %d has empty method", i)
		}

		if r.Requests[i].RelativeURL == "" {
			return errors.BadRequest("facebook.batch.invalid_subrequest", "sub-request %d has empty relative URL", i)
		}
	}

	return nil
}

type SubResponseHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type BatchResponse struct {
	Code    int                 `json:"code"`
	Headers []SubResponseHeader `json:"headers"`
	Body    string              `json:"body"`
}

type BatchClient struct {
	httpClient  *http.Client
	baseURL     string
	version     *APIVersion
	maxParallel int
}

func NewBatchClient(httpClient *http.Client, version *APIVersion) *BatchClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &BatchClient{
		httpClient: httpClient,
		baseURL:    BaseFacebookURL,
		version:    version,
	}
}

func (c *BatchClient) FormatVersionURL() string {
	if c.version != nil {
		return "/" + c.version.String() + "/"
	}

	return ""
}

func authenticateRequest(r *http.Request, accessToken string) {
	r.Header.Set("Authorization", "Bearer "+accessToken)
}

func formatRequestBody(includeHeaders bool, requests []*SubRequest) (url.Values, error) {
	body, err := json.Marshal(requests)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Add("batch", string(body))
	form.Add("include_headers", strconv.FormatBool(includeHeaders))

	return form, nil
}

func (c *BatchClient) Do(ctx context.Context, req *BatchRequest, accessToken string) ([]*BatchResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if len(req.Requests) > MaxBatchSize {
		return nil, errors.BadRequest("facebook.batch.limit_exceeded", "single batch request cannot exceed 50 sub-requests")
	}

	return c.doBatch(ctx, req, req.Requests, accessToken)
}

func (c *BatchClient) DoParallel(ctx context.Context, req *BatchRequest, accessToken string) ([]*BatchResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if len(req.Requests) <= MaxBatchSize {
		return c.Do(ctx, req, accessToken)
	}

	chunks := make([][]*SubRequest, 0, (len(req.Requests)+MaxBatchSize-1)/MaxBatchSize)

	for i := 0; i < len(req.Requests); i += MaxBatchSize {
		end := min(i+MaxBatchSize, len(req.Requests))
		chunks = append(chunks, req.Requests[i:end])
	}

	results := make([][]*BatchResponse, len(chunks))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, 3)

	errCh := make(chan error, 1)

	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)

		go func(i int, chunk []*SubRequest) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() {
				<-sem
			}()

			res, err := c.doBatch(ctx, req, chunk, accessToken)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("chunk %d: %w", i, err):
					cancel()
				default:
				}
				return
			}

			results[i] = res
		}(i, chunk)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	finalResponses := make([]*BatchResponse, 0, len(req.Requests))

	for _, result := range results {
		finalResponses = append(finalResponses, result...)
	}

	return finalResponses, nil
}

func (c *BatchClient) doBatch(ctx context.Context, req *BatchRequest, requests []*SubRequest, accessToken string) ([]*BatchResponse, error) {
	body, err := formatRequestBody(req.IncludeHeaders, requests)
	if err != nil {
		return nil, errors.InternalServerError("facebook.batch.execute.format_body", "formatting JSON batch payload; err: %+v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.FormatVersionURL(), bytes.NewBufferString(body.Encode()))
	if err != nil {
		return nil, errors.InternalServerError("facebook.batch.execute.new_request", "creating new graph batch request; err: %+v", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	authenticateRequest(httpReq, accessToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.InternalServerError("facebook.batch.execute.do_request", "batch request HTTP error; err: %+v", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.InternalServerError("facebook.batch.execute.read_body", "failed to read response body: %+v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.InternalServerError(
			"facebook.batch.execute.status",
			"meta API status %d; raw response: %s",
			resp.StatusCode, string(respBytes),
		)
	}

	var response []*BatchResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, errors.InternalServerError(
			"facebook.batch.execute.decode",
			"decoding META API response; err: %+v; raw body: %s",
			err, string(respBytes),
		)
	}

	return response, nil
}
