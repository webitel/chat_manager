package whatsapp

import "encoding/json"

// Update Entry of event notification.
// The Update value object contains details for the change that triggered the webhook.
// This object is nested within the changes array of the entry array.
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#value-object
type Update struct {
	// The [W]hats[A]pp [B]usiness[A]ccount [ID]
	// Populated from update.entry.id parameter.
	ID string `json:"-"`

	// The value is "whatsapp".
	Product string `json:"messaging_product,omitempty"`

	// Metadata for the business that is subscribed to the webhook.
	// This object contains the following fields:
	//
	// – display_phone_number; The phone number that is displayed for a business.
	// – phone_number_id; ID for the phone number.
	//   A business can respond to a message using this ID.
	//
	Metadata *Metadata `json:"metadata,omitempty"`

	// Array of contacts objects with information for the customer who sent a message to the business.
	// The object can contain the following fields:
	//
	// – wa_id; The customer's WhatsApp ID.
	//   A business can respond to a message using this ID.
	// – profile; An object containing customer profile information.
	//   The profile object can contain the following field:
	//   – name; The customer’s name
	//
	Contacts []*Sender `json:"contacts,omitempty"`

	// Information about a message received by the business that is subscribed to the webhook.
	Messages []*Message `json:"messages,omitempty"`

	Calls []json.RawMessage `json:"calls"`

	// Status for a message that was sent by the business that is subscribed to the webhook.
	Statuses []*Status `json:"statuses,omitempty"`

	// Array of error objects with information received when a message failed.
	// The error object contains the following fields:
	//
	// – code; Error code
	// – title; Error title
	//
	Errors []*MessageError `json:"errors,omitempty"`
}

// Find Contact by WAID string
func (e *Update) GetContact(WAID string) *Sender {
	if e != nil {
		for _, sender := range e.Contacts {
			if sender.WAID == WAID {
				return sender
			}
		}
	}
	return nil
}
