package postgres

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
	// URL: "postgres://" | "postgresql://"
	// DSN: param "=" value   *( " " param "=" value )
	
	scheme, opaque, _ := getScheme(dataSource)
	switch scheme {
	case "postgres", "postgresql":
		{
			if !strings.HasPrefix(opaque, "//") {
				// form: DSN ; passthru input options only
				dataSource = opaque
			} // form: URL ; passthru original input
		}
	// default: invalid scheme ; cause an error from pgx.ParseConfig()
	}
	
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

// Maybe rawDSN is of the form scheme:opaque.
// (Scheme must be [a-zA-Z][a-zA-Z0-9+.-]*)
// If so, return scheme, opaque; else return "", rawDSN.
func getScheme(rawDSN string) (scheme, opaque string, err error) {
	for i := 0; i < len(rawDSN); i++ {
		c := rawDSN[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
		// do nothing
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return "", rawDSN, nil
			}
		case c == ':':
			if i == 0 {
				return "", "", errors.New("dsn: missing protocol scheme")
			}
			return rawDSN[:i], rawDSN[i+1:], nil
		default:
			// we have encountered an invalid character,
			// so there is no valid scheme
			return "", rawDSN, nil
		}
	}
	// it is implied that this is a stand-alone scheme without options
	return rawDSN, "", nil
}