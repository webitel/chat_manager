package postgres

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	// "database/sql"
	"github.com/jmoiron/sqlx"
)

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// OpenDB returns valid postgres DSN database connection pool
func OpenDB(log *slog.Logger, dataSource string, opts ...Option) (*sqlx.DB, error) {
	config, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	config.Logger = NewSlogPGXLogger(log)
	dbo := stdlib.OpenDB(*(config), stdlib.OptionAfterConnect(
		func(ctx context.Context, dc *pgx.Conn) error {
			// SET search_path = 'chat';
			return nil
		},
	))

	pool := &PoolConfig{}
	for _, opt := range opts {
		opt(pool)
	}

	{
		dbo.SetMaxOpenConns(pool.MaxOpenConns)
		dbo.SetMaxIdleConns(pool.MaxIdleConns)
		dbo.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
		dbo.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}

	if err = dbo.Ping(); err != nil {
		return nil, err
	}

	const pgxDriverName = "pgx"

	return sqlx.NewDb(dbo, pgxDriverName), nil
}
