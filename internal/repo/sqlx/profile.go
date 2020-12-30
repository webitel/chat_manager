package sqlxrepo

import (
	"context"
	"database/sql"
	"strconv"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (repo *sqlxRepository) GetProfileByID(ctx context.Context, id int64, uri string) (*Profile, error) {

	args := make([]interface{}, 0, 2)
	// return query param ref like: $1, $2 ... $N
	param := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	// query command text
	query := "SELECT e.* FROM chat.profile e WHERE"
	
	where := make([]string, 0, 2)
	filter := func(cond string) {
		where = append(where, cond)
	}
	
	if uri != "" {
		filter(" e.url_id = " + param(uri))
	}
	// default: by id
	if len(where) == 0 || id != 0 {
		filter(" e.id = " + param(id))
	}

	query += strings.Join(where, " AND ")

	result := &Profile{}
	err := repo.db.GetContext(ctx, result, query, args...) // "SELECT * FROM chat.profile WHERE id=$1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // NOT Found !
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) GetProfiles(ctx context.Context, id int64, size, page int32, fields, sort []string, profileType string, domainID int64) ([]*Profile, error) {
	result := []*Profile{}
	fieldsStr, whereStr, sortStr, limitStr := "*", "", "order by created_at desc", ""
	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %v offset %v", size, (page-1)*size)
	if len(fields) > 0 {
	OUTER:
		for _, field := range fields {
			for _, allowedField := range profileAllColumns {
				if field == allowedField {
					continue OUTER
				}
			}
			return nil, errors.New("fields not allowed")
		}
		fieldsStr = strings.Join(fields, ", ")
	}
	if len(sort) > 0 {
		sortField := sort[0][1:]
		found := false
		for _, allowedField := range profileAllColumns {
			if sortField == allowedField {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("sort not allowed")
		}
		if direction := string(sort[0][0]); direction == "-" {
			sortStr = fmt.Sprintf("order by %s desc", sortField)
		} else {
			sortStr = fmt.Sprintf("order by %s asc", sortField)
		}
	}
	queryStrings := make([]string, 0, 3)
	queryArgs := make([]interface{}, 0, 3)
	if id != 0 {
		queryStrings = append(queryStrings, "id")
		queryArgs = append(queryArgs, id)
	}
	if profileType != "" {
		queryStrings = append(queryStrings, "type")
		queryArgs = append(queryArgs, profileType)
	}
	if domainID != 0 {
		queryStrings = append(queryStrings, "domain_id")
		queryArgs = append(queryArgs, domainID)
	}
	if len(queryArgs) > 0 {
		whereStr = "where"
		for i, _ := range queryArgs {
			whereStr = whereStr + fmt.Sprintf(" %s=$%v and", queryStrings[i], i+1)
		}
		whereStr = strings.TrimRight(whereStr, " and")
	}
	query := fmt.Sprintf("SELECT %s FROM chat.profile %s %s %s", fieldsStr, whereStr, sortStr, limitStr)
	err := repo.db.SelectContext(ctx, &result, query, queryArgs...)
	return result, err
}

/*func (repo *sqlxRepository) CreateProfile(ctx context.Context, p *Profile) error {
	p.ID = 0
	p.CreatedAt = time.Now()
	if p.UrlID == "" {
		p.UrlID = uuid.New().String()
	}
	stmt, err := repo.db.PrepareNamed(
		`insert into chat.profile (name, schema_id, type, variables, domain_id, created_at, url_id)` +
		` values (:name, :schema_id, :type, :variables, :domain_id, :created_at, :url_id)`,
	)
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
}*/

func (repo *sqlxRepository) CreateProfile(ctx context.Context, p *Profile) error {
	
	p.ID = 0
	p.CreatedAt = time.Now().UTC()
	
	if p.UrlID == "" {
		p.UrlID = uuid.New().String()
	}

	err := repo.db.GetContext(ctx, &p.ID,
		// query
		"INSERT INTO chat.profile (name, schema_id, type, variables, domain_id, created_at, url_id)" +
		" VALUES ($1, $2, $3, $4, $5, $6, $7)" +
		" RETURNING id",
		// params ...
		p.Name, // :name,
		p.SchemaID, // :schema_id,
		p.Type, // :type,
		p.Variables, // :variables,
		p.DomainID, // :domain_id,
		p.CreatedAt, // :created_at,
		p.UrlID, // :url_id
	)

	if err != nil {
		if err == sql.ErrNoRows {
			panic("gateway: failed to store NEW bot profile; something went wrong")
		}
		return err
	}

	if p.ID == 0 {
		panic("gateway: failed to store NEW bot profile; got <zero> identifier")
	}

	return nil
}

func (repo *sqlxRepository) UpdateProfile(ctx context.Context, p *Profile) error {
	_, err := repo.db.NamedExecContext(ctx,
	`update chat.profile set
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
