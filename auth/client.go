package auth

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/broker"
	"github.com/micro/go-micro/v2/errors"
	"github.com/micro/go-micro/v2/metadata"

	// "github.com/webitel/chat_manager/api/proto/auth"
	"github.com/webitel/chat_manager/api/proto/auth"
	// "github.com/webitel/chat_manager/app" import cycle
	// "github.com/webitel/chat_manager/auth"
)

const (

	hdrDomainId      = `X-Webitel-Dc`
	hdrDomainName    = `x-Webitel-Domain`
	hdrAccessToken   = `X-Webitel-Access`
	hdrFromService   = `From-Service`
	hdrMicroService  = `Micro-From-Service`
	hdrAuthorization = `Authorization`

	h2pDomainId      = `x-webitel-dc`
	h2pDomainName    = `x-webitel-domain`
	h2pAccessToken   = `x-webitel-access`
	h2pFromService   = `from-service`
	h2pMicroService  = `micro-from-service`
	h2pAuthorization = `authorization`

	serviceEngine    = `engine`
	serviceChatFlow  = `workflow`
	serviceChatSrv   = `webitel.chat.server`
	serviceChatBot   = `webitel.chat.bot`

)

// Authorization: <method> [<credentials>]
func networkCredentials(token string) (method, credentials string, err error) {
	method = strings.TrimSpace(token)
	if sp := strings.IndexByte(method, ' '); sp > 0 {
		method, credentials = method[0:sp], strings.TrimLeft(method[sp+1:], " ")
	}
	return
}

// github.com/micro/go-micro/v2/metadata Authorization credentials
func microMetadataAuthZ(md metadata.Metadata) *Authorization {
	
	if len(md) == 0 {
		return nil // ErrNoAuthorization
	}

	var (

		ok bool
		authZ = new(Authorization)
	)
	
	// X-[Micro-]From-Service: <name>
	if authZ.Service, ok = md[h2pMicroService]; !ok {
		if authZ.Service, ok = md[hdrMicroService]; !ok {
			if authZ.Service, ok = md[h2pFromService]; !ok {
				authZ.Service, _ = md[hdrFromService]
			}
		}
	}
	
	// X-Webitel-Access: <token>
	if authZ.Token, ok = md[h2pAccessToken]; !ok {
		authZ.Token, ok = md[hdrAccessToken]
	}

	if ok && len(authZ.Token) != 0 {
		authZ.Method = hdrAccessToken
		return authZ
	}

	// Authorization: <method>[ <credentials>]
	if authZ.Method, ok = md[h2pAuthorization]; !ok {
		authZ.Method, ok = md[hdrAuthorization]
	}

	if ok && len(authZ.Method) != 0 {
		authZ.Method, authZ.Token, _ = networkCredentials(authZ.Method)
	}

	return authZ
}

func microContextAuthZ(ctx context.Context) (*Authorization, error) {

	// request metadata binding ...
	md, _ := metadata.FromContext(ctx)
	if len(md) == 0 {
		return nil, nil // errors.Unauthorized("no metadata", "")
	}

	authN := microMetadataAuthZ(md)
	
	switch authN.Service {
	case serviceChatFlow,
		 serviceChatBot,
		 serviceChatSrv: // NOTE: webitel.chat.bot passthru original context while searching for gateways URI

		// return nil, nil
	}

	return authN, nil
}

type Client struct {
	// Service Client
	Service   auth.AuthService
	Customers auth.CustomersService
	// Local Cache
	sync.RWMutex
	// Async Authorization events
	// [invalidate.#] Subscriber
	async broker.Subscriber
	cache *Cache // map[string]*authN.Userinfo
}

type ClientOption func(c *Client)

// ClientService option connects given srv.Client()
// to cluster's well-known Authorization service
func ClientService(srv micro.Service) ClientOption {
	return func(ctl *Client) {
		
		ctl.Service   = auth.NewAuthService("go.webitel.app", srv.Client())
		ctl.Customers = auth.NewCustomersService("go.webitel.app", srv.Client())
		
		sub, err := srv.Options().Broker.Subscribe(
			"invalidate.#", ctl.invalidateCache,
			// options ...
		)

		if err != nil {
			// LOG: failed to subscribe for authorization invalidation notifications
		}

		ctl.async = sub
	}
}

func ClientCache(store *Cache) ClientOption {
	return func(ctl *Client) {
		ctl.cache = store
	}
}

