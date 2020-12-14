package main

import (

	"time"
	"sync"
	"context"
	"strings"
	"net/http"

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

	// REGISTER public webhook, callback URL
	err := c.External.Register(ctx, linkURL)

	var event *zerolog.Event

	if err == nil {

		pid := c.Profile.Id
		uri := c.Profile.UrlId

		c.Internal.indexMx.Lock()      // +RW
		c.Internal.profiles[pid] = c   // register: cache entry
		c.Internal.gateways[uri] = pid // register: service URI
		c.Internal.indexMx.Unlock()    // -RW

		event = c.Log.Info()

	} else {

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

	c.External.WebHook(reply, notice)
	return

	// // receiver, _ := c.External.(interface{
	// // 	RecvNotice(http.ResponseWriter, *http.Request) (*Notice, error)
	// // })

	// receiver := c.External

	// update, err := receiver.WebHook(reply, notice)

	// if err != nil {
	// 	// http.Error(res, errors.Wrap(err, "Failed to decode message received").Error(), http.StatusInternalServerError)
	// 	return
	// }

	// channel, err := c.GetChannel(notice.Context(), c.Profile.Id, 0, update.ExternalID)
	
	// if err != nil {
	// 	// http.Error(res, errors.Wrap(err, "Failed to lookup sender channel").Error(), http.StatusInternalServerError)
	// 	return
	// }

	// if channel == nil {

	// 	channel, err = c.NewChannel(update.ExternalID, "", 0, update.Message)
	
	// } else {

	// 	err = channel.SendMessage(update.Message)
	
	// }
}


/*func (c *Gateway) NewChannel(externalID, username string, contactID int64, message *chat.Message) (*Channel, error) {

	start := &chat.StartConversationRequest{
		DomainId: c.DomainID(),
		Username: username,
		User: &chat.User{
			UserId:     contactID,
			Type:       c.Profile.Type, // "telegram", // FIXME: why (?)
			Connection: strconv.FormatInt(c.Profile.Id, 10), // contact: profile.ID
			Internal:   false,
		},
		Message: message, // start
		// Message: &chat.Message{
		// 	Type: "text",
		// 	Value: &chat.Message_Text{
		// 		Text: "/start",
		// 	},
		// 	// Variables: env,
		// },
	}
	// PERFORM: /start chatflow routine
	chat, err := c.Internal.Client.StartConversation(context.TODO(), start)
	
	if err != nil {
		c.Log.Error().Err(err).Msg("Failed to /start external chat")
		return nil, err
	}

	channel := &Channel{
		
		ChatID:     externalID,
		Title:      username,

		Username:   username,
		ContactID:  contactID,

		ChannelID:  chat.ChannelId,
		SessionID:  chat.ConversationId,

		Gateway:    c,
	}

	c.Lock()   // +RW
	c.external[channel.ChatID] = channel
	c.internal[channel.ContactID] = channel
	c.Unlock() // -RW

	return channel, nil
}*/

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

		lookup := chat.CheckSessionRequest{
			// gateway profile identity
			ProfileId:  c.Profile.Id,
			// external client contact
			ExternalId: chatID,
			Username:   contact.Username,
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
				ChannelID:  chat.ChannelId, // chat.channel.id
				
				ContactID:  contact.ID,  // user.contact.id
				Username:   contact.Username,
				
				Title:      contact.Username,
				ChatID:     chatID, // .provider.chat

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
			c.internal[channel.ContactID] = channel
			c.external[channel.ChatID] = channel
			c.Unlock() // -RW

			channel.Log.Info().Msg("RECOVER")
		
		} else {
			// created
			contact.ID = chat.ClientId

			// NOT FOUND !
			channel = &Channel{

				ChannelID:  "", // NEW: chat.channelId == ""
				
				ContactID:  contact.ID,  // user.contact.id
				Username:   contact.Username,
				
				Title:      contact.Username,
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
			
			return channel, nil // .IsNew() == true
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
	messageText := sendMessage.GetText()

	action := "text"
	closed := IsCommandClose(messageText) 
	
	if closed {
		// unify chat.closed reply text
		action = "closed"
		messageText = action // chat: closed
		sendMessage.Value.(*chat.Message_Text).Text = messageText
	}

	sendUpdate := Update{
		// attributes
		ID:      sendMessage.GetId(),
		Title:   recepient.Title,
		// ChatID:  chatID,
		Chat:    recepient, // TO: !
		User:    nil, // &Account{} // UNKNOWN // TODO: reg.GetUser() as a sender
		// event arguments
		Event:   action,
		Message: sendMessage,
		// not applicable yet !
		Edited:          0,
		EditedMessageID: 0,
		// JoinMembersCount: 0,
		// KickMembersCount: 0,
		JoinMembers: nil,
		KickMembers: nil,
	}

	if !recepient.IsNew() && closed {
		// NOTE: Closed by the webitel.chat.server !
		defer func() {
			// Mark "closed" DO NOT SEND .CloseConversation() request !
			recepient.Closed = time.Now().Unix() // SENT: COMMITTED !
			// REMOVE: runtime state !
			_ = recepient.Close() // (messageText)
		} ()
	}

	// PERFORM: deliver TO .remote (provider) side
	err = c.External.SendNotify(ctx, &sendUpdate)

	var event *zerolog.Event
	
	if err == nil {
		event = recepient.Log.Debug()
	} else {
		event = recepient.Log.Error().Err(err)
	}

	event.Str("text", messageText).Msg("<<<<< SEND <<<<<<")

	if err != nil {
		return err
	}

	return nil
}

// Read notification [FROM] external: chat.provider [TO] internal: chat.server
func (c *Gateway) Read(ctx context.Context, notify *Update) error {

	sender := notify.Chat
	contact := notify.User

	if contact.Channel == "" {
		contact.Channel = sender.Provider()
	}

	if contact == nil {
		panic("sender: user <nil>")
	}

	channel := notify.Chat

	// TODO: transform envelope due to event mime-type code name
	sendMessage := notify.Message

	// PERFORM: receive !
	err := channel.Recv(ctx, sendMessage)

	if err != nil {
		return err // NACK(!)
	}

	return nil // ACK(+)
}

/*func (c *Gateway) RecvMessage(notice *chat.SendMessageRequest) (*Channel, error) {

	message := &pbchat.SendMessageRequest{
		// Message:   textMessage,
		AuthUserId: chat.ClientId,
		ChannelId:  chat.ChannelId,
	}
	messageText := &pbchat.Message{
		Type: "text",
		Value: &pbchat.Message_Text{
			Text: update.Text,
		},
		// // FIXME: does we need this here ? 
		// // NOTE: processing consequent message(s) ...
		// Variables: map[string]string {
		// 	"action":  update.Type,
		// 	"channel": update.Channel,
		// 	"replyTo": update.ReplyWith,
		// },
	}
	message.Message = messageText
	// }

	_, err := bot.client.SendMessage(context.Background(), message)
	if err != nil {
		bot.log.Error().Msg(err.Error())
	}
}*/