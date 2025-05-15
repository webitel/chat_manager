package infobip

import (
	"fmt"
	"strings"
	"time"
)

// "github.com/infobip/infobip-api-go-client/v2"

// SETUP
// 1. Get Purchased Numbers
// https://www.infobip.com/docs/api#platform-connectivity/numbers/list-purchased-numbers
// 2. Get all number configurations
// https://www.infobip.com/docs/api#platform-connectivity/numbers/list-configurations-for-number
// 3. Update number configuration
// https://www.infobip.com/docs/api#platform-connectivity/numbers/modify-sms-configurations

// https://www.infobip.com/docs/api#channels/whatsapp/receive-whatsapp-inbound-messages
type Updates struct { // NewMessage struct {
	// Collection of reports, one per every received message
	Results []*Update `json:"results"`
	// Number of returned messages in this request
	MessageCount int64 `json:"messageCount"`
	// Number of remaining new messages on Infobip servers ready to be returned in the next request
	PendingMessageCount int64 `json:"pendingMessageCount"`
}

// Report for every received message.
type Update struct {
	// Number which sent the message.
	From string `json:"from"`
	// Sender provided during the activation process.
	To string `json:"to"`
	// WHATSAPP
	Integration string `json:"integrationType,omitempty"`
	// Date and time when Infobip received the message.
	ReceivedAt Timestamp `json:"receivedAt,omitempty"`
	// The ID that uniquely identifies the received message.
	MessageID string `json:"messageId"`
	// Message content(s)
	Message *Message `json:"message"`
	// Information about recipient.
	Contact *Contact `json:"contact,omitempty"`
	// Message price.
	Price *Price `json:"price"`
}

// Contact info
type Contact struct {
	// Contact name
	Name string `json:"name"`
}

// New Message report
// https://www.infobip.com/docs/api#channels/whatsapp/receive-whatsapp-inbound-messages
type Message struct {
	// Type of the message content.
	// Available values are:
	// - TEXT
	// - IMAGE
	// - DOCUMENT
	// - STICKER
	// - LOCATION
	// - CONTACT
	// - TEXT
	// - VIDEO
	// - VOICE
	// - AUDIO
	// - BUTTON
	// - INTERACTIVE_BUTTON_REPLY
	// - INTERACTIVE_LIST_REPLY
	Type string `json:"type"`
	// Information about the message to which the end user responded.
	Context struct {
		// MessageId of the message to which the end user responded
		ID string `json:"id"`
		// End user's phone number
		From string `json:"from"`
		// Product information included in the incoming message
		ReferredProduct struct {
			// The ID that uniquely identifies the catalog registered with Facebook,
			// connected to the WhatsApp Business Account (WABA) the sender belongs to
			CatalogID string `json:"catalogId"`
			// Product unique identifier, as defined in catalog
			ProductRetailerID string `json:"productRetailerId"`
		} `json:"referredProduct"`
	} `json:"context,omitempty"`
	// Information about the identity of the end user.
	Identity struct {
		// Indicates whether identity is acknowledged
		Ack bool `json:"acknowledged"`
		// Identifier for the latest user_identity_changed system notification
		Hash string `json:"hash"`
		// Indicates when the identity was changed.
		// Has the following format: yyyy-MM-dd'T'HH:mm:ss.SSSZ
		CreatedAt Timestamp `json:"createdAt,omitempty"`
	} `json:"identity,omitempty"`
	// Content of the end user's message
	// Types: [TEXT, BUTTON]
	Text string `json:"text,omitempty"`
	// Types: [IMAGE, AUDIO, VIDEO, DOCUMENT]
	Caption string `json:"caption,omitempty"`
	// Types: [IMAGE, AUDIO, VIDEO, STICKER, DOCUMENT, LOCATION(url)]
	URL string `json:"url,omitempty"`

	// Types: [LOCATION]

	// Longitude. The value must be between -90 and 90.
	// Required. Number <double>
	Longitude float64 `json:"longitude,omitempty"`

	// Latitude. The value must be between -180 and 180.
	// Required. Number <double>
	Latitude float64 `json:"latitude,omitempty"`

	// Location name.
	// Optional
	Location string `json:"name,omitempty"`

	// Location address.
	// Optional
	Address string `json:"address,omitempty"`

	// Types: [BUTTON]

	// Payload of the selected button.
	Payload string `json:"payload,omitempty"`

	// Types: [INTERACTIVE_BUTTON_REPLY]

	// Identifier of the selected button.
	// Required. string [ 0 .. 256 ] characters
	CallbackData string `json:"id,omitempty"`

	// Title of the selected button.
	// Required. string [ 0 .. 20 ] characters
	CallbackTitle string `json:"title,omitempty"`

	// Types: [CONTACT]

	// Contacts information
	Contacts []*ContactInfo `json:"contacts,omitempty"`
}

