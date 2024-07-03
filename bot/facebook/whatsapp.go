package facebook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/bot"
	graph "github.com/webitel/chat_manager/bot/facebook/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook/webhooks"
	"github.com/webitel/chat_manager/bot/facebook/whatsapp"
	"golang.org/x/oauth2"
)

const (
	// [W]hats[A]pp [B]usiness[A]ccount [ID]
	paramWhatsAppAccountID = "whatsapp.business"
	// [W]hats[A]pp [P]hone[N]umber Display
	paramWhatsAppPhoneNumber = "whatsapp.number"
	// [W]hats[A]pp [P]hone[N]umber Account [ID]
	paramWhatsAppNumberID = "whatsapp.account"
)

type whatsAppAccountResult struct {
	Error                             *graph.Error `json:"error,omitempty"`
	*whatsapp.WhatsAppBusinessAccount              // embedded
}

func (c *Client) renderWhatsAppBusinessResponse(rsp http.ResponseWriter, req *http.Request) func([]whatsAppAccountResult, error) {
	return func(res []whatsAppAccountResult, err error) {

		header := rsp.Header()
		header.Set("Pragma", "no-cache")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "close")
		header.Set("Content-Type", "application/json; charset=utf-8") // res.Header.Get("Content-Type"))

		indent := "  "
		enc := json.NewEncoder(rsp)
		enc.SetIndent(indent, indent)

		if err != nil {

			re := errors.FromError(err)

			code := int(re.Code)
			rsp.WriteHeader(code)

			_ = enc.Encode(re)
			return // (4xx) Error
		}

		sort.SliceStable(res, func(i, j int) bool { return res[i].ID < res[j].ID })

		// JSON StartArray
		_, _ = rsp.Write([]byte("[\n" + indent))

		// Result View
		for i, item := range res {

			if item.WhatsAppBusinessAccount == nil || len(item.PhoneNumbers) == 0 {
				continue // DO NOT Show !
			}

			// JSON ArrayItem
			if i != 0 {
				_, _ = rsp.Write([]byte(", ")) // (",\n"+indent))
			}

			_ = enc.Encode(item)
		}
		// JSON EndArray
		_, _ = rsp.Write([]byte("]"))
	}
}

func (c *Client) renderWhatsAppBusinessAccounts(rsp http.ResponseWriter, req *http.Request) func(accounts []*whatsapp.WhatsAppBusinessAccount, err error) {
	return func(accounts []*whatsapp.WhatsAppBusinessAccount, err error) {

		header := rsp.Header()
		header.Set("Pragma", "no-cache")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "close")
		header.Set("Content-Type", "application/json; charset=utf-8") // res.Header.Get("Content-Type"))

		indent := "  "
		enc := json.NewEncoder(rsp)
		enc.SetIndent(indent, indent)

		if err != nil {

			re := errors.FromError(err)

			code := int(re.Code)
			rsp.WriteHeader(code)

			_ = enc.Encode(re)
			return // (4xx) Error
		}

		sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })

		// _ = enc.Encode(pages)

		// JSON StartArray
		_, _ = rsp.Write([]byte("[\n" + indent))

		// // Result View
		var (
			n int
			// item = whatsapp.WhatsAppBusinessAccount{
			// 	// Envelope: Sanitized View
			// }
		)
		// Sanitize fields
		for i, item := range accounts {

			n = len(item.PhoneNumbers)
			if n == 0 {
				continue // DO NOT Show !
			}

			// JSON ArrayItem
			if i != 0 {
				_, _ = rsp.Write([]byte(", ")) // (",\n"+indent))
			}

			// item.ID = WABA.ID
			// item.Name = WABA.Name
			// // item.Page.Picture     = page.Picture
			// // item.Page.AccessToken = page.GetAccessToken()

			// item.Accounts = page.Accounts
			// // item.SubscribedFields = page.SubscribedFields
			// item.SubscribedFields = intersectFields(
			// 	page.SubscribedFields, facebookPageFields,
			// )

			_ = enc.Encode(item)
		}
		// JSON EndArray
		_, _ = rsp.Write([]byte("]"))
	}
}

func (c *Client) whatsAppBackupAccounts(ctx context.Context) error {

	var (
		i, n   int
		WABAID string
		agent  = c.Gateway
		codec  = base64.RawURLEncoding
		WABAs  = c.whatsApp.GetAccounts()
		count  = len(WABAs)
		data   = make([]byte, 0, 16*count)
	)
	const (
		offset = '0' // 0x30
		delim  = ':' - offset
	)
	for _, account := range WABAs {
		if n = len(data); n != 0 && data[n-1] != delim {
			data = append(data, delim)
		}
		WABAID = account.ID
		for i, n = 0, len(WABAID); i < n; i++ {
			r := WABAID[i] // Expect: Digits ASCII
			data = append(data, r-offset)
		}
	}

	bak := codec.EncodeToString(data)
	// BACKUP NEW Internal State
	// c.Log.Info().Str("bak", bak).Msg("WHATSAPP: BACKUP")
	// return nil
	return agent.SetMetadata(
		ctx, map[string]string{
			"wa": bak,
		},
	)

	// -----------------------------------------------

	// // Save Bot's NEW internal state
	// var (
	// 	bak   string
	// 	agent = c.Gateway
	// 	codec = base64.RawURLEncoding
	// )

	// if data := c.whatsApp.Backup(); len(data) != 0 {
	// 	bak = codec.EncodeToString(data)
	// }
	// // BACKUP NEW Internal State
	// return agent.SetMetadata(
	// 	ctx, map[string]string{
	// 		"wa": bak,
	// 	},
	// )
}

func (c *Client) whatsAppRestoreAccounts() error {

	var (
		bak      string
		agent    = c.Gateway
		codec    = base64.RawURLEncoding
		metadata = agent.GetMetadata()
	)

	if metadata != nil {
		bak = metadata["wa"]
	}

	if bak == "" {
		// nothing to restore
		return nil
	}

	data, err := codec.DecodeString(bak)
	// if err == nil {
	// 	err = c.whatsApp.Restore(data)
	// }
	if err != nil {
		c.Log.Error().
			Str("error", "restore: invalid data sequence; "+err.Error()).
			Msg("WHATSAPP: ACCOUNTS")
		return err
	}
	// Decode back[ed]up data; WABAID(s) registered
	const (
		offset = '0' // 0x30
		delim  = ':' - offset
	)
	var (
		b      byte
		ascii  = make([]byte, 0, 16)
		WABAID = make([]string, 0, bytes.Count(data, []byte{delim}))
		grabID = func() {
			if len(ascii) != 0 {
				WABAID = append(
					WABAID, string(ascii),
				)
				ascii = ascii[:0]
			}
		}
	)

	for i := 0; i < len(data); i++ {
		if b = data[i]; b == delim {
			grabID()
			continue
		}
		ascii = append(ascii, b+offset)
	}
	grabID() // last

	if len(WABAID) == 0 {
		return nil // Nothing TODO !
	}

	accounts, err := c.fetchWhatsAppBusinessAccounts(
		context.TODO(), WABAID,
	)

	if err != nil {
		c.Gateway.Log.Error().
			Str("error", "restore: failed to get registered accounts; "+err.Error()).
			Msg("WHATSAPP: ACCOUNTS")
		return err
	}
	// Cache fetched accounts
	c.whatsApp.Register(accounts)
	return nil

	// ---------------------------------------------

	// var (
	// 	bak      string
	// 	agent    = c.Gateway
	// 	codec    = base64.RawURLEncoding
	// 	metadata = agent.GetMetadata()
	// )

	// if metadata != nil {
	// 	bak = metadata["wa"]
	// }

	// if bak == "" {
	// 	// nothing to restore
	// 	return nil
	// }

	// data, err := codec.DecodeString(bak)
	// if err == nil {
	// 	err = c.whatsApp.Restore(data)
	// }
	// if err != nil {
	// 	c.Log.Err(err).Msg("WHATSAPP: ACCOUNTS")
	// }
	// return err
}

// SearchWhatsAppAccounts search for [W]hats[A]pp[B]usiness[A]ccount(s) optionally filter[ed]-by given WABAIDs.
// If WABAID not specified - returns ALL [W]hats[A]pp[B]usiness[A]ccount(s) registered.
func (c *Client) SearchWhatsAppAccounts(ctx context.Context, WABAID ...string) []*whatsapp.WhatsAppBusinessAccount {
	return c.whatsApp.GetAccounts(WABAID...)
}

// GET ?whatsapp=[search][&id=WABA,...[&id=...]]
func (c *Client) handleWhatsAppSearchAccounts(rsp http.ResponseWriter, req *http.Request) {
	// TODO: Authorization Required
	query := req.URL.Query()
	WABAID := Fields(query["id"]...)
	accounts := c.SearchWhatsAppAccounts(req.Context(), WABAID...)
	c.renderWhatsAppBusinessAccounts(rsp, req)(accounts, nil)
}

func (c *Client) RemoveWhatsAppAccounts(ctx context.Context, WABAID ...string) (removed []*whatsapp.WhatsAppBusinessAccount, err error) {
	accounts := c.SearchWhatsAppAccounts(ctx, WABAID...)
	if n := len(WABAID); n != 0 && n != len(accounts) {
		// ERR: NOT ALL requested WhatsApp Business Accounts found !
		err = errors.BadRequest(
			"chat.whatsapp.accounts.remove.partial",
			"remove: not all requested WhatsApp Business Accounts found",
		)
		return // (400) Bad Request
	}

	if len(accounts) == 0 {
		return nil, nil // nothing
	}

	// var res []whatsAppAccountResult
	_, err = c.unsubscribeWhatsAppBusinessAccounts(ctx, accounts)
	if err == nil {
		_ = c.whatsApp.Deregister(accounts)
		err = c.whatsAppBackupAccounts(ctx)
		removed = accounts
	}

	return // removed, err
}

