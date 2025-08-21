package webchat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	DefaultThreshold = 0.5
	RecaptchaURL     = "https://www.google.com/recaptcha/api/siteverify"
)

//var googleErrors = map[string]string{
//	"missing-input-secret":   "the secret parameter is missing",
//	"invalid-input-secret":   "the secret parameter is invalid or malformed",
//	"missing-input-response": "the response parameter is missing",
//	"invalid-input-response": "the response parameter is invalid or malformed",
//	"bad-request":            "the request is invalid or malformed",
//	"timeout-or-duplicate":   "the response is no longer valid: either is too old or has been used previously",
//}

type RecaptchaHandler struct {
	setting *RecaptchaSettings
}

type RecaptchaSettings struct {
	Enabled   bool     `json:"enabled,omitempty"`
	Secret    string   `json:"secret,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
}

type RecaptchaResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type GoogleRecaptchaResponse struct {
	Success        bool     `json:"success,omitempty"`
	ChallengeTs    string   `json:"challenge_ts,omitempty"`
	Hostname       string   `json:"hostname,omitempty"`
	ErrorCodes     []string `json:"error-codes,omitempty"`
	ApkPackageName string   `json:"apk_package_name,omitempty"`
	Score          float64  `json:"score,omitempty"`
	Action         string   `json:"action,omitempty"`
}

func (r *RecaptchaSettings) Normalize() error {
	if r.Secret == "" {
		return errors.New("recaptcha secret should not be empty")
	}
	if r.Threshold == nil {
		def := DefaultThreshold
		r.Threshold = &def
	}
	return nil
}

func NewRecaptchaHandler(settings string) (CaptchaHandler, error) {
	var recatpchaSetting RecaptchaSettings
	err := json.Unmarshal([]byte(settings), &recatpchaSetting)
	if err != nil {
		return nil, err
	}
	err = recatpchaSetting.Normalize()
	if err != nil {
		return nil, err
	}
	return &RecaptchaHandler{setting: &recatpchaSetting}, nil
}

func (h *RecaptchaHandler) Enabled() bool {
	return h.setting.Enabled
}

func (h *RecaptchaHandler) HandleCaptcha(rsp http.ResponseWriter, req *http.Request) {

	var (
		googleOutput GoogleRecaptchaResponse
		success      bool
	)

	response := req.URL.Query().Get("response")
	remoteip := req.URL.Query().Get("remoteip")

	rawUrl, _ := url.Parse(RecaptchaURL)
	params := rawUrl.Query()
	defer req.Body.Close()

	params.Add("secret", h.setting.Secret)
	params.Add("response", response)
	params.Add("remoteip", remoteip)
	rawUrl.RawQuery = params.Encode()
	googleReq, err := http.NewRequest("POST", rawUrl.String(), nil)
	if err != nil {
		returnErrorToResp(rsp, http.StatusInternalServerError, errors.New(fmt.Sprintf("can't create Google request (%s)", err.Error())))
		return
	}

	googleResp, err := http.DefaultClient.Do(googleReq)
	if err != nil {
		returnErrorToResp(rsp, http.StatusInternalServerError, errors.New(fmt.Sprintf("request to Google failed (%s)", err.Error())))
		return
	}
	defer googleResp.Body.Close()
	err = json.NewDecoder(googleResp.Body).Decode(&googleOutput)
	if err != nil {
		returnErrorToResp(rsp, http.StatusInternalServerError, errors.New(fmt.Sprintf("can't unmarshal google response (%s)", err.Error())))
		return
	}

	if !googleOutput.Success { // error from google side
		returnErrorToResp(rsp, http.StatusBadRequest, errors.New(strings.Join(googleOutput.ErrorCodes, ";")))
		return
	}

	if h.setting.Threshold != nil && googleOutput.Score > *h.setting.Threshold {
		success = true
	}

	//rsp.WriteHeader(http.StatusOK)
	err = json.NewEncoder(rsp).Encode(RecaptchaResponse{Success: success})
	if err != nil {
		returnErrorToResp(rsp, http.StatusInternalServerError, errors.New(fmt.Sprintf("can't marshal final results (%s)", err.Error())))
	}
	rsp.WriteHeader(http.StatusOK)
	return
}

//func formatResultError(err error) []byte {
//	bytes, err := json.Marshal(RecaptchaResponse{Error: formatErrorString(err.Error())})
//	if err != nil {
//		// TODO: error while marshalling error
//		return []byte("error")
//	}
//	return bytes
//}

func formatErrorString(err string) string {
	return fmt.Sprintf("captcha: %s", err)
}

func returnErrorToResp(rsp http.ResponseWriter, code int, err error) {
	if err == nil {
		rsp.WriteHeader(http.StatusInternalServerError)
		return
	}
	if code == 0 {
		code = http.StatusInternalServerError
	}
	rsp.WriteHeader(code)
	json.NewEncoder(rsp).Encode(RecaptchaResponse{Error: formatErrorString(err.Error())})
	return
}
