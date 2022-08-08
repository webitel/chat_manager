package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/bot"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	providerType    = "gotd"
	metadataApiId   = "api_id"
	metadataApiHash = "api_hash"
	metadataPhone   = "phone" // international format: +(country code)(city or carrier code)(your number)
)

type App struct {
	*bot.Gateway        // Messages gateway profile
	apiId        int    // Telegram API-ID
	apiHash      string // Telegram API-Hash
	appIdent     []byte // MD5(apiId+apiHash)
	phone        string // currently not used
	// runtime
	exit    chan chan error
	sync    *sync.RWMutex // guard
	started bool          // marks the serve as running (connected)
	//
	me  *tg.User // Current Telegram Account Authorization
	log *zap.Logger
	*telegram.Client
	*message.Sender
	// Auth auth.FlowClient
	Login *sessionLogin
	// peer.Entities
	gaps  *updates.Manager
	store *InmemoryStorage
	cache *InmemoryCache
	peers *peers.Manager
	// auth *auth.Client
	runtime context.Context
	cancel  context.CancelFunc
	onClose []func()
}

var _ bot.Provider = (*App)(nil)

// calc hash sum of the api(Id+Hash) identification
func appHash(apiId int, apiHash string) []byte {
	hash := md5.New()
	hash.Write([]byte(apiHash))
	enc := binary.BigEndian
	var bin [8]byte
	enc.PutUint64(bin[:], uint64(apiId))
	hash.Write(bin[:])
	return hash.Sum(nil)
}

