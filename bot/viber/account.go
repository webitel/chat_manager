package viber

// https://developers.viber.com/docs/api/rest-bot-api/#get-account-info
type getAccount struct {
}

func (getAccount) method() string {
	return "get_account_info"
}

// Member of the bot’s public chat
type Moderator struct {
	Role string
	User
}

// Viber Account Info
type Account struct {
	// Unique numeric id of the account
	// Example: "pa:5752641035484782230"
	Id string `json:"id"`
	// Unique URI of the Account
	Uri string `json:"uri"`
	// Account icon URL
	// JPEG, 720x720, size no more than 512 kb
	Icon string `json:"icon"`
	// Account name
	// Max 75 characters
	Name string `json:"name"`
	// Registration Status
	Status
	// Account country.
	// 2 letters country code - ISO ALPHA-2 Code
	Country string `json:"country,omitempty"`
	// Account location (coordinates).
	// Will be used for finding accounts near me
	Location *Location `json:"location,omitempty"`
	// Account category
	Category string `json:"category,omitempty"`
	// Account sub-category
	Subcategory string `json:"subcategory,omitempty"`
	// Viber internal use
	Hostname string `json:"chat_hostname,omitempty"`
	// Conversation background URL
	// JPEG, max 1920x1920, size no more than 512 kb
	Background string `json:"background,omitempty"`
	// Account registered webhook URL
	Webhook string `json:"webhook,omitempty"`
	// Account registered events – as set by set_webhook request
	Events []string `json:"event_types,omitempty"`
	// Number of subscribers
	Subscribers int `json:"subscribers_count,omitempty"`
	// Members of the bot’s public chat.
	// id, name, avatar, role for each Public Chat member (admin/participant). Deprecated.
	Moderators []*Moderator `json:"members,omitempty"`
}

// Get Account Info
func (c *Bot) getMe(refresh bool) (*Account, error) {

	if !refresh {
		if me := c.Account; me != nil && me.Ok() {
			return me, nil
		}
	} else {
		// clear
		c.Account = nil
	}

	var (
		res Account
		req getAccount
	)

	err := c.do(&req, &res)
	if err == nil {
		err = res.Err()
	}

	if err == nil {
		c.Account = &res
	}

	return &res, err
}
