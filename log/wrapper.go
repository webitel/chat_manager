package log

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"
	"time"

	"context"
	"strings"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/server"
	"go.opentelemetry.io/otel/trace"
)

func HandlerWrapper(debug *slog.Logger) server.HandlerWrapper {
	var seq uint64
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			tx := atomic.AddUint64(&seq, 1)

			var (
				// depth = 0..3
				level = slog.LevelDebug
			)

			debugCtx := []any{
				"req", DeferValue(func() slog.Value {
					// md := req.Header()
					md, _ := metadata.FromContext(ctx)
					head := []string{
						"Remote",
						"User-Agent",
						"From-Service",
						"Micro-From-Id",
						"Micro-From-Host",
						"Micro-From-Service",
					}
					args := make([]slog.Attr, 0, len(head)+2)
					args = append(args, slog.String("endpoint", req.Endpoint()))
					for _, key := range head {
						if vs, ok := md[key]; ok {
							args = append(args, slog.String(
								"h."+strings.ToLower(key), vs,
							))
						}
					}
					args = append(args,
						slog.String("data", JsonValue(req.Body())),
					)
					return slog.GroupValue(args...)
				}),
			}

			debug.Log(
				ctx, level, fmt.Sprintf(
					"[ RECV::REQ ] (#%d) /%s/%s",
					tx, req.Service(), req.Endpoint(),
				),
				debugCtx...,
			)

			// Serve Request
			start := time.Now()
			err := next(ctx, req, rsp)
			spent := time.Since(start)

			debugCtx = append(debugCtx,
				"res", DeferValue(func() slog.Value {
					return slog.GroupValue(
						slog.String("data", JsonValue(rsp)),
						slog.String("time", spent.Round(time.Microsecond).String()),
					)
				}),
			)

			if err != nil {
				level = slog.LevelError
				debugCtx = append(
					debugCtx, "error", err,
				)
			}

			debug.Log(
				ctx, level, fmt.Sprintf(
					"[ SEND::RES ] (#%d) /%s/%s",
					tx, req.Service(), req.Endpoint(),
				),
				debugCtx...,
			)

			return err
		}
	}
}

// JsonValue marshaling
func JsonValue(data any) string {
	jsonb, err := json.Marshal(data)
	if err != nil {
		jsonb, _ = json.Marshal(
			struct {
				Err error `json:"error"`
			}{
				Err: err,
			},
		)
	}
	return string(jsonb)
}

func CallWrapper(debug *slog.Logger) client.CallWrapper {
	var seq uint64
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {

			tx := atomic.AddUint64(&seq, 1)

			var (
				// depth = 0..3
				level = slog.LevelDebug
			)

			debugCtx := []any{
				"req", DeferValue(func() slog.Value {
					return slog.GroupValue(
						slog.String("host", addr),             // {host:port}
						slog.String("service", req.Service()), // {service}-{node.id}
						slog.String("endpoint", req.Endpoint()),
						slog.String("data", JsonValue(req.Body())),
					)
				}),
			}

			if span := trace.SpanContextFromContext(ctx); span.IsValid() {
				debugCtx = append(debugCtx, "trace.id", span.TraceID().String())
			}

			debug.Log(
				ctx, level, fmt.Sprintf(
					"[ CALL::REQ ] (#%d) /%s/%s",
					tx, req.Service(), req.Endpoint(),
				),
				debugCtx...,
			)

			// Serve Request
			start := time.Now()
			err := next(ctx, addr, req, rsp, opts)
			spent := time.Since(start)

			debugCtx = append(debugCtx,
				"res", DeferValue(func() slog.Value {
					return slog.GroupValue(
						slog.String("data", JsonValue(rsp)),
						slog.String("time", spent.Round(time.Microsecond).String()),
					)
				}),
			)

			if err != nil {
				level = slog.LevelError
				debugCtx = append(debugCtx,
					"error", err,
				)
			}

			debug.Log(
				ctx, level, fmt.Sprintf(
					"[ CALL::RES ] (#%d) /%s/%s",
					tx, req.Service(), req.Endpoint(),
				),
				debugCtx...,
			)

			return err
		}
	}
}

const LevelTrace = slog.Level(-8) // for debug1 debug2..
const LevelFatal = slog.Level(10) // for crit..

func TraceLog(log *slog.Logger, msg string, args ...any) {
	// func TraceLog(ctx context.Context, log *slog.Logger, msg string, args ...any) {
	// if ctx == nil {
	ctx := context.TODO()
	// }
	if log == nil {
		log = slog.Default()
	}
	log.Log(ctx, LevelTrace, msg, args...)
}

func FataLog(log *slog.Logger, msg string, args ...any) {
	// func FataLog(ctx context.Context, log *slog.Logger, msg string, args ...any) {
	// if ctx == nil {
	ctx := context.TODO()
	// }
	if log == nil {
		log = slog.Default()
	}
	log.Log(ctx, LevelFatal, msg, args...)
	// os.Exit(1) deffer[ed] funcs not working !!!
}

func SlogObject(obj interface{}) []slog.Attr {
	res := make([]slog.Attr, 0)

	v := reflect.ValueOf(obj)

	if v.Kind() == reflect.Ptr {
		v = reflect.Indirect(v)
	}

	t := v.Type()

	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			value := v.Field(i)

			if field.IsExported() {
				tag := field.Tag.Get("json")
				if tag == "" {
					tag = field.Name
				} else {
					if commaIdx := strings.Index(tag, ","); commaIdx != -1 {
						tag = tag[:commaIdx]
					}
				}

				if value.Kind() == reflect.Struct || value.Kind() == reflect.Map {
					res = append(res, slog.Group(tag, convertAttrsToAny(SlogObject(value.Interface()))...))
				} else {
					res = append(res, slog.Any(tag, value.Interface()))
				}
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			mapValue := v.MapIndex(key)
			if mapValue.Kind() == reflect.Struct || mapValue.Kind() == reflect.Map {
				res = append(res, slog.Group(key.String(), convertAttrsToAny(SlogObject(mapValue.Interface()))...))
			} else {
				res = append(res, slog.Any(key.String(), mapValue.Interface()))
			}
		}
	default:
		res = append(res, slog.Any("_BADKEY_", obj))
	}

	return res
}

func convertAttrsToAny(attrs []slog.Attr) []any {
	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}
	return anyAttrs
}
