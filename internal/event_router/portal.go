package event_router

import (
	"context"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/util/selector"
	"github.com/micro/micro/v3/util/selector/roundrobin"
	chat "github.com/webitel/chat_manager/api/proto/chat"

	// sms "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	store "github.com/webitel/chat_manager/internal/repo/sqlx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// parses: [0@][name@]host:port
func contactServiceHost(contact string) (name, host string) {

	host = strings.TrimSpace(contact)
	if at := strings.LastIndexByte(host, '@'); at > 0 {
		name, host = host[0:at], host[at+1:]
	}

	if at := strings.LastIndexByte(name, '@'); 0 < at && at < len(name)-2 {
		name = name[at+1:]
	}

	return // name, host
}

func (c *eventRouter) pushUpdate(host, addr string, push *chat.Update) error {
	// // PUSH Notification Update Message
	// const defaultServiceName = "go.webitel.portal"
	// if host == "" {
	// 	host = defaultServiceName
	// }
	sp := chat.NewUpdatesService(
		host, client.DefaultClient,
	)

	_, err := sp.OnUpdate(
		// notification
		context.TODO(), push,
		// call.options..
		client.WithSelector(providerHost(addr, nil)),
	)

	if err != nil {
		c.log.Warn("Update: NACK",
			slog.Any("error", err),
			slog.String("addr", addr),
			slog.String("host", host),
			slog.String("push", push.Message.GetType()),
		)

		return err
	}

	c.log.Debug("Update: ACK",
		slog.String("addr", addr),
		slog.String("host", host),
		slog.String("push", push.Message.GetType()),
	)

	return nil
}

// outer.sendMessageToBotUser
func (c *eventRouter) portalSendUpdate(sender, target *store.Channel, message *chat.Message) error {

	// region: --- dump ---
	// buf := bytes.NewBuffer(nil)
	// defer buf.Reset()

	// enc := json.NewEncoder(buf)
	// enc.SetEscapeHTML(false)
	// enc.SetIndent("", "  ")

	// buf.WriteString("[MESSAGE]: ")
	// err := enc.Encode(message)
	// if err != nil {
	// 	return err
	// }
	// buf.WriteString("[FROM]: ")
	// err = enc.Encode(sender)
	// if err != nil {
	// 	return err
	// }
	// buf.WriteString("[PEER]: ")
	// err = enc.Encode(target)
	// if err != nil {
	// 	return err
	// }

	// log.Printf(
	// 	"[SENT:PORTAL]\n%s\n", buf.Bytes(),
	// )
	// endregion: --- dump ---

	if sender == nil {
		// This message is from schema (bot)
		// sender.id = target.ConversationID
	}

	var (
		sendDate   = time.Now()
		sendUpdate = &chat.Update{
			Date: timestamppb.New(sendDate),
			// recipient: chat.member.id
			Chat: &chat.Chat{
				Id: target.ID,
				// Peer: target.User,
			},
			// Message: &chat.Message{},
		}
		// sendMessage = sendUpdate.Message
	)

	switch message.Type {
	case "left":
		{
			// leftMember := message.LeftChatMember
		}
	case "joined":
		{
			// joinMembers := message.NewChatMembers
		}
	default:

	}
	// default: send original message
	sendUpdate.Message = message

	// return c.pushUpdate(
	// 	"", target.ServiceHost.String, sendUpdate,
	// )

	srvName, srvAddr := contactServiceHost(
		target.ServiceHost.String,
	)

	return c.pushUpdate(
		srvName, srvAddr, sendUpdate,
	)
}

// outer.SendMessageToGateway
func (c *eventRouter) portalSendMessage(sender, target *app.Channel, message *chat.Message) error {

	// region: --- dump ---
	// buf := bytes.NewBuffer(nil)
	// defer buf.Reset()

	// enc := json.NewEncoder(buf)
	// enc.SetEscapeHTML(false)
	// enc.SetIndent("", "  ")

	// buf.WriteString("[UPDATE]: ")
	// err := enc.Encode(message)
	// if err != nil {
	// 	return err
	// }
	// buf.WriteString("[FROM]: ")
	// err = enc.Encode(sender)
	// if err != nil {
	// 	return err
	// }
	// buf.WriteString("[PEER]: ")
	// err = enc.Encode(target)
	// if err != nil {
	// 	return err
	// }

	// log.Printf(
	// 	"[SENT:PORTAL]\n%s\n", buf.Bytes(),
	// )
	// endregion: --- dump ---

	if sender.ID == target.ID {

	}

	var (
		sendDate   = time.Now()
		sendUpdate = &chat.Update{
			Date: timestamppb.New(sendDate),
			// recipient: chat.member.id
			Chat: &chat.Chat{
				Id: target.ID,
				// Peer: target.User,
			},
			// Message: &chat.Message{},
		}
		// sendMessage = sendUpdate.Message
	)

	switch message.Type {
	// Service message ; Notify "closed" !
	case "closed":
		{
			// data := &chat.Message{
			// 	Id:        0,
			// 	Type:      "closed",
			// 	Text:      message.Text,
			// 	From:      message.From,
			// 	CreatedAt: message.CreatedAt,
			// }
			// sendUpdate.Message = data
			sendUpdate.Message = message
		}
	default:
		// "text", "file", "contact", etc ..
		// message.From: {
		// 	"id": 524,
		// 	"channel": "bot",
		// 	"contact": "524",
		// 	"first_name": "DEV Lite"
		// }
		{
			// [NEW] Message
			sendUpdate.Message = message
		}
	}

	// _, host := contact.ContactServiceNode(
	// 	target.Contact,
	// )

	// return c.pushUpdate(
	// 	"", host, sendUpdate,
	// )

	srvName, srvAddr := contactServiceHost(
		target.Contact,
	)

	return c.pushUpdate(
		srvName, srvAddr, sendUpdate,
	)
}

type serviceHost struct {
	preferred string
	selector.Selector
}

func providerHost(preferred string, next selector.Selector) selector.Selector {
	// if preffered == "" {
	// 	preffered = "127.0.0.1"
	// }

	if next == nil {
		next = roundrobin.NewSelector()
	}

	return &serviceHost{
		preferred: preferred,
		Selector:  next,
	}
}

var _ selector.Selector = (*serviceHost)(nil)

// Select a route from the pool using the strategy
func (c *serviceHost) Select(hosts []string, opts ...selector.SelectOption) (selector.Next, error) {
	var node string
	for _, addr := range hosts {
		if strings.HasPrefix(addr, c.preferred) {
			node = addr
			break
		}
	}

	if node == "" {
		// log.Warnf("preferred host=%s not found; peer=%s", c.preferred, c.Selector.String())
		log.Printf("[WRN] selector: %s( node: %s ); NOT Found ! next: %s", c.String(), c.preferred, c.Selector.String())
		return c.Selector.Select(hosts, opts...)
	}

	return func() string {
		// log.Infof("preferred peer=%s", node)
		log.Printf("[INF] selector: %s( node: %s ); trying.. ", c.String(), node)
		return node
	}, nil
}

// Record the error returned from a route to inform future selection
func (c *serviceHost) Record(host string, err error) error {
	log.Printf("[ERR] selector: %s( node: %s ); error: %v", c.String(), host, err)
	return nil
}

// Reset the selector
func (c *serviceHost) Reset() error {
	return nil
}

// String returns the name of the selector
func (c *serviceHost) String() string {
	return "chat.messages.provider"
}
