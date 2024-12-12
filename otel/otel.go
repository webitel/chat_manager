package otel

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	mlog "github.com/micro/micro/v3/service/logger"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	olog "go.opentelemetry.io/otel/log"

	slogutil "github.com/webitel/webitel-go-kit/otel/log/bridge/slog"

	// wlog "webitel.go/service/log"

	sdk "github.com/webitel/webitel-go-kit/otel/sdk"

	"go.opentelemetry.io/otel"

	// -------------------- plugin(s) -------------------- //
	_ "github.com/webitel/webitel-go-kit/otel/sdk/log/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/log/stdout"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/metric/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/metric/stdout"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/trace/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/trace/stdout"

	// -------------------- bridge(s) -------------------- //
	wlog "github.com/webitel/chat_manager/log"
	otelmicro "github.com/webitel/chat_manager/otel/log/bridge/micro"
	logb "github.com/webitel/webitel-go-kit/otel/log/bridge/logr"
)

var (
	// Disable the OpenTelemetry SDK for all signals
	disabled = false
	// init ; once !
	initOnce sync.Once

	shutdown func(context.Context) error
	shutOnce sync.Once
	// indicates whether OTEL_LOGS_EXPORTER= provided
	// indicates whether otel.Logger() available
	bridge bool
)

// Disabled reports whether OpenTelemetry SDK is disabled for all signals.
func Disabled() bool {
	return disabled
}

// Shutdown OpenTelemetry SDK environment
func Shutdown(ctx context.Context) (err error) {
	shutOnce.Do(func() {
		// fmt.Println("OTel.Shutdown(!)")
		if shutdown == nil {
			return
		}
		err = shutdown(ctx)
		shutdown = nil
	})
	return // err
}

func configure(ctx context.Context, opts []sdk.Option) (err error) {

	disabled, _ = strconv.ParseBool(
		os.Getenv("OTEL_SDK_DISABLED"),
	)

	if disabled {
		return //
	}

	opts = append([]sdk.Option{
		// default: internals
		sdk.WithSetLevel(otelLogLevel),
		sdk.WithLogBridge(otelLogBridge),
	}, opts...)

	shutdown, err = sdk.Configure(
		ctx, opts...,
	)

	if err != nil {
		defer Shutdown(ctx)
		// otel.Handle(err)
		// os.Exit(1)
		return err
	}

	return nil
}

// Configure OpenTelemetry SDK components ...
func Configure(ctx context.Context, opts ...sdk.Option) (err error) {

	initOnce.Do(func() {
		err = configure(ctx, opts)
	})

	return err
}

// otelLogLevel sets the logger's (used internally to opentelemetry) severity level.
//
// https://github.com/open-telemetry/opentelemetry-go/blob/v1.29.0/internal/global/internal_logging.go#L33
//
// To see Warn messages use a logger with `l.V(1).Enabled() == true`
// To see Info messages use a logger with `l.V(4).Enabled() == true`
// To see Debug messages use a logger with `l.V(8).Enabled() == true`
//
// https://github.com/open-telemetry/opentelemetry-go/blob/v1.29.0/internal/global/internal_logging.go#L20
func otelLogLevel(minLevel olog.Severity) {

	var (
		logger logr.Logger
	)
	if !bridge {
		// default: Stdout(!)
		// sdk.SetLogLevel(set)
		logger = logr.New(
			logb.NewSlogSink(
				slog.Default().Handler(),
				func(v int) slog.Level {
					level := logb.OtelVerbosity(v)
					if level < minLevel {
						// disable: olog.Severity(0); slog.Level(-9)
						level = olog.SeverityUndefined
					}
					return logb.SeverityLevel(level)
				},
			),
		)
	} else {
		// otel.Logger => otel/log/global.Provider
		logger = logb.NewLogger(
			"otel", func(conf *logb.Options) {
				// conf.Provider = global.GetLoggerProvider()
				// conf.Version = ""
				// conf.SchemaURL = ""
				// conf.SeverityError = int(otelog.SeverityError)
				// According to https://pkg.go.dev/go.opentelemetry.io/otel/internal/global#SetLogger
				conf.Severity = func(v int) olog.Severity {
					level := logb.OtelVerbosity(v)
					if level < minLevel {
						// disable: olog.Severity(0);
						level = olog.SeverityUndefined
					}
					return level
				}
			},
		)
	}
	// NOTE: logger used internally to opentelemetry.
	otel.SetLogger(logger)
	// logger.V(4).Info("OTEL_LOG_LEVEL=" + minLevel.String())
}

func otelLogBridge() {
	// OTEL_LOGS_EXPORTER= specified !
	bridge = true

	// log/slog
	stdLog := slog.New(slogutil.WithLevel(
		wlog.Verbose(), otelslog.NewHandler("slog"),
	))
	slog.SetDefault(stdLog)
	stdLog.Info("OTel log/bridge",
		"verbose", wlog.Verbose().String(),
		"package", "log/slog",
	)

	// github.com/micro/micro/v3/service/logger
	microLog := otelmicro.NewLogger(
		"micro",
	)
	microLog.Init(func(opts *mlog.Options) {
		*opts = mlog.DefaultLogger.Options()
	})
	mlog.DefaultLogger = microLog
	mlog.Infof("OTel log/bridge ; verbose=%s ; package=github.com/micro/micro/v3/service/logger", microLog.Options().Level)
	// more here ...
}
