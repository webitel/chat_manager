package facebook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/micro/go-micro/v2/errors"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook.v12/webhooks"
	"golang.org/x/oauth2"
)

// Client of the Facebook App
type Client struct {
	*bot.Gateway // internal
	*http.Client // external
	 oauth2.Config
	 Version string // "v12.0"
	 webhook webhooks.WebHook
	 creds oauth2.TokenSource

	 pages *messengerPages // App Messenger Product Config
	 
	 chatMx *sync.RWMutex // guards c.chats
	 chats map[string]Chat // map[userPSID]{.user,.page}

	 proofMx *sync.Mutex // guards c.proofs
	 proofs map[string]string // map[access_token]appsecret_proof
}

func (c *Client) requestForm(params url.Values, accessToken string) url.Values {

	if params == nil {
		params = url.Values{}
	}

	if accessToken == "" {
		return params
	}

	c.proofMx.Lock()   // +RW
	clientProof, ok := c.proofs[accessToken]; if !ok {
		clientProof = graph.SecretProof(
			accessToken, c.Config.ClientSecret,
		)
		c.proofs[accessToken] = clientProof
	}
	c.proofMx.Unlock() // -RW

	params.Set(graph.ParamAccessToken, accessToken)
	params.Set(graph.ParamSecretProof, clientProof)

	return params
}

var (

	messengerScope = []string{
		// "public_profile",
		// // https://developers.facebook.com/docs/permissions/reference/pages_show_list
		// "pages_show_list",    // GET /{user}/accounts
		// https://developers.facebook.com/docs/permissions/reference/pages_messaging
		"pages_messaging",       // POST /{page}/messages (SendAPI)
		// // https://developers.facebook.com/docs/permissions/reference/pages_read_engagement
		// "pages_read_engagement", // GET /{user}/accounts
		// https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata
		"pages_manage_metadata", // GET|POST|DELETE /{page}/subscribed_apps
	}
)

func (c *Client) PromptPages(rsp http.ResponseWriter, req *http.Request) {

	app := c.Config // shallowcopy
	app.Scopes = messengerScope

	// (302) Found
	http.Redirect(
		rsp, req,
		app.AuthCodeURL(
			"xyz", // ?state= // TODO
			oauth2.SetAuthURLParam(
				"display", "popup",
			),
		),
		http.StatusFound,
	)
}

func IsOAuthCallback(req url.Values) bool {

	return req.Get("state") != ""

	// query := req.URL.Query()
	// return query.Get("state") != ""
}

func (c *Client) completeOAuth(req *http.Request, scope ...string) (*oauth2.Token, error) {

	query := req.URL.Query()

	if state := query.Get("state"); state != "xyz" {
		// http.Error(rsp, "oauth: ?state= is missing or invalid", http.StatusBadRequest)
		return nil, fmt.Errorf("state: invalid or missing")
	}

	if err := query.Get("error"); err != "" {
		// https://openid.net/specs/openid-connect-core-1_0.html#AuthError

		switch err {
		case "consent_required":
		case "login_required":
		case "access_denied":
		// case "":
		default:
		}

		c.Log.Error().
			Str("error", err).
			Str("details", query.Get("error_description")).
			Msg("Facebook: Login FAILED")
		
		if re := query.Get("error_description"); re != "" {
			err += ": " + re
		}

		// http.Error(rsp, err, http.StatusBadRequest)
		return nil, fmt.Errorf(err)
	}

	code := query.Get("code")
	if code == "" {
		err := "oauth2: authorization ?code= is misiing"
		// http.Error(rsp, err, http.StatusBadGateway)
		c.Log.Error().
			Str("error", err).
			Msg("Facebook: Login FAILED")
		return nil, fmt.Errorf(err)
	}

	app := c.Config // shallowcopy
	// app.RedirectURL = "https://dev.webitel.com/chat/ws8/messenger"
	app.Scopes = scope // []string{
	// 	// "pages_show_list",
	// 	// https://developers.facebook.com/docs/permissions/reference/pages_messaging
	// 	"pages_messaging", // FOR /subscribed_apps
	// 	// https://developers.facebook.com/docs/permissions/reference/pages_read_engagement
	// 	"pages_read_engagement",
	// }

	token, err := app.Exchange(
		context.WithValue(context.Background(),
			oauth2.HTTPClient, c.Client,
		),
		code,
	)

	if err != nil {
		// switch re := err.(type) {
		// case *oauth2.RetrieveError:
		//	err = 
		// }
		return nil, err
	}

	return token, nil
}

