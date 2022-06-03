package facebook

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	"golang.org/x/oauth2"
)

var (
	messengerFacebookScope = []string{
		// "public_profile",
		// // https://developers.facebook.com/docs/permissions/reference/pages_show_list
		// "pages_show_list", // GET /{user}/accounts
		// https://developers.facebook.com/docs/permissions/reference/pages_messaging
		"pages_messaging", // POST /{page}/messages (SendAPI)
		// // https://developers.facebook.com/docs/permissions/reference/pages_read_engagement
		// "pages_read_engagement", // GET /{user}/accounts
		// https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata
		"pages_manage_metadata", // GET|POST|DELETE /{page}/subscribed_apps
	}

	// https://developers.facebook.com/docs/messenger-platform/instagram/get-started#2--implement-facebook-login
	messengerInstagramScope = []string{
		// "public_profile",
		// https://developers.facebook.com/docs/permissions/reference/pages_messaging
		"instagram_basic", // POST /{page}/messages (SendAPI)
		"instagram_manage_messages",
		// https://developers.facebook.com/docs/permissions/reference/pages_manage_metadata
		"pages_manage_metadata", // GET|POST|DELETE /{page}/subscribed_apps
	}
)

func (c *Client) SetupMessengerPages(rsp http.ResponseWriter, req *http.Request) {

	// USER_ACCESS_TOKEN
	token, err := c.completeOAuth(req, messengerFacebookScope...)

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
		agent   = c.Gateway
	)

	if data := c.pages.backup(); len(data) != 0 {
		encoding := base64.RawURLEncoding
		dataset = encoding.EncodeToString(data)
	}
	// OVERRIDE OR DELETE
	err = agent.SetMetadata(
		req.Context(), map[string]string{
			// "accounts": dataset,
			"fb": dataset,
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
	User  *graph.User
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

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(batch)

	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch":           {buf.String()},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := token.AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com"+path.Join("/", c.Version),
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
		apps = make([]*struct {
			// ID string `json:"id"` // MUST: Application ID == c.Config.ClientId
			SubscribedFields []string `json:"fields,omitempty"`
		}, 0, 1)
		re = graph.Result{
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
		"https://graph.facebook.com"+path.Join("/", c.Version, "me")+
			"?"+form.Encode(), nil,
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
		}{
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

	res := &UserAccounts{
		User:  &resMe.User,
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
		// form = c.requestForm(form, page.AccessToken)

		req := &batch[i]
		req.Method = http.MethodPost
		req.RelativeURL = path.Join(
			page.ID, "subscribed_apps",
		)
		req.Body = form.Encode()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(batch)

	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch":           {buf.String()},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := pages[0].AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com"+path.Join("/", c.Version),
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
		res = struct {
			graph.Success              // Embedded (Anonymous)
			Error         *graph.Error `json:"error,omitempty"`
		}{
			// Alloc
		}

		save bool
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
		save = (save || len(page.SubscribedFields) == 0)
		page.SubscribedFields = subscribedPageFields
	}

	if save {
		// Save Bot's NEW internal state
		var (
			data  string
			agent = c.Gateway
			enc   = base64.RawURLEncoding
		)

		if bak := c.pages.backup(); len(bak) != 0 {
			data = enc.EncodeToString(bak)
		}
		// BACKUP NEW Internal State
		_ = agent.SetMetadata(
			req.Context(), map[string]string{
				"fb": data,
			},
		)
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
		form  url.Values
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

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(batch)

	if err != nil {
		return err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch":           {buf.String()},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := pages[0].AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n

	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com"+path.Join("/", c.Version),
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
		res = struct {
			// Embedded (Anonymous)
			Success          bool         `json:"success,omitempty"`
			MessagingSuccess bool         `json:"messaging_success,omitempty"`
			Error            *graph.Error `json:"error,omitempty"`
		}{
			// Alloc
		}

		save bool // Need to be saved ?
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
					"(#100) App is not installed: "+c.Config.ClientID,
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

		save = (save || page.SubscribedFields != nil)
		page.SubscribedFields = nil
	}

	if save {
		// Save Bot's NEW internal state
		var (
			data  string
			agent = c.Gateway
			enc   = base64.RawURLEncoding
		)

		if bak := c.pages.backup(); len(bak) != 0 {
			data = enc.EncodeToString(bak)
		}
		// BACKUP NEW Internal State
		_ = agent.SetMetadata(
			req.Context(), map[string]string{
				"fb": data,
			},
		)
	}

	return nil
}
