package viber

import (
	"mime"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/internal/util"
)

// SendMessage request options.
// Maximum total JSON size of the request is 30kb.
// Can send up to 100 messages to a user in an hour (XX:00-XX:00) without reply,
// message count towards the limit is reset when a user replies to a message.
// Once the limit is reached, you will receive an error callback saying
// {"status":12,"status_message":"maximum messages from Public Account to Viber user without reply exceeded.","message_token":XXXXXXXXXXXXXXXXXXX,"chat_hostname":"SN-CHAT-0x_"}
// https://developers.viber.com/docs/api/rest-bot-api/#send-message
type SendMessage struct {
	// Unique Viber user id.
	// REQUIRED. Subscribed valid user id
	PeerId string `json:"receiver"`
	// sendMessage options
	sendOptions
}

// method
func (*SendMessage) method() string {
	return "send_message"
}

// https://developers.viber.com/docs/api/rest-bot-api/#response
type SendResponse struct {
	// Base
	Status
	// Unique ID of the message
	MessageId uint64 `json:"message_token"`
	// Viber internal use
	Hostname string `json:"chat_hostname,omitempty"`
	// An indication of how this message is categorized for billing purposes,
	// allowing you to know if it was charged or not,
	// or whether it counts toward your monthly cap of free chatbot-initiated messages
	Billing int8 `json:"billing_status,omitempty"`
}

// BroadcastMessage request options.
// Maximum total JSON size of the request is 30kb.
// The maximum list length is 300 receivers.
// The Broadcast API is used to send messages to multiple recipients
// with a rate limit of 500 requests in a 10 seconds window.
// https://developers.viber.com/docs/api/rest-bot-api/#broadcast-message
type BroadcastMessage struct {
	// Recipients for the message.
	// REQUIRED. Subscribed valid user id list.
	// The maximum list length is 300 receivers.
	PeerId []string `json:"broadcast_list"`
	// sendMessage options
	sendOptions
}

func (*BroadcastMessage) method() string {
	return "broadcast_message"
}

type BroadcastStatus struct {
	PeerId string `json:"receiver"`
	Status
}

// https://developers.viber.com/docs/api/rest-bot-api/#response-1
type BroadcastResponse struct {
	Status
	MessageId  uint64             `json:"message_token,omitempty"`
	FailStatus []*BroadcastStatus `json:"failed_list,omitempty"`
}

// SendMessage request builder
type sendOptions struct {
	// Sender of the message content.
	// REQUIRED. `name` - max 28 characters.
	// OPTIONAL. `avatar` size should be no more than 100 kb. Recommended 720x720.
	Sender *User `json:"sender"`
	// OPTIONAL. Client version support the API version.
	// Certain features may not work as expected if set to a number that’s below their requirements.
	MinVersion int `json:"min_api_version,omitempty"`
	// The message content
	Message
	// File size in bytes
	FileSize int64 `json:"size,omitempty"`
	// Keyboard buttons layout
	Keyboard *Keyboard `json:"keyboard,omitempty"`
	// Carousel buttons layout
	RichMedia *Keyboard `json:"rich_media,omitempty"`
}

// Text of the message to be send
func (req *sendOptions) Text(text string) *sendOptions {

	msg := &req.Message
	text = strings.TrimSpace(text)
	mediaUrl, err := url.ParseRequestURI(text)
	if err == nil {
		// https://developers.viber.com/docs/api/rest-bot-api/#url-message
		msg.Type = mediaURL
		// REQUIRED. Max 2,000 characters
		msg.MediaURL = mediaUrl.String()
	} else {
		// https://developers.viber.com/docs/api/rest-bot-api/#text-message
		msg.Type = mediaText
		msg.Text = text
	}
	// chaining
	return req
}

