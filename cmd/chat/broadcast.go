package chat

import (
	"context"
	"database/sql"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/micro/micro/v3/service/errors"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	pbmessages "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/internal/auth"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/util"
	"google.golang.org/genproto/googleapis/rpc/status"
)

// Broadcast - sending a message to a specific user (peer.id) from a specific gateway (peer.via).
// When sending a message, a separate conversation is always created in which the message and
// all channels are recorded and immediately closed. The user from whom the message is coming
// will usually have an earlier created_at. For unauthorized requests (BroadcastMessageNA),
// a bot message is used.
//
// The list of performers of different broadcast's logic:
// - simple - to send a message without saving it to the history;
// - socials - to send a message with a saved history to social networks such as telegram, facebook, ...;
// - portal - for the usual sending of a message to save to history for portal;

const (
	fileSourceFile  = "file"
	fileSourceMedia = "media"

	userTypeBot     = "bot"
	userTypeWebitel = "webitel"
)

var (
	validationRules = map[string]struct {
		id, via *regexp.Regexp
	}{
		"telegram": {
			id:  regexp.MustCompile(`^\d+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"gotd": {
			id:  regexp.MustCompile(`^\+?[1-9]\d{1,14}$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"viber": {
			id:  regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"facebook": {
			id:  regexp.MustCompile(`^\d+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"instagram": {
			id:  regexp.MustCompile(`^\d+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"messenger": {
			id:  regexp.MustCompile(`^\d+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"whatsapp": {
			id:  regexp.MustCompile(`^\d+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"vk": {
			id:  regexp.MustCompile(`^[a-zA-Z0-9\-_.,:;]+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"custom": {
			id:  regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()_+=\-\[\]{}|:;"'<>,.?]+$`),
			via: regexp.MustCompile(`^\d+$`),
		},
		"portal": {
			id:  regexp.MustCompile(`^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{12}$`),
			via: regexp.MustCompile(`^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{12}$`),
		},
	}
)

func (c *chatService) executeBroadcast(ctx context.Context, authUser *auth.User, req *pbmessages.BroadcastMessageRequest, resp *pbmessages.BroadcastMessageResponse) error {

	message, err := c.preparationMessage(req.GetMessage())
	if err != nil {
		return err
	}

	for _, peer := range req.GetPeers() {
		switch peer.GetType() {
		case "gotd":
			vars, fail := c.executeBroadcastSimple(ctx, peer, message)
			if fail != nil {
				resp.Failure = append(resp.Failure, fail)
			}

			resp.Variables = util.MargeMaps(resp.Variables, vars)

		case "telegram", "viber", "facebook", "messenger", "instagram", "whatsapp", "vk", "custom":
			vars, fail := c.executeBroadcastSocials(ctx, authUser, peer, message)
			if fail != nil {
				resp.Failure = append(resp.Failure, fail)
			}

			resp.Variables = util.MargeMaps(resp.Variables, vars)

		case "portal":
			fail := c.executeBroadcastPortal(ctx, authUser, peer, message)
			if fail != nil {
				resp.Failure = append(resp.Failure, fail)
			}
		}
	}

	return nil
}

func (c *chatService) preparationMessage(inputMessage *pbmessages.InputMessage) (*pbchat.Message, error) {
	message := mapInputMessageToMessage(inputMessage)

	file := message.GetFile()
	if file.GetId() > 0 {
		mediaType, params, err := mime.ParseMediaType(file.GetMime())
		if err != nil {
			return nil, errors.BadRequest(
				"message.file.mime.invalid",
				"broadcast: file.meme; mimetype is invalid",
			)
		}

		// NOTE: Set file media type 'unknown/unknown' by default
		if mediaType == "" {
			mediaType = "unknown/unknown"
		}

		// NOTE: Set file source by default
		if v, ok := params["source"]; !ok || v == "" {
			file.Mime = mime.FormatMediaType(mediaType, map[string]string{
				"source": fileSourceMedia,
			})
		}
	}

	return message, nil
}

func (c *chatService) executeBroadcastSimple(ctx context.Context, peer *pbmessages.InputPeer, message *pbchat.Message) (map[string]string, *pbmessages.BroadcastError) {
	from, _ := strconv.ParseInt(peer.GetVia(), 10, 64)

	resp, err := c.botClient.BroadcastMessage(ctx, &pbbot.BroadcastMessageRequest{
		Peer:    []string{peer.GetId()},
		From:    from,
		Message: message,
	})
	if err != nil {
		switch v := err.(type) {
		case *errors.Error:
			return nil, buildBroadcastError(peer.GetId(), v.Code, v.Detail)

		default:
			c.log.Error(
				"CALL Bots.BroadcastMessage IS FAILED",
				slog.String("peer.id", peer.GetId()),
				slog.String("peer.via", peer.GetVia()),
				slog.Any("error", err),
			)

			return nil, buildBroadcastInternalServerError(peer.GetId())
		}
	}

	if len(resp.Failure) > 0 {
		return nil, &pbmessages.BroadcastError{
			PeerId: resp.Failure[0].Peer,
			Error:  resp.Failure[0].Error,
		}
	}

	return resp.GetVariables(), nil
}

func (c *chatService) executeBroadcastSocials(ctx context.Context, authUser *auth.User, peer *pbmessages.InputPeer, message *pbchat.Message) (map[string]string, *pbmessages.BroadcastError) {

	client, err := c.repo.GetClientByExternalID(ctx, peer.GetId())
	if err != nil {
		c.log.Error(
			"CALL Repository.GetClientByExternalID IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
	}

	if client == nil {
		return nil, buildBroadcastNotFoundError(peer.GetId(), "peer is not found")
	}

	peerViaInt, _ := strconv.ParseInt(peer.GetVia(), 10, 64)

	chatBot, err := c.repo.GetChatBotByID(ctx, peerViaInt)
	if err != nil {
		c.log.Error(
			"CALL Repository.GetChatBotByID IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
	}

	if chatBot == nil {
		return nil, buildBroadcastNotFoundError(peer.GetId(), "gateway is not found")
	}

	if !chatBot.Enabled {
		return nil, buildBroadcastBadRequestError(peer.GetId(), "gateway is not enabled")
	}

	var (
		userID       int64  = chatBot.FlowID
		userType     string = userTypeBot
		userDomainID int64  = chatBot.DomainID
		userName     string = chatBot.Name
		userChatName string = chatBot.Name
	)

	if authUser != nil {
		wbtUser, err := c.repo.GetWebitelUserByID(ctx, authUser.ID, authUser.DomainID)
		if err != nil {
			return nil, buildBroadcastInternalServerError(peer.GetId())
		}

		if wbtUser == nil {
			return nil, buildBroadcastNotFoundError(peer.GetId(), "auth user is not found")
		}

		userID = wbtUser.ID
		userType = userTypeWebitel
		userDomainID = wbtUser.DomainID
		userName = wbtUser.Name
		userChatName = wbtUser.ChatName
	}

	createdAt := time.Now()
	closedAt := createdAt.Add(time.Millisecond * 6)
	variables := map[string]string{
		"cid":            strconv.FormatInt(client.ID, 10),
		"chat":           chatBot.Provider,
		"from":           client.Name.String,
		"user":           client.ExternalID.String,
		"externalChatID": client.ExternalID.String,
		"flow":           strconv.FormatInt(chatBot.FlowID, 10),
		"broadcast":      "true",
		// "needs_processing": "false",
	}

	conversation := pg.Conversation{
		DomainID:  userDomainID,
		Title:     client.Name,
		Variables: variables,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	from := pg.Channel{
		DomainID: userDomainID,
		UserID:   userID,
		Name:     userName,
		PublicName: sql.NullString{
			String: userChatName,
			Valid:  userChatName != "",
		},
		Type:     userType,
		Internal: true,
		Connection: sql.NullString{
			String: strconv.FormatInt(chatBot.ID, 10),
			Valid:  true,
		},
		Variables: variables,
		ClosedCause: sql.NullString{
			String: pbchat.CloseConversationCause_broadcast_end.String(),
			Valid:  true,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	to := pg.Channel{
		DomainID: userDomainID,
		UserID:   client.ID,
		Type:     client.Type.String,
		Name:     client.Name.String,
		Internal: false,
		Connection: sql.NullString{
			String: strconv.FormatInt(chatBot.ID, 10),
			Valid:  true,
		},
		Variables: variables,
		ClosedCause: sql.NullString{
			String: pbchat.CloseConversationCause_broadcast_end.String(),
			Valid:  true,
		},
		// must be a later value than sender, which is necessary for building a history
		CreatedAt: createdAt.Add(time.Millisecond * 4),
		// must be a later value than sender, which is necessary for building a history
		UpdatedAt: createdAt.Add(time.Millisecond * 4),
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	// For the bot to be the last in the members list
	if userType == userTypeWebitel {
		conversation.CreatedAt = to.CreatedAt.Add(2 * time.Millisecond)
	}

	err = c.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := c.repo.CreateConversationTx(ctx, tx, &conversation); err != nil {
			return err
		}
		from.ConversationID = conversation.ID
		to.ConversationID = conversation.ID

		if userType != userTypeBot {
			if err := c.repo.CreateChannelTx(ctx, tx, &from); err != nil {
				return err
			}
		}

		if err := c.repo.CreateChannelTx(ctx, tx, &to); err != nil {
			return err
		}

		fromApp := mapChannelToAppChannel(&from)
		if _, err := c.saveMessage(ctx, tx, fromApp, message); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		switch v := err.(type) {
		case *errors.Error:
			return nil, buildBroadcastError(peer.GetId(), v.Code, v.Detail)

		default:
			c.log.Error(
				"TRANSACTION IS FAILED",
				slog.String("peer.id", peer.GetId()),
				slog.String("peer.via", peer.GetVia()),
				slog.Any("error", err),
			)

			return nil, buildBroadcastInternalServerError(peer.GetId())
		}
	}

	resp, err := c.botClient.BroadcastMessage(ctx, &pbbot.BroadcastMessageRequest{
		Peer:    []string{peer.GetId()},
		From:    peerViaInt,
		Message: message,
	})
	if err != nil {
		switch v := err.(type) {
		case *errors.Error:
			return nil, buildBroadcastError(peer.GetId(), v.Code, v.Detail)

		default:
			c.log.Error(
				"CALL Bots.BroadcastMessage IS FAILED",
				slog.String("peer.id", peer.GetId()),
				slog.String("peer.via", peer.GetVia()),
				slog.Any("error", err),
			)

			return nil, buildBroadcastInternalServerError(peer.GetId())
		}
	}

	if len(resp.Failure) > 0 {
		return nil, &pbmessages.BroadcastError{
			PeerId: resp.Failure[0].Peer,
			Error:  resp.Failure[0].Error,
		}
	}

	return resp.GetVariables(), nil
}

func (c *chatService) executeBroadcastPortal(ctx context.Context, authUser *auth.User, peer *pbmessages.InputPeer, message *pbchat.Message) *pbmessages.BroadcastError {

	// Data normalization
	peer.Id = strings.ReplaceAll(peer.Id, "-", "")
	peer.Via = strings.ReplaceAll(peer.Via, "-", "")

	client, err := c.repo.GetClientByExternalID(ctx, peer.GetId())
	if err != nil {
		c.log.Error(
			"CALL Repository.GetClientByExternalID IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	if client == nil {
		return buildBroadcastNotFoundError(peer.GetId(), "peer is not found")
	}

	appUser, err := c.repo.GetPortalAppUser(ctx, peer.GetId(), peer.GetVia())
	if err != nil {
		c.log.Error(
			"CALL Repository.GetPortalAppUser IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	if appUser == nil {
		return buildBroadcastNotFoundError(peer.GetId(), "app user is not found")
	}

	schemaId, err := c.repo.GetPortalAppSchemaID(ctx, peer.GetVia())
	if err != nil {
		c.log.Error(
			"CALL Repository.GetPortalAppSchemaID IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	if schemaId == 0 {
		return buildBroadcastNotFoundError(peer.GetId(), "portal app schema is not found")
	}

	var (
		userID       int64  = schemaId
		userType     string = userTypeBot
		userDomainID int64  = appUser.DomainID
		userName     string = "Service"
		userChatName string = "Service"
	)

	if authUser != nil {
		wbtUser, err := c.repo.GetWebitelUserByID(ctx, authUser.ID, authUser.DomainID)
		if err != nil {
			c.log.Error(
				"CALL Repository.GetWebitelUserByID IS FAILED",
				slog.Int64("user.id", authUser.ID),
				slog.Int64("user.dc", authUser.DomainID),
				slog.Any("error", err),
			)

			return buildBroadcastInternalServerError(peer.GetId())
		}

		if wbtUser == nil {
			return buildBroadcastNotFoundError(peer.GetId(), "auth user is not found")
		}

		userID = wbtUser.ID
		userType = userTypeWebitel
		userDomainID = wbtUser.DomainID
		userName = wbtUser.Name
		userChatName = wbtUser.ChatName
	}

	createdAt := time.Now()
	closedAt := createdAt.Add(time.Millisecond * 6)
	variables := map[string]string{
		"cid":                strconv.FormatInt(client.ID, 10),
		"chat":               "portal",
		"from":               client.Name.String,
		"portal.client.id":   peer.GetVia(),
		"portal.service.uid": strings.ReplaceAll(appUser.ID, "-", ""),
		"flow":               strconv.FormatInt(schemaId, 10),
		"broadcast":          "true",
		// "needs_processing":   "false",
	}

	conversation := pg.Conversation{
		DomainID:  userDomainID,
		Title:     client.Name,
		Variables: variables,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	if userType == userTypeBot {
		conversation.Variables = variables
	}

	from := pg.Channel{
		DomainID: userDomainID,
		UserID:   userID,
		Name:     userName,
		Type:     userType,
		PublicName: sql.NullString{
			String: userChatName,
			Valid:  userChatName != "",
		},
		Internal: true,
		Connection: sql.NullString{
			String: "0",
			Valid:  true,
		},
		Variables: variables,
		ClosedCause: sql.NullString{
			String: pbchat.CloseConversationCause_broadcast_end.String(),
			Valid:  true,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	to := pg.Channel{
		DomainID: userDomainID,
		UserID:   client.ID,
		Name:     client.Name.String,
		Type:     "portal",
		Internal: false,
		Connection: sql.NullString{
			String: "0",
			Valid:  true,
		},
		Variables: variables,
		ClosedCause: sql.NullString{
			String: pbchat.CloseConversationCause_broadcast_end.String(),
			Valid:  true,
		},
		// must be a later value than sender, which is necessary for building a history
		CreatedAt: createdAt.Add(time.Millisecond * 4),
		// must be a later value than sender, which is necessary for building a history
		UpdatedAt: createdAt.Add(time.Millisecond * 4),
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	// For the bot to be the last in the members list
	if userType == userTypeWebitel {
		conversation.CreatedAt = to.CreatedAt.Add(2 * time.Millisecond)
	}

	err = c.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := c.repo.CreateConversationTx(ctx, tx, &conversation); err != nil {
			return err
		}
		from.ConversationID = conversation.ID
		to.ConversationID = conversation.ID

		if userType != userTypeBot {
			if err := c.repo.CreateChannelTx(ctx, tx, &from); err != nil {
				return err
			}
		}

		if err := c.repo.CreateChannelTx(ctx, tx, &to); err != nil {
			return err
		}

		fromApp := mapChannelToAppChannel(&from)
		if _, err := c.saveMessage(ctx, tx, fromApp, message); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		switch v := err.(type) {
		case *errors.Error:
			return buildBroadcastError(peer.GetId(), v.Code, v.Detail)

		default:
			c.log.Error(
				"TRANSACTION IS FAILED",
				slog.String("peer.id", peer.GetId()),
				slog.String("peer.via", peer.GetVia()),
				slog.Any("error", err),
			)

			return buildBroadcastInternalServerError(peer.GetId())
		}
	}

	sender := app.Channel{
		DomainID: from.DomainID,
		User: &app.User{
			ID:        from.UserID,
			Channel:   "user",
			Contact:   strconv.FormatInt(from.UserID, 10),
			FirstName: from.Name,
		},
		Chat: &app.Chat{
			ID:      from.ID,
			Channel: "webitel",
			Invite:  conversation.ID,
		},
		Variables: from.Variables,
	}

	target := app.Channel{
		DomainID: to.DomainID,
		User: &app.User{
			ID:        client.ID,
			Channel:   peer.GetType(),
			Contact:   peer.GetId(),
			FirstName: client.FirstName.String,
		},
		Chat: &app.Chat{
			ID:      to.ID,
			Channel: peer.GetType(),
			Invite:  conversation.ID,
		},
		Variables: to.Variables,
	}

	err = c.eventRouter.SendMessageToGateway(&sender, &target, message)
	if err != nil {
		c.log.Error(
			"CALL EventRouter.SendMessageToGateway IS FAILED",
			slog.String("peer.id", peer.GetId()),
			slog.String("peer.via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	return nil
}

type broadcastValidator struct {
	request *pbmessages.BroadcastMessageRequest
	errors  []*pbmessages.BroadcastError
}

func newBroadcastValidator(req *pbmessages.BroadcastMessageRequest) *broadcastValidator {
	return &broadcastValidator{request: req, errors: []*pbmessages.BroadcastError{}}
}

func (v broadcastValidator) validateMessage() error {
	if len(v.request.GetPeers()) == 0 {
		return errors.BadRequest(
			"peers.invalid",
			"broadcast: peers; cannot be empty",
		)
	}

	message := v.request.GetMessage()

	if message.GetText() == "" {
		return errors.BadRequest(
			"message.text.invalid",
			"broadcast: message.text; message text is required",
		)
	}

	file := message.GetFile()

	if file != nil {
		fileId := file.GetId()
		if fileId != "" && !util.IsInteger(fileId) {
			return errors.BadRequest(
				"message.file.id.invalid",
				"broadcast: message.file.id; must be integer string",
			)
		}

		if fileId != "" && file.GetUrl() != "" {
			return errors.BadRequest(
				"message.file.invalid",
				"broadcast: message.file( ? ); require: id -or- url",
			)
		}

		fileSource := file.GetSource()
		if fileSource != "" && fileSource != fileSourceMedia && fileSource != fileSourceFile {
			return errors.BadRequest(
				"message.file.source.invalid",
				"broadcast: message.file.source; values: media -or- file",
			)
		}
	}

	return nil
}

func (v *broadcastValidator) getErrors() []*pbmessages.BroadcastError {
	return v.errors
}

func (v *broadcastValidator) validatePeers() []*pbmessages.InputPeer {
	validPeers := []*pbmessages.InputPeer{}

	for _, peer := range v.request.GetPeers() {
		rule, ok := validationRules[peer.GetType()]
		if !ok {
			v.errors = append(v.errors,
				buildBroadcastBadRequestError(peer.Id, "peer.type is invalid"))
			continue
		}

		if !rule.id.MatchString(peer.GetId()) {
			v.errors = append(v.errors,
				buildBroadcastBadRequestError(peer.Id, "peer.id is invalid"))
			continue
		}

		if !rule.via.MatchString(peer.GetVia()) {
			v.errors = append(v.errors,
				buildBroadcastBadRequestError(peer.Id, "peer.via is invalid"))
			continue
		}

		validPeers = append(validPeers, peer)
	}

	return validPeers
}

func buildBroadcastError(peerId string, code int32, message string) *pbmessages.BroadcastError {
	return &pbmessages.BroadcastError{
		PeerId: peerId,
		Error: &status.Status{
			Code:    code,
			Message: message,
		},
	}
}

func buildBroadcastNotFoundError(peerId, message string) *pbmessages.BroadcastError {
	return buildBroadcastError(peerId, http.StatusNotFound, message)
}

func buildBroadcastBadRequestError(peerId, message string) *pbmessages.BroadcastError {
	return buildBroadcastError(peerId, http.StatusBadRequest, message)
}

func buildBroadcastInternalServerError(peerId string) *pbmessages.BroadcastError {
	return buildBroadcastError(peerId, http.StatusInternalServerError, "internal server error")
}

// mapChannelToAppChannel transform sqlxrepo.Channel struct to app.Channel struct
func mapChannelToAppChannel(channel *sqlxrepo.Channel) *app.Channel {
	firstName, lastName := util.ParseFullName(channel.FullName())

	appChannel := app.Channel{
		DomainID: channel.DomainID,
		User: &app.User{
			ID:        channel.UserID,
			Channel:   channel.Type,
			FirstName: firstName,
			LastName:  lastName,
		},
		Chat: &app.Chat{
			ID:        channel.ID,
			Channel:   channel.Type,
			FirstName: firstName,
			LastName:  lastName,
			Invite:    channel.ConversationID,
		},
		Variables: channel.Variables,
		Created:   channel.CreatedAt.Unix(),
		Updated:   channel.UpdatedAt.Unix(),
		Closed:    0,
	}

	if channel.ClosedAt.Valid {
		appChannel.Closed = channel.ClosedAt.Time.Unix()
	}

	return &appChannel
}

// mapInputMessageToMessage transform pbmessages.InputMessage struct to pbchat.Message struct
func mapInputMessageToMessage(inputMessage *pbmessages.InputMessage) *pbchat.Message {

	// NOTE: Get file and keyboard from input message
	file := inputMessage.GetFile()
	keyboard := inputMessage.GetKeyboard()

	// NOTE: Set chat message text
	chatMessage := &pbchat.Message{
		Text: inputMessage.GetText(),
	}

	// NOTE: Set chat message file
	if file != nil {
		chatFile := &pbchat.File{}
		if file.GetId() != "" {
			parsedFileId, err := strconv.ParseInt(file.GetId(), 10, 64)
			if err == nil && parsedFileId > 0 {
				chatFile.Id = parsedFileId
			}
			chatFile.Mime = mime.FormatMediaType("unknown/unknown", map[string]string{
				"source": file.GetSource(),
			})
		} else if file.GetUrl() != "" {
			chatFile.Url = file.GetUrl()
		}
		chatMessage.File = chatFile
	}

	// NOTE: Set chat keyboard DTO
	if keyboard != nil {
		for _, row := range keyboard.GetRows() {
			chatButtons := &pbchat.Buttons{}
			for _, button := range row.GetButtons() {
				chatButtons.Button = append(chatButtons.Button, &pbchat.Button{
					Caption: button.Caption,
					Text:    button.Text,
					Type:    button.Type,
					Url:     button.Url,
					Code:    button.Code,
				})
			}

			chatMessage.Buttons = append(chatMessage.Buttons, chatButtons)
		}
	}

	return chatMessage
}