type Price struct {
	// The currency in which the price is displayed
	Currency string `json:"currency"`
	// The price per individual message
	PricePerMessage float64 `json:"pricePerMessage"`
}

type SendContent interface {
	// API Endpoint to POST this type of content to ...
	endpoint() string
}

type SendRequest struct {
	// Registered WhatsApp sender number.
	// Must be in international format
	// and comply with WhatsApp's requirements.
	// Required. string [ 1 .. 24 ] characters
	From string `json:"from"`

	// Message recipient number.
	// Must be in international format.
	// Required. string [ 1 .. 24 ] characters
	To string `json:"to"`

	// The ID that uniquely identifies the message sent.
	// Optional. string [ 0 .. 50 ] characters
	MessageId string `json:"messageId,omitempty"`

	// The content object to build a message that will be sent.
	// Required.
	Content SendContent `json:"content"`

	// Custom client data that will be included in a Delivery Report.
	// string [ 0 .. 4000 ] characters
	CallbackData string `json:"callbackData,omitempty"`

	// The URL on your callback server to which delivery and seen reports will be sent.
	// Delivery report format, Seen report format.
	// Optional. string [ 0 .. 2048 ] characters
	NotifyURL string `json:"notifyUrl,omitempty"`
}

// https://www.infobip.com/docs/api#channels/whatsapp/send-whatsapp-text-message
type SendTextMessage struct {

	// Content of the message being sent.
	// Required. string [ 1 .. 4096 ] characters
	Text string `json:"text"`

	// Allows for URL preview from within the message.
	// If set to true, the message content must contain a URL
	// starting with https:// or http://. Defaults to false.
	// Optional.
	PreviewURL bool `json:"previewUrl,omitempty"`
}

func (e *SendTextMessage) endpoint() string {
	return "/whatsapp/1/message/text"
}

// https://www.infobip.com/docs/api#channels/whatsapp/send-whatsapp-document-message
type SendMediaMessage struct {

	// Media Content Type
	MediaType string `json:"-"`
	// URL of a document sent in a WhatsApp message.
	// Must be a valid URL starting with https:// or http://.
	// Maximum document size is 100MB.
	// Required (IMAGE, AUDIO, VOICE, VIDEO, STICKER, DOCUMENT").
	// string [ 1 .. 2048 ] characters
	MediaURL string `json:"mediaUrl"`

	// File name of the document.
	// Optional (type: DOCUMENT). string [ 0 .. 240 ] characters
	Filename string `json:"filename,omitempty"`

	// Caption of the document.
	// string [ 0 .. 3000 ] characters
	// Optional (IMAGE, AUDIO, VOICE, VIDEO, DOCUMENT").
	Caption string `json:"caption,omitempty"`
}

const (
	MediaImage = "IMAGE"
	MediaAudio = "AUDIO"
	MediaVideo = "VIDEO"
	MediaFile  = "DOCUMENT"
)

func mediaType(mime string) string {
	mtype := mime
	// Extract: type[/subtype][; param=value]+
	mopts := strings.IndexAny(mtype, "/;")

	if mopts > 0 {
		mtype = mtype[0:mopts]
	}

	mtype = strings.ToUpper(mtype)
	switch mtype {
	case MediaImage,
		MediaAudio,
		MediaVideo:
		// OK
	default:
		mtype = MediaFile
	}

	return mtype
}

func (e *SendMediaMessage) endpoint() string {

	switch mediaType(e.MediaType) {
	case MediaImage:
		return "/whatsapp/1/message/image"
	case MediaAudio:
		return "/whatsapp/1/message/audio"
	case MediaVideo:
		return "/whatsapp/1/message/video"
	default: // MediaFile
		return "/whatsapp/1/message/document"
	}
}

