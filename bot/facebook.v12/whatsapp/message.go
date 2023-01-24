package whatsapp

// The messages array of objects is nested within the value object and is triggered when a customer updates their profile information
// or a customer sends a message to the business that is subscribed to the webhook.
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#messages-object
type Message struct {

	// The ID for the message that was received by the business.
	// You could use messages endpoint to mark this specific message as read.
	ID string `json:"id,omitempty"`

	// The time when the WhatsApp server received the message from the customer.
	// Date *Timestamp `json:"timestamp,omitempty"` // TODO
	Date int64 `json:"timestamp,string,omitempty"` // TODO

	// The customer's phone number who sent the message to the business.
	From string `json:"from,omitempty"`

	// The type of message that has been received by the business that has subscribed to Webhooks.
	// Possible value can be one of the following:
	//
	// – audio
	// – button
	// – document
	// – text
	// – image
	// – interactive
	// – order
	// – sticker
	// – system – for customer number change messages
	// – video
	// – reaction
	// – unknown
	//
	Type string `json:"type,omitempty"`

	// When messages type is set to text, this object is included.
	// This object includes the following field:
	//
	// – body; The text of the message.
	//
	Text *Text `json:"text,omitempty"`

	// When messages type is set to image,
	// this object is included in the messages object.
	//
	// – caption; Caption for the image, if provided
	// – sha256; Image hash
	// – id; ID for the image
	// – mime_type; Mime type for the image
	//
	Image *Image `json:"image,omitempty"`

	// When messages type is set to sticker,
	// this object is included in the messages object.
	//
	// – mime_type; image/webp
	// – sha256; Hash for the sticker
	// – id; ID for the sticker
	// – animated; Set to true if the sticker is animated; false otherwise.
	//
	Sticker *Sticker `json:"sticker,omitempty"`

	// When the messages type is set to audio, including voice messages,
	// this object is included in the messages object:
	//
	// – id; ID for the audio file
	// – mime_type; Mime type of the audio file
	//
	// Audio interface{} `json:"audio,omitempty"`
	Audio *Audio `json:"audio,omitempty"`

	// When messages type is set to video,
	// this object is included in messages object.
	//
	// – caption; The caption for the video, if provided
	// – filename; The name for the file on the sender's device
	// – sha256; The hash for the video
	// – id; The ID for the video
	// – mime_type; The mime type for the video file
	Video *Video `json:"video,omitempty"`

	// When messages type is set to document,
	// this object is included in the messages object.
	//
	// – caption; Caption for the document, if provided
	// – filename; Name for the file on the sender's device
	// – ha256; Hash
	// – mime_type; Mime type of the document file
	// – id; ID for the document
	//
	Document *Document `json:"document,omitempty"`

	// Reaction message you received from a customer.
	// You will not receive this webbook if the message the customer is reacting to is more than 30 days old.
	Reaction *Reaction `json:"reaction,omitempty"`

	// When the messages type field is set to button,
	// this object is included in the messages object:
	//
	// – payload; The payload for a button set up by the business that a customer clicked as part of an interactive message
	// – text; Button text
	//
	Button *Postback `json:"button,omitempty"`

	// Included in the messages object when a user replies or interacts with one of your messages.
	// The context object can contain the following fields:
	//
	// – forwarded; Set to true if the message received by the business has been forwarded
	// – frequently_forwarded; Set to true if the message received by the business has been forwarded more than 5 times.
	// – from; The WhatsApp ID for the customer who replied to an inbound message
	// – id; The message ID for the sent message for an inbound reply
	// – referred_product; Required for Product Enquiry Messages. The product the user is requesting information about. See Receive Response From Customers. The referred_product object contains the following fields:
	// – catalog_id; Unique identifier of the Meta catalog linked to the WhatsApp Business Account
	// – product_retailer_id; Unique identifier of the product in a catalog
	//
	Context *Context `json:"context,omitempty"`

	// A webhook is triggered when a customer's phone number or profile information has been updated.
	// See messages system identity
	//
	// – acknowledged; State of acknowledgment for the messages system customer_identity_changed
	// – created_timestamp; The time when the WhatsApp Business Management API detected the customer may have changed their profile information
	// – hash; The ID for the messages system customer_identity_changed
	//
	Identity interface{} `json:"identity,omitempty"`

	// When a customer has interacted with your message,
	// this object is included in the messages object.
	//
	// – type:
	//   – button_reply; Sent when a customer clicks a button
	//     – id; Unique ID of a button
	//     – title; Title of a button
	//   – list_reply: Sent when a customer selects an item from a list
	//     – id; Unique ID of the selected list item
	//     – title; Title of the selected list item
	//     – description; Description of the selected row
	//
	Interactive *Interactive `json:"interactive,omitempty"`

	// Included in the messages object when a customer has placed an order.
	// The order object can contain the following fields:
	//
	// – catalog_id; ID for the catalog the ordered item belongs to.
	// – text; Text message from the user sent along with the order.
	// – product_items; Array of product item objects containing the following fields:
	// – product_retailer_id; Unique identifier of the product in a catalog.
	// – quantity; Number of items.
	// – item_price; Price of each item.
	// – currency; Price currency.
	//
	Order interface{} `json:"order,omitempty"`

	// A customer clicked an ad that redirects them to WhatsApp,
	// this object is included in the messages object.
	//
	// – source_url; The Meta URL that leads to the ad or post clicked by the customer.
	//   Opening this url takes you to the ad viewed by your customer.
	// – source_type; The type of the ad’s source; ad or post
	// – source_id; Meta ID for an ad or a post
	// – headline; Headline used in the ad or post
	// – body; Body for the ad or post
	// – media_type; Media present in the ad or post; image or video
	// – image_url; URL of the image, when media_type is an image
	// – video_url; URL of the video, when media_type is a video
	// – thumbnail_url; URL for the thumbnail, when media_type is a video
	//
	Referral interface{} `json:"referral,omitempty"`

	// When messages type is set to system, a customer has updated their phone number or profile information,
	// this object is included in the messages object.
	//
	// – body; Describes the change to the customer's identity or phone number
	// – identity; Hash for the identity fetched from server
	// – new_wa_id; New WhatsApp ID for the customer when their phone number is updated. Available on webhook versions V11 and below
	// – wa_id; New WhatsApp ID for the customer when their phone number is updated. Available on webhook versions V12 and above
	// – type; Type of system update. Will be one of the following:
	// – customer_changed_number; A customer changed their phone number
	// – customer_identity_changed; A customer changed their profile information
	// – customer; The WhatsApp ID for the customer prior to the update
	//
	System interface{} `json:"system,omitempty"`

	// The message that a business received from a customer is not a supported type.
	Errors []interface{} `json:"errors,omitempty"`

	// Contacts shared by customer.
	Contacts []*Contact `json:"contacts,omitempty"`

	// Location GeoPoint shared by customer.
	Location *Location `json:"location,omitempty"`
}

