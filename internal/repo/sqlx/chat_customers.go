package sqlxrepo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"github.com/lib/pq"
	"github.com/micro/micro/v3/service/errors"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/store/postgres"
)

const (
	ASC  = " ASC"
	DESC = " DESC"
)

type chatCustomersArgs struct {
	// Paging
	Page int
	Size int

	// Sorting
	Sort []string

	// Output
	Fields []string

	// Authorization
	PDC  int64
	Self int64

	// Arguments
	Q    string
	ID   []string
	Type string
	Via  *pb.Peer
}

func getCustomersInput(req *app.SearchOptions) (args chatCustomersArgs, err error) {

	// NOTE: Set default arguments.
	args.PDC = req.Context.Creds.Dc
	args.Q = ""
	args.Sort = []string{"name"}
	args.Page = 1
	args.Size = 5
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

	// NOTE: Set the search term (Q) if it's provided. If Q is '*' (wildcard), ignore it.
	if !app.IsPresent(req.Term) {
		args.Q = req.Term
	}

	// NOTE: Set the page number if it is greater than 0, else default to 1.
	if req.Page > 0 {
		args.Page = req.Page
	}

	// NOTE: Set the size of the page (number of results per page) if it's greater than 0, else default to 5.
	if req.Size > 0 {
		args.Size = req.Size
	}

	// NOTE: Check if there are sorting fields provided. If so, validate them and adjust the sort order.
	if len(req.Order) > 0 {
		args.Sort = app.FieldsFunc(req.Order, app.InlineFields)

		specialChars := []byte{'+', '-', '!', ' '}
		for _, spec := range args.Sort {
			if slices.Contains(specialChars, spec[0]) {
				spec = spec[1:]
			}

			switch spec {
			case "id", "type", "name":
				// Pass

			default:
				err = errors.BadRequest(
					"customers.query.sort.input",
					"customers( sort: [%s] ); input: no field support",
					spec,
				)
				return
			}
		}
	}

	// NOTE: Sort the fields for the request and remove any duplicates.
	slices.Sort(args.Fields)
	args.Fields = slices.Compact(args.Fields)
	for _, field := range args.Fields {
		switch field {
		case "id", "type", "name", "via":
			// Pass

		default:
			err = errors.BadRequest(
				"customers.query.fields.input",
				"customers{ %s }; input: no such field",
				field,
			)
			return
		}
	}

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

	// NOTE: Loop through the filter parameters in the request and handle each one based on the field.
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

	// NOTE: Get input from the request and handle any error if occurs.
	ctx.input, err = getCustomersInput(req)
	if err != nil {
		return
	}

	// NOTE: Set parameters for the query, including "pdc" from the input.
	ctx.Params = params{
		"pdc": ctx.input.PDC,
	}

	// NOTE: Construct the base query for retrieving peers data from the database.
	peerQuery := postgres.PGSQL.
		Select(
			`DISTINCT ON (x.external_id, c.type, x.name) x.external_id AS id`,
			`c.type`,
			`x.name`,
			`COALESCE(
				array_agg(DISTINCT b.id::::text) FILTER (WHERE b.id IS NOT NULL), ARRAY[]::::text[]
			) ||
        	COALESCE(
				array_agg(DISTINCT sa.id::::text) FILTER (WHERE sa.id IS NOT NULL), ARRAY[]::::text[]
			) AS via_ids`,
		).
		From(`chat.client x`).
		Join(`
			chat.channel c 
			ON x.id = c.user_id 
			AND c.domain_id = :pdc
		`).
		LeftJoin(`
			chat.bot b 
			ON c.connection::::int8 > 0 
			AND b.id = c.connection::::int8 
			AND b.dc = :pdc
		`).
		LeftJoin(`
			portal.user_account ua
			ON c.connection::::int8 = 0
			AND (
				CASE 
					WHEN c.type = 'portal' 
					THEN x.external_id::::uuid = ua.id 
					ELSE false 
				END
			)
		`).
		LeftJoin(`
			portal.user_service us 
			ON us.account_id = ua.id
		`).
		LeftJoin(`
			portal.service_app sa 
			ON sa.service_id = us.service_id
		`).
		Where(`NOT c.internal`).
		GroupBy(`x.external_id, c.type, x.name`)

	// NOTE: If there is a search query (`Q`), add a condition to the query for filtering by `x.name`.
	if ctx.input.Q != "" {
		ctx.Params.set("q", app.Substring(ctx.input.Q))
		peerQuery = peerQuery.Where("x.name ILIKE :q COLLATE \"default\"")
	}

	// NOTE: If there are IDs provided in the input, filter results by `external_id` using the `ANY` operator.
	if len(ctx.input.ID) > 0 {
		ctx.Params.set("id", ctx.input.ID)
		peerQuery = peerQuery.Where(sq.Expr("x.external_id = ANY(:id)"))
	}

	// NOTE: If a `type` is provided, filter results by `c.type`.
	if ctx.input.Type != "" {
		ctx.Params.set("type", ctx.input.Type)
		peerQuery = peerQuery.Where("c.type = :type")
	}

	// NOTE: If pagination is specified, apply offset and limit to the query.
	if ctx.input.Size > 0 {
		if ctx.input.Page > 1 {
			peerQuery = peerQuery.Offset(uint64((ctx.input.Page - 1) * ctx.input.Size))
		}
		peerQuery = peerQuery.Limit(uint64(ctx.input.Size + 1))
	}

	// NOTE: Iterate through the sort specifications and process each one for ordering the query.
	if len(ctx.input.Sort) > 0 {
		for _, spec := range ctx.input.Sort {
			order := ASC

			// NOTE: Process the sorting symbols to determine the order (ascending or descending).
			switch spec[0] {
			case '+', ' ':
				spec = spec[1:]

			case '-', '!':
				spec = spec[1:]
				order = DESC
			}

			// NOTE: Translate sorting field names to database column identifiers.
			switch spec {
			case "id":
				spec = "x.external_id"

			case "type":
				spec = "c.type"

			case "name":
				spec = "x.name"
			}

			// NOTE: Apply the sorting to the query using the processed field and order.
			peerQuery = peerQuery.OrderBy(spec + order)
		}
	}

	// NOTE: Check if `via` input is provided and process filtering conditions by via's id|type|name.
	if via := ctx.input.Via; via != nil {
		if via.Id != "" {
			ctx.Params.set("via_id", via.Id)
			peerQuery = peerQuery.Where("b.id::::text = :via_id OR sa.id::::text = :via_id")
		}

		if via.Type != "" {
			if via.Type == "portal" {
				peerQuery = peerQuery.Where("c.connection::::int8 = 0")
			} else {
				ctx.Params.set("via_type", via.Type)
				peerQuery = peerQuery.Where("b.provider = :via_type")
			}
		}

		if via.Name != "" {
			ctx.Params.set("via_name", via.Name)
			peerQuery = peerQuery.Where("b.name = :via_name OR sa.app = :via_name")
		}
	}

	// NOTE: Iterate through the `ctx.input.Fields` to decide which fields to select and whether to include the `via` field.
	var withVia bool
	var peerSelectFields []string
	for _, field := range ctx.input.Fields {
		switch field {
		case "id":
			peerSelectFields = append(peerSelectFields, "p.id")

		case "type":
			peerSelectFields = append(peerSelectFields, "p.type")

		case "name":
			peerSelectFields = append(peerSelectFields, "p.name")

		case "via":
			withVia = true
		}
	}

	// NOTE: Create a CTE named "peer" with the previously constructed query `peerQuery` as its expression.
	ctx.With(CTE{
		Name: "peer",
		Expr: peerQuery,
	})

	// NOTE: Check if the "via" field is not selected; if not, execute a basic query on the "peer" CTE.
	if !withVia {
		ctx.Query = postgres.PGSQL.
			Select(peerSelectFields...).
			From(`peer p`)

		ctx.fetch = ctx.scanContactsRows

		return
	}

	// NOTE: Define a Common Table Expression (CTE) named "via" with the SQL query to fetch related via information.
	ctx.With(CTE{
		Name: "via",
		Expr: postgres.PGSQL.
			Select(
				`DISTINCT COALESCE(b.id::::text, sa.id::::text) AS id`,
				`COALESCE(b.provider, 'portal') AS type`,
				`COALESCE(b.name, sa.app) AS name`,
			).
			FromSelect(
				postgres.PGSQL.
					Select(`*`).
					From(`peer`).
					Limit(uint64(ctx.input.Size)),
				`p`,
			).
			LeftJoin(`
				chat.bot b 
				ON b.id::::text = ANY(p.via_ids)
			`).
			LeftJoin(`
				portal.service_app sa 
				ON sa.id::::text = ANY(p.via_ids)
			`),
	})

	// NOTE: Prepare the select fields for the final query by including the "via_ids" and peer fields.
	selectFields := []string{"'via_ids', p.via_ids"}
	for _, field := range peerSelectFields {
		selectFields = append(selectFields, fmt.Sprintf("'%s', %s", field[2:], field))
	}

	// NOTE: Construct the final query with selected fields and join the "peer" and "via" CTEs for the final data fetch.
	ctx.Query = postgres.PGSQL.
		Select(
			fmt.Sprintf(`
				ARRAY(
					SELECT json_build_object(%s)
					FROM peer p
				) AS peer`, strings.Join(selectFields, ",")),
			`ARRAY(
				SELECT json_build_object(
					'id', v.id, 
					'type', v.type, 
					'name', v.name
				) 
				FROM via v
			) AS via`).
		From(`peer p`).
		From(`via v`)

	// NOTE: Set the function to scan the results, including both peer and via data.
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
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Name   string   `json:"name"`
	ViaIDs []string `json:"via_ids"`
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
		into.Peers = into.Peers[:limit]
	}

	return nil
}

func (ctx *chatContactsQuery) scanContactsRowsWithVia(rows *sql.Rows, into *pb.ChatCustomers) error {
	var vias []sqlVia
	var peers []sqlPeer

	// NOTE: scan only first row
	if rows.Next() {
		err := rows.Scan(pq.Array(&peers), pq.Array(&vias))
		if err != nil {
			return fmt.Errorf("error: reading the result: %v", err)
		}
	}

	// NOTE: Set vias to response
	for _, v := range vias {
		into.Vias = append(into.Vias, &pb.Peer{
			Id:   strings.ReplaceAll(v.ID, "-", ""),
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
			Via:  make([]*pb.Peer, 0, len(p.ViaIDs)),
		}

		for _, viaID := range p.ViaIDs {
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
		into.Peers = into.Peers[:limit]
	}

	return nil
}
