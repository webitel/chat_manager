package sqlxrepo

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	// "encoding/json"

	"database/sql"
	"database/sql/driver"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
	"google.golang.org/protobuf/encoding/protojson"

	errs "github.com/pkg/errors"
	// "github.com/micro/go-micro/v2/errors"

	"github.com/webitel/chat_manager/app"
)

func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {

	now := app.CurrentTime() // time.Now()

	m.ID = 0
	m.CreatedAt = now
	// m.UpdatedAt = now

	err := repo.db.GetContext(
		// context, result
		ctx, &m.ID,
		// statement query !
		sentMessageQ,
		// statement params ...
		m.CreatedAt.UTC(), // $1 - SEND timestamp
		m.ChannelID,       // $2 - FROM: sender channel_id
		m.ConversationID,  // $3 - TO: session conversation_id
		m.Type,            // $4 - SEND: message event (default: text)
		m.Text,            // $5 - SEND: message text
		m.Variables,       // $6 - SEND: message vars
	)

	if err != nil {
		return err
	}

	return nil
}

/*func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {
	m.ID = 0
	tmp := time.Now()
	m.CreatedAt = tmp
	m.UpdatedAt = tmp
	stmt, err := repo.db.PrepareNamed(`insert into chat.message (channel_id, conversation_id, text, created_at, updated_at, type)
	values (:channel_id, :conversation_id, :text, :created_at, :updated_at, :type) RETURNING id`)
	if err != nil {
		return err
	}
	var id int64
	err = stmt.GetContext(ctx, &id, *m)
	if err != nil {
		return err
	}
	m.ID = id
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, m.ConversationID)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.channel set updated_at=$1 where id=$2`, tmp, m.ChannelID)
	return err
}*/

func (repo *sqlxRepository) GetMessages(ctx context.Context, id int64, size, page int32, fields, sort []string, domainID int64, conversationID string) ([]*Message, error) {
	result := []*Message{}
	fieldsStr, whereStr, sortStr, limitStr :=
		"m.id, m.channel_id, m.conversation_id, m.text, m.created_at, m.updated_at, m.type, m.variables", //, "+
		//  "c.user_id, c.type as user_type",
		"where c.domain_id=$1 and m.conversation_id=$2",
		"order by created_at desc",
		""

	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %d offset %d", size, (page-1)*size)
	query := fmt.Sprintf("SELECT %s FROM chat.message m left join chat.channel c on m.channel_id = c.id %s %s %s", fieldsStr, whereStr, sortStr, limitStr)
	err := repo.db.SelectContext(ctx, &result, query, domainID, conversationID)
	return result, err
}

func (repo *sqlxRepository) GetLastMessage(conversationID string) (*Message, error) {
	result := &Message{}
	err := repo.db.Get(result, "select id, text, variables from chat.message where conversation_id=$1 order by created_at desc limit 1", conversationID)
	return result, err
}

// Statement to store historical (SENT) message
// $1 - SEND timestamp
// $2 - FROM: sender channel_id
// $3 - TO: session conversation_id
// $4 - SEND: message event (default: text)
// $5 - SEND: message text
// $6 - SEND: message vars
const sentMessageQ = `WITH sender AS (UPDATE chat.channel SET updated_at=$1 WHERE id=$2)
, latest AS (UPDATE chat.conversation SET updated_at=$1 WHERE id=$3)
INSERT INTO chat.message (
  created_at, updated_at, channel_id, conversation_id, type, text, variables
) VALUES (
  $1, NULL, $2, $3, $4, $5, $6
) RETURNING id`

