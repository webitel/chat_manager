package sqlxrepo

import (
	"fmt"
	"strings"
	"time"

	// "strconv"
	"context"

	"database/sql"

	"github.com/jackc/pgconn"
	"github.com/jmoiron/sqlx"

	// "github.com/jackc/pgtype"

	"github.com/google/uuid"

	errs "github.com/pkg/errors"

	"github.com/webitel/chat_manager/app"
)

//type StringIDs []string
//
//func (strs StringIDs) Value() (driver.Value, error) {
//	return strings.Join(strs, ", "), nil
//}

func (repo *sqlxRepository) GetConversationByID(ctx context.Context, id string) (*Conversation, error) {

	list, err := repo.GetConversations(ctx, id, 1, 1, nil, nil, 0, false, 0, 0)

	if err != nil {
		repo.log.Error().Err(err).Str("id", id).
			Msg("Failed lookup DB chat.conversation")
		return nil, err
	}

	var obj *Conversation
	if size := len(list); size != 0 {
		if size != 1 {
			// NOTE: page .next exists !
			// return nil, errors.Conflict(
			// 	"chat.channel.search.id.conflict",
			// 	"chat: got too much records looking for channel "+ id,
			// )
			return nil, errs.New("got too much records")
		}
		obj = list[0]
	}

	if obj == nil || !strings.EqualFold(id, obj.ID) {
		obj = nil // NOT FOUND !
	}

	return obj, nil
}

/*func (repo *sqlxRepository) GetConversationByID(ctx context.Context, id string) (*Conversation, error) {
	conversation := &Conversation{}
	err := repo.db.GetContext(ctx, conversation, "SELECT * FROM chat.conversation WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	conversation.Members, conversation.Messages, err = repo.getConversationInfo(ctx, id)
	if err != nil {
		repo.log.Error().Msg(err.Error())
		return nil, err
	}
	return conversation, nil
}*/

func (repo *sqlxRepository) CreateConversation(ctx context.Context, session *Conversation) error {
	return NewSession(repo.db, ctx, session)
}

/*func (repo *sqlxRepository) CreateConversation(ctx context.Context, c *Conversation) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := repo.db.NamedExecContext(ctx, `insert into chat.conversation (id, title, created_at, closed_at, updated_at, domain_id)
	values (:id, :title, :created_at, :closed_at, :updated_at, :domain_id)`, *c)
	return err
}*/

// TODO: CloseConversation(ctx context.Context, id string, at time.Time) error {}
func (repo *sqlxRepository) CloseConversation(ctx context.Context, id string, cause string) error {

	at := time.Now()

	// with cancellation context
	_, err := repo.db.ExecContext(ctx,
		// query statement
		psqlSessionCloseQ,
		// query params ...
		id, at.UTC(), cause,
	)

	return err
}

/*func (repo *sqlxRepository) CloseConversation(ctx context.Context, id string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.conversation set closed_at=$1 where id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, id)
	return err
}*/