var completeOAuthHTML, _ = template.New("complete.html").Parse(
`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Title</title>
  <script>
    const message = {
      {{- if .}}
      status: 'error',
      detail: {{.Detail}},
      {{- else}}
      status: 'success',
      {{- end}}
    };
    window.opener.postMessage(message, '*');
    window.close();
  </script>
</head>
<body>

</body>
</html>`,
)

func writeCompleteOAuthHTML(w http.ResponseWriter, err error) error {
	
	h := w.Header()
	h.Set("Pragma", "no-cache")
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	var re *errors.Error
	if err != nil {
		re = errors.FromError(err)
	}
	
	return completeOAuthHTML.Execute(w, re)
}

func (c *Client) SetupPages(rsp http.ResponseWriter, req *http.Request) {

	// USER_ACCESS_TOKEN
	token, err := c.completeOAuth(req, messengerScope...)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadRequest)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	accounts, err := c.getMessengerPages(token)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadGateway)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	c.addMessengerPages(accounts)

	// Save Bot's NEW internal state
	var (

		dataset string
		agent = c.Gateway
	)

	if data := backupAccounts(c); len(data) != 0 {
		encoding := base64.RawURLEncoding
		dataset = encoding.EncodeToString(data)
	}
	// OVERRIDE OR DELETE
	err = agent.SetMetadata(
		req.Context(), map[string]string{
			"accounts": dataset,
		},
	)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusInternalServerError)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	// // GET /me?field=,accounts&access_token=USER_ACCESS_TOKEN
	// // 

	// // // GET /debug_token
	// // debugToken, err := c.introspect(userToken.AccessToken)
	// // // GET /{user_id}?fields=
	// // user, err := c.GetProfile(debugToken["user_id"].(string))
	// // GET /{user_id}

	// // 302 Found https://dev.webitel.com/ws8/messenger/pages
	// http.Redirect(rsp, req,
	// 	c.CallbackURL()+ "?pages=",
	// 	// "https://dev.webitel.com" + path.Join("/chat/ws8/messenger")+ "?pages",
	// 	http.StatusFound,
	// )

	// 200 OK
	// NOTE: Static HTML to help UI close popup window !
	_ = writeCompleteOAuthHTML(rsp, nil)
	// header := rsp.Header()
	// header.Set("Pragma", "no-cache")
	// header.Set("Cache-Control", "no-cache")
	// header.Set("Content-Type", "text/html; charset=utf-8")
	// rsp.WriteHeader(http.StatusOK)

	// var re *errors.Error
	// if err != nil {
	// 	re = errors.FromError(err)
	// }
	
	// _ = completeOAuthHTML.Execute(rsp, re)

// 	_, _ = rsp.Write([]byte(
// `<!DOCTYPE html>
// <html lang="en">
// <head>
//   <meta charset="UTF-8">
//   <title>Title</title>
//   <script>
//     window.opener.postMessage('success');
//     // window.close();
//   </script>
// </head>
// <body>

// </body>
// </html>`))

}

// UserAccounts represents set of the
// Facebook User's Messenger Pages GRANTED
type UserAccounts struct {
	 User *graph.User
	 Pages []*Page
}