// New telegram client(app) provider on behalf of `agent`.Bot profile configuration
func New(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {

	var (
		err      error
		apiId    int
		apiHash  string
		config   = agent.Bot
		metadata = config.GetMetadata()
	)

	if s, _ := metadata[metadataApiId]; s != "" {
		apiId, err = strconv.Atoi(s)
		if err != nil {
			apiId = 0
		}
	}

	if apiId == 0 {
		return nil, errors.BadRequest(
			"chat.bot.telegram.api_id.invalid",
			"telegram: api_id is invalid or missing",
		)
	}

	apiHash, _ = metadata[metadataApiHash]
	if apiHash == "" {
		return nil, errors.BadRequest(
			"chat.bot.telegram.api_hash.required",
			"telegram: api_hash required but missing",
		)
	}

	// If API IDentification(apiId+apiHash) didn't change
	// return the latest `state` as a current new one !
	// NOTE: this will not suspend or restart runtime routines
	var (
		latest, _ = state.(*App)
		appIdent  = appHash(apiId, apiHash)
	)
	// update ?
	if latest != nil {
		if bytes.Equal(appIdent, latest.appIdent) {
			latest.Gateway = agent // upgrade !
			return latest, nil
		}
		// NOTE: (apiId|apiHash) changed !
		// TODO: stop latest; start newone !
	}

	// Optional. For quick registration
	phone, _ := metadata[metadataPhone]

	app := &App{

		apiId:    apiId,
		apiHash:  apiHash,
		appIdent: appIdent,

		phone: phone,

		Gateway: agent,

		exit: make(chan chan error),
		sync: new(sync.RWMutex),
	}
	// initialize
	debug, _ := strconv.ParseBool(metadata["debug"])
	app.init(debug)

	// background connect ...
	// ignore authorization error
	_ = app.start()
	return app, nil
}

// String provider's code name
func (c *App) String() string {
	return providerType
}

// channel := notify.Chat
// contact := notify.User
func (c *App) SendNotify(ctx context.Context, notify *bot.Update) error {
	// panic("not implemented") // TODO: Implement
	var (
		peerChannel = notify.Chat // recepient
		// localtime = time.Now()
		sentMessage = notify.Message

		binding map[string]string
	)

	// region: recover latest chat channel state
	chatID, err := strconv.ParseInt(peerChannel.ChatID, 10, 64)
	if err != nil {
		c.Log.Error().Str("error", "invalid chat "+peerChannel.ChatID+" integer identifier").Msg("TELEGRAM: SEND")
		return errors.InternalServerError(
			"chat.gateway.telegram.chat.id.invalid",
			"telegram: invalid chat %s unique identifier; expect integer values", peerChannel.ChatID)
	}

	if peerChannel.Title == "" {
		// FIXME: .GetChannel() does not provide full contact info on recover,
		//                      just it's unique identifier ...  =(
	}

	// // TESTS
	// props, _ := channel.Properties.(map[string]string)
	// endregion

	// bind := func(key, value string) {
	// 	if binding == nil {
	// 		binding = make(map[string]string)
	// 	}
	// 	binding[key] = value
	// }

	// sender := message.NewSender(c.Client.API())
	// // sender = sender.WithResolver(c.peers)
	// // sender.Resolve()
	// peerUser, err := c.Entities.ExtractPeer(&tg.PeerUser{UserID: chatID})
	// sendMessage := sender.To(peerUser)

	sender := c.Sender
	// FIXME: Need optimization; Always get full user profile
	//        but we need to resolve just access_hash by id
	peerUser, err := c.peers.ResolveUserID(ctx, chatID)
	if err != nil {
		return err // Failed to resolve peer user.access_hash
	}
	sendMessage := sender.To(peerUser.InputPeer())

	// var (
	// 	chatAction  string
	// 	sentMessage telegram.Message
	// 	sendUpdate  telegram.Chattable
	// )
	// TODO: resolution for various notify content !
	switch sentMessage.Type { // notify.Event {
	case "text": // default
		_, err = sendMessage.Text(ctx, sentMessage.Text)
		if err != nil {
			c.Gateway.Log.Err(err).Msg("telegram/messages.sendMessage")
			return err
		}
	case "file":
		mediaFile := sentMessage.GetFile()
		mediaType, _, _ := mime.ParseMediaType(mediaFile.Mime)
		if mediaType == "" {
			mediaType = strings.ToLower(strings.TrimSpace(mediaFile.Mime))
		}
		var caption []styling.StyledTextOption
		if text := sentMessage.Text; text != "" {
			caption = append(caption,
				styling.Plain(text),
			)
		}
		uploadFile := sendMessage.Upload(
			message.FromURL(mediaFile.Url),
		)
		switch mediaType {
		case "image/gif":
			_, err = uploadFile.GIF(ctx, caption...)
		default:
			if sub := strings.IndexByte(mediaType, '/'); sub > 0 {
				mediaType = mediaType[0:sub]
			}
			switch mediaType {
			case "image":
				_, err = uploadFile.Photo(ctx, caption...)
				// _, err = sendMessage.PhotoExternal(
				// 	ctx, mediaFile.Url,
				// 	// Caption
				// 	styling.Plain(sentMessage.Text),
				// )
			case "audio":
				_, err = uploadFile.Audio(ctx, caption...)
				// _, err = sendMessage.Upload(
				// 	message.FromURL(mediaFile.Url),
				// ).Audio(
				// 	ctx, styling.Plain(sentMessage.Text),
				// )
			case "video":
				_, err = uploadFile.Video(ctx, caption...)
				// _, err = sendMessage.Upload(
				// 	message.FromURL(mediaFile.Url),
				// ).Video(
				// 	ctx, styling.Plain(sentMessage.Text),
				// )
			default:
				_, err = uploadFile.File(ctx, caption...)
			}
		}

		if err != nil {
			c.Gateway.Log.Err(err).Msg("telegram/messages.sendMedia")
			return err
		}

	// // case "edit":
	// // case "send":

	// // case "read":
	// // case "seen":

	// // case "kicked":
	// case "left": // ACK: ChatService.LeaveConversation()
	// case "joined":
	// case "closed":
	default:
		c.Gateway.Log.Warn().Str("error", "message.type("+sentMessage.Type+") reaction not implemented").Msg("telegram/messages.sendMessage")
		return nil // IGNORE
	}

	// updates, err := c.Client.API().MessagesSendMessage(
	// 	ctx, &tg.MessagesSendMessageRequest{},
	// )

	// // TARGET[chat_id]: MESSAGE[message_id]
	// bind(peerChannel.ChatID, strconv.Itoa(sentMessage.MessageID))
	// // sentBindings := map[string]string {
	// // 	"chat_id":    channel.ChatID,
	// // 	"message_id": strconv.Itoa(sentMessage.MessageID),
	// // }
	// attach sent message external bindings
	if sentMessage.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		sentMessage.Variables = binding
	}
	// +OK
	return nil
}

