package sqlxrepo

import (
	"context"
	"database/sql"
	"time"
)

func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {
	m.ID = 0
	tmp := sql.NullTime{
		time.Now(),
		true,
	}
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
	// TO DO FILTERS
	err := repo.db.SelectContext(ctx, &result, "SELECT m.*, c.user_id, c.type as user_type FROM chat.message m left join chat.channels c on m.channel_id = c.id")
	return result, err
	// query := make([]qm.QueryMod, 0, 6)
	// if size != 0 {
	// 	query = append(query, qm.Limit(int(size)))
	// } else {
	// 	query = append(query, qm.Limit(15))
	// }
	// if page != 0 {
	// 	query = append(query, qm.Offset(int((page-1)*size)))
	// }
	// if id != 0 {
	// 	query = append(query, models.MessageWhere.ID.EQ(id))
	// }
	// if fields != nil && len(fields) > 0 {
	// 	query = append(query, qm.Select(fields...))
	// }
	// if sort != nil && len(sort) > 0 {
	// 	for _, item := range sort {
	// 		query = append(query, qm.OrderBy(item))
	// 	}
	// } else {
	// 	query = append(query, qm.OrderBy("created_at"))
	// }
	// if conversationID != "" {
	// 	query = append(query, models.MessageWhere.ConversationID.EQ(conversationID))
	// }
	// return models.Messages(query...).All(ctx, repo.db)
}
