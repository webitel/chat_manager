package sqlxrepo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
	"github.com/lib/pq"
	"github.com/micro/micro/v3/service/errors"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/store/postgres"
)

type chatCustomersArgs struct {
	// Paging
	Page int
	Size int

	// Output
	Fields []string

	// Authorization
	PDC  int64
	Self int64

	// Arguments
	Q    string
	ID   []string
	Via  *pb.Peer
	Type string
}

func getCustomersInput(req *app.SearchOptions) (args chatCustomersArgs, err error) {

	// NOTE: Set Q and if Q == '*' ignore that
	args.Q = req.Term
	if app.IsPresent(args.Q) {
		args.Q = ""
	}
	args.PDC = req.Context.Creds.Dc
	args.Page = req.GetPage()
	args.Size = req.GetSize()
	args.Fields = app.FieldsFunc(
		req.Fields,
		app.SelectFields(
			// default
			[]string{
				"id",
				"type",
				"name",
			},
			// extra
			[]string{
				"via",
			},
		),
	)

	var (
		inputId = func(input any) error {
			switch data := input.(type) {
			case []string:
				if len(data) > 0 {
					args.ID = data
				}

			case string:
				if data != "" {
					args.ID = []string{data}
				}

			default:
				return errors.BadRequest(
					"customers.query.id.input",
					"customers( id: [string!] ); input: convert %T",
					input,
				)
			}

			return nil
		}

		inputVia = func(input any) error {
			switch data := input.(type) {
			case int64:
				if data > 0 {
					return errors.BadRequest(
						"customers.query.via.id.input",
						"customers( via.id: int! ); input: negative id",
					)
				}

				args.Via = &pb.Peer{
					Id: strconv.FormatInt(data, 10),
				}

			case *pb.Peer:
				args.Via = data

			default:
				return errors.BadRequest(
					"customers.query.via.input",
					"customers( via: peer ); input: convert %T",
					input,
				)
			}

			// Validate
			if via := args.Via; via != nil {
				vs := via.GetId() != ""
				vs = vs || via.GetType() != ""
				vs = vs || via.GetName() != ""

				if !vs { // NO value ! skip
					args.Via = nil
				}
			}

			return nil
		}
	)

	for param, input := range req.Filter {
		switch param {
		case "id":
			err = inputId(input)

		case "via":
			err = inputVia(input)

		case "type":
			switch data := input.(type) {
			case string:
				if data != "" {
					args.Type = data
				}

			default:
				err = errors.BadRequest(
					"customers.query.type.input",
					"customers( type: string ); input: convert %T",
					input,
				)
			}

		case "self":
			args.Self = req.Creds.UserId

		default:
			err = errors.BadRequest(
				"customers.query.args.error",
				"customers( %s: ? ); input: no such argument",
				param,
			)
			return
		}
	}

	return
}

type chatContactsQuery struct {
	SELECT

	input chatCustomersArgs
	fetch func(*sql.Rows, *pb.ChatCustomers) error
}

