package markdown

import (
	"bytes"
	"testing"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown/ast"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	testParser = goldmark.New(
		goldmark.WithParser(
			// NewParser(),
			// goldmark.DefaultParser(),
			parser.NewParser(
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
			),
		),
		goldmark.WithRenderer(
			// goldmark.DefaultRenderer(),
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					// // defaults
					util.Prioritized(NewRenderer(), 400),
					// util.Prioritized(html.NewRenderer(), 1000),
					// // extensions
					// util.Prioritized(NewUnderlineHTMLRenderer(), 498),
					// util.Prioritized(NewSpoilerHTMLRenderer(), 498),
					// util.Prioritized(NewStrikeHTMLRenderer(), 499),
					// util.Prioritized(NewItalicHTMLRenderer(), 499),
					// util.Prioritized(NewBoldHTMLRenderer(), 499),
				),
			),
		),
		// goldmark.WithRendererOptions(
		// 	// customize
		// 	html.WithHardWraps(),
		// 	html.WithXHTML(),
		// ),
	)
)

// 	// goldmark.WithParserOptions(
// 	// 	parser.WithInlineParsers(
// 	// 		util.Prioritized(NewBoldParser(), 500),
// 	// 		util.Prioritized(NewStrikethroughParser(), 500),
// 	// 		// util.Prioritized(parser.NewCodeSpanParser(), 100),
// 	// 		// util.Prioritized(parser.NewLinkParser(), 200),
// 	// 		// util.Prioritized(parser.NewAutoLinkParser(), 300),
// 	// 		// util.Prioritized(parser.NewRawHTMLParser(), 400),
// 	// 		// util.Prioritized(parser.NewEmphasisParser(), 500),
// 	// 	),
// 	// ),
// 	goldmark.WithExtensions(Bold),
// 	goldmark.WithExtensions(Italic),
// 	goldmark.WithExtensions(Underline),
// 	goldmark.WithExtensions(Strikethrough),
// 	// goldmark.WithExtensions(extension.GFM),
// 	// goldmark.WithParserOptions(
// 	// 	parser.WithAutoHeadingID(),
// 	// ),
// 	goldmark.WithRendererOptions(
// 		html.WithHardWraps(),
// 		html.WithXHTML(),
// 	),
// )

func TestMain(m *testing.M) {
	m.Run()
}

func TestMarkdownV2(t *testing.T) {

	// const input = `plain text *bold \*_italic bold ~italic bold strikethrough ||italic bold strikethrough spoiler||~ __underline italic bold___ bold*
	// footer`

	const input = "\n" +
		"plain text *bold \\*text*\n" +
		"_italic \\*text_\n" +
		"__underline__\n" +
		"~strikethrough~\n" +
		"||spoiler||\n" +
		"*bold _italic bold ~italic bold \\_\\\\_strikethrough ||italic bold strikethrough spoiler||~ __underline italic bold___ bold*\n" +
		"[inline URL](http://www.example.com/)\n" +
		"[inline mention of a user](tg://user?id=123456789)\n" +
		"`inline fixed-width code`\n" +
		"```\n" +
		"pre-formatted fixed-width code block\n" +
		"```\n" +
		"```python\n" +
		"pre-formatted fixed-width code block written in the Python programming language\n" +
		"```" + `





		` +
		"footer text\n" +
		""

		// 	const input = `aa

		// bb

		// cc
		// 	d

		// ef`

	t.Logf("input:\n\n%s\n\n", input)

	var (
		buf bytes.Buffer
		md2 = testParser
		res = renderOptions{
			Builder: &entity.Builder{},
			StdOut:  false,
		}
	)
	testParser.Renderer().AddOptions(
		// BuildEntities(&out),
		&res,
	)
	source := []byte(input)
	reader := text.NewReader([]byte(source))
	doc := md2.Parser().Parse(reader) // , opts...)
	doc.Dump(source, 0)
	err := md2.Renderer().Render(&buf, source, doc)

	// err := testParser.Convert(
	// 	[]byte(source), io.Discard, // &buf,
	// )
	if err != nil {
		t.Error(err)
	}
	t.Logf("output:\n\n%s\n\n", buf.String())

	// text, entities := out.Complete()
	text, entities := res.Complete()
	t.Logf("message:\n\n%s\n\n%v\n\n", text, entities)
}
