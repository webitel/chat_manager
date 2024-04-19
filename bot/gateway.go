package bot

import (
	"context"
	errors2 "errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"

	auth "github.com/webitel/chat_manager/api/proto/auth"
	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/app"
)

const ExternalChatPropertyName = "externalChatID"

// Gateway service agent
type Gateway struct {
	// identity
	*Bot // *chat.Profile
	Log  *zerolog.Logger
	// Template of .Bot.Updates; Compiled
	Template *Template
	// communication
	Internal *Service // Local CHAT service client
	External Provider // Remote CHAT service client Receiver|Sender
	// protects the load of the GetChannel(s) queries
	loadMx *sync.Mutex
	// cache: memory
	*sync.RWMutex
	internal map[int64]*Channel  // map[internal.user.id]
	external map[string]*Channel // map[provider.user.id]
	deleted  bool                // indicate whether we need to dispose this bot gateway after last channel closed
}

// DomainID that this gateway profile belongs to
func (c *Gateway) DomainID() int64 {
	return c.Bot.GetDc().GetId()
}

// Register internal webhook callback handler to external service provider
func (c *Gateway) Register(ctx context.Context, force bool) error {

	var (
		bot = c.Bot
		pid = bot.GetId()
		uri = bot.GetUri()
		srv = c.Internal
	)

	if pid == 0 {
		panic("register: bot <zero> identifier")
	}

	if uri == "" {
		panic("register: service URL required")
	}

	rel, err := url.Parse(uri)

	if err != nil {
		panic("register: invalid relative URI specified")
	}

	// FIXME: Validate once more ?
	uri = rel.Path
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	bot.Uri = uri

	// region: pre-register for callback(s) within register process
	srv.indexMx.Lock() // +RW
	e := srv.profiles[pid]
	// Register THIS runtime URI !
	srv.profiles[pid] = c   // register: cache entry
	srv.gateways[uri] = pid // register: service URI
	srv.indexMx.Unlock()    // -RW
	// Removes LAST URI -if- changed !
	if e != nil {
		e.Lock()
		e.Enabled = c.Enabled // false
		// if !e.Enabled && len(e.external) == 0 {
		// 	// Deregister LAST route URI !
		// 	_ = e.Deregister(ctx)
		// }
		if !e.Enabled && len(e.external) == 0 {
			if e.Uri != uri {
				// URI changed; DEREGISTER previous one;
				_ = e.Deregister(ctx)
			}
		}
		e.Unlock()
	}
	// c.Log.Info().Msg("ENABLED")
	// endregion

	if force {
		// FIXME: c.External.Webhook.Registered() bool
		//    or: c.Profile.EnabledAt == 0
		linkURI := strings.TrimRight(srv.URL, "/") + uri
		// REGISTER NEW public webhook, - callback URL
		err = c.External.Register(ctx, linkURI)
	}

	var event *zerolog.Event

	if err == nil {
		event = c.Log.Info()
	} else {
		_ = c.Remove()
		event = c.Log.Error().Err(err)
	}

	event.Msg("REGISTER")

	return err
}

// Deregister internal webhook callback handler from external service provider
/*func (c *Gateway) Deregister(ctx context.Context) error {

	// linkURL := strings.TrimRight(c.Internal.URL, "/") +
	// 	("/" + c.Profile.UrlId)

	// REGISTER public webhook, callback URL
	err := c.External.Deregister(ctx) // , linkURL)

	var event *zerolog.Event

	if err == nil {

		pid := c.Bot.Id
		uri := c.Bot.Uri
		srv := c.Internal

		srv.indexMx.Lock()        // +RW
		e, ok := srv.profiles[pid]
		if ok = (ok && e == c); ok {
			delete(srv.gateways, uri) // deregister: service URI
		}
		srv.indexMx.Unlock()      // -RW

		event = c.Log.Warn()

	} else {

		event = c.Log.Error().Err(err)
	}

	event.Msg("DEREGISTER")

	return err
}*/

