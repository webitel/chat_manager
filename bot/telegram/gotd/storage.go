package client

import (
	"context"
	"sync"

	"go.uber.org/atomic"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"

	backup "github.com/webitel/chat_manager/bot/telegram/gotd/internal/storage"
	protowire "google.golang.org/protobuf/proto"
)

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
type InmemoryStorage struct {
	phones  map[string]peersKey
	data    map[peersKey]peersValue
	dataMux sync.Mutex // guards phones and data

	contactsHash atomic.Int64
}

var _ peers.Storage = (*InmemoryStorage)(nil)

func (f *InmemoryStorage) initLocked() {
	if f.phones == nil {
		f.phones = map[string]peersKey{}
	}
	if f.data == nil {
		f.data = map[peersKey]peersValue{}
	}
}

func (f *InmemoryStorage) Purge() {

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
func (f *InmemoryStorage) Save(ctx context.Context, key peersKey, value peersValue) error {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()
	f.initLocked()

	f.data[key] = value
	return nil
}

// Find implements Storage.
func (f *InmemoryStorage) Find(ctx context.Context, key peersKey) (value peersValue, found bool, _ error) {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()

	value, found = f.data[key]
	return value, found, nil
}

func (f *InmemoryStorage) RestoreData(data []byte) error {

	// TODO: decode secure data set into c.Pages accounts !
	// Decode state ...
	var dataset backup.Dataset
	err := protowire.Unmarshal(data, &dataset)
	if err != nil {
		return err
	}

	f.dataMux.Lock()
	defer f.dataMux.Unlock()

	if f.data == nil {
		f.data = make(map[peersKey]peersValue, len(dataset.Users))
	}

	for id, hash := range dataset.Users {
		// users_* only !
		key := peersKey{
			Prefix: usersPrefix,
			ID:     id,
		}
		if _, ok := f.data[key]; ok {
			continue // DO NOT reassign !
		}
		f.data[key] = peersValue{AccessHash: hash}
	}

	return nil
}

func (f *InmemoryStorage) BackupData() []byte {

	f.dataMux.Lock()
	defer f.dataMux.Unlock()

	users := make(map[int64]int64, len(f.data))

	for key, val := range f.data {
		if key.Prefix != usersPrefix {
			continue
		}
		// users_* only !
		users[key.ID] = val.AccessHash
	}

	// Encode state ...
	data, err := protowire.Marshal(&backup.Dataset{Users: users})
	if err != nil {
		panic(err)
	}
	return data
}

// SavePhone implements Storage.
func (f *InmemoryStorage) SavePhone(ctx context.Context, phone string, key peersKey) error {
	f.dataMux.Lock()
	defer f.dataMux.Unlock()
	f.initLocked()

	f.phones[phone] = key
	return nil
}

// FindPhone implements Storage.
func (f *InmemoryStorage) FindPhone(ctx context.Context, phone string) (key peersKey, value peersValue, found bool, err error) {
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
func (f *InmemoryStorage) GetContactsHash(ctx context.Context) (int64, error) {
	v := f.contactsHash.Load()
	return v, nil
}

// SaveContactsHash implements Storage.
func (f *InmemoryStorage) SaveContactsHash(ctx context.Context, hash int64) error {
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
