package whatsapp

// Indicates where a conversation has started.
type OriginType string

const (
	// Indicates that the conversation started by a business sending the first message to a customer.
	// This applies any time it has been more than 24 hours since the last customer message.
	OriginBusiness = OriginType("business_initiated")
	// Indicates that the conversation started by a business replying to a customer message.
	// This applies only when the business reply is within 24 hours of the last customer message.
	OriginCustomer = OriginType("customer_initiated")
	// Indicates that the conversation originated from a free entry point.
	// These conversations are always customer-initiated.
	OriginReferral = OriginType("referral_conversion")

	//
	OriginUser = OriginType("user_initiated")
)

// Indicates who initiated the conversation
type Origin struct {
	// Indicates where a conversation has started.
	// This can also be referred to as a conversation entry point
	Type OriginType `json:"type"`
}

// Information about the conversation.
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
type Conversation struct {
	// ID of the conversation.
	ID string `json:"id"`
	// Origination info
	Origin *Origin `json:"origin,omitempty"`
	// Date when the conversation expires.
	// This field is only present for messages with a `status` set to `sent`.
	Expiry *Timestamp `json:"expiration_timestamp,omitempty"`
}
