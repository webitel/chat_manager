package whatsapp

import (
	"sync"

	"github.com/golang/protobuf/proto"
	internal "github.com/webitel/chat_manager/bot/facebook/internal"
	protowire "google.golang.org/protobuf/proto"
)

// WhatsApp Business Account Metadata (Profile)
type WhatsAppPhoneNumber struct {
	// PHONE_NUMBER_ID
	*WhatsAppBusinessAccountToNumberCurrentStatus
	// [W]hats[A]pp[B]usiness[A]ccount that this PhoneNumber belongs to
	Account *WhatsAppBusinessAccount
}

var (
	// Default set of Webhooks
	// WhatsAppBusinessAccount
	// object fields to subscribe
	SubscribedFields = []string{
		"messages",
	}
)

// [W]hats[A]pp[B]usiness[A]ccount(s) Manager
type Manager struct {
	// --- App Options ---

	// Business Manager (System User) generated token for App WITH scope granted:
	// - business_management; FIXME
	// - whatsapp_business_management; POST /WHATSAPP_BUSINESS_ACCOUNT_ID[/phone_numbers]
	// - whatsapp_business_messaging;  POST /WHATSAPP_PHONE_NUMBER_ID/messages
	AccessToken string
	// Well-known set of Webhooks WhatsAppBusinessAccount
	// object fields subscribed, by default
	SubscribedFields []string
	// Guards the followings ...
	mx sync.RWMutex
	// App *meta.App
	// Accounts map[WABAID]*BusinessAccount index of ALL accounts connected
	Accounts map[string]*WhatsAppBusinessAccount
	// PhoneNumbers map[WAID]*PhoneNumber index ALL .PhoneNumbers from .Accounts attached
	PhoneNumbers map[string]*WhatsAppPhoneNumber
}

// NewManager
func NewManager(accessToken string, subscribedFields ...string) *Manager {

	if len(subscribedFields) == 0 {
		// Defaults
		subscribedFields = append(
			subscribedFields, SubscribedFields...,
		)
	}

	return &Manager{
		AccessToken:      accessToken,
		SubscribedFields: subscribedFields,
		// Accounts:     make(map[string]*WhatsAppBusinessAccount),
		Accounts:     make(map[string]*WhatsAppBusinessAccount),
		PhoneNumbers: make(map[string]*WhatsAppPhoneNumber),
	}
}

// MUST: be LOCKED
func (c *Manager) register(accounts []*WhatsAppBusinessAccount, force bool) {

	var evicted []*WhatsAppBusinessAccountToNumberCurrentStatus

	for _, ba := range accounts {
		WABA := ba.ID          // dst
		BA := c.Accounts[WABA] // src
		add := (BA == nil)     // Business: NOT REGISTERED; WABA: CREATE !
		if add {
			BA = ba

		} else {
			// WABA: EXISTS
			if !force {
				continue
			}
			// TODO: Deregister(srcBA.[PhoneNumbers])

			for _, wa := range BA.PhoneNumbers { // src
				// dst:WA(s) HAS src:WA.ID ? UPDATE(dst:WA) : REMOVE(src:WA) ;
				if ba.IndexPhoneNumber(wa.ID) == -1 {
					wa = BA.DelPhoneNumber(wa, false)
					delete(c.PhoneNumbers, wa.ID)
					evicted = append(evicted, wa)
					continue
				}
				// TODO: Register(dstBA.[PhoneNumbers]) below
			}
			BA = ba

		}
		// TODO: Register(dstBA.[PhoneNumbers])
		for _, WA := range BA.PhoneNumbers { // dst
			account := c.PhoneNumbers[WA.ID]
			if account == nil {
				account = &WhatsAppPhoneNumber{
					WhatsAppBusinessAccountToNumberCurrentStatus: WA,
				}
			}
			account.Account = BA // NEW
			c.PhoneNumbers[WA.ID] = account
		}
		c.Accounts[WABA] = BA // SET(ADD)
	}

	// return nil
}

func (c *Manager) Register(accounts []*WhatsAppBusinessAccount) {

	// DO NOT Edit while restoring ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	const reset = true
	c.register(accounts, reset)
}

// MUST: be LOCKED
func (c *Manager) deregister(accounts []*WhatsAppBusinessAccount) (evicted []*WhatsAppBusinessAccount) {

	n := len(accounts)
	if n == 0 {
		return // nil
	}

	evicted = make([]*WhatsAppBusinessAccount, 0, n)

	for _, ba := range accounts {
		WABA := ba.ID
		BA := c.Accounts[WABA]
		if BA == nil {
			// NOT REGISTERED
			continue
		}
		for _, WA := range BA.PhoneNumbers {
			delete(c.PhoneNumbers, WA.ID)
			delete(c.PhoneNumbers, WA.PhoneNumber)
		}
		delete(c.Accounts, WABA)
		evicted = append(evicted, BA)
	}

	if len(evicted) == 0 {
		evicted = nil // NULLify
	}

	return // evicted
}

func (c *Manager) Deregister(accounts []*WhatsAppBusinessAccount) (removed []*WhatsAppBusinessAccount) {
	// DO NOT Edit while restoring ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW
	return c.deregister(accounts)
}

// GetAccount lookup for [W]hats[A]pp[B]usiness[A]ccount by given WABAID.
func (c *Manager) GetAccount(WABAID string) *WhatsAppBusinessAccount {
	// CHECK: Preconditions
	if WABAID == "" || c == nil || len(c.Accounts) == 0 {
		return nil
	}
	c.mx.RLock()         // +R
	defer c.mx.RUnlock() // -R
	// LOOKUP: Accounts index
	return c.Accounts[WABAID]
}

