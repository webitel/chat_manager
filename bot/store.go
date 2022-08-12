package bot

import (
	"context"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/app"
)

// Store Bot profiles
type Store interface {
	Create(ctx *app.CreateOptions, obj *Bot) error
	Search(ctx *app.SearchOptions) ([]*Bot, error)
	Update(ctx *app.UpdateOptions, obj *Bot) error
	Delete(ctx *app.DeleteOptions) (int64, error)
	// AnalyticsActiveBotsCount returns count of all, currently enabled chat-gateways (bots)
	// NOTE: Count NOT for given pdc domain only, BUT for his customer's all domain(s)
	AnalyticsActiveBotsCount(ctx context.Context, pdc int64) (n int, err error)
}

// LocateBot fetches single result entry or returns an error
func (srv *Service) LocateBot(req *app.SearchOptions) (*Bot, error) {

	// Force
	req.Size = 1
	// Normalize FIELDS request
	fields := req.Fields
	if len(fields) == 0 {
		fields = []string{"*"}
	}
	fields = app.FieldsFunc(
		fields, app.SelectFields(
			// application: default
			[]string{
				"id", // "dc",
				"name", "uri",
				"enabled", "flow",
				"provider", "metadata",
				"updates",
				"created_at", "created_by",
				"updated_at", "updated_by",
			},
			// operational
			[]string{
				"dc",
			},
		),
	)
	// SET Normalized !
	req.Fields = fields

	// Perform
	res, err := srv.store.Search(req)

	if err != nil {
		return nil, err
	}

	var obj *Bot // result

	switch len(res) {
	case 0: // NOT FOUND !
	case 1: // FOUND !
		obj = res[0]
	default: // CONFLICT !
		return nil, errors.Conflict(
			"chat.bot.locate.conflict",
			"chatbot: too much records found; hint: please provide more specific filter condition(s)",
		)
	}

	if obj == nil {
		return nil, errors.NotFound(
			"chat.bot.locate.not_found",
			"chatbot: not found",
		)
	}

	return obj, nil
}

/*
func (srv *Service) LocateBot(ctx context.Context, oid int64, uri string) (*Bot, error) {

	var (

		res Bot
		req = bot.SelectBotRequest{
			Id:     oid,
			Uri:    uri,
			Fields: nil, // ALL
		}
	)

	err := srv.SelectBot(ctx, &req, &res)

	if err != nil {
		return nil, err
	}

	return &res, nil
}
*/
