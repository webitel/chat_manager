package main

import (
	"context"
	"database/sql"
	"encoding/json"

	pb "github.com/matvoy/chat_server/api/proto/chat"
	pg "github.com/matvoy/chat_server/internal/repo/sqlx"

	"github.com/jmoiron/sqlx"
)

func (s *chatService) closeConversation(ctx context.Context, conversationID *string) error {
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
}

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
	}
	return result, nil
}

func transformProfileToRepoModel(profile *pb.Profile) (*pg.Profile, error) {
	result := &pg.Profile{
		ID:       profile.Id,
		Name:     profile.Name,
		Type:     profile.Type,
		DomainID: profile.DomainId,
		SchemaID: sql.NullInt64{
			profile.SchemaId,
			true,
		},
	}
	result.Variables.Scan(profile.Variables)
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

// func transformConversationFromRepoModel(c *repo.Conversation) *pb.Conversation {
// 	result := &pb.Conversation{
// 		Id:       c.ID,
// 		Title:    *c.Title,
// 		DomainId: c.DomainID,
// 	}
// 	if c.CreatedAt != nil {
// 		result.CreatedAt = c.CreatedAt.Unix() * 1000
// 	}
// 	if c.ClosedAt != nil {
// 		result.ClosedAt = c.ClosedAt.Unix() * 1000
// 	}
// 	if c.UpdatedAt != nil {
// 		result.UpdatedAt = c.UpdatedAt.Unix() * 1000
// 	}
// 	members := make([]*pb.Member, 0, len(c.Members))
// 	for _, item := range c.Members {
// 		members = append(members, &pb.Member{
// 			ChannelId: item.ChannelID,
// 			UserId:    item.UserID,
// 			Username:  item.Username,
// 			Type:      item.Type,
// 			Internal:  item.Internal,
// 			Firstname: item.Firstname,
// 			Lastname:  item.Lastname,
// 		})
// 	}
// 	result.Members = members
// 	return result
// }

// func transformConversationsFromRepoModel(conversations []*repo.Conversation) []*pb.Conversation {
// 	result := make([]*pb.Conversation, 0, len(conversations))
// 	var tmp *pb.Conversation
// 	for _, item := range conversations {
// 		tmp = transformConversationFromRepoModel(item)
// 		result = append(result, tmp)
// 	}
// 	return result
// }

func transformMessageFromRepoModel(message *pg.Message) *pb.HistoryMessage {
	result := &pb.HistoryMessage{
		Id: message.ID,
		// ChannelId: message.ChannelID.String,
		// ConversationId: message.ConversationID,
		FromUserId:   message.UserID.Int64,
		FromUserType: message.UserType.String,
		Type:         message.Type,
		Text:         message.Text.String,
	}
	if message.CreatedAt.Valid {
		result.CreatedAt = message.CreatedAt.Time.Unix() * 1000
	}
	if message.UpdatedAt.Valid {
		result.UpdatedAt = message.UpdatedAt.Time.Unix() * 1000
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
