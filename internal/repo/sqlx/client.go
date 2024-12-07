package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (repo *sqlxRepository) UpdateClientChatID(ctx context.Context, id int64, externalId string) error {
	if externalId == "" {
		return fmt.Errorf("clients: missing external_id for update")
	}
	_, err := repo.db.ExecContext(ctx,
		`UPDATE chat.client AS c SET external_id=coalesce(nullif($2,''),c.external_id) WHERE c.id=$1`,
		id, externalId,
	)
	return err
}

func (repo *sqlxRepository) UpdateClientNumber(ctx context.Context, id int64, number string) error {
	_, err := repo.db.ExecContext(ctx,
		`UPDATE chat.client AS c SET "number"=coalesce(nullif($2,''),c."number") WHERE c.id=$1`,
		id, number,
	)
	return err
}

func (repo *sqlxRepository) GetClientByID(ctx context.Context, id int64) (*Client, error) {
	result := &Client{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.client WHERE id=$1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Error("GetClientByID",
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) GetClientByExternalID(ctx context.Context, externalID string) (*Client, error) {
	result := &Client{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.client WHERE external_id=$1", externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Error("GetClientByExternalID",
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) CreateClient(ctx context.Context, c *Client) error {

	c.ID = 0 // FROM: db shema sequence

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.CreatedAt = c.CreatedAt.UTC()

	for _, text := range []*sql.NullString{
		&c.Name,
		&c.Number,
		&c.ExternalID,
		&c.FirstName, &c.LastName,
	} {
		text.Valid = text.String != ""
	}

	// PERFORM
	err := repo.db.GetContext(ctx, &c.ID,
		"INSERT INTO chat.client (name, number, created_at, external_id, first_name, last_name) "+
			"VALUES ($1, $2, $3, $4, $5, $6) "+
			"RETURNING id",

		c.Name,
		c.Number,
		c.CreatedAt,
		c.ExternalID,
		c.FirstName,
		c.LastName,
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
