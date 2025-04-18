package event_router

import (
	"context"
	"log/slog"
	"strings"

	"database/sql"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/util/selector"

	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	strategy "github.com/webitel/chat_manager/internal/selector"
)

// Channel represents unique chat@gateway communication channel
// FROM: [internal:webite.chat.srv]
// TO:   [external:webitel.chat.bot] gateway service
type channel struct {
	// model
	*store.Channel
	// // provider
	// agent *eventRouter
	trace *slog.Logger
}

// Hostname service node-id that successfully served
// latest request in front of this chat channel as a sender
func (c *channel) Hostname() string {
	return c.Channel.ServiceHost.String
}

// call implements client.CallWrapper to keep tracking channel @workflow service node
func (c *channel) callWrap(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, addr string, req client.Request, rsp interface{}, opts client.CallOptions) error {
		// channel request node-id
		requestNode := c.Channel.ServiceHost.String

		// PERFORM client.Call(!)
		err := next(ctx, addr, req, rsp, opts)
		//
		if err != nil {
			if requestNode != "" {
				c.trace.Warn("LOST",
					slog.String("seed", requestNode), // WANTED
					slog.String("peer", addr),        // REQUESTED
				)
			}
			c.Channel.ServiceHost = sql.NullString{}
			return err
		}

		// channel respond node-id
		respondNode := addr // strings.TrimPrefix(node.Id, "webitel.chat.bot-")

		if requestNode == "" {
			// NEW! Hosted!
			requestNode = respondNode
			c.Channel.ServiceHost =
				sql.NullString{
					String: respondNode,
					Valid:  true,
				}

			c.trace.Info("HOSTED",
				slog.String("seed", requestNode),
				slog.String("peer", addr),
			)

		} else if respondNode != requestNode {
			// Hosted! But JUST Served elsewhere ...
			c.Channel.ServiceHost =
				sql.NullString{
					String: respondNode,
					Valid:  true,
				}

			// TODO: re-store DB new channel.host
			c.trace.Info("RELOCATE",
				slog.String("seed", requestNode), // WANTED
				slog.String("peer", respondNode), // SERVED
			)

			requestNode = respondNode
		}

		return err
	}
}

// CallOption specific for this kind of channel(s)
func (c *channel) callOpts(opts *client.CallOptions) {
	// apply .call options for .this channel ...
	for _, option := range []client.CallOption{
		client.WithSelector(channelLookup{c}),
		client.WithCallWrapper(c.callWrap),
	} {
		option(opts)
	}
}

type channelLookup struct {
	*channel
}

var _ selector.Selector = channelLookup{nil}

// Select a route from the pool using the strategy
func (c channelLookup) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {

	perform := "LOOKUP"
	hostname := c.Hostname()
	// region: recover .this channel@workflow service node
	if hostname == "lookup" {
		hostname = "" // RESET
		c.Channel.ServiceHost = sql.NullString{}
	} // else if hostname != "" {

	// 	// c.Log.Debug().
	// 	// 	Str("peer", c.Host).
	// 	// 	Msg("LOOKUP")
	// }
	// endregion

	if hostname == "" {
		// START
		// return selector.Random(services)
		return strategy.RoundRobin.Select(hosts, opts...)
		// return strategy.PrefferedHost("127.0.0.1")(hosts, opts...) // webitel.chat.bot
	}

	var peer string
	for _, host := range hosts {
		// if strings.HasSuffix(host, hostname) {
		if strings.HasPrefix(host, hostname) {
			peer = host
			break
		}
	}

	if peer == "" {
		c.trace.Warn("[ CHAT::GATE ] "+perform,
			slog.String("error", "host: service unavailable"),
			slog.String("lost", hostname),     // WANTED
			slog.String("next", "roundrobin"), // SERVED
		)

		// return selector.Random(services)
		return strategy.RoundRobin.Select(hosts, opts...)
		// return strategy.PrefferedHost("127.0.0.1")(hosts, opts...) // webitel.chat.bot
	}

	if perform == "RECOVER" { // TODO is always 'false'
		c.trace.Info("[ CHAT::GATE ] "+perform,
			slog.String("host", hostname), // WANTED
			slog.String("addr", peer),     // FOUND
		)
	} // else {
	// 	c.trace.Debug("[ CHAT::GATE ] "+perform,
	// 		slog.String("host", hostname), // WANTED
	// 		slog.String("addr", peer),     // FOUND
	// 	)
	// }

	return func() string {
		return peer
	}, nil
}

// Record the error returned from a route to inform future selection
func (c channelLookup) Record(host string, err error) error {
	if err != nil {

	}
	return nil
}

// Reset the selector
func (c channelLookup) Reset() error {
	return nil
}

// String returns the name of the selector
func (c channelLookup) String() string {
	return "webitel.chat.bot"
}
