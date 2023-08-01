package sqlxrepo

import (
	"context"

	"github.com/webitel/chat_manager/app"
)

type ChatStore interface {

	// GetSession in front of the unique chatID member
	GetSession(ctx context.Context, chatID string) (chat *app.Session, err error)
	// GetSession(ctx context.Context, req *SearchOptions) (chat *app.Session, err error)

	// BindChannel merges given vars with corresponding channel unique identifier
	BindChannel(ctx context.Context, chatID string, vars map[string]string) (env map[string]string, err error)
	// GetMessages(ctx context.Context, search *SearchOptions) ([]*Message, error)

	// SaveMessage creates new historical message
	// if given msg.UpdatedAt.IsZero()
	// or performs update otherwise
	SaveMessage(ctx context.Context, msg *Message) error
	// BindMessage attach given vars to provided message unique id
	BindMessage(ctx context.Context, oid int64, vars map[string]string) error
	// GetMessage lookup for single unique historical message by provided arguments
	// as a partial search filter set
	GetMessage(ctx context.Context, oid int64, senderChatID string, targetChatID string, searchProps map[string]string) (*Message, error)
}

type Store interface {
	CatalogStore
	ChatStore
}
