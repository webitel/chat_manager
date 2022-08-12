package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	"github.com/rs/zerolog"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/bot"

	dbl "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/chat_manager/store/postgres"
)

type pgsqlBotStore struct {
	log *zerolog.Logger
	dbo []*sql.DB // cluster
}

func (s *pgsqlBotStore) primary() *sql.DB {
	return s.dbo[0]
}

// random defines how to create the random number.
func random(min, max int) int {
	// rand.Seed: ensures that the number that is generated is random(almost).
	rand.Seed(app.CurrentTime().UnixNano())
	return rand.Intn(max-min) + min
}

func (s *pgsqlBotStore) secondary() *sql.DB {
	// first is primary (master)
	if n := len(s.dbo); n > 1 {
		return s.dbo[random(1, n)]
	}
	return s.primary()
}

var _ bot.Store = (*pgsqlBotStore)(nil)

// NewBotStore returns PostgreSQL chatbots store
func NewBotStore(log *zerolog.Logger, primary *sql.DB, secondary ...*sql.DB) bot.Store {

	dbo := make([]*sql.DB, len(secondary)+1)

	dbo[0] = primary
	copy(dbo[1:], secondary)

	return &pgsqlBotStore{
		log: log,
		dbo: dbo,
	}
}

func (s *pgsqlBotStore) Create(ctx *app.CreateOptions, obj *bot.Bot) error {

	stmtQ, params, err := createBotRequest(ctx, obj)

	if err != nil {
		return err
	}

	query, args, err := stmtQ.ToSql()

	if err != nil {
		return err
	}

	query, args, err = NamedParams(query, params)

	if err != nil {
		return err
	}

	db := s.primary()

	rows, err := db.QueryContext(
		ctx.Context.Context,
		query, args...,
	)

	err = schemaBotError(err)

	if err != nil {
		return err
	}

	defer rows.Close()

	res, err := searchBotResults(rows, 1)

	if err != nil {
		return err
	}

	switch len(res) {
	case 1:
		obj.Id = res[0].GetId()
		obj.Flow = res[0].GetFlow()
		// err = mergeProto(obj, res[0], "id", "flow")
	case 0:
		err = errors.InternalServerError(
			"chat.bot.create.no_result",
			"postgres: no result",
		)
	default:
		err = errors.Conflict(
			"chat.bot.create.conflict",
			"postgres: too much records",
		)
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *pgsqlBotStore) Search(ctx *app.SearchOptions) ([]*bot.Bot, error) {

	stmtQ, params, err := searchBotRequest(ctx)

	if err != nil {
		return nil, err
	}

	query, args, err := stmtQ.ToSql()

	if err != nil {
		return nil, err
	}

	query, args, err = NamedParams(query, params)

	if err != nil {
		return nil, err
	}

	db := s.secondary()

	rows, err := db.QueryContext(
		ctx.Context.Context,
		query, args...,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return searchBotResults(rows, ctx.GetSize())
}

func (s *pgsqlBotStore) Update(req *app.UpdateOptions, set *bot.Bot) error {

	stmtQ, params, err := updateBotRequest(req, set)

	if err != nil {
		return err
	}

	query, args, err := stmtQ.ToSql()

	if err != nil {
		return err
	}

	query, args, err = NamedParams(query, params)

	if err != nil {
		return err
	}

	db := s.primary() // +W

	rows, err := db.QueryContext(
		req.Context.Context,
		query, args...,
	)

	err = schemaBotError(err)

	if err != nil {
		return err
	}

	defer rows.Close()

	res, err := searchBotResults(rows, 1)

	if err != nil {
		return err
	}

	switch len(res) {
	case 1:
		app.MergeProto(set, res[0], req.Fields...)
	case 0:
		err = errors.InternalServerError(
			"chat.bot.update.no_result",
			"postgres: no result",
		)
	default:
		err = errors.Conflict(
			"chat.bot.update.conflict",
			"postgres: too much records",
		)
	}

	if err != nil {
		return err
	}

	return nil
}

func (s *pgsqlBotStore) Delete(req *app.DeleteOptions) (int64, error) {

	// if ctx.Permanent {
	// 	// DELETE
	// } else {
	// 	// UPDATE SET enabled = false
	// }

	delete := psql.Delete("chat.bot") // postgres.PGSQL

	paramIDs := pgtype.Int8Array{}
	err := paramIDs.Set(req.ID)

	if err != nil {
		return 0, err
	}

	params := params{
		"dc": req.Creds.GetDc(),
		"id": &paramIDs,
	}

	delete = delete.Where("bot.dc = :dc")
	delete = delete.Where("bot.id = ANY(:id)")

	query, args, err := delete.ToSql()

	if err != nil {
		return 0, err
	}

	query, args, err = NamedParams(query, params)

	if err != nil {
		return 0, err
	}

	tx := s.primary()

	res, err := tx.ExecContext(
		req.Context.Context,
		query, args...,
	)

	if err != nil {
		return 0, err
	}

	count, err := res.RowsAffected()

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *pgsqlBotStore) AnalyticsActiveBotsCount(ctx context.Context, pdc int64) (n int, err error) {

	// $1 - [p]rimary [d]omain [c]omponent ID
	const pgsqlTenantBotsCountQ = `WITH tenant AS (
  SELECT customer_id FROM directory.wbt_domain WHERE dc = $1
)
SELECT count(id)
  FROM chat.bot
  JOIN tenant ON true
  JOIN directory.wbt_domain srv on bot.dc = srv.dc AND srv.customer_id = tenant.customer_id
 WHERE bot.enabled
`
	var row *sql.Rows
	row, err = s.secondary().QueryContext(
		// context, statement,
		ctx, pgsqlTenantBotsCountQ,
		// params...
		pdc,
	)

	if err != nil {
		return 0, err
	}

	defer row.Close()

	if !row.Next() {
		err = row.Err()
	} else {
		err = row.Scan(&n)
	}

	if err != nil {
		// if err == sql.ErrNoRows {}
		return 0, err
	}

	return // n, nil
}

func nullChatUpdates(src *bot.ChatUpdates) *bot.ChatUpdates {
	if src != nil {
		for _, s := range []string{
			src.Title, src.Close,
			src.Join, src.Left,
		} {
			if s != "" {
				return src
			}
		}
		// Zero(!) All fields are zero
		src = nil
	}
	return src
}

func createBotRequest(req *app.CreateOptions, obj *bot.Bot) (stmtQ SelectStmt, params params, err error) {

	deref := app.SearchOptions{
		Context: req.Context,
		Fields:  req.Fields,
		Size:    1,
	}

	stmtQ, params, err = searchBotRequest(&deref)

	if err != nil {
		return // stmt, params, err
	}

	stmtQ = stmtQ.
		Prefix("WITH created AS (" +
			"INSERT INTO chat.bot (dc, uri, name, flow_id, enabled, updates, provider, metadata, created_at, created_by, updated_at, updated_by)" +
			" VALUES (:dc, :uri, :name, :flow_id, :enabled, :updates, :provider, :metadata, :created_at, :created_by, :created_at, :created_by)" +
			" RETURNING bot.*" +
			")",
		).
		From("created bot")

	params["dc"] = obj.GetDc().GetId()
	params.set("uri", obj.GetUri())
	params.set("name", obj.GetName())
	params.set("flow_id", obj.GetFlow().GetId())
	params.set("enabled", obj.GetEnabled())
	params.set("updates", dbl.NullJSONBytes(
		nullChatUpdates(obj.GetUpdates()),
	))
	params.set("provider", obj.GetProvider())
	params.set("metadata", dbl.NullJSONBytes(
		obj.GetMetadata(),
	))
	params.set("created_by", obj.GetCreatedBy().GetId())
	params.set("created_at", dbl.NullTimestamp(
		obj.GetCreatedAt(),
	))

	return // stmt, params, nil
}

func searchBotRequest(req *app.SearchOptions) (stmtQ SelectStmt, params params, err error) {

	// ----- FROM -----
	params = map[string]interface{}{}
	stmtQ = psql.Select().From("chat.bot")
	// ----- REALM -----
	if dc := req.Creds.GetDc(); dc != 0 {
		params.set("dc", dc)
		stmtQ = stmtQ.Where("bot.dc = :dc")
	}

	const (
		joinDomains uint8 = (1 << iota)
		joinFlows
		joinCreator
		joinUpdator
	)

	var (
		join uint8
	)

	// INNER JOIN directory.wbt_domain AS srv
	joinDomain := func() {
		if join&joinDomains != 0 {
			return // already
		}
		join |= joinDomains
		stmtQ = stmtQ.Join("directory.wbt_domain srv on srv.dc = bot.dc")
	}
	// LEFT JOIN flow.acr_routing_scheme AS flow
	joinFlow := func() {
		if join&joinFlows != 0 {
			return // already
		}
		join |= joinFlows
		stmtQ = stmtQ.LeftJoin("flow.acr_routing_scheme flow on flow.id = bot.flow_id and flow.domain_id = bot.dc")
	}
	// LEFT JOIN directory.wbt_auth AS created
	joinCreated := func() {
		if join&joinCreator != 0 {
			return // already
		}
		join |= joinCreator
		stmtQ = stmtQ.LeftJoin("directory.wbt_auth created on created.id = bot.created_by")
	}
	// LEFT JOIN directory.wbt_auth AS updated
	joinUpdated := func() {
		if join&joinUpdator != 0 {
			return // already
		}
		join |= joinUpdator
		stmtQ = stmtQ.LeftJoin("directory.wbt_auth updated on updated.id = bot.updated_by")
	}

	// SELECT
	if len(req.Fields) == 0 {
		req.Fields = []string{
			"id", // "dc",
			"uri", "name",
			"flow", "enabled",
			"provider", "metadata",
			"updates", // template of updates
			"created_at", "created_by",
			"updated_at", "updated_by",
		}
	}

	for _, att := range req.Fields {
		switch att {
		case "dc":
			joinDomain() // INNER JOIN directory.wbt_domain AS srv
			stmtQ = stmtQ.Columns(
				"bot.dc",
				"srv.name realm",
			)
		case "id":
			stmtQ = stmtQ.Column("bot.id")
		case "uri":
			stmtQ = stmtQ.Column("bot.uri")
		case "name":
			stmtQ = stmtQ.Column("bot.name")
		case "flow":
			joinFlow() // LEFT JOIN flow.acr_routing_scheme AS flow
			stmtQ = stmtQ.Columns(
				"bot.flow_id",
				"flow.name flow",
			)
		case "enabled":
			stmtQ = stmtQ.Column("bot.enabled")
		case "updates":
			stmtQ = stmtQ.Column("bot.updates")
		case "provider":
			stmtQ = stmtQ.Column("bot.provider")
		case "metadata":
			stmtQ = stmtQ.Column("bot.metadata")
		case "created_at":
			stmtQ = stmtQ.Column("bot.created_at")
		case "created_by":
			joinCreated()
			stmtQ = stmtQ.Columns(
				"bot.created_by created_id",
				"coalesce(created.name, created.auth) created_by",
			)
		case "updated_at":
			stmtQ = stmtQ.Column("bot.updated_at")
		case "updated_by":
			joinUpdated()
			stmtQ = stmtQ.Columns(
				"bot.updated_by updated_id",
				"coalesce(updated.name, updated.auth) updated_by",
			)

		default:
			err = errors.BadRequest(
				"chat.bot.search.field.invalid",
				"chatbot: invalid attribute .%s to select",
				att,
			)
		}
	}

	// ------ FILTER(s) ------
	var (
		oid int64 // GET
	)

	// BY: ?id=
	if size := len(req.ID); size != 0 {
		// Normalize requested size
		if req.Size = size; size == 1 {
			oid = req.ID[0] // GET
			params.set("id", oid)
			stmtQ = stmtQ.Where("bot.id = :id")
		} else {
			param := pgtype.Int8Array{}
			err = param.Set(req.ID)
			if err != nil {
				// ERR: failed to set param
				return // stmt, params, !err
			}
			params.set("id", &param)
			stmtQ = stmtQ.Where("bot.id = ANY(:id)")
		}
	}

	// BY: ?q=
	if term := req.Term; term != "" && !app.IsPresent(term) {
		params.set("q", postgres.Substring(app.Substring(term)))
		joinFlow() // LEFT JOIN flow.acr_routing_scheme flow
		// matchingRule: caseIgnoreSubstringsMatch
		stmtQ = stmtQ.Where(squirrel.Or{
			squirrel.Expr("bot.name ILIKE :q"),
			squirrel.Expr("bot.uri ILIKE :q"),
			squirrel.Expr("bot.provider ILIKE :q"),
			squirrel.Expr("flow.name ILIKE :q"),
		})
	}

	for name, assert := range req.Filter {
		switch name {
		case "uri":
			switch data := assert.(type) {
			case string:
				params.set("uri", data)
				stmtQ = stmtQ.Where("bot.uri LIKE :uri")
			default:
				err = errors.BadRequest(
					"chat.bot.search.uri.invalid",
					"chatbot: invalid URI filter %T type",
					assert,
				)
			}
		case "name":
			switch data := assert.(type) {
			case string:
				if data = strings.TrimSpace(data); data == "" {
					continue
				}
				params.set("name", postgres.Substring(app.Substring(data)))
				stmtQ = stmtQ.Where(`bot."name" ILIKE :name COLLATE "default"`)
			default:
				err = errors.BadRequest(
					"chat.bot.search.name.invalid",
					"chatbot: invalid filter=%s type=%T",
					name, assert,
				)
			}
		case "flow":
		case "enabled":
			switch data := assert.(type) {
			case bool:
				expr := "bot.enabled"
				if !data {
					expr = "NOT " + expr
				}
				stmtQ = stmtQ.Where(expr)
			default:
				err = errors.BadRequest(
					"chat.bot.search.enabled.invalid",
					"chatbot: invalid filter=%s value=%T type",
					name, assert,
				)
			}
		case "provider":
			switch data := assert.(type) {
			case string:
				if data = strings.TrimSpace(data); data == "" {
					continue
				}
				params.set("typeof", postgres.Substring(app.Substring(data)))
				stmtQ = stmtQ.Where(`bot.provider ILIKE :typeof COLLATE "default"`)
			case []string:
				param := pgtype.TextArray{}
				_ = param.Set(data)
				params.set("typeof", data)
				stmtQ = stmtQ.Where(`bot.provider ILIKE ANY(:typeof)`)
			default:
				err = errors.BadRequest(
					"chat.bot.search.provider.invalid",
					"chatbot: invalid filter=%s type=%T",
					name, assert,
				)
			}
		default:
		}
	}

	// ------ ORDER BY ------
	sort := app.FieldsFunc(
		req.Order, app.InlineFields,
	)
	if len(sort) == 0 {
		sort = []string{"id"}
	}
	for _, att := range sort {

		if len(att) == 0 {
			continue // omitempty (400)
		}
		order := " ASC" // default
		switch att[0] {
		// NOT URL-encoded PLUS '+' char
		// we will get as SPACE ' ' char
		case '+', ' ': // be loyal ...
			att = att[1:]
		case '-', '!':
			att = att[1:]
			order = " DESC"
		}

		switch att {
		case "dc":
			joinDomain()
			att = "srv.name"

		case "id", "uri", "name", "enabled",
			"provider", "created_at", "updated_at":
			att = "bot." + att

		case "flow":
			joinFlow() // LEFT JOIN flow.acr_routing_scheme AS flow
			att = "flow.name"

		// case "metadata":

		case "created_by":
			joinCreated()
			att = "coalesce(created.name, created.auth)"

		case "updated_by":
			joinUpdated()
			att = "coalesce(updated.name, updated.auth)"

		default:
			err = errors.BadRequest(
				"chat.bot.search.sort.invalid",
				"chatbot: invalid attribute .%s to sort",
				att,
			)
			return // stmt, params, err(!)
		}

		stmtQ = stmtQ.OrderBy(att + order)
	}

	// ------ OFFSET|LIMIT ------
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			stmtQ = stmtQ.Offset((uint64)((page - 1) * (size)))
		}
		// LIMIT (size+1) -- to indicate whether there are more result entries
		stmtQ = stmtQ.Limit((uint64)(size + 1))
	}

	return
}

