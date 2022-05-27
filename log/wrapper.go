package log

import (
	"time"

	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/server"
)

func HandlerWrapper(log *zerolog.Logger) server.HandlerWrapper {
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			md, _ := metadata.FromContext(ctx)

			trace := log.With()
			// populate headers
			// for key, value := range md {
			// 	log.Str(key, value)
			// }
			for _, key := range []string{
				"Remote",
				"User-Agent",
				"From-Service",
				"Micro-From-Id",
				"Micro-From-Host",
				"Micro-From-Service",
			} {
				if v, ok := md[key]; ok {
					trace = trace.Str(strings.ToLower(key), v) // chaining
				}
			}

			trace = trace.Str("endpoint", req.Endpoint())
			trace = trace.EmbedObject(serverRequest{req})

			span := trace.Logger()

			// span.Trace().Msg("<<<<< SERVE <<<<<<")

			// Serve Request
			start := time.Now()
			err := next(ctx, req, rsp)
			spent := time.Since(start)

			var event *zerolog.Event

			if err == nil {
				event = span.Debug()
			} else {
				event = span.Error().Err(err)
			}

			event.
				Str("spent", spent.String()).
				Msg("<<<<< SERVE <<<<<") // Msg("----- SERVED -----")

			return err
		}
	}
}

type serverRequest struct {
	server.Request
}

func (m serverRequest) MarshalZerologObject(e *zerolog.Event) {
	data, err := json.Marshal(m.Request.Body())
	if err != nil {
		e.AnErr("req", err)
	} else {
		e.RawJSON("req", data)
	}
}

func CallWrapper(log *zerolog.Logger) client.CallWrapper {
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {

			span := log.With().
				Str("peer", addr).             // {host:port}
				Str("service", req.Service()). // {service}-{node.id}
				Str("endpoint", req.Endpoint()).
				EmbedObject(clientRequest{req}).
				Logger()

			// span.Trace().Msg(">>>>> CALL >>>>>>>")

			// Serve Request
			start := time.Now()
			err := next(ctx, addr, req, rsp, opts)
			spent := time.Since(start)

			var event *zerolog.Event
			if err == nil {
				event = span.Trace()
			} else {
				event = span.Error().Err(err)
			}

			event.
				Str("spent", spent.String()).
				Msg(">>>>> CALL >>>>>>") // Msg("----- CALLED -----")

			return err
		}
	}
}

type clientRequest struct {
	client.Request
}

func (c clientRequest) MarshalZerologObject(e *zerolog.Event) {
	data, err := json.Marshal(c.Request.Body())
	if err != nil {
		e.AnErr("req", err)
	} else {
		e.RawJSON("req", data)
	}
}
