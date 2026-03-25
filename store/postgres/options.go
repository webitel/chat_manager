package postgres

import (
	"time"
)

// Option configures the database connection pool.
type Option func(*PoolConfig)

// WithMaxOpenConns sets the maximum number of open connections to the database.
// If n is greater than 0 and the new MaxOpenConns is less than MaxIdleConns,
// then MaxIdleConns will be reduced to match the new MaxOpenConns limit.
// If n <= 0, then there is no limit on the number of open connections.
//
// The default is 0 (unlimited).
func WithMaxOpenConns(n int) Option {
	return func(c *PoolConfig) { c.MaxOpenConns = n }
}

// WithMaxIdleConns sets the maximum number of idle connections in the pool.
//
// If MaxOpenConns is greater than 0 but less than the new n,
// then the new n will be reduced to match the MaxOpenConns limit.
//
// If n <= 0, no idle connections are retained.
//
// The default max idle connections is currently 2.
func WithMaxIdleConns(n int) Option {
	return func(c *PoolConfig) { c.MaxIdleConns = n }
}

// WithConnMaxIdleTime sets the maximum amount of time a connection may be idle before being closed.
//
// Expired connections may be closed lazily before reuse.
//
// If d <= 0, connections are not closed due to a connection's idle time.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *PoolConfig) { c.ConnMaxIdleTime = d }
}

// WithConnMaxLifetime sets the maximum amount of time a connection may be reused.
//
// Expired connections may be closed lazily before reuse.
//
// If d <= 0, connections are not closed due to a connection's age.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *PoolConfig) { c.ConnMaxLifetime = d }
}