// Simplified *tg.User account info
type accountJSON struct {
	// ID of the user
	ID int64
	// First name
	//
	// Use SetFirstName and GetFirstName helpers.
	FirstName string
	// Last name
	//
	// Use SetLastName and GetLastName helpers.
	LastName string
	// Username
	//
	// Use SetUsername and GetUsername helpers.
	Username string
	// Phone number
	//
	// Use SetPhone and GetPhone helpers.
	Phone string
}

// WebHook API http.Handler
// Used for telegram-app authorization
func (c *App) WebHook(rsp http.ResponseWriter, req *http.Request) {

	// Bind HTTP request
	ctx := req.Context()
	switch req.Method {
	case http.MethodGet:
		// / ~ /${uri}
		// GET / - Authorization state request
		// 302 Redirect /?auth=phone
		// 302 Redirect /?auth=code
		// 302 Redirect /?auth=2fa

		query := req.URL.Query()
		if _, is := query["auth"]; is {

			srv := c.Gateway.Internal
			http.ServeFile(rsp, req, filepath.Join(srv.WebRoot, "login.html"))
			return // 200

			// switch strings.ToLower(query.Get("auth")) {
			// // case "cancel":
			// case "phone", "": // wait for mobile phone number form
			// 	// // TODO: scan &redirect_uri=
			// 	// redirectURI = query.Get("redirect_uri")
			// 	// // TODO: Cancel current state
			// 	srv := c.Gateway.Internal
			// 	http.ServeFile(rsp, req, filepath.Join(srv.WebRoot, "login.html"))
			// 	// http.ServeFile(rsp, req, "/home/srgdemon/develab/webitel.go/chat/v3/bot/telegram/client/forms/phone.html")
			// 	return // 200
			// case "code": // wait for authentication code form
			// 	http.ServeFile(rsp, req, "/home/srgdemon/develab/webitel.go/chat/v3/bot/telegram/client/forms/code.html")
			// 	return // 200
			// case "2fa": // wait for 2FA password form
			// 	http.ServeFile(rsp, req, "/home/srgdemon/develab/webitel.go/chat/v3/bot/telegram/client/forms/2fa.html")
			// 	return // 200
			// }
		}

		// status
		status := struct {
			Enabled    bool `json:"enabled"`
			Connected  bool `json:"connected"`
			Authorized bool `json:"authorized"`
			// Account    *tg.User `json:"account,omitempty"`
			Account *accountJSON `json:"account,omitempty"`
			Error   error        `json:"error,omitempty"`
		}{
			Enabled: c.Gateway.Bot.Enabled,
		}
		c.sync.RLock() // LOCKED +R
		status.Connected = c.started
		c.sync.RUnlock() // UNLOCK -R
		if me := c.me; status.Connected && me.GetID() != 0 {
			status.Authorized = true
			status.Account = &accountJSON{
				ID:        me.ID,
				FirstName: me.FirstName,
				LastName:  me.LastName,
				Username:  me.Username,
				Phone:     me.Phone,
			}
		}

		header := rsp.Header()
		header.Set("Pragma", "no-cache")
		header.Set("Cache-Control", "no-cache")
		header.Set("Content-Type", "application/json; charset=utf-8")
		rsp.WriteHeader(http.StatusOK)

		codec := json.NewEncoder(rsp)
		codec.SetEscapeHTML(false)
		codec.SetIndent("", "  ")

		_ = codec.Encode(status)
		return // (200) OK

	case http.MethodPost:
		// Request URL ?query=
		// query := req.URL.Query()
		// POST /${uri}?logout=
		// POST /${uri}?cancel=
		// POST /${uri}?phone=+10005553311
		// POST /${uri}?code=123456
		// POST /${uri}?2fa=my_secret_cloud_password
		var (
			_    = req.ParseForm()
			form = req.Form
		)

		if _, is := form["logout"]; is {
			// TODO: logout
			if yes, _ := strconv.ParseBool(form.Get("confirmed")); !yes {
				// FIXME: Require action confirmation ?
			}

			err := c.Login.LogOut(ctx)
			if err != nil {
				writeTgError(rsp, err, 400)
				return // 400
			}
			writeJSON(rsp, "OK", http.StatusOK)
			return // 200

		} else if _, is = form["cancel"]; is {

			err := c.Login.CancelCode(ctx)
			if err != nil {
				writeTgError(rsp, err, 400)
				return // 400
			}
			writeJSON(rsp, "OK", http.StatusOK)
			return // 200

		} else if phone := form.Get("phone"); phone != "" {
			// Stage Authorization handler

			// sentCode := &tg.AuthSentCode{
			// 	Flags: 0,
			// 	Type: &tg.AuthSentCodeTypeApp{
			// 		Length: 5,
			// 	},
			// 	PhoneCodeHash: "e7b435bv293k3h",
			// 	NextType:      nil,
			// 	Timeout:       0,
			// }
			// err := &tgerr.Error{
			// 	Code:    406,
			// 	Type:    "PHONE_NUMBER_INVALID",
			// 	Message: "The phone number is invalid",
			// }
			sentCode, err := c.Login.SendCode(ctx, phone)
			if err != nil {
				writeTgError(rsp, err, http.StatusBadRequest)
				return // 400
			}
			writeJSON(rsp, sentCode, 200)
			return // 200

		} else if code := form.Get("code"); code != "" {

			// err = auth.ErrPasswordAuthNeeded
			// err = &tgerr.Error{
			// 	Code:    406,
			// 	Type:    "SESSION_PASSWORD_NEEDED",
			// 	Message: "2FA password required",
			// }
			authZ, err := c.Login.SignIn(ctx, code)
			if err != nil {
				writeTgError(rsp, err, 400)
				return // 400
			}
			writeJSON(rsp, authZ, http.StatusOK)
			return // 200

		} else if pass := form.Get("2fa"); pass != "" {

			// err = &tgerr.Error{
			// 	Code:    406,
			// 	Type:    "PASSWORD_HASH_INVALID",
			// 	Message: "PASSWORD_HASH_INVALID",
			// }
			authZ, err := c.Login.Password(ctx, pass)
			if err != nil {
				writeTgError(rsp, err, http.StatusBadRequest)
				return // 400
			}
			writeJSON(rsp, authZ, http.StatusOK)
			return // 200

		} else {
			writeError(rsp, errors.BadRequest("", "Unknown action request"), 400)
			return // 400
		}

	default:
		writeError(rsp, errors.MethodNotAllowed(
			"METHOD_NOT_ALLOWED",
			"(405) Method not allowed",
		), 405)
		return // 405
	}

	writeError(rsp, errors.BadRequest(
		"BAD_REQUEST",
		"Request action unknown",
	), 400)
	return // 400
}

