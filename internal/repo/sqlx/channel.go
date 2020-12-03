package sqlxrepo

import (

	"fmt"
	"time"
	"context"
	// "strings"
	"strconv"
	"database/sql"

	"github.com/pkg/errors"
	"github.com/google/uuid"

	"github.com/jmoiron/sqlx"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgconn"

	"github.com/webitel/chat_manager/internal/contact"
)

/*func (repo *sqlxRepository) GetChannelByID(ctx context.Context, id string) (*Channel, error) {
	
	search := SearchOptions{
		// prepare filter(s)
		Params: map[string]interface{}{
			"id": id, // MUST
		},
		Fields: []string{"id","*"}, // NOT applicable
		Sort:   []string{},
		Page:   0,
		Size:   1, // GET(!)
	}

	// PERFORM SELECT ...
	list, err := GetChannels(repo.db, ctx, &search)

	if err != nil {
		return nil, err
	}

	var obj *Channel
	if size := len(list); size != 0 {
		if size != 1 {
			// NOTE: page .next exists !
			// return nil, errors.Conflict(
			// 	"chat.channel.search.id.conflict",
			// 	"chat: got too much records looking for channel "+ id,
			// )
			return nil, errors.New("got too much records")
		}
		obj = list[0]
	}

	if obj == nil || !strings.EqualFold(id, obj.ID) {
		obj = nil // NOT FOUND !
	}

	return obj, nil
}*/


func (repo *sqlxRepository) GetChannelByID(ctx context.Context, id string) (*Channel, error) {
	
	res := &Channel{}
	err := repo.db.GetContext(ctx, res, "select e.* from chat.channel e where e.id=$1", id)
	
	if err != nil {
	
		if err == sql.ErrNoRows {
			return nil, nil // NOT Found !
		}
	
		repo.log.Error().Err(err).Str("id", id).
			Msg("Failed lookup for channel")
	
			return nil, err
	}
	
	return res, nil
}

/*func (repo *sqlxRepository) GetChannels(
	ctx context.Context,
	userID *int64,
	conversationID *string,
	connection *string,
	internal *bool,
	exceptID *string,
) ([]*Channel, error) {

	search := SearchOptions{
		Params: map[string]interface{}{
			"": nil,
		},
		// default
		Fields: nil,
		Sort:   nil,
		Page:   0,
		Size:   0, // NOLIMIT: default
	}

	searchFilter := func(name string, assert interface{}) {
		if search.Params == nil {
			search.Params = make(map[string]interface{})
		}
		if _, has := search.Params[name]; !has {
			search.Params[name] = assert
		}
	}

	// TODO: forward known filters
	if userID != nil {
		searchFilter("user.id", *userID)
	}
	if conversationID != nil {
		searchFilter("conversation.id", *conversationID)
	}
	if connection != nil {
		searchFilter("contact", *connection)
	}
	if internal != nil {
		searchFilter("internal", *internal)
	}
	if exceptID != nil {
		searchFilter("except", *exceptID)
	}
	// PERFORM: SELECT
	list, err := GetChannels(repo.db, ctx, &search)
	// Error ?
	if err != nil {
		return nil, err
	}
	// V0 compatible (crop the last NULL entry)
	if size := len(list); size != 0 {
		if list[size-1] == nil {
			// NOTE: page .next exists !
			// FIXME: v0 compatible
			list = list[0:size-1]
		}
	}

	return list, err
}*/

