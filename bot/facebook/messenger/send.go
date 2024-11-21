package messenger

import (
	"strings"

	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
)

// https://developers.facebook.com/docs/messenger-platform/reference/send-api/#response
type SendResponse struct {
	// Unique ID for the user
	RecipientID string `json:"recipient_id,omitempty"`

	// Unique ID for the message
	MessageID string `json:"message_id,omitempty"`

	// Send error
	Error *graph.Error `json:"error,omitempty"`
}

// https://developers.facebook.com/docs/messenger-platform/reference/send-api/#properties
type SendRequest struct {
	// The messaging type of the message being sent.
	// For supported types and more information, see Sending Messages - Messaging Types
	Type string `json:"messaging_type,omitempty"`

	// Recipient object
	Recipient *SendRecipient `json:"recipient,omitempty"`

	// Message object. Cannot be sent with sender_action.
	Message *SendMessage `json:"message,omitempty"`

	// Message state to display to the user:
	//
	// typing_on: display the typing bubble
	// typing_off: remove the typing bubble
	// mark_seen: display the confirmation icon
	// Cannot be sent with message. Must be sent as a separate request.
	//
	// When using sender_action, recipient should be the only other property set in the request.
	Action string `json:"sender_action,omitempty"`

	// Optional. Push notification type:
	//
	// REGULAR: sound/vibration
	// SILENT_PUSH: on-screen notification only
	// NO_PUSH: no notification
	// Defaults to REGULAR.
	Notify string `json:"notification_type,omitempty"`

	// Optional. The message tag string. See Message Tags.
	Tag string `json:"tag,omitempty"`
}

type SendMessage struct {
	// Message text. Previews will not be shown for the URLs in this field. Use attachment instead. Must be UTF-8 and has a 2000 character limit. text or attachment must be set.
	Text string `json:"text,omitempty"`

	// Attachment object. Previews the URL. Used to send messages with media or Structured Messages. text or attachment must be set.
	Attachment *SendAttachment `json:"attachment,omitempty"`

	// Optional. Array of quick_reply to be sent with messages
	QuickReplies []*QuickReply `json:"quick_replies,omitempty"`

	// Optional. Custom string that is delivered as a message echo. 1000 character limit.
	Metadata string `json:"metadata,omitempty"`
}

type SendRecipient struct {

	// Page Scoped User ID (PSID) of the message recipient. The user needs to have interacted with any of the Messenger entry points in order to opt-in into messaging with the Page. Note that Facebook Login integrations return user IDs are app-scoped and will not work with the Messenger platform.
	ID string `json:"id,omitempty"`

	// Used for the checkbox plugin and customer chat plugin.
	UserREF string `json:"user_ref,omitempty"`

	// Used for Private Replies to reference the visitor post to reply to.
	PostID string `json:"post_id,omitempty"`

	// Used for Private Replies to reference the post comment to reply to.
	CommentID string `json:"comment_id,omitempty"`
}

// https://developers.facebook.com/docs/messenger-platform/reference/buttons/quick-replies#quick_reply
type QuickReply struct {

	// Must be one of the following
	//
	// text: Sends a text button
	// user_phone_number: Sends a button allowing recipient to send the phone number associated with their account.
	// user_email: Sends a button allowing recipient to send the email associated with their account.
	Type string `json:"content_type,omitempty"`

	// Required if content_type is 'text'. The text to display on the quick reply button. 20 character limit.
	Title string `json:"title,omitempty"`

	// Required if content_type is 'text'. Custom data that will be sent back to you via the messaging_postbacks webhook event. 1000 character limit.
	//
	// May be set to an empty string if image_url is set.
	Payload string `json:"payload,omitempty"` // String, Number

	// Optional. URL of image to display on the quick reply button for text quick replies. Image should be a minimum of 24px x 24px. Larger images will be automatically cropped and resized.
	//
	// Required if title is an empty string.
	ImageURL string `json:"image_url,omitempty"`
}

type SendAttachment struct {

	// Type of attachment, may be:
	// - image
	// - audio
	// - video
	// - file
	// - template
	// For assets, max file size is 25MB.
	Type string `json:"type,omitempty"`

	// Payload of attachment, can either be a
	// - Template Payload
	// - File Attachment Payload
	Payload interface{} `json:"payload,omitempty"`
}

// https://developers.facebook.com/docs/messenger-platform/reference/templates#payload
type TemplateAttachment struct {

	// Value indicating the template type:
	// - generic
	// - button
	// - media
	// - receipt
	// https://developers.facebook.com/docs/messenger-platform/reference/templates#available_templates
	TemplateType string `json:"template_type,omitempty"`

	// https://developers.facebook.com/docs/messenger-platform/reference/templates#available_templates

	// The button template allows you to send a structured message that includes text and buttons.
	*ButtonTemplate
	*GenericTemplate
}

