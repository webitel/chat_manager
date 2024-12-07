package micro

import (
	"log/slog"

	mlog "github.com/micro/micro/v3/service/logger"
)

func Level(v mlog.Level) slog.Level {
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	// TraceLevel -2
	// // DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	// DebugLevel -1
	// // InfoLevel is the default logging priority.
	// // General operational entries about what's going on inside the application.
	// InfoLevel  +0
	// // WarnLevel level. Non-critical entries that deserve eyes.
	// WarnLevel  +1
	// // ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// ErrorLevel +2
	// // FatalLevel level. Logs and then calls `logger.Exit(1)`. highest level of severity.
	// FatalLevel +3

	return slog.Level(v * 4)

	// LevelDebug -4
	// LevelInfo   0
	// LevelWarn   4
	// LevelError  8
}
