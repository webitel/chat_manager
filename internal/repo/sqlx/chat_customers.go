package sqlxrepo

import (
	"database/sql"
	"fmt"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
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

type chatContactsQuery struct {
	input chatCustomersArgs
	SELECT
	peers dataFetch[*pb.Customer]
	// vias  dataFetch[*pb.Peer]
	fetch func(*sql.Rows, *pb.ChatCustomers) error
}

func getContactsQuery(req *app.SearchOptions) (ctx chatContactsQuery, err error) {

	ctx.input, err = getCustomersInput(req)
	if err != nil {
		return ctx, err
	}

	ctx.Params = params{
		"pdc": ctx.input.PDC,
	}

	var left string
	// region: ----- resolve: type, via -----
	left = "c"
	gateAlias := "a"
	gate := &JOIN{
		Kind:  "JOIN", // INNER; chat.bot EXISTS ! IF NOT -- peer MAY be NOT reachable !
		Table: sq.Expr("chat.bot"),
		Alias: gateAlias,
		Pred: sq.And{
			sq.Expr(ident(gateAlias, "id") + " = " + ident(left, "connection::::int8")),
			sq.Expr(ident(gateAlias, "dc") + " = :pdc"),
		},
	}
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "type"),
			ident(left, "user_id"),
			fmt.Sprintf(
				"array_agg(DISTINCT %s) via",
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
			"NOT "+ident(left, "internal"), // REFERENCE chat.client
		).
		GroupBy(
			ident(left, "user_id"),
			ident(left, "type"),
		)
	// peers( type: string )
	if typeOf := ctx.input.Type; typeOf != "" {
		ctx.Params.set("peer.type", app.Substring(typeOf))
		ctx.Query = ctx.Query.Where(
			ident(left, "type") + " ILIKE :peer.type", // COLLATE 'C'
		)
	}
	// peers( via: peer )
	if via := ctx.input.Via; via != nil {
		if via.Id != "" {
			oid, re := strconv.ParseInt(via.Id, 10, 64)
			if re != nil || oid < 1 {
				err = errors.BadRequest(
					"customers.query.via.id.input",
					"customers( via.id: int ); input: invalid id",
				)
			}
			// ctx.Params.set("via.id", oid)
			ctx.Params.set("via.id", via.Id)
			ctx.Query = ctx.Query.Where(
				// ident(gate, "id") + " = :via.id",
				ident(left, "connection") + " = :via.id",
			)
		}
		if via.Type != "" {
			ctx.Params.set("via.type", app.Substring(via.Type))
			ctx.Query = ctx.Query.Where(
				ident(gate.Alias, "provider") + " ILIKE :via.type", // COLLATE 'C'
			)
		}
		if via.Name != "" {
			ctx.Params.set("via.name", app.Substring(via.Name))
			gate.Pred = append(gate.Pred.(sq.And),
				sq.Expr(ident(gate.Alias, "name")+" ILIKE :via.name COLLATE \"default\""),
			)
			// ctx.Query = ctx.Query.Where(
			// 	ident(gate.Alias, "name") + " ILIKE :via.name COLLATE \"default\"",
			// )
		}
	}

	peerQ := ctx.Query

	ctx.Query = postgres.PGSQL.
		Select(
			// c.user_id oid,
			ident("x", "external_id")+" id", // text
			ident("c", "type"),              // text
			// ident("c", "name"),              // text
			// // (via)
			// ident("c", "via"), // int8[]
		).
		From(
			"chat.client x",
		).
		JoinClause(&JOIN{
			Kind:  "JOIN", // INNER; type, via ...
			Table: peerQ.Prefix("(").Suffix(")"),
			Alias: "c",
			Pred: sq.Expr(
				"c.user_id = x.id",
			),
		})

	// peers( q: string )
	if q := ctx.input.Q; q != "" && !app.IsPresent(q) {
		ctx.Params.set("q", app.Substring(q))
		ctx.Query = ctx.Query.Where(
			ident("x", "name") + " ILIKE :q COLLATE \"default\"",
		)
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
		ctx.Query = ctx.Query.Where(
			ident("x", "external_id") + eq,
		)
	}
	// peers( sort: fields )
	// [ORDER-BY]: sort
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
			order = " DESC"
		}
		switch spec {
		// complex
		case "id":
			{
				spec = ident("x", "external_id")
			}
		case "type":
			{
				spec = ident("c", "type")
			}
		case "name":
			{
				spec = ident("x", "name")
			}
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
		ctx.Query = ctx.Query.
			OrderBy(spec + order)
	}
	// [OFFSET|LIMIT]: paging
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			ctx.Query = ctx.Query.Offset(
				(uint64)((page - 1) * (size)),
			)
		}
		// LIMIT (size+1) -- to indicate whether there are more result entries
		ctx.Query = ctx.Query.Limit(
			(uint64)(size + 1),
		)
	}
	// endregion: ----- resolve: type, via -----

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
				ident("c", "via"), // int8[]
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
	ctx.With(CTE{
		Name: viewVia,
		Expr: postgres.PGSQL.
			Select(
				"a.id",
				"a.provider \"type\"",
				"a.name",
			).
			From(
				"chat.bot a",
			).
			JoinClause(CompactSQL(
				`JOIN
				(
					SELECT
						UNNEST(e.via) id
					FROM
						peer e
					GROUP BY
						1
				) q USING(id)`,
			)),
	})
	ctx.Query = postgres.PGSQL.
		Select(
			`ARRAY(SELECT e FROM via e) via`,
			`ARRAY(SELECT e FROM peer e) peer`,
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
			oid   int64
			node  *pb.Peer
			alias pb.Peer
		}
	)

	var (
		vias   []*via
		getVia = func(oid int64) *via {
			var e, n = 0, len(vias)
			for ; e < n && oid != vias[e].oid; e++ {
				// lookup: match by original via( id: int! )
			}
			if e == n {
				panic(fmt.Errorf("via( id: %d ); not fetched", oid))
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

							err := postgres.Int8{
								Value: &data.oid,
							}.DecodeText(nil, src)

							if err == nil {
								node.Id = strconv.FormatInt(data.oid, 10)
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

					via pgtype.Int8Array
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
							if gate := getVia(oid.Int); gate != nil {
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

// func (ctx *chatContactsQuery) scanRows(rows *sql.Rows, into *pb.ContactPeers) error {
// 	return ctx.fetch(rows, into)
// }
