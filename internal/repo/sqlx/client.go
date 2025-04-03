package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/webitel/chat_manager/internal/util"
)

func (repo *sqlxRepository) UpdateClientChatID(ctx context.Context, id int64, externalId string) error {
	if externalId == "" {
		return fmt.Errorf("clients: missing external_id for update")
	}
	_, err := repo.db.ExecContext(ctx, psqlUpdateClientChatIDSQL, id, externalId)
	return err
}

func (repo *sqlxRepository) UpdateClientNumber(ctx context.Context, id int64, number string) error {
	_, err := repo.db.ExecContext(ctx, psqlUpdateClientNumberSQL, id, number)
	return err
}

func (repo *sqlxRepository) UpdateClientName(ctx context.Context, id int64, name string) error {
	if id < 1 {
		return fmt.Errorf("UpdateClient: invalid client ID: %d. It must be a positive integer", id)
	}

	err := repo.db.GetContext(ctx, nil, psqlUpdateClientNameSQL, id, name)
	if err != nil {
		if err == sql.ErrNoRows {
			err = fmt.Errorf("UPDATE: no result")
		}
		repo.log.Error("UpdateClient", "error", err)
	}

	return nil
}

func (repo *sqlxRepository) GetClientByID(ctx context.Context, id int64) (*Client, error) {
	return repo.getClientByColumn(ctx, "id", id)
}

func (repo *sqlxRepository) GetClientByExternalID(ctx context.Context, externalID string) (*Client, error) {
	return repo.getClientByColumn(ctx, "external_id", externalID)
}

func (repo *sqlxRepository) CreateClient(ctx context.Context, c *Client) error {

	c.ID = 0 // FROM: db shema sequence

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.CreatedAt = c.CreatedAt.UTC()

	util.ValidateNullStrings(
		&c.Type,
		&c.Name,
		&c.FirstName,
		&c.LastName,
		&c.Number,
		&c.ExternalID,
	)

	// PERFORM
	err := repo.db.GetContext(ctx, &c.ID, psqlInsertClientSQL,
		c.Type,
		c.Name,
		c.FirstName,
		c.LastName,
		c.Number,
		c.ExternalID,
		c.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			err = fmt.Errorf("INSERT: no result")
		}
		repo.log.Error("CreateClient",
			"error", err,
		)
		return err
	}

	repo.log.Info("CONTACT",
		"oid", c.ID,
		"type", c.Type.String,
		"name", c.Name.String,
		"phone", c.Number.String,
		"contact", c.ExternalID.String,
		"first_name", c.FirstName.String,
		"last_name", c.LastName.String,
	)

	return nil

	// stmt, err := repo.db.PrepareNamed(
	// 	"INSERT INTO chat.client (name, number, created_at, external_id, first_name, last_name) "+
	// 	"VALUES (:name, :number, :created_at, :external_id, :first_name, :last_name) "+
	// 	"RETURNING id",
	// )
	// if err != nil {
	// 	return err
	// }
	// var id int64
	// err = stmt.GetContext(ctx, &id, *c)
	// if err != nil {
	// 	return err
	// }
	// c.ID = id
	// return nil
}

func (repo *sqlxRepository) getClientByColumn(ctx context.Context, column string, value any) (*Client, error) {
	result := &Client{}
	query := fmt.Sprintf(psqlSelectClientByColumnSQL, column)

	err := repo.db.GetContext(ctx, result, query, value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Error(
			"getClientByColumn",
			"column", column,
			"value", value,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

var psqlUpdateClientChatIDSQL = CompactSQL(`
	UPDATE
		chat.client AS c
	SET
		external_id = coalesce(nullif($2, ''), c.external_id)
	WHERE
		c.id = $1
`)

var psqlUpdateClientNumberSQL = CompactSQL(`
	UPDATE
		chat.client AS c
	SET
		"number" = coalesce(nullif($2, ''), c."number")
	WHERE
		c.id = $1
`)

var psqlUpdateClientNameSQL = CompactSQL(`
	UPDATE
		chat.client AS c
	SET
		name = $2
	WHERE
		c.id = $1
`)

var psqlSelectClientByColumnSQL = CompactSQL(`
	SELECT
		c.id,
		c.type,
		c.name,
		c.first_name,
		c.last_name,
		c.number,
		c.external_id,
		c.created_at
	FROM
		chat.client c
	WHERE
		c.%s = $1
`)

var psqlInsertClientSQL = CompactSQL(`
	INSERT INTO chat.client (
		type,
		name,
		first_name,
		last_name,
		number,
		external_id,
		created_at			
	)
	VALUES (
		$1,
		$2,
		$3,
		$4,
		$5,
		$6,
		$7
	)
	RETURNING
		id
`)
