package sqlxrepo

import (
	"fmt"
	"context"
	"database/sql"
	"time"
)

func (repo *sqlxRepository) UpdateClientNumber(ctx context.Context, id int64, number string) error {
	_, err := repo.db.ExecContext(ctx,
		`UPDATE chat.client AS c SET number=$2 WHERE c.id=$1`,
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
		repo.log.Error().Err(err).Msg("GetClientByID")
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
		repo.log.Error().Err(err).Msg("GetClientByExternalID")
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

	for _, text := range
	[]*sql.NullString{
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
		repo.log.Error().Err(err).Msg("CreateClient")
		return err
	}
	
	repo.log.Info().
	
		Int64("oid", c.ID).
		Str("name", c.Name.String).
		Str("phone", c.Number.String).
		Str("contact", c.ExternalID.String).
		Str("first_name", c.FirstName.String).
		Str("last_name", c.LastName.String).

		Msg("CONTACT")
	
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
