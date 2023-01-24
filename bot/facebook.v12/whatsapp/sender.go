package whatsapp

// SendMessage request
// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages
type SendMessage struct {

	// Messaging service used for the request. Use "whatsapp".
	// REQUIRED. Only used for Cloud API.
	//
	// On-Premises API users should not use this field.
	MessagingProduct string `json:"messaging_product,omitempty"`

	// Optional.
	// Currently, you can only send messages to individuals.
	// Set this as individual.
	//
	// Default: individual
	RecipientType string `json:"recipient_type,omitempty"`

	// A message's status.
	// You can use this field to mark a message as read.
	// See the following guides for information:
	//
	// Cloud API: Mark Messages as Read
	// On-Premises API: Mark Messages as Read
	Status string `json:"status,omitempty"`

	// REQUIRED. WhatsApp ID or phone number
	// for the person you want to send a message to.
	// See Phone Numbers, Formatting for more information.
	//
	// If needed, On-Premises API users can get this number by calling the contacts endpoint.
	TO string `json:"to,omitempty"`

	// The type of message you want to send.
	// Optional.	Default: text
	Type string `json:"type,omitempty"`

	// An object containing the ID of a previous message you are replying to. For example:
	// REQUIRED if replying to any message in the conversation. Only used for Cloud API.
	//
	// {"message_id":"MESSAGE_ID"}
	Context map[string]interface{} `json:"context,omitempty"`

	// Allows for URL previews in text messages — See the Sending URLs in Text Messages.
	// This field is optional if not including a URL in your message. Values: false (default), true.
	// REQUIRED if type=text. Only used for On-Premises API.
	//
	// Cloud API users can use the same functionality with the preview_url field inside the text object.
	PreviewURL bool `json:"preview_url,omitempty"`

	// A text object.
	// REQUIRED for type=text messages.
	Text *Text `json:"text,omitempty"`

	// A media object containing an image.
	// REQUIRED when type=image.
	Image *Media `json:"image,omitempty"`

	// Required when type=sticker.
	//
	// A media object containing a sticker.
	//
	// Cloud API: Static and animated third-party outbound stickers are supported in addition to all types of inbound stickers. A static sticker needs to be 512x512 pixels and cannot exceed 100 KB. An animated sticker must be 512x512 pixels and cannot exceed 500 KB.
	// On-Premises API: Only static third-party outbound stickers are supported in addition to all types of inbound stickers. A static sticker needs to be 512x512 pixels and cannot exceed 100 KB. Animated stickers are not supported.
	// For Cloud API users, we support static third-party outbound stickers and all types of inbound stickers. The sticker needs to be 512x512 pixels and the file size needs to be less than 100 KB.
	Sticker *Media `json:"sticker,omitempty"`

	// A media object containing audio.
	// REQUIRED when type=audio.
	Audio *Media `json:"audio,omitempty"`

	// A media object containing video.
	// REQUIRED when type=video.
	Video *Media `json:"video,omitempty"`

	// A media object containing a document.
	// REQUIRED when type=document.
	Document *Media `json:"document,omitempty"`

	// A location object.
	// REQUIRED when type=location.
	Location *Location `json:"location,omitempty"`

	// A template object.
	// REQUIRED when type=template.
	Template *Template `json:"template,omitempty"`

	// A contacts object.
	// REQUIRED when type=contacts.
	Contacts interface{} `json:"contacts,omitempty"`

	// An interactive object.
	// The components of each interactive object generally follow
	// a consistent pattern: header, body, footer, and action.
	// REQUIRED when type=interactive.
	Interactive interface{} `json:"interactive,omitempty"`

	// // object	Only used for On-Premises API.
	// // Contains an hsm object. This option was deprecated with v2.39 of the On-Premises API. Use the template object instead.
	// //
	// // Cloud API users should not use this field.
	// HSM interface{} `json:"hsm,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#template-object
type Template struct {

	// REQUIRED. Name of the template.
	Name string `json:"name"`

	// Namespace of the template.
	// OPTIONAL. Only used for On-Premises API.
	Namespace string `json:"namespace,omitempty"`

	// REQUIRED.
	// Contains a language object.
	// Specifies the language the template may be rendered in.
	//
	// The language object can contain the following fields:
	//
	// – policy string (REQUIRED). The language policy the message should follow. The only supported option is deterministic. See Language Policy Options.
	// – code string (REQUIRED). The code of the language or locale to use. Accepts both language and language_locale formats (e.g., en and en_US). For all codes, see Supported Languages.
	Language interface{} `json:"language,omitempty"`

	// OPTIONAL. Array of components objects containing the parameters of the message.
	Components []interface{} `json:"components,omitempty"`
}
