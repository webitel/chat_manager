package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetWebitelUserByID(ctx context.Context, id, domainID int64) (*WebitelUser, error) {
	result := &WebitelUser{}

	query := `
		SELECT
			u.id,
			COALESCE(NULLIF(u.name, ''), u.username) AS name,
			u.dc,
			COALESCE(NULLIF(u.chat_name, ''), NULLIF(u.name, ''), u.username) AS chat_name
		FROM
			directory.wbt_user u
		WHERE
			u.id = $1 AND -- :id
			u.dc = $2     -- :domainID
	`

	err := repo.db.GetContext(ctx, result, query, id, domainID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn(err.Error(), "error", err)

		return nil, err
	}

	return result, nil
}
