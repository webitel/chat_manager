package gotd

import (
	"context"
	goerr "errors"
	"fmt"
	log2 "github.com/webitel/chat_manager/log"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Telegram Account session runtime
type session struct {
	App *app
	// guard
	sync sync.Mutex
	// internal
	log *zap.Logger
	*telegram.Client
	*message.Sender
	login *sessionAuth // *tg.User
	// updates/peers
	// gaps  *updates.Manager // chat.state.updates
	store *InmemoryStore
	cache *InmemoryCache
	peers *peers.Manager
	// runtime
	data []byte // cache: session data encoded
	stop func() error
}

func newSession(c *app) *session {
	return &session{App: c}
}

func (c *session) init() {

	var (
		debug   = false
		profile = c.App.Gateway.Bot.GetMetadata()
	)

	if str, _ := profile["debug"]; str != "" {
		debug, _ = strconv.ParseBool(str)
	}

	if c.log = zap.NewNop(); debug {
		c.log, _ = zap.NewDevelopment(
			zap.IncreaseLevel(zapcore.InfoLevel),
			// zap.IncreaseLevel(zapcore.DebugLevel),
			zap.AddStacktrace(zapcore.FatalLevel),
		)
	}

	var (
		handler    telegram.UpdateHandler
		dispatcher = tg.NewUpdateDispatcher()
		// sessionStore = &sessionStore{App: c}
		options = telegram.Options{
			// DC:     2,
			// DCList: dcs.Prod(),
			Logger:         c.log,
			SessionStorage: c,
			// NoUpdates:      true,      // we subscribe for updates manualy
			// ReconnectionBackoff: backoff.WithMaxRetries(),
			UpdateHandler: telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
				// Print all incoming updates.
				if debug {
					fmt.Printf("tg://app%d:%s\n→ %s\n",
						c.App.apiId, c.App.apiHash,
						formatObject(u),
					)
				}
				return handler.Handle(ctx, u)
			}),
			Middlewares: []telegram.Middleware{
				// updhook.UpdateHook(gapsManager.Handle),
				// updhook.UpdateHook(peersManager.Handle),
				// updhook.UpdateHook(handler.Handle),
				telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
					return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
						// PERFORM request !
						err := next.Invoke(ctx, input, output)
						if err != nil {
							// c.Login.MiddlewareHook(!)
							if tgerr.Is(err, "AUTH_KEY_UNREGISTERED") { // auth.IsKeyUnregistered(err) { // 401 ?
								// if auth.IsUnauthorized(err) {
								// NOTE: rpc error code 401: SESSION_PASSWORD_NEEDED
								// c.Gateway.Log.Warn().Err(err).Msg("telegram.loggedOut")
								if login := c.login; login != nil {

									login.Auth(nil)
									// login.Lock()
									// defer login.Unlock()

									// login.resetUser(nil)

									// // login.user = nil
									// // login.session = nil
									// // login.signal()
								}
							}
							return err
						}
						// HANDLE results !
						switch res := output.(type) {
						case *tg.UpdatesBox:
							// Generic Updates-like results ...
							err = handler.Handle(ctx, res.Updates)
						// https://core.telegram.org/method/messages.getDialogs
						case *tg.MessagesDialogsBox:
							// Object contains a list of chats with messages and auxiliary data.
							switch rval := res.Dialogs.(type) {
							// Full list of chats with messages and auxiliary data.
							case *tg.MessagesDialogs:
								err = c.peers.Apply(ctx, rval.GetUsers(), rval.GetChats())
							// Incomplete list of dialogs with messages and auxiliary data.
							case *tg.MessagesDialogsSlice:
								err = c.peers.Apply(ctx, rval.GetUsers(), rval.GetChats())
							// Dialogs haven't changed
							case *tg.MessagesDialogsNotModified:
							default:
							}
						// https://core.telegram.org/method/messages.getPeerDialogs
						case *tg.MessagesPeerDialogs:
							err = c.peers.Apply(ctx, res.GetUsers(), res.GetChats())
						// https://core.telegram.org/method/users.getUsers
						case *tg.UserClassVector:
							err = c.peers.Apply(ctx, res.Elems, nil)
						// https://core.telegram.org/method/messages.getChats
						// case *tg.MessagesChatsBox: // messages.getChats(id...)
						// 	err = c.peers.Apply(ctx, nil, res.Chats.GetChats())
						// https://core.telegram.org/method/updates.getDifference
						case *tg.UpdatesDifference:
							err = c.peers.Apply(ctx, res.GetUsers(), res.GetChats())
						// https://core.telegram.org/method/updates.getDifference
						case *tg.UpdatesDifferenceSlice:
							err = c.peers.Apply(ctx, res.GetUsers(), res.GetChats())
						// https://core.telegram.org/method/contacts.resolvePhone
						case *tg.ContactsResolvedPeer:
							err = c.peers.Apply(ctx, res.GetUsers(), res.GetChats())
						default: // NO resultClass reaction ...
						}
						// CACHE processing error ?
						if err != nil {
							// return errors.Wrap(err, "hook")
							c.App.Gateway.Log.Warn("MIDDLEWARE",
								slog.Any("error", err),
							)
						}
						// Operation error ?
						return nil
					}
				}),
			},
		}
	)

	if debug {
		options.Middlewares = append(
			options.Middlewares, prettyMiddleware(
				fmt.Sprintf("tg://app%d:%s", c.App.apiId, c.App.apiHash),
			),
		)
	}

	c.Client = telegram.NewClient(
		c.App.apiId, c.App.apiHash, options,
	)

	api := c.Client.API()

	if c.cache != nil {
		c.cache.Purge()
	} else {
		c.cache = &InmemoryCache{}
	}

	if c.store != nil {
		c.store.Purge()
	} else {
		c.store = &InmemoryStore{}
	}

	// c.cache = &InmemoryCache{}
	// c.store = &InmemoryStorage{}
	// if err := c.restoreHash(); err != nil {
	// 	c.Gateway.Log.Warn().Err(err).Msg("RESTORE: HASH")
	// }

	c.peers = peers.Options{
		Logger:  c.log.Named("peers"),
		Storage: c.store,
		Cache:   c.cache,
	}.Build(api)

	// c.gaps = updates.New(
	// 	updates.Config{
	// 		Logger:       c.log.Named("gaps"),
	// 		Handler:      dispatcher,
	// 		AccessHasher: c.peers,
	// 	},
	// )
	// Chain peers/gaps handlers ...
	// handler = c.peers.UpdateHook(c.gaps)
	handler = c.peers.UpdateHook(dispatcher)

	// Bind Receiver
	dispatcher.OnNewMessage(c.onNewMessage) //
	// dispatcher.OnPeerSettings(c.onPeerSettings) // newInboundUser access from here ...
	dispatcher.OnServiceNotification(c.onServiceNotification)
	// Once telegram/message.Sender state init
	c.Sender = message.NewSender(api)

	// c.onClose = []func(){
	// 	func() {
	// 		// Flush logs ...
	// 		_ = c.log.Sync()
	// 		// // Flush cached data ...
	// 		// sessionStore.StoreSession(context.TODO(), nil)
	// 		// Save logoutTokens...
	// 		c.backup()
	// 	},
	// }

	if c.login == nil {
		c.login = newSessionAuth(c)
	}

	// var err error // GET self(me) error
	// c.login, err = newSessionLogin(
	// 	c.App.apiId, c.App.apiHash, c.peers, c.log.Named("login"),
	// )
	// if err != nil {
	// 	c.App.Gateway.Log.Err(err).Msg("telegram/app.login.self")
	// }
}

