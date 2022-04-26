package infobip

import "text/template"


type ContactInfo struct {

	// Address information
	Address []*ContactAddress `json:"addresses,omitempty"`

	// Birthday information, YYYY-MM-DD formatted string.
	Birthday string `json:"birthday,omitempty"`

	// Email information.
	Emails []*ContactEmail `json:"emails,omitempty"`

	// Full contact name
	Name *ContactName `json:"name,omitempty"`

	// Organization information
	Organization *ContactOrganization`json:"org,omitempty"`

	// Phone information
	Phones []*ContactPhone `json:"phones,omitempty"`

	// URL information
	URLs []*ContactURL `json:"urls,omitempty"`
}

type ContactName struct {

	// First name of a contact. Mandatory value.
	FirstName string `json:"firstName,omitempty"`

	// Middle name of a contact.
	MiddleName string `json:"middleName,omitempty"`

	// Last name of a contact.
	LastName string `json:"lastName,omitempty"`

	// Full name as it normally appears. Mandatory value.
	FormattedName string `json:"formattedName,omitempty"`

	// Name suffix of a contact.
	NameSuffix string `json:"nameSuffix,omitempty"`

	// Name prefix of a contact.
	NamePrefix string `json:"namePrefix,omitempty"`

}

type ContactOrganization struct {

	// Company name
	Company string `json:"company,omitempty"`

	// Description name
	Department string `json:"department,omitempty"`

	// Title
	Title string `json:"title,omitempty"`

}

type ContactAddress struct {

	// Street name
	Street string `json:"street,omitempty"`
	
	// City name
	City string `json:"city,omitempty"`
	
	// State name
	State string `json:"state,omitempty"`
	
	// Zip value
	ZIP string `json:"zip,omitempty"`
	
	// Country name
	Country string `json:"country,omitempty"`
	
	// Country code value
	CountryCode string `json:"countryCode,omitempty"`
	
	// Enum: "HOME" "WORK"
	// Type of an address.
	Type string `json:"type,omitempty"`

}

type ContactEmail struct {

	// Email of a contact
	Email string `json:"email"`

	// Enum: "HOME" "WORK"
	// Type of an email
	Type string `json:"type"`

}

type ContactPhone struct {

	// Contact phone number
	Phone string `json:"phone"`

	// WhatsApp ID
	WhatsAppID string `json:"waId,omitempty"`

	// Enum: "CELL" "MAIN" "IPHONE" "HOME" "WORK"
	// Type of a phone.
	Type string `json:"type"`

}

type ContactURL struct {

	// Contact URL
	URL string `json:"url"`
	
	// Enum: "HOME" "WORK"
	// Type of a URL.
	Type string `json:"type"`

}

var (

	contactInfo, _ = template.New("contact").Parse(
`{{range . -}}
Contact: {{.Name.FormattedName}}
{{- if .Birthday}}
Birthday: {{.Birthday}}
{{- end}}
{{- range .Address}}
Address[{{.Type}}]: {{.Street}}, {{.City}}, {{.State}}, {{.Country}}, {{.ZIP}}
{{- end}}
{{- if .Organization.Company}}
Organization: {{.Organization.Company}}
{{- end}}
{{- range .Emails}}
Email[{{.Type}}]: {{.Email}}
{{- end}}
{{- range .Phones}}
Phone[{{.Type}}]: {{.Phone}}
{{- end}}
{{- range .URLs}}
URL[{{.Type}}]: {{.URL}}
{{- end}}

{{end}}`,
	)
)