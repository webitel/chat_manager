package app

import (
	"github.com/micro/go-micro/v2/errors"

	auth "github.com/webitel/chat_manager/auth"
)

// Control Context operational state
type Control func(ctx *Context) error

// AuthorizationRequire Control
func AuthorizationRequire(methods ...auth.Method) Control {
	return func(ctx *Context) (err error) {
		
		var (

			bind auth.Method
			authZ *auth.Authorization
			authN = &ctx.Authorization
		)
		// Bind Authorization !
		for i := 0; "" == authN.Token && i < len(methods); i++ {
			
			bind = methods[i]
			authZ, err = bind(ctx.Context)
			
			if err == nil && authZ.Token != "" {
				// shallowcopy
				ctx.Authorization = *(authZ)
				break
			}
		}

		// Ensure Authorization bound !
		if authN.Token == "" {
			err = errors.Unauthorized(
				"app.context.unauthorized",
				"context: authorization required",
			)
			return // err
		}
		// Check expiry date has NOT been reached yet !
		if auth.IsExpired(authN.Creds.GetExpiresAt(), ctx.Localtime()) {
			err = errors.Unauthorized(
				"app.context.token.expired",
				"context: authorization token is expired",
			)
			return // err
		}
		// +OK
		return // nil
	}
}


func ClientRequire() Control {
	return func(ctx *Context) error {

		if ctx.Authorization.Service == "" {
			return errors.Unauthorized(
				"app.context.client.unauthorized",
				"context: client authorization required",
			)
		}

		return nil
	}
}


func DomainRequire() Control {
	return func(ctx *Context) error {

		if ctx.Authorization.Creds.GetDc() == 0 {
			return errors.Unauthorized(
				"app.context.domain.unauthorized",
				"context: domain authorization required",
			)
		}

		return nil
	}
}

func UserRequire() Control {
	return func(ctx *Context) error {

		if ctx.Authorization.Creds.GetUserId() == 0 {
			return errors.Unauthorized(
				"app.context.user.unauthorized",
				"context: user authorization required",
			)
		}

		return nil
	}
}