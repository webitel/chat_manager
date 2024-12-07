package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	// "database/sql"
	"github.com/jmoiron/sqlx"
)

// OpenDB returns valid postgres DSN database connection pool
func OpenDB(log *slog.Logger, dataSource string) (*sqlx.DB, error) {

	config, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	config.Logger = NewSlogPGXLogger(log)
	// dataSource = stdlib.RegisterConnConfig(config)
	// db, _ := sql.Open("pgx", dataSource)

	dbo := stdlib.OpenDB(*(config), stdlib.OptionAfterConnect(
		func(ctx context.Context, dc *pgx.Conn) error {
			// SET search_path = 'chat';
			return nil
		},
	))

	err = dbo.Ping()
	if err != nil {
		return nil, err
	}

	const pgxDriverName = "pgx"
	return sqlx.NewDb(dbo, pgxDriverName), nil
}
