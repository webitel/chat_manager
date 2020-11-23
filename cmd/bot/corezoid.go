package main

import (

	"sync"
	"time"

	"bytes"
	"context"
	"strconv"
	"net/http"
	// "io/ioutil"

	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/golang/protobuf/proto"
	merror "github.com/micro/go-micro/v2/errors"

	pb "github.com/webitel/protos/bot"
	pbchat "github.com/webitel/protos/chat"

	"github.com/rs/zerolog/log"
)

// chat request: command/message
type corezoidIncome struct {

	ChatID    string    `json:"id,omitempty"`          // [required] chat.channel.user.id
	Channel   string    `json:"channel,omitempty"`     // [required] underlaying provider name e.g.: telegram, viber, messanger (facebook), skype, slack

	Date      time.Time `json:"-"`                     // [internal] received local timestamp
	Type      string    `json:"action,omitempty"`      // [required] command request !
	Test      bool      `json:"test,omitempty"`        // [optional] bot development indicatior ! TOBE: removed in production !

	From      string    `json:"client_name,omitempty"` // [required] chat.username; remote::display
	Text      string    `json:"text,omitempty"`        // [optional] message text
	// {action:"purpose"} arguments
	ReplyWith string    `json:"replyTo,omitempty"`     // [optional] reply with back-channel type e.g.: chat (this), email etc.
}

// chat response: reply/event/message
type corezoidOutcome struct {
	 // outcome: response
	 Date      time.Time `json:"-"`                         // [internal] sent local timestamp
	 // {action:"chat"} => oneof {replyAction:(startChat|closeChat|answerToChat)} else ignore
	 Type      string    `json:"replyAction,omitempty"`     // [optional] update event type; oneof (startChat|closeChat|answerToChat)
	 From      string    `json:"operator,omitempty"`        // [required] chat.username; local::display
	 Text      string    `json:"answer,omitempty"`          // [required] message text payload
}

type corezoidChat struct {
	 // latest income message
	 corezoidIncome
	 // corresponding reply message
	 corezoidOutcome
}

// Corezoid Bot runtime
type corezoidBot struct {
	profile   *pbchat.Profile // config
	URI       string // this chat-bot back-channel service URL (host::proxy)
	log       zerolog.Logger
	client    pbchat.ChatService
	// runtime cache
	chatMx    sync.RWMutex
	chat      map[string]*corezoidChat
}

func ConfigureCorezoid(profile *pbchat.Profile, client pbchat.ChatService, log *zerolog.Logger) ChatBot {
	//token, ok := profile.Variables["token"]
	//if !ok {
	//	b.log.Fatal().Msg("token not found")
	//	return nil
	//}
	url, ok := profile.Variables["url"]
	if !ok {
		log.Fatal().Msg("url not found")
		return nil
	}
	return &corezoidBot{

		profile:   proto.Clone(profile).(*pbchat.Profile),
		URI:       url,
		log:       log.With().

			Int64("pid", profile.Id).
			Str("gate", "corezoid").
			Str("bot", "АТБ chat-bot").
			Str("uri", "/" + profile.UrlId).
		
		Logger(),
		client:    client,

		chat:      make(map[string]*corezoidChat, 64),
	}
}

func (bot *corezoidBot) DeleteProfile() error {
	return nil
}

// SendMessage implements bot.Sender interface
func (bot *corezoidBot) SendMessage(req *pb.SendMessageRequest) error {

	var (

		chatID = req.GetExternalUserId()
		update = req.GetMessage()
		localtime = time.Now()
	)

	// region: try to get chat latest state
	channel := bot.get(chatID)
	if channel == nil || channel.ChatID != chatID {
		// TODO: preload from persistent db store

		bot.log.Error().
			Str("chat-id", chatID).
			Str(zerolog.ErrorFieldName, "chat: no runtime context found").
		Msg("Failed to send update")

		return merror.NotFound(
			"webitel.chat.send.not_found",
			"chat: channel id:%s: no runtime context found",
			 chatID,
		)
	}
	// endregion

	// var reply corezoidReply
	// // populate channel request context
	// reply.corezoidUpdate = state.Current
	
	// reply.ID       = chatID
	// reply.Text     = req.GetMessage().GetVariables()["text"]
	// reply.Action   = req.GetMessage().GetVariables()["action"]
	// reply.Channel  = req.GetMessage().GetVariables()["channel"]
	// reply.ReplyTo  = req.GetMessage().GetVariables()["replyTo"]

	// prepare channel response details
	ctx := corezoidChat{ // shallowcopy value(s)
		corezoidIncome: channel.corezoidIncome,
	}
	reply := &ctx.corezoidOutcome
	reply.Date = localtime

	switch channel.corezoidIncome.Type {
	case "chat": // chatting
		reply.Type = "answerToChat" // replyAction = startChat|closeChat|answerToChat
		reply.From = "manager" // TODO: resolve sender name
		reply.Text = update.GetText() // reply: message text
	default: // {"action":"purpose"}
		reply.From = "manager" // TODO: resolve sender name
		reply.Text = update.GetText() // reply: message text
	}

	// encode result body
	body, err := json.Marshal(ctx)
	if err != nil {
		// 500 Failed to encode update request
		return err
	}

	corezoidReq, err := http.NewRequest(http.MethodPost, bot.URI, bytes.NewReader(body))
	if err != nil {
		return err
	}
	corezoidReq.Header.Set("Content-Type", "application/json")
	//corezoidReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", profile.token))

	res, err := http.DefaultClient.Do(corezoidReq)
	if err != nil {
		return err
	}
	// _, err = ioutil.ReadAll(corezoidRes.Body)
	
	code := res.StatusCode
	if 200 <= code && code < 300 {
		// Success (!)
		// store latest context response
		// adjust := channel.corezoidOutcome // continuation for latest reply message -if- !adjust.Date.IsZero()
		channel.corezoidOutcome = *(reply) // shallowcopy
	}
	
	return nil
}

