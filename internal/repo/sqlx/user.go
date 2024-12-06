package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetWebitelUserByID(ctx context.Context, userID, domainID int64) (*WebitelUser, error) {
	result := &WebitelUser{}

	query := `
		SELECT
			u.id,
			COALESCE(NULLIF(u.name, ''), u.username) AS name,
			u.dc,
			COALESCE(NULLIF(u.chat_public_name, ''), NULLIF(u.name, ''), u.username) AS chat_public_name
		FROM
			directory.wbt_user u
		WHERE
			u.id = $1 AND -- :userID
			u.dc = $2     -- :domainID
	`

	err := repo.db.GetContext(ctx, result, query, userID, domainID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())

		return nil, err
	}

	return result, nil
}