// GET ?whatsapp=remove[&id=WABA,...[&id=...]]
func (c *Client) handleWhatsAppRemoveAccounts(rsp http.ResponseWriter, req *http.Request) {
	// TODO: Authorization Required
	query := req.URL.Query()
	confirmed, _ := strconv.ParseBool(query.Get("ack"))
	if !confirmed {
		// ERR: User Confirmation required !
		http.Error(rsp, "remove: user confirmation ?ack= required", http.StatusBadRequest)
		return // (400) Bad Request
	}

	ctx := req.Context()
	WABAID := Fields(query["id"]...)
	accounts, err := c.RemoveWhatsAppAccounts(ctx, WABAID...)
	c.renderWhatsAppBusinessAccounts(rsp, req)(accounts, err)
}

// GET ?whatsapp=subscribe[&id=WABA,...[&id=...]]
func (c *Client) handleWhatsAppSubscribeAccounts(rsp http.ResponseWriter, req *http.Request) {
	// TODO: Authorization Required
	ctx := req.Context()
	query := req.URL.Query()
	WABAID := Fields(query["id"]...)
	accounts := c.SearchWhatsAppAccounts(ctx, WABAID...)
	results, err := c.subscribeWhatsAppBusinessAccounts(ctx, accounts)
	if len(accounts) != 0 {
		_ = c.whatsAppBackupAccounts(ctx)
	}
	c.renderWhatsAppBusinessResponse(rsp, req)(results, err)
}

// GET ?whatsapp=unsubscribe[&id=WABA,...[&id=...]]
func (c *Client) handleWhatsAppUnsubscribeAccounts(rsp http.ResponseWriter, req *http.Request) {
	// TODO: Authorization Required
	ctx := req.Context()
	query := req.URL.Query()
	WABAID := Fields(query["id"]...)
	accounts := c.SearchWhatsAppAccounts(ctx, WABAID...)
	res, err := c.unsubscribeWhatsAppBusinessAccounts(ctx, accounts)
	if len(accounts) != 0 {
		_ = c.whatsAppBackupAccounts(ctx)
	}
	c.renderWhatsAppBusinessResponse(rsp, req)(res, err)
}

// https://developers.facebook.com/docs/whatsapp/embedded-signup/steps#get-started-2
var whatsAppOAuthScopes = []string{
	// GET /<WABA_ID>/phone_numbers
	"whatsapp_business_management",
	// POST /<WABA_ID>/messages
	"whatsapp_business_messaging",
	// FIXME: EmbeddedSignup required ?
	// "business_management",
}

type GranularScope struct {
	Permission string   `json:"scope"`
	TargetIDs  []string `json:"target_ids,omitempty"`
}

// https://developers.facebook.com/docs/graph-api/reference/debug_token#fields
type AccessToken struct {

	// Access Type: USER, PAGE ...
	Type string `json:"type"`

	// The ID of the application this access token is for.
	AppID string `json:"app_id,omitempty"`

	// Name of the application this access token is for.
	Application string `json:"application,omitempty"`

	// Timestamp(sec) when this access token expires.
	ExpiresAt int64 `json:"expires_at,omitempty"`

	// Timestamp when app's access to user data expires.
	MaxAge int64 `json:"data_access_expires_at,omitempty"`

	// Whether the access token is still valid or not.
	IsValid bool `json:"is_valid,omitempty"`

	// Timestamp when this access token was issued.
	IssuedAt int64 `json:"issued_at,omitempty"`

	// General metadata associated with the access token.
	// Can contain data like 'sso', 'auth_type', 'auth_nonce'.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// For impersonated access tokens, the ID of the page this token contains.
	ProfileID string `json:"profile_id,omitempty"`

	// List of permissions that the user has granted for the app in this access token.
	Scopes []string `json:"scopes,omitempty"`

	// List of granular permissions that the user has granted for the app in this access token.
	// If permission applies to all, targets will not be shown.
	//
	// shape('scope' => string,'target_ids' => ?int[],)[]
	GranularScopes []*GranularScope `json:"granular_scopes,omitempty"`

	// The ID of the user this access token is for.
	UserID string `json:"user_id,omitempty"`
}

// https://developers.facebook.com/docs/graph-api/reference/debug_token#read
func (c *Client) inspectToken(token *oauth2.Token) (*AccessToken, error) {

	form := url.Values{
		"input_token": {token.AccessToken},
	}
	// ERR: (#100) You must provide an app access token, or a user access token that is an owner or developer of the app
	form = c.requestForm(form, token.AccessToken)

	req, err := http.NewRequest(http.MethodGet,
		"https://graph.facebook.com"+
			path.Join("/", c.Version, "debug_token")+
			"?"+form.Encode(),
		nil,
	)

	if err != nil {
		return nil, err
	}

	res, err := c.Client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var ret struct {
		Data  *AccessToken `json:"data,omitempty"`
		Error *graph.Error
	}

	err = json.NewDecoder(res.Body).Decode(&ret)

	if err == nil && ret.Error != nil {
		err = ret.Error
	}

	if err != nil {
		return nil, err
	}

	return ret.Data, nil
}

