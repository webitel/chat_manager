package sqlxrepo

import (

	"bytes"
	"unicode"

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

// reformat to compact SQL text
func CompactSQL(s string) string {

	var (
		
		eol int // index: endOfLine
		txt []byte // line: text

		src = []byte(s)
		dst = make([]byte, 0, len(src))

		commentLine = []byte("--")
		// commentStart = []byte("/*")
		// commentClose = []byte("*/")
	)

	const LF = '\n'

	for len(src) != 0 {

		eol = bytes.IndexByte(src, LF)
		if eol == -1 {
			eol = len(src) // EOF
		}

		// read line
		txt = src[:eol] // line
		if eol < len(src) {
			(eol)++ // advance '\n'
		}
		// advance line
		src = src[eol:]
		
		// trim comment
		eol = bytes.Index(txt, commentLine)
		if eol != -1 {
			txt = txt[:eol]
		}

		// compress whitespace(s)
		txt = bytes.TrimSpace(txt)

		r, w := 0, 0
		for w = bytes.IndexFunc(txt[r:], unicode.IsSpace); w != -1;
			w = bytes.IndexFunc(txt[r:], unicode.IsSpace) {

			w += r
			txt[w] = ' '
			(w)++
			
			txt = append(txt[:w], bytes.TrimLeftFunc(txt[w:], unicode.IsSpace)...)

			r = w
		}

		// has command text ?
		if len(txt) == 0 {
			continue
		}

		// write command text
		if len(dst) != 0 {
			dst = append(dst, ' ') // space
		}
		dst = append(dst, txt...)
	}

	return string(dst)
}