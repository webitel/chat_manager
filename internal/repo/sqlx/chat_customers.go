package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
	"github.com/micro/micro/v3/service/errors"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/store/postgres"
)

type chatCustomersArgs struct {
	// indicates whether 'portal' users are supported
	ViaPortal bool
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

func (e *chatCustomersArgs) indexField(name string) int {
	var i, n = 0, len(e.Fields)
	for ; i < n && e.Fields[i] != name; i++ {
		// match: by <name>
	}
	if i < n {
		return i
	}
	return -1
}

func getCustomersInput(req *app.SearchOptions) (args chatCustomersArgs, err error) {

	args.ViaPortal = portal.ok

	args.Q = req.Term
	if app.IsPresent(args.Q) {
		args.Q = "" // clear; ignore
	}
	args.PDC = req.Context.Creds.Dc
	args.Page = req.GetPage()
	args.Size = req.GetSize()
	args.Fields = app.FieldsFunc(
		req.Fields, // app.InlineFields,
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
				args.ID = data
			case string:
				if data == "" {
					break // omitted
				}
				args.ID = []string{
					data,
				}
			default:
				return errors.BadRequest(
					"customers.query.id.input",
					"customers( id: [string!] ); input: convert %T",
					input,
				)
			}
			// for _, id := range args.ID {

			// }
			return nil // OK
		}
		inputVia = func(input any) error {
			switch data := input.(type) {
			case int64:
				if 1 < data {
					return errors.BadRequest(
						"customers.query.via.id.input",
						"customers( via.id: int! ); input: negative id",
					)
				}
				// if args.Via != nil {
				// 	// ERR: ambiguous
				// }
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
			return nil // OK
		}
	)

	for param, input := range req.Filter {
		switch param {
		case "id":
			{
				err = inputId(input)
			}
		case "via":
			{
				err = inputVia(input)
			}
		case "type":
			{
				switch data := input.(type) {
				case string:
					if data == "" {
						break // omitted
					}
					args.Type = data
				default:
					err = errors.BadRequest(
						"customers.query.type.input",
						"customers( type: string ); input: convert %T",
						input,
					)
				}
			}
		case "self":
			args.Self = req.Creds.UserId
		default:
			err = errors.BadRequest(
				"customers.query.args.error",
				"customers( %s: ? ); input: no such argument",
				param,
			)
			return // args, err
		}
	}

	return // args, nil
}

// [TODO]: refactor !
// GLOBAL[ONCE]: schema 'portal' support ?
var portal = struct {
	once sync.Once // guards [withSchemaPortal]
	ok   bool      // WITH 'portal' schema support
}{}

// CHECK postgres has 'portal' service schema installed
func withSchemaPortal(ctx context.Context, dc *sqlx.DB) func() {
	return func() {
		var ok pgtype.Bool
		err := dc.QueryRowContext(ctx,
			"SELECT true FROM pg_catalog.pg_tables e "+
				"WHERE e.schemaname = 'portal' AND e.tablename = 'service_app'",
		).Scan(&ok)
		// resolved as:
		portal.ok = (err == nil && ok.Bool)
	}
}

type chatContactsQuery struct {
	input chatCustomersArgs
	SELECT
	peers dataFetch[*pb.Customer]
	// vias  dataFetch[*pb.Peer]
	fetch func(*sql.Rows, *pb.ChatCustomers) error
}

func (c *chatContactsQuery) ToSql() (query string, args []any, err error) {
	query, args, err = c.SELECT.ToSql()
	// used to passthru '?' mark as postgres operator
	// and prevent Sqlizer.ToSql() multi [un]escaping
	query = strings.ReplaceAll(query, "_QM_", "?")
	return
}