// Handler implementes bot.Receiver interface
func (bot *corezoidBot) Handler(w http.ResponseWriter, r *http.Request) {
	// internal, machine-readable chat channel contact (string: profile ID)
	contact := strconv.FormatInt(bot.profile.Id, 10)

	var (

		// update corezoidUpdate // payload
		update corezoidIncome // command/message
		localtime = time.Now() // timestamp
	)

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Error().Err(err).Msg("Failed to decode update request")
		err = errors.Wrap(err, "Failed to decode update request")
		http.Error(w, err.Error(), http.StatusBadRequest) // 400 
		return
	}

	if update.ChatID == "" {
		log.Error().Msg("Got request with no chat.id; ignore")
		http.Error(w, "request: chat.id required but missing", http.StatusBadRequest) // 400
		return
	}

	update.Date = localtime

	// region: runtime state update
	// state := bot.get(update.ChatID)
	// if state == nil {
	// 	state = &corezoidChat{
	// 		corezoidIncome: update, // as latest
	// 	}
	// } else {
	// 	// TODO: re-init, continue but new context
	// 	state.corezoidIncome = update // shallowcopy
	// 	state.corezoidOutcome
	// }
	state := &corezoidChat{
		corezoidIncome: update, // as latest
		// corezoidOutcome: {} // NULLify
	}
	bot.set(state) // store runtime state
	// endregion

	bot.log.Debug().

		Str("chat-id", update.ChatID).
		Str("channel", update.Channel).
		Str("action",  update.Type).
		Str("replyTo", update.ReplyWith).
		Str("text",    update.Text).

	Msg("RECV update")

	switch update.Type {
	case "chat": // incoming chat request (!)
	default: // "Пропозиція", "Предложение" // FIXME: request non-localized (!)
	}

	strChatID := update.ChatID //strconv.FormatInt(update.ID, 10)

	check := &pbchat.CheckSessionRequest{
		ExternalId: strChatID,
		ProfileId:  bot.profile.Id,
		// Username:   update.Message.From.Username,
	}
	resCheck, err := bot.client.CheckSession(context.Background(), check)
	if err != nil {
		bot.log.Error().Msg(err.Error())
		return
	}
	bot.log.Debug().
		Bool("new", !resCheck.Exists).
		Str("channel_id", resCheck.ChannelId).
		Int64("client_id", resCheck.ClientId).
		Msg("CHAT Channel")

	if !resCheck.Exists {

		// region: init chat-flow-routine /start message environment variables
		env := map[string]string {
			"action":    update.Type,
			"channel":   update.Channel,
		}

		// HERE: passthru command-specific arguments ...
		switch update.Type {
		case "Предложение":

			env["replyTo"] = update.ReplyWith
		
		case "chat":
			// ...
		}
		// endregion

		start := &pbchat.StartConversationRequest{
			DomainId: bot.profile.DomainId,
			Username: check.Username,
			User: &pbchat.User{
				UserId:     resCheck.ClientId,
				Type:       update.Channel, // "telegram", // FIXME: why (?)
				Connection: contact, // contact: profile.ID
				Internal:   false,
			},
			Message: &pbchat.Message{
				Type: "text",
				Value: &pbchat.Message_Text{
					Text: update.Text,
				},
				Variables: env,
			},
		}

		_, err := bot.client.StartConversation(context.Background(), start)
		if err != nil {
			bot.log.Error().Msg(err.Error())
			return
		}

	} else {

		message := &pbchat.SendMessageRequest{
			// Message:   textMessage,
			AuthUserId: resCheck.ClientId,
			ChannelId:  resCheck.ChannelId,
		}
		messageText := &pbchat.Message{
			Type: "text",
			Value: &pbchat.Message_Text{
				Text: update.Text,
			},
			// FIXME: does we need this here ? 
			// NOTE: processing consequent message(s) ...
			Variables: map[string]string {
				"action":  update.Type,
				"channel": update.Channel,
				"replyTo": update.ReplyWith,
			},
		}
		message.Message = messageText
		// }

		_, err := bot.client.SendMessage(context.Background(), message)
		if err != nil {
			bot.log.Error().Msg(err.Error())
		}
	}

	// // QUICK TEST
	// response := *(update) // shallowcopy
	// response.Answer = update.Text // ECHO
	// // FIXME: relpy here ?
	// w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// err = json.NewEncoder(w).Encode(response)
	// if err != nil {
	// 	panic(errors.Wrap(err, "Failed to write response"))
	// }
	
	// go corezoidDelayedResponse(b, *(update), time.Second * 10)
}

// get returns latest runtime chat state by given chatID
func (bot *corezoidBot) get(chatID string) *corezoidChat {

	bot.chatMx.RLock()
	state := bot.chat[chatID]
	bot.chatMx.RUnlock()

	return state
}

// set stores given state as a latest runtime chat state
func (bot *corezoidBot) set(ctx *corezoidChat) {

	id := ctx.ChatID

	bot.chatMx.Lock()
	bot.chat[id] = ctx
	bot.chatMx.Unlock()
}