// Register webhook callback URI
func (c *App) Register(ctx context.Context, uri string) error {
	// TODO: check authentication state
	// TODO: go background runtime routine
	return c.start()
}

// Deregister webhook callback URI
func (c *App) Deregister(ctx context.Context) error {
	// FIXME: stop listening ?
	// return c.stop()
	return nil
}

// Close shuts down bot and all it's running session(s)
func (c *App) Close() error {
	return c.stop()
}

func init() {
	// Register messages `gotd` provider
	bot.Register(providerType, New)
}

// stop background connection routine
func (c *App) stop() (err error) {
	c.sync.RLock()
	if !c.started {
		c.sync.RUnlock()
		return nil
	}
	c.sync.RUnlock()

	rpc := make(chan error)

	// c.exit <- rpc // request
	// err := <-rpc  // response

	select {
	case c.exit <- rpc: // request
		err = <-rpc // response
	default:
	}

	c.cancel()
	// c.sync.Lock()
	// c.started = false
	// c.sync.Unlock()

	return err

	// cancel := c.cancel
	// if cancel != nil {
	// 	cancel()
	// }
}

// start background connection routine
func (c *App) start() error {

	// if !c.Gateway.Bot.GetEnabled() {
	// 	return errors.BadRequest(
	// 		"chat.bot.profile.disabled",
	// 		"bot: profile disabled",
	// 	)
	// }

	c.sync.RLock() // LOCKED: +R
	if c.started {
		c.sync.RUnlock()
		return nil
	}
	c.sync.RUnlock() // UNLOCK: -R

	ctx := c.runtime
	if ctx != nil {
		select {
		case <-ctx.Done():
			// return errors.Wrap(c.ctx.Err(), "client already closed")
		default:
			return nil // already running
		}
	}

	ctx = context.Background()
	c.runtime, c.cancel = context.WithCancel(ctx)
	start := make(chan error, 1)
	go c.run(c.runtime, start)

	// mark the server as started
	c.sync.Lock() // LOCKED +RW
	c.started = true
	c.sync.Unlock() // UNLOCK -RW
	// wait for connection state...
	return <-start
}

