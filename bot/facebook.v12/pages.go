package facebook

import (
	"sync"

	"github.com/micro/go-micro/v2/errors"
	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
)

type PageToken struct {
	 // Facebook User who granted access to Page (AccessToken)
	*graph.User
	 // AccessToken is an opaque <PAGE_ACCESS_TOKEN> string
	 // TODO: Hide `json:"-"` on production !!!!!!!!!!!!
	 AccessToken string `json:"-"` // access_token"`
	 // ---------------------------------
	 // AccessToken introspection details
	 // ---------------------------------
	 // IssuedAt unix timestamp (seconds)
	 IssuedAt int64 `json:"issued,omitempty"`
	 // ExpiresAt unix timestamp (seconds); If <Zero> - means Never !
	 ExpiresAt int64 `json:"expires,omitempty"`
}

func (e *PageToken) Equal(t *PageToken) bool {
	// FIXME: (e.Page.ID == t.Page.ID) ?
	if e.AccessToken == t.AccessToken {
		return true
	}
	if e.User.ID == t.User.ID {
		return true
	}
	return false
}

// Page represents Messenger's Page Account subscription
type Page struct {
	 // The Facebook Messanger Page engaged account(s)
	*graph.Page
	 // Accounts represents SET of PAGE_ACCESS_TOKEN and it's .User GRANTOR
	 Accounts []*PageToken `json:"accounts"`
	 // Webhook fields to which the app has subscribed on the Page
	 //
	 // GET /{.Page.ID}/subscribed_apps?fields=subscribed_fields.as(fields)
	 // Applications that have real time update subscriptions for this Page.
	 // Note that we will only return information about the current app
	 SubscribedFields []string `json:"subscribed_fields,omitempty"`
}

func (p *Page) GetAccessToken() string {
	return p.AccessToken
}

// Authorize sets provided token to use immediately !
func (p *Page) Authorize(token *PageToken) {

	var (
		accounts = p.Accounts
		n = len(accounts)
		i int // index of duplicate
	)
	for i = 0; i < n && !accounts[i].Equal(token); i++ {
		// Lookup for given token match !..
	}
	// PUSH TO FRONT !
	if i == n {
		// ADD !
		accounts = append(accounts, nil)
		copy(accounts[1:], accounts[0:n])
		
	} else { // i < n
		// SET !
		if i != 0 {
			copy(accounts[1:i+1], accounts[0:i])
		}
	}
	// PUSH TO FRONT !
	accounts[0] = token
	p.Accounts = accounts
	p.AccessToken = token.AccessToken
	// c.User = token.User
}

// Deathorize Page's p .Accounts for specified .User PSID
// If psid is an empty string - deauthorize Messenger Page at ALL !
// 
// if page.Deauthorize("") {
// 	// NOTE: page.IsAuthorized() == false
// }
func (p *Page) Deauthorize(psid string) []*PageToken {
	
	if psid == "" {
		// ALL !
		removed := p.Accounts
		p.Accounts = nil
		p.AccessToken = ""
		// p.User = nil
		return removed
	}

	var (

		accounts = p.Accounts
		removed []*PageToken
		n = len(accounts)
		i int
	)

	for i = 0; i < n && accounts[i].User.ID != psid; i++ {
		// Lookup for given token match !..
	}

	if i < n {
		removed = append(removed, accounts[i])
		accounts = append(accounts[0:i], accounts[i+1:]...)
		p.Accounts = accounts
		if i == 0 { // Current ?
			// Sanitize !
			p.AccessToken = ""
			// p.User = nil
			// Set previous one, if any !
			n = len(accounts)
			if n != 0 {
				token := accounts[0]
				p.AccessToken = token.AccessToken
				// p.User = token.User
			}
		}
	}

	return removed
}

// IsAuthorized is a shorthand for (p.AccessToken != "")
func (p *Page) IsAuthorized() bool {
	return p.AccessToken != ""
}




// The Messenger Pages state
type messengerPages struct {
	 mx sync.RWMutex
	 pages map[string]*Page
}

// Un/Install Facebook .User's Messenger .Pages accounts
func (c *messengerPages) setPages(accounts *UserAccounts) []*Page {

	var (

		page *Page // [RE]NEW!
		results = make([]*Page, 0, len(accounts.Pages))
		progress = append(([]*Page)(nil), accounts.Pages...)
	)

	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	// Uninstall .pages aceess, this user install before ...
	for asid, that := range c.pages {
		page = nil // NO MATCH !
		// for _, this := range accounts.Pages {
		for n := 0; n < len(progress); n++ {
			this := progress[n]
			if this.ID == asid {
				// Latest <PAGE+ACCESS_TOKEN> !
				page = this
				// Update latest info !
				that.Name = page.Name
				that.SubscribedFields = page.SubscribedFields
				// Mark as proceed !
				progress = append(progress[:n], progress[n+1:]...)
				break
			}
		}
		if page != nil {
			that.Authorize(&PageToken{
				User:        accounts.User,
				AccessToken: page.AccessToken,
				IssuedAt:    0,
				ExpiresAt:   0,
			})
			// UPDATED !
			results = append(results, that)
		} else {
			// DELETED !
			_ = that.Deauthorize(accounts.User.ID)
			if !that.IsAuthorized() {
				// PAGE deleted !
				delete(c.pages, asid)
			}
		}
	}
	// Install NEW !
	for _, add := range progress {
		page = c.pages[add.ID]
		if page == nil {
			page = &Page{
				Page:             add.Page,
				SubscribedFields: add.SubscribedFields,
			}
		}
		page.Authorize(&PageToken{
			User:        accounts.User,
			AccessToken: add.AccessToken,
			IssuedAt:    0,
			ExpiresAt:   0,
		})
		c.pages[add.ID] = page
		// CREATED !
		results = append(results, page)
	}

	return results
}

// ALL/Requested or nothing 
func (c *messengerPages) getPages(pageIds ...string) ([]*Page, error) {

	c.mx.RLock()         // +R
	defer c.mx.RUnlock() // -R

	// Simple case
	n := len(pageIds)
	if n == 0 {
		// return nil, nil
		// ALL
		n = len(c.pages)
	}

	// Prepare results ...
	pages := make([]*Page, 0, n)
	// Find all requested page(s) ...
	if len(pageIds) == 0 {
		// ALL
		for _, page := range c.pages {
			// TOP::latest access_token
			pages = append(pages, page)
		}

	} else {
		// EXACT
		for _, asid := range pageIds {
			if page, ok := c.pages[asid]; ok {
				pages = append(pages, page)
			} else {
				return nil, errors.NotFound(
					"bot.messenger.page.not_found",
					"messenger: page=%s not found",
					 asid,
				)
			}
		}
	}

	return pages, nil
}

func (c *messengerPages) getPage(id string) *Page {

	if id == "" {
		return nil
	}

	res, _ := c.getPages(id)

	if len(res) == 1 {
		return res[0]
	}

	return nil
}

func (c *messengerPages) delPage(id string) *Page {

	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	if page, ok := c.pages[id]; ok {
		delete(c.pages, id)
		return page
	}

	return nil
}