// backup auth.logoutTokens
func (c *session) backup() {

	var textValue string // remove
	if login := c.login; login != nil {
		if data, _ := login.backup(); len(data) != 0 {
			textValue = binaryText.EncodeToString(data)
		}
	}
	// PERFORM:
	err := c.App.Gateway.SetMetadata(
		context.TODO(), map[string]string{
			optionSessionAuth: textValue,
		},
	)
	if err != nil {
		c.App.Gateway.Log.Warn("BACKUP: AUTH",
			slog.Any("error", err),
		)
	}
}

// restore auth.logoutTokens
func (c *session) restore() error {
	profile := c.App.Gateway.GetMetadata()
	if text, _ := profile[optionSessionAuth]; text != "" {
		data, err := binaryText.DecodeString(text)
		if err != nil {
			return err
		}
		return c.login.restore(data)
	}
	return nil
}

// connect runtime routine
func (c *session) connect() error {

	c.init()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	init := make(chan struct{})
	exit := make(chan error, 1)

	c.stop = func() error {
		c.stop = func() error {
			return nil
		}
		cancel()
		return <-exit
	}

	go func() {
		defer close(exit)
		exit <- c.Client.Run(ctx,
			func(ctx context.Context) error {
				close(init)
				// TODO: runtime
				// <-ctx.Done()
				// err = ctx.Err()
				err := c.runtime(ctx)
				if goerr.Is(err, context.Canceled) {
					err = nil
				}
				return err
			},
		)
	}()

	select {
	case <-ctx.Done(): // context canceled
		cancel()
		c.stop = func() error {
			return nil
		}
		return ctx.Err()
	case err := <-exit: // startup timeout
		c.stop = func() error {
			return nil
		}
		return err
	case <-init: // connected; init done
	}

	return nil
}

