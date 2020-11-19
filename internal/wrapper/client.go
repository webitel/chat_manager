package wrapper

import (

	"context"

	"github.com/micro/go-micro/v2/server"
	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/registry"
	"github.com/micro/go-micro/v2/metadata"
	"github.com/micro/go-micro/v2/util/wrapper"
)

func CallFromServiceId(sid string) client.CallWrapper {
	
	if sid == "" {
		sid = server.DefaultId
	}
	headers := metadata.Metadata{
		wrapper.HeaderPrefix + "From-Id": sid,
	}
	
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
			// add headers
			metadata.MergeContext(ctx, headers, false)
			// continue call
			return next(ctx, node, req, rsp, opts)
		}
	}
}

type fromServiceIdWrapper struct {
	client.Client

	// headers to inject
	headers metadata.Metadata
}

var (
	HeaderPrefix = "Micro-"
)

func (f *fromServiceIdWrapper) setHeaders(ctx context.Context) context.Context {
	// don't overwrite keys
	return metadata.MergeContext(ctx, f.headers, false)
}

func (f *fromServiceIdWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Call(ctx, req, rsp, opts...)
}

func (f *fromServiceIdWrapper) Stream(ctx context.Context, req client.Request, opts ...client.CallOption) (client.Stream, error) {
	ctx = f.setHeaders(ctx)
	return f.Client.Stream(ctx, req, opts...)
}

func (f *fromServiceIdWrapper) Publish(ctx context.Context, p client.Message, opts ...client.PublishOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Publish(ctx, p, opts...)
}

// FromServiceId wraps a client to inject service and auth metadata
func FromServiceId(sid string, c client.Client) client.Client {
	if sid == "" {
		sid = server.DefaultId
	}
	return &fromServiceIdWrapper{
		c,
		metadata.Metadata{
			wrapper.HeaderPrefix + "From-Id": sid,
		},
	}
}
