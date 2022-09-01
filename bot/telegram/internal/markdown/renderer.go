package markdown

import (
	"github.com/gotd/td/telegram/message/entity"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown/ast"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type renderOptions struct {
	*entity.Builder
	StdOut bool // Do also write to output ?
}

const optRenderTgo renderer.OptionName = "Telegram"

func (c *renderOptions) SetConfig(cfg *renderer.Config) {
	cfg.Options[optRenderTgo] = c
}

func RenderOptions(builder *entity.Builder) renderer.Option {
	return &renderOptions{builder, false}
}

// builder implements renderer.NodeRenderer
type builder struct {
	*renderOptions
	stack []entity.Token // Token.Start code point(s) stack
}

// SetOption sets given option to the object.
// Unacceptable options may be passed.
// Thus implementations must ignore unacceptable options.
// SetOption implements renderer.NodeRenderer.SetOption.
func (c *builder) SetOption(name renderer.OptionName, value interface{}) {
	switch name {
	// case optBuildEntities:
	// 	c.out = value.(*entity.Builder)
	case optRenderTgo:
		c.renderOptions = value.(*renderOptions)
	}
}

func NewRenderer(opts ...renderer.Option) renderer.NodeRenderer {
	return &builder{}
}

var _ renderer.NodeRenderer = (*builder)(nil)

// RegisterFuncs implements NodeRenderer.RegisterFuncs .
func (c *builder) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {

	// // blocks
	// reg.Register(ast.KindDocument, c.renderDocument)
	// reg.Register(ast.KindHeading, c.renderHeading)
	// reg.Register(ast.KindBlockquote, c.renderBlockquote)
	reg.Register(gast.KindCodeBlock, c.renderCodeBlock)
	// reg.Register(ast.KindFencedCodeBlock, c.renderFencedCodeBlock)
	// reg.Register(ast.KindHTMLBlock, c.renderHTMLBlock)
	// reg.Register(ast.KindList, c.renderList)
	// reg.Register(ast.KindListItem, c.renderListItem)
	reg.Register(gast.KindParagraph, c.renderParagraph)
	// reg.Register(ast.KindTextBlock, c.renderTextBlock)
	// reg.Register(ast.KindThematicBreak, c.renderThematicBreak)

	// // inlines
	// reg.Register(ast.KindAutoLink, c.renderAutoLink)
	// reg.Register(ast.KindCodeSpan, c.renderCodeSpan)
	// reg.Register(ast.KindEmphasis, c.renderEmphasis)
	// reg.Register(ast.KindImage, c.renderImage)
	// reg.Register(ast.KindLink, c.renderLink)
	// reg.Register(ast.KindRawHTML, c.renderRawHTML)
	reg.Register(gast.KindText, c.renderText)
	// reg.Register(ast.KindString, c.renderString)

	reg.Register(ast.KindBold, c.renderInline(entity.Bold(), "*"))
	reg.Register(ast.KindItalic, c.renderInline(entity.Italic(), "_"))
	reg.Register(ast.KindSpoiler, c.renderInline(entity.Spoiler(), "||"))
	reg.Register(ast.KindUnderline, c.renderInline(entity.Underline(), "__"))
	reg.Register(ast.KindStrikethrough, c.renderInline(entity.Strike(), "~"))
	reg.Register(gast.KindCodeSpan, c.renderInline(entity.Code(), "`"))
	reg.Register(gast.KindLink, c.renderLink)

	reg.Register(gast.KindFencedCodeBlock, c.renderFencedCodeBlock)
}

func (c *builder) rawWrite(out util.BufWriter, text []byte) (int, error) {
	n, err := c.Builder.Write(text)
	if c.StdOut && err == nil {
		n, err = out.Write(text)
	}
	return n, err
}

func (c *builder) rawWriteLines(w util.BufWriter, source []byte, n gast.Node) {
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		c.rawWrite(w, line.Value(source))
	}
}

var _LF = []byte{'\n'}

func (c *builder) renderParagraph(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		// if n.HasBlankPreviousLines() {
		// 	c.rawWrite(w, _LF)
		// }
	} else {
		c.rawWrite(w, _LF)
	}
	return gast.WalkContinue, nil
}

func (c *builder) renderLink(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*gast.Link)
	if entering {
		c.tokenOffset()
		c.rawWrite(w, n.Title)
	} else {
		c.tokenFormat(entity.TextURL(
			string(n.Destination),
		))
	}
	return gast.WalkContinue, nil
}

func (c *builder) renderCodeBlock(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		if code, ok := n.(*gast.CodeBlock); ok {
			if code.HasBlankPreviousLines() {
				c.rawWrite(w, []byte{'\n'})
			}
		}
		c.rawWriteLines(w, source, n)
	}
	return gast.WalkContinue, nil
}

func (c *builder) renderFencedCodeBlock(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*gast.FencedCodeBlock)
	language := n.Language(source)
	if entering {
		c.tokenOffset()
		if c.StdOut {
			_, _ = w.WriteString("```")
			if language != nil {
				c.Write(language)
			}
			w.WriteByte('\n')
		}
		c.rawWriteLines(w, source, n)
	} else {

		var style entity.Formatter
		if language == nil {
			style = entity.Code()
		} else {
			style = entity.Pre(string(language))
		}
		c.tokenFormat(style)

		if c.StdOut {
			_, _ = w.WriteString("```\n")
		}
	}
	return gast.WalkContinue, nil
}

func (c *builder) renderText(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*gast.Text)
	segment := n.Segment
	text := segment.Value(source)
	text = UnescapeBytes(text)
	_, _ = c.rawWrite(w, text)
	if n.SoftLineBreak() {
		_, _ = c.rawWrite(w, []byte{'\n'})
	}
	return gast.WalkContinue, nil
}

func (c *builder) renderInline(style entity.Formatter, tag string) renderer.NodeRendererFunc {
	return func(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			c.tokenOffset()
			// w.WriteString("<" + tag + ">")
		} else {
			c.tokenFormat(style)
			// w.WriteString("</" + tag + ">")
		}
		return gast.WalkContinue, nil
	}
}

// tokenOffset remembers c.renderOptions.Builder's
// current state as a start of new token entity
func (c *builder) tokenOffset() {
	// offset := c.out.Token()
	c.stack = append(
		c.stack, c.Token(),
	)
}

// tokenFormat comletes top c.stack token as a separate format entity
func (c *builder) tokenFormat(style entity.Formatter) {
	n := len(c.stack)
	offset := c.stack[n-1]
	c.stack = c.stack[0 : n-1]
	offset.Apply(c.Builder, style)
}
