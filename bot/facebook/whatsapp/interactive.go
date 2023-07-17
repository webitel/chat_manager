package whatsapp

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#interactive-object
type Interactive struct {

	// REQUIRED.
	// The `type` of interactive message you want to send.
	// Supported values:
	//
	// – button;       Use it for Reply Buttons.
	// – list;         Use it for List Messages.
	// – product;      Use for Single Product Messages.
	// – product_list; Use for Multi-Product Messages.
	//
	Type string `json:"type"`

	// ----- Update(RECV) Options -----

	QuickReply *Button `json:"button_reply,omitempty"`
	ListReply  *Button `json:"list_reply,omitempty"`

	// ----- Message(SEND) Options -----

	// REQUIRED.
	// Action you want the user
	// to perform after reading the message.
	Action *Action `json:"action,omitempty"`

	// REQUIRED for type `product_list`.
	// OPTIONAL for other types.
	//
	// Header content displayed on top of a message.
	// You cannot set a header if your interactive
	// object is of `product` type.
	// See header object for more information.
	Header *Header `json:"header,omitempty"`

	// OPTIONAL for type `product`.
	// REQUIRED for other message types.
	//
	// An object with the body of the message.
	// The body object contains the following field:
	//
	// – text (string)
	//   REQUIRED if body is present.
	//   The content of the message.
	//   Emojis and markdown are supported.
	//   Maximum length: 1024 characters.
	//
	Body *Content `json:"body,omitempty"`

	// OPTIONAL.
	// An object with the footer of the message.
	//
	// The footer object contains the following field:
	//
	// – text (string)
	//   REQUIRED if footer is present.
	//   The footer content.
	//   Emojis, markdown, and links are supported.
	//   Maximum length: 60 characters.
	Footer *Content `json:"footer,omitempty"`
}

// Content body
type Content struct {
	// Text of the content body.
	Text string `json:"text"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#action-object
type Action struct {

	// Required for List Messages.
	//
	// Button content.
	// It cannot be an empty string and must be unique within the message.
	// Emojis are supported, markdown is not.
	//
	// Maximum length: 20 characters.
	Button string `json:"button,omitempty"`

	// Required for Reply Buttons.
	// A button object can contain the following parameters:
	//
	// type: only supported type is reply (for Reply Button)
	// title: Button title. It cannot be an empty string and must be unique within the message. Emojis are supported, markdown is not. Maximum length: 20 characters.
	// id: Unique identifier for your button. This ID is returned in the webhook when the button is clicked by the user. Maximum length: 256 characters.
	//
	// You can have up to 3 buttons. You cannot have leading or trailing spaces when setting the ID.
	Buttons []*QuickReply `json:"buttons,omitempty"`

	// Required for Single Product Messages and Multi-Product Messages.
	// Unique identifier of the Facebook catalog linked to your WhatsApp Business Account.
	// This ID can be retrieved via the Meta Commerce Manager.
	CatalogID string `json:"catalog_id,omitempty"`

	// Required for Single Product Messages and Multi-Product Messages.
	// Unique identifier of the product in a catalog.
	//
	// To get this ID go to Meta Commerce Manager and select your Meta Business account.
	// You will see a list of shops connected to your account. Click the shop you want to use.
	// On the left-side panel, click Catalog > Items, and find the item you want to mention.
	// The ID for that item is displayed under the item's name.
	ProductRetailerID string `json:"product_retailer_id,omitempty"`

	// Required for List Messages and Multi-Product Messages.
	// Array of section objects. Minimum of 1, maximum of 10. See section object.
	Sections []*Section `json:"sections,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#header-object
type Header struct {

	// REQUIRED.
	// The header type you would like to use. Supported values:
	//
	// text: Used for List Messages, Reply Buttons, and Multi-Product Messages.
	// video: Used for Reply Buttons.
	// image: Used for Reply Buttons.
	// document: Used for Reply Buttons.
	//
	Type string `json:"type"`

	// REQUIRED if type is set to text.
	// Text for the header. Formatting allows emojis, but not markdown.
	//
	// Maximum length: 60 characters.
	Text string `json:"text,omitempty"`

	// REQUIRED if type is set to image.
	// Contains the media object for this image.
	Image *Image `json:"image,omitempty"`

	// REQUIRED if type is set to video.
	// Contains the media object for this video.
	Video *Video `json:"video,omitempty"`

	// REQUIRED if type is set to document.
	// Contains the media object for this document.
	Document *Document `json:"document,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#section-object
type Section struct {

	// Title of the section.
	// Required if the message has more than one section.
	//
	// Maximum length: 24 characters.
	Title string `json:"title,omitempty"`

	// Required for Multi-Product Messages.
	// Array of product objects.
	// There is a minimum of 1 product per section and a maximum of 30 products across all sections.
	//
	// Each product object contains the following field:
	//
	// – product_retailer_id (string)
	//   Required for Multi-Product Messages. Unique identifier of the product in a catalog.
	//   To get this ID, go to the Meta Commerce Manager, select your account and the shop you want to use.
	//   Then, click Catalog > Items, and find the item you want to mention.
	//   The ID for that item is displayed under the item's name.
	Products []interface{} `json:"product_items,omitempty"`

	// Required for List Messages.
	// Contains a list of rows. You can have a total of 10 rows across your sections.
	// Each row must have a title (Maximum length: 24 characters) and an ID (Maximum length: 200 characters). You can add a description (Maximum length: 72 characters), but it is optional.
	//
	// Example:
	//
	// "rows": [
	// 	{
	// 	"id":"unique-row-identifier-here",
	// 	"title": "row-title-content-here",
	// 	"description": "row-description-content-here",
	// 	}
	// ]
	//
	Rows []*Button `json:"rows,omitempty"`
}

// Unified Button
// Description from interactive.action.row[s]
type Button struct {
	// Unique identifier for your button.
	// This ID is returned in the webhook when the button is clicked by the user.
	// Maximum length: 200~256 characters.
	ID string `json:"id"`

	// Button title.
	// It cannot be an empty string and must be unique within the message.
	// Emojis are supported, markdown is not.
	// Maximum length: 20~24 characters.
	Title string `json:"title"`

	// OPTIONAL. Maximum length: 72 characters
	Description string `json:"description,omitempty"`
}

type QuickReply struct {

	// Button type.
	// Only supported type is "reply" (for Reply Button)
	Type string `json:"type"`

	// Button definition.
	Button `json:"reply"`
}

func NewReplyButton(text, data string) *QuickReply {
	return &QuickReply{
		Type: "reply",
		Button: Button{
			ID:    data,
			Title: text,
		},
	}
}
