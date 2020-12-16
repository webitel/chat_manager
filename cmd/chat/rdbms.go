package main

import (
	"fmt"
	"time"
	
	"context"

	// "database/sql"
	"github.com/jmoiron/sqlx"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"

	"github.com/rs/zerolog"
)

// OpenDB returns valid postgres DSN database connection pool
func OpenDB(dataSource string) (*sqlx.DB, error) { 

	config, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	config.Logger = pgxLogger(0)
	// dataSource = stdlib.RegisterConnConfig(config)
	// db, _ := sql.Open("pgx", dataSource)
	
	dbo := stdlib.OpenDB(*(config), stdlib.OptionAfterConnect(
		func(ctx context.Context, dc *pgx.Conn) error {
			// SET search_path = 'chat';
			return nil
		},
	))

	const pgxDriverName = "pgx"
	return sqlx.NewDb(dbo, pgxDriverName), nil
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

