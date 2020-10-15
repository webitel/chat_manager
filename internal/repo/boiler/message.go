package boilrepo

import (
	"context"

	"github.com/matvoy/chat_server/models"

	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func (repo *boilerRepository) CreateMessage(ctx context.Context, m *models.Message) error {
	if err := m.Insert(ctx, repo.db, boil.Infer()); err != nil {
		return err
	}
	return nil
}

func (repo *boilerRepository) GetMessages(ctx context.Context, id int64, size, page int32, fields, sort []string, conversationID string) ([]*models.Message, error) {
	query := make([]qm.QueryMod, 0, 6)
	if size != 0 {
		query = append(query, qm.Limit(int(size)))
	} else {
		query = append(query, qm.Limit(15))
	}
	if page != 0 {
		query = append(query, qm.Offset(int((page-1)*size)))
	}
	if id != 0 {
		query = append(query, models.MessageWhere.ID.EQ(id))
	}
	if fields != nil && len(fields) > 0 {
		query = append(query, qm.Select(fields...))
	}
	if sort != nil && len(sort) > 0 {
		for _, item := range sort {
			query = append(query, qm.OrderBy(item))
		}
	} else {
		query = append(query, qm.OrderBy("created_at"))
	}
	if conversationID != "" {
		query = append(query, models.MessageWhere.ConversationID.EQ(conversationID))
	}
	return models.Messages(query...).All(ctx, repo.db)
}
