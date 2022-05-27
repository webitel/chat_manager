package wrapper

import (
	"context"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/server"
)

/*func CallFromServiceId(sid string) client.CallWrapper {

	if sid == "" {
		sid = server.DefaultId
	}
	headers := metadata.Metadata{
		HeaderPrefix + "From-Id": sid,
	}

	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {
			// add headers
			metadata.MergeContext(ctx, headers, false)
			// continue call
			return next(ctx, addr, req, rsp, opts)
		}
	}
}*/

type serviceClient struct {
	client.Client

	// headers to inject
	headers metadata.Metadata
}

var (
	HeaderPrefix = "Micro-"
)

func (f *serviceClient) setHeaders(ctx context.Context) context.Context {
	// don't overwrite keys
	return metadata.MergeContext(ctx, f.headers, false)
}

func (f *serviceClient) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Call(ctx, req, rsp, opts...)
}

func (f *serviceClient) Stream(ctx context.Context, req client.Request, opts ...client.CallOption) (client.Stream, error) {
	ctx = f.setHeaders(ctx)
	return f.Client.Stream(ctx, req, opts...)
}

func (f *serviceClient) Publish(ctx context.Context, p client.Message, opts ...client.PublishOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Publish(ctx, p, opts...)
}

// FromServiceId wraps a client to inject service and auth metadata
func FromService(name, id string, c client.Client) client.Client {
	if name == "" {
		panic("wrapper.FromService: service name is missing")
	}
	if id == "" {
		id = server.DefaultId
	}
	return &serviceClient{
		c,
		metadata.Metadata{
			HeaderPrefix + "From-Id":      id,
			HeaderPrefix + "From-Service": name,
		},
	}
}
