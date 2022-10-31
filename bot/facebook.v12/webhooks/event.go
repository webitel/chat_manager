package webhooks

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// https://developers.facebook.com/docs/graph-api/webhooks/getting-started#event-notifications
type Event struct {
	// The object's type (e.g., user, page, etc.)
	Object string `json:"object,omitempty"`
	// An array containing an object describing the changes.
	// Multiple changes from different objects that are of the same type
	// may be batched together.
	Entry interface{} `json:"entry,omitempty"`
}

// Entry base content
type Entry struct {
	// The object's ID
	ObjectID string `json:"id,omitempty"`
	// A UNIX timestamp indicating when the Event Notification was sent (not when the change that triggered the notification occurred).
	Timestamp int64 `json:"time,omitempty"`
	// An array containing an object describing the changed fields and their new values.
	// Only included if you enable the Include Values setting
	// when configuring the Webhooks product in your app's App Dashboard.
	Changes []*FieldValue
	// An array of strings indicating the names of the fields that have been changed. Only included if you disable the Include Values setting when configuring the Webhooks product in your app's App Dashboard.
	ChangedFields []string `json:"changed_fields,omitempty"`
	// // An array containing an object describing the changed fields and their new values.
	// // Only included if you enable the `Include Values` setting
	// // when configuring the Webhooks product in your app's App Dashboard.
	// Changes []interface{}  `json:"changes,omitempty"`
	// Extersion fields below ...
}

// FieldValue describe field value(s) changes
type FieldValue struct {
	// Name of the updated field
	Field string `json:"field"`
	// The result value(s)
	Value json.RawMessage `json:"value,omitempty"`
}

// GetValue tries to unmarshal Value into v.
func (e FieldValue) GetValue(v interface{}) error {
	return json.Unmarshal(e.Value, v)
}

type eventReader struct {
	// X-Hub-Signature: alg=hex; decoded raw sum bytes
	s []byte
	// hmac(alg, key)
	h hash.Hash
	// io.TeeReader(body, hmac)
	r io.Reader
	// req.Body.(io.Closer)
	c io.Closer
}

var _ io.ReadCloser = (*eventReader)(nil)

// https://developers.facebook.com/docs/messenger-platform/webhook#security
func EventReader(key []byte, req *http.Request) (io.ReadCloser, error) {

	var (
		algo = sha1.New
		size = sha1.Size
		hsum = req.Header.Get("X-Hub-Signature")
	)
	// Detect signature algorithm ...
	if eq := strings.IndexByte(hsum, '='); 0 < eq && eq < len(hsum) {
		switch h := strings.ToLower(hsum[0:eq]); h {
		case "sha1": // Default !
		default:
			return nil, fmt.Errorf("webhook: signature %s algorithm not supported", h)
		}
		hsum = hsum[eq+1:]
	}
	// Decode raw signature from HEX sequence
	if c := len(hsum); hex.DecodedLen(c) != size {
		return nil, fmt.Errorf("webhook: signature is invalid or missing")
	}
	// Decode signature HEX sequence to raw bytes
	sum, err := hex.DecodeString(hsum)
	if err != nil {
		return nil, fmt.Errorf("webhook: signature is invalid HEX sequence")
	}

	hash := hmac.New(algo, key)

	return &eventReader{
			s: sum, // []byte(s),
			h: hash,
			r: io.TeeReader(req.Body, hash),
			c: req.Body,
		},
		nil
}

func (r *eventReader) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

func (r *eventReader) Close() error {
	// R/W Full body content to be able to calc valid signature
	_, err := ioutil.ReadAll(r.r)
	if err != nil {
		return err
	}
	// Close underlaying body content
	err = r.c.Close()
	if err != nil {
		return err
	}
	// https://developers.facebook.com/docs/messenger-platform/webhook#security
	if sum := r.h.Sum(nil); !hmac.Equal(sum, r.s) {
		// err = fmt.Errorf("X-Hub-Signature: got = %x; want = %x", r.s, sum)
		err = fmt.Errorf("webhook: signature is invalid")
	}
	return err
}