func getContactsQuery(req *app.SearchOptions) (ctx chatContactsQuery, err error) {

	ctx.input, err = getCustomersInput(req)
	if err != nil {
		return
	}

	ctx.Params = params{
		"pdc": ctx.input.PDC,
	}

	// NOTE: Create a LEFT JOIN with the 'chat.bot' table using alias "a"
	// and the condition where the 'id' of the chat.bot table matches
	// the 'connection::::int8' field, and the 'dc' equals the specified :pdc parameter.
	left := "c"
	gateAlias := "a"
	gate := &JOIN{
		Kind:  "LEFT JOIN",
		Table: sq.Expr("chat.bot"),
		Alias: gateAlias,
		Pred: sq.And{
			sq.Expr(ident(gateAlias, "id") + " = " + ident(left, "connection::::int8")),
			sq.Expr(ident(gateAlias, "dc") + " = :pdc"),
		},
	}

	// NOTE: Build the initial query to select 'type' and 'user_id' from
	// 'chat.channel' and join the 'chat.bot' table using the 'gate' alias.
	// Also, create an aggregated list 'chat_via' of distinct 'id' from 'chat.bot'.
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "type"),
			ident(left, "user_id"),
			fmt.Sprintf(
				"array_agg(DISTINCT %s) chat_via",
				ident(gate.Alias, "id"),
			),
		).
		From(
			"chat.channel "+left,
		).
		JoinClause(
			gate,
		).
		Where(
			ident(left, "domain_id")+" = :pdc",
		).
		Where(
			"NOT "+ident(left, "internal"),
		).
		GroupBy(
			ident(left, "user_id"),
			ident(left, "type"),
		)

	// NOTE: Handle filtering by 'type' input parameter if provided.
	if typeOf := ctx.input.Type; typeOf != "" {
		ctx.Params.set("peer.type", app.Substring(typeOf))
		ctx.Query = ctx.Query.Where(
			ident(left, "type") + " ILIKE :peer.type",
		)
	}

	// NOTE: Handle filtering by 'via' object if provided, including by 'id', 'type', or 'name'.
	if via := ctx.input.Via; via != nil {
		if via.Id != "" {
			oid, re := strconv.ParseInt(via.Id, 10, 64)
			if re != nil || oid < 1 {
				err = errors.BadRequest(
					"customers.query.via.id.input",
					"customers( via.id: int ); input: invalid id",
				)
				return
			}

			ctx.Params.set("via.id", via.Id)
			ctx.Query = ctx.Query.Where(
				ident(left, "connection") + " = :via.id",
			)
		}

		if via.Type != "" {
			ctx.Params.set("via.type", app.Substring(via.Type))
			ctx.Query = ctx.Query.Where(
				ident(gate.Alias, "provider") + " ILIKE :via.type",
			)
		}

		if via.Name != "" {
			ctx.Params.set("via.name", app.Substring(via.Name))
			gate.Pred = append(gate.Pred.(sq.And),
				sq.Expr(ident(gate.Alias, "name")+" ILIKE :via.name COLLATE \"default\""),
			)
		}
	}

	peerQ := ctx.Query

	// NOTE: Build the second query to select 'external_id' as 'id' and 'type'
	// from 'chat.client' and join it with the previously built query ('peerQ')
	// based on the 'user_id' match.
	ctx.Query = postgres.PGSQL.
		Select(
			"x.external_id AS id",
			"c.type",
		).
		From(
			"chat.client x",
		).
		JoinClause(&JOIN{
			Kind:  "JOIN",
			Table: peerQ.Prefix("(").Suffix(")"),
			Alias: "c",
			Pred: sq.Expr(
				"c.user_id = x.id",
			),
		})

	// NOTE: Handle filtering by 'q' (search string) if provided.
	if q := ctx.input.Q; q != "" && !app.IsPresent(q) {
		ctx.Params.set("q", app.Substring(q))
		ctx.Query = ctx.Query.Where(
			ident("x", "name") + " ILIKE :q COLLATE \"default\"",
		)
	}

	// NOTE: Handle filtering by 'id' array if provided.
	if n := len(ctx.input.ID); n > 0 {
		var id pgtype.TextArray
		err = id.Set(ctx.input.ID)
		if err != nil {
			return
		}

		eq, vs := " = ANY(:id)", any(&id)
		if n == 1 {
			eq, vs = " = :id", &id.Elements[0]
		}

		ctx.Params.set("id", vs)
		ctx.Query = ctx.Query.Where(
			ident("x", "external_id") + eq,
		)
	}

	const (
		ASC  = " ASC"
		DESC = " DESC"
	)

	var (
		order = ASC              // NOTE: Set default order is ASC
		sort  = []string{"name"} // NOTE: Set default sort by name
	)

	// NOTE: Check if the 'Order' field is present in the request and process the sorting specifications.
	if len(req.Order) > 0 {
		sort = app.FieldsFunc(req.Order, app.InlineFields)
	}

	// NOTE: Iterate through the sort specifications and process each one for ordering the query.
	for _, spec := range sort {
		switch spec[0] {
		// NOT URL-encoded PLUS '+' char
		// we will get as SPACE ' ' char
		case '+', ' ': // be loyal; URL(+) == ' '
			spec = spec[1:]

		case '-', '!':
			spec = spec[1:]
			order = DESC
		}

		// NOTE: Translate sorting field names to database column identifiers.
		switch spec {
		case "id":
			spec = ident("x", "external_id")

		case "type":
			spec = ident("c", "type")

		case "name":
			spec = ident("x", "name")

		// case "via":
		// 	{
		// 		// TODO !!!!
		// 		spec = ident(left, spec)
		// 		// SELECT * FROM unnest('{f,t,NULL}'::bool[]) ORDER BY 1 ASC;
		// 		// ASC  [false, true, NULL]
		// 		// DESC [NULL, true, false]
		// 		switch order {
		// 		case ASC:
		// 			order = " DESC NULLS LAST"
		// 		default: // DESC
		// 			order = " ASC NULLS FIRST"
		// 		}
		// 	}

		default:
			// NOTE: If an unsupported sorting field is provided, return an error.
			err = errors.BadRequest(
				"customers.query.sort.input",
				"customers( sort: [%s] ); input: no field support",
				spec,
			)
			return
		}

		// NOTE: Apply the sorting to the query using the processed field and order.
		ctx.Query = ctx.Query.OrderBy(spec + order)
	}

	// NOTE: [OFFSET|LIMIT]: paging
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			ctx.Query = ctx.Query.Offset(uint64((page - 1) * size))
		}

		// LIMIT (size+1) -- to indicate whether there are more result entries
		ctx.Query = ctx.Query.Limit(uint64(size + 1))
	}

	// NOTE: Process the selected fields and add them to the query.
	var withVia bool
	slices.Sort(ctx.input.Fields)
	fields := slices.Compact(ctx.input.Fields)
	for _, field := range fields {
		switch field {
		case "id":
			// NOTE: Nothing

		case "type":
			// NOTE: Nothing

		case "name":
			ctx.Query = ctx.Query.Column(ident("x", "name"))

		case "via":
			ctx.Query = ctx.Query.Column(ident("c", "chat_via"))
			withVia = true

		default:
			err = errors.BadRequest(
				"customers.query.fields.input",
				"customers{ %s }; input: no such field",
				field,
			)
			return
		}
	}

	// NOTE: Create a Common Table Expression (CTE) for the peer query.
	ctx.With(CTE{
		Name: "peer",
		Expr: ctx.Query,
	})

	// NOTE: If 'via' is not included in the query, proceed with scanning the contact rows.
	if !withVia {
		ctx.fetch = ctx.scanContactsRows
		return
	}

	// NOTE: Create a CTE for the 'via_chat' query if 'via' field is included.
	ctx.With(CTE{
		Name: "via_chat",
		Expr: postgres.PGSQL.
			Select(
				"b.id",
				"b.provider AS \"type\"",
				"b.name",
				"c.peer_ids",
			).
			From(
				"chat.bot b",
			).
			JoinClause(CompactSQL(
				`JOIN (
					SELECT
						UNNEST(p.chat_via) AS id,
						array_agg(DISTINCT p.id) AS peer_ids
					FROM
						peer p
					GROUP BY
						UNNEST(p.chat_via)
				)
				AS c
				USING (id)`,
			)),
	})

	// NOTE: Define a Common Table Expression (CTE) for the 'via_portal' query.
	// This CTE selects data related to portal user accounts and services.
	ctx.With(CTE{
		Name: "via_portal",
		Expr: postgres.PGSQL.
			Select(
				"sa.id",
				"sa.app AS name",
				"'portal' AS \"type\"",
				"array_agg(DISTINCT p.id) AS peer_ids",
			).
			From(
				"peer p",
			).
			JoinClause(CompactSQL(
				`JOIN
					portal.user_account ua
				ON
					p.id::::uuid = ua.id AND
					p.type = 'portal'
				JOIN
					portal.user_service us
				ON
					us.account_id = ua.id
				JOIN
					portal.service_app sa
				ON
					sa.service_id = us.service_id
				GROUP BY
					sa.id,
					sa.app`,
			)),
	})

	// NOTE: Create a union of two CTEs 'via_chat' and 'via_portal' using UNION ALL.
	// This combines data from both CTEs into a unified result.
	viaExpr, err := WithUnionAll(
		sq.Select(
			"vc.id::::text AS id",
			"vc.name",
			"vc.type",
			"vc.peer_ids",
		).From("via_chat vc"),
		sq.Select(
			"vp.id::::text AS id",
			"vp.name",
			"vp.type",
			"vp.peer_ids",
		).From("via_portal vp"),
	)
	if err != nil {
		return
	}

	// NOTE: Define a CTE for the unified 'via' data, combining results from 'via_chat' and 'via_portal'.
	ctx.With(CTE{
		Name: "via",
		Expr: viaExpr,
	})

	// NOTE: Define a CTE for 'peer_with_via' which retrieves peer information along with their associated 'via' data.
	// The 'via' data is aggregated using COALESCE to ensure that even if no 'via' is associated, an empty array is returned.
	ctx.With(CTE{
		Name: "peer_with_via",
		Expr: sq.Select(
			"p.id",
			"p.type",
			"p.name",
			`COALESCE(
				(SELECT array_agg(DISTINCT v.id) FROM via v WHERE p.id = ANY(v.peer_ids)),
				ARRAY[]::::text[]
			) AS via`,
		).From("peer p"),
	})

	// NOTE: Final query to select the 'via' and 'peer' information, formatting them as JSON objects.
	// The 'via' data is selected from the 'via' CTE, and the 'peer' data is selected from the 'peer_with_via' CTE.
	ctx.Query = postgres.PGSQL.
		Select(
			`ARRAY(
				SELECT
					json_build_object(
						'id', v.id,
						'type', v.type,
						'name', v.name
					)
				FROM
					via v
			) via`,
			`ARRAY(
				SELECT
					json_build_object(
						'id', p.id,
						'type', p.type,
						'name', p.name,
						'via', p.via
					)
				FROM
					peer_with_via p
			) peer`,
		)

	// NOTE: Set rows scanner
	ctx.fetch = ctx.scanContactsRowsWithVia

	return
}