const defaultPartSize = 512 * 1024 // 512 kb

// getFile pumps source (external, telegram) file location into target (internal, storage) media file
func (c *App) getFile(media *chat.File, location tg.InputFileLocationClass) (*chat.File, error) {

	var (
		// GET
		tgapi   = c.Client.API()
		getFile = tg.UploadGetFileRequest{
			Precise:      false,
			CDNSupported: false,
			Location:     location,
			Offset:       0,
			Limit:        defaultPartSize,
		}
		// SET
		stream storage.FileService_UploadFileService
		mpart  = storage.UploadFileRequest_Chunk{
			Chunk: nil,
		}
		push = storage.UploadFileRequest{
			Data: &mpart,
		}
		// CTX
		ctx  = context.Background()
		data []byte // content part
	)

loop:
	for {
		// READ
		part, err := tgapi.UploadGetFile(ctx, &getFile)
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue
			}
			// return block{}, errors.Wrap(err, "get next chunk")
			return nil, err
		}
		// https://core.telegram.org/type/upload.File
		switch part := part.(type) {
		case *tg.UploadFile:
			// Advance
			data = part.Bytes
			getFile.Offset += int64(len(data)) // getFile.Limit
			if stream == nil {
				// Init target
				grpcClient := client.DefaultClient
				store := storage.NewFileService("storage", grpcClient)
				stream, err = store.UploadFile(ctx)
				if err != nil {
					return nil, err
				}
				// https://core.telegram.org/type/storage.FileType
				switch part.Type.(type) {
				case *tg.StorageFileJpeg:
					media.Mime = "image/jpeg"
				case *tg.StorageFileGif:
					media.Mime = "image/gif"
				case *tg.StorageFilePng:
					media.Mime = "image/png"
				case *tg.StorageFilePdf:
					media.Mime = "application/pdf"
				case *tg.StorageFileMp3:
					media.Mime = "audio/mpeg"
				case *tg.StorageFileMov:
					media.Mime = "video/quicktime"
				case *tg.StorageFileMp4:
					media.Mime = "video/mp4"
				case *tg.StorageFileWebp:
					media.Mime = "image/webp"
				// case *tg.StorageFileUnknown: // Unknown type.
				// 	media.Mime = "application/octet-stream"
				// case *tg.StorageFilePartial: // Part of a bigger file.
				// 	if media.Mime == "" {
				// 		panic("telegram/upload.getFile(*tg.StorageFilePartial)")
				// 	}
				default:
					// panic("telegram/upload.getFile(default)")
					if media.Mime == "" {
						media.Mime = "application/octet-stream" // default
					}
				}
				// Default filename generation
				if media.Name == "" {
					media.Name = media.Mime
					// trim media[/subtype]
					if n := strings.IndexByte(media.Name, '/'); n > 0 {
						media.Name = media.Name[0:n]
					}
					switch media.Name {
					case "image", "audio", "video":
					// case "application":
					default:
						media.Name = "file"
					}
					media.Name += "_1" // FIXME: need a suffix generation to be sure filename is unique
					ext, _ := mime.ExtensionsByType(media.Mime)
					if len(ext) != 0 {
						media.Name += ext[0]
					}
				}
				// INIT: WRITE
				err = stream.Send(&storage.UploadFileRequest{
					Data: &storage.UploadFileRequest_Metadata_{
						Metadata: &storage.UploadFileRequest_Metadata{
							DomainId: c.Gateway.DomainID(),
							MimeType: media.Mime,
							Name:     media.Name,
							Uuid:     uuid.Must(uuid.NewRandom()).String(),
						},
					},
				})
				if err != nil {
					return nil, err
				}
				// defer stream.Close()
			}
			// WRITE
			mpart.Chunk = part.Bytes
			err = stream.Send(&push)
			if err != nil {
				if _, re := stream.CloseAndRecv(); re != nil {
					err = re // drain original error from APIl not grpc.stream EOF
				}
				// c.Gateway.Log.Err(re).Interface("res", res).Msg("storage.uploadFile(cancel)")
				return nil, err
			}
			// if len(data) == 0 {
			if len(data) < getFile.Limit {
				// That was the last part !
				// Send EOF file mark !
				mpart.Chunk = nil
				_ = stream.Send(&push)
				break loop
			}
			// time.Sleep(time.Second / 2)
		case *tg.UploadFileCDNRedirect:
			return nil, &downloader.RedirectError{Redirect: part}
		default:
			// return chunk{}, errors.Errorf("unexpected type %T", chunk)
			return nil, errors.BadGateway(
				"telegram.upload.getFile.unexpected",
				"telegram/upload.getFile: unexpected result %T type",
				part,
			)
		}
	}

	// var res *storage.UploadFileResponse
	res, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	fileURI := res.FileUrl
	if path.IsAbs(fileURI) {
		// NOTE: We've got not a valid URL but filepath
		srv := c.Gateway.Internal
		hostURL, err := url.ParseRequestURI(srv.HostURL())
		if err != nil {
			panic(err)
		}
		fileURL := &url.URL{
			Scheme: hostURL.Scheme,
			Host:   hostURL.Host,
		}
		fileURL, err = fileURL.Parse(fileURI)
		if err != nil {
			panic(err)
		}
		fileURI = fileURL.String()
		res.FileUrl = fileURI
	}

	media.Id = res.FileId
	media.Url = res.FileUrl
	media.Size = res.Size

	return media, nil
}