// GetAccounts lookup for [W]hats[A]pp[B]usiness[A]ccount(s) optionally filter[ed]-by given WABAIDs.
// If WABAID not specified - returns ALL accounts registered.
func (c *Manager) GetAccounts(WABAID ...string) []*WhatsAppBusinessAccount {
	// CHECK: Preconditions
	if c == nil || len(c.Accounts) == 0 {
		return nil
	}
	n := len(WABAID)
	if n == 0 {
		c.mx.RLock() // +R
		n = len(c.Accounts)
		c.mx.RUnlock() // -R
	}

	if n == 0 {
		return nil
	}

	res := make([]*WhatsAppBusinessAccount, 0, n)

	c.mx.RLock()         // +R
	defer c.mx.RUnlock() // -R
	for _, ID := range WABAID {
		reg := c.Accounts[ID]
		if reg != nil {
			res = append(res, reg)
		}
	}
	if len(WABAID) == 0 {
		// NO `WABAID`s filter specified;
		// Returns ALL accounts registered;
		for _, WABA := range c.Accounts {
			res = append(res, WABA)
		}
	}
	return res
}

// GetPhoneNumber lookup for [W]hats[A]pp[B]usiness[A]ccount PhoneNumber by given WAID.
func (c *Manager) GetPhoneNumber(WAID string) *WhatsAppPhoneNumber {
	// CHECK: Preconditions
	if WAID == "" || c == nil || len(c.PhoneNumbers) == 0 {
		return nil
	}
	c.mx.RLock()         // +R
	defer c.mx.RUnlock() // -R
	// LOOKUP: PhoneNumbers index
	return c.PhoneNumbers[WAID]
}

// Backup registered Accounts data that MAY be Restore[d] in future.
func (c *Manager) Backup() []byte {
	// TODO: encode internal c.Pages accounts to secure data set
	var dataset internal.WhatsApp
	// DO NOT Edit while backing up ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	for _, BA := range c.Accounts {
		account := &internal.WhatsApp_BusinessAccount{
			Id:           BA.ID,
			Name:         BA.Name,
			Subscribed:   len(BA.SubscribedFields) != 0, // BA.SubscribedApps != nil,
			PhoneNumbers: make([]*internal.WhatsApp_PhoneNumber, 0, len(BA.PhoneNumbers)),
		}

		for _, WA := range BA.PhoneNumbers {
			account.PhoneNumbers = append(
				account.PhoneNumbers, &internal.WhatsApp_PhoneNumber{
					Id:           WA.ID,
					PhoneNumber:  WA.PhoneNumber,
					VerifiedName: WA.VerifiedName,
				},
			)
		}
		dataset.Accounts = append(
			dataset.Accounts, account,
		)
	}
	// Encode state ...
	data, err := protowire.Marshal(proto.MessageV2(&dataset))
	if err != nil {
		panic(err)
	}
	return data
}

// Restore Accounts data previously Back[ed]up.
func (c *Manager) Restore(data []byte) error {
	// TODO: decode secure data set into c.Pages accounts !
	// Decode state ...
	var dataset internal.WhatsApp
	err := protowire.Unmarshal(data, proto.MessageV2(&dataset))
	if err != nil {
		return err
	}

	// DO NOT Edit while restoring ...
	c.mx.Lock()         // +RW
	defer c.mx.Unlock() // -RW

	for _, bak := range dataset.Accounts {
		WABAID := bak.Id
		account := c.Accounts[WABAID]
		if account == nil {
			account = &WhatsAppBusinessAccount{
				ID:   bak.Id,
				Name: bak.Name,
			}
			if bak.Subscribed {
				// account.SubscribedApps = true
				account.SubscribedFields = c.SubscribedFields
			}
		}
		phoneNumbers := account.PhoneNumbers
		if cap(phoneNumbers) < len(bak.PhoneNumbers) {
			phoneNumbers = make([]*WhatsAppBusinessAccountToNumberCurrentStatus, len(phoneNumbers), len(bak.PhoneNumbers))
			copy(phoneNumbers, account.PhoneNumbers)
		}
		account.PhoneNumbers = phoneNumbers
		c.Accounts[WABAID] = account

		for i := len(bak.PhoneNumbers) - 1; i >= 0; i-- {
			WA := bak.PhoneNumbers[i]
			WAID := WA.Id
			// var account *WhatsAppBusinessAccountToNumberCurrentStatus // := getUser(token.Psid)
			accountPhone := c.PhoneNumbers[WAID]
			if accountPhone == nil {
				accountPhone = &WhatsAppPhoneNumber{
					WhatsAppBusinessAccountToNumberCurrentStatus: &WhatsAppBusinessAccountToNumberCurrentStatus{
						ID:           WAID,
						PhoneNumber:  WA.PhoneNumber,
						VerifiedName: WA.VerifiedName,
					},
					Account: account,
				}
			}
			phoneNumber := account.GetPhoneNumber(WAID)
			// var phoneNumber *WhatsAppBusinessAccountToNumberCurrentStatus
			// for _, reg := range account.PhoneNumbers {
			// 	if reg.ID == WAID {
			// 		phoneNumber = reg
			// 		break
			// 	}
			// }
			if phoneNumber == nil {
				phoneNumber = accountPhone.WhatsAppBusinessAccountToNumberCurrentStatus
				account.PhoneNumbers = append(account.PhoneNumbers, phoneNumber)
			}
			c.PhoneNumbers[WAID] = accountPhone
		}
	}
	return nil
}