const (
	// Size (bytes)
	Byte     = 1
	KiloByte = 1024 * Byte
	MegaByte = 1024 * KiloByte
	GigaByte = 1024 * MegaByte
	// Limit
	// Max image size: 1MB on iOS, 3MB on Android.
	ImageMaxSize = 3 * MegaByte
	// Max video size: 26 MB.
	VideoMaxSize = 26 * MegaByte
	// Max document size: 50 MB.
	FileMaxSize = 50 * MegaByte
)

// Media file content to be send
//
// NOTE: The URL must have a resource with a file extension as the last path segment.
// Example: http://www.example.com/path/image.jpeg
func (req *sendOptions) Media(media *chat.File, caption string) *sendOptions {

	// Parse media URL for downloading ...
	mediaUrl, err := url.ParseRequestURI(media.GetUrl())
	if err != nil {
		// ERR: Invalid media URL to reach the file content to download
		panic("send: invalid media URL; " + err.Error())
	}

	mimeType := strings.ToLower(media.GetMime())
	mediaType, _, _ := mime.ParseMediaType(mimeType)
	if mediaType == "" {
		mediaType = mimeType
	}
	// mimetype[/subtype] with no [;options]
	mimeType = mediaType
	// Normalize [/path/]filename.ext
	filename := path.Base(media.GetName())
	extension := path.Ext(filename)
	filename = filename[0 : len(filename)-len(extension)]
	// Default extension
	switch mimeType {
	case "image",
		"image/jpg",
		"image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "image/gif":
		extension = ".gif"
	case "video",
		"video/mp4",
		"video/mpeg",
		"video/mpeg4",
		"video/mpeg4-generic",
		"video/h264",
		"video/x264":
		extension = ".mp4"
	default:
		if extension == "" || extension == "." {
			extension = ""
			ext, _ := mime.ExtensionsByType(mediaType)
			if n := len(ext); n != 0 {
				extension = ext[n-1] // last
			}
		}
	}

	// Split: mimetype[/subtype]
	// var subType string
	if slash := strings.IndexByte(mediaType, '/'); slash > 0 {
		// subType = mediaType[slash+1:]
		mediaType = mediaType[0:slash]
	}
	// Default filename
	switch filename {
	case "", ".", "/":
		filename = mediaType
		switch mediaType {
		case "image":
		case "audio":
		case "video":
		default:
			filename = "file"
		}
		filename += time.Now().UTC().Format("_2006-01-02_15-04-05")
	}
	// Populate filename as the last URL segment
	filename += extension
	if !strings.EqualFold(path.Base(mediaUrl.Path), filename) {
		mediaUrl.Path = path.Join(mediaUrl.Path, "/", filename)
	}
	media.Url = mediaUrl.String()
	// Request content body ...
	msg := &req.Message

	// Media requirements
	//
	// type: pictire
	//
	// The URL must have a resource with a .jpeg, .png or .gif file extension
	// as the last path segment. Example: http://www.example.com/path/image.jpeg.
	// Animated GIFs can be sent as URL messages or file messages.
	// Max image size: 1MB on iOS, 3MB on Android.
	//
	// type: video
	//
	// REQUIRED. Max size 26 MB.
	// Only MP4 and H264 are supported.
	// The URL must have a resource with a .mp4 file extension
	// as the last path segment. Example: http://www.example.com/path/video.mp4
	//
	// type: file
	//
	// REQUIRED. Max size 50 MB. URL should include the file extension.
	// See forbidden file formats for unsupported file types:
	// https://developers.viber.com/docs/api/rest-bot-api/#forbiddenFileFormats
	//
	msg.Type = "" // clear
	size := media.Size
	switch mimeType { // lower(!)
	// picture
	case "image",
		"image/jpg",
		"image/jpeg",
		"image/png",
		"image/gif":

		if size <= ImageMaxSize {
			// https://developers.viber.com/docs/api/rest-bot-api/#picture-message
			msg.Type = mediaImage
			// REQUIRED. The URL must have a resource with a .jpeg, .png or .gif file extension as the last path segment.
			// Example: http://www.example.com/path/image.jpeg. Animated GIFs can be sent as URL messages or file messages.
			// Max image size: 1MB on iOS, 3MB on Android.
			msg.MediaURL = media.Url
			// Description of the photo
			// Can be an empty string if irrelevant
			// Max 512 characters
			msg.Text = strings.TrimSpace(caption)
		}

	// video
	case "video",
		"video/mp4",
		"video/mpeg",
		"video/mpeg4",
		"video/mpeg4-generic",
		// "video/vnd.directv.mpeg",
		// "video/vnd.directv.mpeg-tts",
		// "video/vnd.iptvforum.ttsmpeg2",
		// "video/vnd.dlna.mpeg-tts":
		"video/h264",
		"video/x264":

		if size <= VideoMaxSize {
			// https://developers.viber.com/docs/api/rest-bot-api/#video-message
			msg.Type = mediaVideo
			// URL of the video (MP4, H264)
			// REQUIRED. Max size 26 MB. Only MP4 and H264 are supported.
			// The URL must have a resource with a .mp4 file extension as the last path segment.
			// Example: http://www.example.com/path/video.mp4
			msg.MediaURL = media.Url
			// REQUIRED. Size of the video in bytes
			req.FileSize = media.Size // NOT: `file_size`, BUT `size` !
			// Video duration in seconds; will be displayed to the receiver
			// OPTIONAL. Max 180 seconds
			msg.Duration = 0
			// URL of a reduced size image (JPEG)
			// OPTIONAL. Max size 100 kb.
			// Recommended: 400x400.
			// Only JPEG format is supported
			msg.Thumbnail = ""
			// Description of the video
			// Can be an empty string if irrelevant
			// Max 512 characters
			msg.Text = strings.TrimSpace(caption)
		}
	}
	// default:
	if msg.Type == "" {
		// if size > FileMaxSize {
		// 	// FIXME: What TODO ?
		// }
		// https://developers.viber.com/docs/api/rest-bot-api/#file-message
		msg.Type = mediaFile
		// URL of the file.
		// REQUIRED. Max size 50 MB.
		// URL should include the file extension.
		// See forbidden file formats for unsupported file types
		msg.MediaURL = media.Url
		// REQUIRED. Size of the file in bytes
		req.FileSize = media.Size // NOT: `file_size`, BUT `size` !
		// Name of the file.
		// REQUIRED. File name should include extension.
		// Max 256 characters (including file extension).
		// Sending a file without extension or with the wrong extension
		// might cause the client to be unable to open the file.
		msg.FileName = filename
	}

	// chaining
	return req
}