// ----------------------------------------------------
// u: updates
//   updates:
//   - updatePeerSettings
//       peer: peerUser
//         user_id: 520924760
//       settings: peerSettings
//
//   users:
//   - user
//       id: 520924760
//       access_hash: -5194293066050022801
//       first_name: srgdemon
//       username: srgdemon
//       photo: userProfilePhoto
//         photo_id: 2237354808332887977
//         stripped_thumb: AQgIoOQIwCPyooooAw
//         dc_id: 2
//       status: userStatusRecently
//
//   chats:
//
//   date: 2022-07-01T12:28:19Z
//   seq: 2
// ----------------------------------------------------
// u: updateShort
//   update: updateUserTyping
// 	user_id: 520924760
// 	action: sendMessageTypingAction
// date: 2022-07-01T12:28:46Z
// ----------------------------------------------------
// u: updateShort
//   update: updateNewMessage
// 	message: message
// 		id: 399
// 		from_id: peerUser
// 			user_id: 520924760
// 		peer_id: peerUser
// 			user_id: 520924760
// 		date: 2022-07-01T12:28:52Z
// 		message: Hi
// 	pts: 509
// 	pts_count: 1
// date: 2022-07-01T12:28:52Z
// ----------------------------------------------------
// Settings of a certain peer have changed
func (c *App) onPeerSettings(ctx context.Context, e tg.Entities, update *tg.UpdatePeerSettings) error {
	c.Gateway.Log.Debug().Interface("update", update).Interface("entities", e).Msg("updatePeerSettings")
	return nil
}

