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

// func (repo *boilerRepository) WithTransaction(txFunc func(*sql.Tx) error) (err error) {
// 	var tx *sql.Tx
// 	if tx, err = repo.db.Begin(); err != nil {
// 		repo.log.Error().Msg(err.Error())
// 		return
// 	}
// 	defer func() {
// 		if p := recover(); p != nil || err != nil {
// 			repo.log.Error().Msg(err.Error())
// 			_ = tx.Rollback()
// 		} else {
// 			err = tx.Commit()
// 		}
// 	}()
// 	err = txFunc(tx)
// 	return
// }

// func (repo *boilerRepository) CreateConversationTx(ctx context.Context, tx boil.ContextExecutor, c *models.Conversation) error {
// 	c.ID = uuid.New().String()
// 	if err := c.Insert(ctx, tx, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) CreateMessageTx(ctx context.Context, tx boil.ContextExecutor, m *models.Message) error {
// 	if err := m.Insert(ctx, tx, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) GetChannelByIDTx(ctx context.Context, tx boil.ContextExecutor, id string) (*models.Channel, error) {
// 	result, err := models.Channels(
// 		models.ChannelWhere.ID.EQ(id),
// 		qm.Load(models.ChannelRels.Conversation),
// 	).
// 		One(ctx, tx)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	return result, nil
// }

// func (repo *boilerRepository) GetChannelsTx(
// 	ctx context.Context,
// 	tx boil.ContextExecutor,
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
// 	return models.Channels(query...).All(ctx, tx)
// }

// func (repo *boilerRepository) CloseChannelTx(
// 	ctx context.Context,
// 	tx boil.ContextExecutor,
// 	id string) error {
// 	result, err := models.Channels(models.ChannelWhere.ID.EQ(id)).
// 		One(ctx, tx)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil
// 		}
// 		return err
// 	}
// 	result.ClosedAt = null.Time{
// 		Valid: true,
// 		Time:  time.Now(),
// 	}
// 	_, err = result.Update(ctx, tx, boil.Infer())
// 	return err
// }

// func (repo *boilerRepository) CreateChannelTx(
// 	ctx context.Context,
// 	tx boil.ContextExecutor,
// 	c *models.Channel) error {
// 	c.ID = uuid.New().String()
// 	if err := c.Insert(ctx, tx, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) CloseChannelsTx(ctx context.Context, tx boil.ContextExecutor, conversationID string) error {
// 	_, err := models.Channels(models.ChannelWhere.ConversationID.EQ(conversationID)).
// 		UpdateAll(ctx, tx, models.M{
// 			"closed_at": null.Time{
// 				Valid: true,
// 				Time:  time.Now(),
// 			},
// 		})
// 	return err
// }

// func (repo *boilerRepository) CloseInviteTx(ctx context.Context, tx boil.ContextExecutor, inviteID string) error {
// 	// _, err := models.Invites(models.InviteWhere.ID.EQ(inviteID)).DeleteAll(ctx, tx)
// 	_, err := models.Invites(models.InviteWhere.ID.EQ(inviteID)).
// 		UpdateAll(ctx, repo.db, models.M{
// 			"closed_at": null.Time{
// 				Valid: true,
// 				Time:  time.Now(),
// 			},
// 		})
// 	return err
// }
