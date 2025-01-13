package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetAppUser(ctx context.Context, portalUserId, serviceAppId string) (*AppUser, error) {
	result := &AppUser{}

	query := `
		SELECT
			us.id AS id,
			sa.dc AS domain_id,
			us.account_id AS app_id,
			sa.service_id AS service_id
		FROM portal.user_service us
		JOIN portal.service_app sa
			ON sa.id = $2      -- :app_id
		WHERE
			us.account_id = $1 -- :portal_user_id
			AND us.service_id = sa.service_id
			AND us.dc = sa.dc;
	`

	err := repo.db.GetContext(ctx, result, query, portalUserId, serviceAppId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn(err.Error(), "error", err)

		return nil, err
	}

	return result, nil
}
