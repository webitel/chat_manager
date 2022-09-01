package markdown

import (
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown/ast"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

var DefaultParser = NewParser()

func NewParser(opts ...parser.Option) parser.Parser {

	// opts = append(opts, )

	return parser.NewParser(
		parser.WithBlockParsers(
			// parser.DefaultBlockParsers()...,
			// util.Prioritized(parser.NewSetextHeadingParser(), 100),
			// util.Prioritized(parser.NewThematicBreakParser(), 200),
			// util.Prioritized(parser.NewListParser(), 300),
			// util.Prioritized(parser.NewListItemParser(), 400),
			util.Prioritized(parser.NewCodeBlockParser(), 500), // ```
			// util.Prioritized(parser.NewATXHeadingParser(), 600),
			util.Prioritized(parser.NewFencedCodeBlockParser(), 700), // ```lang
			// util.Prioritized(parser.NewBlockquoteParser(), 800), // >
			// util.Prioritized(parser.NewHTMLBlockParser(), 900),
			util.Prioritized(parser.NewParagraphParser(), 1000),
		),
		parser.WithInlineParsers(
			// parser.DefaultInlineParsers()...
			util.Prioritized(parser.NewCodeSpanParser(), 100), // `inline fixed-width code`
			util.Prioritized(parser.NewLinkParser(), 200),     // [inline URL](http://www.example.com/)
			util.Prioritized(parser.NewAutoLinkParser(), 300), // http://www.example.com mailbox@mx.example.com
			util.Prioritized(parser.NewRawHTMLParser(), 400),  //
			// util.Prioritized(parser.NewEmphasisParser(), 500),
			// extensions
			util.Prioritized(NewInlineParser('*',
				func() gast.Node { return ast.NewBold() }), 500), // *bold \*text*
			util.Prioritized(NewInlineParser('_',
				func() gast.Node { return ast.NewItalic() },           // _italic \*text_
				func() gast.Node { return ast.NewUnderline() }), 500), // __underline__
			util.Prioritized(NewInlineParser('|', nil,
				func() gast.Node { return ast.NewSpoiler() }), 500), // ||spoiler||
			util.Prioritized(NewInlineParser('~',
				func() gast.Node { return ast.NewStrikethrough() }), 500), // ~strikethrough~

			// util.Prioritized(NewUnderlineParser(), 498),
			// // util.Prioritized(NewSpoilerParser(), 498),
			// // util.Prioritized(NewStrikeParser(), 499),
			// util.Prioritized(NewItalicParser(), 499),
			// util.Prioritized(NewBoldParser(), 499),
		),
		// parser.WithParagraphTransformers(
		// 	// parser.DefaultParagraphTransformers()...,
		// 	util.Prioritized(parser.LinkReferenceParagraphTransformer, 100),
		// ),
	)
}
