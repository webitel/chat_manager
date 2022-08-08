package client

import (
	"context"
	"strings"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
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

func (c *App) resolve(ctx context.Context, peer string) (peers.Peer, error) {

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
func (c *App) resolvePhone(ctx context.Context, phone string) (peers.User, error) {
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