func (c *Gateway) Deregister(ctx context.Context) error {

	var (
		pid = c.Bot.Id
		uri = c.Bot.Uri
		srv = c.Internal
	)

	srv.indexMx.Lock() // +RW
	// e, ok := srv.profiles[pid]
	// if ok = (ok && e == c); ok {
	oid, ok := srv.gateways[uri]
	if ok = (ok && oid == pid); ok {
		delete(srv.gateways, uri) // deregister: service URI
	}
	srv.indexMx.Unlock() // -RW

	if !ok {
		// c.Log.Warn().
		// 	Str("error", "bot: out of service").
		// 	// Str("link", uri).
		// 	Msg("DEREGISTER")
		return nil
	}

	// REGISTERED public webhook, callback URL
	link := strings.TrimRight(srv.URL, "/") + uri

	var (
		event *zerolog.Event
		// PERFORM: DEREGISTER
		err = c.External.Deregister(ctx)
	)

	if err == nil {
		event = c.Log.Warn()
	} else {
		event = c.Log.Error().Err(err)
	}

	event.
		Str("link", link).
		Msg("DEREGISTER")

	return err
}

// Remove .this gateway runtime link
// from internal service provider agent
func (c *Gateway) Remove() bool {

	var (
		pid = c.Bot.Id
		uri = c.Bot.Uri
		srv = c.Internal
	)

	srv.indexMx.Lock() // +RW
	e, ok := srv.profiles[pid]
	if ok = (ok && e == c); ok {
		delete(srv.profiles, pid) // register: cache entry
		delete(srv.gateways, uri) // register: service URI
	}
	srv.indexMx.Unlock() // -RW

	if ok {
		c.Log.Warn().Msg("DELETED")
	}

	// var event *zerolog.Event

	// if ok {

	// 	event = c.Log.Warn()

	// } else {
	// 	// NOTE: There may be updated revision running
	// 	event = c.Log.Warn().Str("error", "bot: profile not running")
	// }

	// event.Msg("DISABLE")

	return ok
}

// func (c *Gateway) close(chat *Channel) bool {

// 	c.Lock()   // +RW
// 	e, ok := c.external[chat.ChatID]
// 	if ok = (ok && e == chat); (ok && chat.Closed != 0) {
// 		delete(c.internal, chat.Account.ID)
// 		delete(c.external, chat.ChatID)
// 		if len(c.external) == 0 && c.next != nil {
// 			// NOTE: Closed last active channel !
// 			srv := c.Internal

// 			srv.indexMx.Lock()   // +RW
// 			srv.profiles[c.Id] = c.next // APPLIED !
// 			srv.indexMx.Unlock() // +RW
// 		}
// 	}
// 	c.Unlock() // -RW

// 	// ok = (ok && 0 != chat.Closed)

// 	// if !ok && c.next != nil {
// 	// 	return c.next.close(chat)
// 	// }

// 	return ok
// }

// func (c *Gateway) Shutdown(force bool) error {

// 	if !force {
// 		// MARK: DO NOT accept NEW connections !
// 		c.Bot.Enabled = false
// 		// We need gracefully close all active sessions !
// 		return nil
// 	}

// 	// FORCE !
// 	for _, chat := range c.external {
// 		chat.Close()
// 	}
// }

// WebHook implements basic provider.Receiver interface
// Just delegates control to the underlaying service provider
func (c *Gateway) WebHook(reply http.ResponseWriter, notice *http.Request) {
	// Delegate process to provider ...
	c.External.WebHook(reply, notice)
	// NOTE: if provider did not manualy respond to incoming update request,
	//       next "return" statement will respond with HTTP 200 OK status result by default !
	// return
}