func (repo *sqlxRepository) GetChannels(
	ctx context.Context,
	userID *int64,
	conversationID *string,
	connection *string,
	internal *bool,
	exceptID *string,
) ([]*Channel, error) {
	// result := []*Channel{}
	queryStrings := make([]string, 0, 5)
	queryArgs := make([]interface{}, 0, 5)
	if userID != nil {
		queryStrings = append(queryStrings, "user_id")
		queryArgs = append(queryArgs, *userID)
	}
	if conversationID != nil {
		queryStrings = append(queryStrings, "conversation_id")
		queryArgs = append(queryArgs, *conversationID)
	}
	if connection != nil {
		queryStrings = append(queryStrings, "connection")
		queryArgs = append(queryArgs, *connection)
	}
	if internal != nil {
		queryStrings = append(queryStrings, "internal")
		queryArgs = append(queryArgs, *internal)
	}
	if exceptID != nil {
		queryStrings = append(queryStrings, "except_id")
		queryArgs = append(queryArgs, *exceptID)
	}

	query := "SELECT * FROM chat.channel"
	
	if len(queryArgs) > 0 {
		where := " WHERE closed_at ISNULL"
		for i := range queryArgs {
			where += fmt.Sprintf(" AND %s=$%d", queryStrings[i], i+1)
		}
		query += where
		// where = strings.TrimRight(where, " and")
		// err := repo.db.SelectContext(ctx, &result,
		// 	"SELECT * FROM chat.channel WHERE" + where,
		// 	 queryArgs...,
		// )
		// return result, err
	} else {
		queryArgs = nil
	}

	var res []*Channel
	err := repo.db.SelectContext(ctx, &res, query, queryArgs...) // "SELECT * FROM chat.channel")
	return res, err
	
	// rows, err := repo.db.QueryContext(ctx, query, queryArgs...)
	
	// if err != nil {
	// 	return nil, err
	// }

	// defer rows.Close()

	// list, err := ChannelList(rows, 0)

	// return list, err
}

func (repo *sqlxRepository) CreateChannel(ctx context.Context, c *Channel) error {
	c.ID = uuid.New().String()
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	// normalizing ...
	if c.ServiceHost.String != "" {
		c.Connection.String, _ =
			contact.ContactServiceNode(c.Connection.String)
	} else {
		c.Connection.String, c.ServiceHost.String =
			contact.ContactServiceNode(c.Connection.String)
	}
	
	c.Connection.Valid = c.Connection.String != ""

	_, err := repo.db.ExecContext(ctx,
		"INSERT INTO chat.channel (\n" +
		"  id, type, name, user_id, domain_id, conversation_id, connection, host, internal,\n" +
		"  created_at, updated_at, closed_at, flow_bridge\n" +
		") VALUES (\n" +
		"  $1, $2, $3, $4, $5, $6, $7, $8, $9,\n" +
		"  $10, $11, $12, $13\n" +
		")",
		// params
		c.ID,
		c.Type,
		c.Name,

		c.UserID,
		c.DomainID,
		c.ConversationID,

		c.Connection,
		c.ServiceHost,
		c.Internal,

		c.CreatedAt,
		c.UpdatedAt,
		c.ClosedAt,
		
		c.FlowBridge,

	)

	// _, err := repo.db.NamedExecContext(ctx,
	// `INSERT INTO chat.channel (
	// 	id,
	// 	type,
	// 	conversation_id,
	// 	user_id,
	// 	connection,
	// 	created_at,
	// 	internal,
	// 	closed_at,
	// 	updated_at,
	// 	domain_id,
	// 	flow_bridge,
	// 	name,
	// ) VALUES (
	// 	:id,
	// 	:type,
	// 	:conversation_id,
	// 	:user_id,
	// 	:connection,
	// 	:created_at,
	// 	:internal,
	// 	:closed_at,
	// 	:updated_at,
	// 	:domain_id,
	// 	:flow_bridge,
	// 	:name
	// )`, *c)

	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, now, c.ConversationID)
	return err
}

func (repo *sqlxRepository) CloseChannel(ctx context.Context, id string) (*Channel, error) {
	
	if id == "" {
		return nil, errors.New("Close: channel.id required")
	}
	
	var (

		now = time.Now()
		res = &Channel{}
	)

	err := repo.db.GetContext(ctx, res, psqlChannelCloseQ, id, now.UTC())
	
	if err != nil {
		
		if err == sql.ErrNoRows {
			return nil, nil // NOT FOUND !
		}

		repo.log.Warn().Err(err).
			Msg("Failed to mark channel closed")
		
		return nil, err
	}
	
	return res, nil
}

