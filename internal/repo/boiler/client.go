package boilrepo

// import (
// 	"context"
// 	"database/sql"
// 	"strings"

// 	"github.com/webitel/chat_manager/models"

// 	"github.com/volatiletech/sqlboiler/v4/boil"
// 	"github.com/volatiletech/sqlboiler/v4/queries/qm"
// )

// func (repo *boilerRepository) GetClientByID(ctx context.Context, id int64) (*models.Client, error) {
// 	result, err := models.Clients(models.ClientWhere.ID.EQ(id)).
// 		One(ctx, repo.db)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	return result, nil
// }

// func (repo *boilerRepository) GetClientByExternalID(ctx context.Context, externalID string) (*models.Client, error) {
// 	result, err := models.Clients(qm.Where("LOWER(external_id) like ?", strings.ToLower(externalID))).
// 		One(ctx, repo.db)
// 	if err != nil {
// 		repo.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	return result, nil
// }

// func (repo *boilerRepository) CreateClient(ctx context.Context, c *models.Client) error {
// 	if err := c.Insert(ctx, repo.db, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) GetClients(ctx context.Context, limit, offset int) ([]*models.Client, error) {
// 	return models.Clients(qm.Limit(limit), qm.Offset(offset)).All(ctx, repo.db)
// }