// https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived
func (c *Client) exchangeToken(accessToken string) (*oauth2.Token, error) {

	oauth := c.Config // shallowcopy

	token, err := oauth.Exchange(
		context.WithValue(context.Background(),
			oauth2.HTTPClient, c.Client,
		), "",
		oauth2.SetAuthURLParam("grant_type", "fb_exchange_token"),
		oauth2.SetAuthURLParam("fb_exchange_token", accessToken),
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

func (c *Client) whatsAppVerifyToken(accessToken string) error {

	token, err := c.inspectToken(
		&oauth2.Token{
			AccessToken: accessToken,
		},
	)
	if err != nil {
		return errors.BadGateway(
			"chat.bot.whatsapp.oauth.error",
			"WhatsApp: "+err.Error(),
		)
	}
	required := append(
		[]string(nil), whatsAppOAuthScopes...,
	)
	var require string
next:
	for i := 0; i < len(required); i++ {
		require = required[i]
		for _, grant := range token.GranularScopes {
			if strings.EqualFold(require, grant.Permission) {
				required = append(required[0:i], required[i+1:]...)
				i--
				continue next
			}
		}
	}
	if len(required) != 0 {
		return errors.BadRequest(
			"chat.bot.whatsapp.token.invalid",
			"whatsapp: token.scope=%#v required but not granted",
			required,
		)
	}
	return nil // OK
}

// https://developers.facebook.com/docs/whatsapp/embedded-signup/webhooks#unsubscribe-from-a-waba
func (c *Client) unsubscribeWhatsAppBusinessAccounts(ctx context.Context, accounts []*whatsapp.WhatsAppBusinessAccount) ([]whatsAppAccountResult, error) {

	// Do subscribe for page(s) webhook updates
	n := len(accounts)
	if n == 0 {
		// NO WhatsApp Business Account(s)
		// to be Unsubscribe -OR- ALLready ...
		return nil, nil
	}

	ret := make([]whatsAppAccountResult, n)
	for e, account := range accounts {
		ret[e].WhatsAppBusinessAccount = account
	}

	var (
		form = url.Values{
			// NO params
		}
		batch = make(
			[]graph.BatchRequest, n,
		)
	)

	for i, account := range accounts {
		// [RE]Authorize Each Request
		// form = c.requestForm(form, page.AccessToken)

		req := &batch[i]
		req.Method = http.MethodDelete
		req.RelativeURL = path.Join(
			account.ID, "subscribed_apps",
		)
		req.Body = "" // form.Encode()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(batch)

	if err != nil {
		return nil, err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch":           {buf.String()},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := c.whatsApp.AccessToken
	form = c.requestForm(form, accessToken)
	// Hide ?access_token= from URL query
	form.Del(graph.ParamAccessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://graph.facebook.com"+
			path.Join("/", c.Version),
		strings.NewReader(
			form.Encode(),
		),
	)
	if err != nil {
		return nil, err
	}
	// SET: Authorization !
	req.Header.Set("Authorization",
		"Bearer "+accessToken,
	)
	// PERFORM GraphAPI POST Batch request
	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	// Decode Batch Result(s)
	res := make([]*graph.BatchResult, 0, n)
	err = json.NewDecoder(rsp.Body).Decode(&res)
	if err != nil {
		return ret, err
	}
	var (
		re = struct {
			graph.Success              // Embedded (Anonymous)
			Error         *graph.Error `json:"error,omitempty"`
		}{
			// Allocate
		}
		body  = strings.NewReader("")
		codec = json.NewDecoder(body)
	)
	// BATCH Request(s) order !
	for i, account := range accounts {
		// NULLify
		re.Ok = false
		re.Error = nil
		// Decode JSON Result
		body.Reset(res[i].Body)
		err = codec.Decode(&re)

		if err == nil && re.Error != nil {
			err = re.Error
		}
		if err == nil && !re.Ok {
			err = fmt.Errorf("unsubscribe: WhatsApp Business Account ID=%s not confirmed", account.ID)
		}

		if err != nil {

			c.Gateway.Log.Err(err).
				Str("WABA:ID", account.ID).
				Str("WABA:Name", account.Name).
				Int("code", res[i].Code).
				Msg("WHATSAPP: UNSUBSCRIBE")

			rpcErr, _ := err.(*graph.Error)
			if rpcErr == nil {
				rpcErr = &graph.Error{
					Message: err.Error(),
				}
			}
			ret[i].Error = rpcErr
			continue
		}
		// SUCCESS !
		// account.SubscribedApps = nil
		account.SubscribedFields = nil
	}

	return ret, nil
}

// https://developers.facebook.com/docs/whatsapp/embedded-signup/webhooks#subscribe-to-a-whatsapp-business-account
func (c *Client) subscribeWhatsAppBusinessAccounts(ctx context.Context, accounts []*whatsapp.WhatsAppBusinessAccount) ([]whatsAppAccountResult, error) {
	// DO NOT process subscribed page(s)
	var (
		subscribe []*whatsapp.WhatsAppBusinessAccount
		account   *whatsapp.WhatsAppBusinessAccount
		results   = make([]whatsAppAccountResult, len(accounts))
		proc      []int // results[index] of entries to process subscriptions !
	)
	for i := 0; i < len(accounts); i++ {
		account = accounts[i]
		results[i].WhatsAppBusinessAccount = account
		// if account.SubscribedApps != nil {
		if len(account.SubscribedFields) != 0 {
			// IGNORE: already subscribed !
			if subscribe == nil {
				subscribe = make([]*whatsapp.WhatsAppBusinessAccount, 0, len(accounts)-1)
				subscribe = append(subscribe, accounts[0:i]...)
			}
			continue // OMIT
		}
		if subscribe != nil {
			subscribe = append(subscribe, account)
		}
		proc = append(proc, i)
	}

	if subscribe != nil {
		accounts = subscribe
	}

	// Do subscribe for Account(s) updates
	n := len(accounts)
	if n == 0 {
		// NO ANY Business Account(s) to Subscribe ! -OR-
		// ALLready Subscribed at least on ANYone field(s)
		return results, nil
	}

	var (
		form = url.Values{
			// "verify_token": []string{""},
		}
		batch = make([]graph.BatchRequest, n)
	)

	for i, account := range accounts {
		req := &batch[i]
		req.Method = http.MethodPost
		req.RelativeURL = path.Join(
			account.ID, "subscribed_apps",
		)
		req.Body = "" // form.Encode()
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(batch)

	if err != nil {
		return nil, err
	}

	form = url.Values{
		"include_headers": {"false"},
		"batch":           {buf.String()},
	}
	// TODO: USER_ACCESS_TOKEN
	accessToken := c.whatsApp.AccessToken
	form = c.requestForm(form, accessToken)

	// TODO: Increase Call Context Timeout * n
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://graph.facebook.com"+
			path.Join("/", c.Version),
		strings.NewReader(
			form.Encode(),
		),
	)
	if err != nil {
		return nil, err
	}
	// PERFORM GraphAPI Batch request
	rsp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	ret := make([]*graph.BatchResult, 0, n)
	err = json.NewDecoder(rsp.Body).Decode(&ret)
	if err != nil {
		return nil, err
	}
	var (
		res = struct {
			graph.Success              // Embedded (Anonymous)
			Error         *graph.Error `json:"error,omitempty"`
		}{
			// Alloc
		}
		body  = strings.NewReader("")
		codec = json.NewDecoder(body)
	)
	// BATCH Request(s) order !
	for i, account := range accounts {
		// NULLify
		res.Ok = false
		res.Error = nil
		// Decode JSON Result
		body.Reset(ret[i].Body)
		err = codec.Decode(&res)

		if err == nil && res.Error != nil {
			err = res.Error
		}
		if err == nil && !res.Ok {
			err = fmt.Errorf("subscribe: WhatsApp Business Account.ID=%s not confirmed", account.ID)
		}

		if err != nil {

			c.Gateway.Log.Err(err).
				Str("WABA:ID", account.ID).
				Str("WABA:Name", account.Name).
				Int("code", ret[i].Code).
				Msg("WHATSAPP: SUBSCRIBE")

			rpcErr, _ := err.(*graph.Error)
			if rpcErr == nil {
				rpcErr = &graph.Error{
					Message: err.Error(),
				}
			}
			results[proc[i]].Error = rpcErr
			continue
		}
		// SUCCESS !
		// account.SubscribedApps = true
		account.SubscribedFields = c.whatsApp.SubscribedFields
	}

	return results, nil

}

// https://developers.facebook.com/docs/whatsapp/embedded-signup/manage-accounts#get-shared-waba-id-with-access-token
func (c *Client) getSharedWhatsAppBusinessAccounts(userToken *oauth2.Token) ([]*whatsapp.WhatsAppBusinessAccount, error) {

	token, err := c.inspectToken(userToken)
	if err != nil {
		return nil, err
	}

	var WABAID []string
	for _, scope := range token.GranularScopes {
		// business_management: BusinessAccount.id(s)
		// whatsapp_business_management: WhatsAppBusinessAccount.id(s)
		if scope.Permission == "whatsapp_business_messaging" { // WhatsAppBusinessAccount.id(s)
			WABAID = append(WABAID, scope.TargetIDs...) // copy
			break
		}
	}
	return c.fetchWhatsAppBusinessAccounts(context.TODO(), WABAID)
}

func (c *Client) fetchWhatsAppBusinessAccounts(ctx context.Context, WABAID []string) ([]*whatsapp.WhatsAppBusinessAccount, error) {

	n := len(WABAID)
	if n == 0 {
		return nil, nil
	}

	// return WABAID, nil
	form := url.Values{
		"ids": []string{strings.Join(WABAID, ",")},
		"fields": []string{strings.Join([]string{
			"id", // default
			"name",
			"country",
			"ownership_type",
			"account_review_status",
			"business_verification_status",
			"subscribed_apps{whatsapp_business_api_data{id}}",
			"phone_numbers{id,verified_name,display_phone_number,is_official_business_account,is_pin_enabled,messaging_limit_tier,account_mode,name_status,status}",
		}, ",")},
	}
	accessToken := c.whatsApp.AccessToken
	form = c.requestForm(form, accessToken)
	// Hide ?access_token= from query ...
	delete(form, graph.ParamAccessToken)
	// Add Authorization header BELOW ...

	// https://developers.facebook.com/docs/graph-api/reference/whats-app-business-account/phone_numbers/#Reading
	req, err := http.NewRequestWithContext(ctx,
		http.MethodGet, "https://graph.facebook.com"+
			path.Join("/", c.Version, "/")+
			"?"+form.Encode(),
		http.NoBody,
	)

	if err != nil {
		return nil, err
	}
	// Authorize GraphAPI Request
	req.Header.Add("Authorization", "Bearer "+accessToken)

	res, err := c.Client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	type (
		whatsAppPhoneNumbersEdge struct {
			Data          []*whatsapp.WhatsAppBusinessAccountToNumberCurrentStatus `json:"data,omitempty"`
			*graph.Paging `json:"paging,omitempty"`
		}
		whatsAppBusinessApiData struct {
			ID   string `json:"id"`
			Name string `json:"name,omitempty"`
			Link string `json:"link,omitempty"`
		}
		whatsAppApplication struct {
			whatsAppBusinessApiData `json:"whatsapp_business_api_data"`
		}
		whatsAppSubscribedAppsEdge struct {
			Data []whatsAppApplication `json:"data,omitempty"`
			// *graph.Paging `json:"paging,omitempty"`
		}
		whatsAppBusinessAccountNode struct {
			PhoneNumbers                      *whatsAppPhoneNumbersEdge   `json:"phone_numbers,omitempty"`
			SubscribedApps                    *whatsAppSubscribedAppsEdge `json:"subscribed_apps,omitempty"`
			*whatsapp.WhatsAppBusinessAccount                             // embedded
		}
	)
	var (
		rpc = struct {
			// Public JSON result
			Error *graph.Error `json:"error,omitempty"`
			// Private JSON result
			data map[string]*whatsAppBusinessAccountNode
			raw  json.RawMessage
		}{
			data: make(map[string]*whatsAppBusinessAccountNode, n),
			// raw:  make(json.RawMessage, 0, res.ContentLength), // NO Content-Length Header provided !  =(
		}
	)

	err = json.NewDecoder(res.Body).Decode(&rpc.raw)
	if err != nil {
		// ERR: Invalid JSON
		return nil, err
	}
	// CHECK: for RPC `error` first
	err = json.Unmarshal(rpc.raw, &rpc) // {"error"}
	if err == nil && rpc.Error != nil {
		// RPC: Result Error
		err = rpc.Error
	}
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(rpc.raw, &rpc.data)
	if err != nil {
		// ERR: Unexpected JSON result
		return nil, err
	}

	list := make([]*whatsapp.WhatsAppBusinessAccount, 0, len(rpc.data))
	for _, item := range rpc.data {
		account := item.WhatsAppBusinessAccount
		account.PhoneNumbers = item.PhoneNumbers.Data
		// account.SubscribedApps = nil // NOT Subscribed !
		account.SubscribedFields = nil
		if apps := item.SubscribedApps; apps != nil {
			for _, app := range apps.Data {
				if app.ID == c.ClientID {
					// account.SubscribedApps = true
					account.SubscribedFields = c.whatsApp.SubscribedFields
					break
				}
			}
		}
		list = append(list, account)
	}
	return list, nil
}

func (c *Client) SetupWhatsAppBusinessAccounts(rsp http.ResponseWriter, req *http.Request) {

	// USER_ACCESS_TOKEN
	token, err := c.completeOAuth(req, whatsAppOAuthScopes...)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadRequest)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	accounts, err := c.getSharedWhatsAppBusinessAccounts(token)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadGateway)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	// _, err = c.subscribeWhatsAppBusinessAccounts(req.Context(), accounts)

	if err != nil {
		// _ = c.unsubscribeWhatsAppBusinessAccounts(req.Context(), accounts) // FIXME
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	// Merge & Save Accounts registered
	c.whatsApp.Register(accounts)
	_ = c.whatsAppBackupAccounts(context.TODO())

	// 200 OK
	// NOTE: Static HTML to help UI close popup window !
	_ = writeCompleteOAuthHTML(rsp, nil)
}

func (c *Client) whatsAppDialogPhoneNumber(chat *bot.Channel) (account *whatsapp.WhatsAppPhoneNumber, err error) {

	// Resolve & Attach | Recover ?
	var WAID string
	switch opts := chat.Properties.(type) {
	case *whatsapp.WhatsAppPhoneNumber:

		if opts != nil {
			account = opts
		}

	case map[string]string:

		WAID, _ = opts[paramWhatsAppNumberID]
		if WAID == "" {
			// NOTE: We cannot determine WHatsApp conversation side(s)
			// It all starts from WhatsApp Business Account Phone Number identification !..
			err = errors.BadRequest(
				"chat.bot.whatsapp.account.missing",
				"whatsapp: missing .number=? reference for .user=%s conversation",
				chat.Account.Contact, // chat.ChatID,
			)
			return // nil, err
		}

		account = c.whatsApp.GetPhoneNumber(WAID)

		if account != nil {
			chat.Properties = account
		}

		// default:
	}

	if account == nil {
		err = errors.NotFound(
			"chat.bot.whatsapp.account.not_found",
			"whatsapp: conversation .user=%s .peer=%s not found",
			chat.Account.Contact, WAID,
		)
		return // nil, err
	}

	// Resolved & Attached
	return account, nil
}

// {object:"whatsapp_business_account"} event handler
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components
// https://developers.facebook.com/docs/whatsapp/cloud-api/guides/set-up-webhooks
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components#notification-payload-object
func (c *Client) whatsAppOnUpdates(ctx context.Context, event *webhooks.Entry) {
	// The WhatsApp Business Account ID for the business that is subscribed to the webhook.
	var (
		update whatsapp.Update
		// accountId = event.ObjectID // WABAID
	)
	// The changes that triggered the webhook.
	for _, change := range event.Changes {
		// The type of notification.
		// The only option for this API is messages.
		switch change.Field {
		case "messages":
			// OK: Accept !
			// update.messages;
			// update.statuses;
			// update.pricing;
		default:
			c.Gateway.Log.Warn().
				Str("error", "update: field{"+change.Field+"} not supported").
				Msg("whatsapp.onUpdate")
			continue // process event.Changes
		}
		update = whatsapp.Update{} // sanitize
		err := change.GetValue(&update)
		if err != nil {
			c.Gateway.Log.Err(err).Msg("whatsapp.onUpdate")
			continue // process event.Changes
		}
		// TODO: Process update event args ...
		update.ID = event.ObjectID // WABAID
		if len(update.Messages) != 0 {
			c.whatsAppOnMessages(ctx, &update)
		} // else if statuses := update.Statuses; len(statuses) != 0 {

		// } else if errors := update.Errors; len(errors) != 0 {

		// }

	}
}

var (
	contactsInfo, _ = template.New("contacts").Parse(
		`{{range . -}}
Contact: {{.ContactName.Name}}
{{- if .Birthday}}
Birthday: {{.Birthday}}
{{- end}}
{{- range .Addresses}}
Address[{{.Type}}]: {{.Street}}, {{.City}}, {{.State}}, {{.Country}}, {{.ZIP}}
{{- end}}
{{- if .Organization}}
Organization: {{.Organization.Company}}
{{- end}}
{{- range .Emails}}
Email[{{.Type}}]: {{.Address}}
{{- end}}
{{- range .Phones}}
Phone[{{.Type}}]: {{.Phone}}{{if .WAID}} +WhatsApp{{end}}
{{- end}}
{{- range .URLs}}
URL[{{.Type}}]: {{.URL}}
{{- end}}
{{- end}}`,
	)
)

func (c *Client) whatsAppOnStatuses(
	ctx context.Context,
	update *whatsapp.Update,
	account *whatsapp.WhatsAppPhoneNumber,
) {

	// for _, status := range update.Statuses {
	// 	// TODO:
	// }
}

func (c *Client) whatsAppOnUnknown(
	ctx context.Context,
	update *whatsapp.Update,
	account *whatsapp.WhatsAppPhoneNumber,
	message *whatsapp.Message,
) {

	// Example:
	// {
	//   "object": "whatsapp_business_account",
	//   "entry": [
	//     {
	//       "id": "112885955140116",
	//       "changes": [
	//         {
	//           "value": {
	//             "messaging_product": "whatsapp",
	//             "metadata": {
	//               "display_phone_number": "94742477523",
	//               "phone_number_id": "109039232194741"
	//             },
	//             "contacts": [
	//               {
	//                 "profile": {
	//                   "name": "Yoga"
	//                 },
	//                 "wa_id": "94742238908"
	//               }
	//             ],
	//             "messages": [
	//               {
	//                 "from": "94742238908",
	//                 "id": "wamid.HBgLOTQ3NDIyMzg5MDgVAgASGCBBQUNGQTZBRkJFNERFQjE1OTdEM0JBNzQ2NzNDRUQ2QQA=",
	//                 "timestamp": "1685927475",
	//                 "type": "interactive"
	//               }
	//             ],
	//             "errors": [
	//               {
	//                 "code": 131000,
	//                 "title": "Something went wrong",
	//                 "message": "Something went wrong",
	//                 "error_data": {
	//                   "details": "Unsupported webhook payload"
	//                 }
	//               }
	//             ]
	//           },
	//           "field": "messages"
	//         }
	//       ]
	//     }
	//   ]
	// }

	// TODO: show logs
	var (
		notice  *zerolog.Event
		contact *whatsapp.Sender
	)
	for e, err := range update.Errors {

		if err == nil {
			continue
		}

		message = nil
		contact = nil
		notice = c.Log.Warn().Err(err)

		if len(update.Messages) > e {
			message = update.Messages[e]
			if message != nil {
				_ = notice.
					Str("from-waid", message.From).
					Str("msg-type", message.Type)

				contact = update.GetContact(message.From)
				if contact != nil {
					_ = notice.
						Str("from-name", contact.GetName())
				}
			}
		}

		if contact == nil && len(update.Contacts) > e {
			if contact = update.Contacts[e]; contact != nil {
				_ = notice.
					Str("from-waid", contact.WAID).
					Str("from-name", contact.GetName())
			}
		}

		notice.Msg("WHATSAPP.onError")
	}
}

func (c *Client) whatsAppOnSystemMsg(
	ctx context.Context,
	account *whatsapp.WhatsAppPhoneNumber, // TO: WABA
	message *whatsapp.Message, // FROM: WABA
) {

	// Example:
	// {
	// 	"from": "94742181320",
	// 	"id": "wamid.HBgLOTQ3NDIxODEzMjAVAgASGBJFRDk3MDRFMEM1MTNCQjA2NkMA",
	// 	"timestamp": "1685957943",
	// 	"system": {
	// 		"body": "User A changed from \u200e94742181320 to 94774211984\u200e",
	// 		"wa_id": "94774211984",
	// 		"type": "user_changed_number"
	// 	},
	// 	"type": "system"
	// }

	sender := message.System
	fromWAID := message.From // FROM_WA_ID
	// system:customer_changed_number
	chatWAID := sender.WAID // NEW_WA_ID; v12.0+
	if chatWAID == "" {
		chatWAID = sender.NewWAID // NEW_WA_ID; v11.0-
	}
	if chatWAID == "" {
		// FIXME: How to handle that ?
		// This is NOT {type:[customer|user]_changed_number}
		// Guess, this is {type:[customer|user]_identity_changed}
		// How NEW name will be provided ? Need more examples ...
		return // IGNORE
	}
	// NEW Customer profile FOR UPDATE
	customer := bot.Account{
		ID:        0,          // unknown: resolve from store ?!
		FirstName: "",         // unknown: resolve from store ?!
		Channel:   "whatsapp", // update.Product,
		Contact:   chatWAID,   // NEW_WA_ID: PHONE_NUMBER changed
	}

	// Update Customer profile
	// input: will create chat.client -if- not exists yet
	channel, err := c.Gateway.GetChannel(
		ctx, fromWAID, &customer,
	)

	if err != nil {
		// LOG: -ed by Gateway.GetChannel
		return // OK: ignore
	}

	return // OK: TODO nothing more !

	// Customer inactive ? NO dialog !
	if channel.IsNew() {
		return // OK: ignore
	}

	// output: merged
	customer = channel.Account
	// update: build
	sendUpdate := bot.Update{
		Title: channel.Title,
		Chat:  channel,
		User:  &channel.Account,
		Message: &chat.Message{
			Type: "contact",
			// Text: "/update",
			Text: sender.Body,
			Contact: &chat.Account{
				Id:        customer.ID,      // customer:owned
				Channel:   customer.Channel, // "whatsapp"
				Contact:   customer.Contact, // NEW_WA_ID
				FirstName: customer.FirstName,
				LastName:  customer.LastName,
				Username:  customer.Username,
			},
		},
	}

	err = c.Gateway.Read(ctx, &sendUpdate)
	if err != nil {
		// Failed to persist customer update
		return
	}

}

func (c *Client) whatsAppOnMessages(ctx context.Context, update *whatsapp.Update) {

	var (
		// TO:recipient
		businessId    = update.ID
		phoneNumber   = update.Metadata.DisplayPhoneNumber
		phoneNumberId = update.Metadata.PhoneNumberID
	)
	// TO:BUSINESS := update.ID // WABAID // [W]hats[A]pp [B]usiness[A]ccount Object[ID]
	// TO:NUMBER := update.Metadata // /WABA/phone_number(s)
	var recipient *whatsapp.WhatsAppPhoneNumber
	for _, number := range c.whatsApp.PhoneNumbers {
		if number.ID == update.Metadata.PhoneNumberID {
			recipient = number
			break
		}
	}
	if recipient == nil {
		c.Gateway.Log.Error().
			Str("chat", update.Product). // "whatsapp"
			Str("to:ba", businessId).
			Str("to:wa", phoneNumberId).
			Str("to", phoneNumber).
			Str("error", "recipient: account (wa) business (ba) phone-number (to) not found").
			Msg("whatsApp.onMessages")
		return // RECIPIENT: Business Account's Phone Number NOT FOUND !
	}
	// FIXME: Unknown
	// An array of error objects describing the error(s).
	if len(update.Errors) > 0 {
		c.whatsAppOnUnknown(ctx, update, recipient, nil)
		return // OK
	}
	// [update.Statuses] webhook is triggered when
	// a message is sent or delivered to a customer
	// or the customer reads the delivered message sent by a business
	if len(update.Statuses) > 0 {
		c.whatsAppOnStatuses(ctx, update, recipient)
		return // OK
	}

	// BATCH: Process message(s)...
	for _, message := range update.Messages {
		switch message.Type {
		case "text":
			// message.Text
		case "image":
			// message.Image
		case "audio":
			// message.Audio
		case "video":
			// message.Video
		case "button":
			// message.Button
		case "document":
			// message.Document
		case "interactive":
			// message.Interactive
		case "order":
			// message.Order
		case "sticker":
			// message.Sticker
		case "system": // – for customer number change messages
			// message.System
			c.whatsAppOnSystemMsg(
				ctx, recipient, message,
			)
			continue // OK: handled !

		// case "unknown":
		default:
			// FIXME: len(update.Errors) == 0
			c.whatsAppOnUnknown(
				ctx, update, recipient, message,
			)
			continue // OK: handled !
		}
		// GET Sender as WA Customer contact name
		sender := update.GetContact(message.From)
		// if sender == nil {
		// 	panic("WhatsAppBusinessAccount: update.Contacts(message.From(" + message.From + ")) NOT Found")
		// }
		contact := bot.Account{
			ID:        0,                // LOOKUP
			FirstName: sender.GetName(), // contacts[message.from] ? profile.name : "noname"
			Channel:   update.Product,   // "whatsapp",
			Contact:   sender.WAID,      // PHONE_NUMBER
		}

		// GET Chat channel dialog
		chatID := message.From
		channel, err := c.Gateway.GetChannel(
			ctx, chatID, &contact,
		)

		if err != nil {
			// Failed locate chat channel !
			re := errors.FromError(err)
			if re.Code == 0 {
				re.Code = (int32)(http.StatusBadGateway)
			}
			// http.Error(reply, re.Detail, (int)(re.Code))
			// return nil, re // 503 Bad Gateway
			c.Gateway.Log.Err(re).
				Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
				Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
				Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
				Str("chat", update.Product).
				Str("from", contact.Contact).
				Str("user", contact.DisplayName()).
				Msg("whatsApp.rely")
			continue
		}

		var sendMsg chat.Message
		// Facebook Message SENT Mapping !
		props := map[string]string{
			// ChatID: MessageID
			chatID: message.ID,
		}
		// WhatsApp Chat Bindings ...
		if channel.IsNew() {
			// BIND Channel START properties !
			props[paramWhatsAppNumberID] = recipient.ID
			props[paramWhatsAppAccountID] = recipient.Account.ID
			props[paramWhatsAppPhoneNumber] = recipient.PhoneNumber
		} // else { // BIND Message SENT properties ! }
		sendMsg.Variables = props

		referral := message.Referral
		if referral != nil {
			// You get the following webhook when a conversation is started
			// after a user clicks an ad with a Click to WhatsApp’s call-to-action
			// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#received-message-triggered-by-click-to-whatsapp-ads
		}

		switch message.Type {

		case "text": // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#text-messages

			text := message.Text
			sendMsg.Type = "text"
			sendMsg.Text = text.Body

		case "audio": // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#media-messages
			// message.Audio
			media := &message.Audio.Document
			sendMsg.File, err = c.whatsAppDownloadMedia(
				ctx, media, update.Metadata.PhoneNumberID, message.ID,
			)

			if err != nil {
				c.Log.Err(err).
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMediaMessage")
				err = nil
				continue // next: message(s)
			}

			sendMsg.Type = "file"
			sendMsg.Text = media.Caption

		case "image":
			// message.Image
			media := &message.Image.Document
			sendMsg.File, err = c.whatsAppDownloadMedia(
				ctx, media, update.Metadata.PhoneNumberID, message.ID,
			)

			if err != nil {
				c.Log.Err(err).
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMediaMessage")
				err = nil
				continue // next: message(s)
			}

			sendMsg.Type = "file"
			sendMsg.Text = media.Caption

		case "sticker":
			// message.Sticker
			media := &message.Sticker.Document
			sendMsg.File, err = c.whatsAppDownloadMedia(
				ctx, media, update.Metadata.PhoneNumberID, message.ID,
			)

			if err != nil {
				c.Log.Err(err).
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMediaMessage")
				err = nil
				continue // next: message(s)
			}

			sendMsg.Type = "file"
			sendMsg.Text = media.Caption

		case "video":
			// message.Video
			media := &message.Video.Document
			sendMsg.File, err = c.whatsAppDownloadMedia(
				ctx, media, update.Metadata.PhoneNumberID, message.ID,
			)

			if err != nil {
				c.Log.Err(err).
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMediaMessage")
				err = nil
				continue // next: message(s)
			}

			sendMsg.Type = "file"
			sendMsg.Text = media.Caption

		case "document":
			// message.Document
			media := message.Document
			sendMsg.File, err = c.whatsAppDownloadMedia(
				ctx, media, update.Metadata.PhoneNumberID, message.ID,
			)

			if err != nil {
				c.Log.Err(err).
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMediaMessage")
				err = nil
				continue // next: message(s)
			}

			sendMsg.Type = "file"
			sendMsg.Text = media.Caption

		case "button": // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#received-callback-from-a-quick-reply-button
			// message.Button (Quick Reply Button pressed)
			reply := message.Button
			text := reply.Data // button.code
			sendMsg.Type = "text"
			sendMsg.Text = text

		case "reaction":

			reaction := message.Reaction
			c.Gateway.Log.Warn().
				Str("wamid", reaction.WAMID).
				Str("reaction", reaction.Emoji).
				Str("error", "reaction: not supported").
				Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
				Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
				Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
				Str("chat", update.Product).
				Str("from", contact.Contact).
				Str("user", contact.DisplayName()).
				Msg("whatsApp.onMessage")
			continue // next: message(s)

		case "interactive":

			// "messages": [
			// 	{
			// 		"context": {
			// 			"from": "15550907777",
			// 			"id": "wamid.HBgMMzgwOTc2NTUzMzMyFQIAERgSRjBGMTY0RjkwODZBMDFDMjRBAA=="
			// 		},
			// 		"from": "380XXXXXXXXX",
			// 		"id": "wamid.HBgMMzgwOTc2NTUzMzMyFQIAEhggMUY5OUY1QjU1N0MyOTRDQzFBMDkyRDJBNDA3NjUwRTAA",
			// 		"timestamp": "1673430050",
			// 		"type": "interactive",
			// 		"interactive": {
			// 			"type": ". . .",
			// 			// . . .
			// 		}
			// 	}
			// ]
			var (
				replyContext = message.Context
				interactive  = message.Interactive
			)
			// Context MAY NOT be provided
			if replyContext != nil {
				sendMsg.ReplyToVariables = map[string]string{
					chatID: replyContext.MID,
				}
			}

			switch interactive.Type {
			case "button_reply":
				// "interactive": {
				// 	"type": "button_reply",
				// 	"button_reply": {
				// 		"id": "POSTBACK_BUTTON_2_ID",
				// 		"title": "REPLY_BUTTON_2"
				// 	}
				// }
				reply := interactive.QuickReply
				text := reply.ID // button.code

				sendMsg.Type = "text"
				sendMsg.Text = text

			case "list_reply":
				// "interactive": {
				// 	"type": "list_reply",
				// 	"list_reply": {
				// 		"id": "SECTION_2_ROW_1_ID",
				// 		"title": "SECTION_2_ROW_1_TITLE",
				// 		"description": "SECTION_2_ROW_1_DESCRIPTION"
				// 	}
				// }
				reply := interactive.ListReply
				text := reply.ID // button.code

				sendMsg.Type = "text"
				sendMsg.Text = text

			default:

				c.Gateway.Log.Warn().
					Str("interactive", interactive.Type).
					Str("error", "interactive: type not supported").
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).
					Str("from", contact.Contact).
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onMessageReply")
				continue // next: message(s)

			}

		// case "order":

		case "system":
			// for customer number change messages
			// message.System

		case "unknown": // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#unknown-messages
			// message.Errors
		default:
			// message.Location // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#location-messages
			if location := message.Location; location != nil {
				// FIXME: Google Maps Link to Place with provided coordinates !
				sendMsg.Type = "text"
				sendMsg.Text = fmt.Sprintf(
					"https://www.google.com/maps/place/%f,%f",
					location.Latitude, location.Longitude,
				)

			} else if contacts := message.Contacts; contacts != nil {
				// message.Contacts // https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples#contacts-messages
				// Convert given .Contacts to
				// human-readable .Text message
				buf := bytes.NewBuffer(nil)
				err := contactsInfo.Execute(
					buf, contacts,
				)
				if err != nil {
					buf.Reset()
					_, _ = buf.WriteString(err.Error())
				}
				// Resolve the first ContactInfo from the list
				sentContact := message.Contacts[0]
				sendContact := &chat.Account{
					// Id:        0,
					FirstName: sentContact.FirstName,
					LastName: strings.Join([]string{
						sentContact.MiddleName, sentContact.LastName,
					}, " "),
				}

				if len(sentContact.Phones) != 0 {
					sendContact.Channel = "phone"
					sendContact.Contact = sentContact.Phones[0].Phone
				} else if len(sentContact.Emails) != 0 {
					sendContact.Channel = "email"
					sendContact.Contact = sentContact.Emails[0].Address
				} else {
					sendContact.Channel = "name"
					sendContact.Contact = sentContact.Name
				}

				sendMsg.Type = "contact" // "text"
				sendMsg.Text = buf.String()
				sendMsg.Contact = sendContact

			} else {

				c.Log.Warn().
					Str("error", "message: content type not supported; ignore").
					Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
					Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
					Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
					Str("chat", update.Product).        // "whatsapp"
					Str("from", contact.Contact).       // PHONE_NUMBER
					Str("user", contact.DisplayName()).
					Msg("whatsApp.onNewMessage")
				continue // next: message(s)
			}
		}
		// CAN Forward ?
		if sendMsg.Type == "" {
			// NO reaction for message content type or is malformed
			c.Log.Warn().
				Str("error", "NO reaction for the message content received -or- is malformed").
				Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
				Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
				Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
				Str("chat", update.Product).        // "whatsapp"
				Str("from", contact.Contact).       // PHONE_NUMBER
				Str("user", contact.DisplayName()).
				Msg("whatsApp.onNewMessage")
			err = nil
			continue // IGNORE
		}

		sendUpd := bot.Update{
			Title:   channel.Title,
			Chat:    channel,
			User:    &channel.Account,
			Message: &sendMsg,
		}

		// Rely Bot's Account Message received !
		var bound interface{}
		if channel.IsNew() {
			bound = channel.Properties
			channel.Properties = props // map[string]string
		}
		err = c.Gateway.Read(ctx, &sendUpd)
		if bound != nil {
			if env, _ := bound.(map[string]string); env != nil {
				// merge/reset with START/NEW properties
				for param, value := range props {
					env[param] = value
				}
			}
			channel.Properties = bound
		}
		if err != nil {
			c.Log.Err(err).
				Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
				Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
				Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
				Str("chat", update.Product).        // "whatsapp"
				Str("from", contact.Contact).       // PHONE_NUMBER
				Str("user", contact.DisplayName()).
				Msg("whatsApp.onNewMessage")
			err = nil
			continue
		}
		c.Log.Info().
			Str("to", recipient.PhoneNumber).   // WhatsApp [PhoneNumber] Display
			Str("to:wa", recipient.ID).         // WhatsApp [PhoneNumber] ID
			Str("to:ba", recipient.Account.ID). // WhatsApp [BusinessAccount] ID
			Str("chat", update.Product).        // "whatsapp"
			Str("from", contact.Contact).       // PHONE_NUMBER
			Str("user", contact.DisplayName()).
			Msg("whatsApp.onNewMessage")
	}
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#retrieve-media-url
func (c *Client) whatsAppRetrieveMediaURL(ctx context.Context, media *whatsapp.Document, phoneNumberId string) error {

	if media.ID == "" {
		panic("WHATSAPP: GET /MEDIA.ID required but missing")
	}

	uri := "https://graph.facebook.com" +
		path.Join("/", c.Version, media.ID)

	if phoneNumberId != "" {
		// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#parameters-2
		uri += "?phone_number_id=" + url.QueryEscape(phoneNumberId)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, uri,
		nil,
	)

	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.whatsApp.AccessToken)

	res, err := c.Client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	// {
	// 	"url": "https:\/\/lookaside.fbsbx.com\/whatsapp_business\/attachments\/?mid=564952118844469&ext=1672849737&hash=ATsyMpSaV_WxR3Br_g9BaknBhBlKzULEcCGeboa2XbcIzw",
	// 	"mime_type": "image\/jpeg",
	// 	"sha256": "838ec83e79a72b51f8d927b7af0265d5fea751692d305b1f916efeb202633ba0",
	// 	"file_size": 93895,
	// 	"id": "564952118844469",
	// 	"messaging_product": "whatsapp"
	// }
	rpc := struct {
		Error *graph.Error `json:"error,omitempty"`
		URL   string       `json:"url,omitempty"`
		*whatsapp.Document
	}{
		Document: media, // refresh partial field(s)
	}

	err = json.NewDecoder(res.Body).Decode(&rpc)
	// if err != nil {
	// 	// ERR: Invalid JSON value
	// 	return err
	// }

	if err == nil && rpc.Error != nil {
		err = rpc.Error
	}

	if err != nil {
		return err
	}

	// Populate result field(s)
	media.Link = rpc.URL
	// media.SHA256 = rpc.SHA256
	// media.MIMEType = rpc.MIMEType
	// media.FileSize = rpc.FileSize

	return nil
}

func (c *Client) whatsAppMediaClient() (client *http.Client) {
	client = c.media
	if client != nil {
		return // client
	}
	media := *(http.DefaultClient) // shallowcopy
	transport := media.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	switch tr := transport.(type) {
	case *bot.TransportDump:
		if tr.WithBody {
			tr = &bot.TransportDump{
				Transport: tr.Transport,
				WithBody:  false,
			}
			transport = tr
		}
	default:
		transport = &bot.TransportDump{
			Transport: transport,
			WithBody:  false,
		}
	}
	media.Transport = transport
	client = &media
	c.media = client
	return // client
}

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#download-media
func (c *Client) whatsAppDownloadMedia(ctx context.Context, media *whatsapp.Document, phoneNumberId, messageId string) (*chat.File, error) {

	if media.Link == "" {
		err := c.whatsAppRetrieveMediaURL(ctx, media, phoneNumberId)
		if err != nil {
			return nil, err
		}
		if media.Link == "" {
			return nil, errors.BadRequest(
				"chat.bot.whatsapp.media.link.missing",
				"whatsapp: download media.link required but missing",
			)
		}
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, media.Link,
		nil,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.whatsApp.AccessToken)

	res, err := c.whatsAppMediaClient().Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.BadGateway(
			"chat.bot.whatsapp.media.download.error",
			"whatsapp: media; download: ("+strconv.Itoa(res.StatusCode)+") "+res.Status,
		)
	}

	doc := &chat.File{
		Id:   0,
		Url:  "", // media.Link; (401) Authorization Required
		Mime: media.MIMEType,
		Name: media.Filename,
		Size: media.FileSize,
	}

	// Content-Type
	if mediaType := res.Header.Get("Content-Type"); mediaType != "" {
		// Split: mediatype/subtype[;opt=param]
		if mediaType, _, re := mime.ParseMediaType(mediaType); re == nil {
			doc.Mime = mediaType
		}
	}
	// Content-Length
	doc.Size = res.ContentLength
	// Content-Disposition
	if disposition := res.Header.Get("Content-Disposition"); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if filename := params["filename"]; filename != "" {
				// RFC 7578, Section 4.2 requires that if a filename is provided, the
				// directory path information must not be used.
				switch filename = filepath.Base(filename); filename {
				case ".", string(filepath.Separator):
					// invalid
				default:
					doc.Name = filename
				}
			}
		}
	}
	// Generate unique filename
	var (
		filebase = strings.Map(
			func(r rune) rune {
				switch r {
				case '_', '-', '.':
					return -1
				}
				return r
			},
			time.Now().Format("2006-01-02_15-04-05.999"),
		) // combines media filename with the timestamp received
		filename = filepath.Base(doc.Name)
		filexten = filepath.Ext(filename)
	)
	filename = filename[0 : len(filename)-len(filexten)]
	if mediaType := doc.Mime; mediaType != "" {
		// Get file extension for MIME type
		var ext []string
		switch filexten {
		default:
			ext = []string{filexten}
		case "", ".":
			switch strings.ToLower(mediaType) {
			case "application/octet-stream":
				ext = []string{".bin"}
			case "image/jpeg": // IMAGE
				ext = []string{".jpg"}
			case "audio/mpeg": // AUDIO
				ext = []string{".mp3"}
			case "audio/ogg": // VOICE
				ext = []string{".ogg"}
			default:
				// Resolve for MIME type ...
				ext, _ = mime.ExtensionsByType(mediaType)
			}
		}
		// Split: mediatype[/subtype]
		var subType string
		if slash := strings.IndexByte(mediaType, '/'); slash > 0 {
			subType = mediaType[slash+1:]
			mediaType = mediaType[0:slash]
		}
		if len(ext) == 0 { // != 1 {
			ext = strings.FieldsFunc(
				subType,
				func(c rune) bool {
					return !unicode.IsLetter(c)
				},
			)
			for n := len(ext) - 1; n >= 0; n-- {
				if ext[n] != "" {
					ext = []string{
						"." + ext[n],
					}
					break
				}
			}
		}
		if n := len(ext); n != 0 {
			filexten = ext[n-1] // last
		}
		if filename == "" || strings.EqualFold(filename, "file") {
			filename = strings.ToLower(mediaType)
			switch mediaType {
			case "image", "audio", "video":
			default:
				filename = "file"
			}
			if messageId != "" {
				filename += "_" + media.ID // messageId
			}
		}
	}
	// Build unique filename
	if filename != "" {
		filename += "_"
	}
	filename += filebase
	if filexten != "" {
		filename += filexten
	}
	// Populate unique filename
	doc.Name = filename

	// CONNECT: storage service
	serviceClient := client.DefaultClient
	storageClient := storage.NewFileService("storage", serviceClient)
	upstream, err := storageClient.UploadFile(context.TODO())

	if err != nil {
		return nil, err
	}

	// c.Gateway.Log.Debug().Interface("media", media).Msg("storage.uploadFile")
	err = upstream.Send(&storage.UploadFileRequest{
		Data: &storage.UploadFileRequest_Metadata_{
			Metadata: &storage.UploadFileRequest_Metadata{
				DomainId: c.Gateway.DomainID(), // recipient.DomainID(),
				MimeType: doc.Mime,
				Name:     doc.Name,
				Uuid:     uuid.Must(uuid.NewRandom()).String(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	// defer stream.Close()

	var (
		n    int
		buf  = make([]byte, 4096) // Chunks Size
		data = storage.UploadFileRequest_Chunk{
			// Chunk: nil, // buf[:],
		}
		push = storage.UploadFileRequest{
			Data: &data,
		}
		sent int64
	)
	for {
		n, err = res.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				err = nil
			} else {
				break
			}
		}
		data.Chunk = buf[0:n]
		err = upstream.Send(&push)
		if err != nil {
			break
		}
		if n == 0 {
			break
		}
		sent += int64(n)
	}

	if err != nil {
		return nil, err
	}

	var ret *storage.UploadFileResponse
	ret, err = upstream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	fileURI := ret.FileUrl
	if path.IsAbs(fileURI) {
		// NOTE: We've got not a valid URL but filepath
		srv := c.Gateway.Internal
		hostURL, err := url.ParseRequestURI(srv.HostURL())
		if err != nil {
			panic(err)
		}
		fileURL := &url.URL{
			Scheme: hostURL.Scheme,
			Host:   hostURL.Host,
		}
		fileURL, err = fileURL.Parse(fileURI)
		if err != nil {
			panic(err)
		}
		fileURI = fileURL.String()
		ret.FileUrl = fileURI
	}

	doc.Id = ret.FileId
	doc.Url = ret.FileUrl
	if doc.Size != sent {
		panic("whatsapp: download media; content length delta: " + strconv.FormatInt(doc.Size-sent, 10))
	}
	// doc.Size = sent // ret.Size // ???

	return doc, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// https://github.com/webitel/storage/blob/main/web/handlers.go#L75
// https://github.com/webitel/storage/blob/main/model/utils.go#L53
type storageError struct {
	Id            string `json:"id"`
	Message       string `json:"message"`               // Message to be display to the end user without debugging information
	DetailedError string `json:"detailed_error"`        // Internal error string to help the developer
	RequestId     string `json:"request_id,omitempty"`  // The RequestId that's also set in the header
	StatusCode    int    `json:"status_code,omitempty"` // The http status code
	Where         string `json:"-"`                     // The function where it happened in the form of Struct.Func
	IsOAuth       bool   `json:"is_oauth,omitempty"`    // Whether the error is OAuth specific
	params        map[string]interface{}
}

func (err *storageError) Error() string {
	return err.Message
}

// var membuf = &sync.Pool{
// 	New: func() interface{} {
// 		return make([]byte, 32*1024) // 32K
// 	},
// }

// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#upload-media
func (c *Client) whatsAppUploadMedia(ctx context.Context, from *whatsapp.WhatsAppPhoneNumber, media *chat.File) (*whatsapp.Document, error) {

	var (
		client                 = c.whatsAppMediaClient()
		formReader, formWriter = io.Pipe()
		formData               = multipart.NewWriter(formWriter)
	)

	// ASYNC Drain Media source
	go func() {

		defer formWriter.Close()
		defer formData.Close()

		err := formData.WriteField("messaging_product", "whatsapp")
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}

		// GET Media content source
		req, err := http.NewRequestWithContext(ctx,
			http.MethodGet, media.Url, http.NoBody,
		)
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}

		res, err := client.Do(req)
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			var rpcErr storageError
			err = json.NewDecoder(res.Body).Decode(&rpcErr)
			if err == nil && rpcErr.Message != "" {
				err = &rpcErr
			}
			if err != nil {
				formWriter.CloseWithError(err)
				return
			}
		}
		// MIMEType from source
		mediaType, _, err := mime.ParseMediaType(
			res.Header.Get("Content-Type"),
		)
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}
		// MIMEType default
		if mediaType == "" {
			mediaType = media.Mime
		}
		err = formData.WriteField("type", mediaType)
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}

		fieldname, filename := "file", media.Name
		fileHeader := make(textproto.MIMEHeader, 3)
		fileHeader.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
				escapeQuotes(fieldname), escapeQuotes(filename)),
		)
		// fieldhead.Set("Content-Type", media.Mime)
		for _, hdr := range []string{
			"Content-Type",
			"Content-Length",
		} {
			fileHeader.Set(hdr, res.Header.Get(hdr))
		}
		fileData, err := formData.CreatePart(fileHeader)

		// // filename := media.Name
		// // fileData, err := formData.CreateFormFile("file", filename)
		// ERR: (#100) Param file must be a file with one of the following types:
		// text/plain,
		// video/mp4, video/3gpp,
		// image/jpeg, image/png, image/webp,
		// audio/aac, audio/mp4, audio/mpeg, audio/amr, audio/ogg, audio/opus,
		// application/vnd.ms-powerpoint, application/msword, application/vnd.openxmlformats-officedocument.wordprocessingml.document, application/vnd.openxmlformats-officedocument.presentationml.presentation, application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, application/pdf, application/vnd.ms-excel.
		// Received file of type 'application/octet-stream'.
		if err != nil {
			formWriter.CloseWithError(err)
			return
		}
		// Proxy Media content source ...
		_, err = io.Copy(fileData, res.Body)
		// chunk := membuf.Get().([]byte)
		// // defer membuf.Put(chunk)
		// _, err = io.CopyBuffer(fileData, res.Body, chunk)
		// membuf.Put(chunk)

		if err != nil {
			formWriter.CloseWithError(err)
			return
		}
		// DEFERED: err = res.Body.Close
	}()

	// AWAIT Proxy Media source
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, "https://graph.facebook.com"+
			path.Join("/", c.Version, from.ID, "media"),
		formReader,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.whatsApp.AccessToken)
	req.Header.Set("Content-Type", formData.FormDataContentType())

	res, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var rpc struct {
		Error *graph.Error `json:"error,omitempty"`
		// Uploaded Media Document {id}
		*whatsapp.Document
	}

	err = json.NewDecoder(res.Body).Decode(&rpc)
	if err == nil && rpc.Error != nil {
		err = rpc.Error
	}

	if err != nil {
		return nil, err
	}

	return rpc.Document, nil
}

