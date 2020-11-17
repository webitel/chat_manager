package log

import (
	"context"
	"github.com/rs/zerolog"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/server"
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/metadata"
	
)

func HandlerWrapper(log *zerolog.Logger) server.HandlerWrapper {
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			
			md, _ := metadata.FromContext(ctx)
			
			// Serve Request
			err := next(ctx, req, rsp)
			
			var e *zerolog.Event
			if err == nil {
				e = log.Debug()
			} else {
				e = log.Error().Err(err)
			}

			// populate headers
			// for key, value := range md {
			// 	log.Str(key, value)
			// }
			for _, key := range []string {
				"Remote",
				"User-Agent",
				"From-Service",
				"Micro-From-Service",
			} {
				if v, ok := md[key]; ok {
					e = e.Str(key, v) // chaining
				}
			}

			e.Str("Endpoint", req.Endpoint()).Msg("SERVED")

			return err
		}
	}
}

func CallWrapper(log *zerolog.Logger) client.CallWrapper {
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
			// Serve Request
			err := next(ctx, node, req, rsp, opts)
			
			var e *zerolog.Event
			if err == nil {
				e = log.Trace()
			} else {
				e = log.Error().Err(err)
			}

			e.
				// Str("service", req.Service()).
				Str("api", req.Endpoint()).
				
				Str("node", node.Id). // {service}-{node.id}
				Str("addr", node.Address). // {host:port}
				
				Msg("CALLED")

			return err
		}
	}
}