func searchBotResults(rows *sql.Rows, limit int) ([]*bot.Bot, error) {

	// Fetch result entries
	cols, err := rows.Columns()

	if err != nil {
		return nil, err
	}

	// Build convertion(s)
	var (
		obj *bot.Bot                                // target: scan result entry
		row = make([]func() interface{}, len(cols)) // projection: index[column]obj.value
	)

	for i, col := range cols {
		switch col {
		case "dc", "realm":
			row[i] = func() interface{} {
				return ScanRefer(&obj.Dc) // **bot.Refer
			}
		case "id":
			row[i] = func() interface{} {
				return &obj.Id // *int64
			}
		case "uri":
			row[i] = func() interface{} {
				return &obj.Uri // *string
			}
		case "name":
			row[i] = func() interface{} {
				return &obj.Name // *string
			}
		case "flow_id", "flow":
			row[i] = func() interface{} {
				return ScanRefer(&obj.Flow) // **bot.Refer
			}
		case "enabled":
			row[i] = func() interface{} {
				return &obj.Enabled // *bool NOTNULL
			}
		case "updates":
			row[i] = func() interface{} { // *bot.ChatUpdates NULL
				return ScanFunc(func(src interface{}) error {
					if src == nil {
						obj.Updates = nil
						return nil
					}

					dst := obj.Updates
					if dst == nil {
						dst = new(bot.ChatUpdates)
					}

					err := ScanJSON(dst)(src)
					if err != nil {
						return err
					}

					obj.Updates = nullChatUpdates(dst)
					return nil
				})
			}
		case "provider":
			row[i] = func() interface{} {
				return &obj.Provider // *string NOTNULL
			}
		case "metadata":
			row[i] = func() interface{} {
				return dbl.ScanJSONBytes(&obj.Metadata) // *map[string]string
			}
		case "created_at":
			row[i] = func() interface{} {
				return dbl.ScanTimestamp(&obj.CreatedAt) // *int64
			}
		case "created_id", "created_by":
			row[i] = func() interface{} {
				return ScanRefer(&obj.CreatedBy) // **bot.Refer
			}
		case "updated_at":
			row[i] = func() interface{} {
				return dbl.ScanTimestamp(&obj.UpdatedAt) // *int64
			}
		case "updated_id", "updated_by":
			row[i] = func() interface{} {
				return ScanRefer(&obj.UpdatedBy) // **bot.Refer
			}
		default:
			return nil, errors.InternalServerError(
				"chat.bot.search.result.error",
				"postgres: invalid column .%s name",
				col,
			)
		}
	}

	var (
		page []bot.Bot
		list []*bot.Bot
		vals = make([]interface{}, len(cols)) // scan values
	)

	if limit > 0 {
		page = make([]bot.Bot, limit)
		list = make([]*bot.Bot, 0, limit+1)
	}

	for rows.Next() {

		if 0 < limit && limit == len(list) {
			// We reached the limit result count !
			// Mark the result
			list = append(list, nil)
			break
		}

		// Alloc result entry
		if len(page) != 0 {
			obj = &page[0]
			page = page[1:]
		} else {
			obj = new(bot.Bot)
		}

		// Build row2entry projection
		for c, val := range row {
			vals[c] = val()
		}

		// Scan entry values ...
		err = rows.Scan(vals...)

		if err != nil {
			break
		}

		// Result entry !
		list = append(list, obj)
	}

	if err == nil {
		err = rows.Err()
	}

	return list, err
}

