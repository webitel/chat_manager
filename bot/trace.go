package bot

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/textproto"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

var (
	h1Traceid = textproto.CanonicalMIMEHeaderKey("X-Webitel-Traceid")
	// h2Traceid = strings.ToLower(h1Traceid)
)

func traceHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// has valid span attached by the instrumentation library ?
		var (
			ctx     = r.Context()
			span    = trace.SpanFromContext(ctx)
			spanCtx = span.SpanContext()
		)

		var (
			traceLvl = (slog.LevelDebug - 4) // TRACE
			traceLog = slog.Default()
		)

		if traceLog.Enabled(ctx, traceLvl) {
			oid := fmt.Sprintf("%p", ctx)
			traceLog = traceLog.With(
				slog.String("http.rpc.id", oid),
			)
			if dump, err := httputil.DumpRequest(r, true); err != nil {
				traceLog.Log(ctx, slog.LevelError, "httputil.DumpRequest", "error", err)
			} else {
				for dump[len(dump)-1] == '\n' {
					dump = dump[:len(dump)-1]
				}
				traceLog.Log(ctx, traceLvl, fmt.Sprintf("\n\n%s\n\n", dump))
			}
			// response
			// src: https://stackoverflow.com/questions/29319783/go-logging-responses-to-incoming-http-requests-inside-http-handlefunc
			recorder := httptest.NewRecorder()
			response := w // original
			defer func() {

				res := recorder.Result()
				if _, h2 := w.(http.Pusher); h2 {
					// Upgrade: h2c
					res.Proto = "HTTP/2.0"
					res.ProtoMajor = 2
					res.ProtoMinor = 0
				} else {
					res.Proto = r.Proto
					res.ProtoMajor = r.ProtoMajor
					res.ProtoMinor = r.ProtoMinor
				}
				if dump, err := httputil.DumpResponse(res, true); err != nil {
					traceLog.Log(ctx, slog.LevelError, "httputil.DumpResponse", "error", err)
				} else {
					for dump[len(dump)-1] == '\n' {
						dump = dump[:len(dump)-1]
					}
					traceLog.Log(ctx, traceLvl, fmt.Sprintf("\n\n%s\n\n", dump))
				}

				for h, v := range res.Header {
					response.Header()[h] = v
				}

				response.WriteHeader(res.StatusCode)
				recorder.Body.WriteTo(response)

			}()
			// substitute
			w = recorder
		}

		if spanCtx.IsValid() {

			// // HTTP Request [http.request.header.<key>]
			// // https://opentelemetry.io/docs/specs/semconv/attributes-registry/http/
			// var (
			// 	e    int
			// 	head = make([]attribute.KeyValue, len(r.Header))
			// )
			// for h, vs := range r.Header {
			// 	att := &head[e]
			// 	att.Key = attribute.Key(
			// 		"http.request.header." + strings.ToLower(h),
			// 	)
			// 	att.Value = attribute.StringSliceValue(vs)
			// 	e++
			// }
			// span.SetAttributes(head...)

			// HTTP Response
			w.Header().Add(h1Traceid, spanCtx.TraceID().String())
		}

		// invoke
		next.ServeHTTP(w, r)
	})
}

func traceMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(
		traceHandler(next), "", // "server",
		otelhttp.WithPublicEndpointFn(func(_ *http.Request) bool { return true }), // always root span !
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
		otelhttp.WithSpanNameFormatter(func(operation string, req *http.Request) string {
			return req.Method + " " + req.URL.RequestURI() + " " + req.Proto
		}),
		// otelhttp.WithClientTrace(f func(context.Context) *httptrace.ClientTrace),
		// otelhttp.WithFilter(f otelhttp.Filter),
		// otelhttp.WithMessageEvents(events ...event),
		// otelhttp.WithMeterProvider(provider metric.MeterProvider),
		// otelhttp.WithPropagators(ps propagation.TextMapPropagator),
		// --- NOTE: Span should always be treated as a root Span ! ---
		// otelhttp.WithPublicEndpoint(),
		// otelhttp.WithPublicEndpointFn(fn func(*http.Request) bool),
		// ------------------------------------------------------------
		// otelhttp.WithServerName(server string),
		// otelhttp.WithSpanNameFormatter(f func(operation string, r *http.Request) string),
		// otelhttp.WithSpanOptions(opts ...trace.SpanStartOption),
		// otelhttp.WithTracerProvider(provider trace.TracerProvider),
	)
}
