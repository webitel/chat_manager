package sqlxrepo

import (
	"context"
	"fmt"
	"time"
)

func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {
	m.ID = 0
	tmp := time.Now()
	m.CreatedAt = tmp
	m.UpdatedAt = tmp
	stmt, err := repo.db.PrepareNamed(`insert into chat.message (channel_id, conversation_id, text, created_at, updated_at, type)
	values (:channel_id, :conversation_id, :text, :created_at, :updated_at, :type) RETURNING id`)
	if err != nil {
		return err
	}
	var id int64
	err = stmt.GetContext(ctx, &id, *m)
	if err != nil {
		return err
	}
	m.ID = id
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, m.ConversationID)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.channel set updated_at=$1 where id=$2`, tmp, m.ChannelID)
	return err
}

func (repo *sqlxRepository) GetMessages(ctx context.Context, id int64, size, page int32, fields, sort []string, conversationID string) ([]*Message, error) {
	result := []*Message{}
	fieldsStr, whereStr, sortStr, limitStr := "m.*, c.user_id, c.type as user_type", "where conversation_id=$1", "order by created_at desc", ""
	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %v offset %v", size, (page-1)*size)
	query := fmt.Sprintf("SELECT %s FROM chat.message m left join chat.channel c on m.channel_id = c.id %s %s %s", fieldsStr, whereStr, sortStr, limitStr)
	err := repo.db.SelectContext(ctx, &result, query, conversationID)
	return result, err
}

func (repo *sqlxRepository) GetLastMessage(conversationID string) (*Message, error) {
	result := &Message{}
	err := repo.db.Get(result, "select id, text, variables from chat.message where conversation_id=$1 order by created_at desc limit 1", conversationID)
	return result, err
}
