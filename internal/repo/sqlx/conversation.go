package sqlxrepo

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

//type StringIDs []string
//
//func (strs StringIDs) Value() (driver.Value, error) {
//	return strings.Join(strs, ", "), nil
//}

func (repo *sqlxRepository) GetConversationByID(ctx context.Context, id string) (*Conversation, error) {
	conversation := &Conversation{}
	err := repo.db.GetContext(ctx, conversation, "SELECT * FROM chat.conversation WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	conversation.Members, conversation.Messages, _, err = repo.getConversationInfo(ctx, id, -1)
	if err != nil {
		repo.log.Error().Msg(err.Error())
		return nil, err
	}
	return conversation, nil
}

func (repo *sqlxRepository) CreateConversation(ctx context.Context, c *Conversation) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
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
) ([]*Conversation, error) {
	// TO DO FILTERS
	if size == 0 {
		size = 100
	}
	conversations := make([]*Conversation, 0, size)
	rows, err := repo.db.QueryxContext(ctx, `
		select *
			from chat.conversation c
				left join LATERAL (
					select json_agg(s) as messages
					from (
						SELECT
							   m.id,
							   m.text,
							   m.type,
							   mch.user_id,
							   mch.type as user_type,
							   m.created_at,
							   m.updated_at
						FROM chat.message m
							left join chat.channel mch on m.channel_id = mch.id
						where m.conversation_id = c.id
						order by m.created_at desc
						limit 10
					) s
				) m on true
				left join LATERAL (
					select json_agg(ss) as members
					from (
						select
							   ch.id,
							   ch.type,
							   ch.user_id,
							   ch.name,
							   ch.internal,
							   ch.created_at,
							   ch.updated_at
						from chat.channel ch
						where ch.conversation_id = c.id
					) ss
				) ch on true
			where domain_id = 1 --and closed_at isnull
			order by c.created_at desc
		limit 100;
		`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	for rows.Next() {
		tmp := new(Conversation)
		rows.StructScan(tmp)
		tmp.Members.Scan(tmp.MembersBytes)
		tmp.Messages.Scan(tmp.MessagesBytes)
		conversations = append(conversations, tmp)
	}
	return conversations, nil
}

func (repo *sqlxRepository) getConversationInfo(ctx context.Context, id string, userID int64) (members ConversationMembers, messages ConversationMessages, channelID string, err error) {
	members = ConversationMembers{}
	err = repo.db.SelectContext(ctx, &members, "SELECT * FROM chat.channel where conversation_id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		return
	}
	messages = ConversationMessages{}
	err = repo.db.GetContext(ctx, &messages, `SELECT m.*, c.user_id, c.type as user_type
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
	return
}