// New message in a private chat or in a basic group.
// https://core.telegram.org/constructor/updateNewMessage
func (c *App) onNewMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {

	c.Gateway.Log.Debug().Interface("update", update).Interface("entities", e).Msg("updateNewMessage")

	// switch update.Message.(type) {
	// case *tg.MessageService:
	// case *tg.Message:
	// default:
	// }
	sentMessage, ok := update.Message.(*tg.Message)
	if !ok || sentMessage.Out {
		// Outgoing message, not interesting.
		return nil
	}
	// Handle Private chats only !
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
		c.Gateway.Log.Warn().Str("error", "not private; sender .from.chatId is missing").Msg("IGNORE")
		return nil // IGNORE Unable to resolve sender
	}

	// ECHO
	// _, err := c.Sender.Answer(e, update).Text(ctx, sentMessage.GetMessage())
	// return err

	peer, err := c.peers.ResolveUserID(ctx, fromId)

	if err != nil {
		c.Gateway.Log.Err(err).Interface("peer", peerId).Msg("telegram/updateNewMessage.peer")
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
		c.Gateway.Log.Warn().Str("error", errorMsg).Msg("IGNORE")
		return nil
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
			c.Gateway.Log.Warn().Err(re).Msg("telegram/messages.readHistory")
		}
	}()

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

	channel, err := c.Gateway.GetChannel(
		ctx, chatId, contact,
	)

	if err != nil {
		// Failed locate chat channel !
		c.Gateway.Log.Err(err).Msg("telegram/updateNewMessage")
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

	// endregion
	sendUpdate := bot.Update{

		// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),

		User:  contact,
		Chat:  channel,
		Title: channel.Title,

		Message: new(chat.Message),
	}

	sendMessage := sendUpdate.Message

	// coalesce := func(argv ...string) string {
	// 	for _, s := range argv {
	// 		if s = strings.TrimSpace(s); s != "" {
	// 			return s
	// 		}
	// 	}
	// 	return ""
	// }

	file := &chat.File{}
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
					file.Size = int64(photoSize.Size)
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

			file, err := c.getFile(file, &location)
			if err != nil {
				c.Gateway.Log.Err(err).Msg("telegram.upload.getFile")
				return nil // break
			}
			sendMessage.Type = "file"
			sendMessage.File = file
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
				c.Gateway.Log.Warn().Str("error", "MessageMediaDocument is not *tg.Document").Msg("IGNORE")
				return nil
			}
			file.Mime = doc.GetMimeType()
			file.Size = int64(doc.GetSize())
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
							file.Size = int64(thumb.Size)
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

					file.Mime = "image/webp"
					file.Name = uuid.Must(uuid.NewRandom()).String() + ".webp"

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

					if file.Name == "" {
						file.Name = att.GetFileName()
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

			file, err := c.getFile(file, &location)
			if err != nil {
				c.Gateway.Log.Err(err).Msg("telegram.upload.getFile")
				return nil // break
			}
			sendMessage.Type = "file"
			sendMessage.File = file

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
			c.Gateway.Log.Warn().Str("error", fmt.Sprintf("media.(%T) reaction not implemented", media)).Msg("telegram/updateNewMessage")
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
	err = c.Gateway.Read(ctx, &sendUpdate)

	if err != nil {
		// FIXME: send error as an answer ?
		return err
	}

	return nil
}

