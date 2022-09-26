package viber

import (
	"html"
	"strings"

	"github.com/webitel/chat_manager/api/proto/chat"
)

// Keyboard options
// https://developers.viber.com/docs/tools/keyboards/#general-keyboard-parameters
type Keyboard struct {
	// Keyboard Type ???
	// (3) keyboard is not valid. [object has missing required properties (["Type"])]
	Type string // const: "keyboard"
	// REQUIRED. Array containing all keyboard buttons by order.
	Buttons []*Button
	// OPTIONAL. Background color of the keyboard
	// Valid color HEX value e.g.: "#FFFFFF"
	// Default Viber keyboard background
	BgColor string `json:",omitempty"`
	// OPTIONAL. When true - the keyboard will always be displayed with the same height as the native keyboard.
	// When false - short keyboards will be displayed with the minimal possible height.
	// Maximal height will be native keyboard height.
	// Default: false.
	DefaultHeight bool `json:",omitempty"`
	// OPTIONAL (api level 3). How much percent of free screen space in chat should be taken by keyboard.
	// The final height will be not less than height of system keyboard
	// 40..70
	CustomDefaultHeight int `json:",omitempty"`
	// OPTIONAL (api level 3). Allow use custom aspect ratio for Carousel content blocks.
	// Scales the height of the default square block (which is defined on client side) to the given value in percents.
	// It means blocks can become not square and it can be used to create Carousel content with correct custom aspect ratio.
	// This is applied to all blocks in the Carousel content.
	// 20..100	Default: 100
	HeightScale int `json:",omitempty"`
	// OPTIONAL (api level 4). Represents size of block for grouping buttons during layout.
	// 1-6	    Default: 6
	ButtonsGroupColumns int `json:",omitempty"`
	// OPTIONAL (api level 4). Represents size of block for grouping buttons during layout.
	// 1-7      Default: 7 for Carousel content; 2 for Keyboard
	ButtonsGroupRows int `json:",omitempty"`
	// OPTIONAL (api level 4). Customize the keyboard input field.
	// regular - display regular size input field.
	// minimized - display input field minimized by default.
	// hidden - hide the input field
	InputFieldState string `json:",omitempty"`
	// // OPTIONAL (api level 6). JSON Object, which describes Carousel content to be saved via favorites bot, if saving is available.
	// // See: https://developers.viber.com/docs/tools/keyboards/#favorites-metadata
	// FavoritesMetadata *struct{}
}

