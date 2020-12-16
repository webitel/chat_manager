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

type Properties map[string]string

func (e Properties) Value() (interface{}, error) {
	return NullProperties(e), nil
}

func (e *Properties) Scan(src interface{}) error {
	return ScanProperties((*map[string]string)(e))(src)
}