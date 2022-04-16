package facebook

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"sort"

	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	"github.com/webitel/chat_manager/bot/facebook.v12/messenger"
	"golang.org/x/oauth2"
)

// Retrive Facebook User profile and it's accounts (Pages) access granted
// Refresh Pages webhook subscription state
func (c *Client) getInstagramPages(token *oauth2.Token) (*UserAccounts, error) {

	// GET /me?fields=name,accounts{name,access_token}

	form := c.requestForm(url.Values{
		"fields": {"name,accounts{name,access_token,instagram_business_account.as(instagram){username}}"}, // ,profile_picture_url}}"},
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

func (c *Client) SetupInstagramPages(rsp http.ResponseWriter, req *http.Request) {

	// USER_ACCESS_TOKEN
	token, err := c.completeOAuth(req, messengerInstagramScope...)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadRequest)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	accounts, err := c.getInstagramPages(token)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusBadGateway)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	c.addInstagramPages(accounts)

	// Save Bot's NEW internal state
	var (

		dataset string
		agent = c.Gateway
	)

	if data := c.instagram.backup(); len(data) != 0 {
		encoding := base64.RawURLEncoding
		dataset = encoding.EncodeToString(data)
	}
	// OVERRIDE OR DELETE
	err = agent.SetMetadata(
		req.Context(), map[string]string{
			// "instagram": dataset,
			"ig": dataset,
		},
	)

	if err != nil {
		// http.Error(rsp, err.Error(), http.StatusInternalServerError)
		_ = writeCompleteOAuthHTML(rsp, err)
		return
	}

	// 200 OK
	// NOTE: Static HTML to help UI close popup window !
	_ = writeCompleteOAuthHTML(rsp, nil)
}

func (c *Client) addInstagramPages(accounts *UserAccounts) {
	_ = c.instagram.setPages(accounts)
}

func (c *Client) GetInstagramPages(rsp http.ResponseWriter, req *http.Request) {

	// TODO: Authorization Required

	query := req.URL.Query()
	pageId := Fields(query["id"]...)

	pages, err := c.instagram.getPages(pageId...)

	if err != nil {
		http.Error(rsp, err.Error(), http.StatusNotFound)
		return
	}

	sort.SliceStable(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })

	header := rsp.Header()
	header.Set("Pragma", "no-cache")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "close")
	header.Set("Content-Type", "application/json; charset=utf-8") // res.Header.Get("Content-Type"))

	indent := "  "
	enc := json.NewEncoder(rsp)
	enc.SetIndent(indent, indent)

	// _ = enc.Encode(pages)

	// JSON StartArray
	_, _ = rsp.Write([]byte("["))

	// Result View
	var (

		n int
		item = Page{
			Page: &graph.Page{
				// Envelope: Sanitized View
			},
		}
	)
	// Sanitize fields
	for _, page := range pages {

		if len(page.Accounts) == 0 {
			continue // DO NOT Show !
		}

		// JSON ArrayItem
		if n == 0 {
			indent = "\n"+indent
		} else if n == 1 {
			indent = ", "
		}
		_, _ = rsp.Write([]byte(indent))

		item.Page.ID          = page.ID
		item.Page.Name        = page.Name
		item.Page.Instagram   = page.Instagram
		// item.Page.Picture     = page.Picture
		// item.Page.AccessToken = page.GetAccessToken()

		item.Accounts         = page.Accounts
		item.SubscribedFields = page.SubscribedFields

		_ = enc.Encode(item)
		n++ // Output: Count
	}
	// JSON EndArray
	_, _ = rsp.Write([]byte("]"))
}



func (c *Client) WebhookInstagram(batch []*messenger.Entry) {

	for _, entry := range batch {
		if len(entry.Messaging) != 0 {
			// Array containing one messaging object.
			// Note that even though this is an array,
			// it will only contain one messaging object.
			// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#entry
			for _, event := range entry.Messaging {
				if event.Message != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messages
					_ = c.WebhookMessage(event)
				} else if event.Postback != nil {
					// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/messaging_postbacks
					_ = c.WebhookPostback(event)
				} // else {
				// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events#event_list
				// }
			}

		} // else if len(entry.Standby) != 0 {
		// 	// Array of messages received in the standby channel.
		// 	// https://developers.facebook.com/docs/messenger-platform/reference/webhook-events/standby
		// 	for _, event := range entry.Standby {
		// 		
		// 	}
		// }
	}
}