package messenger

import (
	"github.com/webitel/chat_manager/bot/facebook.v12/webhooks"
)

// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#format
// var (
// 	Batch []*Entry
// 	Event = webhooks.Event{
// 		Object: "page",
// 		Entry:  &Batch,
// 	}
// )

// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#entry
type Entry struct {
	// // Page ID of page
	// ID string `json:"id,omitempty"`
	// // Time of update (epoch time in milliseconds)
	// Time int64 `json:"time,omitempty"`
	webhooks.Entry // Base
	// Array containing one messaging object.
	// Note that even though this is an array,
	// it will only contain one messaging object.
	Messaging []*Messaging `json:"messaging,omitempty"`
	// Array of messages received in the standby channel.
	Standby []*Messaging `json:"standby,omitempty"`
}

type Messaging struct {
	// Sender user ID. sender.id: <PSID>
	// The PSID of the user that triggered the webhook event.
	Sender *Account `json:"sender,omitempty"`
	// Recipient user ID. recipient.id: <PAGE_ID>
	// Your Page ID.
	Recipient *Account `json:"recipient,omitempty"`

	// Timestamp
	Timestamp int64 `json:"timestamp,omitempty"`
	// messages. Message has been sent to your Page.
	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
	Message *Message `json:"message,omitempty"`
	// messaging_postbacks. Postback button, Get Started button, or persistent menu item is tapped.
	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
	*Postback `json:"postback,omitempty"`
	// message_deliveries
	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/message-deliveries
	// *Delivery `json:"delivery,omitempty"`
	// messaging_handovers
	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_handovers
	PassThreadControl    *pass_thread_control    `json:"pass_thread_control,omitempty"`
	TakeThreadControl    *take_thread_control    `json:"take_thread_control,omitempty"`
	RequestThreadControl *request_thread_control `json:"request_thread_control,omitempty"`
	AppRoles             app_roles               `json:"app_roles,omitempty"`
}

type Account struct {
	// The PSID of the user that triggered the webhook event.
	ID string `json:"id,omitempty"`
	// The user_ref of the user that triggered the webhook event. This is only available for webhook event from the chat plugin.
	UserRef string `json:"user_ref,omitempty"`
}

// Postbacks occur when a postback button, Get Started button, or persistent menu item is tapped.
// You can subscribe to this callback by selecting the `messaging_postbacks` field when setting up your webhook.
// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
type Postback struct {
	// Message ID
	MessageID string `json:"mid,omitempty"`
	// Title for the CTA that was clicked on.
	// This is sent to all apps subscribed to the page.
	// For apps other than the original CTA sender,
	// the postback event will be delivered via the standby channel.
	Title string `json:"title,omitempty"`
	// Payload parameter that was defined with the button.
	// This is only visible to the app that send the original template message.
	Payload string `json:"payload,omitempty"`
	// Referral information for how the user got into the thread. Structure follows referral event:
	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_referrals#referral
	Referral interface{} `json:"referral,omitempty"`
}

// Message callback will occur when a message has been sent to your Page.
// Messages are always sent in order. You may receive text messages or messages with attachments.
//
// Attachment types image, audio, video, file, or location are the main supported types.
// You may also receive fallback attachments. A common example of a 'fallback' is when
// a user shares a URL with a Page, an attachment is created based on link sharing.
// For unsupported shares made by users to your Page a fallback with no payload might be sent.
//
// You can subscribe to this callback by selecting message when setting up your webhook.
type Message struct {
	// Message ID
	ID string `json:"mid"`
	// Text of message
	Text string `json:"text,omitempty"`
	// Included when your business sends a message to the customer
	IsEcho bool `json:"is_echo,omitempty"`
	// Included when a customer deletes a message
	IsDeleted bool `json:"is_deleted,omitempty"`
	// Included when a customer sends a message with unsupported media
	IsUnsupported bool `json:"is_unsupported,omitempty"`
	// Reference to the message id (mid) that this message is replying to
	ReplyTo *ReplyTo `json:"reply_to,omitempty"`
	// Optional custom data provided by the sending app
	QuickReply *QuickReply `json:"quick_reply,omitempty"`
	// Array containing attachment data
	Attachments []*Attachment `json:"attachments,omitempty"`
	// Referral of the message from Shops product details page.
	Referral *Referral `json:"referral,omitempty"`
}

// // A quick_reply payload is only provided with a text message when the user tap on a Quick Replies button.
// type QuickReply struct {
// 	// Custom data provided by the app
// 	Payload string `json:"payload"`
// }

// Story reference
type Story struct {
	// The Story ID.
	ID string `json:"id"`
	// CDN Media content URL.
	URL string `json:"url"`
}

type ReplyTo struct {
	// Reference to the message ID that this message is replying to
	MessageID string `json:"mid,omitempty"`
	// Instagram Story
	Story *Story `json:"story,omitempty"`
}

// Referral payload is only provided when the user sends a message from Shops product detail page.
type Referral struct {
	Product struct {
		// Reference to the message ID that this message is replying to
		ID string `json:"id"`
	} `json:"product"`
}

type Attachment struct {
	// audio, file, image, location, video or fallback
	Type    string     `json:"type"`
	Payload attachment `json:"payload"` // attachment
}

type attachment struct {
	// URL of the attachment type.
	// Applicable to attachment type: audio, file, image, location, video, fallback
	URL string `json:"url,omitempty"`
	// Title of the attachment.
	// Applicable to attachment type: fallback
	Title string `json:"title,omitempty"`
	// Persistent id of this sticker, for example 369239263222822 references the Like sticker.
	// Applicable to attachment type: image only if a sticker is sent.
	StickerID int64 `json:"sticker_id,omitempty"`
	// Coordinates. Applicable to attachment type: location
	Coordinates *struct {
		Latitude  float64 `json:"lat"`  // Number ?
		Longitude float64 `json:"long"` // Number ?
	} `json:"coordinates,omitempty"`

	*Product `json:"product,omitempty"`
}

type Product struct {
	Elements []*product `json:"elements"`
}

type product struct {
	// Product ID from Facebook product catalog
	ID string `json:"id,omitempty"`
	// External ID that is associated with the Product. (ex: SKU/ Content ID)
	RetailerID string `json:"retailer_id,omitempty"`
	// URL of product
	ImageURL string `json:"image_url,omitempty"`
	// Title of product
	Title string `json:"title,omitempty"`
	// Subtitle of product
	Subtitle string `json:"subtitle,omitempty"`
}
