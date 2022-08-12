package telegram

import (
	"fmt"
	"reflect"
	tmpl "text/template"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Helper template functions to correctly escape values
// for different message content formatting
var builtin = tmpl.FuncMap{
	"md":  MarkdownEscaper,   // Markdown (legacy)
	"md2": MarkdownV2Escaper, // MarkdownV2
	// "html": tmpl.HTMLEscaper, // builtin
}

// MarkdownEscaper returns the escaped value of the textual representation of
// its arguments in a form suitable for embedding in a URL query.
func MarkdownEscaper(args ...any) string {
	return telegram.EscapeText(telegram.ModeMarkdown, evalArgs(args))
}

// URLQueryEscaper returns the escaped value of the textual representation of
// its arguments in a form suitable for embedding in a URL query.
func MarkdownV2Escaper(args ...any) string {
	return telegram.EscapeText(telegram.ModeMarkdownV2, evalArgs(args))
}

// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

var (
	errorType       = reflect.TypeOf((*error)(nil)).Elem()
	fmtStringerType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
)

// printableValue returns the, possibly indirected, interface value inside v that
// is best for a call to formatted printer.
func printableValue(v reflect.Value) (any, bool) {
	if v.Kind() == reflect.Pointer {
		v, _ = indirect(v) // fmt.Fprint handles nil.
	}
	if !v.IsValid() {
		return "<no value>", true
	}

	if !v.Type().Implements(errorType) && !v.Type().Implements(fmtStringerType) {
		if v.CanAddr() && (reflect.PointerTo(v.Type()).Implements(errorType) || reflect.PointerTo(v.Type()).Implements(fmtStringerType)) {
			v = v.Addr()
		} else {
			switch v.Kind() {
			case reflect.Chan, reflect.Func:
				return nil, false
			}
		}
	}
	return v.Interface(), true
}

// evalArgs formats the list of arguments into a string. It is therefore equivalent to
//	fmt.Sprint(args...)
// except that each argument is indirected (if a pointer), as required,
// using the same rules as the default string evaluation during template
// execution.
func evalArgs(args []any) string {
	ok := false
	var s string
	// Fast path for simple common case.
	if len(args) == 1 {
		s, ok = args[0].(string)
	}
	if !ok {
		for i, arg := range args {
			a, ok := printableValue(reflect.ValueOf(arg))
			if ok {
				args[i] = a
			} // else let fmt do its thing
		}
		s = fmt.Sprint(args...)
	}
	return s
}
