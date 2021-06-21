package main

import (
	"bufio"
	"io"
	"net"

	"github.com/google/uuid"

	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	stdlog "github.com/micro/go-micro/v2/logger"
)

type transportDump struct {
	r http.RoundTripper
	WithBody bool
}

func (d *transportDump) RoundTrip(req *http.Request) (*http.Response, error) {

	// region: DUMP Request
	reqId, _ := uuid.NewRandom() // fmt.Sprintf("%p", h.Context())
	dump, err := httputil.DumpRequestOut(req, d.WithBody && req.ContentLength > 0)
	
	tracef := stdlog.Tracef
	if err != nil {
		tracef = stdlog.Errorf
		dump = []byte("httputil.DumpRequestOut: "+ err.Error())
	}
	tracef("\t>>>>> OUTBOUND (%s) >>>>>\n\n%s\n\n", reqId, dump)
	// endregion
	
	// PERFORM !
	resp, err := d.r.RoundTrip(req)
	
	if err != nil {
		tracef = stdlog.Errorf
		dump = []byte("error: "+ err.Error())
		tracef("\t>>>>> RESPONSE (%s) >>>>>\n\n%s\n\n", reqId, dump)
		// Failure(!)
		return resp, err
	}

	// region: DUMP Response
	dump, err = httputil.DumpResponse(resp, d.WithBody && resp.ContentLength > 0)
	
	tracef = stdlog.Tracef
	if err != nil {
		tracef = stdlog.Errorf
		dump = []byte("httputil.DumpResponse: "+ err.Error())
	}
	tracef("\t>>>>> RESPONSE (%s) >>>>>\n\n%s\n\n", reqId, dump)
	// endregion

	// Success(!)
	return resp, err
}

func init() {

	// http.DefaultTransport = &transportDump{
	// 	r: http.DefaultTransport,
	// 	WithBody: true,
	// }
}

type response struct {
	http.ResponseWriter
	*httptest.ResponseRecorder
}

func (w *response) Header() http.Header {
	return w.ResponseRecorder.Header()
}

func (w *response) WriteHeader(code int) {
	w.ResponseRecorder.WriteHeader(code)
}

func (w *response) Write(b []byte) (int, error) {
	return w.ResponseRecorder.Write(b)
}

type conn struct {
	net.Conn
	r io.Reader
	w io.Writer
}

func (w *response) Hijack() (c net.Conn, rw *bufio.ReadWriter, err error) {
	hw, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	c, rw, err = hw.Hijack()
	// if err == nil {
	// 	c = &conn{
	// 		Conn: c,
	// 		r: io.TeeReader(c, os.Stdout),
	// 		w: io.MultiWriter(c, os.Stdout),
	// 	}
	// }
	return
}

func (c *conn) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *conn) Write(p []byte) (n int, err error){
	return c.w.Write(p)
}

// ContentTypeHandler wraps and returns a http.Handler, validating the request
// content type is compatible with the contentTypes list. It writes a HTTP 415
// error if that fails.
//
// Only PUT, POST, and PATCH requests are considered.
func dumpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		reqId, _ := uuid.NewRandom() // fmt.Sprintf("%p", h.Context())
		
		if dump, err := httputil.DumpRequest(r, r.ContentLength > 0); err != nil {
			stdlog.Tracef("httputil.DumpRequest(error): %v", err)
		} else {
			for dump[len(dump)-1] == '\n' {
				dump = dump[:len(dump)-1]
			}
			stdlog.Tracef("\t<<<<< INBOUND (%s) <<<<<\n\n%s\n\n", reqId, dump)
		}
		
		// src: https://stackoverflow.com/questions/29319783/go-logging-responses-to-incoming-http-requests-inside-http-handlefunc
		wr := &response{
			ResponseRecorder: httptest.NewRecorder(),
			ResponseWriter: w,
		}
		// recorder := httptest.NewRecorder()
		defer func() {

			rw := wr.Result() // recorder.Result()
			if dump, err := httputil.DumpResponse(rw, rw.ContentLength > 0); err != nil {
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
			// recorder.Body.WriteTo(w)
			_, _ = wr.Body.WriteTo(w)

		}()

		// next.ServeHTTP(recorder, r)
		next.ServeHTTP(wr, r)
	})
}