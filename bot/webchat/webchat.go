package webchat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	errs "github.com/micro/go-micro/v2/errors"
	"github.com/pkg/errors"

	"github.com/gorilla/websocket"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
)

// webChat room with external client
// posibly with multiple peer connections
type webChat struct {
	 // Chat Service (internal) connection
	 Bot *WebChatBot
	 // Chat Channel (external) definition
	*bot.Channel
	 // This chat opened connections (different tabs)
	 conn []*websocket.Conn
	 closed bool
	 // Buffered channel for sync write operations.
	 send chan func() // [sync] write(!)
	 wbuf bytes.Buffer // codec write buffer: size = Bot.options.WriteBufferSize
	 // Chat history: messages
	 msgi map[int64]int // index[msg.id]
	 msgs []*chat.Message // ordinal
	 
}

// save given *chat.Message m to this *webChat c local history store
func (c *webChat) pushMessage(m *chat.Message) {
	if m.Id == 0 {
		// NOTE: This is the service message !
		return
	}
	
	if i, ok := c.msgi[m.Id]; ok {
		c.msgs[i] = m
		return // edited
	}
	c.msgi[m.Id] = len(c.msgs)
	c.msgs = append(c.msgs, m)
	return
}

type originPattern interface {
	 match(origin string) bool
}

type originAny bool
func (pttn originAny) match(origin string) bool {
	return origin != "" && (bool)(pttn)
}

type originString string
func (pttn originString) match(origin string) bool {
	return (string)(pttn) == (origin)
}

type originWildcard [2]string
func (pttn originWildcard) match(origin string) bool {
	prefix, suffix := pttn[0], pttn[1]
	return len(origin) >= len(prefix)+len(suffix) && 
		strings.HasPrefix(origin, prefix) &&
		strings.HasSuffix(origin, suffix)
}

// WebChatBot gateway provider
type WebChatBot struct {
	
	*bot.Gateway
	// Websocket configuration options
	 Websocket websocket.Upgrader
	 // ReadTimeout duration allowed to wait for
	 // incoming message from peer connection.
	 // Also used to send periodical PINGs
	 // to keep-alive peer connection.
	 ReadTimeout time.Duration
	 // WriteTimeout duration allowed to write
	 // a single frame/message to the peer connection
	 WriteTimeout time.Duration
	 // MessageSizeMax allowed from/for peer connection.
	 // JSON-encoded single frame/message MAX size.
	 MessageSizeMax int64 
	 // Unexported: runtime chat(s) store
	*sync.RWMutex
	 chat map[string]*webChat
}

