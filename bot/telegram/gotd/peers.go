package gotd

import (
	"context"
	"strings"
	"sync"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"go.uber.org/atomic"
)

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isPhoneNumber(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		switch {
		// case unicode.IsSpace(r):
		// case unicode.IsDigit(r):
		case r >= '0' && r <= '9':
		case r == '(' || r == ')':
		case r == '+' || r == '-':
		case r == ' ' || r == '\t':
		default:
			return false
		}
	}

	return true
	// r := rune(s[0])
	// return r == '+' || isDigit(r)
}

func (c *app) resolve(ctx context.Context, peer string) (peers.Peer, error) {

	peer = strings.TrimSpace(peer)

	if isPhoneNumber(peer) {
		// return c.peers.ResolvePhone(ctx, peer)
		return c.resolvePhone(ctx, peer)
	}

	return c.peers.Resolve(ctx, peer)
}

func cleanupPhone(phone string) string {
	var needClean bool
	for _, ch := range phone {
		if !isDigit(ch) {
			needClean = true
			break
		}
	}
	if !needClean {
		return phone
	}

	clean := strings.Builder{}
	clean.Grow(len(phone) + 1)

	for _, ch := range phone {
		if isDigit(ch) {
			clean.WriteRune(ch)
		}
	}

	return clean.String()
}

// ResolvePhone uses given phone to resolve User.
//
// Input example:
//
//	+13115552368
//	+1 (311) 555-0123
//	+1 311 555-6162
//	13115556162
//
// Note that Telegram represents phone numbers according to the E.164 standard
// without the plus sign (”+”) prefix. The resolver therefore takes an easy
// route and just deletes any non-digit symbols from phone number string.
func (c *app) resolvePhone(ctx context.Context, phone string) (peers.User, error) {
	// Lookup internal storage
	phone = cleanupPhone(phone)
	key, v, found, err := c.store.FindPhone(ctx, phone)
	if err != nil {
		return peers.User{}, err // errors.Wrap(err, "find by phone")
	}

	if found {
		user, err := c.peers.GetUser(ctx, &tg.InputUser{
			UserID:     key.ID,
			AccessHash: v.AccessHash,
		})
		if err != nil {
			return peers.User{}, err
		}
		return user, nil
	}

	// TODO: Resolve as NOT yet known but publicly available
	var (
		flood bool
		peer  *tg.ContactsResolvedPeer
	)
	for i := 0; i < 2; i++ {
		peer, err = c.Client.API().ContactsResolvePhone(ctx, phone)
		if flood, err = tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue // RETRY
			}
			return peers.User{}, err // FAILURE
		}
		break // SUCCESS
	}

	// Phone Number prefix may return multiple users that match condition
	var user *tg.User
	switch len(peer.GetUsers()) {
	case 1:
		var ok bool
		if user, ok = peer.Users[0].AsNotEmpty(); !ok {
			user = nil
		}
	case 0: // NOT FOUND
	default:
		// return peers.User{}, TOO_MUCH_PEERS
	}

	if user != nil {
		return c.peers.User(user), nil
	}

	return peers.User{}, &peers.PhoneNotFoundError{Phone: phone}
}

type (
	peersKey   = peers.Key
	peersValue = peers.Value
)

const (
	usersPrefix   = "users_"
	chatsPrefix   = "chats_"
	channelPrefix = "channel_"
)

// InmemoryStorage is basic in-memory Storage implementation.
type InmemoryStore struct {
	phones  map[string]peersKey
	data    map[peersKey]peersValue
	dataMux sync.Mutex // guards phones and data

	contactsHash atomic.Int64
}

var _ peers.Storage = (*InmemoryStore)(nil)

func (f *InmemoryStore) initLocked() {
	if f.phones == nil {
		f.phones = map[string]peersKey{}
	}
	if f.data == nil {
		f.data = map[peersKey]peersValue{}
	}
}

func (f *InmemoryStore) Purge() {

	if f == nil {
		return
	}

	f.dataMux.Lock()

	if len(f.phones) != 0 {
		f.phones = nil
	}

	if len(f.data) != 0 {
		f.data = nil
	}

	f.dataMux.Unlock()

	f.contactsHash.Store(0)
}

// Save implements Storage.
func (f *InmemoryStore) Save(ctx context.Context, key peersKey, value peersValue) error {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()
	f.initLocked()

	f.data[key] = value
	return nil
}

// Find implements Storage.
func (f *InmemoryStore) Find(ctx context.Context, key peersKey) (value peersValue, found bool, _ error) {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()

	value, found = f.data[key]
	return value, found, nil
}

// SavePhone implements Storage.
func (f *InmemoryStore) SavePhone(ctx context.Context, phone string, key peersKey) error {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()
	f.initLocked()

	f.phones[phone] = key
	return nil
}

// FindPhone implements Storage.
func (f *InmemoryStore) FindPhone(ctx context.Context, phone string) (key peersKey, value peersValue, found bool, err error) {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()

	key, found = f.phones[phone]
	if !found {
		return peersKey{}, peersValue{}, false, nil
	}
	value, found = f.data[key]
	return key, value, found, nil
}

// GetContactsHash implements Storage.
func (f *InmemoryStore) GetContactsHash(ctx context.Context) (int64, error) {
	v := f.contactsHash.Load()
	return v, nil
}

// SaveContactsHash implements Storage.
func (f *InmemoryStore) SaveContactsHash(ctx context.Context, hash int64) error {
	f.contactsHash.Store(hash)
	return nil
}