func (repo *sqlxRepository) GetConversations(
	ctx context.Context,
	id string,
	size int32,
	page int32,
	fields []string,
	sort []string,
	domainID int64,
	active bool,
	userID int64,
	messageSize int32,
) ([]*Conversation, error) {
	// conversations := make([]*Conversation, 0, size)
	var (
		fieldsStr = // "c.*, m.*, ch.*"
		"c.id, c.title, c.created_at, c.closed_at, c.updated_at, c.domain_id" +
			", m.messages" +
			", ch.members"

		whereStr = ""
		sortStr  = "order by c.created_at desc"
		limitStr = ""
	)
	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %d offset %d", size+1, (page-1)*size)
	if messageSize == 0 {
		messageSize = 10
	}
	// messageLimitStr := fmt.Sprintf("limit %d", messageSize)
	queryStrings := make([]string, 0, 4)
	queryArgs := make([]interface{}, 0, 4)
	argCounter := 1
	if userID != 0 {
		whereStr = "right join chat.channel rch on c.id = rch.conversation_id where rch.user_id=$1 and"
		queryArgs = append(queryArgs, userID)
		argCounter++
	}
	if id != "" {
		queryStrings = append(queryStrings, "c.id")
		queryArgs = append(queryArgs, id)
	}
	// TO DO GET DOMAIN FROM TOKEN
	if domainID != 0 {
		queryStrings = append(queryStrings, "c.domain_id")
		queryArgs = append(queryArgs, domainID)
	}
	if len(queryStrings) > 0 {
		if whereStr == "" {
			whereStr = "where"
		}
		if active != false {
			whereStr = whereStr + " c.closed_at is not null and"
		}
		for i, _ := range queryStrings {
			whereStr = whereStr + fmt.Sprintf(" %s=$%v and", queryStrings[i], i+argCounter)
		}
	}
	whereStr = strings.TrimRight(whereStr, " and")
	query := CompactSQL(fmt.Sprintf(`
		select %s
			from chat.conversation c
				left join LATERAL (
					select json_agg(s) as messages
					from (
						SELECT
							m.id,
							m.channel_id,
							m.created_at,
							m.updated_at,
							m.type,
							m.text,
							(case when (m.file_id isnull and nullif(m.file_url,'') isnull) then null else
								json_build_object('id',m.file_id,'url',m.file_url,'size',m.file_size,'type',m.file_type,'name',m.file_name)
							end) as file,
							m.reply_to as reply_to_message_id,
							m.forward_id as forward_from_message_id
						FROM chat.message m
						where m.conversation_id = c.id
						order by m.created_at desc
						%s
					) s
				) m on true
				left join LATERAL (
					select json_agg(ss) as members
					from (
						select
							   ch.id,
							   ch.type,
							   ch.name,
							   ch.user_id,
							   ch.internal,
								 coalesce(cn.external_id, ch.user_id::text) external_id,
							   ch.created_at,
							   ch.updated_at
						from chat.channel ch
						left join chat.client cn on not ch.internal and cn.id = ch.user_id
						where ch.conversation_id = c.id
					) ss
				) ch on true
			%s
			%s
		%s;`,
		fieldsStr, "" /*messageLimitStr*/, whereStr, sortStr, limitStr,
	))
	// rows, err := repo.db.QueryxContext(ctx, query, queryArgs...)
	rows, err := repo.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		// if err == sql.ErrNoRows {
		// 	return nil, nil
		// }
		return nil, err
	}
	defer rows.Close()
	// for rows.Next() {
	// 	tmp := new(Conversation)
	// 	rows.StructScan(tmp)
	// 	tmp.Members.Scan(tmp.MembersBytes)
	// 	tmp.Messages.Scan(tmp.MessagesBytes)
	// 	conversations = append(conversations, tmp)
	// }
	// return conversations, nil
	list, err := ConversationList(rows, (int)(size))
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

/*func (repo *sqlxRepository) getConversationInfo(ctx context.Context, id string) (members ConversationMembers, messages ConversationMessages, err error) {
	members = ConversationMembers{}
	err = repo.db.SelectContext(ctx, &members,
		`select
			   ch.id,
			   ch.type,
			   ch.user_id,
			   ch.name,
			   ch.internal,
			   ch.created_at,
			   ch.updated_at
		from chat.channel ch
		where ch.conversation_id = $1`, id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		// if err == sql.ErrNoRows {
		// 	err = nil
		// 	return
		// }
		return
	}
	messages = ConversationMessages{}
	err = repo.db.GetContext(ctx, &messages, `
		SELECT
			   m.id,
			   m.text,
			   m.type,
			   m.channel_id,
			   m.created_at,
			   m.updated_at
		FROM chat.message m
		where m.conversation_id=$1
		order by m.created_at desc
		limit 10`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		repo.log.Warn().Msg(err.Error())
		return
	}
	return
}*/

// NewSession creates NEW chat session DB record
func NewSession(dcx sqlx.ExtContext, ctx context.Context, session *Conversation) error {

	// Generate NEW unique UUID for this brand NEW chat session
	session.ID = uuid.New().String()
	localtime := app.CurrentTime() // time.Now() // .UTC()

	if session.CreatedAt.IsZero() {
		session.CreatedAt = localtime
	}
	if session.UpdatedAt.Before(session.CreatedAt) {
		session.UpdatedAt = session.CreatedAt
	}
	session.ClosedAt.Valid = false

	// FIXME:
	session.Title.Valid = true // NOTNULL
	session.Title.String =
		strings.TrimSpace(
			session.Title.String,
		)

	_, err := dcx.ExecContext(
		// cancellation context
		ctx,
		// statement query
		psqlSessionNewQ,
		// statement params ...
		session.ID,
		session.DomainID,
		session.Title,

		session.CreatedAt.UTC(),
		session.UpdatedAt.UTC(),
		nil, // session.ClosedAt,

		NullMetadata(session.Variables),
	)

	if err != nil {
		return err
	}
	// +OK
	return nil
}

