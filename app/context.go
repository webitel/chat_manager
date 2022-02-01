package app

import (
	"context"
	"time"

	authN "github.com/webitel/chat_manager/auth"
)

// Context operational
type Context struct {

	Date time.Time
	Error error
	context.Context
	authN.Authorization
}

type contextKey struct{}

// GetContext returns ctx bound app *Context authorization or an error
func GetContext(ctx context.Context, ctl ...Control) (app *Context, err error) {
	
	ok := false
	app, ok = ctx.Value(contextKey{}).(*Context)
	
	if !ok || app == nil {
		app = &Context{
			Date: CurrentTime(),
		}
		// Chain current app context ...
		// GetContext(app.Context) == app
		app.Context = NewContext(ctx, app)
	}

	// Check control(s) ...
	for _, control := range ctl {
		err = control(app)
		if err != nil {
			return app, err
		}
	}

	return app, nil
}

func NewContext(ctx context.Context, app *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, app)
}

func (ctx *Context) Localtime() time.Time {

	if !ctx.Date.IsZero() {
		return ctx.Date
	}

	// Once
	ctx.Date = CurrentTime()
	return ctx.Date
}

func (ctx *Context) Epochtime(precision time.Duration) (nsec int64) {
	return DateEpochtime(ctx.Localtime(), precision)
}

func (ctx *Context) Timestamp() (nsec int64) {
	return DateTimestamp(ctx.Localtime())
}