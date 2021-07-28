package postgres

import (
	sq "github.com/Masterminds/squirrel"
)

// StatementBuilder is a parent builder for other builders, e.g. SelectBuilder.
var PGSQL = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