type sqlVia struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (v *sqlVia) Scan(value any) error {
	switch data := value.(type) {
	case []byte:
		err := json.Unmarshal(data, v)
		if err != nil {
			return fmt.Errorf("error: parsing JSON: %v", err)
		}

	default:
		return fmt.Errorf("error: invalid value type")
	}

	return nil
}

type sqlPeer struct {
	ID   string   `json:"id"`
	Type string   `json:"type"`
	Name string   `json:"name"`
	Via  []string `json:"via"`
}

func (p *sqlPeer) Scan(value any) error {
	switch data := value.(type) {
	case []byte:
		err := json.Unmarshal(data, p)
		if err != nil {
			return fmt.Errorf("error: parsing JSON: %v", err)
		}

	default:
		return fmt.Errorf("error: invalid value type")
	}

	return nil
}

func (ctx *chatContactsQuery) scanContactsRows(rows *sql.Rows, into *pb.ChatCustomers) error {

	// NOTE: scan all rows and set peers to response
	for rows.Next() {
		var peer sqlPeer

		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return err
		}

		references := make([]any, 0, len(columnTypes))
		for _, columnType := range columnTypes {
			switch columnType.Name() {
			case "id":
				references = append(references, &peer.ID)

			case "type":
				references = append(references, &peer.Type)

			case "name":
				references = append(references, &peer.Name)
			}
		}

		err = rows.Scan(references...)
		if err != nil {
			return fmt.Errorf("error: reading the result: %v", err)
		}

		into.Peers = append(into.Peers, &pb.Customer{
			Id:   peer.ID,
			Name: peer.Name,
			Type: peer.Type,
		})
	}

	// NOTE: Set next and page to response
	limit := ctx.input.Size
	size := len(into.Peers)
	into.Page = int32(ctx.input.Page)
	if limit > 0 && size > limit {
		into.Next = true
	}

	return nil
}