// NewWebChatBot initialize new agent.Profile service provider
// func NewWebChatBot(agent *bot.Gateway) (bot.Provider, error) {
func NewWebChatBot(agent *bot.Gateway, state bot.Provider) (bot.Provider, error) {
	// panic("not mplemented")
	bot := &WebChatBot{
		Gateway: agent,
		// Setup: defaults ...
		Websocket: websocket.Upgrader{
			HandshakeTimeout:  10 * time.Second,
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
			WriteBufferPool:   &sync.Pool{
				New: func() interface{} {
					return nil
				},
			},
			Subprotocols:      nil,
			EnableCompression: false,
			CheckOrigin:       func(req *http.Request) bool {
				return true // Default: NO check at all (!)
			},
			Error:             func(rsp http.ResponseWriter, req *http.Request, code int, err error) {
				// panic("not implemented")
				if err == nil {
					err = fmt.Errorf(http.StatusText(code))
				}
				rsp.Header().Set("Sec-Websocket-Version", "13")
				http.Error(rsp, err.Error()/*http.StatusText(code)*/, code) // err.Error(), code)
				
				if err == nil {
					agent.Log.Error().
						Str("peer", webChatRemoteIP(req)).
						Msgf("%d %s", code, http.StatusText(code))
				} else {
					agent.Log.Err(err).
						Str("peer", webChatRemoteIP(req)).
						Msgf("%d %s", code, http.StatusText(code))
				}
			},
		},
		ReadTimeout: time.Second * 30,  // 30s (PING)
		WriteTimeout: time.Second * 10, // 10s
		MessageSizeMax: 4 << 10,        // 4096 (bytes)
		// // runtime chat store
		// chat: make(map[string]*webChat, 4096), // (slots)
	}

	opts := &bot.Websocket
	profile := agent.GetMetadata()
	// config := agent.Profile
	// profile := config.GetProfile()

	if s := profile["handshake_timeout"]; s != "" {
		tout, err := time.ParseDuration(s)
		if err != nil {
			return nil, errors.Wrap(err, "[handshake_timeout]: %s invalid duration string")
		}
		if tout > 0 {
			// Check: MIN
			if tout < time.Second {
				tout = time.Second
			}
			// Check: MAX
			if tout > time.Minute {
				tout = time.Minute
			}
			// SET
			opts.HandshakeTimeout = tout
			
		} else {
			// FIXME: assume no timeout
		}
	}

	if s := profile["message_size_max"]; s != "" {
		size, err := strconv.Atoi(s)
		if err != nil {
			return nil, errors.Wrap(err, "[message_size_max]: %s invalid integer value")
		}
		if size > 0 {
			const (
				sizeMin = 1 << 10 // 1K
				sizeMax = sizeMin << 3 // 8K
			)
			// Check: MIN
			if size < sizeMin {
				size = sizeMin
			}
			// Check: MAX
			if size > sizeMax {
				size = sizeMax
			}
			// SET
			bot.MessageSizeMax = int64(size)
			opts.ReadBufferSize = size
			opts.WriteBufferSize = size
			
		} else {
			// FIXME: assume no PING
		}
	}

	if s := profile["write_timeout"]; s != "" {
		tout, err := time.ParseDuration(s)
		if err != nil {
			return nil, errors.Wrap(err, "[write_timeout]: %s invalid duration string")
		}
		if tout > 0 {
			const (
				tmin = time.Second // 1s
				tmax = time.Minute // 1m
			)
			// Check: MIN
			if tout < tmin {
				tout = tmin
			}
			// Check: MAX
			if tout > tmax {
				tout = tmax
			}
			// SET
			bot.WriteTimeout = tout
			
		} else {
			// FIXME: assume no PING
		}
	}

	if s := profile["read_timeout"]; s != "" {
		tout, err := time.ParseDuration(s)
		if err != nil {
			return nil, errors.Wrap(err, "[read_timeout]: %s invalid duration string")
		}
		if tout > 0 {
			const (
				tmin = time.Second * 10 // 10s
				tmax = time.Minute * 10 // 10m
			)
			// Check: MIN
			if tout < tmin {
				tout = tmin
			}
			// Check: MAX
			if tout > tmax {
				tout = tmax
			}
			// SET
			bot.ReadTimeout = tout
			
		} else {
			// FIXME: assume no PING
		}
	}

	// AllowOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// An origin may contain a wildcard (*) to replace 0 or more characters
	// (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty.
	// Only one wildcard can be used per origin.
	allowOrigin := profile["allow_origin"]
	allowOrigins := strings.Split(allowOrigin, ",")
	allowedOrigins := make([]originPattern, 0, len(allowOrigins))
	for _, origin := range allowOrigins {
		// Normalize
		origin = strings.ToLower(origin)
		if origin == "*" {
			// If "*" is present in the list, turn the whole list into a match all
			allowedOrigins = append(allowedOrigins[:0], originAny(true))
			break
		} else if i := strings.IndexByte(origin, '*'); i >= 0 {
			// Split the origin in two: start and end string without the *
			allowedOrigins = append(allowedOrigins, originWildcard{origin[0:i], origin[i+1:]})
		} else if origin != "" {
			allowedOrigins = append(allowedOrigins, originString(origin))
		}
	}
	// // Default value is ["*"]
	// if allowOrigin == "" && len(allowedOrigins) == 0 {
	// 	allowedOrigins = append(allowedOrigins, originAny(true))
	// }
	if len(allowedOrigins) != 0 {
		// X-Access-Control-Allow-Origin
		opts.CheckOrigin = func(req *http.Request) bool {
			// return true
			origin := req.Header.Get(hdrOrigin)
			origin = strings.ToLower(origin)
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin.match(origin) {
					return true
				}
			}
			return false
		}
	}

	if state, ok := state.(*WebChatBot); ok && state != nil {
		bot.RWMutex = state.RWMutex
		bot.chat = state.chat
	} else {
		bot.RWMutex = new(sync.RWMutex)
		bot.chat = make(map[string]*webChat, 4096)
	}

	// go bot.runtime(context.Background())

	return bot, nil
}

const (
	// Canonical WebChat Provider name
	providerWebChat = "webchat"
)

func (*WebChatBot) String() string {
	return providerWebChat
}

// Register webhook callback URI
func (*WebChatBot) Register(ctx context.Context, uri string) error {
	return nil
}
// Deregister webhook callback URI
func (*WebChatBot) Deregister(ctx context.Context) error {
	return nil
}

