package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetPortalAppUser(ctx context.Context, portalUserId, serviceAppId string) (*AppUser, error) {
	var result AppUser

	query := `
		SELECT
			us.id AS id,
			sa.dc AS domain_id,
			us.account_id AS app_id,
			sa.service_id AS service_id
		FROM
			portal.user_service us
		JOIN
			portal.service_app sa
		ON
			sa.id = $2         -- :service_app_id
		WHERE
			us.account_id = $1 -- :portal_user_id
			AND us.service_id = sa.service_id
			AND us.dc = sa.dc;
	`

	err := repo.db.GetContext(ctx, &result, query, portalUserId, serviceAppId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &result, nil
}

func (repo *sqlxRepository) GetPortalAppSchemaID(ctx context.Context, portalAppId string) (int64, error) {
	var result int64

	query := `
		SELECT
			si.schema_id
		FROM
			portal.service_app sa
		JOIN
			portal.service_im si
		ON
			sa.service_id = si.id
			AND sa.dc = si.dc
		WHERE
			sa.id = $1;  -- :portal_app_id
	`

	err := repo.db.GetContext(ctx, &result, query, portalAppId)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return result, nil
}
