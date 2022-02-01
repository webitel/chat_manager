package webhooks

import "net/http"

const (

	ObjectUser          = "user"
	ObjectPage          = "page"
	ObjectPermissions   = "permissions"
	ObjectPayments      = "payments"
)

// https://developers.facebook.com/docs/graph-api/reference/app/subscriptions#read
type Subscription struct {
	// Indicates whether or not the subscription is active.
	Active      bool     `json:"active,omitempty"`
	// Indicates the object type that this subscription applies to.
	// enum{ user, page, permissions, payments}
	Object      string   `json:"object,omitempty"`
	// The set of fields in this object that are subscribed to.
	Fields      []string `json:"fields,omitempty"`
	// The URL that will receive the POST request when an update is triggered,
	// and a GET request when attempting to publish operation.
	CallbackURL string   `json:"callback_url,omitempty"`
}

func (hub *Subscription) Verification(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// Subscriptions map[.Object].Subscription
type Subscriptions map[string]*Subscription