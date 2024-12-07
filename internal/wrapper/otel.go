package wrapper

import (
	"context"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context/metadata"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func OtelMicroCall(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) (err error) {
		// metadata.MergeContext
		head, _ := metadata.FromContext(ctx)
		patch := make(metadata.Metadata, len(head))
		for h, v := range head {
			patch[h] = v // clone
		}
		propagators := otel.GetTextMapPropagator()
		propagators.Inject(ctx, propagation.MapCarrier(patch))
		ctx = metadata.NewContext(ctx, patch)
		// DO: CALL(!)
		err = next(ctx, addr, req, rsp, opts)
		return err
	}
}
