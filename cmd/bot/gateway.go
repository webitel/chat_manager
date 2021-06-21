package main

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/rs/zerolog"

	gate "github.com/webitel/chat_manager/api/proto/bot"
	chat "github.com/webitel/chat_manager/api/proto/chat"
)

// Gateway service agent
type Gateway struct {
	 // identity
	 Log *zerolog.Logger
	 Profile *chat.Profile
	 // communication
	 Internal *Service // Local CHAT service client 
	 External Provider // Remote CHAT service client Receiver|Sender
	 // cache: memory
	 sync.RWMutex
	 internal map[int64]*Channel // map[internal.user.id]
	 external map[string]*Channel // map[provider.user.id]
}

// DomainID that this gateway profile belongs to
func (c *Gateway) DomainID() int64 {
	return c.Profile.GetDomainId()
}

// Register internal webhook callback handler to external service provider
func (c *Gateway) Register(ctx context.Context, force bool) error {

	if c.Profile.UrlId == "" {
		panic("register: URL required")
	}
	
	// FIXME: c.External.Webhook.Registered() bool
	//    or: c.Profile.EnabledAt == 0
	linkURL := strings.TrimRight(c.Internal.URL, "/") +
		("/" + c.Profile.UrlId)

	// region: pre-register for callback(s) within register process
	pid := c.Profile.Id
	uri := c.Profile.UrlId

	c.Internal.indexMx.Lock()      // +RW
	c.Internal.profiles[pid] = c   // register: cache entry
	c.Internal.gateways[uri] = pid // register: service URI
	c.Internal.indexMx.Unlock()    // -RW
	// endregion

	// REGISTER public webhook, callback URL
	err := c.External.Register(ctx, linkURL)

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
func (c *Gateway) Deregister(ctx context.Context) error {
	
	// linkURL := strings.TrimRight(c.Internal.URL, "/") +
	// 	("/" + c.Profile.UrlId)

	// REGISTER public webhook, callback URL
	err := c.External.Deregister(ctx) // , linkURL)

	var event *zerolog.Event

	if err == nil {

		pid := c.Profile.Id
		uri := c.Profile.UrlId

		c.Internal.indexMx.Lock()        // +RW
		e, ok := c.Internal.profiles[pid]
		if ok = ok && e == c; ok {
			delete(c.Internal.gateways, uri) // deregister: service URI
		}
		c.Internal.indexMx.Unlock()      // -RW

		event = c.Log.Warn()

	} else {

		event = c.Log.Error().Err(err)
	}

	event.Msg("DEREGISTER")

	return err
}

// Remove .this gateway runtime link
// from internal service provider agent
func (c *Gateway) Remove() bool {
	
	pid := c.Profile.Id
	uri := c.Profile.UrlId

	c.Internal.indexMx.Lock()        // +RW
	e, ok := c.Internal.profiles[pid]
	if ok = ok && e == c; ok {
		delete(c.Internal.profiles, pid) // register: cache entry
		delete(c.Internal.gateways, uri) // register: service URI
	}
	c.Internal.indexMx.Unlock()      // -RW

	var event *zerolog.Event

	if ok {

		event = c.Log.Warn()

	} else {

		event = c.Log.Error().Str("error", "gateway: profile not running")
	}

	event.Msg("REMOVE")

	return ok
}

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

		ok bool
		channel *Channel
	)

	if contact == nil {
		contact = &Account{}
	}

	if !ok && contact.ID != 0 {
	
		c.RLock()   // +R
		channel, ok = c.internal[contact.ID]
		c.RUnlock() // -R
	
	}
	
	if !ok && chatID != "" {
	
		c.RLock()   // +R
		channel, ok = c.external[chatID]
		c.RUnlock() // -R
	
	}

	if !ok && chatID != "" {

		// if contact.GetUsername() == "noname" {
		// 	panic("channel: contact required")
		// }
		title := contact.DisplayName()
		lookup := chat.CheckSessionRequest{
			// gateway profile identity
			ProfileId:  c.Profile.Id,
			// external client contact
			ExternalId: chatID,
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

			channel = &Channel{
				// RECOVER
				Title:      title,          // contact.username,
				ChatID:     chatID,         // provider.chat.id

				Account:   *(contact),      // user.contact.id
				ChannelID:  chat.ChannelId, // chat.channel.id

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

			c.Lock()   // +RW
			c.external[channel.ChatID] = channel
			c.internal[channel.Account.ID] = channel // [channel.ContactID] = channel
			c.Unlock() // -RW

			channel.Log.Info().Msg("RECOVER")
		
		} else {
			// created
			contact.ID = chat.ClientId

			// NOT FOUND !
			channel = &Channel{

				ChannelID:  "", // NEW: chat.channelId == ""
				
				Account:  *(contact),
				// ContactID:  contact.ID,  // user.contact.id
				// Username:   contact.Username,
				
				Title:      title, // contact.Username,
				ChatID:     chatID, // .provider.chat

				Gateway:    c, // .profile.id

				Log: c.Log.With().
	
					Str("chat-id", chatID).
					Str("username", contact.Username).

					// Str("session-id", c.SessionID). // UNKNOWN
					Int64("contact-id", chat.ClientId).

					Logger(),
			}
			// .IsNew() == true
			return channel, nil
		}
	}

	return channel, nil
}