// InmemoryCache is basic in-memory Cache implementation.
type InmemoryCache struct {
	users    map[int64]*tg.User
	usersMux sync.Mutex

	usersFull    map[int64]*tg.UserFull
	usersFullMux sync.Mutex

	chats    map[int64]*tg.Chat
	chatsMux sync.Mutex

	chatsFull    map[int64]*tg.ChatFull
	chatsFullMux sync.Mutex

	channels    map[int64]*tg.Channel
	channelsMux sync.Mutex

	channelsFull    map[int64]*tg.ChannelFull
	channelsFullMux sync.Mutex
}

func (f *InmemoryCache) Purge() {

	if f == nil {
		return
	}

	f.usersMux.Lock()
	if len(f.users) != 0 {
		f.users = nil
	}
	f.usersMux.Unlock()

	f.usersFullMux.Lock()
	if len(f.usersFull) != 0 {
		f.usersFull = nil
	}
	f.usersFullMux.Unlock()

	f.chatsMux.Lock()
	if len(f.chats) != 0 {
		f.chats = nil
	}
	f.chatsMux.Unlock()

	f.chatsFullMux.Lock()
	if len(f.chatsFull) != 0 {
		f.chatsFull = nil
	}
	f.chatsFullMux.Unlock()

	f.channelsMux.Lock()
	if len(f.channels) != 0 {
		f.channels = nil
	}
	f.channelsMux.Unlock()

	f.channelsFullMux.Lock()
	if len(f.channelsFull) != 0 {
		f.channelsFull = nil
	}
	f.channelsFullMux.Unlock()
}

// SaveUsers implements Cache.
func (f *InmemoryCache) SaveUsers(ctx context.Context, users ...*tg.User) error {
	f.usersMux.Lock()
	defer f.usersMux.Unlock()
	if f.users == nil {
		f.users = map[int64]*tg.User{}
	}

	for _, u := range users {
		f.users[u.GetID()] = u
	}

	return nil
}

// SaveUserFulls implements Cache.
func (f *InmemoryCache) SaveUserFulls(ctx context.Context, users ...*tg.UserFull) error {
	f.usersFullMux.Lock()
	defer f.usersFullMux.Unlock()
	if f.usersFull == nil {
		f.usersFull = map[int64]*tg.UserFull{}
	}

	for _, u := range users {
		f.usersFull[u.GetID()] = u
	}

	return nil
}

// FindUser implements Cache.
func (f *InmemoryCache) FindUser(ctx context.Context, id int64) (*tg.User, bool, error) {
	f.usersMux.Lock()
	defer f.usersMux.Unlock()

	u, ok := f.users[id]
	return u, ok, nil
}

// FindUserFull implements Cache.
func (f *InmemoryCache) FindUserFull(ctx context.Context, id int64) (*tg.UserFull, bool, error) {
	f.usersFullMux.Lock()
	defer f.usersFullMux.Unlock()

	u, ok := f.usersFull[id]
	return u, ok, nil
}

// SaveChats implements Cache.
func (f *InmemoryCache) SaveChats(ctx context.Context, chats ...*tg.Chat) error {
	f.chatsMux.Lock()
	defer f.chatsMux.Unlock()
	if f.chats == nil {
		f.chats = map[int64]*tg.Chat{}
	}

	for _, c := range chats {
		f.chats[c.GetID()] = c
	}

	return nil
}

// SaveChatFulls implements Cache.
func (f *InmemoryCache) SaveChatFulls(ctx context.Context, chats ...*tg.ChatFull) error {
	f.chatsFullMux.Lock()
	defer f.chatsFullMux.Unlock()
	if f.chatsFull == nil {
		f.chatsFull = map[int64]*tg.ChatFull{}
	}

	for _, c := range chats {
		f.chatsFull[c.GetID()] = c
	}

	return nil
}

// FindChat implements Cache.
func (f *InmemoryCache) FindChat(ctx context.Context, id int64) (*tg.Chat, bool, error) {
	f.chatsMux.Lock()
	defer f.chatsMux.Unlock()

	c, ok := f.chats[id]
	return c, ok, nil
}

// FindChatFull implements Cache.
func (f *InmemoryCache) FindChatFull(ctx context.Context, id int64) (*tg.ChatFull, bool, error) {
	f.chatsFullMux.Lock()
	defer f.chatsFullMux.Unlock()

	c, ok := f.chatsFull[id]
	return c, ok, nil
}

// SaveChannels implements Cache.
func (f *InmemoryCache) SaveChannels(ctx context.Context, channels ...*tg.Channel) error {
	f.channelsMux.Lock()
	defer f.channelsMux.Unlock()
	if f.channels == nil {
		f.channels = map[int64]*tg.Channel{}
	}

	for _, c := range channels {
		f.channels[c.GetID()] = c
	}

	return nil
}

// SaveChannelFulls implements Cache.
func (f *InmemoryCache) SaveChannelFulls(ctx context.Context, channels ...*tg.ChannelFull) error {
	f.channelsFullMux.Lock()
	defer f.channelsFullMux.Unlock()
	if f.channelsFull == nil {
		f.channelsFull = map[int64]*tg.ChannelFull{}
	}

	for _, c := range channels {
		f.channelsFull[c.GetID()] = c
	}

	return nil
}

// FindChannel implements Cache.
func (f *InmemoryCache) FindChannel(ctx context.Context, id int64) (*tg.Channel, bool, error) {
	f.channelsMux.Lock()
	defer f.channelsMux.Unlock()

	c, ok := f.channels[id]
	return c, ok, nil
}

// FindChannelFull implements Cache.
func (f *InmemoryCache) FindChannelFull(ctx context.Context, id int64) (*tg.ChannelFull, bool, error) {
	f.channelsFullMux.Lock()
	defer f.channelsFullMux.Unlock()

	c, ok := f.channelsFull[id]
	return c, ok, nil
}