// Included in the messages object when a user replies or interacts with one of your messages.
// The context object can contain the following fields:
//
// – forwarded; Set to true if the message received by the business has been forwarded
// – frequently_forwarded; Set to true if the message received by the business has been forwarded more than 5 times.
// – from; The WhatsApp ID for the customer who replied to an inbound message
// – id; The message ID for the sent message for an inbound reply
// – referred_product; Required for Product Enquiry Messages. The product the user is requesting information about. See Receive Response From Customers. The referred_product object contains the following fields:
// – catalog_id; Unique identifier of the Meta catalog linked to the WhatsApp Business Account
// – product_retailer_id; Unique identifier of the product in a catalog
type Context struct {
	// The message ID for the sent message for an inbound reply
	MID string `json:"id,omitempty"`
	// The WhatsApp ID for the customer who replied to an inbound message
	From string `json:"from,omitempty"`
	// Set to true if the message received by the business has been forwarded
	Forwarded bool `json:"forwarded,omitempty"`
	// Set to true if the message received by the business has been forwarded more than 5 times.
	FrequentlyForwarded bool `json:"frequently_forwarded,omitempty"`
	// Required for Product Enquiry Messages. The product the user is requesting information about. See Receive Response From Customers. The referred_product object contains the following fields:
	ReferredProduct *struct {
		// Unique identifier of the Meta catalog linked to the WhatsApp Business Account
		CatalogID string `json:"catalog_id,omitempty"`
		// Unique identifier of the product in a catalog
		ProductRetailerID string `json:"product_retailer_id,omitempty"`
	} `json:"referred_product,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#text-object
type Text struct {

	// Required for text messages.
	//
	// The text of the text message which can contain URLs which begin with http:// or https:// and formatting.
	// See available formatting options https://developers.facebook.com/docs/whatsapp/on-premises/reference/messages#formatting.
	//
	// If you include URLs in your text and want to include a preview box in text messages (preview_url: true),
	// make sure the URL starts with http:// or https:// —https:// URLs are preferred.
	// You must include a hostname, since IP addresses will not be matched.
	//
	// Maximum length: 4096 characters
	//
	Body string `json:"body"`

	// Optional. By default, WhatsApp recognizes URLs and makes them clickable,
	// but you can also include a preview box with more information about the link.
	// Set this field to true if you want to include a URL preview box.
	//
	// The majority of the time, the receiver will see a URL they can click on when you send an URL,
	// set preview_url to true, and provide a body object with a http or https link.
	//
	// URL previews are only rendered after one of the following has happened:
	// - The business has sent a message template to the user.
	// - The user initiates a conversation with a "click to chat" link.
	// - The user adds the business phone number to their address book and initiates a conversation.
	//
	// Default: false.
	//
	// If you have used the On-Premises API, you have seen this field being used inside the message object.
	// Please use preview_url inside the text object for Cloud API calls.
	//
	PreviewURL bool `json:"preview_url,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#media-object
type Media struct {

	// The media object ID.
	// Do not use this field when message type is set to text.
	// Required when `type` is oneof
	// * audio
	// * document
	// * image
	// * sticker
	// * video
	// and you are not using a link.
	ID string `json:"id,omitempty"`

	// Required when type is audio, document, image, sticker, or video
	// and you are not using an uploaded media ID (i.e. you are hosting the media asset on your server).
	//
	// The protocol and URL of the media to be sent. Use only with HTTP/HTTPS URLs.
	//
	// Do not use this field when message type is set to text.
	//
	Link string `json:"link,omitempty"`

	// Optional. Describes the specified image, document, or video media.
	// Do not use with audio or sticker media.
	Caption string `json:"caption,omitempty"`

	// Optional. Describes the filename for the specific document. Use only with document media.
	// The extension of the filename will specify what format the document is displayed as in WhatsApp.
	Filename string `json:"filename,omitempty"`

	// // Optional. Only used for On-Premises API.
	// // This path is optionally used with a link when the HTTP/HTTPS link is not directly accessible and requires additional configurations like a bearer token.
	// // For information on configuring providers, see the Media Providers documentation.
	// Provider string `json:"provider,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#messages-object

type Document struct {
	// Media Document
	Media

	// Document file hash.
	SHA256 string `json:"sha256,omitempty"`

	// MIME type for the image
	MIMEType string `json:"mime_type,omitempty"`

	// Size of the Media Document
	FileSize int64 `json:"file_size,omitempty"`
}

type Image struct {
	// Image Media Document
	Document
}

type Sticker struct {
	// Sticker Media Document
	Image

	// Set to true if the sticker is animated; false otherwise.
	Animated bool `json:"animated,omitempty"`
}

type Audio struct {
	// Audio Media Document
	Document
}

type Video struct {
	// Video Media Document
	Document
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#location-object
type Location struct {

	// REQUIRED.
	// Longitude of the location.
	Longitude float64 `json:"longitude"`

	// REQUIRED.
	// Latitude of the location.
	Latitude float64 `json:"latitude"`

	// OPTIONAL.
	// Name of the location.
	Name string `json:"name,omitempty"`

	// OPTIONAL.
	// Address of the location.
	// Only displayed if name is present.
	Address string `json:"address,omitempty"`
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#reaction-object
type Reaction struct {

	// REQUIRED.
	// The WhatsApp Message ID (wamid) of the message on which the reaction should appear.
	// The reaction will not be sent if:
	//
	// * The message is older than 30 days
	// * The message is a reaction message
	// * The message has been deleted
	// * If the ID is of a message that has been deleted, the message will not be delivered.
	WAMID string `json:"message_id"`

	// REQUIRED.
	// Emoji to appear on the message.
	//
	// All emojis supported by Android and iOS devices are supported.
	// Rendered-emojis are supported.
	// If using emoji unicode values, values must be Java- or JavaScript-escape encoded.
	// Only one emoji can be sent in a reaction message
	// Use an empty string to remove a previously sent emoji.
	Emoji string `json:"emoji"`
}

// QuickReply Button
type Postback struct {
	// Button text
	Text string `json:"text"`
	// The payload for a button set up by the business that a customer clicked as part of an interactive message
	Data string `json:"payload"`
}
