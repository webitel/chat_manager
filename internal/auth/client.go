package auth

import (
	"context"
	"log/slog"
	"strings"

	pbauth "github.com/webitel/chat_manager/api/proto/auth"

	"github.com/micro/micro/v3/service/context/metadata"
	"github.com/micro/micro/v3/service/errors"
)

const (
	hdrFromMicroService = `Micro-From-Service`
	hdrFromService      = `From-Service`

	serviceChatSrv  = `webitel.chat.server`
	serviceChatGate = `webitel.chat.bot`
	serviceChatFlow = `workflow`
	serviceEngine   = `engine`

	h2pDomainId      = `x-webitel-dc`
	h2pDomainName    = `x-webitel-domain`
	h2pTokenAccess   = `x-webitel-access`
	h2pAuthorization = `authorization`

	hdrDomainId      = `X-Webitel-Dc`
	hdrDomainName    = `x-Webitel-Domain`
	hdrTokenAccess   = `X-Webitel-Access`
	hdrAuthorization = `Authorization`
)

type User struct {
	ID       int64 `db:"id" json:"id"`
	DomainID int64 `db:"dc" json:"dc"`
}

type Client interface {
	MicroAuthentication(rpc *context.Context) (*User, error)
	GetServiceName(rpc *context.Context) string
}

type client struct {
	log *slog.Logger
	//chatCache  cache.ChatCache
	authClient pbauth.AuthService
}

func NewClient(
	log *slog.Logger,
	//chatCache cache.ChatCache,
	authClient pbauth.AuthService,
) Client {
	return &client{
		log,
		//chatCache,
		authClient,
	}
}

func (c *client) GetServiceName(rpc *context.Context) string {
	md, _ := metadata.FromContext(*rpc)
	if len(md) == 0 {
		return ""
	}
	serviceName := md[hdrFromService]
	return serviceName
}

func (c *client) MicroAuthentication(rpc *context.Context) (*User, error) {
	// request metadata binding ...
	md, _ := metadata.FromContext(*rpc)
	if len(md) == 0 {
		return nil, errors.Unauthorized("no metadata", "")
	}
	microFromService, _ := md[hdrFromMicroService]
	switch microFromService {
	case serviceChatFlow,
		serviceChatGate,
		serviceChatSrv: // NOTE: webitel.chat.bot passthru original context while searching for gateways URI

		return nil, nil
	}
	// context authorization credentials
	_, token, err := getAuthTokenFromMetadata(md)
	if err != nil {
		return nil, errors.Unauthorized("invalid token", "")
	}
	// provided ?
	if len(token) == 0 {
		return nil, errors.Unauthorized("invalid token", "")
	}
	// TO DO CACHE USER INFO
	//exists, err := c.chatCache.GetUserInfo(token)
	//if err != nil {
	//	return errors.InternalServerError("failed to get userinfo from cache", err.Error())
	//}
	//if exists {
	//	return nil
	//}
	uiReq := &pbauth.UserinfoRequest{
		AccessToken: token,
	}
	ctx := metadata.Set(*rpc, h2pTokenAccess, token)
	info, err := c.authClient.UserInfo(ctx, uiReq)
	if err != nil {
		return nil, errors.Unauthorized("failed to get userinfo from app", err.Error())
	}
	// TO DO CACHE USER INFO
	// infoBytes, _ := proto.Marshal(info)
	//if err := c.chatCache.SetUserInfo(token, infoBytes, info.ExpiresAt); err != nil {
	//	return errors.InternalServerError("failed to get userinfo to cache", err.Error())
	//}
	return &User{
		ID:       info.UserId,
		DomainID: info.Dc,
	}, nil
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
