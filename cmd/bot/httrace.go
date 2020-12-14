package main

import (
	"github.com/google/uuid"
	
	"net/http/httptest"
	"net/http/httputil"
	"net/http"
	stdlog "github.com/micro/go-micro/v2/logger"
)

type transportDump struct {
	r http.RoundTripper
	WithBody bool
}

func (d *transportDump) RoundTrip(h *http.Request) (*http.Response, error) {
	reqId, _ := uuid.NewRandom() // fmt.Sprintf("%p", h.Context())
	dump, _ := httputil.DumpRequestOut(h, d.WithBody)
	stdlog.Tracef("\t>>>>> OUTBOUND (%s) >>>>>\n\n%s\n\n", reqId, dump)
	resp, err := d.r.RoundTrip(h)
	dump, _ = httputil.DumpResponse(resp, d.WithBody)
	stdlog.Tracef("\t>>>>> RESPONSE (%s) >>>>>\n\n%s\n\n", reqId, dump)
	return resp, err
}

func init() {

	// http.DefaultTransport = &transportDump{
	// 	r: http.DefaultTransport,
	// 	WithBody: true,
	// }
}

// ContentTypeHandler wraps and returns a http.Handler, validating the request
// content type is compatible with the contentTypes list. It writes a HTTP 415
// error if that fails.
//
// Only PUT, POST, and PATCH requests are considered.
func dumpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		reqId, _ := uuid.NewRandom() // fmt.Sprintf("%p", h.Context())
		
		if dump, err := httputil.DumpRequest(r, true); err != nil {
			stdlog.Tracef("httputil.DumpRequest(error): %v", err)
		} else {
			for dump[len(dump)-1] == '\n' {
				dump = dump[:len(dump)-1]
			}
			stdlog.Tracef("\t<<<<< INBOUND (%s) <<<<<\n\n%s\n\n", reqId, dump)
		}
		
		// src: https://stackoverflow.com/questions/29319783/go-logging-responses-to-incoming-http-requests-inside-http-handlefunc
		recorder := httptest.NewRecorder()
		defer func() {

			rw := recorder.Result()
			if dump, err := httputil.DumpResponse(rw, true); err != nil {
				stdlog.Tracef("httputil.DumpResponse(error): %v", err)
			} else {
				for dump[len(dump)-1] == '\n' {
					dump = dump[:len(dump)-1]
				}
				stdlog.Tracef("\t<<<<< RESPOND (%s) <<<<<\n\n%s\n\n", reqId, dump)
			}

			for h, v := range rw.Header {
				w.Header()[h] = v
			}

			w.WriteHeader(rw.StatusCode)
			recorder.Body.WriteTo(w)

		}()

		next.ServeHTTP(recorder, r)
	})
}