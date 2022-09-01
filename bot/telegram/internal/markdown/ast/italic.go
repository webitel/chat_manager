// Package ast defines AST nodes that represents extension's elements
package ast

import (
	gast "github.com/yuin/goldmark/ast"
)

// A Italic struct represents a strikethrough of GFM text.
type Italic struct {
	gast.BaseInline
}

// Dump implements Node.Dump.
func (n *Italic) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// KindItalic is a NodeKind of the Italic node.
var KindItalic = gast.NewNodeKind("Italic")

// Kind implements Node.Kind.
func (n *Italic) Kind() gast.NodeKind {
	return KindItalic
}

// NewItalic returns a new Italic node.
func NewItalic() *Italic {
	return &Italic{}
}
