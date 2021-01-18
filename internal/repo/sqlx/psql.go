package sqlxrepo

import (
	sq "github.com/Masterminds/squirrel"
)

// StatementBuilder is a parent builder for other builders, e.g. SelectBuilder.
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type (
	// shorthands
	InsertStmt = sq.InsertBuilder
	SelectStmt = sq.SelectBuilder
	UpdateStmt = sq.UpdateBuilder
	DeleteStmt = sq.DeleteBuilder
)