func (c *Client) whatsAppSendUpdate(ctx context.Context, notice *bot.Update) error {

	sender, err := c.whatsAppDialogPhoneNumber(notice.Chat)
	if err != nil {
		return err
	}

	var (
		// account = notice.User
		channel = notice.Chat
		chatId  = channel.ChatID // channel.Account.Contact

		sentMsg = notice.Message
		sendMsg = &whatsapp.SendMessage{
			MessagingProduct: "whatsapp",
			RecipientType:    "individual",
			Status:           "",
			TO:               chatId,
		}
		enVars map[string]string //TODO
		setVar = func(key, val string) {
			if enVars == nil {
				enVars = make(map[string]string)
			}
			enVars[key] = val
		}
	)

	// Transform from internal to external message structure
	switch sentMsg.Type {
	case "text", "":

		sendMsg.Type = "text"
		sendMsg.Text = &whatsapp.Text{
			Body: sentMsg.Text,
		}

		keyboard := sentMsg.Buttons
		if keyboard == nil {
			// FIXME: Flow "menu" application does NOT process .Inline buttons =(
			keyboard = sentMsg.Inline
		}

		if len(keyboard) != 0 {
			buttons := whatsAppSendButtons(keyboard)
			// See: https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages#action-object
			if n := len(buttons); n == 0 {

			} else if n <= 3 {
				// send: interactive(button).action.button[s]:
				// You can have up to 3 buttons.
				// You cannot have leading or trailing spaces when setting the ID.
				for _, reply := range buttons {
					// id: Unique identifier for your button. Maximum length: 256 characters.
					// title: Emojis are supported, markdown is not. Maximum length: 20 characters.
					reply.Title = printable(reply.Title, 20, true)
				}
				sendMsg.Type = "interactive"
				sendMsg.Interactive = &whatsapp.Interactive{
					Type: "button",
					Body: &whatsapp.Content{
						Text: sentMsg.Text, // "BUTTON_TEXT",
					},
					Action: &whatsapp.Action{
						Buttons: buttons,
					},
				}

			} else {
				// send: interactive(list).action.section[s].row(s):
				// You can have a total of 10 rows across your sections.
				const max = 10
				var count = len(buttons)
				if count > max {
					count = max
				}
				list := make([]*whatsapp.Button, 0, count)
				for _, item := range buttons {
					// title: Maximum length: 24 characters
					// id: Maximum length: 200 characters
					item.ID = scanTextPlain(item.ID, 200)
					list = append(list, &item.Button)
				}

				sendMsg.Type = "interactive"
				sendMsg.Interactive = &whatsapp.Interactive{
					Type: "list",
					// Header: &whatsapp.Header{
					// 	Type: "text",
					// 	Text: "HEADER_TEXT",
					// },
					Body: &whatsapp.Content{
						Text: sentMsg.Text, // "BUTTON_TEXT",
					},
					// Footer: &whatsapp.Content{
					// 	Text: "FOOTER_TEXT",
					// },
					Action: &whatsapp.Action{
						Button: "RESPOND",
						Sections: []*whatsapp.Section{
							{Rows: list},
						},
					},
				}
			}
		}

	case "file":

		const (
			MediaImage = "image"
			MediaAudio = "audio"
			MediaVideo = "video"
		)

		var (
			src = sentMsg.File
			dst *whatsapp.Document
		)
		for _, mediaType := range []string{
			MediaImage, MediaAudio, MediaVideo,
		} {
			if strings.HasPrefix(src.Mime, mediaType) {
				if len(src.Mime) == len(mediaType) || src.Mime[len(mediaType)] == '/' {
					dst, err = c.whatsAppUploadMedia(ctx, sender, src)
					if err != nil {
						return err
					}
					sendMsg.Type = mediaType
					break
				}
			}
		}
		switch sendMsg.Type {
		case MediaImage:
			// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#upload-media
			sendMsg.Image = &dst.Media
			// sendMsg.Image = &whatsapp.Media{
			// 	ID:      "",
			// 	Link:    doc.Url,
			// 	Caption: sentMsg.Text,
			// }
		case MediaAudio:
			sendMsg.Audio = &dst.Media
			// sendMsg.Audio = &whatsapp.Media{
			// 	ID:   "",
			// 	Link: doc.Url,
			// }
		case MediaVideo:
			sendMsg.Video = &dst.Media
			// sendMsg.Video = &whatsapp.Media{
			// 	ID:      "",
			// 	Link:    doc.Url,
			// 	Caption: sentMsg.Text,
			// }
		default:
			sendMsg.Type = "document"
			sendMsg.Document = &dst.Media
			// sendMsg.Document = &whatsapp.Media{
			// 	ID:       "",
			// 	Link:     doc.Url,
			// 	Caption:  sentMsg.Text,
			// 	Filename: doc.Name,
			// }
		}

	case "joined": // ACK: ChatService.JoinConversation()

		peer := contactPeer(sentMsg.NewChatMembers[0])
		updates := c.Gateway.Template
		text, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMsg.Type).
				Msg("whatsApp.updateChatMember")
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// Send Text
		sendMsg.Type = "text"
		sendMsg.Text = &whatsapp.Text{
			Body: text,
		}

	case "left": // ACK: ChatService.LeaveConversation()

		peer := contactPeer(sentMsg.LeftChatMember)
		updates := c.Gateway.Template
		text, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMsg.Type).
				Msg("whatsApp.updateLeftMember")
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// Send Text
		sendMsg.Type = "text"
		sendMsg.Text = &whatsapp.Text{
			Body: text,
		}

	// case "typing":
	// case "upload":

	// case "invite":
	case "closed":

		updates := c.Gateway.Template
		text, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMsg.Type).
				Msg("whatsApp.updateChatClose")
		}
		// Template for update specified ?
		if text == "" {
			// IGNORE: message text is missing
			return nil
		}
		// Send Text
		sendMsg.Type = "text"
		sendMsg.Text = &whatsapp.Text{
			Body: text,
		}

	default:
		c.Log.Warn().
			Str("error", "send: message type="+sentMsg.Type+" not supported").
			Msg("whatsApp.sendMessage")
		return nil
	}

	res, err := c.whatsAppSendMessage(ctx, sender, sendMsg)

	if err != nil {
		return err // nil
	}

	// TARGET[chat_id]: MESSAGE[message_id]
	if len(res.Messages) == 1 {
		WAMID := res.Messages[0].ID
		setVar(chatId, WAMID)
	}
	// sentBindings := map[string]string {
	// 	"chat_id":    channel.ChatID,
	// 	"message_id": strconv.Itoa(sentMessage.MessageID),
	// }
	// attach sent message external bindings
	if sentMsg.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		sentMsg.Variables = enVars
	}
	// +OK
	return nil
}

