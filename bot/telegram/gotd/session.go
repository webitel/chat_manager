package client

import (
	"context"
	"encoding/base64"
	"sync"

	"github.com/gotd/td/telegram"
)

const (
	metadataSession = ".gotd"
)

type sessionStore struct {
	*App
	sync sync.Mutex
	data []byte
}

// Storage is secure persistent storage for client session.
//
// NB: Implementation security is important, attacker can abuse it not only for
// connecting as authenticated user or bot, but even decrypting previous
// messages in some situations.
var _ telegram.SessionStorage = (*sessionStore)(nil)

var sessionEncoding = base64.RawStdEncoding

func (c *sessionStore) canAccess() bool {
	return c.App.Gateway.Bot.GetId() != 0
}

func (c *sessionStore) LoadSession(ctx context.Context) ([]byte, error) {
	// RESTORE Session configuration
	c.sync.Lock()
	data := c.data
	c.sync.Unlock()
	if len(data) != 0 {
		return data, nil
	}
	if !c.canAccess() {
		return nil, nil // Bot NOT created !
	}
	metadata := c.App.Gateway.Bot.GetMetadata()
	s, _ := metadata[metadataSession]
	return sessionEncoding.DecodeString(s)
}

func (c *sessionStore) StoreSession(ctx context.Context, data []byte) error {
	// BACKUP Session configuration
	if data == nil {
		c.sync.Lock()
		data = c.data
		c.data = nil
		c.sync.Unlock()
		if data == nil {
			return nil // no cache data
		}
	} else if !c.canAccess() { // && data != nil
		// Bot profile NOT created
		c.sync.Lock()
		c.data = make([]byte, len(data))
		copy(c.data, data)
		c.sync.Unlock()
		return nil
	}
	// json.Compact(data)
	c.sync.Lock()
	c.data = nil // clear cache
	c.sync.Unlock()
	return c.App.Gateway.SetMetadata(
		ctx, map[string]string{
			metadataSession: sessionEncoding.EncodeToString(data),
		},
	)
}
