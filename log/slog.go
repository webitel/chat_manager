package log

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	mlog "github.com/micro/micro/v3/service/logger"
	sfmt "github.com/samber/slog-formatter"
)

func parseLevel(s string) (v slog.Level, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("wlog: level string %q: %w", s, err)
		}
	}()

	name := s
	offset := 0
	if i := strings.IndexAny(s, "+-"); i >= 0 {
		name = s[:i]
		offset, err = strconv.Atoi(s[i:])
		if err != nil {
			return // info, err
		}
	}
	switch strings.ToUpper(name) {
	case "TRACE":
		v = (slog.LevelDebug - 4)
	case "DEBUG":
		v = slog.LevelDebug
	case "INFO":
		v = slog.LevelInfo
	case "WARN":
		v = slog.LevelWarn
	case "ERROR":
		v = slog.LevelError
	case "FATAL":
		v = (slog.LevelError + 4)
	default:
		err = errors.New("unknown name")
		return // info, err
	}
	v += slog.Level(offset)
	return // v, nil
}

func slog2microLevel(v slog.Level) mlog.Level {
	m := mlog.Level(v / 4)
	if (v % 4) < 0 { // negative ?
		m-- // DEBUG-2  => trace
	}
	return m
}

func console(output *os.File, verbose slog.Level) slog.Handler {
	colorize, _ := strconv.ParseBool(
		os.Getenv("WBTL_LOG_COLOR"),
	)
	if colorize {
		colorize = isatty.IsTerminal(
			output.Fd(),
		)
	}
	return sfmt.NewFormatterHandler(
	// sfmt.FormatByType(func(e *myError) slog.Value {
	// 	return slog.GroupValue(
	// 		slog.Int("code", e.code),
	// 		slog.String("message", e.msg),
	// 	)
	// }),
	// sfmt.ErrorFormatter("error_with_generic_formatter"),
	// sfmt.FormatByKey("email", func(v slog.Value) slog.Value {
	// 	return slog.StringValue("***********")
	// }),
	// sfmt.FormatByGroupKey([]string{"a-group"}, "hello", func(v slog.Value) slog.Value {
	// 	return slog.StringValue("eve")
	// }),
	// sfmt.FormatByGroup([]string{"hq"}, func(attrs []slog.Attr) slog.Value {
	// 	return slog.GroupValue(
	// 		slog.Group(
	// 			"address",
	// 			lo.ToAnySlice(attrs)...,
	// 		),
	// 	)
	// }),
	// sfmt.PIIFormatter("hq"),
	// sfmt.ErrorFormatter("err"),
	// sfmt.ErrorFormatter("error"),
	)(
		// slog.NewJSONHandler(os.Stdout, nil),
		tint.NewHandler(output, &tint.Options{
			AddSource: false,
			Level:     verbose.Level(),
			// ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			// 	return attr
			// },
			TimeFormat: "Jan 02 15:04:05.000", // time.StampMilli,
			NoColor:    !colorize,
		}),
	)
}