func MessageRequest(req *SearchOptions) (stmt SelectStmt, params []interface{}, err error) {

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
		From("chat.message AS m").
		Columns(

			"m.id",
			"coalesce(m.channel_id,m.conversation_id) AS channel_id", // senderChatID
			"m.conversation_id", // targetChatID

			"m.created_at",
			"m.updated_at",

			"m.type",
			"m.text",
			"m.file_id",
			"m.file_url",
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
			stmt = stmt.Where("m.id=" + param(q))

		case []int64: // []OID
			size := len(q)
			if size == 0 {
				break // invalid
			}
			req.Size = size // normalized !
			var v pgtype.Int8Array
			_ = v.Set(q)

			stmt = stmt.Where("m.id = ANY(" + param(&v) + ")")

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
			stmt = stmt.Where("coalesce(m.channel_id,m.conversation_id)=" + param(q))
		default:
			err = errs.Errorf("search=message filter=from:chat.id convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [TO] conversation_id
	if q, ok := req.Params["chat.id"]; ok && q != nil {
		switch q := q.(type) {
		case string: // UUID
			stmt = stmt.Where("m.conversation_id=" + param(q))
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
			data := NullMetadata(q)
			if len(data) == 0 {
				err = errs.Errorf("search=message filter=props convert=%#v error=failed to encode props", q)
				return SelectStmt{}, nil, err
			}
			stmt = stmt.Where("m.variables @> " + param(string(data)) + "::JSONB")
		default:
			err = errs.Errorf("search=message filter=props convert=%#v", q)
			return SelectStmt{}, nil, err
		}
	}
	// [TYPE] text | file
	if q, ok := req.Params["type"]; ok && q != nil {
		switch q := q.(type) {
		case string:
			stmt = stmt.Where("m.type=" + param(q))
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
}

// ChannelList scan sql.Rows dataset tuples.
// Zero or negative `size` implies NOLIMIT startegy.
// MAY: Return len([]*Cahnnels) == (size+1)
// which indicates that .`next` result page exist !
func MessageList(rows *sql.Rows, limit int) ([]*Message, error) {

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
		obj  *Message                                // cursor: target for current tuple
		doc  *Document                               //
		plan = make([]func() interface{}, len(cols)) // , len(cols))
	)

	for c, col := range cols {
		switch col {

		case "id":
			plan[c] = func() interface{} { return &obj.ID } // NOTNULL (!)
		case "channel_id":
			plan[c] = func() interface{} { return ScanString(&obj.ChannelID) } // NULL: *sql.NullString{}
		case "conversation_id":
			plan[c] = func() interface{} { return &obj.ConversationID } // NOTNULL (!)

		case "created_at":
			plan[c] = func() interface{} { return ScanDatetime(&obj.CreatedAt) } // NOTNULL (!)
		case "updated_at":
			plan[c] = func() interface{} { return ScanDatetime(&obj.UpdatedAt) } // NULL: **time.Time

		case "type":
			plan[c] = func() interface{} { return &obj.Type } // NOTNULL (!)
		case "text":
			plan[c] = func() interface{} { return ScanString(&obj.Text) } // NULL: *sql.NullString
		case "file_id":
			plan[c] = func() interface{} { return ScanInteger(&doc.ID) }
		case "file_url":
			plan[c] = func() interface{} { return ScanString(&doc.URL) }
		case "file_size":
			plan[c] = func() interface{} { return ScanInteger(&doc.Size) }
		case "file_type":
			plan[c] = func() interface{} { return ScanString(&doc.Type) }
		case "file_name":
			plan[c] = func() interface{} { return ScanString(&doc.Name) }

		case "reply_to":
			plan[c] = func() interface{} { return ScanInteger(&obj.ReplyToMessageID) }
		case "forward_id":
			plan[c] = func() interface{} { return ScanInteger(&obj.ForwardFromMessageID) }

		// case "variables":       proj[c] = func() interface{} { return ScanProperties(&obj.Variables) }
		case "variables":
			plan[c] = func() interface{} { return &obj.Variables }

		default:

			return nil, errs.Errorf("sql: scan %T column %q not supported", obj, col)

		}
	}

	dst := make([]interface{}, len(cols)) // , len(cols))

	var (
		page []Message  // mempage
		list []*Message // results
	)

	if limit > 0 {

		page = make([]Message, limit)
		list = make([]*Message, 0, limit+1)

	}

	// var (

	// 	err error
	// 	row *Message
	// )

	for rows.Next() {

		if 0 < limit && len(list) == limit {
			// indicate next page exists !
			// if rows.Next() {
			list = append(list, nil)
			// }
			break
		}

		if len(page) != 0 {
			obj = &page[0]
			page = page[1:]
		} else {
			obj = new(Message)
		}

		if doc == nil {
			doc = new(Document)
		}

		for c, bind := range plan {
			dst[c] = bind()
		}

		err = rows.Scan(dst...)

		if err != nil {
			break
		}

		// region: check file document attached
		if doc.ID != 0 {
			obj.File, doc = doc, nil
		}
		// endregion

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

func schemaMessageError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *pgconn.PgError:
		// TODO: handle shema-specific errors, constraints ...
	}
	return err
}

// GetMessages unified for [D]ata[C]onnection sql[x].QueryerContext interface
func GetMessages(dcx sqlx.ExtContext, req *SearchOptions) ([]*Message, error) {

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

	stmt, args, err := MessageRequest(req)
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
	list, err := MessageList(rows, req.GetSize())
	// Error ?
	err = schemaMessageError(err)

	if err != nil {
		return list, err
	}

	return list, err
}

func (repo *sqlxRepository) GetMessage(ctx context.Context, oid int64, senderChatID, targetChatID string, searchProps map[string]string) (*Message, error) {

	search := SearchOptions{
		Operation: Operation{
			ID:      "",
			Time:    time.Now(),
			Context: ctx, // cancellation
		},
		// prepare filter(s)
		Params: make(map[string]interface{}),
		Fields: []string{"id", "*"}, // NOT applicable
		Sort:   []string{},
		Page:   0,
		Size:   1, // GET(!)
	}

	if oid != 0 {
		search.Params["id"] = oid
	}

	// [FROM] channelID
	if senderChatID != "" {
		search.Params["sender.id"] = senderChatID
	}

	// [TO] conversationID
	if targetChatID != "" {
		search.Params["chat.id"] = targetChatID
	}

	if searchProps != nil {
		delete(searchProps, "")
		if len(searchProps) != 0 {
			search.Params["props"] = searchProps
		}
	}

	// PERFORM SELECT ...
	list, err := GetMessages(repo.db, &search)

	if err != nil {
		repo.log.Error("Failed lookup DB chat.message",
			"error", err,
			"id", oid,
		)
		return nil, err
	}

	var obj *Message
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

	if obj == nil || (oid != 0 && oid != obj.ID) {
		obj = nil // NOT FOUND !
	}

	return obj, nil
}

func (repo *sqlxRepository) DeleteMessages(ctx context.Context, mid ...int64) (n int64, err error) {

	var e int
	for e = 0; e < len(mid) && mid[e] > 0; e++ {
		// lookup zero identifier(s) spec
	}
	if e < len(mid) {
		// Found ZERO (!)
		req := make([]int64, e, len(mid)-1)
		copy(req, mid[0:e])
		for e++; e < len(mid); e++ {
			if mid[e] > 0 {
				req = append(req, mid[e])
			}
		}
		mid = req
	}

	if len(mid) == 0 {
		return 0, nil // Nothing !
	}

	params := make([]interface{}, 1)
	query := "DELETE FROM chat.message WHERE message.id = "
	if len(mid) == 1 {
		params[0] = mid[0]
		query += "$1"
	} else {
		var keys pgtype.Int8Array
		_ = keys.Set(mid)
		params[0] = &keys
		query += "ANY($1)" // ::[]int8
	}
	query += ";"

	res, re := repo.db.ExecContext(ctx, query, params...)
	if err = re; err != nil {
		return 0, err
	}

	n, err = res.RowsAffected()
	if err != nil {
		return // 0, err
	}

	return // n, nil
}

/*func (repo *sqlxRepository) GetMessages(search *SearchOptions) (*Message, error) {

	// search := SearchOptions{
	// 	// prepare filter(s)
	// 	Params: map[string]interface{}{
	// 		"id": id, // MUST
	// 	},
	// 	Fields: []string{"id","*"}, // NOT applicable
	// 	Sort:   []string{},
	// 	Page:   0,
	// 	Size:   1, // GET(!)
	// }

	search.Page = 0
	search.Size = 1

	// PERFORM SELECT ...
	list, err := GetMessages(repo.db, search)

	if err != nil {
		repo.log.Error().Err(err).Str("id", oid).
			Msg("Failed lookup DB chat.message")
		return nil, err
	}

	var obj *Message
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

	if obj == nil || oid != obj.ID {
		obj = nil // NOT FOUND !
	}

	return obj, nil
}*/

var protojsonCodec = struct {
	protojson.MarshalOptions
	protojson.UnmarshalOptions
}{
	MarshalOptions: protojson.MarshalOptions{
		Indent:          "",    // compact
		Multiline:       false, // compact
		AllowPartial:    true,  // contract
		UseProtoNames:   true,  // contract
		UseEnumNumbers:  true,  // compact
		EmitUnpopulated: false, // compact
	},
	UnmarshalOptions: protojson.UnmarshalOptions{
		AllowPartial:   true, // contract
		DiscardUnknown: true, // contract
	},
}

func contentJSONB(msg *Message) EvalFunc {
	return func() (driver.Value, error) {
		codec := protojsonCodec
		jsonb, err := codec.Marshal(&msg.Content)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(jsonb, jsonNullObject) {
			return nil, nil // NULL
		}
		// pgtype.JSONB
		return jsonb, nil
	}
}

func SaveMessage(ctx context.Context, dcx sqlx.ExtContext, msg *Message) (err error) {

	var doc Document
	if msg.File != nil {
		doc = *(msg.File)
	}
	// msg.Text.Valid = len(msg.Text.String) != 0

	if msg.UpdatedAt.IsZero() {

		msg.ID = 0 // NOTE: generated by DB schema sequence

		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = app.CurrentTime() // time.Now().UTC()
			// m.UpdatedAt.IsZero() // NOTE: NEW ! NOT an EDIT !
		}
		msg.CreatedAt = msg.CreatedAt.Truncate(app.TimePrecision)

		// var props map[string]string
		// if len(msg.Variables) != 0 {
		// 	_ = json.Unmarshal(msg.Variables, &props)
		// }

		err = sqlx.GetContext(
			// context, result
			ctx, dcx, &msg.ID,
			// statement query !
			psqlMessageNewQ,
			// statement params ...
			msg.CreatedAt.UTC(),       // $1  - SEND timestamp
			NullString(msg.ChannelID), // $2  - FROM: sender channel_id
			msg.ConversationID,        // $3  - TO: session conversation_id
			// ---------------------------------------------------------------------------
			msg.Type,              // $4  - NEW message content type: text or file
			msg.Text,              // $5  - NEW message text or document caption
			NullInteger(doc.ID),   // $6  - NEW message file document id
			NullString(doc.URL),   // $7  - NEW message file document URL
			NullInteger(doc.Size), // $8  - NEW message file document size
			NullString(doc.Type),  // $9  - NEW message file document type
			NullString(doc.Name),  // $10 - NEW message file document name
			// ---------------------------------------------------------------------------
			NullInteger(msg.ReplyToMessageID),     // $11 - NEW message is reply to some previous message id
			NullInteger(msg.ForwardFromMessageID), // $12 - NEW message is forwarding from previous message id
			NullMetadata(msg.Variables),           // $13 - NEW message extra properties
			contentJSONB(msg),                     // $14 - NEW message raw content{keyboard|postback|contact|...}
			// NullMetadata(props),                   // $12 - NEW message extra properties
			// msg.Variables,                         // $12 - NEW message extra properties
		)

		if err == nil && msg.ID == 0 {
			// err = ...
			panic("unreachable code")
		}

		if err != nil {
			// handle and disclose DB schema specific errors here ...
			return err
		}

		return err // nil
	}

	// NOTE: (!msg.UpdatedAt.IsZero()) EDIT !
	// MUST: be manually set before calling this method
	// to indicate EDIT operation to be performed !
	if msg.ID == 0 {
		panic("postgres: edit message <zero> id")
	}
	if msg.ChannelID == "" {
		// if msg.ChannelID.String == "" {
		panic("postgres: edit message for <nil> channel id")
	}
	// msg.ChannelID.Valid = true
	msg.UpdatedAt = msg.UpdatedAt.Truncate(app.TimePrecision)
	// PERFORM: UPDATE
	var oid int64
	err = sqlx.GetContext(
		// context, result
		ctx, dcx, &oid,
		// statement query !
		psqlMessageEditQ,
		// statement params ...
		msg.ID,                    // $1 - original message_id
		NullString(msg.ChannelID), // $2 - original sent from chat_id (author)
		msg.UpdatedAt.UTC(),       // $3 - edit date timestamp, updated_at
		// message content changes
		msg.Text,              // $4 - EDIT: NEW message text or document caption
		NullInteger(doc.ID),   // $5 - EDIT: NEW message file document id
		NullString(doc.URL),   // $6 - EDIT: NEW message file document URL
		NullInteger(doc.Size), // $7 - EDIT: NEW message file document size
		NullString(doc.Type),  // $8 - EDIT: NEW message file document MIME type
		NullString(doc.Name),  // $9 - EDIT: NEW message file document name
	)

	if err == nil && oid != msg.ID {
		panic("postgres: edit message not found")
	}

	if err != nil {
		// handle and disclose DB schema specific errors here ...
		return err
	}

	return err // nil
}

func (repo *sqlxRepository) SaveMessage(ctx context.Context, msg *Message) error {
	return SaveMessage(ctx, repo.db, msg)
}

func (repo *sqlxRepository) BindMessage(ctx context.Context, oid int64, vars map[string]string) error {

	_, err := repo.db.ExecContext(ctx,
		"UPDATE chat.message SET variables = $2 WHERE id = $1",
		oid, NullMetadata(vars),
	)

	return err
}

// Statement to save historical (SENT) message
//
// $1 - SENT: message sent timestamp
// $2 - FROM: message sender chat_id
// $3 - SENT: message TO conversation id -- FIXME: this is the chat@bot chat channel id
//
// $4 - SENT: NEW message content type: text or file
// $5 - SENT: NEW message text or document caption
// $6 - SENT: NEW message file document id
// $7 - SENT: NEW message file document URL
// $8 - SENT: NEW message file document size
// $9 - SENT: NEW message file document MIME type
// $10 - SENT: NEW message file document name
//
// $11 - SENT: NEW message as reply to previous message id
// $12 - SENT: NEW message as forward from previous message id
// $13 - SENT: NEW message extra properties
// $14 - SENT: NEW message raw content{keyboard|postback|contact|...}
const psqlMessageNewQ =
// Mark channel as 'just seen'; it's either chat@channel or chat@workflow
`WITH seenUser AS (UPDATE chat.channel SET updated_at=$1 WHERE id=$2)
, seenBot AS (UPDATE chat.conversation SET updated_at=$1 WHERE id=coalesce($2,$3))
INSERT INTO chat.message (
  created_at, updated_at, channel_id, conversation_id,
  type, text, file_id, file_url, file_size, file_type, file_name,
  reply_to, forward_id, variables, content
) VALUES (
  $1, NULL, NULLIF($2,$3), $3,
  $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14
) RETURNING id`

// Statement to edit historical (SENT) message
//
// $1 - EDIT: original message_id
// $2 - FROM: original message sender chat_id
//
// $3 - EDIT: timestamp updated at
// $4 - EDIT: NEW message text or document caption
// $5 - EDIT: NEW message file document id
// $6 - EDIT: NEW message file document URL
// $7 - EDIT: NEW message file document size
// $8 - EDIT: NEW message file document MIME type
// $9 - EDIT: NEW message file document name
const psqlMessageEditQ = `WITH seenUser AS (UPDATE chat.channel SET updated_at=$3 WHERE id=$2)
, seenBot AS (UPDATE chat.conversation SET updated_at=$3 WHERE id=$2)
UPDATE chat.message SET
  updated_at = $3, text = $4, file_id = $5, file_url = $6,
  file_size = $7, file_type = $8, file_name = $9
 WHERE id = $1 AND channel_id = $2
RETURNING id` // to keep same results with Save() message operation