// ConversationRequest returns SELECT statement
/*func ConversationRequest(req *SearchOptions) (stmt SelectStmt, params []interface{}, err error) {

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
	From("chat.conversation AS c").
	Columns(

		"m.id",
		"coalesce(m.channel_id,m.conversation_id) AS channel_id", // senderChatID
		"m.conversation_id", // targetChatID

		"m.created_at",
		"m.updated_at",

		"m.type",
		"m.text",
		"m.file_id",
		"m.file_size",
		"m.file_type",
		"m.file_name",

		"m.reply_to",
		"m.forward_id",

		"m.variables",
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
		case int64: // OID

			// id, err := uniqueID(q)
			// if err != nil {
			// 	return err
			// }
			// stmt = stmt.Where("c.id="+param(id))

			req.Size = 1 // normalized !
			stmt = stmt.Where("m.id="+param(q))

		case []int64: // []OID
			size := len(q)
			if size == 0 {
				break // invalid
			}
			req.Size = size // normalized !
			var v pgtype.Int8Array
			_ = v.Set(q)

			stmt = stmt.Where("m.id = ANY("+param(&v)+")")

		default:
			// err = errors.InternalServerError(
			// 	"chat.channel.search.id.filter",
			// 	"chat: channel",
			// )
			err = errs.Errorf("search=message filter=id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [FROM] channel_id
	if q, ok := req.Params["sender.id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("coalesce(m.channel_id,m.conversation_id)="+param(q))
		default:
			err = errs.Errorf("search=message filter=from:chat.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [TO] conversation_id
	if q, ok := req.Params["chat.id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("m.conversation_id="+param(q))
		default:
			err = errs.Errorf("search=message filter=to:chat.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [HAS] variables
	if q, ok := req.Params["props"]; ok && q != nil {
		switch q := q.(type) {
		case map[string]string:
			if q == nil {
				break
			}
			// {"":""} => {}
			delete(q, "")
			if len(q) == 0 {
				break // FIXME: ISNULL ?
			}
			// JSONB::bytes
			data := NullProperties(q)
			if len(data) == 0 {
				err = errs.Errorf("search=message filter=props convert=%#v error=failed to encode props", q)
				return SelectStmt{}, nil, err
			}
			stmt = stmt.Where("m.variables @> "+param(string(data))+"::JSONB")
		default:
			err = errs.Errorf("search=message filter=props convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [TYPE] text | file
	if q, ok := req.Params["type"]; ok && q != nil {
		switch q := q.(type) {
		case string:
			stmt = stmt.Where("m.type="+param(q))
		default:
			err = errs.Errorf("search=message filter=type convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	if len(params) == 0 {
		// NOTE: no any filter specified !
		// List all messages disallowed !
		err = errs.Errorf("search=message filter=nope")
		return SelectStmt{}, nil, err
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
			ref = "m.created_at"
		default:
			err = errs.Errorf("search=message sort=%s", ref)
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
}*/

/*
func scanChatMembers(dst *[]*ConversationMember) ScanFunc {
	return func(src interface{}) error {

	}
}

func scanChatMessages(dst *[]*Message) ScanFunc {
	return func(src interface{}) error {

	}
}*/