func getContactsQuery(req *app.SearchOptions) (ctx chatContactsQuery, err error) {

	ctx.input, err = getCustomersInput(req)
	if err != nil {
		return // ctx, err
	}

	ctx.Params = params{
		"pdc": ctx.input.PDC,
	}

	// region: ----- STEP 1: resolve: type, via -----
	const (
		left = "c" // [FROM] alias
	)

	var (
		cteVia     string                // optional: [via] gateways selection
		viaPortal  = ctx.input.ViaPortal // support: schema [portal] installed ?
		viaGateway = true                // support: default !
	)
	// peers( via: peer )
	if via := ctx.input.Via; via != nil {
		// PREPARE Filter [VIA] Query
		var (
			ok                 bool             // ANY of via.* filter(s) applied ?
			gateQ, portQ, viaQ sq.SelectBuilder // query part(s) FROM
		)

		// [viaGateway:true]
		gateQ = psql.Select().
			Columns(
				"e.id::::text", // int8
				"e.provider \"type\"",
				"e.name",
			).
			From("chat.bot e").
			Where("e.dc = :pdc")

		viaQ = psql.Select()

		if viaPortal {
			portQ = psql.Select().
				Columns(
					"e.id::::text",      // uuid
					"'portal' \"type\"", // "type"
					"e.app \"name\"",    // "name"
				).
				From("portal.service_app e").
				Where("e.dc = :pdc")
		}
		switch via.Id {
		case "", "*":
			// ANY ; disable !
			via.Id = ""
		default:
			{
				// Valid INT ?
				if oid, _ := strconv.ParseInt(via.Id, 10, 64); oid > 0 {
					ok = true                     // applied !
					viaPortal = false             // exclude !
					ctx.Params.set("via.id", oid) // int8
					gateQ = gateQ.Where(
						"e.id = :via.id",
					)
				} else if ctx.input.ViaPortal {
					// Valid UUID ?
					if pid, not := uuid.Parse(via.Id); not == nil {
						ok = true                     // applied !
						viaGateway = false            // exclude !
						ctx.Params.set("via.id", pid) // uuid
						portQ = portQ.Where(
							":via.id = ANY(ARRAY[e.id,e.service_id])",
						)
					}
				}
				// applied ?
				if !ok {
					return ctx, errors.BadRequest(
						"customers.query.via.id.input",
						"customers( via.id: string ); input: invalid id",
					)
				}
			}
		}
		switch via.Type {
		case "", "*":
			// ANY ; disable !
			via.Type = ""
		default:
			{
				// optimize simple case
				if strings.ToLower(via.Type) == "portal" {
					if viaPortal { // oneof !
						viaGateway = false // disable !
					}
				}
				eq, vs := "=", any(strings.ToLower(via.Type))
				if strings.ContainsAny(via.Type, "*?") {
					eq, vs = "ILIKE", app.Substring(via.Type)
				}
				ctx.Params.set("via.type", vs)
				if !viaPortal {
					// FROM [chat.bot] ONLY !
					gateQ = gateQ.Where(fmt.Sprintf(
						"e.provider %s :via.type COLLATE \"C\"", eq,
					))
				} else if !viaGateway {
					// FROM [portal.service_app] ONLY !
					portQ = portQ.Where(fmt.Sprintf(
						"'portal' %s :via.type COLLATE \"C\"", eq,
					))
				} else {
					// .. UNION ALL ..
					viaQ = viaQ.Where(fmt.Sprintf(
						"e.type %s :via.type COLLATE \"C\"", eq,
					))
				}
				ok = true // applied !
			}
		}
		switch via.Name {
		case "", "*":
			// ANY ; disable !
			via.Name = ""
		default:
			{
				eq, vs := "ILIKE", any(via.Name) // case:ignore MATCH
				if strings.ContainsAny(via.Name, "*?") {
					vs = app.Substring(via.Name)
				}
				ctx.Params.set("via.name", vs)
				if !viaPortal {
					// FROM [chat.bot] ONLY !
					gateQ = gateQ.Where(fmt.Sprintf(
						"e.name %s :via.name COLLATE \"default\"", eq,
					))
				} else if !viaGateway {
					// FROM [portal.service_app] ONLY !
					portQ = portQ.Where(fmt.Sprintf(
						"e.app %s :via.name COLLATE \"default\"", eq,
					))
				} else {
					// .. UNION ALL ..
					viaQ = viaQ.Where(fmt.Sprintf(
						"e.name %s :via.name COLLATE \"default\"", eq,
					))
				}
				ok = true // applied !
			}
		}

		// applied ?
		if ok {
			// BUILD
			if !viaPortal {
				// FROM [chat.bot] ONLY !
				viaQ = gateQ
			} else if !viaGateway {
				// FROM [portal.service_app] ONLY !
				viaQ = portQ
			} else {
				// SELECT id, type, name .. UNION ALL ..
				// preserve [viaQ] prepared filter(s) applied !
				viaQ = viaQ.Columns("*").FromSelect(
					gateQ.SuffixExpr(portQ.Prefix("UNION ALL")),
					"e", // alias
				)
			}
			if via.Id != "" {
				// [finally]: select [:via.id] as [deleted] gateway
				// to be still able to query historical records
				viaQ = psql.Select(
					"DISTINCT ON (id) *", // id, "type", "name"
				).FromSelect(
					// UNION order matters !
					// first [found] than [deleted] !
					viaQ.Suffix("UNION ALL "+
						"SELECT (:via.id)::::text, NULL, '[deleted]'",
					), "e",
				)
			}

			// filter [via] gateway(s) requested, FIRST !
			cteVia = "q" // selected !
			ctx.With(
				CTE{Name: cteVia, Expr: viaQ},
			)

			// normalized by filter(s) set ..
			ctx.input.ViaPortal = viaPortal
		}
	}

	// PROJECT: [chat.channel] gateway.id, a.k.a "via", relation ..
	// [NOTE]: "_QM_" stands for [Q]uestion[M]ark, as an alias !
	// '?' is used by [squirrel] as a default SQL parameter placeholder
	// so its hard to pass thru as a part of SQL (operator, in our case)
	//
	// here we inject "_QM_" as a placeholder, which will be replaced on [chatContactsQuery.ToSql] method
	var chatViaQ string // [chat.channel] column text
	if !viaPortal {
		// FROM [chat.bot] ONLY !
		chatViaQ = "SELECT %[1]s.connection WHERE %[1]s.connection::::int8 > 0"
	} else if !viaGateway {
		// FROM [portal.service_app] ONLY !
		// ::uuid::text ; normalize UUID for GROUP BY clause
		chatViaQ = "SELECT (%[1]s.props->>'portal.client.id')::::uuid::::text" +
			" WHERE %[1]s.type = 'portal' AND %[1]s.props _QM_ 'portal.client.id'"
	} else {
		// .. UNION ALL ..
		chatViaQ = `SELECT
		(
			CASE
			WHEN %[1]s.type = 'portal' AND %[1]s.props _QM_ 'portal.client.id'
			THEN (%[1]s.props->>'portal.client.id')::::uuid::::text
			WHEN %[1]s.connection::::int8 > 0
			THEN %[1]s.connection -- chat.bot::text
			END
		)`
	}

	// endregion: ----- STEP 1 -----

	// region: ----- STEP 2: Build channel EXISTS subquery -----

	representativeChannelQ := postgres.PGSQL.
		Select(
			ident(left, "type"),
			"via.id AS via_id",
		).
		From("chat.channel " + left).
		Where(ident(left, "user_id") + " = x.id").
		Where(ident(left, "domain_id") + " = :pdc").
		Where("NOT " + ident(left, "internal")).
		JoinClause(CompactSQL(fmt.Sprintf(
			"LEFT JOIN LATERAL ("+chatViaQ+") via(id) ON true", left,
		))).
		OrderBy(ident(left, "type") + " ASC").
		OrderBy(ident(left, "id") + " ASC").
		Limit(1)

	// Apply type filter
	if ctx.input.Type != "" {
		eq, vs := "=", any(ctx.input.Type)
		if strings.ContainsAny(ctx.input.Type, "*?") {
			eq, vs = "ILIKE", app.Substring(ctx.input.Type)
		}
		ctx.Params.set("peer.type", vs)
		representativeChannelQ = representativeChannelQ.Where(fmt.Sprintf(
			"%s %s :peer.type COLLATE \"C\"",
			ident(left, "type"), eq,
		))
	}

	// Apply via filter
	if cteVia != "" {
		representativeChannelQ = representativeChannelQ.JoinClause("JOIN q ON via.id = q.id")
	}

	// endregion: ----- STEP 2 -----

	// region: ----- STEP 3: Build main user query -----

	sqlStr, _, err := representativeChannelQ.ToSql()
	if err != nil {
		return ctx, err
	}

	mainQuery := postgres.PGSQL.
		Select(
			ident("x", "id")+" user_id",
			ident("x", "external_id"),
			ident("x", "name"),
			"rep.type",
		).
		From("chat.client x").
		JoinClause(
			"CROSS JOIN LATERAL (" + sqlStr + ") AS rep",
		)

	if q := ctx.input.Q; q != "" && !app.IsPresent(q) {
		eq, vs := "=", any(q)
		if strings.ContainsAny(q, "*?") {
			eq, vs = "ILIKE", app.Substring(q)
		}
		ctx.Params.set("q", vs)
		mainQuery = mainQuery.Where(fmt.Sprintf(
			"%s %s :q COLLATE \"default\"",
			ident("x", "name"), eq,
		))
	}
	// peers( id: [string!] )
	if n := len(ctx.input.ID); n > 0 {
		var id pgtype.TextArray
		err = id.Set(ctx.input.ID)
		if err != nil {
			return // nil, err
		}
		eq, vs := " = ANY(:id)", any(&id)
		if n == 1 {
			eq, vs = " = :id", &id.Elements[0]
		}
		ctx.Params.set("id", vs)
		mainQuery = mainQuery.Where(
			ident("x", "external_id") + eq,
		)
	}

	// Apply sorting (only user-level fields for now)
	const (
		ASC  = " ASC"
		DESC = " DESC"
	)
	var (
		order string
		sort  = app.FieldsFunc(
			req.Order, app.InlineFields,
		)
	)
	if len(sort) == 0 {
		sort = []string{
			"name", // ALPHA
		}
	}
	for _, spec := range sort {
		order = ASC // default
		switch spec[0] {
		// NOT URL-encoded PLUS '+' char
		// we will get as SPACE ' ' char
		case '+', ' ': // be loyal; URL(+) == ' '
			spec = spec[1:]
		case '-', '!':
			spec = spec[1:]
			order = DESC
		}
		switch spec {
		// complex
		case "id":
			mainQuery = mainQuery.OrderBy(ident("x", "external_id") + order)
		case "type":
			mainQuery = mainQuery.OrderBy("rep.type" + order)
		case "name":
			mainQuery = mainQuery.OrderBy(ident("x", "name") + order)
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
			{
				err = errors.BadRequest(
					"customers.query.sort.input",
					"customers( sort: [%s] ); input: no field support",
					spec,
				)
				return // nil, err
			}
		}
	}

	// [OFFSET|LIMIT]: paging
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			mainQuery = mainQuery.Offset(
				(uint64)((page - 1) * size),
			)
		}
		mainQuery = mainQuery.Limit(
			(uint64)(size + 1),
		)
	}

	// Store as CTE
	ctx.With(CTE{
		Name: "target_users",
		Expr: mainQuery,
	})

	// endregion: ----- STEP 3 -----

	// region: ----- STEP 4: Aggregate channels ONLY for target users -----

	channelAggQuery := postgres.PGSQL.
		Select(
			ident(left, "type"),
			ident(left, "user_id"),
			"array_agg(DISTINCT via.id ORDER BY via.id) FILTER(WHERE via.id IS NOT NULL) via",
		).
		From("chat.channel "+left).
		JoinClause("JOIN target_users tu ON tu.user_id = "+ident(left, "user_id")).
		Where(ident(left, "domain_id")+" = :pdc").
		Where("NOT "+ident(left, "internal")).
		JoinClause(CompactSQL(fmt.Sprintf(
			"LEFT JOIN LATERAL ("+chatViaQ+") via(id) ON true", left,
		))).
		GroupBy(
			ident(left, "user_id"),
			ident(left, "type"),
		)

	// Apply same filters to aggregation for consistency
	if ctx.input.Type != "" {
		eq := "="
		if strings.ContainsAny(ctx.input.Type, "*?") {
			eq = "ILIKE"
		}
		channelAggQuery = channelAggQuery.Where(fmt.Sprintf(
			"%s %s :peer.type COLLATE \"C\"",
			ident(left, "type"), eq,
		))
	}

	if cteVia != "" {
		channelAggQuery = channelAggQuery.JoinClause("JOIN q ON via.id = q.id")
	}

	// endregion: ----- STEP 4 -----

	// region: ----- STEP 5: Final assembly -----

	ctx.Query = postgres.PGSQL.
		Select(
			ident("x", "external_id")+" id",
			ident("c", "type"),
		).
		From("chat.client x").
		JoinClause("JOIN target_users tu ON tu.user_id = x.id").
		JoinClause(&JOIN{
			Kind:  "JOIN",
			Table: channelAggQuery.Prefix("(").Suffix(")"),
			Alias: "c",
			Pred:  sq.Expr("c.user_id = x.id"),
		})

	// Re-apply sorting to final query
    for _, spec := range sort {
		order = ASC
		switch spec[0] {
		case '+', ' ':
			spec = spec[1:]
		case '-', '!':
			spec = spec[1:]
			order = DESC
		}
		switch spec {
		case "id":
			ctx.Query = ctx.Query.OrderBy(ident("x", "external_id") + order)
		case "type":
			ctx.Query = ctx.Query.OrderBy(ident("c", "type") + order)
		case "name":
			ctx.Query = ctx.Query.OrderBy(ident("x", "name") + order)
		}
	} 
	ctx.Query = ctx.Query.OrderBy(ident("x", "id") + ASC)


	// endregion: ----- STEP 5 -----

	// region: ----- select: fields -----
	ctx.peers = dataFetch[*pb.Customer]{
		// id
		func(node *pb.Customer) any {
			return postgres.Text{Value: &node.Id}
		},
		// type
		func(node *pb.Customer) any {
			return postgres.Text{Value: &node.Type}
		},
	}

	var (
		withVia = false
		columns = make([]string, 0, 2) // except: id, type
		column  = func(name string) bool {
			var e, n = 0, len(columns)
			for ; e < n && columns[e] != name; e++ {
				// lookup: already queried ?
			}
			if e < n {
				// FOUND !
				return false
			}
			// NOT FOUND !
			columns = append(columns, name)
			return true
		}
	)
	for _, field := range ctx.input.Fields {
		switch field {
		// core:identity
		case "id":
		case "type":
		// user:fields
		case "name":
			if !column("name") {
				break // duplicate
			}
			ctx.Query = ctx.Query.Column(
				ident("x", "name"), // text
			)
			ctx.peers = append(ctx.peers,
				func(node *pb.Customer) any {
					return postgres.Text{Value: &node.Name}
				},
			)
		case "via":
			if !column("via") {
				break // duplicate
			}
			ctx.Query = ctx.Query.Column(
				ident("c", "via"), // text[] -- int8[] | uuid[]
			)
			ctx.peers = append(ctx.peers,
				func(node *pb.Customer) any {
					return postgres.Text{Value: &node.Name}
				},
			)
			// TODO: prepare VIAs query !!!
			withVia = true
		default:
			err = errors.BadRequest(
				"customers.query.fields.input",
				"customers{ %s }; input: no such field",
				field,
			)
		}
	}
	// endregion: ----- select: fields -----

	if !withVia {
		ctx.fetch = ctx.scanPageRows
		return // ctx, nil
	}
	// Complex selection
	const (
		viewVia  = "via"  // gateways
		viewPeer = "peer" // contacts
	)
	ctx.With(CTE{
		Name: viewPeer,
		Expr: ctx.Query,
	})
	// [VIEW]: via
	viaQ := postgres.PGSQL.
		Select(
			"e.id",
			// "type", "name" projection below ..
		).
		From(CompactSQL(
			`(
					SELECT UNNEST(via) id --, "type" -- [contact] type
						FROM peer
					GROUP BY 1 --, 2 -- [NOTE]: gateway[ messenger ]; contacts[ facebook, instagram, whatsapp ]
				) e`,
		))

	if cteVia != "" {
		// JOIN WITH filter(s) select[ed]
		viaQ = viaQ.JoinClause(CompactSQL(fmt.Sprintf(
			`LEFT JOIN %s q ON q.id = e.id`,
			cteVia,
		))).Columns(
			"q.type",
			"coalesce(q.name, '[deleted]') \"name\"",
		)
	} else if !viaPortal {
		// FROM [chat.bot] ONLY !
		viaQ = viaQ.JoinClause(CompactSQL(
			`LEFT JOIN chat.bot b ON b.dc = :pdc AND b.id::::text = e.id`,
		)).Columns(
			"b.provider \"type\"",
			"coalesce(b.name, '[deleted]') \"name\"",
		)
	} else if !viaGateway {
		// FROM [portal.service_app] ONLY !
		viaQ = viaQ.JoinClause(CompactSQL(
			`LEFT JOIN portal.service_app a ON a.dc = :pdc AND a.id::::text = e.id`,
		)).Columns(
			"(select 'portal' where a.id notnull) \"type\"",
			"coalesce(a.app, '[deleted]') \"name\"",
		)
	} else {
		// .. UNION ALL ..
		viaQ = viaQ.JoinClause(CompactSQL(
			`LEFT JOIN chat.bot b ON b.dc = :pdc AND b.id::::text = e.id`,
		)).JoinClause(CompactSQL(
			`LEFT JOIN portal.service_app a ON a.dc = :pdc AND a.id::::text = e.id`,
		)).Columns(
			"(case when a.id notnull then 'portal' else b.provider end) \"type\"",
			"coalesce(b.name, a.app, '[deleted]') \"name\"",
		)
	}

	ctx.With(CTE{
		Name: viewVia,
		Expr: viaQ,
	})

	ctx.Query = postgres.PGSQL.
		Select(
			"ARRAY(SELECT e FROM via e) via",
			"ARRAY(SELECT e FROM peer e) peer",
		)

	ctx.fetch = ctx.scanPageData
	return // ctx, nil
}