func (c *Client) whatsAppSendMessage(ctx context.Context, sender *whatsapp.WhatsAppPhoneNumber, update *whatsapp.SendMessage) (*whatsapp.Update, error) {

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(update)

	if err != nil {
		// ERR: Failed to encode JSON request
		return nil, err
	}

	defer buf.Reset()

	accessToken := c.whatsApp.AccessToken
	query := c.requestForm(nil, accessToken)
	delete(query, graph.ParamAccessToken)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://graph.facebook.com"+
			path.Join("/", c.Version, sender.ID, "messages")+
			"?"+query.Encode(),
		&buf,
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.whatsApp.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var ret struct {
		Error *graph.Error `json:"error,omitempty"`
		*whatsapp.Update
	}
	// var ret struct {
	// 	Error    *graph.Error `json:"error,omitempty"`
	// 	Product  string       `json:"messaging_product,omitempty"`
	// 	Contacts []*struct {
	// 		ID          string `json:"wa_id"` // WHATSAPP_ID
	// 		PhoneNumber string `json:"input"` // PHONE_NUMBER
	// 	} `json:"contacts,omitempty"`
	// 	Messages []*struct {
	// 		ID string `json:"id"` // WAMID
	// 	} `json:"messages,omitempty"`
	// }

	err = json.NewDecoder(res.Body).Decode(&ret)

	if err == nil && ret.Error != nil {
		err = ret.Error
	}

	if err != nil {
		return nil, err
	}

	return ret.Update, nil
}

