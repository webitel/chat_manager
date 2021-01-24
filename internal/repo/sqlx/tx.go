package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	// "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func (repo *sqlxRepository) WithTransaction(txFunc func(*sqlx.Tx) error) (err error) {
	var tx *sqlx.Tx
	if tx, err = repo.db.Beginx(); err != nil {
		repo.log.Error().Msg(err.Error())
		return
	}
	defer func() {
		if p := recover(); p != nil || err != nil {
			repo.log.Error().Msg(err.Error())
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	err = txFunc(tx)
	return
}

func (repo *sqlxRepository) CreateConversationTx(ctx context.Context, tx *sqlx.Tx, c *Conversation) error {
	return NewSession(tx, ctx, c)
}

/*func (repo *sqlxRepository) CreateConversationTx(ctx context.Context, tx *sqlx.Tx, c *Conversation) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := tx.NamedExecContext(ctx, `insert into chat.conversation (id, title, created_at, closed_at, updated_at, domain_id)
	values (:id, :title, :created_at, :closed_at, :updated_at, :domain_id)`, *c)
	return err
}

func (repo *sqlxRepository) CreateMessageTx(ctx context.Context, tx *sqlx.Tx, m *Message) error {
	m.ID = 0
	tmp := time.Now()
	m.CreatedAt = tmp
	m.UpdatedAt = tmp
	stmt, err := tx.PrepareNamed(
		"INSERT INTO chat.message (channel_id, conversation_id, text, created_at, updated_at, type)\n" +
		"VALUES (:channel_id, :conversation_id, :text, :created_at, :updated_at, :type)",
	)
	if err != nil {
		return err
	}
	var id int64
	err = stmt.GetContext(ctx, &id, *m)
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}*/

func (repo *sqlxRepository) CreateMessageTx(ctx context.Context, tx *sqlx.Tx, m *Message) error {
	return SaveMessage(ctx, tx, m)
}

func (repo *sqlxRepository) GetChannelByIDTx(ctx context.Context, tx *sqlx.Tx, id string) (*Channel, error) {
	result := &Channel{}
	err := tx.GetContext(ctx, result, "SELECT * FROM chat.channel WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) GetChannelsTx(
	ctx context.Context,
	tx *sqlx.Tx,
	userID *int64,
	conversationID *string,
	connection *string,
	internal *bool,
	exceptID *string,
) ([]*Channel, error) {
	result := []*Channel{}
	queryStrings := make([]string, 0, 5)
	queryArgs := make([]interface{}, 0, 5)
	if userID != nil {
		queryStrings = append(queryStrings, "user_id")
		queryArgs = append(queryArgs, *userID)
	}
	if conversationID != nil {
		queryStrings = append(queryStrings, "conversation_id")
		queryArgs = append(queryArgs, *conversationID)
	}
	if connection != nil {
		queryStrings = append(queryStrings, "connection")
		queryArgs = append(queryArgs, *connection)
	}
	if internal != nil {
		queryStrings = append(queryStrings, "internal")
		queryArgs = append(queryArgs, *internal)
	}
	if exceptID != nil {
		queryStrings = append(queryStrings, "except_id")
		queryArgs = append(queryArgs, *exceptID)
	}
	if len(queryArgs) > 0 {
		where := " closed_at is null and"
		for i, _ := range queryArgs {
			where = where + fmt.Sprintf(" %s=$%v and", queryStrings[i], i+1)
		}
		where = strings.TrimRight(where, " and")
		err := tx.SelectContext(ctx, &result, fmt.Sprintf("SELECT * FROM chat.channel where%s", where), queryArgs...)
		return result, err
	}
	err := tx.SelectContext(ctx, &result, "SELECT * FROM chat.channel")
	return result, err
}

func (repo *sqlxRepository) CreateChannelTx(ctx context.Context, tx *sqlx.Tx, c *Channel) error {
	return NewChannel(tx, ctx, c)
}

/*func (repo *sqlxRepository) CreateChannelTx(
	ctx context.Context,
	tx *sqlx.Tx,
	c *Channel) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := tx.NamedExecContext(ctx,
		`insert into chat.channel (
			id, 
			type, 
			conversation_id, 
			user_id, 
			connection, 
			created_at, 
			internal, 
			closed_at, 
			updated_at, 
			domain_id, 
			flow_bridge,
			name
		)
		values (
			:id, 
			:type, 
			:conversation_id, 
			:user_id, 
			:connection, 
			:created_at, 
			:internal, 
			:closed_at, 
			:updated_at, 
			:domain_id, 
			:flow_bridge,
			:name
			)`,
		*c,
	)
	return err
}*/

func (repo *sqlxRepository) CloseChannelsTx(ctx context.Context, tx *sqlx.Tx, conversationID string) error {
	_, err := tx.ExecContext(ctx, `update chat.channel set closed_at=$1 where conversation_id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, conversationID)
	return err
}

func (repo *sqlxRepository) CloseConversationTx(ctx context.Context, tx *sqlx.Tx, conversationID string) error {
	_, err := tx.ExecContext(ctx, `update chat.conversation set closed_at=$1 where id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, conversationID)
	return err
}

func (repo *sqlxRepository) CloseInviteTx(ctx context.Context, tx *sqlx.Tx, inviteID string) (bool, error) {
	return CloseInvite(ctx, tx, inviteID)
	// _, err := tx.ExecContext(ctx, `update chat.invite set closed_at=$1 where id=$2`, sql.NullTime{
	// 	Valid: true,
	// 	Time:  time.Now(),
	// }, inviteID)
	// return err
}