// POST /?batch=[
// 	{"method":"GET","relative_uri":"{PAGE-ID}?fields=subscribed_apps{subscribed_fields}"},
// 	. . .
// ]
func (c *Client) getSubscribedFields(token *oauth2.Token, pages []*Page) error {

	n := len(pages)
	if n == 0 {
		return nil
	}

	var (

		form = url.Values{
			"fields": {"subscribed_fields.as(fields)"},
		}
		batch = make([]graph.BatchRequest, n)
	)

	for i, page := range pages {
		// [RE]Authorize Each Request
		form = c.requestForm(form, page.AccessToken)

		req := &batch[i]
		// GET /{PAGE-ID}/subscribed_apps
		// Applications that have real time update subscriptions for this Page.
		// Note that we will only return information about the current app !!!
		// https://developers.facebook.com/docs/graph-api/reference/page/#edges
		req.Method = http.MethodGet
		req.RelativeURL = path.Join(
			page.ID, "subscribed_apps",
		) + "?" + form.Encode()
	}

	jsonb, err := json.Marshal(batch)
	
	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch": {string(jsonb)},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := token.AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com" + path.Join("/", c.Version),
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	
	if err != nil {
		return err
	}

	defer rsp.Body.Close()

	res := make([]*graph.BatchResult, 0, n)
	err = json.NewDecoder(rsp.Body).Decode(&res)
	if err != nil {
		return err
	}
	// BATCH Request(s) order !
	var (
		
		apps = make([]*struct{
			// ID string `json:"id"` // MUST: Application ID == c.Config.ClientId
			SubscribedFields []string `json:"fields,omitempty"`
		}, 0, 1)
		re = graph.Result {
			Data: &apps,
		}
	)
	for i, page := range pages {
		
		apps = apps[:0]
		re.Error = nil
		
		err = json.NewDecoder(
			strings.NewReader(res[i].Body),
		).Decode(&re)

		if err == nil && re.Error != nil {
			err = re.Error
		}

		if err != nil {

			c.Log.Err(err).
				Str("id", page.ID).
				Str("page", page.Name).
				Int("code", res[i].Code).
				Msg("SUBSCRIBED: FIELDS")

			continue
		}

		page.SubscribedFields = nil
		if len(apps) == 1 {
			page.SubscribedFields =
				apps[0].SubscribedFields
		}

		c.Log.Info().
			Str("page", page.Name).
			Str("page.id", page.ID).
			Strs("fields", page.SubscribedFields).
			Msg("SUBSCRIBED: FIELDS")
	}

	return nil
}

