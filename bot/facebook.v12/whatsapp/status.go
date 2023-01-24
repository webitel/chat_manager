package whatsapp

// Status of the message
type StatusText string

const (
	// A business sends a message to a customer
	StatusSent = StatusText("sent")
	// A message sent by a business has been delivered
	StatusDelivered = StatusText("delivered")
	// A message sent by a business has been read
	StatusRead = StatusText("read")
)

// The statuses object is nested within the value object and is triggered when
// a message is sent or delivered to a customer or the customer reads the delivered message
// sent by a business that is subscribed to the Webhooks.
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#statuses-object
type Status struct {

	// The ID for the message that the business that is subscribed to the webhooks sent to a customer
	MessageID string `json:"id,omitempty"`

	// The WhatsApp ID for the customer that the business,
	// that is subscribed to the webhooks, sent to the customer.
	RecipientID string `json:"recipient_id,omitempty"`

	// * delivered; A webhook is triggered when a message sent by a business has been delivered
	// * read; A webhook is triggered when a message sent by a business has been read
	// * sent; A webhook is triggered when a business sends a message to a customer
	Status StatusText `json:"status,omitempty"`

	// Date for the status message
	Date *Timestamp `json:"timestamp,omitempty"`

	// Information about the conversation.
	//
	// – id; Represents the ID of the conversation the given status notification belongs to.
	// – origin (object); Indicates who initiated the conversation
	//   – type; Indicates where a conversation has started. This can also be referred to as a conversation entry point
	//     * business_initiated; Indicates that the conversation started by a business sending the first message to a customer. This applies any time it has been more than 24 hours since the last customer message.
	//     * customer_initiated; Indicates that the conversation started by a business replying to a customer message. This applies only when the business reply is within 24 hours of the last customer message.
	//     * referral_conversion; Indicates that the conversation originated from a free entry point. These conversations are always customer-initiated.
	// – expiration_timestamp – Date when the conversation expires. This field is only present for messages with a `status` set to `sent`.
	//
	// WhatsApp defines a conversation as a 24-hour session of messaging between a person and a business.
	// There is no limit on the number of messages that can be exchanged in the fixed 24-hour window.
	// The 24-hour conversation session begins when:
	// * A business-initiated message is delivered to a customer
	// * A business’ reply to a customer message is delivered
	//
	// The 24-hour conversation session is different from the 24-hour customer support window.
	// The customer support window is a rolling window that is refreshed when a customer-initiated message is delivered to a business.
	// Within the customer support window businesses can send free-form messages.
	// Any business-initiated message sent more than 24 hours after the last customer message must be a template message.
	//
	Conversation *Conversation `json:"conversation,omitempty"`

	// An object containing billing information.
	//
	// // – billable; Indicates if the given message or conversation is billable.
	// //   Default is true for all conversations, including those inside your free tier limit,
	// //   except those initiated from free entry points. Free entry point conversatsion are not billable, false.
	// //   You will not be charged for free tier limit conversations, but they are considered billable and will be reflected on your invoice.
	// //   Deprecated. Visit the WhatsApp Changelog for more information.
	// – category; Indicates the conversation pricing category:
	// – business_initiated; The business sent a message to a customer more than 24 hours after the last customer message
	// – referral_conversion; The conversation originated from a free entry point. These conversations are always customer-initiated.
	// – customer_initiated; The business replied to a customer message within 24 hours of the last customer message
	// – pricing_model; Type of pricing model used by the business. Current supported value is CBP
	//
	Pricing *Pricing `json:"pricing,omitempty"`
}

// Billing information
type Pricing struct {

	// Indicates if the given message or conversation is billable.
	// Default is true for all conversations, including those inside your free tier limit, except those initiated from free entry points.
	// Free entry point conversatsion are not billable, false.
	// You will not be charged for free tier limit conversations, but they are considered billable and will be reflected on your invoice.
	//
	// Deprecated. Visit the WhatsApp Changelog for more information.
	// https://developers.facebook.com/docs/whatsapp/business-platform/changelog
	//
	Billable bool `json:"billable,omitempty"`

	// The conversation pricing category
	Category OriginType `json:"category,omitempty"`

	// Type of pricing model used by the business.
	// Current supported value is "CBP".
	Model string `json:"pricing_model,omitempty"`
}
