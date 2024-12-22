package sqlxrepo

import (
	"fmt"
	"io"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
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

type (
	TextDecoder = pgtype.TextDecoder
	DecodeText  func(src []byte) error
)

var (
	_ TextDecoder = DecodeText(nil)
)

func (fn DecodeText) Scan(src any) error {
	if src == nil {
		return fn.DecodeText(nil, nil)
	}

	switch data := src.(type) {
	case string:
		return fn.DecodeText(nil, []byte(data))
	case []byte:
		text := make([]byte, len(data))
		copy(text, data)
		return fn.DecodeText(nil, text)
	}

	return fmt.Errorf("pgx: cannot scan %T value %[1]v into %T", src, fn)
}

func (fn DecodeText) DecodeText(_ *pgtype.ConnInfo, src []byte) error {
	if fn != nil {
		return fn(src)
	}
	// IGNORE
	return nil
}

type Sqlizer interface {
	sq.Sqlizer
}

type CTE struct {
	MATERIALIZED *bool
	Name         string
	Cols         []string
	Expr         Sqlizer
}

func (e *CTE) ToSql() (CTE string, _ []interface{}, err error) {
	query, args, err := e.Expr.ToSql() // convertToSql(e.Source)
	if err != nil {
		return "", nil, err
	}
	CTE = e.Name
	// if e.Recursive {
	// 	CTE = "RECURSIVE " + CTE
	// }
	if len(e.Cols) > 0 {
		CTE += "(" + strings.Join(e.Cols, ",") + ")"
	}
	CTE += " AS"
	if is := e.MATERIALIZED; is != nil {
		if !(*is) {
			CTE += " NOT"
		}
		CTE += " MATERIALIZED"
	}
	return CTE + " (" + query + ")", args, nil
}

type WITH struct {
	RECURSIVE bool
	tables    []*CTE
	nx        map[string]int
}

func (c *WITH) index(name string) int {
	if name != "" && c != nil {
		if e, ok := c.nx[name]; ok {
			return e
		}
	}
	return -1
}

func (c *WITH) Has(name string) bool {
	// return -1 < c.index(name)
	_, ok := c.nx[name]
	return ok
}

func (c *WITH) With(table CTE) bool {
	if table.Expr == nil {
		panic("WITH CTE( AS:expr! ) required")
	}
	name := table.Name
	if name == "" {
		panic("WITH CTE( name:string! ) required")
	}
	if c.Has(name) {
		return false
	}
	e := len(c.tables)
	if e < 1 && c.nx == nil {
		c.nx = make(map[string]int)
	}
	c.nx[name] = e
	c.tables = append(c.tables, &table)
	return true
}

func (c *WITH) ToSql() (WITH string, _ []interface{}, err error) {
	var CTE string
	for e, cte := range c.tables {
		CTE, _, err = cte.ToSql()
		if err != nil {
			return "", nil, err
		}
		if e > 0 {
			WITH += ", "
		} else {
			WITH = "WITH "
			if c.RECURSIVE {
				WITH += " RECURSIVE "
			}
		}
		WITH += CTE
	}
	return WITH, nil, nil
}

type SELECT struct {
	WITH
	Query sq.SelectBuilder
	// JOIN   map[string]Sqlizer // map[alias]expr
	Params params
}

var _ Sqlizer = (*SELECT)(nil)

func (e *SELECT) SQLText() (query string, err error) {
	var (
		WITH   string
		SELECT = e.Query.Suffix("") // shallowcopy
	)
	WITH, _, err = e.WITH.ToSql()
	if err != nil {
		return // "", nil, err
	}
	if WITH != "" {
		SELECT = SELECT.Prefix(WITH)
	}
	query, _, err = SELECT.ToSql()
	return
}

func (e *SELECT) ToSql() (query string, args []interface{}, err error) {
	query, err = e.SQLText()
	if err == nil && len(e.Params) > 0 {
		query, args, err = NamedParams(query, e.Params)
		query = CompactSQL(query)
	}
	return // query, args, err
}

func coalesce(text string, args ...string) string {
	if text != "" {
		return text
	}
	for _, text := range args {
		if text != "" {
			return text
		}
	}
	return ""
}

type JOIN struct {
	Kind  string  // [INNER|CROSS|LEFT|RIGHT[ OUTER] ]JOIN
	Table Sqlizer // RIGHT: [schema.]table(type)|[LATERAL](SELECT)
	Alias string  // AS
	Pred  Sqlizer // ON
}

func (rel *JOIN) SQL() string {
	var err error
	parts := make([]string, 2, 6)
	parts[0] = rel.Kind
	parts[1], _, err = rel.Table.ToSql()
	if err != nil {
		panic(err)
	}
	if rel.Alias != "" {
		parts = append(
			parts, "AS", rel.Alias,
		)
	}
	var ON string
	if rel.Pred != nil {
		ON, _, err = rel.Pred.ToSql()
		if err != nil {
			panic(err)
		}
	}
	parts = append(
		parts, "ON", coalesce(ON, "true"),
	)
	return strings.Join(parts, " ")
}

func (rel *JOIN) ToSql() (join string, _ []interface{}, err error) {
	return rel.SQL(), nil, nil
}

// CompactSQL formats given SQL text to compact form.
// - replaces consecutive white-space(s) with single SPACE(' ')
// - suppress single-line comment(s), started with -- up to [E]nd[o]f[L]ine
// - suppress multi-line comment(s), enclosed into /* ... */ pairs
// - transmits literal '..' or ".." sources in their original form
// https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-OPERATORS
func CompactSQL(s string) string {

	var (
		r = strings.NewReader(s)
		w strings.Builder
	)

	w.Grow(int(r.Size()))

	var (
		err  error
		char rune
		last rune
		hold rune

		isSpace = func() (is bool) {
			switch char {
			case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
				is = true
			}
			return // false
		}
		isPunct = func(char rune) (is bool) {
			switch char {
			// none; start of text
			case 0:
				is = true
			// special
			// ':' USES [squirrel] for :named parameters,
			//     so we need to keep SPACE if there were any
			case ',', '(', ')', '[', ']', ';', '\'', '"': // , ':':
				is = true
			// operators
			case '+', '-', '*', '/', '<', '>', '=', '~', '!', '@', '#', '%', '^', '&', '|':
				is = true
			}
			return // false
		}
		isQuote = func() (is bool) {
			switch char {
			case '\'', '"': // SQUOTE, DQUOTE:
				is = true
			}
			return // false
		}
		// context
		space   bool // [IN] [w]hite[sp]ace(s)
		quote   rune // [IN] [l]i[t]eral(s); *QUOTE(s)
		comment rune // [IN] [c]o[m]ment; [-|*]
		// helpers
		isComment = func() bool {
			switch comment {
			case '-':
				{
					// comment: close(\n)
					if char == '\n' { // EOL
						space = true // inject
						comment = 0  // close
						hold = 0     // clear
					}
					return true // still IN ...
				}
			case '*':
				{
					// comment: close(*/)
					if hold == 0 && char == '*' {
						// MAY: close(*/)
						hold = char
						// need more data ...
					} else if hold == '*' && char == '/' {
						space = true // inject
						comment = 0  // close
						hold = 0     // clear
					}
					return true // still IN ...
				}
				// default: 0
			}
			// NOTE: (comment == 0)
			switch hold {
			// comment: start(--)
			case '-': // single-line
				{
					if char == hold {
						hold = 0       // clear
						comment = char // start
						return true
					}
					return false
				}
			// comment: start(/*)
			case '/': // multi-line
				{
					if char == '*' {
						hold = 0       // clear
						comment = char // start
						return true
					}
					return false
				}
			case 0:
				{
					// NOTE: (hold == 0)
					switch char {
					case '-':
					case '/':
					default:
						// NOT alike ...
						return false
					}
					// need more data ...
					hold = char
					// DO NOT write(!)
					return true
				}
			default:
				{
					// NO match
					// need to write hold[ed] char
					return false
				}
			}
		}
		isLiteral = func() bool {
			if !isQuote() || last == '\\' { // ESC(\')
				return quote > 0 // We are IN ?
			}
			// close(?)
			if quote == char { // inLiteral(?)
				quote = 0
				return true // as last
			}
			// start(!)
			quote = char
			return true
		}
		// [re]write
		output = func() {
			if hold > 0 {
				w.WriteRune(hold)
				last = hold
				hold = 0
			}
			if space {
				space = false
				if !isPunct(last) && !isPunct(char) {
					w.WriteRune(' ') // INJECT SPACE(' ')
				}
			}
			w.WriteRune(char)
			last = char
		}
	)

	var e int
	for {

		char, _, err = r.ReadRune()
		if err != nil {
			break
		}
		e++ // char index position

		if isComment() {
			// suppress; DO NOT write(!)
			continue
		}

		if isLiteral() {
			// [re]write: as is (!)
			output()
			continue
		}

		if isSpace() {
			// fold sequence ...
			space = true
			continue
		}
		// [re]write: [hold]char
		output()
	}

	if err != io.EOF {
		panic(err)
	}

	return w.String()
}

func WithUnion(builder sq.SelectBuilder, unionQuery sq.SelectBuilder) (sq.SelectBuilder, error) {
	query, params, err := unionQuery.ToSql()
	if err != nil {
		return builder, fmt.Errorf("error: Failed to build union query: %w", err)
	}

	return builder.Suffix(fmt.Sprintf("UNION %s", query), params...), nil
}

func WithUnionAll(builder sq.SelectBuilder, unionQuery sq.SelectBuilder) (sq.SelectBuilder, error) {
	query, params, err := unionQuery.ToSql()
	if err != nil {
		return builder, fmt.Errorf("error: Failed to build union all query: %w", err)
	}

	return builder.Suffix(fmt.Sprintf("UNION ALL %s", query), params...), nil
}
