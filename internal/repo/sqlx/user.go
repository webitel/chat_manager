package sqlxrepo

import (
	"context"
	"database/sql"
)

func (repo *sqlxRepository) GetWebitelUserByID(ctx context.Context, id int64) (*WebitelUser, error) {
	result := &WebitelUser{}
	err := repo.db.GetContext(ctx, result, "SELECT id, name, dc FROM directory.wbt_user WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}
