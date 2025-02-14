package chat

import (
	"context"
	"database/sql"
	"log/slog"
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
	"github.com/webitel/chat_manager/internal/util"
	"google.golang.org/genproto/googleapis/rpc/status"
)

// Broadcast ...
// - simple
// - socials
// - portal
//

func (c *chatService) executeBroadcast(ctx context.Context, authUser *auth.User, req *pbmessages.BroadcastMessageRequest, resp *pbmessages.BroadcastMessageResponse) error {
	message, _ := util.MapInputMessageToMessage(req.GetMessage())

	for _, peer := range req.GetPeers() {
		switch peer.GetType() {
		case "gotd":
			vars, fail := c.executeBroadcastSimple(ctx, peer, message)
			if fail != nil {
				resp.Failure = append(resp.Failure, fail)
			}

			resp.Variables = util.MargeMaps(resp.Variables, vars)

		case "telegram", "viber", "facebook", "messenger", "instagram", "whatsapp", "vk":
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

func (c *chatService) executeBroadcastSimple(ctx context.Context, peer *pbmessages.InputPeer, message *pbchat.Message) (map[string]string, *pbmessages.BroadcastError) {
	from, _ := strconv.ParseInt(peer.GetVia(), 10, 64)

	resp, err := c.botClient.BroadcastMessage(ctx, &pbbot.BroadcastMessageRequest{
		Peer:    []string{peer.GetId()},
		From:    from,
		Message: message,
	})
	if err != nil {
		c.log.Error(
			"CALL Bots.BroadcastMessage IS FAILED",
			slog.String("peer", peer.GetId()),
			slog.String("via", peer.GetVia()),
			slog.Any("error", err),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
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
	wbtUser, err := c.repo.GetWebitelUserByID(ctx, authUser.ID, authUser.DomainID)
	if err != nil {
		c.log.Warn(
			"NOT FOUND WEBITEL USER",
			slog.Int64("user_id", authUser.ID),
			slog.Int64("domain_id", authUser.DomainID),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
	}

	createdAt := time.Now()
	closedAt := createdAt.Add(time.Millisecond * 5)

	client, err := c.repo.GetClientByExternalID(ctx, peer.GetId())
	if err != nil {
		return nil, buildBroadcastNotFoundError(peer.GetId())
	}

	conversation := pg.Conversation{
		DomainID: wbtUser.DomainID,
		Title: sql.NullString{
			String: wbtUser.Name,
			Valid:  true,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	from := pg.Channel{
		DomainID:   wbtUser.DomainID,
		UserID:     wbtUser.ID,
		Name:       wbtUser.Name,
		PublicName: wbtUser.ChatName,
		Type:       "webitel", // bot from schema
		Internal:   true,
		Variables: map[string]string{
			"cid":              peer.GetId(),
			"portal.client.id": peer.GetVia(),
			"broadcast":        "true",
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	to := pg.Channel{
		DomainID: wbtUser.DomainID,
		UserID:   client.ID,
		Type:     peer.GetType(),
		Name:     client.Name.String,
		Internal: false,
		Variables: map[string]string{
			"cid":              peer.GetId(),
			"portal.client.id": peer.GetVia(),
			"broadcast":        "true",
		},
		// must be a later value than sender, which is necessary for building a history
		CreatedAt: createdAt.Add(time.Millisecond * 5),
		// must be a later value than sender, which is necessary for building a history
		UpdatedAt: createdAt.Add(time.Millisecond * 5),
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	// optimize
	err = c.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := c.repo.CreateConversationTx(ctx, tx, &conversation); err != nil {
			return err
		}
		from.ConversationID = conversation.ID
		to.ConversationID = conversation.ID

		if err := c.repo.CreateChannelTx(ctx, tx, &from); err != nil {
			return err
		}

		if err := c.repo.CreateChannelTx(ctx, tx, &to); err != nil {
			return err
		}

		fromApp, _ := util.MapChannelToAppChannel(&from)
		if _, err := c.saveMessage(ctx, tx, fromApp, message); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.log.Warn(
			"TRANSACTION IS FAILED",
			slog.String("peer", peer.GetId()),
			slog.String("via", peer.GetVia()),
			slog.Any("error", err),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
	}

	peerViaInt, _ := strconv.ParseInt(peer.GetVia(), 10, 64)

	resp, err := c.botClient.BroadcastMessage(ctx, &pbbot.BroadcastMessageRequest{
		Peer:    []string{peer.GetId()},
		From:    peerViaInt,
		Message: message,
	})
	if err != nil {
		c.log.Error(
			"CALL Bots.BroadcastMessage IS FAILED",
			slog.String("peer", peer.GetId()),
			slog.String("via", peer.GetVia()),
			slog.Any("error", err),
		)

		return nil, buildBroadcastInternalServerError(peer.GetId())
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
	wbtUser, err := c.repo.GetWebitelUserByID(ctx, authUser.ID, authUser.DomainID)
	if err != nil {
		c.log.Warn(
			"NOT FOUND WEBITEL USER",
			slog.Int64("user_id", authUser.ID),
			slog.Int64("domain_id", authUser.DomainID),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	createdAt := time.Now()
	closedAt := createdAt.Add(time.Millisecond * 5)

	client, err := c.repo.GetClientByExternalID(ctx, peer.GetId())
	if err != nil {
		return buildBroadcastNotFoundError(peer.GetId())
	}

	appUser, err := c.repo.GetAppUser(ctx, peer.GetId(), peer.GetVia())
	if err != nil {
		return buildBroadcastNotFoundError(peer.GetId())
	}

	clientIdStr := strconv.FormatInt(client.ID, 10)
	portalServiceUserId := strings.ReplaceAll(appUser.ID, "-", "")

	conversation := pg.Conversation{
		DomainID: wbtUser.DomainID,
		Title: sql.NullString{
			String: wbtUser.Name,
			Valid:  true,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	from := pg.Channel{
		DomainID:   wbtUser.DomainID,
		UserID:     wbtUser.ID,
		Name:       wbtUser.Name,
		PublicName: wbtUser.ChatName,
		Type:       "webitel", // bot from schema
		Internal:   true,
		Variables: map[string]string{
			"cid":                clientIdStr,
			"portal.client.id":   peer.GetVia(),
			"portal.service.uid": portalServiceUserId,
			"broadcast":          "true",
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	to := pg.Channel{
		DomainID: wbtUser.DomainID,
		UserID:   client.ID,
		Type:     peer.GetType(),
		Name:     client.Name.String,
		Internal: false,
		Variables: map[string]string{
			"cid":                clientIdStr,
			"portal.client.id":   peer.GetVia(),
			"portal.service.uid": portalServiceUserId,
			"broadcast":          "true",
		},
		// must be a later value than sender, which is necessary for building a history
		CreatedAt: createdAt.Add(time.Millisecond * 5),
		// must be a later value than sender, which is necessary for building a history
		UpdatedAt: createdAt.Add(time.Millisecond * 5),
		ClosedAt: sql.NullTime{
			Time:  closedAt,
			Valid: true,
		},
	}

	// optimize
	err = c.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := c.repo.CreateConversationTx(ctx, tx, &conversation); err != nil {
			return err
		}
		from.ConversationID = conversation.ID
		to.ConversationID = conversation.ID

		if err := c.repo.CreateChannelTx(ctx, tx, &from); err != nil {
			return err
		}

		if err := c.repo.CreateChannelTx(ctx, tx, &to); err != nil {
			return err
		}

		fromApp, _ := util.MapChannelToAppChannel(&from)
		if _, err := c.saveMessage(ctx, tx, fromApp, message); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.log.Warn(
			"TRANSACTION IS FAILED",
			slog.String("peer", peer.GetId()),
			slog.String("via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	sender := app.Channel{
		DomainID: from.DomainID,
		User: &app.User{
			// ID:        524,
			// Channel:   "bot",
			// Contact:   "524",
			// FirstName: "DEV Lite",
			ID:        from.UserID,
			Channel:   "user", // or bot from chat_flow
			Contact:   strconv.FormatInt(from.UserID, 10),
			FirstName: from.Name,
		},
		Chat: &app.Chat{
			ID:      from.ID,   // conversation id or channel id | if type == bot then conversation id == channel id | if type == bot => only one channel (member)
			Channel: from.Type, // chatflow for flow_manager
			Invite:  conversation.ID,
			Contact: "0@10.10.10.89:12701",
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
			Contact: "go.webitel.khatsko.portal@", // go.webitel.portal | @10.10.10.89:12701
		},
		Variables: to.Variables,
	}

	err = c.eventRouter.SendMessageToGateway(&sender, &target, message)
	if err != nil {
		c.log.Error(
			"CALL EventRouter.SendMessageToGateway IS FAILED",
			slog.String("peer", peer.GetId()),
			slog.String("via", peer.GetVia()),
			slog.Any("error", err),
		)

		return buildBroadcastInternalServerError(peer.GetId())
	}

	return nil
}

// func (c *chatService) createConversation(ctx context.Context, conversation *pg.Conversation, channels []*pg.Channel, message *pbchat.Message) error {
// 	return c.repo.WithTransaction(func(tx *sqlx.Tx) error {
// 		if err := c.repo.CreateConversationTx(ctx, tx, conversation); err != nil {
// 			return err
// 		}

// 		for _, channel := range channels {
// 			if err := c.repo.CreateChannelTx(ctx, tx, channel); err != nil {
// 				return err
// 			}

// 			channel.ConversationID = conversation.ID
// 		}

// 		fromApp, _ := util.MapChannelToAppChannel(&from)
// 		if _, err := c.saveMessage(ctx, tx, fromApp, message); err != nil {
// 			return err
// 		}

// 		return nil
// 	})
// }

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
			"peers: cannot be empty",
		)
	}

	if v.request.GetMessage().GetText() == "" {
		return errors.BadRequest(
			"message.invalid",
			"message: message text is required",
		)
	}

	fileId := v.request.GetMessage().GetFile().GetId()
	if fileId != "" && !util.IsInteger(fileId) {
		return errors.BadRequest(
			"message.file.id.invalid",
			"file.id: must be integer string",
		)
	}

	// TODO: Need to check file.source

	return nil
}

func (v *broadcastValidator) getErrors() []*pbmessages.BroadcastError {
	return v.errors
}

func (v *broadcastValidator) validatePeers() []*pbmessages.InputPeer {
	type rule struct {
		id, via string
	}

	rules := map[string]rule{
		"telegram": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"gotd": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"viber": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"facebook": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"instagram": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"messenger": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"whatsapp": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"vk": rule{
			id:  `^\d+(\.\d+)?$`,
			via: `^\d+(\.\d+)?$`,
		},
		"portal": rule{
			id:  `^[a-z0-9]+$`,
			via: `^[a-z0-9]+$`,
		},
	}

	validPeers := []*pbmessages.InputPeer{}

	for _, peer := range v.request.GetPeers() {
		rule, ok := rules[peer.GetType()]
		if !ok {
			v.errors = append(v.errors,
				buildBroadcastError(peer.Id, http.StatusBadRequest, "peer.type is invalid"))
			continue
		}

		if !regexp.MustCompile(rule.id).MatchString(peer.GetId()) {
			v.errors = append(v.errors,
				buildBroadcastError(peer.Id, http.StatusBadRequest, "peer.id is invalid"))
			continue
		}

		if !regexp.MustCompile(rule.id).MatchString(peer.GetId()) {
			v.errors = append(v.errors,
				buildBroadcastError(peer.Id, http.StatusBadRequest, "peer.via is invalid"))
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

func buildBroadcastNotFoundError(peerId string) *pbmessages.BroadcastError {
	return buildBroadcastError(peerId, http.StatusNotFound, "peer is not found")
}

func buildBroadcastInternalServerError(peerId string) *pbmessages.BroadcastError {
	return buildBroadcastError(peerId, http.StatusInternalServerError, "internal server error")
}
