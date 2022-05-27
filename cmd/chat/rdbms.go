package chat

import (
	"fmt"
	"time"

	"context"

	// "database/sql"
	"github.com/jmoiron/sqlx"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"

	log "github.com/micro/micro/v3/service/logger"
	"github.com/rs/zerolog"
)

// OpenDB returns valid postgres DSN database connection pool
func OpenDB(dataSource string) (*sqlx.DB, error) {

	config, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	config.Logger = pgxLogger(0)
	config.LogLevel = pgx.LogLevelTrace
	// dataSource = stdlib.RegisterConnConfig(config)
	// db, _ := sql.Open("pgx", dataSource)

	const driverName = "pgx"
	dbo := stdlib.OpenDB(*(config),
		stdlib.OptionBeforeConnect(
			func(ctx context.Context, dsn *pgx.ConnConfig) error {
				log.Infof("Database [%s] Connect %s", driverName, dsn.ConnString())
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

	err = dbo.Ping()
	if err != nil {
		return nil, err
	}

	return sqlx.NewDb(dbo, driverName), nil
}

type pgxLogger int

func (pgxLogger) Log(ctx context.Context, rate pgx.LogLevel, text string, data map[string]interface{}) {

	var e *zerolog.Event

	switch rate {
	// case pgx.LogLevelTrace:
	// 	e = logger.Trace()
	case pgx.LogLevelDebug,
		pgx.LogLevelInfo:
		e = logger.Debug()
	case pgx.LogLevelWarn:
		e = logger.Warn()
	case pgx.LogLevelError:
		e = logger.Error()
	// case pgx.LogLevelNone:
	// 	panic("log: level none")
	default:
		e = logger.Trace()
	}

	if !e.Enabled() {
		return
	}

	e.EmbedObject(pgxLogdata(data)).Msg(text)
}

type pgxLogdata map[string]interface{}

func (ctx pgxLogdata) MarshalZerologObject(e *zerolog.Event) {

	for key, v := range ctx {
		switch key {
		case "pid":
			e = e.Uint32("pid", v.(uint32))
		case "err":
			err := v.(error)
			// switch err := err.(type) {}
			e = e.Err(err)
		case "sql":
			query, _ := v.(string)
			e = e.Str("query", query) // "\n\n"+query+"\n\n")
		case "args":
			params, _ := v.([]interface{})
			e = e.Str("params", fmt.Sprintf("%+v", params))
		case "time":
			// e = e.Dur("time", v.(time.Duration))
			e = e.Str("spent", v.(time.Duration).String())
		case "rowCount":
			e = e.Int("rows", v.(int))
		case "commandTag":
			e = e.Int64("rows", v.(pgconn.CommandTag).RowsAffected())
		default:
			e = e.Interface(key, v)
		}
	}
}
