package facebook

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/micro/go-micro/v2/errors"
	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	internal "github.com/webitel/chat_manager/bot/facebook.v12/internal"
	protowire "google.golang.org/protobuf/proto"
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

// ASID represents an [A]pp-[S]coped Facebook Page [ID]entifier
func (p *Page) ASID() string {
	if p != nil && p.Page != nil {
		return p.Page.ID
	}
	return ""
}

// ASID represents an [I]nsta[G]ram-[S]coped Facebook Page [ID]entifier
func (p *Page) IGSID() string {
	if p != nil && p.Page != nil {
		if p.Page.Instagram != nil {
			return p.Page.Instagram.ID
		}
	}
	return ""
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
	 // Pages indexes
	 // [ASID]:  page.id
	 // [IGSID]: page.instagram.id
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
				if IGSID := page.IGSID(); IGSID != "" { // NEW
					if igsid := that.IGSID(); igsid != "" {
						if igsid != IGSID {
							// REMOVE OLD Index
							delete(c.pages, igsid)
						} // else {
						// 	// IGSID MATCH Index
						// }
					} else {
						// CREATE NEW Index
						c.pages[IGSID] = that
					}
				} else if igsid := that.IGSID(); igsid != "" { // OLD
					// NOTE: page.Instagram == nil ! // NEW
					delete(c.pages, igsid)
				}
				that.Instagram = page.Instagram
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
				igsid := that.IGSID()
				if igsid != "" {
					delete(c.pages, igsid)
				}
			}
		}
	}
	// Install NEW !
	for _, add := range progress {
		ASID := add.ASID()
		page = c.pages[ASID]
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
		c.pages[ASID] = page
		IGSID := page.IGSID()
		if IGSID != "" {
			c.pages[IGSID] = page
		}
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
		for asid, page := range c.pages {
			if asid != page.ASID() {
				// NOT page.id index
				// MAY page.instagram.id index
				continue
			}
			// TOP::latest access_token
			pages = append(pages, page)
		}

	} else {
		// EXACT
		for _, asid := range pageIds {
			if page, ok := c.pages[asid]; ok {
				i := len(pages)-1
				for ; i >= 0 && pages[i] != page; i-- {
					// Lookup for duplicate index
				}
				if i >= 0 {
					// FOUND !
					continue
				}
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



func (c *messengerPages) backup() []byte {
	// TODO: encode internal c.Pages accounts to secure data set
	var dataset internal.Messenger

	// DO NOT Edit while backing up ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	for _, def := range c.pages {
		if !def.IsAuthorized() {
			continue
		}
		page := &internal.Page{
			Id:       def.ID,
			Name:     def.Name,
			// Picture:  def.Picture.Data.URL,
			Accounts: make([]*internal.Page_Account, 0, len(def.Accounts)),
			SubscribedFields: def.SubscribedFields,
		}
		// page.ID
		// page.Name
		if IGUser := def.Instagram; IGUser != nil {
			page.Instagram = &internal.Page_Instagram{
				Id:       IGUser.ID,
				Name:     IGUser.Name,
				Picture:  IGUser.PictureURL,
				Username: IGUser.Username,
			}
		}
		for _, account := range def.Accounts {
			// account.User
			// account.AccessToken
			// account.SubscribedFields
			page.Accounts = append(
				page.Accounts, &internal.Page_Account{
					Psid:             account.User.ID,
					Name:             account.User.Name,
					// Picture:          account.User.Picture.Data.URL,
					AccessToken:      account.AccessToken,
					
				},
			)
		}
		dataset.Pages = append(dataset.Pages, page)
	}
	// Encode state ...
	data, err := protowire.Marshal(proto.MessageV2(&dataset))
	if err != nil {
		panic(err)
	}
	return data
}

func (c *messengerPages) restore(data []byte) error {
	// TODO: decode secure data set into c.Pages accounts !
	// Decode state ...
	var dataset internal.Messenger
	err := protowire.Unmarshal(data, proto.MessageV2(&dataset))
	if err != nil {
		return err
	}

	var (
		users = make(map[string]*graph.User)
		getUser = func(psid string) *graph.User {
			user := users[psid]
			if user != nil {
				return user
			}
			lookup:
			for _, page := range c.pages {
				for _, grant := range page.Accounts {
					if grant.User.ID == psid {
						user = grant.User
						break lookup
					}
				}
			}
			if user != nil {
				users[psid] = user
				return user
			}
			return nil
		}
	)

	// DO NOT Edit while restoring ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	for _, bak := range dataset.Pages {
		ASID := bak.Id
		page := c.pages[ASID] // c.pages.getPage(ASID) // LOCKED
		if page == nil {
			page = &Page{
				Page: &graph.Page{
					ID:   ASID,
					Name: bak.Name,
					// Picture: &graph.PagePicture{
					// 	Data: &graph.ProfilePicture{
					// 		Width: 50,
					// 		Height: 50,
					// 		URL: channel.Picture,
					// 	},
					// },
					// AccessToken: bak.AccessToken,
				},
				// User: &graph.User{
				// 	ID: account.Psid,
				// 	Name: account.Name,
				// 	// Picture: &graph.PagePicture{
				// 	// 	Data: &graph.ProfilePicture{
				// 	// 		Width: 50,
				// 	// 		Height: 50,
				// 	// 		URL: account.Picture,
				// 	// 	},
				// 	// },
				// },
				SubscribedFields: bak.SubscribedFields,
			}
		}
		if IGUser := bak.Instagram; IGUser != nil {
			page.Instagram = &graph.InstagramUser{
				ID:             IGUser.Id,
				Name:           IGUser.Name,
				Username:       IGUser.Username,
				// PictureURL:     IGUser.Picture,
				// FollowersCount: 0,
				// Website:        "",
			}
		}
		// accounts := make([]*PageToken, 0, len(bak.Accounts))
		accounts := page.Accounts
		if cap(accounts) < len(bak.Accounts) {
			accounts = make([]*PageToken, len(accounts), len(bak.Accounts))
			copy(accounts, page.Accounts)
		}
		page.Accounts = accounts
		// NOTE: Latest (0) must be ACTIVATED !
		for i := len(bak.Accounts)-1; i >= 0; i-- {
			token := bak.Accounts[i]
			user := getUser(token.Psid)
			if user == nil {
				user = &graph.User{
					ID:   token.Psid,
					Name: token.Name,
					// Picture: &graph.PagePicture{
					// 	Data: &graph.ProfilePicture{
					// 		Width: 50,
					// 		Height: 50,
					// 		URL: account.Picture,
					// 	},
					// },
				}
				users[token.Psid] = user
			}
			page.Authorize(&PageToken{
				User:        user,
				IssuedAt:    0,
				ExpiresAt:   0,
				AccessToken: token.AccessToken,
			})
		}
		c.pages[ASID] = page
		IGSID := page.IGSID()
		if IGSID != "" {
			c.pages[IGSID] = page
		}
	}
	return nil
}