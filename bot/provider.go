package bot

import (
	"context"
	"net/http"
	// chat "github.com/webitel/chat_manager/api/proto/chat"
	// store "github.com/webitel/chat_manager/internal/repo/sqlxrepo"
)

// Sender interface
type Sender interface {
	
	// channel := notify.Chat
	// contact := notify.User
	SendNotify(ctx context.Context, notify *Update) error
}

// Receiver as a http.Handler
type Receiver interface {
	// WebHook callback http.Handler
	// 
	// // bot := BotProvider(agent *Gateway)
	// ...
	// recv := Update{/* decode from notice.Body */}
	// err = c.Gateway.Read(notice.Context(), recv)
	//
	// if err != nil {
	// 	http.Error(res, "Failed to deliver .Update notification", http.StatusBadGateway)
	// 	return // 502 Bad Gateway
	// }
	//
	// reply.WriteHeader(http.StatusOK)
	// 
	WebHook(reply http.ResponseWriter, notice *http.Request)
	// Register webhook callback URI
	Register(ctx context.Context, uri string) error
	// Deregister webhook callback URI
	Deregister(ctx context.Context) error
}


type Provider interface {
	 // String provider's code name
	 String() string
	 Sender
	 Receiver
	 // Close shuts down bot and all it's running session(s)
	 Close() error
}

// NewProvider factory method
// Gateway agent represents .Bot profile settings, linked to .Internal CHAT *Service provider.
// Optional, (state != nil) specifies running Provider state, need to be updated with NEW agent revision.
type NewProvider func(agent *Gateway, state Provider) (Provider, error)
// type NewProvider func(agent *Gateway) (Provider, error)

// Well-known providers registry
var providers = make(map[string]NewProvider)

// Register new provider name factory method
// to be able to connect external chat bot profiles
func Register(provider string, factory NewProvider) {

	if _, ok := providers[provider]; ok {
		panic("chat: duplicate "+ provider +" provider register")
	}

	providers[provider] = factory
}

// GetProvider returns factory method (constructor)
// corresponding to given provider's code name
// or nil if not yet registered
func GetProvider(name string) NewProvider {
	return providers[name]
}