// GetChannel lookup for given .Profile.ID + .externalID unique channel state
// If NOT found internal cache entry, will attempt to lookup into persistent DB store
func (c *Gateway) GetChannel(ctx context.Context, chatID string, contact *Account) (*Channel, error) {

	var (
		ok      bool
		channel *Channel
	)

	if contact == nil {
		if chatID != "" {
			contact = &Account{
				Channel: c.GetProvider(),
				Contact: chatID,
			}
		} else {
			return nil, errors2.New("not enough information to form/get channel")
		}
	}

	c.loadMx.Lock()
	defer c.loadMx.Unlock()

	if contact.ID != 0 {

		c.RLock() // +R
		channel, ok = c.internal[contact.ID]
		c.RUnlock() // -R

	}

	if !ok && chatID != "" {

		c.RLock() // +R
		channel, ok = c.external[chatID]
		c.RUnlock() // -R

	}
	if !ok {

		// if contact.GetUsername() == "noname" {
		// 	panic("channel: contact required")
		// }
		title := contact.DisplayName()
		lookup := chat.CheckSessionRequest{
			// gateway profile identity
			ProfileId: c.Bot.Id,
			// external client contact
			ExternalId: contact.Contact,
			Username:   title,
		}
		// passthru request cancellation context
		chat, err := c.Internal.Client.CheckSession(ctx, &lookup)

		if err != nil {
			c.Log.Error().Err(err).Msg("Failed to lookup chat channel")
			return nil, err
		}

		if chat.Exists && chat.ChannelId != "" {
			// populate
			contact.ID = chat.ClientId
			externalChatId, found := chat.Properties[ExternalChatPropertyName]
			if found {
				chatID = externalChatId
			} else if chatID == "" {
				chatID = contact.Contact
			}
			channel = &Channel{
				// RECOVER
				Title:  title,  // contact.username,
				ChatID: chatID, // provider.chat.id

				Account:   *(contact),     // user.contact.id
				ChannelID: chat.ChannelId, // chat.channel.id

				Gateway:    c, // .profile.id
				Properties: chat.Properties,

				Log: c.Log.With().
					Str("chat-id", chatID).
					Str("username", contact.Username).

					// Str("session-id", c.SessionID). // UNKNOWN
					Str("channel-id", chat.ChannelId).
					Int64("contact-id", chat.ClientId).
					Logger(),
			}

			c.Lock() // +RW
			c.external[channel.ChatID] = channel
			c.internal[channel.Account.ID] = channel // [channel.ContactID] = channel
			c.Unlock()                               // -RW

			channel.Log.Info().Msg("RECOVER")

		} else {

			// created: client !
			contact.ID = chat.ClientId
			if chatID == "" {
				chatID = contact.Contact
			}
			// NO Channel FOUND !
			// CHECK: Can we accept NEW one ?
			if !c.Bot.GetEnabled() {
				c.Log.Warn().Msg("DISABLED")
				return nil, errors.New(
					"chat.bot.channel.disabled",
					"chat: bot is disabled",
					http.StatusBadGateway,
				)
			}

			// NOT FOUND !
			channel = &Channel{

				ChannelID: "", // NEW: chat.channelId == ""

				Account: *(contact),
				// ContactID:  contact.ID,  // user.contact.id
				// Username:   contact.Username,

				Title:  title,  // contact.Username,
				ChatID: chatID, // .provider.chat

				Gateway: c, // .profile.id

				// add
				Properties: map[string]string{ExternalChatPropertyName: chatID},

				Log: c.Log.With().
					Str("chat-id", chatID).
					Str("username", contact.Username).

					// Str("session-id", c.SessionID). // UNKNOWN
					Int64("contact-id", chat.ClientId).
					Logger(),
			}

			c.Lock() // +RW
			c.external[channel.ChatID] = channel
			c.internal[channel.Account.ID] = channel // [channel.ContactID] = channel
			c.Unlock()                               // -RW

			channel.Log.Info().Msg("PREPARE")

			// // .IsNew() == true
			// return channel, nil
		}
	}

	// Update: client.external_id ?
	if channel != nil {
		// Channel User Contact profile
		customer := &channel.Account

		newChatTitle := contact.DisplayName()
		updateChatTitle := (newChatTitle != "noname" && newChatTitle != channel.Account.DisplayName())
		// Does customer profile name changed ?
		if updateChatTitle {

			customer.FirstName = contact.FirstName
			customer.LastName = contact.LastName
			customer.Username = contact.Username

			metadata, _ := channel.Properties.(map[string]string)
			if metadata != nil {
				// External User's (Contact) Full Name
				metadata["from"] = customer.DisplayName() // newTitle
			}
			// Dialog active ?
			if !channel.IsNew() {
				var err error
				channel.Title, err = c.Template.MessageText("title", customer)
				if err != nil {
					channel.Log.Warn().Err(err).Msg("BOT.onContactUpdate")
					err = nil
				}
			}
		}
		// NEW: client.external_id
		newChatId := contact.Contact
		updateChatId := (newChatId != "" && chatID != "" && newChatId != chatID)
		// Does customer profile ID changed ?
		// This condition consider that senderId = chatId (but what if user deleted chat? then new [out side] client will be created? )
		// NOTE: whatsapp.update.messages.system.type.customer_changed_number
		if updateChatId {

			if customer.Channel != "whatsapp" {
				// panic("BOT.onContactUpdate: client.external_id changed; client.channel(" + customer.Channel + ") not supported")
				channel.Log.Warn().
					Str("error", customer.Channel+": no support; accept: whatsapp").
					// Str("chat-id", chatID). // OLD client.external_id
					// Int64("contact-id", customer.ID). // BOT.client.id
					// Str("channel", c.GetProvider()). // BOT.provider(channel-type)
					Str("new-contact", customer.Channel). // BOT.client.type
					Str("new-chat-id", newChatId).        // NEW client.external_id
					Str("new-title", channel.Title).
					Msg("BOT.onContactUpdate")
				return channel, nil
			}

			customer.Contact = newChatId
			channel.ChatID = newChatId

			metadata, _ := channel.Properties.(map[string]string)
			if metadata != nil {
				// External User's (Contact) unique IDentifier; Chat's type- specific !
				metadata["user"] = customer.Contact
			}

			c.Lock() // +RW
			// channel.ChatID = channel.Account.Contact
			if e, ok := c.external[chatID]; ok {
				if e == channel {
					delete(c.external, chatID) // DEL: OLD
				}
				c.external[channel.ChatID] = channel // ADD: NEW
			}
			// c.internal[channel.Account.ID] = channel
			c.Unlock() // -RW
		}

		if updateChatId || updateChatTitle {
			// Update channel logger info
			channel.Log = c.Log.With().
				Str("chat-id", channel.ChatID).
				Str("username", customer.DisplayName()). // Username).
				// Str("session-id", c.SessionID). // UNKNOWN
				Str("channel-id", channel.ChannelID).
				Int64("contact-id", customer.ID).
				Logger()

			// Persist store.clients changes
			contactName := customer.DisplayName()
			if contactName == "noname" {
				contactName = "" // DO NOT Update !
			}
			ok, err := c.Internal.store.UpdateContact(
				ctx, &app.User{
					ID:        customer.ID,      // resolved
					Channel:   customer.Channel, // resolved
					Contact:   customer.Contact, // resolved -or- updated
					FirstName: contactName,      // resolved -or- updated
					// LastName:  "",
					// UserName:  "",
					// Language:  "",
				},
			)

			if err == nil && !ok {
				err = fmt.Errorf("client: not found")
			}

			var log *zerolog.Event
			if err == nil {
				log = channel.Log.Info()
			} else {
				log = channel.Log.Warn().Err(err)
				err = nil // LOG -and- IGNORE
			}
			log.
				// Str("chat-id", chatID).             // NEW client.external_id
				// Int64("contact-id", customer.ID).   // BOT.client.id
				// Str("channel", c.GetProvider()).    // BOT.provider(channel-type)
				Str("contact-type", customer.Channel). // BOT.client.type
				Str("from-chat-id", chatID).           // OLD client.external_id
				Msg("BOT.onContactUpdate")
		}

	}

	return channel, nil
}

