// Package ast defines AST nodes that represents extension's elements
package ast

import (
	gast "github.com/yuin/goldmark/ast"
)

// A Bold struct represents a strikethrough of GFM text.
type Bold struct {
	gast.BaseInline
}

// Dump implements Node.Dump.
func (n *Bold) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, nil, nil)
}

// KindBold is a NodeKind of the Bold node.
var KindBold = gast.NewNodeKind("Bold")

// Kind implements Node.Kind.
func (n *Bold) Kind() gast.NodeKind {
	return KindBold
}

// NewBold returns a new Bold node.
func NewBold() *Bold {
	return &Bold{}
}
