package sqlxrepo

import (
	"context"
	"database/sql"
	"time"
)

func (repo *sqlxRepository) GetProfileByID(ctx context.Context, id int64) (*Profile, error) {
	result := &Profile{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.profile WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) GetProfiles(ctx context.Context, id int64, size, page int32, fields, sort []string, profileType string, domainID int64) ([]*Profile, error) {
	result := []*Profile{}
	// TO DO FILTERS
	err := repo.db.SelectContext(ctx, &result, "SELECT * FROM chat.profile")
	return result, err
}

func (repo *sqlxRepository) CreateProfile(ctx context.Context, p *Profile) error {
	p.ID = 0
	p.CreatedAt = time.Now()
	stmt, err := repo.db.PrepareNamed(`insert into chat.profile (name, schema_id, type, variables, domain_id, created_at)
	values (:name, :schema_id, :type, :variables, :domain_id, :created_at)`)
	if err != nil {
		return err
	}
	var id int64
	err = stmt.GetContext(ctx, &id, *p)
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (repo *sqlxRepository) UpdateProfile(ctx context.Context, p *Profile) error {
	_, err := repo.db.NamedExecContext(ctx, `update chat.profile set
		name=:name,
		schema_id=:schema_id,
		type=:type,
		variables=:variables,
		domain_id=:domain_id
	where id=:id`, *p)
	return err
}

func (repo *sqlxRepository) DeleteProfile(ctx context.Context, id int64) error {
	_, err := repo.db.ExecContext(ctx, "delete from chat.profile where id=$1", id)
	// count, err := res.RowsAffected()
	// if err == nil {
	// 	/* check count and return true/false */
	// }
	return err
}
