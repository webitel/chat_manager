package micro

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	micro "github.com/micro/micro/v3/service/logger"
	"go.opentelemetry.io/otel/log"

	"github.com/webitel/webitel-go-kit/otel/log/bridge"
)

type Options struct {
	Provider   log.LoggerProvider
	Attributes []log.KeyValue
}

type Logger struct {
	sync.RWMutex
	opts   micro.Options
	bridge *bridge.Handler
}

func micro2slogAttrs(fields map[string]any) (attrs []slog.Attr) {
	n := len(fields)
	if n == 0 {
		return nil
	}
	attrs = make([]slog.Attr, 0, n)
	for k, vs := range fields {
		attrs = append(attrs, slog.Attr{})
		add := &attrs[len(attrs)-1]
		add.Key = k
		switch v := vs.(type) {
		case int:
			add.Value = slog.IntValue(v)
		case int64:
			add.Value = slog.Int64Value(v)
		case uint64:
			add.Value = slog.Int64Value(int64(v))
		default:
			add.Value = slog.StringValue(fmt.Sprint(vs))
		}
	}
	return
}

func convertFields(fields map[string]any) (attrs []log.KeyValue) {
	n := len(fields)
	if n == 0 {
		return nil
	}
	attrs = make([]log.KeyValue, 0, n)
	for k, v := range fields {
		attrs = append(attrs, log.KeyValue{
			Key: k, Value: bridge.Value(v),
		})
	}
	return
}

// Init(opts...) should only overwrite provided options
func (h *Logger) Init(opts ...micro.Option) error {
	c := &h.opts
	if c.Context == nil {
		c.Context = context.Background()
	}
	for _, opt := range opts {
		opt(c)
	}
	if fields := c.Fields; len(fields) > 0 {
		h.bridge = h.bridge.WithAttrs(
			convertFields(fields)...,
		)
	}
	return nil
}

func (h *Logger) String() string {
	return "otel"
}

// Fields set fields to always be logged
func (h *Logger) Fields(fields map[string]interface{}) micro.Logger {
	n := len(fields)
	if n == 0 {
		return h
	}
	h.Lock()
	defer h.Unlock()
	// h.opts.Fields = copyFields(fields)
	// h.bridge.WithAttrs(convertFields(fields)...)
	// return h
	h2 := h // self
	c2 := &h2.opts
	if len(c2.Fields) == 0 {
		c2.Fields = copyFields(fields)
	} else {
		for k, v := range fields {
			c2.Fields[k] = v
		}
	}
	h2.bridge = h.bridge.WithAttrs(convertFields(fields)...)
	// h2 := &Logger{
	// 	opts:   h.opts,
	// 	bridge: h.bridge.WithAttrs(convertFields(fields)...),
	// }
	// c2 := &h2.opts
	// if len(c2.Fields) > 0 {
	// 	c2.Fields = copyFields(c2.Fields)
	// }
	// if c2.Fields == nil {
	// 	c2.Fields = copyFields(fields)
	// } else {
	// 	for k, v := range fields {
	// 		c2.Fields[k] = v
	// 	}
	// }
	return h2
}

func copyFields(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// logCallerfilePath returns a package/file:line description of the caller,
// preserving only the leaf directory name and file name.
func logCallerfilePath(loggingFilePath string) string {
	// To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	idx := strings.LastIndexByte(loggingFilePath, '/')
	if idx == -1 {
		return loggingFilePath
	}
	idx = strings.LastIndexByte(loggingFilePath[:idx], '/')
	if idx == -1 {
		return loggingFilePath
	}
	return loggingFilePath[idx+1:]
}

func micro2slogLevel(level micro.Level) slog.Level {
	switch level {
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	case micro.TraceLevel:
		return slog.LevelDebug - 4
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	case micro.DebugLevel:
		return slog.LevelDebug
	// InfoLevel is the default logging priority.
	// General operational entries about what's going on inside the application.
	case micro.InfoLevel:
		return slog.LevelInfo
	// WarnLevel level. Non-critical entries that deserve eyes.
	case micro.WarnLevel:
		return slog.LevelWarn
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	case micro.ErrorLevel:
		return slog.LevelError
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. highest level of severity.
	case micro.FatalLevel:
		return slog.LevelError + 4
	}
	return slog.LevelInfo
}

func slogLevel(v micro.Level) slog.Level {
	return slog.Level(v) * 4 // e.g: INFO, INFO+1, INFO+2, INFO+3
}

func otelSeverity(v micro.Level) log.Severity {
	const delta = int(log.SeverityInfo) // - log.Severity(slog.LevelInfo)
	return log.Severity(int(slogLevel(v)) + delta)
}

func (h *Logger) head(v micro.Level) (e log.Record) {
	e.SetTimestamp(time.Now())
	e.SetSeverity(otelSeverity(v))
	e.SetSeverityText(v.String())
	return
}

func (h *Logger) emit(ctx context.Context, rec log.Record, msg string) {
	// rec.SetObservedTimestamp(
	// 	rec.Timestamp(),
	// )
	rec.AddAttributes(
		h.bridge.Attributes.List()...,
	)
	rec.SetBody(
		log.StringValue(msg),
	)
	h.bridge.Emit(ctx, rec)
}

func (h *Logger) Log(level micro.Level, data ...any) {
	if !h.opts.Level.Enabled(level) {
		// front: disabled
		return
	}
	ctx := h.opts.Context
	event := h.head(level)
	if !h.bridge.Enabled(ctx, event) {
		// back: disabled
		return
	}
	h.emit(ctx, event, fmt.Sprint(data...))
}

func (h *Logger) Logf(level micro.Level, format string, args ...interface{}) {
	if !h.opts.Level.Enabled(level) {
		// front: disabled
		return
	}
	ctx := h.opts.Context
	rec := h.head(level)
	if !h.bridge.Enabled(ctx, rec) {
		// back: disabled
		return
	}
	h.emit(ctx, rec, fmt.Sprintf(format, args...))
}

func (l *Logger) Options() micro.Options {
	// not guard against options Context values
	l.RLock()
	opts := l.opts // shallowcopy
	opts.Fields = copyFields(l.opts.Fields)
	l.RUnlock()
	return opts
}

// NewLogger builds a new logger based on options
func NewLogger(name string, opts ...bridge.Option) micro.Logger {

	c := micro.Options{
		Level:           micro.InfoLevel,
		Fields:          make(map[string]any),
		Out:             os.Stderr,
		CallerSkipCount: 2,
		Context:         context.Background(),
	}

	h := &Logger{
		opts:   c,
		bridge: bridge.NewHandler(name, opts...),
	}
	// if err := h.Init(opts...); err != nil {
	// 	h.Log(micro.FatalLevel, err)
	// }
	return h
}
