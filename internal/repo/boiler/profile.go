package boilrepo

// import (
// 	"context"
// 	"database/sql"

// 	"github.com/webitel/chat_manager/models"

// 	"github.com/volatiletech/sqlboiler/v4/boil"
// 	"github.com/volatiletech/sqlboiler/v4/queries/qm"
// )

// func (repo *boilerRepository) GetProfileByID(ctx context.Context, id int64) (*models.Profile, error) {
// 	result, err := models.Profiles(models.ProfileWhere.ID.EQ(id)).
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

// func (repo *boilerRepository) GetProfiles(ctx context.Context, id int64, size, page int32, fields, sort []string, profileType string, domainID int64) ([]*models.Profile, error) {
// 	query := make([]qm.QueryMod, 0, 8)
// 	if size != 0 {
// 		query = append(query, qm.Limit(int(size)))
// 	} else {
// 		query = append(query, qm.Limit(15))
// 	}
// 	if page != 0 {
// 		query = append(query, qm.Offset(int((page-1)*size)))
// 	}
// 	if id != 0 {
// 		query = append(query, models.ProfileWhere.ID.EQ(id))
// 	}
// 	if fields != nil && len(fields) > 0 {
// 		query = append(query, qm.Select(fields...))
// 	}
// 	if sort != nil && len(sort) > 0 {
// 		for _, item := range sort {
// 			query = append(query, qm.OrderBy(item))
// 		}
// 	}
// 	if profileType != "" {
// 		query = append(query, models.ProfileWhere.Type.EQ(profileType))
// 	}
// 	if domainID != 0 {
// 		query = append(query, models.ProfileWhere.DomainID.EQ(domainID))
// 	}
// 	return models.Profiles(query...).All(ctx, repo.db)
// }

// func (repo *boilerRepository) CreateProfile(ctx context.Context, p *models.Profile) error {
// 	p.ID = 0
// 	if err := p.Insert(ctx, repo.db, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) UpdateProfile(ctx context.Context, p *models.Profile) error {
// 	if _, err := p.Update(ctx, repo.db, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (repo *boilerRepository) DeleteProfile(ctx context.Context, id int64) error {
// 	_, err := models.Profiles(models.ProfileWhere.ID.EQ(id)).DeleteAll(ctx, repo.db)
// 	return err
// }