// Retrive Facebook User profile and it's accounts (Pages) access granted
// Refresh Pages webhook subscription state
func (c *Client) getMessengerPages(token *oauth2.Token) (*UserAccounts, error) {

	// GET /me?fields=name,accounts{name,access_token}

	form := c.requestForm(url.Values{
		"fields": {"name,accounts{name,access_token}"},
		}, token.AccessToken,
	)

	req, err := http.NewRequest(http.MethodGet,
		"https://graph.facebook.com" + path.Join("/", c.Version, "me") +
			"?" + form.Encode(), nil,
	)

	if err != nil {
		return nil, err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var (
		
		pages []*Page
		resMe = struct {
			graph.User
			Accounts graph.Result `json:"accounts"`
		} {
			Accounts: graph.Result{
				Data: &pages,
			},
		}
	)

	err = json.NewDecoder(rsp.Body).Decode(&resMe)

	if err != nil {
		// Failed to decode JSON result
		return nil, err
	}

	if resMe.Accounts.Error != nil {
		// GraphAPI request error
		return nil, resMe.Accounts.Error
	}

	res := &UserAccounts {
		User: &resMe.User,
		Pages: pages,
		// Pages: make(map[string]*messengerPage, len(pages)),
	}

	// GET Each Page's subscription state !
	err = c.getSubscribedFields(token, pages)

	if err != nil {
		// Failed to GET Page(s) subscribed_fields (subscription) state !
		return nil, err
	}

	return res, nil
}

func (c *Client) addMessengerPages(accounts *UserAccounts) {
	_ = c.pages.setPages(accounts)
}

// Install Facebook App for all page(s) specified
// Other words, subscribe Facebook App to the Page's webhook updates
// https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/#Creating
func (c *Client) subscribePages(pages []*Page) error {

	n := len(pages)
	if n == 0 {
		return nil
	}

	var (

		subscribedPageFields = []string{
			// "standby",
			"messages",
			"messaging_postbacks",
			// "messaging_handovers",
			// "user_action",
		}

		// https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/#parameters-2
		form = url.Values{
			// Page Webhooks fields that you want to subscribe
			"subscribed_fields": {
				strings.Join(subscribedPageFields, ","),
			},
		}
		// 
		batch = make([]graph.BatchRequest, n)
	)

	for i, page := range pages {
		// [RE]Authorize Each Request
		form = c.requestForm(form, page.AccessToken)

		req := &batch[i]
		req.Method = http.MethodPost
		req.RelativeURL = path.Join(
			page.ID, "subscribed_apps",
		)
		req.Body = form.Encode()
	}

	bytes, err := json.Marshal(batch)
	
	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch": {string(bytes)},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := pages[0].AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com" + path.Join("/", c.Version),
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	
	if err != nil {
		return err
	}

	defer rsp.Body.Close()

	ret := make([]*graph.BatchResult, 0, n)
	err = json.NewDecoder(rsp.Body).Decode(&ret)
	if err != nil {
		return err
	}
	var (
		// ret graph.Success
		// res = graph.Result{
		// 	Data: ret,
		// }
		res = struct{
			graph.Success // Embedded (Anonymous)
			Error *graph.Error `json:"error,omitempty"`
		} {
			// Alloc
		}
	)
	// BATCH Request(s) order !
	for i, page := range pages {
		// NULLify
		res.Ok = false
		res.Error = nil
		// Decode JSON Result
		err = json.NewDecoder(
			strings.NewReader(ret[i].Body),
		).Decode(&res)

		if err == nil && res.Error != nil {
			err = res.Error
		}
		if err == nil && !res.Ok {
			err = fmt.Errorf("subscribe: page=%s not confirmed", page.ID)
		}
		
		if err != nil {
			c.Log.Err(err).
			Str("page-id", page.ID).
			Str("page", page.Name).
			Int("code", ret[i].Code).
			Msg("SUBSCRIBE: PAGE")
			continue
		}
		// SUCCESS !
		page.SubscribedFields = subscribedPageFields
	}

	return nil
}

// Uninstall Facebook App for all page(s) specified
// Other words, unsubscribe Facebook App from the Page's webhook updates
// https://developers.facebook.com/docs/graph-api/reference/page/subscribed_apps/#Deleting
// 
// [TODO]: We need to wait for page's ALL active chat(s) to close, before doing this ...
func (c *Client) unsubscribePages(pages []*Page) error {

	n := len(pages)
	if n == 0 {
		return nil
	}

	var (

		form url.Values
		batch = make([]graph.BatchRequest, n)
	)

	for i, page := range pages {
		// [RE]Authorize Each Request
		form = c.requestForm(form, page.AccessToken)

		req := &batch[i]
		req.Method = http.MethodDelete
		req.RelativeURL = path.Join(
			page.ID, "subscribed_apps",
		) + "?" + form.Encode()

	}

	bytes, err := json.Marshal(batch)
	
	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch": {string(bytes)},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := pages[0].AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n

	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com" + path.Join("/", c.Version),
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return err
	}

	rsp, err := c.Client.Do(req)
	
	if err != nil {
		return err
	}

	defer rsp.Body.Close()

	ret := make([]*graph.BatchResult, 0, n)
	err = json.NewDecoder(rsp.Body).Decode(&ret)
	if err != nil {
		return err
	}
	// BATCH Request(s) order !
	var (
		// re struct {
		// 	Success bool `json:"success,omitempty"`
		// 	MessagingSuccess bool `json:"messaging_success,omitempty"`
		// }
		// res = graph.Result{
		// 	Data: &re,
		// }
		res = struct{
			// Embedded (Anonymous)
			Success bool `json:"success,omitempty"`
			MessagingSuccess bool `json:"messaging_success,omitempty"`
			Error *graph.Error `json:"error,omitempty"`
		} {
			// Alloc
		}
	)
	for i, page := range pages {
		// NULLify
		res.Error = nil
		res.Success = false
		res.MessagingSuccess = false

		err = json.NewDecoder(
			strings.NewReader(ret[i].Body),
		).Decode(&res)

		if err == nil && res.Error != nil {
			switch res.Error.Code {
			case 100: // Invalid parameter
				// Example DEBUG:
				// {
				// 	"error": {
				// 		"message": "(#100) App is not installed: 2066377413624819",
				// 		"type":"OAuthException",
				// 		"code":100,
				// 		"fbtrace_id":"AnSpVSRcQsQ9h4dOrWaeKOy"
				// 	}
				// }
				idempotent := strings.HasPrefix(res.Error.Message,
					"(#100) App is not installed: "+ c.Config.ClientID,
				)
				if idempotent {
					// Already Unsubscribed ! OK
					page.SubscribedFields = nil

					res.Error = nil
					res.Success = true
					res.MessagingSuccess = true
					// continue // next page result !
				}

			case 200: // Permissions error
			case 210: // User not visible
			}
			err = res.Error
		}
		if err == nil && !res.Success {
			err = fmt.Errorf("unsubscribe: success not confirmed")
		}
		// if err == nil && !re.MessagingSuccess {
		// 	err = fmt.Errorf("unsubscribe: messaging_success not confirmed")
		// }
		if err != nil {
			c.Log.Err(err).
			Str("page", page.Name).
			Str("page-id", page.ID).
			Int("code", ret[i].Code).
			Msg("UNSUBSCRIBE: PAGE")

			continue
		}

		page.SubscribedFields = nil
	}

	return nil
}

