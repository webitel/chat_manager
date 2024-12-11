package bot

import (
	"bytes"
	"io"
	"strings"
	"sync"
	tmpl "text/template"

	"github.com/pkg/errors"
	"github.com/webitel/chat_manager/api/proto/bot"
	"github.com/webitel/chat_manager/api/proto/chat"
)

const (
	UpdateChatClose  = "close" // chat closed
	UpdateChatTitle  = "title" // form chat title
	UpdateChatMember = "join"  // chat member joined
	UpdateLeftMember = "left"  // chat member left the conversation
)

var (
	templatePeer = chat.Account{
		Id:        0,
		Channel:   "telegram",
		Contact:   "7654321",
		FirstName: "José Antonio",
		LastName:  "Domínguez Bandera",
		Username:  "banderas",
	}
	buffers = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(nil)
		},
	}
	// TemplateTests maps[name]data of well-known template(s) for validity tests purpose
	TemplateTests = map[string]interface{}{
		UpdateChatClose:  nil, // no context
		UpdateChatTitle:  &templatePeer,
		UpdateChatMember: &templatePeer,
		UpdateLeftMember: &templatePeer,
	}
)

type Template tmpl.Template

func NewTemplate(name string) *Template {
	root := tmpl.New(name)
	root.Delims("$(", ")")
	// // FIXME: Parse error: function not defined !
	// root.Funcs(tmpl.FuncMap{
	// 	"md2": fmt.Sprint,
	// })
	return (*Template)(root)
}

func (m *Template) Root() *tmpl.Template {
	return (*tmpl.Template)(m)
}

// Test well-known chatUpdates message templates
// Each profile can distribute its own auxiliary functions,
// so the verification of templates is left to the providers
func (m *Template) Test(ctx map[string]interface{}) error {
	if ctx == nil {
		ctx = TemplateTests
	}
	root := m.Root()
	// for validation purpose
	root.Option("missingkey=error")
	for name, data := range ctx {
		node := root.Lookup(name)
		if node == nil {
			continue // not defined
		}
		// testTemplate
		err := node.Execute(io.Discard, data)
		if err != nil {
			return errors.Wrap(err, "updates."+name)
		}
	}
	return nil // ok
}

func (m *Template) FromProto(on *bot.ChatUpdates) error {
	root := m.Root()
	for _, e := range []struct {
		name, text string
	}{
		{UpdateChatClose, on.GetClose()},
		{UpdateChatTitle, on.GetTitle()},
		{UpdateChatMember, on.GetJoin()},
		{UpdateLeftMember, on.GetLeft()},
	} {
		e.text = strings.TrimSpace(e.text)
		// addTemplate
		node := root.Lookup(e.name)
		if node == nil {
			if e.text == "" {
				continue
			}
			node = root.New(e.name)
		}
		_, err := node.Parse(e.text)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Template) MessageText(name string, ctx interface{}) (text string, err error) {
	root := m.Root()
	node := root.Lookup(name)
	if node == nil {
		return // "", nil
	}
	buf := buffers.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		buffers.Put(buf)
	}()
	err = node.Execute(buf, ctx)
	if err == nil {
		text = buf.String()
	}
	return // text|"", err|nil
}