// CallbackURL returns reverse URL string
// to reach this c.Bot's webhook handler
func (c *Gateway) CallbackURL() string {

	srv := c.Internal
	botURL, err := url.ParseRequestURI(srv.HostURL())
	if err != nil {
		panic(err)
	}

	// Combine URL Path
	bot := c.Bot
	botURL.Path = path.Join(
		botURL.Path, "/", bot.GetUri(),
	)

	return botURL.String()
}

// SetMetadata merge and update profile's metadata keys on behalf of Bot request
func (c *Gateway) SetMetadata(ctx context.Context, set map[string]string) error {

	bot := c.Bot
	src := bot.GetMetadata()
	dst := make(map[string]string, len(src)+len(set))
	for key, val := range src {
		dst[key] = val // COPY !
	}
	for key, val := range set {
		if key != "" && val != "" {
			dst[key] = val // RESET !
		} else {
			delete(dst, key) // REMOVE !
		}
	}
	// if len(dst) == 0 {
	// 	dst = nil
	// }
	// SET NEW .Metadata
	bot.Metadata = dst

	if bot.GetId() == 0 {
		// NOT Created yet !
		return nil
	}

	rpc, _ := app.GetContext(ctx,
		func(ctx *app.Context) error {
			// Bot SELF Authorization
			if ctx.Authorization.Creds == nil {
				ctx.Authorization.Creds = &auth.Userinfo{
					Dc:     bot.Dc.GetId(),
					Domain: bot.Dc.GetName(),
					// Update RESETs Bot-entry's .Updated_* fields
					// So we provide the latest values to NOT track bot's self updates !
					UserId:            bot.UpdatedBy.GetId(),
					Name:              bot.UpdatedBy.GetName(),
					Username:          "",
					PreferredUsername: "",
					Extension:         "",
					Scope:             nil,
					Roles:             nil,
					License:           nil,
					Permissions:       nil,
					UpdatedAt:         0,
					ExpiresAt:         0,
				}
			}
			return nil
		},
	)

	srv := c.Internal
	err := srv.store.Update(
		&app.UpdateOptions{
			Context: *(rpc),
			Fields: []string{
				"metadata",
			},
		},
		c.Bot,
	)

	if err != nil {
		// RESET OLD .Metadata
		bot.Metadata = src
		return err
	}

	return nil
}

