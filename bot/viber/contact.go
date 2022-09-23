package viber

import "text/template"

var (
	// A template for representing a contact in text
	contactInfo, _ = template.New("contact").Parse(
		`{{- if .Name}}
Name: {{.Name}}
{{- end}}
{{- if .Phone}}
Phone: {{.Phone}}
{{- end}}`,
	)
)
