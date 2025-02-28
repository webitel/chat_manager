package sqlxrepo

import (

	// "fmt"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	proto "github.com/webitel/chat_manager/api/proto/chat"

	"github.com/google/uuid"
	errs "github.com/micro/micro/v3/service/errors"
	"github.com/pkg/errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"

	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/internal/contact"
)

func (repo *sqlxRepository) GetChannelByID(ctx context.Context, id string) (*Channel, error) {
	search := SearchOptions{
		// prepare filter(s)
		Params: map[string]any{
			"id": id, // MUST
		},
		Fields: []string{"id", "*"}, // NOT applicable
		Sort:   []string{},
		Page:   0,
		Size:   1, // GET(!)
	}

	// PERFORM SELECT ...
	list, err := GetChannels(repo.db, ctx, &search)
	if err != nil {
		repo.log.Error("Failed lookup DB chat.channel",
			"error", err,
			"id", id,
		)
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
}

func (repo *sqlxRepository) GetChannelByPeer(ctx context.Context, peerId, fromId string) (*Channel, error) {
	query := `
		SELECT
			chat.*
		FROM
			chat.client peer
		JOIN LATERAL (
			SELECT
				m.id,
				m.conversation_id,
				m."connection"::int8,
				m.type,
				m.props
			FROM
				chat.channel m
			WHERE
				NOT m.internal AND
				m.user_id = peer.id AND
				m.connection = ($1::text) -- :from_id::text
			ORDER BY
				m.created_at DESC -- last
			LIMIT 1
		) chat ON true
		WHERE
			peer.external_id = $2 -- :peer_id
		;`

	rows, err := repo.db.QueryContext(ctx, query, fromId, peerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Fetch !
	list, err := ChannelList(rows, 1)
	// Error ?
	err = schemaChannelError(err)

	if len(list) == 0 {
		return nil, errs.NotFound("sql.channel.get_channel_by_peer.error", "peer not found")
	}

	return list[0], err
}

/*func (repo *sqlxRepository) GetChannelByID(ctx context.Context, id string) (*Channel, error) {

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
}*/

func (repo *sqlxRepository) GetChannels(
	ctx context.Context,
	userID *int64,
	conversationID *string,
	connection *string,
	internal *bool,
	exceptID *string,
	active *bool,
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
	if active != nil {
		searchFilter("active", *active)
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
			list = list[0 : size-1]
		}
	}

	return list, err
}

/*func (repo *sqlxRepository) GetChannels(
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
}*/

func (repo *sqlxRepository) CreateChannel(ctx context.Context, c *Channel) error {
	return NewChannel(repo.db, ctx, c)
}

/*func (repo *sqlxRepository) CreateChannel(ctx context.Context, c *Channel) error {
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
}*/

func (repo *sqlxRepository) CloseChannel(ctx context.Context, id string, cause string) (*Channel, error) {

	if id == "" {
		return nil, errors.New("Close: channel.id required")
	}

	var (
		now             = time.Now()
		needsProcessing bool
		// res = &Channel{}
	)
	switch cause {
	case proto.LeaveConversationCause_client_timeout.String(),
		proto.LeaveConversationCause_agent_timeout.String(),
		proto.LeaveConversationCause_silence_timeout.String():
		needsProcessing = true
	}

	rows, err := repo.db.QueryContext(ctx, psqlChannelCloseQ, id, now.UTC(), cause, needsProcessing)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	// Fetch !
	list, err := ChannelList(rows, 1)
	// Error ?
	err = schemaChannelError(err)

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
			return nil, errors.New("too much records affected")
		}
		obj = list[0]
	}

	if obj == nil || !strings.EqualFold(id, obj.ID) {
		obj = nil // NOT FOUND !
	}

	return obj, nil

	// err := repo.db.GetContext(ctx, res, psqlChannelCloseQ, id, now.UTC())

	// if err != nil {

	// 	if err == sql.ErrNoRows {
	// 		return nil, nil // NOT FOUND !
	// 	}

	// 	repo.log.Warn().Err(err).
	// 		Msg("Failed to mark channel closed")

	// 	return nil, err
	// }

	// return res, nil
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
	query := `
		UPDATE
			chat.channel
		SET
			closed_at = $2 -- :currentTimeUTC
		WHERE
			conversation_id = $1 -- :conversationID
	`

	_, err := repo.db.ExecContext(ctx, query, conversationID, app.CurrentTime().UTC())

	return err
}