func (c *session) runtime(ctx context.Context) error {

	// c.login.restore()
	c.App.restore()
	// FIXME: To avoid users.getUsers call twice
	// we will get a sleep for a while to give a chance
	// to cache session user while subscribing for updates on startup
	// https://github.com/gotd/td/blob/7b7dc0206dbf6f5a3525fe656b92d1c282d17e66/telegram/connect.go#L26
	//
	// runtime.Gosched()
	time.Sleep(time.Second / 2)
	onAuthZstate := c.login.Subscribe()
	// // // NOTE: PUSH immediately -if- authorized
	// // // isAuthorized := len(onAuthZstate) == 1
	// // self, _ := c.peers.Self(context.TODO())
	// // c.me = self.Raw()
	// c.me = c.login.User() // sync.Load()
	// // isAuthorized := self.Raw().GetID() != 0
	defer func() {
		//
		c.login.Unsubscribe(onAuthZstate)
		// _ = c.backup()
		// Flush logs ...
		_ = c.log.Sync()
	}()

	var (
		currentUser *tg.User
		sessionUser *tg.User
	)

	for {
		// remember
		// sessionUser := c.me
		sessionUser = c.login.User()

		select {
		// onAuthorizationState changed
		case currentUser = <-onAuthZstate:
			if currentUser == nil {
				_ = c.onLoggedOut(ctx)
				continue // loop // for
			}
			// SignedIn (Authorized)
			// CHECK: whether this is session restore
			if currentUser.GetID() != sessionUser.GetID() {
				// _ = c.session.doSave(ctx, false)
				_ = c.saveSession(ctx, false)
			}
		// onRuntimeCancel
		case <-ctx.Done():
			return ctx.Err()
		}
		// Authorized; New Login (!)
		_ = c.onNewLogin(ctx, currentUser)
		// continue; listen to authorizationState changes
	}
}

func (c *session) onNewLogin(ctx context.Context, auth *tg.User) error {
	// // Notify update manager about authentication.
	// err := c.gaps.Auth(
	// 	ctx, c.App.Client.API(),
	// 	auth.ID, auth.Bot, true,
	// )

	// if err != nil {
	// 	return err
	// }

	return c.loadDialogs(ctx)
}

func (c *session) onLoggedOut(ctx context.Context) error {
	// // TODO: clear cache, peers and so on ...
	// _ = c.gaps.Logout()
	// FIXME: c.peers.me.Store(nil)
	c.cache.Purge()
	c.store.Purge()
	// c.peers.Logout() // FIXME: clear cache entities ...
	return nil
}

func (c *session) loadDialogs(ctx context.Context) error {
	// prepare request
	req := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{}, // all
		Limit:      100,                  // Let the server choose ...
	}
	var i int
