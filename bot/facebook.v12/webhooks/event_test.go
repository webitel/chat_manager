package webhooks

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// POST /chat/ws8/messenger HTTP/1.1
// Host: dev.webitel.com
// Connection: close
// Accept: */*
// Accept-Encoding: deflate, gzip
// Connection: close
// Content-Length: 308
// Content-Type: application/json
// Facebook-Api-Version: v12.0
// P-Mi-Om: FgIA
// User-Agent: facebookexternalua
// X-Forwarded-For: 173.252.127.5
// X-Forwarded-Proto: https
// X-Hub-Signature: sha1=1747201b9166b6b098e71f208c80608ee2319ce3
// X-Real-Ip: 173.252.127.5

// {"object":"page","entry":[{"id":"109029271668032","time":1641911658210,"messaging":[{"sender":{"id":"4714860478621687"},"recipient":{"id":"109029271668032"},"timestamp":1641911238525,"message":{"mid":"m_jv_8uAX-09NuJMIlMr9SPwXZZNY8-VOVSoeZ7NXBqXGePgSC4Ut9TowGYr7vFNwcIXbeMIJINrXORck9IIeU2Q","text":"Yo"}}]}]}

type readCloser struct {
	io.Reader
}

func (r *readCloser) Close() error {
	return nil
}

func TestReader_Close(t *testing.T) {

	req := &http.Request{
		Header: http.Header{
			"X-Hub-Signature": {"sha1=1747201b9166b6b098e71f208c80608ee2319ce3"},
		},
		Body: &readCloser{
			Reader: strings.NewReader(`{"object":"page","entry":[{"id":"109029271668032","time":1641911658210,"messaging":[{"sender":{"id":"4714860478621687"},"recipient":{"id":"109029271668032"},"timestamp":1641911238525,"message":{"mid":"m_jv_8uAX-09NuJMIlMr9SPwXZZNY8-VOVSoeZ7NXBqXGePgSC4Ut9TowGYr7vFNwcIXbeMIJINrXORck9IIeU2Q","text":"Yo"}}]}]}`),
		},
	}

	// type fields struct {
	// 	v       []byte
	// 	r       io.Reader
	// 	h       hash.Hash
	// 	c       io.Closer
	// 	Request *http.Request
	// }
	tests := []struct {
		name    string
		// fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "general",

		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if r, err := EventReader([]byte("e9f9cbd2f09e5a09cca2f40824902e31"), req); (err != nil) != tt.wantErr {
				t.Errorf("EventReader() error = %v, wantErr %v", err, tt.wantErr)
			} else if err := r.Close(); (err != nil) != tt.wantErr {
				t.Errorf("Reader.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
