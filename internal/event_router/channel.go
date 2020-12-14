package event_router

import (
	
	"context"
	"strings"

	"database/sql"
	"github.com/rs/zerolog"

	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/client/selector"
	"github.com/micro/go-micro/v2/registry"

	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	
	// strategy "github.com/webitel/chat_manager/internal/selector"
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
/*func (c *channel) peer() selector.SelectOption {

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
}*/

// lookup is client.Selector.Strategy to peek preffered @workflow node,
// serving .this specific chat channel
func (c *channel) peer(services []*registry.Service) selector.Next {

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
		return selector.Random(services)
	}
	
	var peer *registry.Node
	
	lookup:
	for _, service := range services {
		for _, node := range service.Nodes {
			if strings.HasSuffix(node.Id, hostname) {
				peer = node
				break lookup
			}
		}
	}

	if peer == nil {
		
		c.trace.Warn().
			Str("peer", hostname). // WANTED
			Str("peek", "random"). // SELECT
			Str("error", "host: service unavailable").
			Msg(perform)

		return selector.Random(services)
	}

	var event *zerolog.Event
	if perform == "RECOVER" {
		event = c.trace.Info()
	} else {
		event = c.trace.Trace()
	}

	event.
		Str("host", hostname). // WANTED
		Str("addr", peer.Address). // FOUND
		Msg(perform)
	
	return func() (*registry.Node, error) {

		return peer, nil
	}
}

// call implements client.CallWrapper to keep tracking channel @workflow service node
func (c *channel) call(next client.CallFunc) client.CallFunc {
	return func(ctx context.Context, node *registry.Node, req client.Request, rsp interface{}, opts client.CallOptions) error {
		// channel request node-id
		requestNode := c.Channel.ServiceHost.String

		// PERFORM client.Call(!)
		err := next(ctx, node, req, rsp, opts)
		// 
		if err != nil {
			if requestNode != "" {
				c.trace.Warn().
					Str("peer", requestNode). // WANTED
					Str("host", node.Id). // REQUESTED
					Str("addr", node.Address).
					Msg("LOST")
			}
			c.Channel.ServiceHost = sql.NullString{}
			return err
		}

		// channel respond node-id
		respondNode := strings.TrimPrefix(node.Id, "webitel.chat.bot-")

		if requestNode == "" {
			// NEW! Hosted!
			requestNode = respondNode
			c.Channel.ServiceHost =
				sql.NullString{
					String: respondNode,
					Valid:  true,
				}

			c.trace.Info().
				Str("host", respondNode).
				Str("addr", node.Address).
				Msg("HOSTED")
		
		} else if respondNode != requestNode {
			// Hosted! But JUST Served elsewhere ...
			c.Channel.ServiceHost =
				sql.NullString{
					String: respondNode,
					Valid:  true,
				}

			// TODO: re-store DB new channel.host
			
			c.trace.Info().
				Str("peer", requestNode). // WANTED
				Str("host", respondNode). // SERVED
				Str("addr", node.Address).
				Msg("RELOCATE")

			requestNode = respondNode
		}

		return err
	}
}

// CallOption specific for this kind of channel(s)
func (c *channel) sendOptions(opts *client.CallOptions) {
	// apply .call options for .this channel ...
	client.WithSelectOption(
		selector.WithStrategy(c.peer),
	)(opts)
	client.WithCallWrapper(c.call)(opts)
}