// Send notification [FROM] internal: chat.server [TO] external: chat.provider
func (c *Gateway) Send(ctx context.Context, notify *gate.SendMessageRequest) error {

	profileID := notify.GetProfileId()
	if profileID != c.Profile.Id {
		panic("gateway: attempt to send to invalid profile.id")
	}

	// lookup: active channel by external chat.id
	chatID := notify.GetExternalUserId() // ExternalID
	recepient, err := c.GetChannel(ctx, chatID, nil)
	if err != nil {
		return err
	}

	sendMessage := notify.GetMessage()
	sendUpdate := Update{
		// attributes
		ID:      sendMessage.GetId(),
		Title:   recepient.Title,
		// ChatID:  chatID,
		Chat:    recepient, // TO: !
		User:    nil, // &Account{} // UNKNOWN // TODO: reg.GetUser() as a sender
		// event arguments
		//Event:   action,
		Message: sendMessage,
		// not applicable yet !
		Edited:          0,
		EditedMessageID: 0,
		// JoinMembersCount: 0,
		// KickMembersCount: 0,
		JoinMembers: nil,
		KickMembers: nil,
	}

	if recepient.Account.ID != 0 {
		// shallowcopy
		contact := recepient.Account
		sendUpdate.User = &contact
	}

	// RECV closed
	closed := sendMessage.Type == "closed" // TODO: !!!

	if sendMessage.File != nil {
		
		sendUpdate.Event = "file"

	// } else if sendMessage.Buttons != nil{

	// 	sendUpdate.Event = "menu"


	} else if sendMessage.Text != "" {

		// messageText := sendMessage.GetText()

		sendUpdate.Event = "text"
		// closed = closed || IsCommandClose(messageText) 
		
		if closed {
			// unify chat.closed reply text
			sendUpdate.Event = "closed"
			// messageText = "closed" // chat: closed
			sendMessage.Text = "closed"
		}

		if !recepient.IsNew() && closed {
			// NOTE: Closed by the webitel.chat.server !
			if recepient.Closed == 0 {
				recepient.Closed = time.Now().Unix() // SENT: COMMITTED !
			}
			defer func() {
				// Mark "closed" DO NOT SEND .CloseConversation() request !
				// recepient.Closed = time.Now().Unix() // SENT: COMMITTED !
				// REMOVE: runtime state !
				_ = recepient.Close() // (messageText)
			} ()
		}
	}

	// PERFORM: deliver TO .remote (provider) side
	err = c.External.SendNotify(ctx, &sendUpdate)

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
		
		Str("send", sendUpdate.Event).
		Str("text", sendUpdate.Message.GetText()).
		EmbedObject(ZerologJSON("file", sendUpdate.Message.GetFile())).
		
		Msg("<<<<< SEND <<<<<<")

	if err != nil {
		return err
	}

	return nil
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
func (c *Gateway) Read(ctx context.Context, notify *Update) error {

	// sender: chat/user
	channel := notify.Chat
	contact := notify.User

	if contact == nil {
		panic("channel: chat user <nil> contact")
	}

	if contact.Channel == "" {
		contact.Channel = channel.Provider()
	}

	// TODO: transform envelope due to event mime-type code name
	sendMessage := notify.Message

	// PERFORM: receive !
	err := channel.Recv(ctx, sendMessage)

	if err != nil {
		return err // NACK(!)
	}

	return nil // ACK(+)
}