next: // paging
	for i = 0; true; i++ { // NO more 7 times !..
		// c.Gateway.Log.Debug().Int("page", i+1).Msg(
		// 	"messages.getDialogs -------------------------------------",
		// )
		res, err := c.Client.API().MessagesGetDialogs(ctx, req)
		if err != nil {
			if flood, err := tgerr.FloodWait(ctx, err); err != nil {
				if flood || tgerr.Is(err, tg.ErrTimeout) {
					continue
				}
				// return block{}, errors.Wrap(err, "get next chunk")
				return err
			}
			c.App.Gateway.Log.Warn("messages.getDialogs",
				slog.Any("error", err),
			)
			break next

		} else {
			// TODO: handle pagination ...
			switch res := res.(type) {
			case *tg.MessagesDialogs:
				break next

			case *tg.MessagesDialogsSlice:

				if 0 < req.Limit && len(res.Messages) < req.Limit {
					break next // last page !
				}
				top, ok := res.MapMessages().LastAsNotEmpty()
				if !ok {
					break next
				}
				req.OffsetDate = top.GetDate()

			case *tg.MessagesDialogsNotModified:
				break next
			}
		}
	}
	// c.Gateway.Log.Debug().Int("pages", i+1).Msg(
	// 	"messages.getDialogs -------------------------------------",
	// )
	return nil
}

func (c *session) dropSession() {
	// drop encoded cache data
	profile := c.App.Gateway.GetMetadata()
	delete(profile, optionSessionData)
	// drop decoded cache data
	c.sync.Lock()
	defer c.sync.Unlock()
	if len(c.data) != 0 {
		c.data = c.data[:0]
	}
}

func (c *session) saveSession(ctx context.Context, drop bool) error {
	// Is BOT created ?
	profile := c.App.Gateway.Bot
	canWrite := profile.GetId() != 0
	if !canWrite {
		return nil // IGNORE: profile NOT created yet !
	}
	c.sync.Lock()
	data := c.data // source: cache
	c.sync.Unlock()
	reset := "" // NOTE: len(data) == 0; drop == true
	if !drop && len(data) != 0 {
		reset = binaryText.EncodeToString(data)
	}
	return c.App.Gateway.SetMetadata(
		ctx, map[string]string{
			optionSessionData: reset,
		},
	)
}

var _ telegram.SessionStorage = (*session)(nil)

func (c *session) LoadSession(ctx context.Context) (data []byte, err error) {
	// defer c.App.Gateway.Log.Trace().Msg("session.Load")
	defer func() {
		// buf := bytes.NewBuffer(
		// 	make([]byte, 0, len(data)),
		// )
		// err := json.Indent(buf, data, "", "  ")
		//log := c.App.Gateway.Log.Trace
		// if err != nil {
		// 	log = func() *zerolog.Event {
		// 		return c.App.Gateway.Log.Err(err)
		// 	}
		// 	buf.Reset()
		// 	_, _ = buf.Write(data)
		// }
		c.App.Gateway.TraceLog("session.Load")
		// fmt.Printf("%s\n", buf.String())
	}()
	// RESTORE Session configuration
	c.sync.Lock()
	data = c.data
	defer c.sync.Unlock()
	if len(data) != 0 {
		return data, nil
	}
	// if !c.canWrite() {
	// 	return nil, nil // Bot NOT created !
	// }
	profile := c.App.Gateway.Bot.GetMetadata()
	text, _ := profile[optionSessionData]
	data, err = binaryText.DecodeString(text)
	if err == nil {
		c.data = data // cache
	}
	return // data, err
}