// Button options
// https://developers.viber.com/docs/tools/keyboards/#buttons-parameters
type Button struct {
	// OPTIONAL. Button width in columns.
	// Valid: 1-6; Default: 6
	Columns int `json:",omitempty"`
	// OPTIONAL. Button height in rows.
	// Valid: 1-2 (1-7 for Rich Media messages); Default: 1
	Rows int `json:",omitempty"`
	// OPTIONAL. Background color of button
	// Valid: Valid color HEX value; Default: Viber button color
	BgColor string `json:",omitempty"`
	// OPTIONAL. Determine whether the user action is presented in the conversation
	Silent bool `json:",omitempty"`
	// OPTIONAL. Type of the background media
	// Valid: `picture`, `gif`. For `picture` - JPEG and PNG files are supported. Max size: 500 kb; Default: `picture`.
	BgMediaType string `json:",omitempty"`
	// OPTIONAL. URL for background media content (picture or gif).
	// Will be placed with aspect to fill logic.
	// Valid: URL
	BgMedia string `json:",omitempty"`
	// OPTIONAL (api level 6). Options for scaling the bounds of the background to the bounds of this view:
	// `crop` - contents scaled to fill with fixed aspect. some portion of content may be clipped.
	// `fill` - contents scaled to fill without saving fixed aspect.
	// `fit` - at least one axis (X or Y) will fit exactly, aspect is saved.
	BgMediaScaleType string `json:",omitempty"`
	// OPTIONAL (api level 6).
	// Options for scaling the bounds of an image to the bounds of this view:
	// `crop` - contents scaled to fill with fixed aspect. some portion of content may be clipped.
	// `fill` - contents scaled to fill without saving fixed aspect.
	// `fit` - at least one axis (X or Y) will fit exactly, aspect is saved.
	ImageScaleType string `json:",omitempty"`
	// OPTIONAL. When true - animated background media (gif) will loop continuously.
	// When false - animated background media will play once and stop.
	BgLoop bool `json:",omitempty"`
	// OPTIONAL. Type of action pressing the button will perform.
	// reply - will send a reply to the bot.
	// open-url - will open the specified URL and send the URL as reply to the bot.
	// Note: location-picker and share-phone are not supported on desktop, and require adding any text in the ActionBody parameter.
	// Valid: reply, open-url, location-picker, share-phone, none; Default: reply.
	ActionType string `json:",omitempty"`
	// REQUIRED. Text for reply and none.
	// ActionType or URL for open-url.
	// `reply` - text
	// `open-url` - URL
	ActionBody string
	// OPTIONAL. URL of image to place on top of background (if any).
	// Can be a partially transparent image that will allow showing some of the background.
	// Will be placed with aspect to fill logic
	Image string `json:",omitempty"`
	// OPTIONAL. Text to be displayed on the button.
	// Can contain some HTML tags - see keyboard design for more details
	Text string `json:",omitempty"`
	// OPTIONAL. Vertical alignment of the text
	// Valid: top, middle, bottom; Default: middle.
	TextVAlign string `json:",omitempty"`
	// OPTIONAL. Horizontal align of the text
	// Valid: left, center, right; Default: center.
	TextHAlign string `json:",omitempty"`
	// OPTIONAL (api level 4). Custom paddings for the text in points.
	// The value is an array of Integers [top, left, bottom, right]
	// Valid: per padding 0..12; Default: [12,12,12,12]
	TextPaddings []int `json:",omitempty"`
	// OPTIONAL. Text opacity
	// Valid: 0-100; Default: 100.
	TextOpacity int `json:",omitempty"`
	// OPTIONAL. Text size out of 3 available options
	// Valid: small, regular, large; Default: regular.
	TextSize string `json:",omitempty"`
	// OPTIONAL. Determine the `open-url` action result, in app or external browser.
	// Valid: internal, external. Default: internal.
	OpenURLType string `json:",omitempty"`
	// OPTIONAL. Determine the url media type.
	// not-media - force browser usage.
	// video - will be opened via media player.
	// gif - client will play the gif in full screen mode.
	// picture - client will open the picture in full screen mode
	// Valid: not-media, video, gif, picture; Default: not-media.
	OpenURLMediaType string `json:",omitempty"`
	// OPTIONAL. Background gradient to use under text, Works only when TextVAlign is equal to top or bottom.
	// Valid: Hex value (6 characters).
	TextBgGradientColor string `json:",omitempty"`
	// OPTIONAL. (api level 6) If true the size of text will decreased to fit (minimum size is 12).
	TextShouldFit bool `json:",omitempty"`
	// OPTIONAL (api level 3). Internal browser configuration for `open-url` action with internal type
	InternalBrowser *Browser `json:",omitempty"`
	// OPTIONAL (api level 6). JSON Object, which includes map configuration for `open-map` action with internal type.
	Map *Location `json:",omitempty"`
	// OPTIONAL (api level 6). Draw frame above the background on the button, the size will be equal the size of the button.
	Frame *Frame `json:",omitempty"`
	// OPTIONAL (api level 6). Specifies media player options.
	// Will be ignored if OpenURLMediaType is not `video` or `audio`.
	MediaPlayer *Player `json:",omitempty"`
}

// Internal Browser configuration
type Browser struct {
	// OPTIONAL (api level 3). Action button in internal’s browser navigation bar.
	// forward - will open the forward via Viber screen and share current URL or predefined URL.
	// send - sends the currently opened URL as an URL message, or predefined URL if property ActionPredefinedURL is not empty.
	// open-externally - opens external browser with the current URL.
	// send-to-bot - (api level 6) sends reply data in msgInfo to bot in order to receive message.
	// none - will not display any button.
	// Default: forward.
	ActionButton string `json:",omitempty"`
	// OPTIONAL (api level 3). If ActionButton is send or forward then
	// the value from this property will be used to be sent as message,
	// otherwise ignored
	ActionPredefinedURL string `json:",omitempty"`
	// OPTIONAL (api level 3). Type of title for internal browser if has no CustomTitle field.
	// default means the content in the page’s <OG:title> element or in <title> tag.
	// domain - means the top level domain.
	// Valid: domain, default; Default: default.
	TitleType string `json:",omitempty"`
	// OPTIONAL (api level 3). Custom text for internal’s browser title,
	// TitleType will be ignored in case this key is presented
	// Valid: String up to 15 characters.
	CustomTitle string `json:",omitempty"`
	// OPTIONAL (api level 3). Indicates that browser should be opened in a full screen or in partial size (50% of screen height).
	// Full screen mode can be with orientation lock (both orientations supported, only landscape or only portrait).
	// Valid: fullscreen, fullscreen-portrait, fullscreen-landscape, partial-size; Default: fullscreen.
	Mode string `json:",omitempty"`
	// OPTIONAL (api level 3). Should the browser’s footer will be displayed (default) or not (hidden)
	// Valid: default, hidden; Default: default.
	FooterType string `json:",omitempty"`
	// OPTIONAL (api level 6). Custom reply data for send-to-bot action that will be resent in msgInfo.
	ActionReplyData string `json:",omitempty"`
}

// Frame above the background on the button
type Frame struct {
	// OPTIONAL (api level 6). Width of border
	// Valid: 0..10; Default: 1.
	BorderWidth uint8 `json:",omitempty"`
	// OPTIONAL (api level 6). Color of border
	// #XXXXXX	#000000
	BorderColor string `json:",omitempty"`
	// OPTIONAL (api level 6). The border will be drawn with rounded corners
	// Valid: 0..10; Default: 0.
	CornerRadius uint8 `json:",omitempty"`
}

