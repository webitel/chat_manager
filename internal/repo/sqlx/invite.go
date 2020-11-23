package sqlxrepo

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func (repo *sqlxRepository) GetInviteByID(ctx context.Context, id string) (*Invite, error) {
	result := &Invite{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.invite WHERE id=$1 and closed_at is null", id)
	if err != nil {
		
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) GetInvites(ctx context.Context, userID int64) ([]*Invite, error) {
	result := []*Invite{}
	err := repo.db.SelectContext(ctx, &result, "SELECT * FROM chat.invite where user_id=$1", userID)
	return result, err
}

func (repo *sqlxRepository) CreateInvite(ctx context.Context, m *Invite) error {
	m.ID = uuid.New().String()
	m.CreatedAt = time.Now()
	_, err := repo.db.NamedExecContext(ctx, `insert into chat.invite (id, conversation_id, user_id, title, timeout_sec, inviter_channel_id, created_at, domain_id)
	values (:id, :conversation_id, :user_id, :title, :timeout_sec, :inviter_channel_id, :created_at, :domain_id)`, *m)
	if err != nil {
		return err
	}
	return nil
}

func (repo *sqlxRepository) CloseInvite(ctx context.Context, inviteID string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.invite set closed_at=$1 where id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, inviteID)
	return err
}