// SendNotify implements Sender interface.
// channel := notify.Chat
// contact := notify.User
func (c *WebChatBot) SendNotify(ctx context.Context, notify *bot.Update) error {
	// panic("not mplemented")

	var (

		channel = notify.Chat // recepient
		message = notify.Message
	)

	closed := message.Type == "closed"

	c.RLock()   // +R
	room := c.chat[channel.ChatID]
	c.RUnlock() // -R

	if room == nil {
		if closed {
			return nil
		}
		defer channel.Close()
		c.Log.Error().Str("chat-id", channel.ChatID).Msg("CHAT: Channel NOT connected; Force .Close(!)")
		return errors.New("chat: no channel connection")
	}

	switch message.Type {
	case "text": // default
	case "file":
	case "left":
	case "joined":
	case "closed":
		defer func() {
			// // c.Lock()   // +RW
			// // delete(c.chat, chat.ChatID)
			// // c.Unlock() // -RW
			// close(room.send)
			room.send <- func() {
				for i := len(room.conn)-1; i >= 0; i-- {
					conn := room.conn[i]
					_ = conn.SetWriteDeadline(
						time.Now().Add(c.WriteTimeout),
					)
					err := conn.WriteMessage(
						websocket.CloseMessage, // []byte{},
						websocket.FormatCloseMessage(
							websocket.CloseNormalClosure,
							"BYE",
						),
					)
					if err != nil {
						c.Log.Err(err).
							Str("conn", conn.RemoteAddr().String()).
							Msg("WS.Close(!)")
					} else {
						c.Log.Debug().
							Str("conn", conn.RemoteAddr().String()).
							Msg("WS.Close(!)")
					}
					conn.Close()
				}
				room.conn = room.conn[:0]
				room.closed = true
			}
		} ()

	default:
	}

	update := webChatResponse{
		Message: message,
	}

	room.send <- func() {
		room.pushMessage(message)
		room.broadcast(update)
	}

	return nil // err
}

var (

	hdrHost           = http.CanonicalHeaderKey("Host")
	hdrOrigin         = http.CanonicalHeaderKey("Origin")
	hdrSetCookie      = http.CanonicalHeaderKey("Set-Cookie")

	hdrForwardedProto = http.CanonicalHeaderKey("X-Forwarded-Proto") // $scheme;
	hdrForwardedFor   = http.CanonicalHeaderKey("X-Forwarded-For") // $proxy_add_x_forwarded_for;
	hdrRealIP         = http.CanonicalHeaderKey("X-Real-IP")
)

// reports whether given req transport protocol is secured (TLS)
func httpIsSecure(req *http.Request) bool {

	const httpSecure = "https"

	fwdProto := req.Header.Get(hdrForwardedProto)
	if fwdProto = strings.TrimSpace(fwdProto); fwdProto != "" {
		return strings.ToLower(fwdProto) == httpSecure
	}

	return strings.ToLower(req.URL.Scheme) == httpSecure
}

// returns given req originator's IP address
func webChatRemoteIP(req *http.Request) string {

	// X-Forwarded-For: <client>, <proxy1>, <proxy2>
	fwdFor := req.Header.Get(hdrForwardedFor)
	if fwdFor = strings.TrimSpace(fwdFor); fwdFor != "" {
		if comma := strings.IndexByte(fwdFor, ','); comma > 3 {
			return strings.TrimSpace(fwdFor[:comma])
		} // else { // malformed ! }
	}

	// X-Real-IP: <client>
	realIP := req.Header.Get(hdrRealIP)
	if realIP = strings.TrimSpace(realIP); realIP != "" {
		return realIP
	}

	if hostIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		return hostIP
	}

	return ""
}

// returns given req remote device(+client) unique identification
func webChatDeviceID(req *http.Request) (id string, ok bool) {

	cookie, err := req.Cookie("cid")

	var deviceID string // end-user IDentifier
	if err != nil && err != http.ErrNoCookie {
		// Cookie parse error !
		// c.Log.Err(err)
	
	} else if cookie != nil {
		// GET device unique chat IDentifier
		deviceID = cookie.Value
	}

	if deviceID != "" {
		// DETECTED
		return deviceID, true
	}

	return deviceID, false
}

func generateRandomString(length int) string { // (string, error) {
	buf := make([]byte, int(math.Ceil(float64(length)/2)))
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		// return "", err
		panic(err)
	}
	// base64.RawURLEncoding.EncodeToString()
	text := hex.EncodeToString(buf)
	return text[:length]
}