func (ctx *chatContactsQuery) scanContactsRowsWithVia(rows *sql.Rows, into *pb.ChatCustomers) error {
	var vias []sqlVia
	var peers []sqlPeer

	// NOTE: scan only first row
	if rows.Next() {
		err := rows.Scan(pq.Array(&vias), pq.Array(&peers))
		if err != nil {
			return fmt.Errorf("error: reading the result: %v", err)
		}
	}

	// NOTE: Set vias to response
	for _, v := range vias {
		into.Vias = append(into.Vias, &pb.Peer{
			Id:   v.ID,
			Name: v.Name,
			Type: v.Type,
		})
	}

	// NOTE: Set peers to response
	for _, p := range peers {
		customer := &pb.Customer{
			Id:   p.ID,
			Name: p.Name,
			Type: p.Type,
			Via:  make([]*pb.Peer, 0, len(p.Via)),
		}

		for _, viaID := range p.Via {
			index := slices.IndexFunc(vias, func(v sqlVia) bool {
				return v.ID == viaID
			})
			if index != -1 {
				customer.Via = append(customer.Via, &pb.Peer{
					Id: strconv.Itoa(index + 1),
				})
			}
		}

		into.Peers = append(into.Peers, customer)
	}

	// NOTE: Set next and page to response
	limit := ctx.input.Size
	size := len(into.Peers)
	into.Page = int32(ctx.input.Page)
	if limit > 0 && size > limit {
		into.Next = true
	}

	return nil
}
