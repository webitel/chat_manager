package log

import (
	"log/slog"
	"os"
	"reflect"
	"time"

	"context"
	"strings"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/server"
)

func HandlerWrapper(log *slog.Logger) server.HandlerWrapper {
	return func(next server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			md, _ := metadata.FromContext(ctx)

			trace := log.With(
				slog.String("endpoint", req.Endpoint()),
				slog.Any("body", SlogObject(req.Body())),
			)
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
					trace = trace.With(slog.String(strings.ToLower(key), v)) // chaining
				}
			}

			// span.Trace().Msg("<<<<< SERVE <<<<<<")

			// Serve Request
			start := time.Now()
			err := next(ctx, req, rsp)
			spent := time.Since(start)

			if err != nil {
				trace.With(
					slog.Duration("spent", spent),
					slog.Any("error", err),
				).Error("<<<<< SERVE <<<<<")
			} else {
				trace.With(
					slog.Duration("spent", spent),
				).Debug("<<<<< SERVE <<<<<")
			}
			// Msg("----- SERVED -----")

			return err
		}
	}
}

func CallWrapper(log *slog.Logger) client.CallWrapper {
	return func(next client.CallFunc) client.CallFunc {
		return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {

			span := log.With(
				slog.String("peer", addr),             // {host:port}
				slog.String("service", req.Service()), // {service}-{node.id}
				slog.String("endpoint", req.Endpoint()),
				slog.Any("body", SlogObject(req.Body())),
			)

			// span.Trace().Msg(">>>>> CALL >>>>>>>")

			// Serve Request
			start := time.Now()
			err := next(ctx, addr, req, rsp, opts)
			spent := time.Since(start)

			if err != nil {
				span.With(
					slog.Duration("spent", spent),
				).Error(">>>>> CALL >>>>>>", slog.Any("error", err))
			} else {
				span.With(
					slog.Duration("spent", spent),
				).Debug(">>>>> CALL >>>>>>")
			}
			// Msg("----- CALLED -----")

			return err
		}
	}
}

const LevelTrace = slog.Level(-8) // for debug1 debug2..
const LevelFatal = slog.Level(10) // for crit..

func TraceLog(log *slog.Logger, msg string, args ...any) {
	log.Log(context.Background(), LevelTrace, msg, args...)
}

func FataLog(log *slog.Logger, msg string, args ...any) {
	log.Log(context.Background(), LevelFatal, msg, args...)
	os.Exit(1)
}

func SlogObject(obj interface{}) []slog.Attr {
	res := make([]slog.Attr, 0, 0)

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
				key := field.Tag.Get("json")
				if key == "" {
					key = field.Name
				}
				res = append(res, slog.Any(key, value))
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			res = append(res, slog.Any(key.String(), v.MapIndex(key)))
		}
	default:
		res = append(res, slog.Any("_BADKEY_", obj))
	}

	return res
}
