package facebook

import (
	"github.com/golang/protobuf/proto"
	graph "github.com/webitel/chat_manager/bot/facebook.v12/graph/v12.0"
	internal "github.com/webitel/chat_manager/bot/facebook.v12/internal"
	protowire "google.golang.org/protobuf/proto"
)

// Chat represents Facebook User TO Messenger Page conversation
type Chat struct {
	 // Facebook User. Sender, initiator
	 User graph.User
	 // Facebook Page. Bot, recipient
	 Page *Page
}



func backupAccounts(c *Client) []byte {
	// TODO: encode internal c.Pages accounts to secure data set
	var dataset internal.Messenger

	// DO NOT Edit while backing up ...
	c.pages.mx.Lock()         // +RW
	defer c.pages.mx.Unlock() // -RW

	for _, def := range c.pages.pages {
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

func restoreAccounts(c *Client, data []byte) error {
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
			for _, page := range c.pages.pages {
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
	c.pages.mx.Lock()         // +RW
	defer c.pages.mx.Unlock() // -RW

	for _, bak := range dataset.Pages {
		page := c.pages.pages[bak.Id] // c.pages.getPage(bak.Id) // LOCKED
		if page == nil {
			page = &Page{
				Page: &graph.Page{
					ID:   bak.Id,
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
		c.pages.pages[page.ID] = page
	}
	return nil
}