func (c *session) StoreSession(ctx context.Context, data []byte) error {
	defer func() {
		// var (
		// 	buf = bytes.NewBuffer(
		// 		make([]byte, 0, len(data)),
		// 	)
		// 	log = c.App.Gateway.Log.Trace
		// )
		// if len(data) != 0 {
		// 	err := json.Indent(buf, data, "", "  ")
		// 	if err != nil {
		// 		log = func() *zerolog.Event {
		// 			return c.App.Gateway.Log.Err(err)
		// 		}
		// 		buf.Reset()
		// 		_, _ = buf.Write(data)
		// 	}
		// }
		// // log().Msg("session.Store")
		// log().Msg("session.Cache")
		// fmt.Printf("%s\n", buf.String())
		c.App.Gateway.TraceLog("session.Cache")
	}()
	// BACKUP Session configuration
	if data == nil {
		c.sync.Lock()
		data = c.data
		c.data = nil
		c.sync.Unlock()
		if data == nil {
			return nil // no cache data
		}
		// } else if !c.canWrite() { // && data != nil
		// 	// Bot profile NOT created
		// 	c.sync.Lock()
		// 	c.data = make([]byte, len(data))
		// 	copy(c.data, data)
		// 	c.sync.Unlock()
		// 	return nil
		// }
		// // json.Compact(data)
		// c.sync.Lock()
		// c.data = nil // clear cache
		// c.sync.Unlock()
	} else {
		c.sync.Lock() // cache: latest !
		c.data = make([]byte, len(data))
		copy(c.data, data)
		c.sync.Unlock()
	}
	// // Is BOT created ?
	// if !c.canWrite() {
	// 	return nil
	// }
	// return c.App.Gateway.SetMetadata(
	// 	ctx, map[string]string{
	// 		metadataSession: sessionEncoding.EncodeToString(data),
	// 	},
	// )
	return nil
}

// ------------ UpdatesHandler ------------