func (ctx *chatContactsQuery) scanPageRows(rows *sql.Rows, into *pb.ChatCustomers) (err error) {

	var (
		node *pb.Customer
		heap []pb.Customer

		page = into.GetPeers() // input
		data []*pb.Customer    // output

		plan = ctx.peers
		eval = make([]any, len(plan))
		size = ctx.input.Size
	)

	if 0 < size {
		data = make([]*pb.Customer, 0, size)
	}

	if n := size - len(page); 1 < n {
		heap = make([]pb.Customer, n) // mempage; tidy
	}

	var (
		r, c int // row, column
	)
	into.Page = int32(ctx.input.Page) // request[ed]
	for rows.Next() {
		// LIMIT
		if 0 < size && len(data) == size {
			into.Next = true
			if into.Page < 1 {
				into.Page = 1 // default
			}
			break
		}
		// RECORD
		node = nil // NEW
		if r < len(page) {
			// [INTO] given page records
			// [NOTE] order matters !
			node = page[r]
		} else if len(heap) > 0 {
			node = &heap[0]
			heap = heap[1:]
		}
		// ALLOC
		if node == nil {
			node = new(pb.Customer)
		}
		// [BIND] data fields to scan row
		c = 0
		for _, bind := range plan {
			df := bind(node)
			if df != nil {
				eval[c] = df
				c++
				continue
			}
		}
		// FETCH; decode
		err = rows.Scan(eval...)
		if err != nil {
			return err
		}
		// output: list
		data = append(data, node)
		r++
	}

	into.Peers = data
	if !into.Next && into.Page < 2 {
		into.Page = 0 // hide; this is all results available !
	}

	return nil
}