// Send notification [FROM] internal: chat.server [TO] external: chat.provider
func (c *Gateway) Send(ctx context.Context, notify *gate.SendMessageRequest) error {

	profileID := notify.GetProfileId()
	if profileID != c.Bot.Id {
		panic("gateway: attempt to send to invalid profile.id")
	}

	// lookup: active channel by external chat.id

	// external user id!!
	chatID := notify.GetExternalUserId() // ExternalID
	recepient, err := c.GetChannel(ctx, chatID, nil)
	if err != nil {
		return err
	}

	sendMessage := notify.GetMessage()
	sendUpdate := Update{
		// attributes
		ID: sendMessage.GetId(),
		// ChatID:  chatID,
		Chat: recepient, // TO: !
		// User:    nil, // &Account{} // UNKNOWN // TODO: reg.GetUser() as a sender
		Title: recepient.Title,
		// event arguments
		//Event:   action,
		Message: sendMessage,
		// // not applicable yet !
		// Edited:          0,
		// EditedMessageID: 0,
		// // JoinMembersCount: 0,
		// // KickMembersCount: 0,
		// JoinMembers: nil,
		// KickMembers: nil,
	}

	if recepient.Account.ID != 0 {
		// shallowcopy
		contact := recepient.Account
		sendUpdate.User = &contact
	}

	// RECV closed
	isClosed := (sendMessage.Type == "closed") // TODO: !!!

	if sendMessage.File != nil {

		// sendUpdate.Event = "file"

		// } else if sendMessage.Buttons != nil{

		// 	sendUpdate.Event = "menu"

	} else if sendMessage.Text != "" {

		// // messageText := sendMessage.GetText()

		// sendUpdate.Event = "text"
		// // closed = closed || IsCommandClose(messageText)

		if isClosed {
			// // unify chat.closed reply text
			// sendUpdate.Event = "closed"
			// // messageText = "closed" // chat: closed
			sendMessage.Text = "closed"
		}

		if !recepient.IsNew() && isClosed {
			// NOTE: Closed by the webitel.chat.server !
			if recepient.Closed == 0 {
				recepient.Closed = time.Now().Unix() // SENT: COMMITTED !
			}
			defer func() {
				// Mark "closed" DO NOT SEND .CloseConversation() request !
				// recepient.Closed = time.Now().Unix() // SENT: COMMITTED !
				// REMOVE: runtime state !
				_ = recepient.Close() // (messageText)
			}()
		}
	}

	// PERFORM: deliver TO .remote (provider) side
	// Get *Gateway controller, linked on start message !
	// This might be (gate == c) but may NOT, after .Bot UPDATE !
	// So active channel(s) must work with the corresponding *Gateway controller(s), that was started on !
	gate := recepient.Gateway
	err = gate.External.SendNotify(ctx, &sendUpdate)

	var event *zerolog.Event

	if err == nil {
		// FIXME: .GetChannel() does not provide full contact info on recover,
		//                      just it's unique identifier ...  =(
		// if recepient.Title == "" {
		// 	recepient.Title = recepient.Account.DisplayName()
		// }
		event = recepient.Log.Debug()
	} else {
		event = recepient.Log.Error().Err(err)
	}

	event.
		Str("send", sendUpdate.Message.GetType()). // sendUpdate.Event).
		Str("text", sendUpdate.Message.GetText()).
		EmbedObject(ZerologJSON("file", sendUpdate.Message.GetFile())).
		Msg("<<<<< SEND <<<<<<")

	if err != nil {
		return err
	}

	return nil
}