func (repo *sqlxRepository) CheckUserChannel(ctx context.Context, channelID string, userID int64) (*Channel, error) {
	search := SearchOptions{
		// prepare filter(s)
		Params: map[string]interface{}{
			"id": channelID, // MUST
			// "user.id": userID,
		},
		Fields: []string{"id", "*"}, // NOT applicable
		Sort:   []string{},
		Page:   0,
		Size:   1, // GET(!)
	}

	if userID != 0 {
		search.Params["user.id"] = userID
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

	if obj == nil || !strings.EqualFold(channelID, obj.ID) {
		obj = nil // NOT FOUND !
	}

	return obj, nil
}

/*func (repo *sqlxRepository) CheckUserChannel(ctx context.Context, channelID string, userID int64) (*Channel, error) {
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
}*/

/*func (repo *sqlxRepository) UpdateChannel(ctx context.Context, channelID string) (int64, error) {

	updatedAt := time.Now()

	_, err := repo.db.ExecContext(ctx,
		"UPDATE chat.channel SET updated_at=$2 WHERE id=$1",
		 channelID, updatedAt.UTC(),
	)

	if err != nil {
		return 0, err
	}

	const precision = time.Millisecond

	return updatedAt.UnixNano()/(int64)(precision), nil
}*/

func (repo *sqlxRepository) UpdateChannel(ctx context.Context, chatID string, readAt *time.Time) error {

	now := app.CurrentTime() // time.Now()

	if readAt != nil && !readAt.IsZero() {

		const divergence = time.Millisecond

		lastMs := now.Truncate(divergence)
		readMs := readAt.Truncate(divergence)

		if readMs.After(lastMs) {
			return errors.Errorf(
				"channel: update until %s date is beyond localtime %s",
				readMs.Format(app.TimeStamp), lastMs.Format(app.TimeStamp),
			)
		}

	} else {

		readAt = &now // MARK reed ALL messages !
	}

	query := `
		UPDATE
			chat.channel
		SET
			updated_at = $2 -- :readAtUTC
		WHERE
			id = $1 AND -- :chatID
			COALESCE(updated_at, created_at) < $2 -- :readAtUTC
	`

	_, err := repo.db.ExecContext(ctx, query, chatID, readAt.UTC())

	if err != nil {
		return err
	}

	return nil
}

func (repo *sqlxRepository) UpdateChannelHost(ctx context.Context, channelID, host string) error {
	query := `
		UPDATE
			chat.channel
		SET
			host = $2 -- :host
		WHERE
			id = $1 -- :channelID
	`

	_, err := repo.db.ExecContext(ctx, query, channelID, host)

	return err
}

func (repo *sqlxRepository) BindChannel(ctx context.Context, channelID string, vars map[string]string) (env map[string]string, err error) {

	if vars != nil {
		// remove invalid (empty) key
		delete(vars, "")
	}

	if len(vars) == 0 {
		// FIXME: remove all binding keys ?
		return nil, nil
	}

	var (
		expr   = "COALESCE(props,'{}')"
		params = make([]interface{}, 0, 3)
	)

	param := func(v interface{}) (sql string) {
		params = append(params, v)
		return "$" + strconv.Itoa(len(params))
	}
	// $1 - chat.channel.id
	_ = param(channelID)

	var (
		del []string          // key(s) to be removed
		set map[string]string // key(s) to be reseted
	)

	for key, value := range vars {
		key = strings.TrimSpace(key)
		if key == "" {
			continue // omit empty keys
		}
		// CASE: blank "" -or- null
		if value == "" {
			// TODO: "props - '$key'"
			if del == nil {
				del = make([]string, 0, len(vars))
			}
			del = append(del, key)
			continue
		}
		// TODO: "props || '{$key: $value}'::jsonb"
		if set == nil {
			set = make(map[string]string, len(vars))
		}
		set[key] = value
	}
	// 1. Remove empty value[d] keys
	if len(del) != 0 {

		var keys pgtype.TextArray
		_ = keys.Set(del)

		expr += " - " + param(&keys) + "::text[]"
	}
	// 2. Reset attributes
	if len(set) != 0 {

		jsonb, _ := json.Marshal(set)

		expr += " || " + param(string(jsonb)) + "::jsonb"
	}

	var setupVars = CompactSQL(
		`-- conversation_id
	WITH s AS (
		UPDATE chat.conversation
		   SET props = %[1]s
		 WHERE id = $1
		RETURNING props
	)
	-- channel_id
	, c AS (
		UPDATE chat.channel
		   SET props = %[1]s
		 WHERE NOT EXISTS(SELECT true FROM s) AND id = $1
		RETURNING props
	)
	-- invite_id ???
	SELECT props FROM s
	 UNION ALL
	SELECT props FROM c
	`)

	// _, err := repo.db.ExecContext(ctx,
	// 	"UPDATE chat.channel SET props="+ expr +" WHERE id=$1",
	// 	 params...,
	// )

	var res Metadata
	err = repo.db.GetContext(ctx, &res,
		// dbx.ScanJSONBytes(&env),
		fmt.Sprintf(setupVars, expr),
		params...,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Invalid channel_id
			err = errs.NotFound(
				"chat.channel.id.invalid",
				"chat: channel id=%s not found",
				channelID,
			)
		}
	}

	return res, err
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
			"c.name",

			"c.user_id",
			"c.domain_id",

			"c.conversation_id",
			"c.connection",
			"c.internal",
			"c.host",
			"c.props", // Chat.StartConversation(.message.variables)

			"c.created_at",
			"c.updated_at",

			"c.joined_at",
			"c.closed_at",
			"c.closed_cause",

			"c.flow_bridge",
		)

	// region: apply filters

	// UUID := func(s string) bool {
	// 	_, err := uuid.Parse(s)
	// 	return err == nil
	// }

	// TODO: !!!
	// uniqueID := func(s string) (interface{}, error) {
	// 	// n := len(s)
	// 	// if n < 32 || n > 36 {

	// 	// }
	// 	id, err := uuid.Parse(s)

	// 	if err != nil {
	// 		return nil, err // uuid.Must(!)
	// 	} else {
	// 		return id.String(), nil // normalized(!)
	// 	}
	// }

	if q, ok := req.Params["id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID

			// id, err := uniqueID(q)
			// if err != nil {
			// 	return err
			// }
			// stmt = stmt.Where("c.id="+param(id))

			req.Size = 1 // normalized !
			stmt = stmt.Where("c.id=" + param(q))

		case []string: // []UUID
			size := len(q)
			if size == 0 {
				break // invalid
			}
			req.Size = size // normalized !
			var v pgtype.Int8Array
			_ = v.Set(q)

			stmt = stmt.Where("c.id = ANY(" + param(&v) + ")")

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
			stmt = stmt.Where("c.user_id=" + param(q))
		default:
			err = errors.Errorf("search=channel filter=user.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["conversation.id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("c.conversation_id=" + param(q))
		default:
			err = errors.Errorf("search=channel filter=conversation.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["contact"]; ok && q != nil {
		switch q := q.(type) {
		case string:
			// TODO: escape !!!
			stmt = stmt.Where("c.connection=" + param(q))
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
			stmt = stmt.Where("c.internal=" + param(v))
		default:
			err = errors.Errorf("search=channel filter=internal convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["except"]; ok && q != nil { // FIXME: "sender" ?
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("c.id <> " + param(q))
		default:
			err = errors.Errorf("search=channel filter=except convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if q, ok := req.Params["active"]; ok && q != nil {
		switch q := q.(type) {
		case bool:
			if q {
				stmt = stmt.Where("c.closed_at ISNULL")
			} else {
				stmt = stmt.Where("c.closed_at IS NOT NULL")
			}
		default:
			err = errors.Errorf("search=channel filter=active convert=%#v", q)
			return SelectStmt{}, nil, err
		}
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
		case '+', ' ':
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
			// if rows.Next() {
			list = append(list, nil)
			// }
			break
		}

		if len(page) != 0 {

			row = &page[0]
			page = page[1:]

		} else {

			row = new(Channel)
		}

		err = row.Scan(rows) // , plan)

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

// NewChannel creates NEW channel record and attach it to the related conversation
func NewChannel(dcx sqlx.ExtContext, ctx context.Context, channel *Channel) error {

	localtime := app.CurrentTime() // time.Now() // .UTC()

	// Support custom IDs, generated by application; e.g.: from: INVITE token
	if channel.ID == "" {
		// Generate NEW unique UUID for this channel
		channel.ID = uuid.New().String()
	}

	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = localtime
	}

	if channel.UpdatedAt.Before(channel.CreatedAt) {
		channel.UpdatedAt = channel.CreatedAt
	}

	if channel.ClosedAt.Time.IsZero() {
		channel.ClosedAt.Valid = false
	} else if channel.ClosedAt.Time.Before(channel.CreatedAt) {
		channel.ClosedAt.Time = channel.UpdatedAt.UTC()
	} else {
		channel.ClosedAt.Time = channel.ClosedAt.Time.UTC()
	}

	// normalizing ...
	if channel.ServiceHost.String != "" {
		channel.Connection.String, _ =
			contact.ContactServiceNode(channel.Connection.String)
	} else {
		channel.Connection.String, channel.ServiceHost.String =
			contact.ContactServiceNode(channel.Connection.String)
	}

	// channel.Connection.Valid = channel.Connection.String != ""
	// channel.ServiceHost.Valid = channel.ServiceHost.String != ""

	for _, param := range []*sql.NullString{
		&channel.Connection, &channel.ServiceHost,
	} {
		param.Valid = param.String != ""
	}

	_, err := dcx.ExecContext(
		// cancellation context
		ctx,
		// statement query
		psqlChannelNewQ,
		// statement params ...
		channel.ID,
		channel.Type,
		channel.Name,

		channel.UserID,
		channel.DomainID,

		channel.ConversationID,
		channel.Internal,

		channel.Connection,
		channel.ServiceHost,

		NullMetadata(channel.Variables), // $10

		channel.CreatedAt.UTC(),
		channel.UpdatedAt.UTC(),
		channel.JoinedAt,
		channel.ClosedAt,

		channel.FlowBridge,

		channel.ClosedCause,
		channel.PublicName,
	)

	if err != nil {
		return err
	}
	// +OK
	return nil
}

// postgres: chat.channel.close(!)
// $1 - channel_id
// $2 - local timestamp
// $3 - close cause
// $4 - needs_processing
var psqlChannelCloseQ = fmt.Sprintf(`WITH closed AS (UPDATE chat.channel c SET 
closed_at=$2, 
closed_cause = $3, 
props=(case when $4 AND c.internal then jsonb_set(c.props, ARRAY['%s'],to_jsonb('true'::text), true) else c.props end) 
WHERE c.id=$1 AND c.closed_at ISNULL RETURNING c.*)
UPDATE chat.conversation s SET updated_at=$2 FROM closed c WHERE s.id=c.conversation_id
RETURNING c.*
`, ChatNeedsProcessingVariable)

// Create NEW channel and attach to related conversation
// $1  - id
// $2  - type
// $3  - name
// $4  - user_id
// $5  - domain_id
// $6  - conversation_id
// $7  - internal
// $8  - connection
// $9  - host
// $10 - props
// $11 - created_at
// $12 - updated_at
// $13 - joined_at
// $14 - closed_at
// $15 - flow_bridge
const psqlChannelNewQ = `WITH created AS (
 INSERT INTO chat.channel (
   id, type, name, user_id, domain_id,
   conversation_id, internal, connection, host, props,
   created_at, updated_at, joined_at, closed_at, flow_bridge, closed_cause, public_name
 ) VALUES (
   $1, $2, $3, $4, $5,
   $6, $7, $8, $9, $10,
   $11, $12, $13, $14, $15,
   $16, $17
 )
 RETURNING conversation_id
)
UPDATE chat.conversation s
   SET updated_at=$11
  FROM created AS c
 WHERE s.id=c.conversation_id
`
