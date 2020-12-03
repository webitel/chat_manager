package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetWebitelUserByID(ctx context.Context, id int64) (*WebitelUser, error) {
	result := &WebitelUser{}
	err := repo.db.GetContext(ctx, result,
		"SELECT u.id, COALESCE(NULLIF(u.name, ''), u.username) AS name, u.dc\n" +
		"  FROM directory.wbt_user u WHERE u.id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}
