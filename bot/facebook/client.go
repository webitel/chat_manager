package facebook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/webitel/chat_manager/api/proto/chat"

	cache "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook/webhooks"
	"github.com/webitel/chat_manager/bot/facebook/whatsapp"
	"golang.org/x/oauth2"
)

// Client of the Facebook App
type Client struct {
	*bot.Gateway // internal
	*http.Client // external
	media        *http.Client
	oauth2.Config
	Version string // "v12.0"
	webhook webhooks.WebHook
	creds   oauth2.TokenSource
	// Extra Fields subscription hook(s)
	hookIGStoryMention func(IGSID string, mention *IGStoryMention)
	hookIGMediaMention func(IGSID string, mention *IGMention)
	hookIGMediaComment func(IGSID string, comment *IGComment)

	pages     *messengerPages // App Messenger Product Config
	instagram *messengerPages // App Messenger Product Config
	whatsApp  *whatsapp.Manager

	peerCache cache.LRU[string, *chat.Channel]

	chatMx *sync.RWMutex   // guards c.chats
	chats  map[string]Chat // map[userPSID]{.user,.page}

	proofMx *sync.Mutex       // guards c.proofs
	proofs  map[string]string // map[access_token]appsecret_proof
}

func (c *Client) requestForm(params url.Values, accessToken string) url.Values {

	if params == nil {
		params = url.Values{}
	}

	if accessToken == "" {
		return params
	}

	c.proofMx.Lock() // +RW
	clientProof, ok := c.proofs[accessToken]
	if !ok {
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

func (c *Client) PromptSetup(
	w http.ResponseWriter, r *http.Request,
	scope []string, state string,
	opts ...oauth2.AuthCodeOption,
) {

	app := c.Config // shallowcopy
	app.Scopes = scope

	// (302) Found
	http.Redirect(w, r,
		app.AuthCodeURL(
			state, // ?state= // TODO
			opts...,
		),
		http.StatusFound,
	)
}

func IsOAuthCallback(req url.Values) (state string, ok bool) {

	state = req.Get("state")
	return state, ("" != state)

	// query := req.URL.Query()
	// return query.Get("state") != ""
}

func (c *Client) completeOAuth(req *http.Request, scope ...string) (*oauth2.Token, error) {

	query := req.URL.Query()

	// if state := query.Get("state"); state != "fb" { // "xyz"
	// 	// http.Error(rsp, "oauth: ?state= is missing or invalid", http.StatusBadRequest)
	// 	return nil, fmt.Errorf("state: invalid or missing")
	// }

	if err := query.Get("error"); err != "" {
		// https://openid.net/specs/openid-connect-core-1_0.html#AuthError

		switch err {
		case "consent_required":
		case "login_required":
		case "access_denied":
		// case "":
		default:
		}

		c.Log.Error("Facebook: Login FAILED",
			slog.Any("error", err),
			slog.String("details", query.Get("error_description")),
		)

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
		c.Log.Error("Facebook: Login FAILED",
			slog.Any("error", err),
		)
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
	<div>
	  {{if . -}}
	  <strong>ERR:</strong> {{html .Detail}}
	  {{- else -}}
	  <strong>OK</strong>
	  {{- end}}
	</div>
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
		c.Log.Error("DEAUTHORIZE: REQUEST",
			slog.String("error", "signature: invalid"),
		)
		return err
	}

	c.Log.Warn("MESSENGER: DEAUTHORIZE",
		slog.String("page-id", req.PageID),
		slog.String("user-id", req.UserID),
		slog.Time("issued", time.Unix(req.IssuedAt, 0)),
	)

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
	// Facebook Pages
	pages, err := c.pages.getPages(pageId...) // LOCK: +R

	if err != nil {
		c.Log.Error("MESSENGER: DEAUTHORIZE",
			slog.Any("error", err),
		)
		return err
	}

	for _, page := range pages {
		// NOTE: ALL(unspec) -or- SINGLE(spec) !
		_ = page.Deauthorize(req.UserID)
		if !page.IsAuthorized() {
			_ = c.pages.delPage(page.ID) // LOCK: +RW
		}
	}
	// Instagram
	pages, err = c.instagram.getPages(pageId...) // LOCK: +R

	if err != nil {
		c.Log.Error("INSTAGRAM: DEAUTHORIZE",
			slog.Any("error", err),
		)
		return err
	}

	for _, page := range pages {
		// NOTE: ALL(unspec) -or- SINGLE(spec) !
		_ = page.Deauthorize(req.UserID)
		if !page.IsAuthorized() {
			_ = c.instagram.delPage(page.ID) // LOCK: +RW
		}
	}

	// Save Bot's NEW internal state
	agent := c.Gateway
	// cbot := agent.Bot

	var (
		fb, ig string
		enc    = base64.RawURLEncoding
	)
	// if data := backupAccounts(c); len(data) != 0 {
	if data := c.pages.backup(); len(data) != 0 {
		fb = enc.EncodeToString(data)
	}
	if data := c.instagram.backup(); len(data) != 0 {
		ig = enc.EncodeToString(data)
	}
	// OVERRIDE !
	err = agent.SetMetadata(
		context.TODO(), map[string]string{
			"fb": fb,
			"ig": ig,
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
		http.MethodGet, "https://graph.facebook.com"+
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

// Chat represents Facebook User TO Messenger Page conversation
type Chat struct {
	// Facebook User. Sender, initiator
	User graph.User
	// Facebook Page. Bot, recipient
	Page *Page
}

func (c *Client) getChat(page *Page, psid string) (conn *Chat, err error) {

	// TODO: Lookup internal cache first ...
	c.chatMx.RLock() // +R
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

	// NOTE: Now we can build a minimal recipient
	// contact info with given input arguments only..
	// So ignore any error(s) getting UserProfile info
	// Just LOG them, if any, but return no error ...

	defer func() {
		if err != nil {
			// chatID := psid
			platform := "facebook"
			facebook := page // MUST: recipient
			pageName := facebook.Name
			instagram := facebook.Instagram
			if instagram != nil {
				platform = "instagram"
				pageName = instagram.Name
				if pageName == "" {
					pageName = instagram.Username
				}
			}
			c.Log.Error(platform+".getUserProfile",
				slog.Any("error", err),
				slog.String("page.asid", page.ID), // recipient
				slog.String(platform, pageName),   // sender
				// Str("from", "noname").  // Unavailable(!)
				slog.String("user.psid", psid),
			)
			// err = res.Error
			err = nil
		}
		// NO/Invalid result (?)
		if chat.User.ID != psid {
			// Minimal Userinfo (!)
			chat.User = graph.UserProfile{
				Name: "noname", // Unavailable(!)
				ID:   psid,
			}
		}
		// TODO: Cache result for PSID
		c.chatMx.Lock() // +W
		c.chats[psid] = chat
		c.chatMx.Unlock() // -W

		conn = &chat
		// return // conn, nil
	}()

	query := c.requestForm(url.Values{
		"fields": {"name"}, // ,first_name,middle_name,last_name,picture{width(50),height(50)}"},
	}, page.AccessToken,
	)

	req, re := http.NewRequest(http.MethodGet,
		"https://graph.facebook.com"+path.Join(
			"/", c.Version, psid,
		)+"?"+query.Encode(),
		nil,
	)

	if err = re; err != nil {
		return nil, err
	}

	rsp, re := c.Client.Do(req)

	if err = re; err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	var (
		res = struct {
			// Private JSON result
			raw json.RawMessage
			// data *graph.User
			// Public JSON result
			Error *graph.Error `json:"error,omitempty"`
		}{
			// raw:  make(json.RawMessage, 0, res.ContentLength), // NO Content-Length Header provided !  =(
		}
	)

	// // https://developers.facebook.com/docs/messenger-platform/identity/user-profile/#profile_unavailable
	// rpcError := bytes.NewReader([]byte(`{
	// 	"error": {
	// 		"message": "(#100) No profile available for this user.",
	// 		"type": "OAuthException",
	// 		"code": 100,
	// 		"error_subcode": 2018218,
	// 		"fbtrace_id": "APLx_C4fHhrLqjNFqOXv7tw"
	// 	}
	// }`))
	// err = json.NewDecoder(rpcError).Decode(&res.raw)
	err = json.NewDecoder(rsp.Body).Decode(&res.raw)
	if err != nil {
		// ERR: Invalid JSON
		return nil, err
	}
	// CHECK: for RPC `error` first
	err = json.Unmarshal(res.raw, &res) // {"error"}
	if err == nil && res.Error != nil {
		// RPC Error !
		err = res.Error
	}
	if err != nil {
		return nil, err
	}
	// Decode UserProfile payload data
	err = json.Unmarshal(res.raw, &chat.User)
	if err != nil {
		// ERR: Unexpected JSON result
		return nil, err
	}
	// Build result defered (!)
	return // &chat, nil
}
