package boilrepo

import (
	"context"
	"database/sql"
	"time"

	"github.com/matvoy/chat_server/models"

	"github.com/google/uuid"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func (repo *boilerRepository) GetInviteByID(ctx context.Context, id string) (*models.Invite, error) {
	result, err := models.Invites(models.InviteWhere.ID.EQ(id), models.InviteWhere.ClosedAt.IsNull()).
		One(ctx, repo.db)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (repo *boilerRepository) GetInvites(ctx context.Context, userID int64) ([]*models.Invite, error) {
	return models.Invites(models.InviteWhere.UserID.EQ(userID)).All(ctx, repo.db)
}

func (repo *boilerRepository) CreateInvite(ctx context.Context, m *models.Invite) error {
	m.ID = uuid.New().String()
	if err := m.Insert(ctx, repo.db, boil.Infer()); err != nil {
		return err
	}
	return nil
}

func (repo *boilerRepository) CloseInvite(ctx context.Context, inviteID string) error {
	// _, err := models.Invites(models.InviteWhere.ID.EQ(inviteID)).DeleteAll(ctx, repo.db)
	_, err := models.Invites(models.InviteWhere.ID.EQ(inviteID)).
		UpdateAll(ctx, repo.db, models.M{
			"closed_at": null.Time{
				Valid: true,
				Time:  time.Now(),
			},
		})
	return err
}
