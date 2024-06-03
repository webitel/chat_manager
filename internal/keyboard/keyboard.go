package keyboard

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/micro/micro/v3/service/errors"
	v1 "github.com/webitel/chat_manager/api/proto/chat"
	v2 "github.com/webitel/chat_manager/api/proto/chat/messages"
)

func MarkupV2(src []*v1.Buttons) (*v2.ReplyMarkup, error) {
	rc := len(src)
	if rc < 1 {
		// FIXME: <nil> | [0:0] ?
		return nil, nil
	}
	dst := &v2.ReplyMarkup{
		Buttons: make([]*v2.ButtonRow, 0, rc),
	}
	// var r, c int // [r]ow | [c]olumn
	var (
		r, _    int // [r]ow | [c]olumn
		row2    *v2.ButtonRow
		lastRow = func() *v2.ButtonRow {
			if r < len(dst.Buttons) {
				return dst.Buttons[r]
			}
			return nil
		}
		nextRow = func() *v2.ButtonRow {
			row2 := lastRow()
			if row2 == nil || len(row2.Row) > 0 {
				// <nil> -or- non-empty !
				r = len(dst.Buttons)
				row2 = new(v2.ButtonRow)
				dst.Buttons = append(dst.Buttons, row2)
			} // else {
			// 	// NOT <nil> -but- empty ! [re]write last ..
			// }
			return row2
		}
		// remove_keyboard ?
		remove = false
		// unique replies index
		replies = make(map[any]*v2.Button)
		// hasButton
		nextBtn = func(src *v1.Button) (dst *v2.Button, err error) {
			if remove {
				return // nil, nil
			}
			if src == nil {
				return // nil, nil
			}
			var PK any
			defer func() {
				if PK == nil {
					return
				}
				if _, ok := replies[PK]; ok {
					dst = nil // duplicate
					return
				}
				replies[PK] = dst
			}()
			typeOf := src.Type
			typeOf = strings.ToLower(typeOf)
			switch typeOf {
			case "clear", "remove", "remove_keyboard":
				// telegram.NewRemoveKeyboard(true)
				remove = true
			case "email", "mail":
				dst = &v2.Button{
					Text: coalesce(src.Text, src.Caption, typeOf),
					Type: &v2.Button_Share{
						Share: v2.Button_email, // normalize
					},
				}
				PK = *dst.Type.(*v2.Button_Share)
			case "phone", "contact":
				dst = &v2.Button{
					Text: coalesce(src.Text, src.Caption, typeOf),
					Type: &v2.Button_Share{
						Share: v2.Button_Request(
							v2.Button_Request_value[typeOf],
						),
					},
				}
				PK = *dst.Type.(*v2.Button_Share)
			case "location":
				dst = &v2.Button{
					Text: coalesce(src.Text, src.Caption, typeOf),
					Type: &v2.Button_Share{
						Share: v2.Button_location, // normalize
					},
				}
				PK = *dst.Type.(*v2.Button_Share)
			case "url":
				nav, re := url.ParseRequestURI(src.Url)
				if err = re; err != nil {
					err = errors.BadRequest(
						"messages.keyboard.button.url.invalid",
						"keyboard: %v", err,
					)
					return nil, err
				}
				if !nav.IsAbs() {
					err = errors.BadRequest(
						"messages.keyboard.button.url.invalid",
						"keyboard: absolute URL required; scheme: missing",
					)
					return
				}
				// if nav.Fragment != "" {
				// }
				caption := coalesce(src.Text, src.Caption)
				if caption == "" {
					caption = fmt.Sprintf(
						"%s://%s", nav.Scheme, nav.Host,
					)
				}
				dst = &v2.Button{
					Text: caption,
					Type: &v2.Button_Url{
						Url: nav.String(), // REQUIRE: request URL
					},
				}
				PK = v2.Button_Url{Url: caption}
			default:
				// case "reply", "postback":
				caption := coalesce(src.Text, src.Caption)
				callback := coalesce(src.Code, caption)
				dst = &v2.Button{
					Text: coalesce(caption, callback),
					Type: &v2.Button_Code{
						Code: callback,
					},
				}
				PK = *dst.Type.(*v2.Button_Code)
			}
			// none
			return // dst, nil
		}
	)
	// convert
	for _, row1 := range src {
		row2 = nextRow()
		for _, btn1 := range row1.GetButton() {
			btn2, err := nextBtn(btn1)
			if err != nil {
				return dst, err
			}
			if remove {
				// remove: keyboard ?
				dst.Buttons = nil
				return dst, nil
			}
			if btn2 != nil {
				row2.Row = append(
					row2.Row, btn2,
				)
			}
		}
	}
	row2 = lastRow()
	if row2 != nil && len(row2.GetRow()) < 1 {
		dst.Buttons = dst.Buttons[0:r] // -1
	}
	return dst, nil
}

func coalesce(text ...string) string {
	for _, vs := range text {
		vs = strings.TrimSpace(vs)
		if vs != "" {
			return vs
		}
	}
	return ""
}