/*func (repo *sqlxRepository) CloseChannel(ctx context.Context, id string) (*Channel, error) {
	result := &Channel{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.channel WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	tmp := sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}
	_, err = repo.db.ExecContext(ctx, `update chat.channel set closed_at=$1 where id=$2`, tmp, id)
	if err != nil {
		return nil, err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, result.ConversationID)
	return result, err
}*/

func (repo *sqlxRepository) CloseChannels(ctx context.Context, conversationID string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.channel set closed_at=$1 where conversation_id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, conversationID)
	return err
}

func (repo *sqlxRepository) CheckUserChannel(ctx context.Context, channelID string, userID int64) (*Channel, error) {
	result := &Channel{}
	err := repo.db.GetContext(ctx, result, "SELECT * FROM chat.channel WHERE id=$1 and user_id=$2", channelID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return result, nil
}

func (repo *sqlxRepository) UpdateChannel(ctx context.Context, channelID string) (int64, error) {
	updatedAt := time.Now()
	_, err := repo.db.ExecContext(ctx, `update chat.channel set updated_at=$1 where id=$2`, updatedAt, channelID)
	return updatedAt.Unix() * 1000, err
}


// ChannelsRequest prepares SELECT chat.channel command statement
func ChannelRequest(req *SearchOptions) (stmt SelectStmt, params []interface{}, err error) {

	param := func(args ...interface{}) (sql string) {
		
		if params == nil {
			params = make([]interface{}, 0, len(args))
		}

		for _, v := range args {
			params = append(params, v)
			if sql != "" {
				sql += ","
			}
			sql += "$" + strconv.Itoa(len(params))
		}
		// if v0, ok := params[name]; ok {
		// 	if v0 != v {
		// 		panic(errors.Errorf("param=%s value=%v set=%v", name, v0, v))
		// 	}
		// }
		return sql
	}
	
	stmt = psql.Select().
	From("chat.channel c").
	Columns(
		"c.id",
		"c.type",
		"c.conversation_id",
		"c.user_id",
		"c.connection",
		"c.created_at",
		"c.internal",
		"c.closed_at",
		"c.domain_id",
		"c.flow_bridge",
		"c.updated_at",
		"c.name",
		"c.joined_at",
		"c.closed_cause",
	)

	// region: apply filters

	// UUID := func(s string) bool {
	// 	_, err := uuid.Parse(s)
	// 	return err == nil
	// }

	if q, ok := req.Params["id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID

			req.Size = 1 // normalized !
			stmt = stmt.Where("c.id="+param(q))

		case []string: // []UUID
			size := len(q)
			if size == 0 {
				break // invalid
			}
			req.Size = size // normalized !
			var v pgtype.Int8Array
			_ = v.Set(q)

			stmt = stmt.Where("c.id = ANY("+param(&v)+")")
			
		default:
			// err = errors.InternalServerError(
			// 	"chat.channel.search.id.filter",
			// 	"chat: channel",
			// )
			err = errors.Errorf("search=channel filter=id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["user.id"]; ok && q != nil {
		switch q := q.(type) {
		// case string: 
			// TODO: username[@domain]
		case int64:
			stmt = stmt.Where("c.user_id="+param(q))
		default:
			err = errors.Errorf("search=channel filter=user.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["conversation.id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("c.conversation_id="+param(q))
		default:
			err = errors.Errorf("search=channel filter=conversation.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["contact"]; ok && q != nil {
		switch q := q.(type) {
		case string: 
			// TODO: escape !!!
			stmt = stmt.Where("c.connection="+param(q))
		default:
			err = errors.Errorf("search=channel filter=contact convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["internal"]; ok && q != nil {
		// FIXME: (.type = 'webitel')
		switch v := q.(type) {
		// case string: 
			// TODO: username[@domain]
		case bool:
			// userId := q
			stmt = stmt.Where("c.internal="+param(v))
		default:
			err = errors.Errorf("search=channel filter=internal convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["except"]; ok && q != nil { // FIXME: "sender" ?
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("c.id <> "+param(q))
		default:
			err = errors.Errorf("search=channel filter=except convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// VIEW: OPENED ONLY !
	if len(params) != 0 {
		stmt = stmt.Where("c.closed_at ISNULL")
	}
	// endregion
	
	// region: sort order
	sort := req.Sort
	if len(sort) == 0 {
		sort = []string{"!created_at"}
	}
	req.Sort = sort
	for _, ref := range sort {
		if ref == "" {
			continue
		}
		order := "" // ASC
		switch ref[0] {
		case '+':
			order = " ASC"
			ref = ref[1:]
		case '-', '!':
			order = " DESC"
			ref = ref[1:]
		}
		switch ref {
		case "created_at":
			ref = "c.created_at"
		default:
			err = errors.Errorf("search=channel sort=%s", ref)
			return SelectStmt{}, nil, err
		}
		stmt = stmt.OrderBy(ref + order)
	}
	// endregion
	
	// region: limit/offset
	size, page := req.GetSize(), req.GetPage()
	
	if size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page > 1 {
			stmt = stmt.Offset((uint64)((page - 1) * (size)))
		}
		// LIMIT (size+1) -- to indicate whether there are more result entries
		stmt = stmt.Limit((uint64)(size + 1))
	}
	// endregion

	return stmt, params, nil
}

// ChannelList scan sql.Rows dataset tuples.
// Zero or negative `size` implies NOLIMIT startegy.
// MAY: Return len([]*Cahnnels) == (size+1)
// which indicates that .`next` result page exist !
func ChannelList(rows *sql.Rows, limit int) ([]*Channel, error) {

	// TODO: prepare projection

	// 
	if limit < 0 {
		limit = 0
	}

	var (

		page []Channel
		list []*Channel
	)

	if limit > 0 {

		page = make([]Channel, limit)
		list = make([]*Channel, 0, limit+1)

	}

	var (
		
		err error
		row *Channel
	)

	for rows.Next() {

		if 0 < limit && len(list) == limit {
			// indicate next page exists !
			list = append(list, nil)
			break
		}


		if len(page) != 0 {

			row = &page[0]
			page = page[1:]

		} else {

			row = new(Channel)
		}

		err := row.Scan(rows) // , plan)
		
		if err != nil {
			break
		}

		list = append(list, row)

	}

	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		return nil, err
	}

	return list, nil
}

func schemaChannelError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *pgconn.PgError:
		// TODO: handle shema-specific errors, constraints ...
	}
	return err
}

// GetChannels unified for [D]ata[C]onnection sql[x].QueryerContext interface
func GetChannels(dcx sqlx.ExtContext, ctx context.Context, req *SearchOptions) ([]*Channel, error) {

	// region: bind context session
	// session, start := store.GetSession(ctx, dbx)
	// if start {
	// 	ctx = session.Context // chaining DC session context
	// }
	// region

	// local: session
	// req.Time = session.Time
	// req.Context = session.Context

	stmt, args, err := ChannelRequest(req)
	if err != nil {
		return nil, err // 400
	}
	query, _, err := stmt.ToSql()
	if err != nil {
		return nil, err // 500
	}
	
	// region: bind context transaction
	// // dc := session
	// tx, err := session.BeginTxx(ctx, nil) // +R
	// if err != nil {
	// 	return nil, err
	// }
	// // defer dc.Rollback()
	// defer tx.Rollback()
	// endregion

	rows, err := dcx.QueryContext(ctx, query, args...)
	// rows, err := tx.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	// Fetch !
	list, err := ChannelList(rows, req.GetSize())
	// Error ?
	err = schemaChannelError(err)

	if err != nil {
		return list, err
	}

	return list, err
}

// postgres: chat.channel.close(!)
// $1 - channel_id
// $2 - local timestamp
const psqlChannelCloseQ =
`WITH closed AS (UPDATE chat.channel c SET closed_at=$2 WHERE c.id=$1 RETURNING c.*)
UPDATE chat.conversation s SET updated_at=$2 FROM closed c WHERE s.id=c.conversation_id
RETURNING c.*
`