func updateBotRequest(req *app.UpdateOptions, set *bot.Bot) (stmt SelectStmt, params params, err error) {

	// UPDATE
	update := psql.Update("chat.bot")

	params = map[string]interface{}{
		"dc": req.Creds.GetDc(),
		"id": set.GetId(),
	}

	update = update.Where("bot.id = :id")
	update = update.Where("bot.dc = :dc")

	fields := req.Fields
	if len(fields) == 0 {
		// default: set
		fields = []string{
			// "id", "dc", "uri",
			"name", "flow", "enabled",
			// "provider",
			"metadata", "updates",
			// "created_at", // "created_by",
			// "updated_at", "updated_by",
		}
	}

	for _, att := range fields {
		// READONLY
		switch att {
		case "id", "dc", "uri", "provider",
			"created_at", "created_by",
			"updated_at", "updated_by":

			err = errors.BadRequest(
				"chat.bot.update.field.readonly",
				"chatbot: update .%s; attribute is readonly",
				att,
			)
			return // stmt, params, err
		// EDITABLE
		// case "uri":
		// 	params.set("uri", set.GetUri())
		// 	update = update.Set("uri", dbl.Expr(":uri"))
		case "name":
			params.set("name", set.GetName())
			update = update.Set("name", dbl.Expr(":name"))
		case "flow", "flow.id", "flow_id":
			params.set("flow_id", set.GetFlow().GetId())
			update = update.Set("flow_id", dbl.Expr(":flow_id"))
		case "enabled":
			params.set("enabled", set.GetEnabled())
			update = update.Set("enabled", dbl.Expr(":enabled"))
		case "updates":
			params.set("updates", dbl.NullJSONBytes(
				nullChatUpdates(set.GetUpdates())),
			)
			update = update.Set("updates", dbl.Expr(":updates"))
		case "metadata":
			params.set("metadata", dbl.NullJSONBytes(
				set.GetMetadata(),
			))
			update = update.Set("metadata", dbl.Expr(":metadata"))
		// INVALID
		default:
			err = errors.BadRequest(
				"chat.bot.update.fields.invalid",
				"chatbot: update .%s; attribute is unknown",
				att,
			)
			return // stmt, params, err
		}
	}
	// Set normalized
	req.Fields = fields

	// FIXME: From given `set` object ?
	// -OR- from context authorization ?
	params.set("date", dbl.NullTimestamp(set.UpdatedAt)) // req.Timestamp())
	params.set("user", set.GetUpdatedBy().GetId())       // req.Creds.GetUserId())

	update = update.Set("updated_at", dbl.Expr(":date"))
	update = update.Set("updated_by", dbl.Expr(":user"))

	// Normalize updated values
	var updateBotQ string
	updateBotQ, _, err = update.ToSql()

	if err != nil {
		return // stmt, params, err
	}

	// SELECT FROM UPDATE !
	fetch := app.SearchOptions{
		Context: req.Context,
		Fields:  fields,
		Size:    1,

		// ID: []int64{set.GetId()},
	}

	stmt, _, err = searchBotRequest(&fetch)
	// params: [id, dc]

	if err != nil {
		return // stmt, params, err
	}

	stmt = stmt.Prefix(
		"WITH updated AS (" +
			updateBotQ +
			" RETURNING bot.*" +
			")",
	).
		From("updated bot")

	return // stmt, params, nil
}