func (c *Gateway) DeleteMessage(ctx context.Context, update *Update) error {
	// sender: chat/user
	channel := update.Chat
	contact := update.User

	if contact == nil {
		panic("channel: chat user <nil> contact")
	}

	if contact.Channel == "" {
		contact.Channel = channel.Provider()
	}

	// if channel.IsNew() {
	// 	// MAY Delete historical message(s)
	//  // so THAT(Sender) session will not be available
	// 	// channel.IsNew() will be returned; its OK !
	// }

	// TODO: transform envelope due to event mime-type code name
	deleteMsg := update.Message
	// REQUIRE: .ID | .Variables

	// PERFORM: delete !
	err := channel.DeleteMessage(ctx, deleteMsg)

	if err != nil {
		return err // NACK(!)
	}

	return nil // ACK(+)
}

type zerologFunc func(event *zerolog.Event)

func (fn zerologFunc) MarshalZerologObject(event *zerolog.Event) {
	fn(event)
}

func ZerologJSON(key string, obj interface{}) zerolog.LogObjectMarshaler {
	return zerologFunc(func(event *zerolog.Event) {

		if obj == nil {
			event.Str(key, "")
			return
		}

		data, err := json.Marshal(obj)

		if err != nil {
			event.Err(err)
			return
		}

		event.RawJSON(key, data)

	})
}

// Read notification [FROM] external: chat.provider [TO] internal: chat.server
func (c *Gateway) Read(ctx context.Context, notify *Update) (err error) {

	// sender: chat/user
	channel := notify.Chat
	contact := notify.User

	// if c.Internal.OFF {
	// 	c.Internal.Log.Warn().Str("error", "OFF: used to drain queue of obsolete messages").Msg("SERVICE")
	// 	_ = channel.Close()
	// 	return // Drain external provider's message queues ...
	// }

	if contact == nil {
		panic("channel: chat user <nil> contact")
	}

	if contact.Channel == "" {
		contact.Channel = channel.Provider()
	}

	if channel.IsNew() {
		if channel.Title, err = c.Template.MessageText("title", contact); err != nil {
			channel.Log.Warn().Err(err).Msg("bot.updateChatTitle")
			err = nil
		}
	}

	// TODO: transform envelope due to event mime-type code name
	sendMessage := notify.Message

	// PERFORM: receive !
	err = channel.Recv(ctx, sendMessage)

	if err != nil {
		return err // NACK(!)
	}

	return nil // ACK(+)
}
