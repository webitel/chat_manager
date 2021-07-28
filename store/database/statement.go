package database

import (
	sq "github.com/Masterminds/squirrel"
)

type (
	// shorthands
	InsertStmt = sq.InsertBuilder
	SelectStmt = sq.SelectBuilder
	UpdateStmt = sq.UpdateBuilder
	DeleteStmt = sq.DeleteBuilder
)

// Expr shorthand to `github.com/Masterminds/squirrel.Expr`
// Expr builds value expressions for InsertBuilder and UpdateBuilder.
//
// Example:
//     .Values(Expr("FROM_UNIXTIME(?)", t))
func Expr(sql string, args ...interface{}) interface{} {
	return sq.Expr(sql, args...)
}