// DeauthorizeRequest parameters
type DeauthorizeRequest struct {
	// The Messenger's Page ID
	PageID string `json:"profile_id,omitempty"`
	// User who removed your App. FIXME: Blank stands for your self request ?
	UserID string `json:"user_id,omitempty"`
	// MUST be constant HMAC-SHA256
	Algorithm string `json:"algorithm,omitempty"`
	// IssuedAt request timestamp (unix seconds)
	IssuedAt int64 `json:"issued_at,omitempty"`
}

// Deauthorize Callback URL triggered after
// DELETE /{PAGE-ID}/subscribed_apps
//
// https://stackoverflow.com/questions/9670293/how-to-use-deauthorize-callback-url-with-facebook-js-sdk
func (c *Client) DeauthorizeRequest(signed string) (*DeauthorizeRequest, error) {

	// POST /{deauthorize_callback_url}
	// Content-Type: application/x-www-form-urlencoded
	// 
	// signed_request=9MLZPR8Wl48BjO8DYOuPsk8lC2J1oXoXHxsxip8isE0.eyJwcm9maWxlX2lkIjoiMTEyMzcxOTcxMjc4MTk4IiwidXNlcl9pZCI6IiIsImFsZ29yaXRobSI6IkhNQUMtU0hBMjU2IiwiaXNzdWVkX2F0IjoxNjQyNDIyNzA1fQ

	// NOTE: This kind triggered when we DELETE /{page}/subscribed_apps
	// {
	// 	"profile_id": "112371971278198", // PAGE_ID; 
	// 	"user_id": "", // FIXME: Stands for self APP ? for ALL Users ?
	// 	"algorithm": "HMAC-SHA256",
	// 	"issued_at": 1642422705
	// }

	// NOTE: This kind triggered when User remove our App from https://www.facebook.com/settings/?tab=business_tools
	// signed_request=O7aXgQlIR62fsNBZzcdMtVIEYoboR1uXavjHNmkd3AU.eyJ1c2VyX2lkIjoiNDcwNTI3ODkxMjg0MzU5MSIsImFsZ29yaXRobSI6IkhNQUMtU0hBMjU2IiwiaXNzdWVkX2F0IjoxNjQyNTA5NTk3fQ
	// {
	// 	"profile_id": "", // FIXME: No "profile_id" means for user's ALL pages ?
	// 	"user_id": "4705278912843591", // USER_ID
	// 	"algorithm": "HMAC-SHA256",
	// 	"issued_at": 1642509597
	// }

	encoding := base64.RawURLEncoding

	parts := strings.Split(signed, ".")
	payload := parts[1]
	signature := parts[0]
	// Signature Valid ?
	rsum, err := encoding.DecodeString(signature)
	if err != nil {
		// Invalid signature
		return nil, err
	}

	algo := sha256.New
	hash := hmac.New(algo, []byte(c.Config.ClientSecret))
	_, _ = hash.Write([]byte(payload))
	hsum := hash.Sum(nil)
	// Signature Match ?
	if !hmac.Equal(hsum, rsum) {
		// Invalid signature
		return nil, err
	}

	// Decode Request !
	var req DeauthorizeRequest
	err = json.NewDecoder(base64.NewDecoder(
		encoding, strings.NewReader(payload),
	)).Decode(&req)

	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (c *Client) Deauthorize(signedRequest string) error {

	req, err := c.DeauthorizeRequest(signedRequest)

	if err != nil {
		c.Log.Error().
		Str("error", "signature: invalid").
		Msg("DEAUTHORIZE")
		return err
	}

	c.Log.Warn().
	Str("page-id", req.PageID).
	Str("user-id", req.UserID).
	Time("issued", time.Unix(req.IssuedAt, 0)).
	Msg("DEAUTHORIZE")

	if req.UserID == "" {
		// FIXME: Triggered by myself !
		// DELETE /{PAGE-ID}/subscribed_apps?access_token=<PAGE_ACCESS_TOKEN>
		return nil
	}

	// NOTE: ALL(unspec) -or- SINGLE(spec) !
	var pageId []string // ALL
	if req.PageID != "" {
		pageId = []string{
			req.PageID, // EXACT
		}
	}
	pages, err := c.pages.getPages(pageId...) // LOCK: +R

	if err != nil {
		c.Log.Error().
		Str("error", "lookup: profile_id=%s; "+ err.Error()).
		Msg("DEAUTHORIZE")
		return err
	}

	for _, page := range pages {
		// NOTE: ALL(unspec) -or- SINGLE(spec) !
		_ = page.Deauthorize(req.UserID)
		if !page.IsAuthorized() {
			_ = c.pages.delPage(page.ID) // LOCK: +RW
		}
	}

	// Save Bot's NEW internal state
	agent := c.Gateway
	// cbot := agent.Bot

	var dataset string
	if data := backupAccounts(c); len(data) != 0 {
		encoding := base64.RawURLEncoding
		dataset = encoding.EncodeToString(data)
	}
	// OVERRIDE !
	err = agent.SetMetadata(
		context.TODO(), map[string]string{
			"accounts": dataset,
		},
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) RemovePages(pageIds ...string) ([]*Page, error) {

	// TODO: UnsubscribePages()
	// TODO: Remove tracks

	// Find ALL requested page(s)...
	pages, err := c.pages.getPages(pageIds...)

	if err != nil {
		return nil, err
	}

	// DELETE /{PAGE-ID}/subscribed_apps
	err = c.unsubscribePages(pages)

	if err != nil {
		return nil, err
	}

	// REMOVE ?pages=
	// for _, id := range pageIds {
	// 	delete(c.pages, id)
	// }
	for _, page := range pages {
		if len(page.SubscribedFields) == 0 {
			_ = c.pages.delPage(page.ID)
		}
	}
	

	return pages, nil
}

func (c *Client) introspect(accessToken string) (map[string]interface{}, error) {
	// APP_ACCESS_TOKEN
	clientToken, err := c.creds.Token()
	if err != nil {
		return nil, err
	}

	form := url.Values{
		"input_token": {accessToken},
	}
	form = c.requestForm(form, clientToken.AccessToken)

	req, err := http.NewRequest(
		http.MethodGet, "https://graph.facebook.com" +
			path.Join("/", c.Version, "/debug_token"),
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return nil, err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var res graph.Result
	err = json.NewDecoder(rsp.Body).Decode(&res)
	
	if err != nil {
		return nil, err
	}

	if res.Error != nil {
		return nil, res.Error
	}

	return res.Data.(map[string]interface{}), nil
}

// ---------------- Messenger Platform ----------------- //

func (c *Client) getChat(page *Page, psid string) (*Chat, error) {

	// TODO: Lookup internal cache first ...
	c.chatMx.RLock()   // +R
	chat := c.chats[psid]
	c.chatMx.RUnlock() // -R

	if chat.User.ID == psid {
		return &chat, nil
	}

	// GET /{PSID}?fields=name&access_token={PAGE_ACCESS_TOKEN}
	if page == nil {
		// Authorization NOT provided
		chat.User.ID = psid
		chat.User.Name = "messenger" // "noname"
		return nil, errors.NotFound(
			"bot.messenger.chat.user.not_found",
			"messenger: chat channel not found",
		)
	}
	chat.Page = page

	query := c.requestForm(url.Values{
			"fields": {"name"}, // ,first_name,middle_name,last_name,picture{width(50),height(50)}"},
		}, page.AccessToken,
	)

	req, err := http.NewRequest(http.MethodGet,
		"https://graph.facebook.com" + path.Join(
			"/", c.Version, psid,
		) + "?" + query.Encode(),
		nil,
	)

	if err != nil {
		return nil, err
	}

	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	// var res graph.User
	// err = json.NewDecoder(rsp.Body).Decode(&res)
	err = json.NewDecoder(rsp.Body).Decode(&chat.User)
	if err != nil {
		return nil, err
	}

	// if res.ID != psid {
	if chat.User.ID != psid {
		// Invalid result !
	}

	// TODO: Cache result for PSID
	c.chatMx.Lock()   // +W
	c.chats[psid] = chat
	c.chatMx.Unlock() // -W

	return &chat, nil
}
