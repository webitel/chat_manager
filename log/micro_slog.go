package log

import (
	"context"
	"fmt"
	"github.com/micro/micro/v3/service/logger"

	"log/slog"
)

// SlogAdapter -
type SlogAdapter struct {
	log *slog.Logger
}

// NewSlogAdapter
func NewSlogAdapter(log *slog.Logger) *SlogAdapter {
	return &SlogAdapter{
		log: log,
	}
}

// Init
func (s *SlogAdapter) Init(opts ...logger.Option) error {
	return nil
}

// Fields
func (s *SlogAdapter) Fields(fields map[string]interface{}) logger.Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}

	return &SlogAdapter{
		log: s.log.With(attrs...),
	}
}

// Log
func (s *SlogAdapter) Log(level logger.Level, v ...interface{}) {
	slogLevel := mapMicroLevelToSlog(level)
	s.log.Log(context.Background(), slogLevel, fmt.Sprint(v...))
}

// Log
func (s *SlogAdapter) Logf(level logger.Level, format string, v ...interface{}) {
	slogLevel := mapMicroLevelToSlog(level)
	s.log.Log(context.Background(), slogLevel, fmt.Sprintf(format, v...))
}

// Options
func (s *SlogAdapter) Options() logger.Options {
	return logger.Options{}
}

// String повертає ім'я логера
func (s *SlogAdapter) String() string {
	return "slog"
}

// Map рівнів Micro до slog
func mapMicroLevelToSlog(level logger.Level) slog.Level {
	switch level {
	case logger.TraceLevel:
		return LevelTrace // Кастомний TRACE
	case logger.DebugLevel:
		return slog.LevelDebug
	case logger.InfoLevel:
		return slog.LevelInfo
	case logger.WarnLevel:
		return slog.LevelWarn
	case logger.ErrorLevel:
		return slog.LevelError
	case logger.FatalLevel:
		return LevelFatal // slog не має Fatal, використовуємо LevelFatal
	default:
		return slog.LevelInfo
	}
}
