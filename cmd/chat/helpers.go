package chat

import (

	// "time"
	"context"
	"database/sql"
	"encoding/json"

	pb "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/app"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	// "github.com/jmoiron/sqlx"
)

func (s *chatService) closeConversation(ctx context.Context, conversationID *string) error {
	err := s.repo.CloseConversation(ctx, *conversationID)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to update chat CLOSED")
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

func transformProfileFromRepoModel(profile *pg.Profile) (*pb.Profile, error) {
	variableBytes, err := profile.Variables.MarshalJSON()
	variables := make(map[string]string)
	err = json.Unmarshal(variableBytes, &variables)
	if err != nil {
		return nil, err
	}
	result := &pb.Profile{
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

func transformProfileToRepoModel(profile *pb.Profile) (*pg.Profile, error) {
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

func transformProfilesFromRepoModel(profiles []*pg.Profile) ([]*pb.Profile, error) {
	result := make([]*pb.Profile, 0, len(profiles))
	var tmp *pb.Profile
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

func (s *chatService) createClient(ctx context.Context, req *pb.CheckSessionRequest) (client *pg.Client, err error) {
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

func transformConversationFromRepoModel(c *pg.Conversation) *pb.Conversation {

	// const (
	// 	precision = (int64)(time.Millisecond)
	// )

	result := &pb.Conversation{

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

		page := make([]pb.Member, size)
		list := make([]*pb.Member, size)

		for e, src := range c.Members {

			dst := &page[e]

			dst.Type = src.Type
			dst.Internal = src.Internal
			dst.ChannelId = src.ID
			dst.UserId = src.UserID
			dst.Username = src.Name

			list[e] = dst
		}

		result.Members = list
	}

	if size := len(c.Messages); size != 0 {

		page := make([]pb.HistoryMessage, size)
		list := make([]*pb.HistoryMessage, size)

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
				dst.File = &pb.File{
					Id:   doc.ID,
					Url:  "",
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

func transformConversationsFromRepoModel(conversations []*pg.Conversation) []*pb.Conversation {
	result := make([]*pb.Conversation, 0, len(conversations))
	var tmp *pb.Conversation
	for _, item := range conversations {
		tmp = transformConversationFromRepoModel(item)
		result = append(result, tmp)
	}
	return result
}

func transformMessageFromRepoModel(message *pg.Message) *pb.HistoryMessage {
	result := &pb.HistoryMessage{
		Id:        message.ID,
		ChannelId: message.ChannelID, //.String,
		// ConversationId: message.ConversationID,
		//FromUserId:   message.UserID.Int64,
		//FromUserType: message.UserType.String,
		Type:      message.Type,
		Text:      message.Text, //.String,
		CreatedAt: message.CreatedAt.Unix() * 1000,
		UpdatedAt: message.UpdatedAt.Unix() * 1000,
	}
	return result
}

func transformMessagesFromRepoModel(messages []*pg.Message) []*pb.HistoryMessage {
	result := make([]*pb.HistoryMessage, 0, len(messages))
	var tmp *pb.HistoryMessage
	for _, item := range messages {
		tmp = transformMessageFromRepoModel(item)
		result = append(result, tmp)
	}
	return result
}
