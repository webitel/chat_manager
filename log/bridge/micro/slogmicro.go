package micro

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	mlog "github.com/micro/micro/v3/service/logger"
)

type Logger struct {
	conf Options
	slog *slog.Logger
}

func New(opts ...Option) *Logger {
	h := &Logger{
		conf: mlog.Options{
			Level:           mlog.InfoLevel,
			Fields:          nil, // map[string]interface{}{},
			Out:             os.Stdout,
			CallerSkipCount: 2,
			Context:         context.Background(),
		},
	}
	h.Init(opts...)
	return h
}

var _ mlog.Logger = (*Logger)(nil)

// Init initialises options
func (h *Logger) Init(opts ...mlog.Option) error {
	conf := &h.conf
	for _, setup := range opts {
		setup(conf)
	}
	back := h.slog
	if back == nil {
		back = slog.Default()
	}
	if len(conf.Fields) > 0 {
		back = back.With(
			convertAny(conf.Fields)...,
		)
	}
	h.slog = back
	return nil
}

// The Logger options
func (c *Logger) Options() mlog.Options {
	if c != nil {
		return c.conf
	}
	// default
	return mlog.Options{}
}

// Fields set fields to always be logged
func (h *Logger) Fields(fields map[string]any) mlog.Logger {
	if len(fields) == 0 {
		return h
	}
	h2 := (*h)
	conf := &h2.conf
	conf.Fields = combineFields(
		conf.Fields, fields,
	)
	back := h2.slog
	if back == nil {
		back = slog.Default()
	}
	back = back.With(
		convertAny(conf.Fields)...,
	)
	h2.slog = back
	return &h2
}

// Log writes a log entry
func (h *Logger) Log(v mlog.Level, args ...any) {
	// TODO decide does we need to write message if log level not used?
	// [CHECK]: front enabled ?
	if !h.conf.Level.Enabled(v) {
		return
	}
	var (
		level = Level(v)
		ctx   = context.Background()
	)
	// [CHECK]: back enabled ?
	if !h.slog.Enabled(ctx, level) {
		return
	}
	var pc uintptr
	// if !internal.IgnorePC {
	// 	var pcs [1]uintptr
	// 	// skip [runtime.Callers, this function, this function's caller]
	// 	runtime.Callers(3, pcs[:])
	// 	pc = pcs[0]
	// }
	event := slog.NewRecord(
		time.Now(), Level(v),
		fmt.Sprint(args...), pc,
	)
	// r.Add(args...)
	_ = h.slog.Handler().Handle(ctx, event)
}

// Logf writes a formatted log entry
func (h *Logger) Logf(v mlog.Level, format string, args ...any) {
	// TODO decide does we need to write message if log level not used?
	// [CHECK]: front enabled ?
	if !h.conf.Level.Enabled(v) {
		return
	}
	var (
		level = Level(v)
		ctx   = context.Background()
	)
	// [CHECK]: back enabled ?
	if !h.slog.Enabled(ctx, level) {
		return
	}
	var pc uintptr
	// if !internal.IgnorePC {
	// 	var pcs [1]uintptr
	// 	// skip [runtime.Callers, this function, this function's caller]
	// 	runtime.Callers(3, pcs[:])
	// 	pc = pcs[0]
	// }
	event := slog.NewRecord(
		time.Now(), level,
		fmt.Sprintf(format, args...), pc,
	)
	// r.Add(args...)
	_ = h.slog.Handler().Handle(ctx, event)
}

// String returns the name of logger
func (*Logger) String() string {
	return "slog"
}
