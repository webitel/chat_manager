package bot

import (
	"fmt"
	"testing"
	"text/template"

	"github.com/webitel/chat_manager/api/proto/bot"
)

var (
	defaults = bot.ChatUpdates{
		Close: `❌ Діалог завершено!`,
		Title: `<$(.FirstName)$(if .LastName)$(.LastName | printf " %s")$(end)>$(if .Username)$(.Username | printf " aka @%s")$(end)`,
		Join:  `md2: 👤 *$(md2 .FirstName)*$(if .LastName)$(md2 .LastName | printf " ||%s||")$(end)`,
		Left:  `md2: 👤 ~*$(md2 .FirstName)*$(if .LastName)$(md2 .LastName | printf " ||%s||")$(end)~`,
	}
	templates *Template
)

func TestMain(m *testing.M) {
	templates = NewTemplate("tests")
	templates.Root().Funcs(
		template.FuncMap{
			"md":  fmt.Sprint,
			"md2": fmt.Sprint,
			// "html": builtin,
		},
	)
	err := templates.FromProto(&defaults)
	if err != nil {
		panic(err)
	}
	m.Run()
}

func TestTemplate_ParseProto(t *testing.T) {
	type args struct {
		pb *bot.ChatUpdates
	}
	tests := []struct {
		name string
		// m       *Template
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "general",
			args:    args{&defaults},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := templates
			if err := m.FromProto(tt.args.pb); (err != nil) != tt.wantErr {
				t.Errorf("Template.ParseProto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTemplate_MessageText(t *testing.T) {
	type args struct {
		name string
		ctx  interface{}
	}
	tests := []struct {
		name     string
		args     args
		wantText string
		wantErr  bool
	}{
		// TODO: Add test cases.
		{"close", args{"close", nil}, "❌ Діалог завершено!", false},
		{"title", args{"title", &templatePeer}, "<José Antonio Domínguez Bandera> aka @banderas", false},
		{"join", args{"join", &templatePeer}, "md2: 👤 *José Antonio* ||Domínguez Bandera||", false},
		{"left", args{"left", &templatePeer}, "md2: 👤 ~*José Antonio* ||Domínguez Bandera||~", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := templates
			gotText, err := m.MessageText(tt.args.name, tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Template.MessageText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotText != tt.wantText {
				t.Errorf("Template.MessageText() = %v, want %v", gotText, tt.wantText)
			}
		})
	}
}
