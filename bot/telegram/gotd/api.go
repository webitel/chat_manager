package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gotd/td/tgerr"
	"github.com/micro/micro/v3/service/errors"
)

func writeJSON(w http.ResponseWriter, res interface{}, code int) {

	if code == 0 {
		code = http.StatusOK
	}

	header := w.Header()
	header.Set("Pragma", "no-cache")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "close")
	header.Set("Content-Type", "application/json; charset=utf-8")

	w.WriteHeader(code)

	codec := json.NewEncoder(w)
	codec.SetEscapeHTML(false)
	codec.SetIndent("", "  ")

	_ = codec.Encode(res)
}

func writeTgError(w http.ResponseWriter, err error, code int) {

	if code == 0 {
		code = http.StatusInternalServerError
	}

	var res *errors.Error
	if re, is := tgerr.As(err); is {
		if re.Message == "" {
			re.Message = re.Type
		}
		switch {
		// case 400 <= re.Code && re.Code < 500:
		// 	code = http.StatusBadRequest
		case 500 <= re.Code:
			code = http.StatusBadGateway
		default:
			code = re.Code
		}
		res = &errors.Error{
			Id:     re.Type,
			Code:   int32(code),
			Detail: re.Message,
		}

	} else {

		res = errors.FromError(err)
		if res.Code != 0 {
			code = int(res.Code)
		} else {
			res.Code = int32(code)
		}
	}

	writeJSON(w, res, code)
}

func writeError(w http.ResponseWriter, err error, code int) {

	if code == 0 {
		code = http.StatusInternalServerError
	}

	var res *errors.Error
	if re, is := tgerr.As(err); is {
		if re.Message == "" {
			re.Message = re.Type
		}
		switch {
		// case 400 <= re.Code && re.Code < 500:
		// 	code = http.StatusBadRequest
		case 500 <= re.Code:
			code = http.StatusBadGateway
		default:
			code = re.Code
		}
		res = &errors.Error{
			Id:     "telegram.mtproto.rpc.error",
			Code:   int32(code),
			Detail: fmt.Sprintf("telegram: (%d) %s", re.Code, re.Message),
		}

	} else {

		res = errors.FromError(err)
		if res.Code != 0 {
			code = int(res.Code)
		} else {
			res.Code = int32(code)
		}
	}

	writeJSON(w, res, code)
}

// func writeRedirect(w http.ResponseWriter, r *http.Request, res interface{}, uri string, code int) {

// 	if res != nil {

// 		h := w.Header()
// 		h.Set("Pragma", "no-cache")
// 		h.Set("Cache-Control", "no-cache")
// 		// h.Set("Connection", "close")
// 		h.Set("Content-Type", "application/json; charset=utf-8")

// 		http.Redirect(w, r, uri, code)
// 		writeJSON(w, res, code)
// 		return
// 	}

// 	http.Redirect(w, r, uri, code)
// }
