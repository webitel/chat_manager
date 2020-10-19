package auth

import (
	"context"
	"strings"

	pbauth "github.com/webitel/chat_manager/api/proto/auth"
	cache "github.com/webitel/chat_manager/internal/chat_cache"

	"github.com/micro/go-micro/v2/errors"
	"github.com/micro/go-micro/v2/metadata"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

const (
	botServiceKey  = `Micro-From-Service`
	botServiceName = `webitel.chat.bot`

	h2pDomainId      = `x-webitel-dc`
	h2pDomainName    = `x-webitel-domain`
	h2pTokenAccess   = `x-webitel-access`
	h2pAuthorization = `authorization`

	hdrDomainId      = `X-Webitel-Dc`
	hdrDomainName    = `x-Webitel-Domain`
	hdrTokenAccess   = `X-Webitel-Access`
	hdrAuthorization = `Authorization`
)

type Client interface {
	MicroAuthentication(rpc *context.Context) error
}

type client struct {
	log        *zerolog.Logger
	chatCache  cache.ChatCache
	authClient pbauth.AuthService
}

func NewClient(
	log *zerolog.Logger,
	chatCache cache.ChatCache,
	authClient pbauth.AuthService,
) Client {
	return &client{
		log,
		chatCache,
		authClient,
	}
}

func (c *client) MicroAuthentication(rpc *context.Context) error {
	// metadata binding ...
	md, _ := metadata.FromContext(*rpc)
	if len(md) == 0 {
		return errors.Unauthorized("no metadata", "")
	}
	serviceName, ok := md[botServiceKey]
	if ok && serviceName == botServiceName {
		return nil
	}
	// context authorization credentials
	_, token, err := getAuthTokenFromMetadata(md)
	if err != nil {
		return errors.Unauthorized("invalid token", "")
	}
	// provided ?
	if len(token) == 0 {
		return errors.Unauthorized("invalid token", "")
	}
	exists, err := c.chatCache.GetUserInfo(token)
	// session, err := rpc.App.GetSession(rpc.App.Context, token)
	if err != nil {
		return errors.InternalServerError("failed to get userinfo from cache", err.Error())
	}
	if exists {
		return nil
	}
	uiReq := &pbauth.UserinfoRequest{
		AccessToken: token,
	}
	ctx := metadata.Set(*rpc, h2pTokenAccess, token)
	info, err := c.authClient.UserInfo(ctx, uiReq)
	if err != nil {
		return errors.Unauthorized("failed to get userinfo from app", err.Error())
	}
	infoBytes, _ := proto.Marshal(info)
	if err := c.chatCache.SetUserInfo(token, infoBytes, info.ExpiresAt); err != nil {
		return errors.InternalServerError("failed to get userinfo to cache", err.Error())
	}
	return nil
}

// method:<type> credentials:<token>
func getAuthTokenFromMetadata(md map[string]string) (method, credentials string, err error) {
	// FROM: Go-Micro metadata ...
	// X-Access-Token:
	credentials = md[h2pTokenAccess]
	if len(credentials) == 0 {
		credentials = md[hdrTokenAccess]
	}
	if len(credentials) > 0 {
		method = hdrTokenAccess
		return // X-Webitel-Access, credentials, nil
	}
	// Authorization:
	credentials = md[h2pAuthorization]
	if len(credentials) == 0 {
		credentials = md[hdrAuthorization]
	}
	if len(credentials) > 0 {
		method, credentials, err = networkCredentials(credentials)
		if err != nil {
			return "error", err.Error(), err
		}
		return // mechanism, credentials, nil
	}
	return "anonymous", "", nil
}

// Authorization: <method> [credentials]
func networkCredentials(token string) (method, credentials string, err error) {
	credentials = strings.TrimSpace(token)
	if sp := strings.IndexByte(credentials, ' '); sp > 0 {
		method, credentials = credentials[0:sp], strings.TrimLeft(credentials[sp+1:], " ")
	}
	return
}
