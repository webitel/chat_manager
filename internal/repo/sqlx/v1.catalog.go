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
	// Query of the chat history ; offset: backwards
	GetHistory(req *app.SearchOptions) (*api.ChatMessages, error)
	// Get contact history by contact
	GetContactChatHistory(req *app.SearchOptions) (*api.GetContactChatHistoryResponse, error)
	// Query of the chat updates ; offset: forward
	GetUpdates(req *app.SearchOptions) (*api.ChatMessages, error)
}
