package gotd

import (
	"context"
	"time"

	"github.com/go-faster/errors"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tgerr"
	backup "github.com/webitel/chat_manager/bot/telegram/gotd/internal/storage"
	protowire "google.golang.org/protobuf/proto"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

/*/ termAuth implements authentication via terminal.
type sessionAuth struct {
	noSignUp

	phoneNumber  string
	logoutTokens [][]byte // up to 20

	stage interface{}
	// nil - Unauthorized
	// string - Phone
	// *tg.AuthSentCode = Code sent; await user action
	// telegram.ErrPasswordAuthNeeded = Code verified; await 2FA password
	// *tg.User = Authorized; OK
	complete chan interface{}
}

type (
	authorizationPhone string
	authorizationCode  string
	authorization2FA   string
)

func (c sessionAuth) run(ctx context.Context) {
	select {
	case stage := <-c.complete:
		switch stage.(type) {
		case authorizationPhone:

		case authorizationCode:

		case authorization2FA:

		}
	case <-ctx.Done():
	}
}

func (c sessionAuth) Phone(ctx context.Context) (string, error) {
	if c.phoneNumber != "" {
		return c.phoneNumber, nil
	}

	if c.phoneNumber == "" {
		fmt.Print("[TELEGRAM] Enter phone number: ")
		phone, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", err
		}
		c.phoneNumber = strings.TrimSpace(phone)
	}
	return c.phoneNumber, nil
}

// type authorizationCode string

func (a sessionAuth) Code(ctx context.Context, req *tg.AuthSentCode) (string, error) {

	fmt.Print("[TELEGRAM] Enter code: ")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (a sessionAuth) Password(_ context.Context) (string, error) {
	fmt.Print("[TELEGRAM] Enter 2FA password: ")
	bytePwd, err := term.ReadPassword(0)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytePwd)), nil
}

var defaultAuth auth.Flow = auth.NewFlow(
	// auth.Env("USER_", auth.CodeAuthenticatorFunc(
	// 	func(ctx context.Context, state *tg.AuthSentCode) (string, error) {
	// 		fmt.Print("Enter code: ")
	// 		code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	// 		if err != nil {
	// 			return "", err
	// 		}
	// 		return strings.TrimSpace(code), nil
	// 	},
	// )),
	termAuth{
		phone: os.Getenv("USER_PHONE"),
	},
	auth.SendCodeOptions{
		// AllowFlashCall allows phone verification via phone calls.
		AllowFlashCall: false,
		// Pass true if the phone number is used on the current device.
		// Ignored if AllowFlashCall is not set.
		CurrentNumber: false,
		// If a token that will be included in eventually sent SMSs is required:
		// required in newer versions of android, to use the android SMS receiver APIs.
		AllowAppHash: false,
	},
)
*/

type sessionAuth struct {
	// ctor
	*session
	// opts
	// sync.Mutex        // guard
	// rxURI string // redirectURI
	phone string // current phone number
	// state
	requestAt time.Time
	request   *tg.AuthSentCode // login: await verification code
	sessionAt time.Time        // authenticatedAt timestamp
	// session   *tg.AuthAuthorization // successful login session
	tokens [][]byte // future login tokens
	user   *tg.User // loggedIn

	notify []chan *tg.User
	// err error
}

func newSessionAuth(c *session) *sessionAuth {

	login := &sessionAuth{
		session: c,
	}

	// // Fetch user info.
	// // me, err := app.Self(ctx)
	// me, err := peers.Self(context.TODO())
	// if err == nil {
	// 	login.user = me.Raw()
	// }

	// state = err
	return login
}

func (c *sessionAuth) Lock() {
	c.session.sync.Lock()
}

func (c *sessionAuth) Unlock() {
	c.session.sync.Unlock()
}

func (c *sessionAuth) User() *tg.User {

	c.Lock()
	defer c.Unlock()

	return c.user
}

func (c *sessionAuth) Auth(user *tg.User) {
	c.Lock()
	defer c.Unlock()
	c.doAuth(user)
}

func (c *sessionAuth) doAuth(user *tg.User) {
	if user != nil && !user.Self {
		panic("auth.Authorization.user!self")
	}
	self := c.user // session User
	c.user = user  // current User
	// notify -if- changed !
	if self.GetID() != user.GetID() {
		c.signal()
	}
}

func (c *sessionAuth) API() *tg.Client {
	return c.session.App.Client.API()
}

// restore .logoutTokens dataset
func (c *sessionAuth) restore(data []byte) error {

	if len(data) == 0 {
		return nil
	}

	// Decode state ...
	var state backup.Login
	err := protowire.Unmarshal(data, &state)
	if err != nil {
		return err
	}

	const max = 20
	n := len(state.Tokens)
	if n > max {
		n = max
	}

	if n == 0 {
		return nil
	}

	c.Lock()
	defer c.Unlock()
	// Count of available slots ?
	m := max - len(c.tokens)
	if m < n {
		n = m
	}

	c.tokens = append(c.tokens,
		state.Tokens[0:n]...,
	)

	return nil
}

// backup .logoutTokens dataset
func (c *sessionAuth) backup() ([]byte, error) {

	var tokens [][]byte

	c.Lock()
	if n := len(c.tokens); n != 0 {
		tokens = make([][]byte, n)
		copy(tokens, c.tokens)
	}
	c.Unlock()

	if len(tokens) == 0 {
		return nil, nil
	}

	// Encode state ...
	data, err := protowire.Marshal(
		&backup.Login{Tokens: tokens},
	)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func authSentCodeClassParser(sentCode tg.AuthSentCodeClass) (*tg.AuthSentCode, error) {
	parsedCode, ok := sentCode.(*tg.AuthSentCode)
	if !ok {
		return nil, errors.New("error asserting AuthSentCodeClass into AuthSentCode")
	}
	return parsedCode, nil
}

// SendCode sends the verification code for login
func (c *sessionAuth) SendCode(ctx context.Context, phone string) (*tg.AuthSentCode, error) {

	c.Lock()
	defer c.Unlock()
	// Latest attempt timestamp
	c.requestAt = time.Now()
	if c.request != nil {
		// TODO: resendCode
		request, err := c.API().AuthResendCode(ctx,
			&tg.AuthResendCodeRequest{
				PhoneNumber:   phone,
				PhoneCodeHash: c.request.PhoneCodeHash,
			},
		)
		if err != nil {
			// https://core.telegram.org/method/auth.resendCode#possible-errors
			if re, is := tgerr.As(err); is {
				switch re.Type {
				case tg.ErrPhoneCodeExpired: // "PHONE_CODE_EXPIRED": // The phone code you provided has expired.
				case tg.ErrPhoneCodeHashEmpty: // "PHONE_CODE_HASH_EMPTY": // phone_code_hash is missing.
				case tg.ErrPhoneNumberInvalid: // "PHONE_NUMBER_INVALID": // The phone number is invalid.
				case tg.ErrSendCodeUnavailable: //  "SEND_CODE_UNAVAILABLE": // Returned when all available options for this type of number
					// were already used (e.g. flash-call, then SMS, then this error might be returned to trigger a second resend).
				}
			}
			return nil, err
		}
		c.phone = phone
		
		requestPtr, err := authSentCodeClassParser(request)
		if err != nil {
			return nil, err
		}

		c.request = requestPtr
		return requestPtr, nil
	}

	sendCode := &tg.AuthSendCodeRequest{
		APIID:       c.session.App.apiId,
		APIHash:     c.session.App.apiHash,
		PhoneNumber: phone,
		// Settings: tg.CodeSettings{
		// 	AllowFlashcall:  false,
		// 	CurrentNumber:   false,
		// 	AllowAppHash:    false,
		// 	AllowMissedCall: false,
		// 	LogoutTokens:    c.tokens,
		// },
	}
	if tokens := c.tokens; len(tokens) != 0 {
		sendCode.Settings.SetLogoutTokens(tokens)
	}
	// PERFORM: auth.sendCode
	sentCode, err := c.API().AuthSendCode(
		ctx, sendCode,
	)

	if err != nil {
		// https://core.telegram.org/method/auth.sendCode#possible-errors
		if re, is := tgerr.As(err); is {
			switch re.Type {
			case tg.ErrAPIIDInvalid: // "API_ID_INVALID": // API ID invalid.
			case tg.ErrAPIIDPublishedFlood: // "API_ID_PUBLISHED_FLOOD": // This API id was published somewhere, you can't use it now.
			case "AUTH_RESTART": // Restart the authorization process.
			case tg.ErrPhoneNumberAppSignupForbidden: // "PHONE_NUMBER_APP_SIGNUP_FORBIDDEN": // You can't sign up using this app.
			case tg.ErrPhoneNumberBanned: // "PHONE_NUMBER_BANNED": // The provided phone number is banned from telegram.
			case tg.ErrPhoneNumberFlood: // "PHONE_NUMBER_FLOOD": // You asked for the code too many times.
			case tg.ErrPhoneNumberInvalid: // "PHONE_NUMBER_INVALID": // The phone number is invalid.
			case tg.ErrPhonePasswordFlood: // "PHONE_PASSWORD_FLOOD": // You have tried logging in too many times.
			case tg.ErrPhonePasswordProtected: // "PHONE_PASSWORD_PROTECTED": // This phone is password protected.
			case tg.ErrSMSCodeCreateFailed: // "SMS_CODE_CREATE_FAILED": // An error occurred while creating the SMS code.
			}
		}
		return nil, err
	}
	
	c.phone = phone

	sentCodePtr, err := authSentCodeClassParser(sentCode)
	if err != nil {
		return nil, err
	}
	c.request = sentCodePtr

	return sentCodePtr, nil
}

// Cancel the login verification code
func (c *sessionAuth) CancelCode(ctx context.Context) error {

	c.Lock()
	defer c.Unlock()

	if c.request == nil {
		return nil // DO Nothing !
	}

	_, _ = c.API().AuthCancelCode(ctx,
		&tg.AuthCancelCodeRequest{
			PhoneNumber:   c.phone,
			PhoneCodeHash: c.request.PhoneCodeHash,
		},
	)

	// Make idempotent !
	// _, err := c.api.AuthCancelCode(ctx, ...
	// if err != nil {
	// 	// https://core.telegram.org/method/auth.cancelCode#possible-errors
	// 	if re, is := tgerr.As(err); is {
	// 		switch re.Type {
	// 		case tg.ErrPhoneCodeExpired: // "PHONE_CODE_EXPIRED": // The phone code you provided has expired.
	// 		case tg.ErrPhoneNumberInvalid: // "PHONE_NUMBER_INVALID": // The phone number is invalid.
	// 		}
	// 	}
	// 	return err
	// }

	c.request = nil
	return nil
	// panic("not implemented")
}

// loginSession checks that `res` is *tg.AuthAuthorization and returns authorization result or error.
func loginSession(res tg.AuthAuthorizationClass) (*tg.AuthAuthorization, error) {
	switch res := res.(type) {
	case *tg.AuthAuthorization:
		return res, nil // ok
	case *tg.AuthAuthorizationSignUpRequired:
		return nil, &auth.SignUpRequired{
			TermsOfService: res.TermsOfService,
		}
	default:
		return nil, errors.Errorf("got unexpected response %T", res)
	}
}

// SignIn performs sign in with provided login code.
func (c *sessionAuth) SignIn(ctx context.Context, code string) (*tg.AuthAuthorization, error) {

	c.Lock()
	defer c.Unlock()

	if c.request == nil {
		// auth.sendCode() NOT performed !
		return nil, &tgerr.Error{
			Code:    500,
			Type:    "AUTH_RESTART",
			Message: "Restart the authorization process",
		}
	}

	res, err := c.API().AuthSignIn(ctx,
		&tg.AuthSignInRequest{
			PhoneNumber:   c.phone,
			PhoneCodeHash: c.request.PhoneCodeHash,
			PhoneCode:     code,
		},
	)

	if err != nil {
		// https://core.telegram.org/method/auth.signIn#possible-errors
		if re, is := tgerr.As(err); is {
			switch re.Type {
			case tg.ErrPhoneCodeEmpty: // "PHONE_CODE_EMPTY": // (400) phone_code is missing.
			case tg.ErrPhoneCodeExpired: // "PHONE_CODE_EXPIRED": // (400) The phone code you provided has expired.
			case tg.ErrPhoneCodeInvalid: // "PHONE_CODE_INVALID": // (400) The provided phone code is invalid.
			case tg.ErrPhoneNumberInvalid: // "PHONE_NUMBER_INVALID": // (406) The phone number is invalid.
			case tg.ErrPhoneNumberOccupied: // "PHONE_NUMBER_UNOCCUPIED": // (400) The phone number is not yet being used.
			case "SIGN_IN_FAILED": // (500) Failure while signing in.
			// https://core.telegram.org/api/auth#2fa
			case "SESSION_PASSWORD_NEEDED":
				// return nil, auth.ErrPasswordAuthNeeded
			}
		}
		return nil, err
	}

	authZ, err := loginSession(res)

	// if errors.Is(err, auth.ErrPasswordAuthNeeded) {
	// 	password, err := f.Auth.Password(ctx)
	// 	if err != nil {
	// 		return errors.Wrap(err, "get password")
	// 	}
	// 	if _, err := client.Password(ctx, password); err != nil {
	// 		return errors.Wrap(err, "sign in with password")
	// 	}
	// 	return nil
	// }

	if err != nil {
		return nil, err
	}

	c.request = nil          // Code used !
	c.sessionAt = time.Now() // Authenticated timestamp
	// c.user, _ = authZ.GetUser().AsNotEmpty()
	// c.signal() // LOCKED
	user, _ := authZ.GetUser().AsNotEmpty()
	// LOCKED
	c.doAuth(user)

	return authZ, nil
}

// Password performs login via secure remote password (aka 2FA).
func (c *sessionAuth) Password(ctx context.Context, password string) (*tg.AuthAuthorization, error) {

	c.Lock()
	defer c.Unlock()

	p, err := c.API().AccountGetPassword(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get SRP parameters")
	}

	a, err := auth.PasswordHash([]byte(password), p.SRPID, p.SRPB, p.SecureRandom, p.CurrentAlgo)
	if err != nil {
		return nil, errors.Wrap(err, "compute password hash")
	}

	res, err := c.API().AuthCheckPassword(ctx,
		&tg.InputCheckPasswordSRP{
			SRPID: p.SRPID,
			A:     a.A,
			M1:    a.M1,
		},
	)

	if err != nil {
		// https://core.telegram.org/method/auth.checkPassword#possible-errors
		if re, is := tgerr.As(err); is {
			switch re.Type {
			case tg.ErrPasswordHashInvalid: // (400) The provided password hash is invalid.
				return nil, auth.ErrPasswordInvalid
			case tg.ErrSRPIDInvalid: // (400) Invalid SRP ID provided.
			case tg.ErrSRPPasswordChanged: // (400) Password has changed.
			}
		}
		return nil, errors.Wrap(err, "check password")
	}

	authZ, err := loginSession(res)

	if err != nil {
		return nil, errors.Wrap(err, "check")
	}

	c.request = nil          // Code used !
	c.sessionAt = time.Now() // Authenticated timestamp
	// c.user, _ = authZ.GetUser().AsNotEmpty()
	// c.signal() // LOCKED
	user, _ := authZ.GetUser().AsNotEmpty()
	// LOCKED
	c.doAuth(user)

	return authZ, nil
}

// LogOut logs out the user.
func (c *sessionAuth) LogOut(ctx context.Context) error {

	c.Lock()
	defer c.Unlock()

	res, err := c.API().AuthLogOut(ctx)
	// var (
	// 	err error
	// 	res = &tg.AuthLoggedOut{
	// 		FutureAuthToken: []byte("VsDfV2s78eq"),
	// 	}
	// )

	if err != nil {
		// https://core.telegram.org/method/auth.logOut#possible-errors
		// if re, is := tgerr.As(err); is {
		// 	switch re.Type {
		// 	case *tgerr.Error: // (400)
		// 	}
		// }
		return err
	}

	if len(res.FutureAuthToken) != 0 {
		//
		// https://core.telegram.org/api/auth#logout-tokens
		//
		// When invoking auth.logOut on a previously authorized session with 2FA enabled,
		// the server may return a future_auth_token, which should be stored in the local database.
		//
		// At all times, the logout token database should contain at most 20 tokens:
		// evict older tokens as new tokens are added.
		//
		const max = 20 // max
		tokens := c.tokens
		if tokens == nil {
			tokens = make([][]byte, 0, max)
		}
		n := len(tokens)
		if n < max {
			tokens = append(tokens, nil) // extend
		} else { // n == max
			n-- // evict oldest for new one !
		}
		if n != 0 {
			_ = copy(tokens[1:], tokens[0:n])
		}
		// push to front
		tokens[0] = res.FutureAuthToken
		c.tokens = tokens
	}
	// FIXME: Middleware will do it's job: AUTH_KEY_UNREGISTERED !
	// c.user = nil
	// c.signal()

	// LOCKED
	c.doAuth(nil)

	return nil
}

// signal state LOCKED !
func (c *sessionAuth) signal() {
	for sub, notify := range c.notify {
		select {
		case notify <- c.user: // sent; OK
		default: // busy (full); unsubscribe
			c.notify = append(c.notify[0:sub], c.notify[sub+1:]...)
			close(notify)
			panic("onAuthorizationState: blocked; unsubscribe")
		}
	}
}

func (c *sessionAuth) init() {
	// Fetch user info.
	// me, err := app.Self(ctx)
	self, err := c.peers.Self(context.TODO())
	if err != nil {
		if tgerr.IsCode(err, 401) {
			err = nil // Ignore: UNAUTHORIZED
		} else {
			c.session.log.Error(err.Error()) // LOG: getSelf ERROR
		}
	}

	// c.Lock()
	// defer c.Unlock()

	// c.resetUser(self.Raw())
	c.Auth(self.Raw())

	// sessionUser := c.user
	// currentUser := self.Raw()
	// c.user = currentUser

	// if currentUser.GetID() != sessionUser.GetID() {
	// 	c.signal() // notify subscribers
	// }
}

// Subscribe for authorizationState changes
// Trigger:
// - non <nil> *tg.User reference is case of session new Login
// - <nil> *tg.User reference in case of session Logout
func (c *sessionAuth) Subscribe() <-chan *tg.User {

	notify := make(chan *tg.User, 1) // buffered

	c.Lock()
	reinit := len(notify) == 0
	c.notify = append(c.notify, notify)
	c.Unlock()

	if reinit {
		c.init()
	}

	// if c.user != nil {
	// 	// Immediate if user authorized !
	// 	notify <- c.user
	// }

	return notify
}

func (c *sessionAuth) Unsubscribe(notify <-chan *tg.User) {

	var origin chan *tg.User

	c.Lock()

	for i, sub := range c.notify {
		if sub == notify {
			c.notify = append(c.notify[0:i], c.notify[i+1:]...)
			origin = sub
			break
		}
	}

	c.Unlock()

	if origin == nil {
		return // Subscription: NOT found !
	}

	close(origin)

	for {
		select {
		case _, ok := <-notify:
			if !ok {
				return // break for
			}
		default:
			return // break for
		}
	}
}

func (c *sessionAuth) MiddlewareHook(next tg.Invoker) telegram.InvokeFunc {
	return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
		// PERFORM request !
		err := next.Invoke(ctx, input, output)
		// Logout/Terminate session interception
		if c.user != nil && auth.IsUnauthorized(err) {

			c.Auth(nil)
			// c.Lock()

			// c.user = nil
			// c.signal()

			// c.Unlock()
		}
		// Operation error ?
		return err
	}
}
