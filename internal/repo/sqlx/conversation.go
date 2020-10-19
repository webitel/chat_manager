package sqlxrepo

import (
	"context"
	"database/sql"
	"github.com/jmoiron/sqlx"
	"time"

	"github.com/google/uuid"
	pb "github.com/webitel/chat_manager/api/proto/chat"
)

//type StringIDs []string
//
//func (strs StringIDs) Value() (driver.Value, error) {
//	return strings.Join(strs, ", "), nil
//}

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
	members, messages, _, err := repo.getConversationInfo(ctx, id, -1)
	if err != nil {
		repo.log.Error().Msg(err.Error())
		return nil, err
	}
	result := &pb.Conversation{
		Id:        conversation.ID,
		Title:     conversation.Title.String,
		CreatedAt: conversation.CreatedAt.Time.Unix() * 1000,
		DomainId:  conversation.DomainID,
		Members:   members,
		Messages:  messages,
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
	var ids = make([]string, len(conversations))
	for i, c := range conversations {
		ids[i] = c.ID //fmt.Sprintf("'%s'", c.ID)
		//members, messages, selfChannelID, err := repo.getConversationInfo(ctx, c.ID, userID)
		//if err != nil {
		//	repo.log.Error().Msg(err.Error())
		//	return nil, err
		//}
		conv := &pb.Conversation{
			Id:        c.ID,
			Title:     c.Title.String,
			CreatedAt: c.CreatedAt.Time.Unix() * 1000,
			DomainId:  c.DomainID,
			Members:   []*pb.Member{},
			//Members:       members,
			//SelfChannelId: selfChannelID,
			//Messages:      messages,
		}
		if c.ClosedAt != (sql.NullTime{}) {
			conv.ClosedAt = c.ClosedAt.Time.Unix() * 1000
		}
		if c.UpdatedAt != (sql.NullTime{}) {
			conv.UpdatedAt = c.UpdatedAt.Time.Unix() * 1000
		}
		result = append(result, conv)
	}
	channels := []*Channel{}
	s := "SELECT * FROM chat.channel where conversation_id in (?)"
	q, vs, err := sqlx.In(s, ids)
	q = repo.db.Rebind(q)
	err = repo.db.SelectContext(context.Background(), &channels, q, vs...)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			err = nil
			return nil, nil
		}
		return nil, err
	}
	for _, ch := range channels {
		for _, conv := range result {
			if ch.ConversationID == conv.Id {
				if ch.UserID == userID && ch.Type == "webitel" {
					conv.SelfChannelId = ch.ID
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
				conv.Members = append(conv.Members, tmp)
			}
		}
	}

	return result, nil
}

func (repo *sqlxRepository) getConversationInfo(ctx context.Context, id string, userID int64) (members []*pb.Member, messages []*pb.HistoryMessage, channelID string, err error) {
	channels := []*Channel{}
	err = repo.db.SelectContext(ctx, &channels, "SELECT * FROM chat.channel where conversation_id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		return
	}
	members = make([]*pb.Member, 0, len(channels))
	for _, ch := range channels {
		if ch.UserID == userID && ch.Type == "webitel" {
			channelID = ch.ID
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
	lastMessage := new(Message)
	err = repo.db.GetContext(ctx, lastMessage, `SELECT m.*, c.user_id, c.type as user_type
		FROM chat.message m
		left join chat.channel c
		on m.channel_id = c.id
		where m.conversation_id=$1
		order by m.created_at desc
		limit 1`, id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		return
	}
	if *lastMessage != (Message{}) {
		messages = []*pb.HistoryMessage{{
			Id:           lastMessage.ID,
			FromUserId:   lastMessage.UserID.Int64,
			FromUserType: lastMessage.UserType.String,
			Type:         lastMessage.Type,
			Text:         lastMessage.Text.String,
		}}
		if lastMessage.CreatedAt != (sql.NullTime{}) {
			messages[0].CreatedAt = lastMessage.CreatedAt.Time.Unix() * 1000
		}
		if lastMessage.UpdatedAt != (sql.NullTime{}) {
			messages[0].UpdatedAt = lastMessage.UpdatedAt.Time.Unix() * 1000
		}
	}
	return
}