// const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// func generateRandomString(n int) string {
//     b := make([]byte, n)
//     for i := range b {
//         b[i] = letterBytes[rand.Intn(len(letterBytes))]
//     }
//     return string(b)
// }

// Close bot and ALL it's running session(s) ...
func (c *WebChatBot) Close() error {

	c.Lock() // +RW
	for _, room := range c.chat {
		close(room.send)
	}
	c.Unlock() // -RW

	return nil
}

var cookieNeverExp = time.Date(2038, time.January, 19, 03, 14, 8, 000000000, time.UTC) // 2147483648 (2^31)

// Receiver
// WebHook callback http.Handler
// 
// // bot := BotProvider(agent *Gateway)
// ...
// recv := Update{/* decode from notice.Body */}
// err = c.Gateway.Read(notice.Context(), recv)
//
// if err != nil {
// 	http.Error(res, "Failed to deliver .Update notification", http.StatusBadGateway)
// 	return // 502 Bad Gateway
// }
//
// reply.WriteHeader(http.StatusOK)
// 
func (c *WebChatBot) WebHook(rsp http.ResponseWriter, req *http.Request) {

	// TODO: Check preconfigured CORS Options ...

	// c.Gateway.Log.Info().Msgf("[RW]: %T", rsp)

	if req.Method != http.MethodGet {
		c.Websocket.Error(rsp, req, http.StatusMethodNotAllowed, nil)
		// http.Error(rsp, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if !websocket.IsWebSocketUpgrade(req) {
		// TODO: handle other supported options here
		// http.ServeFile(rsp, req, "~/webitel/chat/bot/webchat/webchat.html")
		c.Websocket.Error(rsp, req, http.StatusBadRequest, nil)
		// // http.Error(rsp, "400 Bad Request", http.StatusBadRequest)
		return
	}

	if !c.Websocket.CheckOrigin(req) {
		c.Websocket.Error(rsp, req, http.StatusForbidden,
			fmt.Errorf("Origin: %s; Not Allowed", req.Header.Get(hdrOrigin)),
		)
		// http.Error(rsp, "403 Forbidden", http.StatusForbidden)
		return
	}

	responseHeader := rsp.Header()
	responseHeader.Set("Access-Control-Allow-Credentials", "true")
	responseHeader.Set("Access-Control-Allow-Methods", "OPTIONS, GET")
	responseHeader.Set("Access-Control-Allow-Headers", "Connection, Upgrade, Sec-Websocket-Version, Sec-Websocket-Extensions, Sec-Websocket-Key, Sec-Websocket-Protocol, Cookie")
	responseHeader.Set("Access-Control-Allow-Origin", req.Header.Get(hdrOrigin))

	var room *webChat
	deviceID, ok := webChatDeviceID(req)
	if !ok || deviceID == "" {
		// // Definitely: creating NEW client !
		// if !httpIsSecure(req) {
		// 	c.Websocket.Error(rsp, req, http.StatusMethodNotAllowed,
		// 		fmt.Errorf("Chat: secure connection required"),
		// 	)
		// 	// http.Error(rsp,
		// 	// 	"chat: secure connection required",
		// 	// 	 http.StatusBadRequest,
		// 	// )
		// 	return
		// }
		// Generate NEW client (+device) ID !
		deviceID = generateRandomString(32)

	} else {

		c.RWMutex.RLock()   // +R
		room = c.chat[deviceID]
		c.RWMutex.RUnlock() // -R

	}

	if room == nil {

		endUser := &bot.Account{
			ID:        0,
			FirstName: "Web",
			LastName:  "Chat",
			Username:  "",
			Channel:   c.String(),
			Contact:   deviceID,
		}
		// Find -or- Create chat User (client) !
		channel, err := c.Gateway.GetChannel(
			context.TODO(), deviceID, endUser,
		)
	
		if err != nil {
			// MAY: bot is disabled !
			c.Gateway.Log.Err(err).Msg("Failed to .GetChannel()")
			// Failed locate chat channel !
			re := errs.FromError(err); if re.Code == 0 {
				re.Code = (int32)(http.StatusBadGateway)
			}
			// conn.Write(!)
			http.Error(rsp, re.Detail, (int)(re.Code))
			return // 503 Bad Gateway
		}

		room = &webChat{

			Bot:     c,
			Channel: channel,
			
			send: make(chan func(), 1), // buffered
			
			msgi: make(map[int64]int, 32),
			msgs: make([]*chat.Message, 0, 32),
		}

		size := c.Websocket.WriteBufferSize
		if size > 0 { // reinit
			room.wbuf.Grow(size)
			room.wbuf.Reset()
		}
	}
	// Set-Cookie: cid=; IF not provided
	if !ok {
		// Proxy-Path:
		cookiePath := "/"
		if siteURL, err := url.Parse(c.Gateway.Internal.URL); err == nil {
			cookiePath = siteURL.Path // Resolve path prefix from public URL
		}
		cookiePath = strings.TrimRight(cookiePath, "/") + req.URL.Path
		// Set-Cookie:
		cookie := &http.Cookie{
			Name:       "cid",
			Value:      deviceID, // unique client + device identifier
			Path:       cookiePath, // req.URL.Path, // "/"+ c.Profile.UrlId, // TODO: prefix from NGINX proxy location
			// Domain:     domain, // req.Header.Get("Host"),
			Expires:    cookieNeverExp, // 2147483648 (2^31)
			// RawExpires: "",
			MaxAge:     0,
			Secure:     httpIsSecure(req), // req.URL.Schema == "https"
			HttpOnly:   true,
			// Cross-origin ([site]: example.com <-> [chat]: webitel.com) Set-Cookie
			// NOTE: https://developer.mozilla.org/de/docs/Web/HTTP/Headers/Set-Cookie/SameSite#none
			SameSite:   http.SameSiteNoneMode, // http.SameSiteLaxMode,
			// Raw:        "",
			// Unparsed:   nil,
		}
		if !cookie.Secure {
			cookie.SameSite = http.SameSiteLaxMode
		}
		responseHeader.Add(hdrSetCookie, cookie.String())
	}

	// UPGRADE: connection protocol !
	conn, err := c.Websocket.Upgrade(rsp, req, responseHeader)

	// NOTE: req released !
	if err != nil {
		c.Log.Err(err)
		return
	}

	c.join(room, conn)
}

