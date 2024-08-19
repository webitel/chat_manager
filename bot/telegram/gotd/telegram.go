package gotd

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
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"github.com/webitel/chat_manager/bot/telegram/internal/markdown"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() {
	bot.Register(providerType, connect)
}

type app struct {
	*bot.Gateway        // Messages gateway profile
	apiId        int    // Telegram API-ID
	apiHash      string // Telegram API-Hash
	appIdent     []byte // MD5(.apiId+.apiHash)
	phone        string // currently not used
	//
	*session // Telegram session runtime
}

const (
	providerType  = "gotd"
	optionApiId   = "api_id"
	optionApiHash = "api_hash"
	optionPhone   = "phone" // international format: +(country code)(city or carrier code)(your number)
	optionDebug   = "debug"

	optionSessionData = ".gotd"
	optionSessionAuth = ".auth"
)

var (
	binaryText = base64.RawStdEncoding
)

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

func connect(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {
	var (
		err     error
		apiId   int
		apiHash string
		config  = agent.Bot
		profile = config.GetMetadata()
	)

	if s, _ := profile[optionApiId]; s != "" {
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

	apiHash, _ = profile[optionApiHash]
	if apiHash == "" {
		return nil, errors.BadRequest(
			"chat.bot.telegram.api_hash.required",
			"telegram: api_hash required but missing",
		)
	}

	// Parse and validate message templates
	agent.Template = bot.NewTemplate(
		providerType,
	)
	// Populate telegram-specific markdown-escape helper funcs
	agent.Template.Root().Funcs(
		markdown.TemplateFuncs,
	)
	// Parse message templates
	if err = agent.Template.FromProto(
		agent.Bot.GetUpdates(),
	); err == nil {
		// Quick tests ! <nil> means default (well-known) test cases
		err = agent.Template.Test(nil)
	}
	if err != nil {
		return nil, errors.BadRequest(
			"chat.bot.telegram.updates.invalid",
			err.Error(),
		)
	}

	// If API IDentification(apiId+apiHash) didn't change
	// return the latest `state` as a current new one !
	// NOTE: this will not suspend or restart runtime routines
	var (
		latest, _ = state.(*app)
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
	phone, _ := profile[optionPhone]

	app := &app{

		apiId:    apiId,
		apiHash:  apiHash,
		appIdent: appIdent,

		phone: phone,

		Gateway: agent,
	}

	log := agent.Log.With().
		Str("tg", fmt.Sprintf("/app%d:%s", apiId, apiHash)).
		Logger()

	agent.Log = &log
	// await connection
	return app, app.connect()
}

func (c *app) connect() error {
	conn := c.session
	if conn == nil {
		conn = &session{App: c}
		c.session = conn
	}
	return conn.connect()
}

var _ bot.Provider = (*app)(nil)

// String provider's code name
func (c *app) String() string {
	return providerType
}

// Close shuts down bot and all it's running session(s)
func (c *app) Close() error {
	return c.stop() // await
}

// Register webhook callback URI
func (c *app) Register(ctx context.Context, uri string) error {
	// TODO: check authentication state
	// TODO: go background runtime routine
	return nil // c.start()
}

// Deregister webhook callback URI
func (c *app) Deregister(ctx context.Context) error {
	// FIXME: stop listening ?
	// return c.stop()
	return nil
}

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}

// channel := notify.Chat
// contact := notify.User
func (c *app) SendNotify(ctx context.Context, notify *bot.Update) error {
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
		_, err = sendMessage.StyledText(
			ctx, FormatText(sentMessage.Text), // markdown.FormatText(sentMessage.Text),
		)
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
			FromMediaFile(mediaFile, nil),
			// message.FromURL(mediaFile.Url),
			// message.FromSource(source.NewHTTPSource(), mediaFile.Url),
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
				// _, err = uploadFile.Photo(ctx, caption...)
				inputFile, re := uploadFile.AsInputFile(ctx)
				if err = re; err == nil {
					_, err = sendMessage.Media(ctx,
						message.UploadedPhoto(inputFile, caption...),
					)
				}

			case "audio":
				// _, err = uploadFile.Audio(ctx, caption...)
				inputFile, re := uploadFile.AsInputFile(ctx)
				if err = re; err == nil {
					// Send as an Audio document
					_, err = sendMessage.Media(ctx,
						message.UploadedDocument(inputFile, caption...).
							Filename(mediaFile.Name).
							MIME(mediaFile.Mime).
							Audio(), // .Title(mediaFile.Name),
					)
				}

			case "video":
				// _, err = uploadFile.Video(ctx, caption...)
				inputFile, re := uploadFile.AsInputFile(ctx)
				if err = re; err == nil {
					// Send as a Video document
					_, err = sendMessage.Media(ctx,
						message.UploadedDocument(inputFile, caption...).
							Filename(mediaFile.Name).
							MIME(mediaFile.Mime).
							Video(),
					)
				}

			default:
				// _, err = uploadFile.File(ctx, caption...)
				inputFile, re := uploadFile.AsInputFile(ctx)
				if err = re; err == nil {
					// Send as a Document file
					_, err = sendMessage.Media(ctx,
						message.UploadedDocument(inputFile, caption...).
							Filename(mediaFile.Name).
							MIME(mediaFile.Mime),
					)
				}
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
	case "left":

		peer := contactPeer(sentMessage.LeftChatMember)
		updates := c.Gateway.Template
		messageText, err := updates.MessageText("left", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("telegram/bot.updateLeftMember")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		_, err = sendMessage.StyledText(
			ctx, markdown.FormatText(messageText),
		)
		if err != nil {
			c.Gateway.Log.Err(err).
				Msg("telegram/bot.updateLeftMember")
			return err
		}

	case "joined":

		peer := contactPeer(sentMessage.NewChatMembers[0])
		updates := c.Gateway.Template
		messageText, err := updates.MessageText("join", peer)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("telegram/bot.updateChatMember")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		// format new message to the engine for saving it in the DB as operator message [WTEL-4695]
		messageToSave := &chat.Message{
			Type:      "text",
			Text:      messageText,
			CreatedAt: time.Now().UnixMilli(),
			From:      peer,
		}
		if peerChannel != nil && peerChannel.ChannelID != "" {
			_, err = c.Gateway.Internal.Client.SaveAgentJoinMessage(ctx, &chat.SaveAgentJoinMessageRequest{Message: messageToSave, Receiver: peerChannel.ChannelID})
			if err != nil {
				return err
			}
		}
		_, err = sendMessage.StyledText(
			ctx, markdown.FormatText(messageText),
		)
		if err != nil {
			c.Gateway.Log.Err(err).
				Msg("telegram/bot.updateChatMember")
			return err
		}

	case "closed":

		updates := c.Gateway.Template
		messageText, err := updates.MessageText("close", nil)
		if err != nil {
			c.Gateway.Log.Err(err).
				Str("update", sentMessage.Type).
				Msg("telegram/bot.updateChatClose")
		}
		messageText = strings.TrimSpace(
			messageText,
		)
		if messageText == "" {
			// IGNORE: empty message text !
			return nil
		}
		_, err = sendMessage.StyledText(
			ctx, markdown.FormatText(messageText),
		)
		if err != nil {
			c.Gateway.Log.Err(err).
				Msg("telegram/bot.updateChatClose")
			return err
		}

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
func (c *app) WebHook(rsp http.ResponseWriter, req *http.Request) {

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

		// region: --- AdminAuthorization(!) ---
		if c.Gateway.AdminAuthorization(rsp, req) != nil {
			return // Authorization FAILED(!)
		}
		// endregion: --- AdminAuthorization(!) ---

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
		// c.sync.RLock() // LOCKED +R
		status.Connected = true // c.started
		// c.sync.RUnlock() // UNLOCK -R
		if me := c.login.User(); status.Connected && me.GetID() != 0 {
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

			err := c.login.LogOut(ctx)
			if err != nil {
				writeTgError(rsp, err, 400)
				return // 400
			}
			writeJSON(rsp, "OK", http.StatusOK)
			return // 200

		} else if _, is = form["cancel"]; is {

			err := c.login.CancelCode(ctx)
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
			sentCode, err := c.login.SendCode(ctx, phone)
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
			authZ, err := c.login.SignIn(ctx, code)
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
			authZ, err := c.login.Password(ctx, pass)
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

// Broadcast given `req.Message` message [to] provided `req.Peer(s)`
func (c *app) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {

	if authZ := c.session.login.User(); authZ == nil {
		return errors.BadGateway(
			"chat.broadcast.telegram.unauthorized",
			"telegram: app unauthorized",
		)
	}

	var (
		n          = len(req.GetPeer())
		inputPeers = make([]struct {
			id    int
			input tg.InputPeerClass
		}, 0, n)
		resolvedPeer = func(peerId int, peer tg.InputPeerClass) error {
			if peer.Zero() {
				return &peers.PhoneNotFoundError{}
			}
			for _, resolved := range inputPeers {
				if reflect.DeepEqual(resolved.input, peer) {
					return errors.BadRequest(
						"chat.broadcast.peer.duplicate",
						"peer: duplicate; ignore",
					)
				}
			}
			// inputPeers[id] = peer
			inputPeers = append(inputPeers, struct {
				id    int
				input tg.InputPeerClass
			}{
				id:    peerId,
				input: peer,
			})

			return nil
		}
		resolvedError = func(peerId int, err error) {

			res := rsp.GetFailure()
			if res == nil {
				res = make([]*chat.BroadcastPeer, 0, n)
			}

			var re *status.Status
			switch err := err.(type) {
			case *tgerr.Error:
				re = status.New(codes.Code(err.Code), err.Message)
			case *errors.Error:
				re = status.New(codes.Code(err.Code), err.Detail)
			default:
				re = status.New(codes.Unknown, err.Error())
			}

			res = append(res, &chat.BroadcastPeer{
				Peer:  req.Peer[peerId],
				Error: re.Proto(),
			})

			rsp.Failure = res
		}
		id int
	)

	for id < n {
		peerId := req.Peer[id]
		peer, err := c.resolve(ctx, peerId)
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue // retry
			}
			resolvedError(id, err)
			id++ // next
			continue
		}
		err = resolvedPeer(id, peer.InputPeer())
		if err != nil {
			resolvedError(id, err)
		}
		id++ // next
		// continue
	}

	// PERFORM: sendMessage to resolved peer(s)...
	id, n = 0, len(inputPeers)
	for id < n {
		peer := inputPeers[id].input
		sendMessage := c.Sender.To(peer)
		// TODO: transform given msg to sendMessage
		// sendMessage.Text(ctx, "message test")
		_, err := sendMessage.Text(ctx, req.GetMessage().GetText())
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue // retry
			}
			resolvedError(inputPeers[id].id, err)
			id++ // next
			continue
		}
		id++ // next
		// continue
	}

	return nil
}
