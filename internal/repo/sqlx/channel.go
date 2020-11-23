package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (repo *sqlxRepository) GetChannelByID(ctx context.Context, id string) (*Channel, error) {
	res := &Channel{}
	err := repo.db.GetContext(ctx, res, "select e.* from chat.channel e where e.id=$1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // NOT Found !
		}
		repo.log.Error().Err(err).Msg("Failed to lookup chat.channel")
		return nil, err
	}
	return res, nil
}

func (repo *sqlxRepository) GetChannels(
	ctx context.Context,
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
		for i := range queryArgs {
			where = where + fmt.Sprintf(" %s=$%d and", queryStrings[i], i+1)
		}
		where = strings.TrimRight(where, " and")
		err := repo.db.SelectContext(ctx, &result,
			"SELECT * FROM chat.channel WHERE" + where,
			 queryArgs...,
		)
		return result, err
	}
	err := repo.db.SelectContext(ctx, &result, "SELECT * FROM chat.channel")
	return result, err
}

func (repo *sqlxRepository) CreateChannel(ctx context.Context, c *Channel) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := repo.db.NamedExecContext(ctx,
	`INSERT INTO chat.channel (
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
	) VALUES (
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
	)`, *c)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, c.ConversationID)
	return err
}

func (repo *sqlxRepository) CloseChannel(ctx context.Context, id string) (*Channel, error) {
	result := &Channel{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.channel WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	tmp := sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}
	_, err = repo.db.ExecContext(ctx, `update chat.channel set closed_at=$1 where id=$2`, tmp, id)
	if err != nil {
		return nil, err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, result.ConversationID)
	return result, err
}

func (repo *sqlxRepository) CloseChannels(ctx context.Context, conversationID string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.channel set closed_at=$1 where conversation_id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, conversationID)
	return err
}

func (repo *sqlxRepository) CheckUserChannel(ctx context.Context, channelID string, userID int64) (*Channel, error) {
	result := &Channel{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.channel WHERE id=$1 and user_id=$2", channelID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) UpdateChannel(ctx context.Context, channelID string) (int64, error) {
	updatedAt := time.Now()
	_, err := repo.db.ExecContext(ctx, `update chat.channel set updated_at=$1 where id=$2`, updatedAt, channelID)
	return updatedAt.Unix() * 1000, err
}