// sendText creates and sends a text message to a list of peers via Viber.
// It initializes a single BroadcastMessage, sets the sender, peer IDs,
// and the text content, and returns the message.
func sendText(sender *User, peerId []string, text string) BroadcastMessage {
	var message BroadcastMessage

	message.Sender = sender
	message.PeerId = peerId
	message.Text(text)

	return message
}

// sendFile creates and sends a file (e.g., image, video) message with an caption to a list of peers via Viber.
// If the file's media type (image/video) and size exceed the predefined limits,
// the caption is sent as a separate text message.
func sendFile(sender *User, peerId []string, file *chat.File, caption string) (messageWithFile BroadcastMessage, messageWithCaption *BroadcastMessage) {
	messageWithFile.Sender = sender
	messageWithFile.PeerId = peerId
	messageWithFile.Media(file, caption)

	size := file.GetSize()
	caption = strings.TrimSpace(caption)
	mediaType := util.ParseMediaType(file.GetMime())
	isSentCaption :=
		(mediaType == "image" && size <= ImageMaxSize) ||
			(mediaType == "video" && size <= VideoMaxSize)

	if !isSentCaption && caption != "" {
		messageWithCaption = new(BroadcastMessage)

		messageWithCaption.Sender = sender
		messageWithCaption.PeerId = peerId
		messageWithCaption.Text(caption)
	}

	return messageWithFile, messageWithCaption
}