func coalesce(s ...string) string {
	for _, v := range s {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// returns printable text only
// OPTIONAL:
// - limited by `max` UTF-8 chars count
// - allow `emoji` symbols ?
func printable(s string, max int, emoji bool) string {

	if max <= 0 {
		max = len(s)
	}

	var (
		d, c int
		rs   []byte
		n    = max
		drop bool
		more bool
		// flags = make([]string, 0, 3)
	)
	for i, r := range s {

		// flags = flags[0:0]
		// if unicode.IsPrint(r) {
		// 	flags = append(flags, "print")
		// }
		// if unicode.IsDigit(r) {
		// 	flags = append(flags, "digit")
		// }
		// if unicode.IsLetter(r) {
		// 	flags = append(flags, "letter")
		// }
		// if unicode.IsNumber(r) {
		// 	flags = append(flags, "number")
		// }
		// if unicode.IsSymbol(r) {
		// 	flags = append(flags, "symbol")
		// }
		// if unicode.IsPunct(r) {
		// 	flags = append(flags, "punct")
		// }
		// if unicode.IsSpace(r) {
		// 	flags = append(flags, "space")
		// }
		// fmt.Printf("[%c]: %s\n", r, strings.Join(flags, "|"))

		drop = !unicode.IsPrint(r)
		drop = drop || (unicode.IsSymbol(r) && !emoji)

		// if !unicode.IsSymbol(r) && unicode.IsPrint(r) && (n < max || !unicode.IsSpace(r)) {
		if !drop && (n < max || !unicode.IsSpace(r)) {
			// Accept (pass-thru) valid UTF-8 character
			if n--; n <= 0 {
				if d != 0 {
					more = (i - d) < len(rs)
					rs = rs[0 : i-d]
				} else {
					more = (i + utf8.RuneLen(r)) < len(s)
					s = s[0 : i+utf8.RuneLen(r)]
				}
				break // limit chats exceeded
			}
			continue
		}
		// Invalidate (remove) character; Need modification(s)
		if rs == nil {
			rs = []byte(s)
		}
		c = utf8.RuneLen(r)
		rs = append(rs[0:i-d], rs[i-d+c:]...)
		d += c
	}
	if rs != nil {
		rs = bytes.TrimRightFunc(rs, unicode.IsSpace)
	} else {
		s = strings.TrimRightFunc(s, unicode.IsSpace)
	}
	// return strings.TrimRightFunc(s, unicode.IsSpace)
	// s = strings.TrimRightFunc(s, unicode.IsSpace)
	const dots = 2            // count of last runes to be replaced with dots
	if more && max-dots > 0 { // dots {
		c = 0 // zero
		if rs == nil {
			rs = []byte(s)
		}
		var i int
		for i = 0; i < dots; i++ {
			r, c := utf8.DecodeLastRune(rs)
			if r == utf8.RuneError {
				break
			}
			rs = rs[0 : len(rs)-c]
		}
		for ; i > 0; i-- {
			rs = append(rs, '.')
		}
		s = string(rs)
	}
	return s
}

func whatsAppSendButtons(keyboard []*chat.Buttons) []*whatsapp.QuickReply {

	var count int
	for _, row := range keyboard {
		count += len(row.Button)
	}

	var (
		reply   *whatsapp.QuickReply
		mempage = make([]whatsapp.QuickReply, count)
		replies = make([]*whatsapp.QuickReply, 0, count)
	)
	// for row, layout := range keyboard {
	// 	for col, button := range layout.Button {
	for _, layout := range keyboard {
		for _, button := range layout.Button {
			// Caption string
			// Text    string
			// Type    string
			// Code    string
			// Url     string
			switch strings.ToLower(button.Type) {
			case "email", "mail":
				// NOT SUPPORTED
			case "phone", "contact":
				// NOT SUPPORTED; WAID is the Customer's Phone Number !
			case "location":
				// NOT SUPPORTED
			case "url":
				// NOT SUPPORTED
			case "reply", "postback":
				// Buttons !
				if reply == nil {
					if len(mempage) != 0 {
						reply = &mempage[0]
						// mempage = mempage[1:]
					} else {
						reply = new(whatsapp.QuickReply)
					}
				}

				// ----- LIMIT interactive(list).action.section[s].row(s)
				// title: Maximum length: 24 characters
				// id: Maximum length: 200 characters
				// ----- LIMIT interactive(button).action.button(s)
				// title: Button title. It cannot be an empty string and must be unique within the message. Emojis are supported, markdown is not. Maximum length: 20 characters.
				// id: Unique identifier for your button. This ID is returned in the webhook when the button is clicked by the user. Maximum length: 256 characters.

				reply.Type = "reply"
				reply.Title = printable(coalesce(button.Text, button.Caption, button.Code), 24, true)
				reply.ID = scanTextPlain(coalesce(button.Code, button.Text), 256)

				replies = append(replies, reply)
				if len(mempage) != 0 {
					mempage = mempage[1:]
				}
				reply = nil

			default:
				// button.type is unknown;
			}
		}
	}

	return replies
}