// Media Player options
type Player struct {
	// OPTIONAL (api level 6). Media player’s title (first line)
	Title string `json:",omitempty"`
	// OPTIONAL (api level 6). Media player’s subtitle (second line)
	Subtitle string `json:",omitempty"`
	// OPTIONAL (api level 6). The URL for player’s thumbnail (background)
	ThumbnailURL string `json:",omitempty"`
	// OPTIONAL (api level 6). Whether the media player should be looped forever or not.
	Loop bool `json:",omitempty"`
}

func coalesce(text ...string) string {
	for _, next := range text {
		next = strings.TrimSpace(next)
		if next != "" {
			return next
		}
	}
	return ""
}

// map[row.count]btn.width(columns)
// https://developers.viber.com/docs/tools/keyboards/#keyboard-design
var buttonsLayout = map[int][]int{
	1: {6},
	2: {3, 3},
	3: {2, 2, 2},
	4: {2, 2, 1, 1},
	5: {2, 1, 1, 1, 1},
	6: {1, 1, 1, 1, 1, 1},
}

// ButtonText default styling
func ButtonText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Has custom HTML ?
	if text[0] == '<' {
		return text
	}
	// Button's text default style ... // #ffc107 - yellow
	return "<font color=\"#ffffff\"><b>" + html.EscapeString(text) + "</b></font>"
}

// Default button(s) styling
var (
	buttonFrame = Frame{
		BorderColor: "#808d9d", // viber:light-gray
		// BorderColor:  "#ffc107", // webitel:yellow
		BorderWidth:  1,
		CornerRadius: 6,
	}
	keyboardButton = Button{
		TextSize: "large",
		BgColor:  "#1d2733", // viber:dark-gray
		// BgColor:  "#171a2a", // webitel:dark-blue
		// BgColor:  "#086d55",
		// Frame:      &buttonFrame,
	}
)

// NewButton returns a keyboard button with the default style
func NewButton(action, text, code string) *Button {
	// shallowcopy
	btn := keyboardButton
	frame := buttonFrame
	btn.Frame = &frame
	// constructor
	btn.ActionType = action
	btn.ActionBody = coalesce(code, text)
	btn.Text = ButtonText(coalesce(text, code))

	return &btn
}

func ButtonNone(text string) *Button {
	return NewButton(
		"none",
		text,
		"#none",
	)
}

func ButtonURL(text, url string) *Button {
	return NewButton(
		"open-url",
		text,
		url,
	)
}

func ButtonReply(text, code string) *Button {
	return NewButton(
		"reply",
		text,
		code,
	)
}

func ButtonContact(text string) *Button {
	return NewButton(
		"share-phone",
		"📱"+coalesce(text, "Share Contact"),
		"#contact",
	)
}

func ButtonLocation(text string) *Button {
	return NewButton(
		"location-picker",
		"📍"+coalesce(text, "Share Location"),
		"#location",
	)
}

func (req *sendOptions) Menu(layout []*chat.Buttons) *sendOptions {

	var size int
	for _, line := range layout {
		size += len(line.GetButton())
	}

	var (
		rows [][]*Button
		btns = make([]*Button, 0, size)
	)
	for _, line := range layout {
		r := len(rows)
		if r == 0 {
			rows = append(rows, nil) // first row
		} else if len(rows[r-1]) != 0 {
			rows = append(rows, nil) // add new row
		} else {
			r-- // back to previous due to no buttons
		}
		row := rows[r]
		// Note: keyboards can contain up to 24 rows.
		// https://developers.viber.com/docs/tools/keyboards/#keyboard-design
		for _, btn := range line.GetButton() {

			switch strings.ToLower(btn.Type) {
			case "url":
				row = append(row,
					ButtonURL(
						btn.GetText(),
						btn.GetUrl(),
					),
				)
			// case "postback":
			case "reply", "postback":
				row = append(row,
					ButtonReply(
						btn.GetText(),
						btn.GetCode(),
					),
				)

			case "location":
				row = append(row,
					ButtonLocation(
						btn.GetText(),
					),
				)
			case "contact", "phone":
				row = append(row,
					ButtonContact(
						btn.GetText(),
					),
				)
			case "email": // not-supported
			default:
				row = append(row,
					ButtonNone(
						btn.GetText(),
					),
				)
			}
		}
		rows[r] = row
	}

	for _, row := range rows {
		n := len(row)
		for c, btn := range row {
			btn.Rows = 1
			btn.Columns = buttonsLayout[n][c]
			btns = append(btns, btn)
		}
	}

	req.Keyboard = &Keyboard{
		// Type:       "rich_media",
		Type:          "keyboard",
		Buttons:       btns,
		DefaultHeight: false,
	}

	return req
}