// New message in a private chat or in a basic group.
// https://core.telegram.org/constructor/updateNewMessage
func (c *session) onNewMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {

	sentMessage, ok := update.Message.(*tg.Message)
	if !ok || sentMessage.Out {
		// Outgoing message, not interesting.
		return nil
	}

	log := c.App.Gateway.Log

	log.Debug("updateNewMessage",
		slog.Any("update", log2.SlogObject(update)),
		slog.Any("entities", log2.SlogObject(e)),
	)

	// NOTE: Handle Private chats only !
	// var senderUser *tg.User // (.FromID == nil) == Self
	var fromId int64 // == userId
	// Peer ID, the chat where this message was sent
	peerId := sentMessage.GetPeerID()
	switch dialog := peerId.(type) {
	case *tg.PeerUser: // Chat partner
		fromId = dialog.GetUserID()
	// case *tg.PeerChat: // Group.
	// case *tg.PeerChannel: // Channel/supergroup
	default:
		// Not interesting.
		return nil
	}

	if fromId == 0 {
		log.Warn("IGNORE",
			slog.String("error", "not private; sender .from.userId is missing"),
		)
		return nil // IGNORE Unable to resolve sender
	}

	// ECHO
	// _, err := c.Sender.Answer(e, update).Text(ctx, sentMessage.GetMessage())
	// return err

	peer, err := c.peers.ResolveUserID(ctx, fromId)

	if err != nil {
		log.Error("telegram/updateNewMessage.peer",
			slog.Any("error", err),
			slog.Any("peer", log2.SlogObject(peerId)),
		)
		return nil // IGNORE Unable to resolve sender peer
	}

	user := peer.Raw()

	if user == nil || user.Bot || user.Self {
		// IGNORE:
		// - Saved Messages (Self)
		// - Other Bots (Bot)
		errorMsg := "message.from.<nil>"
		if user != nil {
			if user.Bot {
				errorMsg = "message.from.bot"
			} else if user.Self {
				errorMsg = "message.from.self"
			}
		}
		log.Warn("IGNORE",
			slog.String("error", errorMsg),
		)
		return nil
	}

	// region: contact
	chatId := strconv.FormatInt(fromId, 10)
	contact := &bot.Account{

		ID: 0, // LOOKUP

		Channel: "telegram",
		Contact: strconv.FormatInt(fromId, 10),

		FirstName: user.FirstName,
		LastName:  user.LastName,
		Username:  user.Username,
	}

	channel, err := c.App.Gateway.GetChannel(
		ctx, chatId, contact,
	)

	if err != nil {
		// Failed locate chat channel !
		log.Error("telegram/updateNewMessage",
			slog.Any("error", err),
		)
		// re := errors.FromError(err)
		// if re.Code == 0 {
		// 	re.Code = (int32)(http.StatusBadGateway)
		// 	// HTTP 503 Bad Gateway
		// }
		return err
		// // FIXME: Reply with 200 OK to NOT receive this message again ?!.
		// _ = telegram.WriteToHTTPResponse(
		// 	reply, telegram.NewMessage(senderChat.ID, re.Detail),
		// )
		// // reply := telegram.NewMessage(senderChat.ID, re.Detail)
		// // defer func() {
		// // 	_, _ = c.BotAPI.Send(reply)
		// // } ()
		// // // http.Error(reply, re.Detail, (int)(re.Code))
		// return // HTTP 200 OK; WITH reply error message
	}

	// TODO: messages.reedHistory(message.id)
	defer func() {
		_, re := c.Client.API().MessagesReadHistory(
			ctx, &tg.MessagesReadHistoryRequest{
				Peer:  peer.InputPeer(),
				MaxID: sentMessage.ID,
			},
		)
		if re != nil {
			log.Warn("telegram/messages.readHistory",
				slog.Any("error", re),
			)
		}
	}()

	sendUpdate := bot.Update{

		// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),

		User:  contact,
		Chat:  channel,
		Title: channel.Title,

		Message: new(chat.Message),
	}

	sendMessage := sendUpdate.Message

	mediaFile := &chat.File{}
	if media, ok := sentMessage.GetMedia(); ok {
		// switch media := media.(type) {
		// https://core.telegram.org/type/MessageMedia
		switch media := media.(type) {
		case *tg.MessageMediaEmpty: // Empty constructor.
		case *tg.MessageMediaPhoto: // Attached photo.

			// https://core.telegram.org/api/files#downloading-files
			// photo, _ := media.GetPhoto()
			photo, _ := media.Photo.AsNotEmpty()
			if photo == nil {
				// FIXME: (*tg.PhotoEmpty) ?
			}
			// const (
			// 	// 20 Mb = 1024 Kb * 1024 b
			// 	fileSizeMax = 20 * 1024 * 1024
			// )
			location := tg.InputPhotoFileLocation{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
				ThumbSize:     "",
			}
			// Message is a photo, available sizes of the photo
			// Lookup for suitable file size to download ...
			// Peek the biggest, last one ...
			// From biggest to smallest ...
		locate:
			for i := len(photo.Sizes) - 1; i >= 0; i-- {
				// omit files that are too large,
				// which will result in a download error
				// https://core.telegram.org/type/PhotoSize
				// photoSize := photo.Sizes[i]
				switch photoSize := photo.Sizes[i].(type) {
				case *tg.PhotoSizeEmpty: // Empty constructor. Image with this thumbnail is unavailable.
				case *tg.PhotoSize: // Image description.
					location.ThumbSize = photoSize.GetType()
					mediaFile.Size = int64(photoSize.Size)
					break locate
				case *tg.PhotoCachedSize: // Description of an image and its content.
				case *tg.PhotoStrippedSize: // Just the image's content
				case *tg.PhotoSizeProgressive: // Progressively encoded photosize
					location.ThumbSize = photoSize.GetType()
					break locate
				case *tg.PhotoPathSize: // Messages with animated stickers can have a compressed svg (< 300 bytes) to show the outline of the sticker before fetching the actual lottie animation.
				default:
				}
			}
			// if i < 0 {
			// 	i = 0 // restoring the previous logic: the smallest one !..
			// }
			if location.ThumbSize == "" {
				// FIXME: !!!
			}

			mediaFile, err := getFile(c.App, mediaFile, &location)
			if err != nil {
				log.Error("telegram.upload.getFile",
					slog.Any("error", err),
				)
				return nil // break
			}
			sendMessage.Type = "file"
			sendMessage.File = mediaFile
			sendMessage.Text = sentMessage.Message // caption

		case *tg.MessageMediaGeo: // Attached map.

			// FIXME: Google Maps Link to Place with provided coordinates !
			location, _ := media.Geo.AsNotEmpty()

			sendMessage.Type = "text"
			sendMessage.Text = fmt.Sprintf(
				"https://www.google.com/maps/place/%f,%f",
				location.Lat, location.Long,
			)

		case *tg.MessageMediaContact: // Attached contact.

			sendMessage.Type = "contact" // "text"
			// sendMessage.Text = contact.PhoneNumber
			sendMessage.Contact = &chat.Account{
				Id:        0, // int64(media.UserID),
				Channel:   "phone",
				Contact:   media.PhoneNumber,
				FirstName: media.FirstName,
				LastName:  media.LastName,
			}

			if media.UserID == fromId {
				sendMessage.Contact.Id = channel.Account.ID // MARK: sender:owned
			}

			contactName := strings.TrimSpace(strings.Join(
				[]string{media.FirstName, media.LastName}, " ",
			))

			if contactName != "" {
				// SIP -like AOR ...
				contactName = "<" + contactName + ">"
			}

			contactText := strings.TrimSpace(strings.Join(
				[]string{contactName, media.PhoneNumber}, " ",
			))
			// Contact: [<.FirstName[ .LastName]> ].PhoneNumber
			sendMessage.Text = contactText

		case *tg.MessageMediaUnsupported: // Current version of the client does not support this media type.
		case *tg.MessageMediaDocument: // Document (video, audio, voice, sticker, any media type except photo)

			doc, _ := media.Document.(*tg.Document)
			if doc == nil {
				log.Warn("IGNORE",
					slog.String("error", "MessageMediaDocument is not *tg.Document"),
				)
				return nil
			}
			mediaFile.Mime = doc.GetMimeType()
			mediaFile.Size = int64(doc.GetSize())
			location := tg.InputDocumentFileLocation{
				ID:            doc.ID,
				AccessHash:    doc.AccessHash,
				FileReference: doc.FileReference,
				// https://core.telegram.org/api/files#downloading-files
				// If downloading the thumbnail of a document,
				// thumb_size should be taken from the type field of the desired PhotoSize of the photo;
				// otherwise, provide an empty string.
				ThumbSize: "",
			}
			// https://core.telegram.org/type/DocumentAttribute
			for _, att := range doc.Attributes {
				switch att := att.(type) {
				case *tg.DocumentAttributeImageSize: // Defines the width and height of an image uploaded as document
				case *tg.DocumentAttributeAnimated: // Defines an animated GIF
				case *tg.DocumentAttributeSticker: // Defines a sticker

					// We cannot animate *.tgs sticker, so just forward an image
				stickerThumb:
					for i := len(doc.Thumbs) - 1; i >= 0; i-- {
						switch thumb := doc.Thumbs[i].(type) {
						case *tg.PhotoSizeEmpty: // Empty constructor. Image with this thumbnail is unavailable.
						case *tg.PhotoSize: // Image description.
							location.ThumbSize = thumb.GetType()
							mediaFile.Size = int64(thumb.Size)
							break stickerThumb
						case *tg.PhotoCachedSize: // Description of an image and its content.
						case *tg.PhotoStrippedSize: // Just the image's content
						case *tg.PhotoSizeProgressive: // Progressively encoded photosize
							location.ThumbSize = thumb.GetType()
							break stickerThumb
						case *tg.PhotoPathSize: // Messages with animated stickers can have a compressed svg (< 300 bytes) to show the outline of the sticker before fetching the actual lottie animation.
						default:
						}
					}
					// Alternative emoji as a caption
					sendMessage.Text = att.GetAlt()
					mediaFile.Mime = "image/webp"

				case *tg.DocumentAttributeVideo: // Defines a video
					// locate:
					// for i := len(doc.VideoThumbs) - 1; i >= 0; i-- {
					// 	// omit files that are too large,
					// 	// which will result in a download error
					// 	// https://core.telegram.org/type/PhotoSize
					// 	// photoSize := photo.Sizes[i]
					// 	location.ThumbSize = doc.VideoThumbs[i].GetType()
					// 	break
					// }
				case *tg.DocumentAttributeAudio: // Represents an audio file
				case *tg.DocumentAttributeFilename: // A simple document with a file name

					if mediaFile.Name == "" {
						mediaFile.Name = att.GetFileName()
					}

				case *tg.DocumentAttributeHasStickers: // Whether the current document has stickers attached
					// default:
				}
			}
			// Verify !
			if location.ThumbSize == "" {
				// https://core.telegram.org/api/files#downloading-files
				// If downloading the thumbnail of a document,
				// thumb_size should be taken from the type field of the desired PhotoSize of the photo;
				// otherwise, provide an empty string.
			}

			mediaFile, err := getFile(c.App, mediaFile, &location)
			if err != nil {
				log.Error("telegram.upload.getFile",
					slog.Any("error", err),
				)
				return nil // break
			}
			sendMessage.Type = "file"
			sendMessage.File = mediaFile

			caption := sentMessage.Message
			if sendMessage.Text == "" && caption != "" {
				sendMessage.Text = caption
			}

		case *tg.MessageMediaWebPage: // Preview of webpage
		case *tg.MessageMediaVenue: // Venue
		case *tg.MessageMediaGame: // Telegram game
		case *tg.MessageMediaInvoice: // Invoice
		case *tg.MessageMediaGeoLive: // Indicates a live geolocation
		case *tg.MessageMediaPoll: // Poll
		case *tg.MessageMediaDice: // Dice
		default: // Unknown
		}

		if sendMessage.Type == "" {
			log.Warn("telegram/updateNewMessage",
				slog.String("error", fmt.Sprintf("media.(%T) reaction not implemented", media)),
			)
			return nil // IGNORE
		}

	} else {
		// Prepare text message content
		sendMessage.Type = "text"
		sendMessage.Text = sentMessage.GetMessage()
	}

	sendMessage.Variables = map[string]string{
		chatId: strconv.Itoa(sentMessage.ID),
		// "chat_id":    chatID,
		// "message_id": strconv.Itoa(recvMessage.MessageID),
	}
	if channel.IsNew() { // && contact.Username != "" {
		sendMessage.Variables["username"] = contact.Username
	}
	// Forward message to the gateway ...
	err = c.App.Gateway.Read(ctx, &sendUpdate)

	if err != nil {
		// FIXME: send error as an answer ?
		return err
	}

	return nil
}

