package markdown

import (
	"strings"

	gast "github.com/yuin/goldmark/ast"

	// "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type inlineParser struct {
	c byte // delimiter
	// n int  // sequence count
	n []func() gast.Node
}

func (p *inlineParser) IsDelimiter(b byte) bool {
	return p.c == b
}

func (p *inlineParser) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

// OnMatch returns a new node based on the number of delimiters consumed.
// If the number of delimiters consumed is greater than the number of
// functions provided, it will use the last function in the slice.
func (p *inlineParser) OnMatch(consumes int) gast.Node {
	var (
		node     gast.Node
		nodesLen = len(p.n)
	)

	for m := consumes - 1; m >= 0 && node == nil; m-- {
		if m >= nodesLen {
			continue
		}
		new := p.n[m]
		if new != nil {
			node = new()
		}
	}

	if node == nil {
		panic(`inline: style "` + strings.Repeat(string(p.c), consumes) + `" undefined`)
	}
	return node
}

// NewInlineParser return a new InlineParser that parses
// like `c{n}`text`c{n}` inline expressions.
func NewInlineParser(c byte, n ...func() gast.Node) parser.InlineParser {
	return &inlineParser{c, n}
}

func (s *inlineParser) Trigger() []byte {
	return []byte{s.c}
}

func (s *inlineParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 1, parser.DelimiterProcessor(s))
	if node == nil {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (s *inlineParser) CloseBlock(parent gast.Node, pc parser.Context) {
	// nothing to do
}
