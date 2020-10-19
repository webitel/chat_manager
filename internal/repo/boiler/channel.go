package boilrepo

// import (
// 	"context"
// 	"database/sql"
// 	"time"

// 	"github.com/webitel/chat_manager/models"

// 	"github.com/google/uuid"
// 	"github.com/volatiletech/null/v8"
// 	"github.com/volatiletech/sqlboiler/v4/boil"
// 	"github.com/volatiletech/sqlboiler/v4/queries/qm"
// )

// func (repo *boilerRepository) GetChannelByID(ctx context.Context, id string) (*models.Channel, error) {
// 	result, err := models.Channels(
// 		models.ChannelWhere.ID.EQ(id),
// 		qm.Load(models.ChannelRels.Conversation),
// 	).
// 		One(ctx, repo.db)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	return result, nil
// }

// func (repo *boilerRepository) GetChannels(
// 	ctx context.Context,
// 	userID *int64,
// 	conversationID *string,
// 	connection *string,
// 	internal *bool,
// 	exceptID *string,
// ) ([]*models.Channel, error) {
// 	query := make([]qm.QueryMod, 0, 6)
// 	query = append(query, models.ChannelWhere.ClosedAt.IsNull())
// 	if userID != nil {
// 		query = append(query, models.ChannelWhere.UserID.EQ(*userID))
// 	}
// 	if conversationID != nil {
// 		query = append(query, models.ChannelWhere.ConversationID.EQ(*conversationID))
// 	}
// 	if connection != nil {
// 		query = append(query, models.ChannelWhere.Connection.EQ(
// 			null.String{
// 				*connection,
// 				true,
// 			},
// 		))
// 	}
// 	if internal != nil {
// 		query = append(query, models.ChannelWhere.Internal.EQ(*internal))
// 	}
// 	if exceptID != nil {
// 		query = append(query, models.ChannelWhere.ID.NEQ(*exceptID))
// 	}
// 	return models.Channels(query...).All(ctx, repo.db)
// }

// func (repo *boilerRepository) CreateChannel(ctx context.Context, c *models.Channel) error {
// 	c.ID = uuid.New().String()
// 	if err := c.Insert(ctx, repo.db, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) CloseChannel(ctx context.Context, id string) (*models.Channel, error) {
// 	result, err := models.Channels(models.ChannelWhere.ID.EQ(id)).
// 		One(ctx, repo.db)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	result.ClosedAt = null.Time{
// 		Valid: true,
// 		Time:  time.Now(),
// 	}
// 	_, err = result.Update(ctx, repo.db, boil.Infer())
// 	return result, err
// }

// func (repo *boilerRepository) CloseChannels(ctx context.Context, conversationID string) error {
// 	_, err := models.Channels(models.ChannelWhere.ConversationID.EQ(conversationID)).
// 		UpdateAll(ctx, repo.db, models.M{
// 			"closed_at": null.Time{
// 				Valid: true,
// 				Time:  time.Now(),
// 			},
// 		})
// 	return err
// }
