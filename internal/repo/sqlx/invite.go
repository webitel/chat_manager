package sqlxrepo

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func (repo *sqlxRepository) GetInviteByID(ctx context.Context, id string) (*Invite, error) {
	res := &Invite{}
	err := repo.db.GetContext(ctx, res,
		"SELECT * FROM chat.invite"+
		" WHERE id=$1 AND closed_at ISNULL",
		 id,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return res, nil
}

func (repo *sqlxRepository) GetInvites(ctx context.Context, userID int64) ([]*Invite, error) {
	res := []*Invite{}
	err := repo.db.SelectContext(ctx, &res,
		"SELECT * FROM chat.invite WHERE user_id=$1",
		 userID,
	)
	return res, err
}

func (repo *sqlxRepository) CreateInvite(ctx context.Context, m *Invite) (err error) {
	
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.CreatedAt = m.CreatedAt.UTC()
	
	if m.Title.String == "" {
		// TODO: get .FROM inviter channel contact display name
		_, err = repo.db.ExecContext(ctx,
			"WITH sender AS ("+
			"SELECT COALESCE(contact.name, NULLIF(account.name,''), account.username, channel.name) AS display"+
			" FROM chat.channel" +
			" LEFT JOIN chat.client AS contact ON (contact.id, false) = (channel.user_id, channel.internal)"+
			" LEFT JOIN directory.wbt_user AS account ON (account.id, true) = (channel.user_id, channel.internal)"+
			" WHERE channel.id = $1"+
			") "+
			"INSERT INTO chat.invite ("+
			  "id, conversation_id, user_id, title, timeout_sec, inviter_channel_id, created_at, domain_id, props" +
			") VALUES ($1, $2, $3, COALESCE((SELECT display FROM sender), 'noname'), $4, $5, $6, $7, $8)",
			m.ID,
			m.ConversationID,
			m.UserID,
			m.TimeoutSec,
			m.InviterChannelID,
			m.CreatedAt,
			m.DomainID,
			m.Variables,
		)

	} else { // typical logic }

		_, err = repo.db.ExecContext(ctx,
			"INSERT INTO chat.invite ("+
			"id, conversation_id, user_id, title, timeout_sec, inviter_channel_id, created_at, domain_id, props" +
			") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			m.ID,
			m.ConversationID,
			m.UserID,
			m.Title,
			m.TimeoutSec,
			m.InviterChannelID,
			m.CreatedAt,
			m.DomainID,
			m.Variables,
		)
	}

	if err != nil {
		return err
	}

	return nil
}

func (repo *sqlxRepository) CloseInvite(ctx context.Context, inviteID string) error {
	_, err := repo.db.ExecContext(ctx,
		"UPDATE chat.invite SET closed_at=$1 WHERE id=$2", 
		time.Now(), inviteID,
	)
	return err
}
