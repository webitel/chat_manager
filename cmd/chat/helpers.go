package chat

import (

	// "time"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	pbchat "github.com/webitel/chat_manager/api/proto/chat"

	"github.com/webitel/chat_manager/app"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	// "github.com/jmoiron/sqlx"
)

func (s *chatService) closeConversation(ctx context.Context, conversationID *string, cause string) error {
	err := s.repo.CloseConversation(ctx, *conversationID, cause)
	if err != nil {
		s.log.Error("Failed to update chat CLOSED",
			slog.Any("error", err),
			slog.String("conversation_id", *conversationID),
		)
		return err
	}
	return nil
}

/*func (s *chatService) closeConversation(ctx context.Context, conversationID *string) error {
	if err := s.repo.WithTransaction(func(tx *sqlx.Tx) error {
		if err := s.repo.CloseConversationTx(ctx, tx, *conversationID); err != nil {
			return err
		}
		if err := s.repo.CloseChannelsTx(ctx, tx, *conversationID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.log.Error().Msg(err.Error())
		return err
	}
	return nil
}*/

func transformProfileFromRepoModel(profile *pg.Profile) (*pbchat.Profile, error) {
	variableBytes, err := profile.Variables.MarshalJSON()
	variables := make(map[string]string)
	err = json.Unmarshal(variableBytes, &variables)
	if err != nil {
		return nil, err
	}
	result := &pbchat.Profile{
		Id:        profile.ID,
		Name:      profile.Name,
		Type:      profile.Type,
		DomainId:  profile.DomainID,
		SchemaId:  profile.SchemaID.Int64,
		Variables: variables,
		UrlId:     profile.UrlID,
	}
	return result, nil
}

func transformProfileToRepoModel(profile *pbchat.Profile) (*pg.Profile, error) {
	result := &pg.Profile{
		ID:       profile.Id,
		Name:     profile.Name,
		Type:     profile.Type,
		DomainID: profile.DomainId,
		UrlID:    profile.UrlId,
		SchemaID: sql.NullInt64{
			profile.SchemaId,
			true,
		},
	}
	// {"":""} {}
	props := profile.GetVariables()
	if props != nil {
		delete(props, "")
		if len(props) == 0 {
			props = nil
		}
	}
	// reset: normalized !
	profile.Variables = props

	if props != nil {

		data, err := json.Marshal(props)

		if err == nil {
			err = result.Variables.Scan(data)
		}

		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func transformProfilesFromRepoModel(profiles []*pg.Profile) ([]*pbchat.Profile, error) {
	result := make([]*pbchat.Profile, 0, len(profiles))
	var tmp *pbchat.Profile
	var err error
	for _, item := range profiles {
		tmp, err = transformProfileFromRepoModel(item)
		if err != nil {
			return nil, err
		}
		result = append(result, tmp)
	}
	return result, nil
}

func (s *chatService) createClient(ctx context.Context, req *pbchat.CheckSessionRequest) (client *pg.Client, err error) {
	client = &pg.Client{
		ExternalID: sql.NullString{
			req.ExternalId,
			true,
		},
		Name: sql.NullString{
			req.Username,
			true,
		},
	}
	err = s.repo.CreateClient(ctx, client)
	return
}

func transformConversationFromRepoModel(c *pg.Conversation) *pbchat.Conversation {

	// const (
	// 	precision = (int64)(time.Millisecond)
	// )

	result := &pbchat.Conversation{

		Id:       c.ID,
		Title:    c.Title.String,
		DomainId: c.DomainID,

		CreatedAt: app.DateTimestamp(c.CreatedAt), // .UnixNano()/precision,
		UpdatedAt: app.DateTimestamp(c.UpdatedAt),
	}

	// if !c.UpdatedAt.IsZero() {
	// 	result.UpdatedAt = c.UpdatedAt.UnixNano()/precision
	// }

	if size := len(c.Members); size != 0 {

		page := make([]pbchat.Member, size)
		list := make([]*pbchat.Member, size)

		for e, src := range c.Members {

			dst := &page[e]

			dst.ChannelId = src.ID
			dst.Type = src.Type
			dst.UserId = src.UserID
			dst.Username = src.Name
			dst.Internal = src.Internal
			dst.ExternalId = src.ChatID

			list[e] = dst
		}

		result.Members = list
	}

	if size := len(c.Messages); size != 0 {

		page := make([]pbchat.HistoryMessage, size)
		list := make([]*pbchat.HistoryMessage, size)

		for e, src := range c.Messages {

			dst := &page[e]

			dst.Id = src.ID
			dst.ChannelId = src.ChannelID // .String
			dst.CreatedAt = app.DateTimestamp(src.CreatedAt)
			dst.UpdatedAt = app.DateTimestamp(src.UpdatedAt) // Read/Seen Until !
			// dst.CreatedAt = src.CreatedAt.UnixNano()/precision
			// dst.UpdatedAt = item.UpdatedAt.Unix() * 1000,
			// dst.FromUserId:   item.UserID,
			// dst.FromUserType: item.UserType,
			dst.Type = src.Type
			dst.Text = src.Text // .String
			// File ?
			if doc := src.File; doc != nil {
				dst.File = &pbchat.File{
					Id:   doc.ID,
					Url:  doc.URL,
					Size: doc.Size,
					Mime: doc.Type,
					Name: doc.Name,
				}
			}

			// // Edited ?
			// if !src.UpdatedAt.IsZero() {
			// 	dst.UpdatedAt = src.UpdatedAt.UnixNano()/precision
			// }

			// TODO: ReplyToMessage ?
			// TODO: ForwardFromMessage ?

			list[e] = dst
		}

		result.Messages = list
	}

	return result
}

func transformConversationsFromRepoModel(conversations []*pg.Conversation) []*pbchat.Conversation {
	result := make([]*pbchat.Conversation, 0, len(conversations))
	var tmp *pbchat.Conversation
	for _, item := range conversations {
		tmp = transformConversationFromRepoModel(item)
		result = append(result, tmp)
	}
	return result
}

var epoch = time.Date(1970, time.January, 01, 00, 00, 00, 000000000, time.UTC)

func timestamp(date time.Time) int64 {
	if date.IsZero() || !date.After(epoch) {
		return 0
	}
	return date.UnixMilli()
}

func transformMessageFromRepoModel(message *pg.Message) *pbchat.HistoryMessage {
	result := &pbchat.HistoryMessage{
		Id:        message.ID,
		ChannelId: message.ChannelID, //.String,
		// ConversationId: message.ConversationID,
		//FromUserId:   message.UserID.Int64,
		//FromUserType: message.UserType.String,
		Type:      message.Type,
		Text:      message.Text,                 //.String,
		CreatedAt: timestamp(message.CreatedAt), // message.CreatedAt.Unix() * 1000,
		UpdatedAt: timestamp(message.UpdatedAt), // message.UpdatedAt.Unix() * 1000,
	}
	return result
}

func transformMessagesFromRepoModel(messages []*pg.Message) []*pbchat.HistoryMessage {
	result := make([]*pbchat.HistoryMessage, 0, len(messages))
	var tmp *pbchat.HistoryMessage
	for _, item := range messages {
		tmp = transformMessageFromRepoModel(item)
		result = append(result, tmp)
	}
	return result
}