type InteractiveHeader struct {

	// Type of the header content. Required.
	// - TEXT
	// - VIDEO
	// - IMAGE
	// - DOCUMENT
	Type string `json:"type"`

	// Content of the header used when creating interactive buttons.
	// Required (type: TEXT)
	Text string `json:"text,omitempty"`

	// DOCUMENT
	// URL of a document sent in the header of a message containing one or more interactive buttons. Must be a valid URL starting with https:// or http://. Supported document type is PDF. Maximum document size is 100MB.
	// Required (type: IMAGE, VIDEO, DOCUMENT).
	// string [ 1 .. 2048 ] characters
	MediaURL string `json:"mediaUrl,omitempty"`

	// Filename of the document.
	// Optional (type: DOCUMENT). string [ 0 .. 240 ] characters
	Filename string `json:"filename,omitempty"`
}

type InteractiveFooter struct {
	// Content of the message footer.
	// Required. string [ 1 .. 60 ] characters
	Text string `json:"text"`
}

// https://www.infobip.com/docs/api#channels/whatsapp/send-whatsapp-interactive-buttons-message
type InteractiveButtonsMessage struct {

	// Header of a message containing one or more interactive elements.
	// Optional.
	Header *InteractiveHeader `json:"header,omitempty"`

	// Body of a message containing one or more interactive elements.
	// Required
	Body struct {

		// Content of the message body.
		// Rquired. string [ 1 .. 1024 ] characters
		Text string `json:"text"`
	} `json:"body"`

	// Footer of a message containing one or more interactive elements.
	// Optional.
	Footer *InteractiveFooter `json:"footer,omitempty"`

	// Allows you to specify buttons sent in the message.
	// Required.
	Action struct {

		// An array of buttons sent in a message.
		// It can have up to three buttons.
		// Required. Array of objects [ 1 .. 3 ] items
		Buttons []Button `json:"buttons"`
	} `json:"action"`
}

func (e *InteractiveButtonsMessage) endpoint() string {
	// TEXT
	return "/whatsapp/1/message/interactive/buttons"
}

type Button struct {

	// Unique identifier of the button.
	// Required. string [ 1 .. 256 ] characters
	ID string `json:"id"`

	// REPLY
	// Required. string
	Type string `json:"type"`

	// Unique title of the button. Doesn't allow emojis or markdown.
	// Required. string [ 1 .. 20 ] characters
	Title string `json:"title"`
}

type Timestamp time.Time

func (t *Timestamp) UnmarshalText(data []byte) error {
	dt, err := time.Parse("2006-01-02T15:04:05.000-0700", string(data))
	if err != nil {
		return err
	}
	*(*time.Time)(t) = dt
	return nil
}

type SendResponse struct {

	// The destination address of the message.
	To string `json:"to"`

	// Number of messages required to deliver.
	Pending int32 `json:"messageCount"`

	// The ID that uniquely identifies the message sent.
	// If not passed, it will be automatically generated
	// and returned in a response.
	MessageID string `json:"messageId"`

	// Indicates the status of the message
	// and how to recover from an error should there be any.
	Status *MessageStatus `json:"status"`

	//
	Error *RequestError `json:"requestError,omitempty"`
}

type MessageStatus struct {

	// Status group ID.
	GroupID int32 `json:"groupId,omitempty"`

	// Status group name.
	GroupName string `json:"groupName,omitempty"`

	// Status ID.
	ID int32 `json:"id,omitempty"`

	// Status name.
	Name string `json:"name,omitempty"`

	// Human-readable description of the status.
	Detail string `json:"description,omitempty"`

	// Action that should be taken to eliminate the error.
	Action string `json:"action,omitempty"`
}

type RequestError struct {
	Exception *ServiceError `json:"serviceException,omitempty"`
}

func (e *RequestError) Error() string {
	return e.Exception.Error()
}

type ServiceError struct {

	// Identifier of the error.
	ID string `json:"messageId,omitempty"`

	// Detailed error description.
	Message string `json:"text,omitempty"`

	// Map of validation errors.
	Validations map[string][]string `json:"validationErrors,omitempty"`
}

func (e *ServiceError) Error() string {
	for param, errs := range e.Validations {
		return param + ": " + errs[0] // Any(!)
	}
	return fmt.Sprintf("(#%s) %s", e.ID, e.Message)
}

type StatusError struct {
	Code     int
	Status   string
	Response []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("(#%d) %s ; %s", e.Code, e.Status, e.Response)
}