func (c *App) init(debug bool) {

	c.log, _ = zap.NewDevelopment(
		zap.IncreaseLevel(zapcore.InfoLevel),
		zap.AddStacktrace(zapcore.FatalLevel),
	)

	var (
		handler      telegram.UpdateHandler
		dispatcher   = tg.NewUpdateDispatcher()
		sessionStore = &sessionStore{App: c}
		options      = telegram.Options{
			// DC:     2,
			// DCList: dcs.Prod(),
			Logger:         c.log,
			SessionStorage: sessionStore,
			UpdateHandler: telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
				// Print all incoming updates.
				if debug {
					fmt.Println("u:", formatObject(u))
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
								if login := c.Login; login != nil {

									login.Mutex.Lock()
									defer login.Mutex.Unlock()

									login.user = nil
									login.session = nil
									login.signal()
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
							c.Gateway.Log.Warn().Err(err).Msg("MIDDLEWARE")
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
			options.Middlewares, prettyMiddleware(),
		)
	}

	c.Client = telegram.NewClient(
		c.apiId, c.apiHash, options,
	)

	c.cache = &InmemoryCache{}
	c.store = &InmemoryStorage{}
	// if err := c.restoreHash(); err != nil {
	// 	c.Gateway.Log.Warn().Err(err).Msg("RESTORE: HASH")
	// }

	c.peers = peers.Options{
		Logger:  c.log.Named("peers"),
		Storage: c.store,
		Cache:   c.cache,
	}.Build(
		c.Client.API(),
	)

	c.gaps = updates.New(
		updates.Config{
			Logger:       c.log.Named("gaps"),
			Handler:      dispatcher,
			AccessHasher: c.peers,
		},
	)
	// Chain peers/gaps handlers ...
	handler = c.peers.UpdateHook(c.gaps)

	// Bind Receiver
	dispatcher.OnPeerSettings(c.onPeerSettings) // newInboundUser access from here ...
	dispatcher.OnNewMessage(c.onNewMessage)
	// Once telegram/message.Sender state init
	c.Sender = message.NewSender(c.Client.API())

	c.onClose = []func(){
		func() {
			// Flush logs ...
			_ = c.log.Sync()
			// Flush cached data ...
			sessionStore.StoreSession(context.TODO(), nil)
			// Save logoutTokens...
			c.backup()
		},
	}
}

// runtime routine
func (c *App) run(ctx context.Context, start chan<- error) {

	var (
		exit chan error
		app  = c.Client
		err  = app.Run(ctx, func(ctx context.Context) error {
			// gain .exit request
			defer func() {
				if exit == nil {
					select { // try
					case exit = <-c.exit:
					default: // non-blocking
					}
				}
			}()
			// Subscribe onAuthorizationState changes
			// NOTE: calls getMe() here ...
			var state error
			c.Login, state = newSessionLogin(
				c.apiId, c.apiHash, c.peers, c.log.Named("login"),
			)
			//
			select {
			case start <- state:
				close(start)
			default:
			}

			c.restore() // login.tokens

			onAuthZstate := c.Login.Subscribe()
			defer func() {
				c.Login.Unsubscribe(onAuthZstate)
			}()

			for {

				me := c.me // current
				select {   // blocks
				case c.me = <-onAuthZstate: // c.Login.Wait(): // FIXME: each loop gen new chan !
					// c.Gateway.Log.Debug().Bool("authZ", (c.me != nil)).Msg("telegram.onAuthorizationState")
					if c.me == me {
						continue // loop // for
					}
					if c.me == nil {
						// LoggedOut (Unauthorized)
						c.Gateway.Log.Warn().Str("state", "loggedOut").Msg("telegram.onAuthorizationState")
						// TODO: clear cache, peers and so on ...
						_ = c.gaps.Logout()
						// c.peers.me.Store(nil)
						c.cache.Purge()
						c.store.Purge()
						// c.peers.Logout() // FIXME: clear cache entities ...
						continue // loop // for
					}
					// SignedIn (Authorized)
					c.Gateway.Log.Info().Str("state", "signedIn").Msg("telegram.onAuthorizationState")
					// break // select
				case <-ctx.Done():
					return ctx.Err()
				case exit = <-c.exit:
					return ctx.Err()
				}

				// MUST: c.me != nil

				// Notify update manager about authentication.
				err := c.gaps.Auth(
					ctx, app.API(),
					c.me.ID, c.me.Bot, true,
				)

				if err != nil {
					return err
				}

				// Get all dialogs and fill internal cache entities ...
				// c.peers middleware will handle and cache results ...
				req := &tg.MessagesGetDialogsRequest{
					OffsetPeer: &tg.InputPeerEmpty{}, // all
					Limit:      100,                  // Let the server choose ...
				}
				var i int
			paging:
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
						c.Gateway.Log.Warn().Err(err).Msg("messages.getDialogs")
						break paging

					} else {
						// TODO: handle pagination ...
						switch res := res.(type) {
						case *tg.MessagesDialogs:
							break paging

						case *tg.MessagesDialogsSlice:

							if 0 < req.Limit && len(res.Messages) < req.Limit {
								break paging // last page !
							}
							top, ok := res.MapMessages().LastAsNotEmpty()
							if !ok {
								break paging
							}
							req.OffsetDate = top.GetDate()

						case *tg.MessagesDialogsNotModified:
							break paging
						}
					}
				}
				// c.Gateway.Log.Debug().Int("pages", i+1).Msg(
				// 	"messages.getDialogs -------------------------------------",
				// )

				// wait for .stop or .cancel request
				// Just contine the main loop ....
			}
		})
	)

	c.sync.Lock()
	c.started = false
	c.sync.Unlock()
	// finally
	for _, closed := range c.onClose {
		closed()
	}
	// finally
	select {
	// try respond to .stop() request
	case exit <- err:
	// catch
	default:
		select {
		// try respond to .start() request
		case start <- err:
		// catch
		default:
		}
	}
}

// const metadataHash = ".hash"
const metadataAuth = ".auth"

var hashCodec = base64.RawURLEncoding

// backup: bot.metadata[.hash] = c.store
func (c *App) backup() {

	if login := c.Login; login != nil {
		if data, _ := login.backup(); len(data) != 0 {
			// set := hashCodec.EncodeToString(data)
			err := c.Gateway.SetMetadata(
				context.TODO(), map[string]string{
					metadataAuth: hashCodec.EncodeToString(data),
				},
			)
			if err != nil {
				c.Gateway.Log.Warn().Err(err).Msg("BACKUP: AUTH")
			}
		}
	}
}

func (c *App) restore() error {
	profile := c.Gateway.GetMetadata()
	if set := profile[metadataAuth]; set != "" {
		data, err := hashCodec.DecodeString(set)
		if err != nil {
			return err
		}
		return c.Login.restore(data)
	}
	return nil
}
