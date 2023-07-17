package whatsapp

// Contact information for the customer who sent a message to the business.
// See Update.Contacts.
type Sender struct {

	// The customer's WhatsApp ID.
	// A business can respond to a message using this ID.
	WAID string `json:"wa_id"`

	// The customer’s name
	Name string `json:"name"`

	// Customer's profile information.
	// Can contain the following field:
	// – name; The customer’s name
	Profile map[string]string `json:"profile,omitempty"`
}

func (e *Sender) GetName() string {
	if e == nil {
		return ""
	}
	if e.Name != "" {
		return e.Name
	}
	if e.Profile != nil {
		return e.Profile["name"]
	}
	return ""
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#contacts-object
type Contact struct {

	// REQUIRED.
	// Full contact name formatted as a name object.
	// The object can contain the following fields:
	//
	// – formatted_name; REQUIRED(string). Full name, as it normally appears.
	// – first_name; OPTIONAL(string). First name.
	// – last_name; OPTIONAL(string). Last name.
	// – middle_name; OPTIONAL(string). Middle name.
	// – suffix; OPTIONAL(string). Name suffix.
	// – prefix; OPTIONAL(string). Name prefix.
	//
	// *At least one of the optional parameters needs to be included along with the formatted_name parameter.
	//
	ContactName `json:"name"`

	// OPTIONAL.
	// Full contact address(es) formatted as an addresses object.
	// The object can contain the following fields:
	//
	// – street; OPTIONAL(string). Street number and name.
	// – city; OPTIONAL(string). City name.
	// – state; OPTIONAL(string). State abbreviation.
	// – zip; OPTIONAL(string). ZIP code.
	// – country; OPTIONAL(string). Full country name.
	// – country_code; OPTIONAL(string). Two-letter country abbreviation.
	// – type; OPTIONAL(string). Standard values are HOME and WORK.
	//
	Addresses []*ContactAddress `json:"addresses,omitempty"`

	// OPTIONAL.
	// YYYY-MM-DD formatted string.
	Birthday string `json:"birthday,omitempty"`

	// OPTIONAL.
	// Contact email address(es) formatted as an emails object.
	// The object can contain the following fields:
	//
	// – email; OPTIONAL(string). Email address.
	// – type; OPTIONAL(string). Standard values are HOME and WORK.
	//
	Emails []*ContactEmail `json:"emails,omitempty"`

	// OPTIONAL.
	// Contact organization information formatted as an org object.
	// The object can contain the following fields:
	//
	// – company; OPTIONAL(string). Name of the contact's company.
	// – department; OPTIONAL(string). Name of the contact's department.
	// – title; OPTIONAL(string). Contact's business title.
	//
	Organization *Organization `json:"org,omitempty"`

	// OPTIONAL.
	// Contact phone number(s) formatted as a phone object.
	// The object can contain the following fields:
	//
	// – phone; OPTIONAL(string). Automatically populated with the `wa_id` value as a formatted phone number.
	// – type; OPTIONAL(string). Standard Values are CELL, MAIN, IPHONE, HOME, and WORK.
	// – wa_id; OPTIONAL(string). WhatsApp ID.
	//
	Phones []*ContactPhone `json:"phones,omitempty"`

	// OPTIONAL.
	// Contact URL(s) formatted as a urls object.
	// The object can contain the following fields:
	//
	// – url; OPTIONAL(string). URL.
	// – type; OPTIONAL(string). Standard values are HOME and WORK.
	//
	URLs []*ContactURL `json:"urls,omitempty"`
}

type ContactName struct {
	// REQUIRED. Full name, as it normally appears.
	Name string `json:"formatted_name"`
	// OPTIONAL. First name.
	FirstName string `json:"first_name,omitempty"`
	// OPTIONAL. Middle name.
	MiddleName string `json:"middle_name,omitempty"`
	// OPTIONAL. Last name.
	LastName string `json:"last_name,omitempty"`
	// OPTIONAL. Name suffix.
	Suffix string `json:"suffix,omitempty"`
	// OPTIONAL. Name prefix.
	Prefix string `json:"prefix,omitempty"`
}

type ContactAddress struct {
	// OPTIONAL. Standard values are HOME and WORK.
	Type string `json:"type,omitempty"`
	// OPTIONAL. Street number and name.
	Street string `json:"street,omitempty"`
	// OPTIONAL. City name.
	City string `json:"city,omitempty"`
	// OPTIONAL. State abbreviation.
	State string `json:"state,omitempty"`
	// OPTIONAL. ZIP code.
	ZIP string `json:"zip,omitempty"`
	// OPTIONAL. Full country name.
	Country string `json:"country,omitempty"`
	// OPTIONAL. Two-letter country abbreviation.
	CountryCode string `json:"country_code,omitempty"`
}

type ContactEmail struct {
	// OPTIONAL. Standard values are HOME and WORK.
	Type string `json:"type,omitempty"`
	// OPTIONAL. Email address.
	Address string `json:"email,omitempty"`
}

type Organization struct {
	// OPTIONAL. Name of the contact's company.
	Company string `json:"company,omitempty"`
	// OPTIONAL. Name of the contact's department.
	Department string `json:"department,omitempty"`
	// OPTIONAL. Contact's business title.
	Title string `json:"title,omitempty"`
}

type ContactPhone struct {
	// OPTIONAL. Standard Values are CELL, MAIN, IPHONE, HOME, and WORK.
	Type string `json:"type,omitempty"`
	// OPTIONAL. Automatically populated with the `wa_id` value as a formatted phone number.
	Phone string `json:"phone,omitempty"`
	// OPTIONAL. WhatsApp ID.
	WAID string `json:"wa_id,omitempty"`
}

type ContactURL struct {
	// OPTIONAL. Standard values are HOME and WORK.
	Type string `json:"type,omitempty"`
	// OPTIONAL. URL.
	URL string `json:"url,omitempty"`
}
