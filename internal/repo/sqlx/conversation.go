package sqlxrepo

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	pb "github.com/matvoy/chat_server/api/proto/chat"
)

func (repo *sqlxRepository) GetConversationByID(ctx context.Context, id string) (*pb.Conversation, error) {
	conversation := &Conversation{}
	err := repo.db.GetContext(ctx, conversation, "SELECT * FROM chat.conversation WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	channels := []*Channel{}
	err = repo.db.SelectContext(ctx, &channels, "SELECT * FROM chat.channel where conversation_id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	members := make([]*pb.Member, 0, len(channels))
	for _, ch := range channels {
		tmp := &pb.Member{
			// ChannelId: ch.ID,
			UserId:   ch.UserID,
			Type:     ch.Type,
			Username: ch.Name,
			Internal: ch.Internal,
		}
		if ch.UpdatedAt.Valid {
			tmp.UpdatedAt = ch.UpdatedAt.Time.Unix() * 1000
		}
		members = append(members, tmp)
	}
	result := &pb.Conversation{
		Id:        conversation.ID,
		Title:     conversation.Title.String,
		CreatedAt: conversation.CreatedAt.Time.Unix() * 1000,
		DomainId:  conversation.DomainID,
		Members:   members,
	}
	if conversation.ClosedAt != (sql.NullTime{}) {
		result.ClosedAt = conversation.ClosedAt.Time.Unix() * 1000
	}
	if conversation.UpdatedAt != (sql.NullTime{}) {
		result.UpdatedAt = conversation.UpdatedAt.Time.Unix() * 1000
	}
	return result, nil
}

func (repo *sqlxRepository) CreateConversation(ctx context.Context, c *Conversation) error {
	c.ID = uuid.New().String()
	tmp := sql.NullTime{
		time.Now(),
		true,
	}
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := repo.db.NamedExecContext(ctx, `insert into chat.conversation (id, title, created_at, closed_at, updated_at, domain_id)
	values (:id, :title, :created_at, :closed_at, :updated_at, :domain_id)`, *c)
	return err
}

func (repo *sqlxRepository) CloseConversation(ctx context.Context, id string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.conversation set closed_at=$1 where id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, id)
	return err
}

func (repo *sqlxRepository) GetConversations(
	ctx context.Context,
	id string,
	size int32,
	page int32,
	fields []string,
	sort []string,
	domainID int64,
	active bool,
	userID int64,
) ([]*pb.Conversation, error) {
	// TO DO FILTERS
	conversations := []*Conversation{}
	err := repo.db.SelectContext(ctx, &conversations, "SELECT * FROM chat.conversation")
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	result := make([]*pb.Conversation, 0, len(conversations))
	for _, c := range conversations {
		channels := []*Channel{}
		selfChannelID := ""
		err := repo.db.SelectContext(ctx, &channels, "SELECT * FROM chat.channel where conversation_id=$1", c.ID)
		if err != nil {
			repo.log.Warn().Msg(err.Error())
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		if len(channels) == 0 {
			continue
		}
		members := make([]*pb.Member, 0, len(channels))
		for _, ch := range channels {
			if ch.UserID == userID && ch.Type == "webitel" {
				selfChannelID = ch.ID
			}
			tmp := &pb.Member{
				// ChannelId: ch.ID,
				UserId:   ch.UserID,
				Type:     ch.Type,
				Username: ch.Name,
				Internal: ch.Internal,
			}
			if ch.UpdatedAt.Valid {
				tmp.UpdatedAt = ch.UpdatedAt.Time.Unix() * 1000
			}
			members = append(members, tmp)
		}
		conv := &pb.Conversation{
			Id:            c.ID,
			Title:         c.Title.String,
			CreatedAt:     c.CreatedAt.Time.Unix() * 1000,
			DomainId:      c.DomainID,
			Members:       members,
			SelfChannelId: selfChannelID,
		}
		if c.ClosedAt != (sql.NullTime{}) {
			conv.ClosedAt = c.ClosedAt.Time.Unix() * 1000
		}
		if c.UpdatedAt != (sql.NullTime{}) {
			conv.UpdatedAt = c.UpdatedAt.Time.Unix() * 1000
		}
		result = append(result, conv)
	}
	return result, nil
}