// ConversationList scan sql.Rows dataset tuples.
// Zero or negative `size` implies NOLIMIT startegy.
// MAY: Return len([]*Conversation) == (size+1)
// which indicates that .`next` result page exist !
func ConversationList(rows *sql.Rows, limit int) ([]*Conversation, error) {

	//
	if limit < 0 {
		limit = 0
	}

	// TODO: prepare projection
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// alloc projection map
	var (
		obj  *Conversation                           // cursor: target for current tuple
		plan = make([]func() interface{}, len(cols)) // , len(cols))
	)

	for c, col := range cols {
		switch col {

		case "id":
			plan[c] = func() interface{} { return &obj.ID } // NOTNULL (!)
		case "title":
			plan[c] = func() interface{} { return &obj.Title } // NULL: *sql.NullString

		case "created_at":
			plan[c] = func() interface{} { return ScanDatetime(&obj.CreatedAt) } // NOTNULL (!)
		case "updated_at":
			plan[c] = func() interface{} { return ScanDatetime(&obj.UpdatedAt) } // NULL: **time.Time
		case "closed_at":
			plan[c] = func() interface{} { return &obj.ClosedAt } // NULL: *sql.NullTime

		case "domain_id":
			plan[c] = func() interface{} { return ScanInteger(&obj.DomainID) } // NOTNULL: (!)
		case "props":
			plan[c] = func() interface{} { return &obj.Variables } // NULL: *pg.Metadata

		case "members":
			plan[c] = func() interface{} { return ScanJSON(&obj.Members) } // NOTNULL: (!)
		case "messages":
			plan[c] = func() interface{} { return ScanJSON(&obj.Messages) } // NOTNULL: (!)

		default:

			return nil, errs.Errorf("sql: scan %T column %q not supported", obj, col)

		}
	}

	dst := make([]interface{}, len(cols)) // , len(cols))

	var (
		page []Conversation  // mempage
		list []*Conversation // results
	)

	if limit > 0 {

		page = make([]Conversation, limit)
		list = make([]*Conversation, 0, limit+1)

	}

	// var (

	// 	err error
	// 	row *Message
	// )

	for rows.Next() {

		if 0 < limit && len(list) == limit {
			// indicate next page exists !
			// rows.Next(!)
			list = append(list, nil)
			break
		}

		if len(page) != 0 {

			obj = &page[0]
			page = page[1:]

		} else {

			obj = new(Conversation)
		}

		for c, bind := range plan {
			dst[c] = bind()
		}

		err = rows.Scan(dst...)

		if err != nil {
			break
		}

		// // region: check file document attached
		// if doc.ID != 0 {
		// 	obj.File, doc = doc, nil
		// }
		// // endregion

		list = append(list, obj)

	}

	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		return nil, err
	}

	return list, nil
}

func schemaConversationError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *pgconn.PgError:
		// TODO: handle shema-specific errors, constraints, violations ...
	}
	return err
}

// GetConversations unified for [D]ata[C]onnection sql[x].QueryerContext interface
/*func GetConversations(dcx sqlx.ExtContext, req *SearchOptions) ([]*Conversation, error) {

	// region: bind context session
	// session, start := store.GetSession(ctx, dbx)
	// if start {
	// 	ctx = session.Context // chaining DC session context
	// }
	// region

	// local: session
	// req.Time = session.Time
	// req.Context = session.Context
	ctx := req.Context

	stmt, args, err := ConversationRequest(req)
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
	list, err := ConversationList(rows, req.GetSize())
	// Error ?
	err = schemaConversationError(err)

	if err != nil {
		return list, err
	}

	return list, err
}*/

// postgres: chat.session.close(!)
// $1 - conversation_id
// $2 - local timestamp
const psqlSessionCloseQ = `WITH c0 AS (
  UPDATE chat.invite
     SET closed_at=$2
   WHERE conversation_id=$1
     AND closed_at ISNULL
), c1 AS (
  DELETE FROM chat.conversation_confirmation
   WHERE conversation_id=$1
), c2 AS (
  DELETE FROM chat.conversation_node
   WHERE conversation_id=$1
), c3 AS (
  UPDATE chat.conversation
     SET closed_at=$2
   WHERE id=$1
     AND closed_at ISNULL
)
UPDATE chat.channel
   SET closed_at=$2, , closed_cause=$3
 WHERE conversation_id=$1
   AND closed_at ISNULL
`

// postgres: chat.session.create(!)
// $1  - session.id
// $2  - session.domain_id
// $3  - session.title
// $4  - session.created_at
// $5  - session.updated_at
// $6  - session.closed_at // FIXME: NULL ?
// $7  - session.Variables
const psqlSessionNewQ = `INSERT INTO chat.conversation (
  id, domain_id, title, created_at, updated_at, closed_at, props
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)`