func schemaBotError(err error) error {
	if err == nil {
		return nil
	}

	switch re := err.(type) {
	case *errors.Error:
		return re
	case *pgconn.PgError:
		return postgresErrorT(re)
	default:
		err = errors.InternalServerError(
			"chat.bot.store.error",
			re.Error(),
		)
	}

	return err
}

func postgresErrorT(err *pgconn.PgError) error {
	if err == nil {
		return nil
	}
	// Message:"duplicate key value violates unique constraint "bot_uri_uindex""
	// Detail:"Key (uri)=(/chat/ws8/webichat) already exists."
	// Hint:""
	// Position:0
	// InternalPosition:0
	// InternalQuery:""
	// Where:""
	// SchemaName:"chat"
	// TableName:"bot"
	// ColumnName:""
	// DataTypeName:""
	// ConstraintName:"bot_uri_uindex"
	// File:"nbtinsert.c"
	// Line:570
	// Routine:"_bt_check_unique"
	// See: https://postgrespro.com/docs/postgresql/12/errcodes-appendix
	switch err.Code {
	case "23502": // not_null_violation
	case "23503": // foreign_key_violation
	case "23505": // unique_violation
		switch err.ConstraintName {
		case "bot_uri_uindex":
			return errors.BadRequest(
				"chat.bot.uri.unique_violation",
				"chatbot: duplicate URI registration",
			)
		default:
		}
	}

	return err
}

func ScanRefer(dst **bot.Refer) dbl.ScanFunc {
	return func(src interface{}) error {
		if src == nil {
			return nil
		}

		val := *(dst)
		ref := func() *bot.Refer {
			if val == nil {
				val = new(bot.Refer)
			}
			return val
		}

		switch data := src.(type) {
		case int64:
			ref().Id = data
		case string:
			ref().Name = data
		default:
			return fmt.Errorf("database: convert %[1]T value to %[2]T type", src, ref)
		}

		*(dst) = val
		return nil
	}
}