func NewClient(opts ...ClientOption) *Client {

	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Close() error {
	if sub := c.async; sub != nil {
		c.async = nil
		if err := sub.Unsubscribe(); err != nil {
			// LOG: failed to unsubscribe from authorization invalidation notifications
		}
	}
	return nil
}

func (c *Client) VerifyToken(ctx context.Context, token string) (*auth.Userinfo, error) {

	now := time.Now() // app.CurrentTime()
	// TODO: lookup internal cache !
	if cache, ok := c.cache.Get(token); ok {
		if creds, ok := cache.(*auth.Userinfo); ok {
			if !IsExpired(creds.GetExpiresAt(), now) {
				return creds, nil
			}
			c.cache.Remove(token)
		}
	}
	// c.Lock()   // +RW
	// creds, ok := c.cache[token]
	// if ok && IsExpired(creds.ExpiresAt, now) {
	// 	delete(c.cache, token)
	// 	creds = nil
	// }
	// c.Unlock() // -RW

	// Cached !
	// if creds != nil {
	// 	return creds, nil
	// }

	// Request to verify token
	creds, err := c.Service.UserInfo(
		// context
		metadata.NewContext(ctx,
			metadata.Metadata{
				h2pAccessToken: token,
			},
		),
		// request
		&auth.UserinfoRequest{
			AccessToken: token,
		},
		// options ...
	)

	if err != nil {
		return nil, err
	}

	// TODO: cache this successful verified token
	exp := c.cache.defaultExpiry
	if creds.GetExpiresAt() > 0 {
		const (
			precision = int64(time.Millisecond)
			timestamp = int64(time.Second) / precision
		)
		leftSecs := (creds.ExpiresAt - (now.UnixNano() / precision)) / timestamp
		if leftSecs <= 0 {
			return creds, errors.Unauthorized(
				"app.context.access.expired",
				"context: token authorization is expired",
			)
		}
		if leftSecs < exp {
			exp = leftSecs
		}
		c.cache.AddWithExpiresInSecs(token, creds, exp)
	}
	// c.Lock()   // +RW
	// c.cache[token] = creds
	// c.Unlock() // -RW

	return creds, nil
}

func (c *Client) GetAuthorization(ctx context.Context) (*Authorization, error) {
	
	authN, err := microContextAuthZ(ctx)

	if err != nil {
		// 400 Bad Request
	}

	if authN == nil || authN.Token == "" {
		return nil, nil // ErrNoAuthorization
	}

	dc := authN.Creds.GetDc()
	authN.Creds, err = c.VerifyToken(ctx, authN.Token)

	if err == nil && dc != 0 && authN.Creds.GetDc() != dc {
		// X-Webitel-Dc: invalid domain component spec
	}

	return authN, err
}

func (c *Client) invalidateCache(notice broker.Event) error {

	var (

		topic = notice.Topic()
		keys = strings.Split(topic, ".")
		// "invalidate" == keys[0]
		objclass = keys[1]
		objectId = keys[2]

	)

	switch objclass {
	case "customer":
		c.cache.Purge()
	
	case "session":
		for _, token := range c.cache.Keys() {
			if tokenString, ok := token.(string); ok {
				if tokenString == objectId {
					c.cache.Remove(token)
					// LOG
				}
			}
		}
	case "domain":
		oid, err := strconv.ParseInt(objectId, 10, 64)
		if err != nil {
			// LOG
			break // switch
		}
		for _, token := range c.cache.Keys() {
			if cache, ok := c.cache.Get(token); ok {
				if entry, ok := cache.(*auth.Userinfo); ok {
					if entry.GetDc() == oid {
						c.cache.Remove(token)
						// LOG
					}
				}
			}
		}
	case "user":
		oid, err := strconv.ParseInt(objectId, 10, 64)
		if err != nil {
			// LOG
			break // switch
		}
		for _, token := range c.cache.Keys() {
			if cache, ok := c.cache.Get(token); ok {
				if entry, ok := cache.(*auth.Userinfo); ok {
					if entry.GetUserId() == oid {
						c.cache.Remove(token)
						// LOG
					}
				}
			}
		}
	default:
		// LOG
	}

	return nil
}

func (c *Client) GetCustomer(ctx context.Context, token string) (*auth.Customer, error) {
	// Request to verify token
	res, err := c.Customers.GetCustomer(
		// context
		metadata.NewContext(ctx,
			metadata.Metadata{
				h2pAccessToken: token,
			},
		),
		// request
		&auth.GetCustomerRequest{
			// Id:    "",
			Valid: true,
			// Domain: &auth.ObjectId{
			// 	Id:   0,
			// 	Name: "",
			// },
			// Fields: nil,
			// Sort:   nil,
		},
		// options ...
	)

	if err != nil {
		return nil, err
	}

	return res.GetCustomer(), nil
}