func (c *session) onServiceNotification(ctx context.Context, e tg.Entities, update *tg.UpdateServiceNotification) error {

	var (
		tgWarn *tgerr.Error
	)
	log := c.App.Gateway.Log.With()

	if typeOf := update.GetType(); typeOf == "" {
		log.Info("Telegram Notifications (777000):")
	} else {
		tgWarn = tgerr.New(400, typeOf)
		tgWarn.Message = update.GetMessage()
		log.Error("Telegram Notifications (777000):",
			slog.String("error", tgWarn.Type),
			slog.String("backoff", (time.Second*time.Duration(tgWarn.Argument)).String()),
		)
	}
	fmt.Printf("\n%s\n\n", update.GetMessage())
	// notice.Msg("updateServiceNotification")
	// fmt.Printf("Telegram Notifications (777000):\n\n%s\n\n", update.GetMessage())

	if tgWarn.IsType("AUTH_KEY_DROP_DUPLICATE") {
		// stop; await !
		go func() {
			_ = c.stop() // await: stop
			c.dropSession()
			_ = c.connect() // await: start
		}()
		// c.restart = func() {
		// 	todo := TODO{c}
		// 	todo.dropSession()
		// 	todo.reinit(true) // .Client.Options.SessionStore(new)
		// 	_ = c.start()
		// }
		// _ = c.stop() // await

		// // Authorization error. Use the "Log out" button to log out, then log in again with your phone number. We apologize for the inconvenience.
		// // Note: Logging out will remove your Secret Chats. Use the "Cancel" button if you'd like to save some data from your secret chats before proceeding.
		// err := c.Login.LogOut(ctx)
		// if err != nil {
		// 	notice = c.Gateway.Log.Err(err)
		// } else {
		// 	notice = c.Gateway.Log.Info()
		// }
		// notice.Bool("force", true).Msg("telegram/auth.logOut(!)")
	}

	// c.stop()
	return nil
}
