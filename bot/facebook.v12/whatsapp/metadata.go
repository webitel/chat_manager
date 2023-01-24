package whatsapp

// Metadata for the business that is subscribed to the webhook
type Metadata struct {
	// The phone number that is displayed for a business.
	DisplayPhoneNumber string `json:"display_phone_number"`
	// ID for the phone number. A business can respond to a message using this ID.
	PhoneNumberID string `json:"phone_number_id"`
}
