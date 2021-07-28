package auth

import (
	"context"
	"strings"
	"time"

	authN "github.com/webitel/chat_manager/api/proto/auth"
	// "github.com/webitel/chat_manager/app" import cycle
)

// Authorization Credentials
type Authorization struct {
	 Service string
	 Method string
	 Token string
	 Creds *authN.Userinfo
}

func IsExpired(expiry int64, date time.Time) bool {
	if expiry <= 0 {
		return false
	}
	epoch := date.UnixNano() / int64(time.Millisecond)
	return expiry <= epoch
}

func (authZ *Authorization) HasPermission(code string) bool {

	if code == "" {
		return false
	}
	
	for _, granted := range authZ.Creds.GetPermissions() {
		if code == granted.Id {
			return true
		}
	}

	return false
}

func (authZ *Authorization) HasObjclass(name string) *authN.Objclass {

	if name == "" {
		return nil
	}
	
	for _, granted := range authZ.Creds.GetScope() {
		if name == granted.Class {
			return granted
		}
	}

	return nil
}

func (authZ *Authorization) CanAccess(scope *authN.Objclass, mode AccessMode) bool {

	if scope == nil {
		// NOTE: NOT found means that objclass
		// NOT granted by license products setup
		return false
	}

	var (
		
		super, access string
	)

	switch mode {
	case ADD, READ|ADD:
		access, super = "x", "add"
	case READ, NONE: // default
		access, super = "r", "read"
	case WRITE, READ|WRITE:
		access, super = "w", "write"
	case DELETE, READ|DELETE:
		access, super = "d", "delete"
	}

	// Check can BYPASS Security Policy(-ies) ?
	if authZ.HasPermission(super) {
		return true
	}
	// Check has requested access mode GRANTED ?
	for i := len(access)-1; i >= 0; i-- {
		mode := access[i]
		if strings.IndexByte(scope.Access, mode) < 0 {
			break // ERR: require MODE access TO scope.Class but NOT GRANTED !
		}
	}

	return false
}

// func IsExpired(exp int64, now time.Time) bool {
// 	if exp <= 0 {
// 		return false
// 	}
// 	date := app.EpochtimeDate(exp, time.Millisecond)
// 	return now.Before(date)
// }

// func (c *Authorization) IsExpired() bool {
// 	return c.Creds.GetExpiresAt() != 0 &&
// 		IsExpired(c.Creds.ExpiresAt, app.CurrentTime())
// }

type Method func(context.Context) (*Authorization, error)

var (
	
	methods []Method
)

type contextAuthZ struct{}

func NewContext(ctx context.Context, authZ *Authorization) context.Context {
	return context.WithValue(ctx, contextAuthZ{}, authZ)
}

func GetAuthorization(ctx context.Context) (authZ *Authorization, err error) {
	
	for _, authN := range methods {
		authZ, err = authN(ctx)
		if authZ != nil && err == nil {
			return // authZ, nil
		}
	}

	return nil, err
}