// // routine opened c.chat channel(s); read messages ...
// func (c *WebChatBot) runtime(ctx context.Context) {

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			break
// 		}
// 	}
// }

// ChatInfo state message
type webChatInfo struct {
	Id   string          `json:"id"`
	User *bot.Account        `json:"user"`
	Msgs []*chat.Message `json:"msgs,omitempty"`
}

func (c *WebChatBot) join(client *webChat, conn *websocket.Conn) {

	chatID := client.ChatID
	primary := len(client.conn) == 0
	// PUSH: chatInfo !
	client.send <- func() {
		// build
		chatInfo := webChatInfo{
			Id:   client.ChannelID,
			User: &client.Account,
			Msgs: client.msgs,
		}
		jsonb, ok := client.encodeJSON(chatInfo) // json.Marshal(chatInfo)
		err := client.sendFrame(conn, websocket.TextMessage, jsonb)
		// if err != nil {
		// 	// client.conn = append(client.conn[0:i], client.conn[i+1:]...)
		// 	// _ = conn.Close()
		// }
		if ok && err == nil {
			// PUSH: to .writePump()
			client.conn = append(client.conn, conn)
		}
	}
	// conn.SetCloseHandler(func(code int, text string) error {
	// 	for i := 0; i < len(client.conn); i++ {
	// 		if client.conn[i] == conn {
	// 			client.conn = append(client.conn[0:i], client.conn[i+1:]...)
	// 			client.Log.Warn().Str("ws", conn.RemoteAddr().String()).Msg("REMOVED")
	// 			break
	// 		}
	// 	}
	// 	return nil
	// })
	// JOIN (!)
	c.RWMutex.Lock()   // +RW
	room, ok := c.chat[chatID]
	if ok && room == client {
		// // TODO: duplicate this chat connection
		// // go client.readPump(conn)
		c.RWMutex.Unlock() // -RW
		// DO NOT START WRITE ROUTINE !!!
		primary = false
	} else if room != nil {
		c.RWMutex.Unlock() // -RW
		panic("WebChatBot.join(): duplicate chat room id")
	} else {
		// Register NEW !
		c.chat[client.ChatID] = client
		c.RWMutex.Unlock() // -RW
		// secondary = false
		client.Log.Info().Msg("JOIN")
	}
	
	// STARTUP (!)
	if primary { // primary
		// TODO: show chatInfo
		if client.Channel.IsNew() {
			// /start
			commandStart := bot.Update {
			
				// ChatID: strconv.FormatInt(recvMessage.Chat.ID, 10),
				
				User:   &client.Account,
				Chat:    client.Channel,
				Title:   client.Channel.Title,
		
				Message: &chat.Message{
					Type: "text",
					Text: "/start",
				},
			}

			err := client.Gateway.Read(context.TODO(), &commandStart)

			if err != nil {
				client.Log.Err(err).Msg("START")
			} else {
				client.Log.Info().Msg("START")
			}

		} else {

			// RECOVER from DB !..
		}

		go client.writePump()

	} else { // secondary ...

	}


	go client.readPump(conn)
}

