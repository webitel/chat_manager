package event_router

import (
	"strings"
	

	"context"

	"github.com/rs/zerolog"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"

	strategy "github.com/webitel/chat_manager/internal/selector"

	store "github.com/webitel/chat_manager/internal/repo/sqlx"
)


// Channel represents unique chat@workflow communication channel
// FROM: [internal:webite.chat.srv] 
// TO:   [external:webitel.chat.bot] gateway service
type channel struct {
	// model
	*store.Channel
	// // provider
	// agent *eventRouter
	trace *zerolog.Logger
}

// Hostname service node-id that successfully served
// latest request in front of this chat channel as a sender
func (c *channel) Hostname() string {
	return c.Channel.ServiceHost.String
}

// trying to locate provider's (webitel.chat.bot) node
// that successfully served latest request
// in front of .this channel sender
func (c *channel) peer() selector.SelectOption {

	balancer := selector.Random
	if serviceNodeId := c.Hostname(); serviceNodeId != "" {
		balancer = strategy.PrefferedNode(serviceNodeId)
	}
	return selector.WithStrategy(balancer)
}

func (c *channel) call(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {

		// channel request node-id
		requestNode := c.Channel.ServiceHost.String

		// doRequest
		err := next(ctx, node, req, rsp, opts)
		// 
		if err != nil {
			if requestNode != "" {
				c.trace.Warn().
					Str("chat", c.ID).
					Str("channel", c.Type).
					Str("host", requestNode).
					Msg("LOST")
			}
			c.Channel.ServiceHost.Valid = false
			c.Channel.ServiceHost.String = ""
			return err
		}

		// channel respond node-id
		respondNode := strings.TrimPrefix(node.Id, "webitel.chat.bot-")

		if requestNode == "" {
			// NEW! Hosted!
			c.Channel.ServiceHost.String = respondNode
			c.Channel.ServiceHost.Valid = true
			requestNode = respondNode
			
			// re := c.agent.WriteConversationNode(c.ID, c.Host)
			// if err = re; err != nil {
			// 	// s.log.Error().Msg(err.Error())
			// 	return err
			// }

			c.trace.Info().
				Str("chat", c.ID).
				Str("channel", c.Type).
				Str("host", requestNode).
				Msg("HOSTED")
		
		} else if respondNode != requestNode { // !strings.HasSuffix(node.Id, requestNode) {
			// Hosted! But JUST Served elsewhere ...
			c.Channel.ServiceHost.String = respondNode
			c.Channel.ServiceHost.Valid = true

			// re := c.Store.WriteConversationNode(c.ID, c.Host)
			// if err = re; err != nil {
			// 	// s.log.Error().Msg(err.Error())
			// 	return err
			// }

			c.trace.Info().
				Str("chat", c.ID).
				Str("channel", c.Type).
				Str("peer", requestNode). // CURRENT
				Str("host", respondNode). // SERVED
				Msg("RE-HOST")

			requestNode = respondNode
		}

		return err
	}
}

func (c *channel) sendOptions(opts *client.CallOptions) {
	client.WithSelectOption(c.peer())(opts)
	client.WithCallWrapper(c.call)(opts)
}