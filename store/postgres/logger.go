package postgres

import (
	"context"
	"github.com/jackc/pgx/v4"
	"log/slog"
)

type SlogPGXLogger struct {
	logger *slog.Logger
}

func NewSlogPGXLogger(log *slog.Logger) *SlogPGXLogger {
	return &SlogPGXLogger{
		logger: log,
	}
}

func (l *SlogPGXLogger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]any) {
	slogLevel := mapPGXLogLevelToSlog(level)
	attrs := make([]any, 0, len(data)*2)
	for k, v := range data {
		attrs = append(attrs, k, v)
	}

	l.logger.Log(ctx, slogLevel, msg, attrs...)
}

func mapPGXLogLevelToSlog(level pgx.LogLevel) slog.Level {
	switch level {
	case pgx.LogLevelTrace,
		pgx.LogLevelDebug,
		pgx.LogLevelInfo:
		return slog.LevelDebug // or Trace ?
	case pgx.LogLevelWarn:
		return slog.LevelWarn
	case pgx.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