// WebChatRequest message envelope
type webChatRequest struct {
	Id      *json.RawMessage `json:"seq,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  interface{}      `json:"params,omitempty"`
	Message *chat.Message    `json:"message,omitempty"` // { method: "send" } as default
}

// WebChatResponse message envelope
type webChatResponse struct {
	Id      *json.RawMessage `json:"seq,omitempty"`
	Error   string           `json:"error,omitempty"`
	Result  interface{}      `json:"result,omitempty"`  // generic
	Message *chat.Message    `json:"message,omitempty"` // chat message update
}

// single websocket [conn]ection READer routine
func (c *webChat) readPump(conn *websocket.Conn) {
	defer func() {
		// // c.RWMutex.Lock() //   +RW
		select { // sync remove operation
		case c.send <- func() {
			// [sync] remove this conn
			var ok bool
			for i, this := range c.conn {
				if ok = (this == conn); ok {
					c.conn = append(c.conn[:i], c.conn[i+1:]...)
					break
				}
			}
			// if ok {
			// 	c.Log.Info().
			// 		Str("ws", conn.RemoteAddr().String()).
			// 		Msg("[WS] >>> READ.Close(!) <<< OK")
			// } else {
			// 	c.Log.Warn().
			// 		Str("ws", conn.RemoteAddr().String()).
			// 		Msg("[WS] >>> READ.Close(!) <<< NOT FOUND")
			// }
			// // NOTE: DO NOT c.closed = true due to
			// // page reloaded conn may return !
		}:
		default:
			c.Log.Error().
				Str("ws", conn.RemoteAddr().String()).
				Msg("[WS] >>> READ.Close(!) <<< OMITTED")
			// FIXME: Expect to be closed !
			// How to check it's NOT but full ?
		}
		
		// c.RWMutex.Unlock() // -RW
		_ = conn.Close() // Undelaying TCP
		// if err := conn.Close(); err != nil {
		// 	c.Log.Err(err).
		// 		Str("ws", conn.RemoteAddr().String()).
		// 		Msg("[WS] >>> READ.Close(!) <<<")
		// } else {
		// 	c.Log.Warn().
		// 		Str("ws", conn.RemoteAddr().String()).
		// 		Msg("[WS] >>> READ.Close(!) <<<")
		// }

	}()

	pongTimeout := c.Bot.ReadTimeout

	conn.SetReadLimit(c.Bot.MessageSizeMax)
	conn.SetReadDeadline(time.Now().Add(pongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})
	c.Log.Info().
		Str("ws", conn.RemoteAddr().String()).
		Msg("[WS] >>> Listen <<<")

	// reader := bytes.NewReader(nil)

	for {

		// typeOf, data, err := conn.ReadMessage()
		_, data, err := conn.ReadMessage()
		
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err, websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNoStatusReceived, // FIXME: Normal Close just with NO status text provided ?
			) {
				c.Log.Err(err).
					Str("ws", conn.RemoteAddr().String()).
					Msg("READ.Unexpected(!)")
			} else {
			// 	c.Log.Warn().Err(err).
			// 		Str("ws", conn.RemoteAddr().String()).
			// 		Msg("READ.Expected(+)")
				// _ = conn.WriteMessage(
				// 	websocket.CloseMessage, // []byte{},
				// 	websocket.FormatCloseMessage(
				// 		websocket.CloseNormalClosure,
				// 		"BYE",
				// 	),
				// )
			}
			return // runtime
		}
		// validate request
		var (
			
			msg *chat.Message
			req webChatRequest
			res webChatResponse
		)

		err = json.Unmarshal(data, &req)

		// if err == nil && request.ID == nil || len(*request.ID) == 0 {
		// 	// SEND: {"error": "request.id required but missing"}
		// 	err = fmt.Errorf("request.id required but missing")
		// 	break // loop: readPump
		// }

		// Respond TO Request ...
		res.Id = req.Id

		switch strings.ToLower(req.Method) {
		case "send", "": // default: "send"
			if msg = req.Message; msg == nil {
				err = fmt.Errorf("send: message is missing")
			}
		default:
			// SEND: {"error": "method not allowed"}
			err = fmt.Errorf("method=%q not allowed", req.Method)
		}

		// message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		// // c.hub.broadcast <- message
		if err == nil {
			err = c.Bot.Read(
				context.TODO(),
				&bot.Update{
					ID:      0,
					Chat:    c.Channel,
					User:    &c.Account,
					Title:   "",
					// Event:   msg.GetType(), // "text"
					Message: msg,
				},
			)
		}

		// if err != nil {
		// 	// c.Log.Err(err).Msg("Failed to deliver message")
		// }
		if err != nil {
			res.Error = err.Error()
			c.Log.Err(err).Str("ws", conn.RemoteAddr().String()).Msg("Request Error")
			// panic(err)
			// TODO: send reply to originator channel only !
		} else {
			res.Message = msg
		}
		// encoded result message
		respData, _ := c.encodeJSON(res)
		// respData, err := json.Marshal(res)
		// if err != nil {
		// 	res.Error = err.Error()
		// 	res.Message = nil
		// 	respData, _ = json.Marshal(res)
		// }

		// broadcast to sibling connection(s)
		broadcast := func() {

			if res.Error != "" {
				// Just respond with NO broadcast
				_ = c.sendFrame(conn, websocket.TextMessage, respData)
				return
			}
			// Push history ...
			c.pushMessage(msg)
			// Send response ...
			_ = c.sendFrame(conn, websocket.TextMessage, respData)

			// encoded notify message
			var noteData []byte
			for i := len(c.conn)-1; i >= 0; i-- {
				peer := c.conn[i]
				if peer == conn {
					// c.sendFrame(conn, websocket.TextMessage, resultData)
					continue // self
				}
				if len(noteData) == 0 {
					update := webChatResponse{
						Message: msg,
					}
					// noteData, _ = json.Marshal(update)
					noteData, _ = c.encodeJSON(update)
				}
				_ = c.sendFrame(peer, websocket.TextMessage, noteData)
			}
		}

		select {
		case c.send <- broadcast:
		default:
			c.Log.Warn().Msg("Broadcast to closed(c.send) channel")
			return
		}
	}
}

// webChat room WRITEr routine (multiplexor)
func (c *webChat) writePump() {
	// Send PINGs to peer with this period.
	// Must be less than c.Bot.ReadTimeout.
	pingInterval := (c.Bot.ReadTimeout * 9) / 10;
	pingTracker := time.NewTicker(pingInterval)
	
	defer func() {
		pingTracker.Stop()
		// // c.Conn.Close()
		// for _, conn := range c.conn {
		// 	conn.Close()
		// }
		// if len(c.conn) == 0 {
			c.Bot.RWMutex.Lock()   // +RW
			found := (c == c.Bot.chat[c.ChatID])
			if found {
				delete(c.Bot.chat, c.ChatID)
			}
			c.Bot.RWMutex.Unlock() // -RW
			// Ensure service closed this chat !
			if c.Channel.Closed == 0 {
				_ = c.Channel.Close()
			}
			if found {
				c.Log.Warn().Msg("[WS] <<< STOP >>>")
			}
		// }
		// c.Log.Warn().Msg("[WS] >>> Shutdown <<<")
	} ()

	c.Log.Info().Msg("[WS] >>> START <<<")

	for {
		select {
		case send, ok := <- c.send:
			if !ok {
				// NOTE: (send == nil)
				for i := 0; i < len(c.conn); i++ {
					conn := c.conn[i]
					_ = conn.SetWriteDeadline(
						time.Now().Add(c.Bot.WriteTimeout),
					)
					err := conn.WriteMessage(
						websocket.CloseMessage, // []byte{},
						websocket.FormatCloseMessage(
							websocket.CloseNormalClosure,
							"BYE",
						),
					)
					if err != nil {
						c.Log.Err(err).
							Str("ws", conn.RemoteAddr().String()).
							Msg("WRITE.Close(!)")
					} else {
						c.Log.Warn().
							Str("ws", conn.RemoteAddr().String()).
							Msg("WRITE.Close(!)")
					}
					defer conn.Close()
				}
				c.conn = c.conn[:0]
				c.closed = true
				break // select
			}
			// sync
			send()
		
		case <-pingTracker.C:

			for i := len(c.conn)-1; i >= 0; i-- {
				conn := c.conn[i]
				_ = conn.SetWriteDeadline(time.Now().Add(c.Bot.WriteTimeout))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					// [sync] remove: bad connection ...
					c.conn = append(c.conn[0:i], c.conn[i+1:]...)
					c.Log.Err(err).
						Str("ws", conn.RemoteAddr().String()).
						Msg("PING")
				} else {
					c.Log.Debug().
						Str("ws", conn.RemoteAddr().String()).
						Msg("PING")
				}
			}
			// Next PING: no connections !..
			// Force close this chat !
			if len(c.conn) == 0 {
				// _ = c.Channel.Close()
				// go c.Channel.Close()
				// continue // Gracefully shutdown this chat room !
				c.closed = true
				break // select
			}
		}

		if c.closed && len(c.conn) == 0 {
			break // for
		}
	}
}

// sendFrame writes given frame message data to single conn
func (c *webChat) sendFrame(conn *websocket.Conn, typeof int, data []byte) (err error) {
	
	defer func() {
		if err != nil {
			for i := 0; i < len(c.conn); i++ {
				if c.conn[i] == conn {
					c.conn = append(c.conn[0:i], c.conn[i+1:]...)
					break
				}
			}
			conn.Close() // FIXME: will catch on .readPump(?)
		}
	} ()

	err = conn.SetWriteDeadline(time.Now().Add(c.Bot.WriteTimeout))
	
	if err != nil {
		c.Log.Err(err).
			Str("ws", conn.RemoteAddr().String()).
			Msg("WS.SetWriteDeadline(!)")
		return // err
	}
	// if !ok {
	// 	// The hub closed the channel.
	// 	conn.WriteMessage(websocket.CloseMessage, []byte{})
	// 	return
	// }

	var w io.WriteCloser
	w, err = conn.NextWriter(typeof)
	if err != nil {
		// if err == websocket.ErrCloseSent {}
		c.Log.Err(err).
			Str("ws", conn.RemoteAddr().String()).
			Msg("WS.NextWriter(!)")
		return // err
	}

	_, err = w.Write(data)
	if err != nil {
		c.Log.Err(err).
			Str("ws", conn.RemoteAddr().String()).
			Msg("WS.Write(!)")
		return // err
	}

	// // Add queued chat messages to the current websocket message.
	// n := len(c.send)
	// for i := 0; i < n; i++ {
	// 	w.Write(newline)
	// 	w.Write(<-c.send)
	// }

	err = w.Close()
	if err != nil {
		c.Log.Err(err).
			Str("ws", conn.RemoteAddr().String()).
			Msg("WS.Flush(!)")
		return // err
	}

	c.Log.Debug().
			Str("ws", conn.RemoteAddr().String()).
			Str("data", string(data)).
			Msg("WS.Write(!)")

	return // nil
}

// encodes given message m to JSON
// using prepared buffer for writing
// [MUST]: Be SYNC; Protect call with c.Bot.send <- func() { /*ONLY!*/ }
func (c *webChat) encodeJSON(m interface{}) (data []byte, ok bool) {
	
	buf := &c.wbuf
	buf.Reset()
	
	enc := json.NewEncoder(buf)
	err := enc.Encode(m)

	// Marshal: +OK
	if err == nil {
		return buf.Bytes(), true
	}
	
	// Marshal: -ERR
	res := webChatResponse{}

	switch obj := m.(type) {
	case webChatResponse:
		res.Id = obj.Id
	case *webChatResponse:
		res.Id = obj.Id
	}
	
	res.Error = err.Error()
	
	buf.Reset()
	_ = enc.Encode(res)
	
	return buf.Bytes(), false
}

// broadcast given message m to all peer connections
// [MUST]: Be SYNC; Protect call with c.Bot.send <- func() { /*ONLY!*/ }
func (c *webChat) broadcast(m interface{}) {
	// err := chat.Conn.WriteJSON(update)
	
	// Encode JSON once for all recepients ...
	// jsonb, _ := json.Marshal(e)
	jsonb, _ := c.encodeJSON(m)

	for i := len(c.conn)-1; i >= 0; i-- {
		_ = c.sendFrame(c.conn[i], websocket.TextMessage, jsonb)
		// if err != nil {
		// 	// NOTE: removes c.conn[i], but we are moving backwards !
		// }
	}
}

func init() {
	// Register "webchat" provider ...
	bot.Register(providerWebChat, NewWebChatBot)
}