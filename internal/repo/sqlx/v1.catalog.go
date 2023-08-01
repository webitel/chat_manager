package sqlxrepo

import (
	"github.com/webitel/chat_manager/app"

	api "github.com/webitel/chat_manager/api/proto/chat/messages"
)

type CatalogStore interface {
	// Query of external chat customers
	GetCustomers(req *app.SearchOptions, res *api.ChatCustomers) error
	// Query of chat conversations
	GetDialogs(req *app.SearchOptions, res *api.ChatDialogs) error
	// Query of chat participants
	GetMembers(req *app.SearchOptions) (*api.ChatMembers, error)
	// Query of chat messages history
	GetHistory(req *app.SearchOptions) (*api.ChatMessages, error)
}