func (ctx *chatContactsQuery) scanPageData(rows *sql.Rows, into *pb.ChatCustomers) error {

	type (
		via struct {
			id    string // int64
			node  *pb.Peer
			alias pb.Peer
		}
	)

	var (
		vias   []*via
		getVia = func(id string) *via {
			var e, n = 0, len(vias)
			for ; e < n && id != vias[e].id; e++ {
				// lookup: match by original via( id: int! )
			}
			if e == n {
				panic(fmt.Errorf("via( id: %d ); not fetched", id))
			}
			return vias[e]
		}
		fetch = []any{ // TextDecoder{
			// via(s)
			DecodeText(func(src []byte) error {
				// parse: array(row(via))
				rows, err := pgtype.ParseUntypedTextArray(string(src))
				if err != nil {
					return err
				}

				var (
					data *via
					node *pb.Peer
					heap []pb.Peer

					page = into.GetVias() // input
					// data []*pb.Peer        // output

					// eval = make([]any, 4) // len(ctx.plan))
					size = len(rows.Elements)
					// temporary
					text pgtype.Text
					// scan plan
					scan = []TextDecoder{
						// id
						DecodeText(func(src []byte) error {

							err := postgres.Text{ // .Int8
								Value: &data.id,
							}.DecodeText(nil, src)

							if err == nil {
								node.Id = data.id // strconv.FormatInt(data.id, 10)
							}

							return err
						}),
						// type
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Type = text.String
							return nil
						}),
						// name
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Name = text.String
							return nil
						}),
					}
				)

				if 0 < size {
					// data = make([]*pb.Peer, 0, size)
					vias = make([]*via, 0, size)
				}

				if n := size - len(page); 1 < n {
					heap = make([]pb.Peer, n) // mempage; tidy
				}

				var (
					// c   int // column
					raw *pgtype.CompositeTextScanner
				)
				for r, row := range rows.Elements {
					raw = pgtype.NewCompositeTextScanner(nil, []byte(row))
					// RECORD
					node = nil // NEW
					if r < len(page) {
						// [INTO] given page records
						// [NOTE] order matters !
						node = page[r]
					} else if len(heap) > 0 {
						node = &heap[0]
						heap = heap[1:]
					}
					// ALLOC
					if node == nil {
						node = new(pb.Peer)
					}
					data = &via{
						node: node,
					}

					for _, col := range scan {
						raw.ScanDecoder(col)
						err = raw.Err()
						if err != nil {
							return err
						}
					}

					data.alias = pb.Peer{
						Id: strconv.Itoa(len(vias) + 1), // local: serial pk
					}
					vias = append(vias, data)
					// output
					into.Vias = append(into.Vias, node)
				}
				return nil
			}),
			// peer(s)
			DecodeText(func(src []byte) error {
				// parse: array(row(peer))
				rows, err := pgtype.ParseUntypedTextArray(string(src))
				if err != nil {
					return err
				}

				var (
					node *pb.Customer
					heap []pb.Customer

					page = into.GetPeers() // input
					data []*pb.Customer    // output

					plan = ctx.peers
					eval = make([]any, len(plan))
					size = len(rows.Elements)

					via pgtype.TextArray // pgtype.Int8Array
				)

				limit := ctx.input.Size
				into.Page = int32(ctx.input.Page)
				if 0 < limit && limit < size {
					size = limit
					into.Next = true
					// NOTE: unused "via" record MAY be present !
				}

				if e := ctx.input.indexField("via"); -1 < e {
					// hijack
					plan[e] = func(node *pb.Customer) any {
						return DecodeText(func(src []byte) error {
							return via.DecodeText(nil, src)
						})
					}
				}

				if 0 < size {
					data = make([]*pb.Customer, 0, size)
				}

				if n := size - len(page); 1 < n {
					heap = make([]pb.Customer, n) // mempage; tidy
				}

				var (
					c   int // column
					raw *pgtype.CompositeTextScanner
				)
				for r, row := range rows.Elements[0:size] {
					raw = pgtype.NewCompositeTextScanner(nil, []byte(row))
					// RECORD
					node = nil // NEW
					if r < len(page) {
						// [INTO] given page records
						// [NOTE] order matters !
						node = page[r]
					} else if len(heap) > 0 {
						node = &heap[0]
						heap = heap[1:]
					}
					// ALLOC
					if node == nil {
						node = new(pb.Customer)
					}
					// [BIND] data fields to scan row
					c = 0
					for _, bind := range ctx.peers {

						df := bind(node)
						if df != nil {
							eval[c] = df
							c++
							continue
						}
						// (df == nil)
						// omit; pseudo calc
					}

					for _, col := range eval {
						raw.ScanDecoder(col.(TextDecoder))
						err = raw.Err()
						if err != nil {
							return err
						}
					}
					// REFERENCES
					if n := len(via.Elements); n > 0 {
						refs := make([]*pb.Peer, 0, n)
						for _, oid := range via.Elements {
							if gate := getVia(oid.String); gate != nil {
								refs = append(refs, &gate.alias)
							}
						}
						node.Via = refs
					}
					// output: list
					data = append(data, node)
				}

				into.Peers = data
				if !into.Next && into.Page < 2 {
					into.Page = 0 // hide; ALL results available
				}

				return nil
			}),
		}
	)

	for rows.Next() {
		err := rows.Scan(fetch...)
		if err != nil {
			return err
		}
		// once; MUST: single row
		break
	}

	return rows.Err()
}