// The generic template allows you to send a structured message that includes an image, text and buttons.
// https://developers.facebook.com/docs/messenger-platform/reference/templates/generic#payload
type GenericTemplate struct {
	// TemplateType: "generic"

	// Optional. The aspect ratio used to render images specified by element.image_url.
	// Must be horizontal (1.91:1) or square (1:1). Defaults to horizontal.
	ImageAspectRatio string `json:"image_aspect_ratio,omitempty"`

	// An array of element objects that describe instances of the generic template to be sent.
	// Specifying multiple elements will send a horizontally scrollable carousel of templates.
	// A maximum of 10 elements is supported.
	Elements []*GenericElement `json:"elements,omitempty"`
}

// The generic template supports a maximum of 10 elements per message.
// At least one property must be set in addition to title.
type GenericElement struct {
	// The title to display in the template. 80 character limit.
	Title string `json:"title"`

	// Optional. The subtitle to display in the template. 80 character limit.
	Subtitle string `json:"subtitle,omitempty"`

	// Optional. The URL of the image to display in the template.
	ImageURL string `json:"image_url,omitempty"`

	// Optional. The default action executed when the template is tapped.
	// Accepts the same properties as URL button, except title.
	DefaultAction *Button `json:"default_action,omitempty"`

	// Optional. An array of buttons to append to the template.
	// A maximum of 3 buttons per element is supported.
	Buttons []*Button `json:"buttons,omitempty"`
}

// The button template allows you to send a structured message that includes text and buttons.
// https://developers.facebook.com/docs/messenger-platform/reference/templates/button#payload
type ButtonTemplate struct {
	// TemplateType: "button"

	// UTF-8-encoded text of up to 640 characters. Text will appear above the buttons.
	Text string `json:"text,omitempty"`
	// Set of 1-3 buttons that appear as call-to-actions.
	Buttons []*Button `json:"buttons,omitempty"`
}

// https://developers.facebook.com/docs/messenger-platform/reference/buttons/postback#properties
type Button struct {
	// Type of button. Must be postback.
	Type string `json:"type,omitempty"`
	// Button title. 20 character limit.
	Title string `json:"title,omitempty"`
	// This data will be sent back to your webhook. 1000 character limit.
	Payload string `json:"payload,omitempty"`
	// Extensions:
	//
	// URL Button; https://developers.facebook.com/docs/messenger-platform/reference/buttons/url#properties
	// This URL is opened in a mobile browser when the button is tapped. Must use HTTPS protocol if messenger_extensions is true.
	URL string `json:"url,omitempty"`
	// Call Button; https://developers.facebook.com/docs/messenger-platform/reference/buttons/call#properties
	// Login Button; https://developers.facebook.com/docs/messenger-platform/reference/buttons/login#properties
	// . . .
}

// https://developers.facebook.com/docs/messenger-platform/reference/attachment-upload-api#payload
type FileAttachment struct {

	// Optional. URL of the file to upload. Max file size is 25MB (after encoding). A Timeout is set to 75 sec for videos and 10 secs for every other file type.
	URL string `json:"url,omitempty"`

	// Optional. Set to true to make the saved asset sendable to other message recipients. Defaults to false.
	IsReusable bool `json:"is_reusable,omitempty"`
}

// NewSendMessage is used to create new SendMessage struct
func NewSendMessage() *SendMessage {
	return &SendMessage{}
}

// SetFile is used to send a text in messsage
func (sm *SendMessage) SetText(text string) error {
	sm.Text = text

	return nil
}

// SetFile is used to send a file of type: image, audio, video, document
func (sm *SendMessage) SetFile(mimeType, url string) error {
	switch getMediaType(mimeType) {
	case "image":
		sm.Attachment = &SendAttachment{
			Type: "image",
			Payload: &FileAttachment{
				URL: url,
			},
		}

	case "audio":
		sm.Attachment = &SendAttachment{
			Type: "audio",
			Payload: &FileAttachment{
				URL: url,
			},
		}

	case "video":
		sm.Attachment = &SendAttachment{
			Type: "video",
			Payload: &FileAttachment{
				URL: url,
			},
		}

	default:
		sm.Attachment = &SendAttachment{
			Type: "document",
			Payload: &FileAttachment{
				URL: url,
			},
		}
	}

	return nil
}

// getMediaType parse mimetype and return file type, like: image, audio and video
func getMediaType(mtyp string) string {
	mtyp = strings.TrimSpace(mtyp)
	mtyp = strings.ToLower(mtyp)
	subt := strings.IndexByte(mtyp, '/')
	if subt > 0 {
		return mtyp[:subt]
	}
	return mtyp
}
