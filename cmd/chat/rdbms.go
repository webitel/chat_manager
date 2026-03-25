package chat

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/webitel/chat_manager/store/postgres"
)

// OpenDB returns valid postgres DSN database connection pool
func OpenDB(log *slog.Logger, dataSource string, opts ...postgres.Option) (*sqlx.DB, error) {
	config, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	config.Logger = &pgxLogger{
		log: log,
	}
	config.LogLevel = pgx.LogLevelTrace
	// dataSource = stdlib.RegisterConnConfig(config)
	// db, _ := sql.Open("pgx", dataSource)

	const driverName = "pgx"
	dbo := stdlib.OpenDB(*(config),
		stdlib.OptionBeforeConnect(
			func(ctx context.Context, dsn *pgx.ConnConfig) error {
				log.Info(
					"connected",
					"driver", driverName,
					"host", dsn.Host,
				)
				return nil
			},
		),
		stdlib.OptionAfterConnect(
			func(ctx context.Context, conn *pgx.Conn) error {
				// TODO: SET search_path = 'chat';
				// res, err := conn.Exec(ctx, "SET search_path = 'chat';")
				// return res.Close()
				return nil
			},
		),
	)

	pool := &postgres.PoolConfig{}
	for _, opt := range opts {
		opt(pool)
	}

	{
		dbo.SetMaxOpenConns(pool.MaxOpenConns)
		dbo.SetMaxIdleConns(pool.MaxIdleConns)
		dbo.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
		dbo.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}

	err = dbo.Ping()
	if err != nil {
		return nil, err
	}

	return sqlx.NewDb(dbo, driverName), nil
}

type pgxLogger struct {
	log *slog.Logger
}

func (p *pgxLogger) Log(ctx context.Context, lvl pgx.LogLevel, text string, data map[string]any) {
	// todo
	l := logWithQueryData(p.log, data)

	switch lvl {
	// case pgx.LogLevelTrace:
	// 	e = logger.Trace()
	case pgx.LogLevelDebug, pgx.LogLevelInfo:
		l.Debug(text)
	case pgx.LogLevelWarn:
		l.Warn(text)
	case pgx.LogLevelError:
		l.Error(text)
	// case pgx.LogLevelNone:
	// 	panic("log: level none")
	default:
		l.Debug(text)
	}
}

func logWithQueryData(log *slog.Logger, data map[string]any) *slog.Logger {
	for key, v := range data {
		switch key {
		case "pid":
			log = log.With(slog.Any("pid", v))
		case "err":
			log = log.With(slog.Any("error", v))
		case "sql":
			query, _ := v.(string)
			log = log.With(slog.String("query", query))
		case "args":
			params, _ := v.([]any)
			log = log.With(slog.String("params", fmt.Sprintf("%+v", params)))
		case "time":
			log = log.With(slog.Duration("spent", v.(time.Duration)))
		case "rowCount":
			log = log.With(slog.Int("rows", v.(int)))
		case "commandTag":
			log = log.With(slog.Int64("rows", v.(pgconn.CommandTag).RowsAffected()))
		default:
			log = log.With(slog.Any(key, v))
		}
	}

	return log
}
