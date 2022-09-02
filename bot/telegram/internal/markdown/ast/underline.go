// Package ast defines AST nodes that represents extension's elements
package ast

import (
	gast "github.com/yuin/goldmark/ast"
)

// A Underline struct represents a strikethrough of GFM text.
type Underline struct {
	gast.BaseInline
}

// Dump implements Node.Dump.
func (n *Underline) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// KindUnderline is a NodeKind of the Underline node.
var KindUnderline = gast.NewNodeKind("Underline")

// Kind implements Node.Kind.
func (n *Underline) Kind() gast.NodeKind {
	return KindUnderline
}

// NewUnderline returns a new Underline node.
func NewUnderline() *Underline {
	return &Underline{}
}
