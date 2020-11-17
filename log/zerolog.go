package log

import (
	"os"
	"github.com/rs/zerolog"
)

// var Default = zerolog.New(os.Stdout)

// Console returns new zerolog.ConsoleWriter
func Console(level string, color bool) (*zerolog.Logger, error) {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		return nil, err
	}

	// l := zerolog.New(os.Stdout).With().Timestamp().Logger()
	c := zerolog.New(
		zerolog.ConsoleWriter{
			Out: os.Stdout,
			NoColor:!color,
		}).
		With().
			Timestamp().
		Logger()
	
	c = c.Level(l)

	return &c, nil
}