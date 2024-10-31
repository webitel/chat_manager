package sqlxrepo

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	api "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/auth"
	dbx "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/chat_manager/store/postgres"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ CatalogStore = (*sqlxRepository)(nil)

// Query of external chat customers
func (c *sqlxRepository) GetCustomers(req *app.SearchOptions, res *api.ChatCustomers) error {

	ctx := req.Context.Context
	cte, err := getContactsQuery(req)
	if err != nil {
		return err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	// err = cte.scanRows(rows, res)
	err = cte.fetch(rows, res)
	if err != nil {
		return err
	}

	return nil
}

// Query of chat conversations
func (c *sqlxRepository) GetDialogs(req *app.SearchOptions, res *api.ChatDialogs) error {

	ctx := req.Context.Context
	cte, plan, err := searchChatDialogsQuery(req)
	if err != nil {
		return err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	err = fetchDialogRows(
		rows, plan, res, req.GetSize(),
	)
	if err != nil {
		return err
	}

	return nil
	// panic("not implemented")
}

// Query of chat participants
func (c *sqlxRepository) GetMembers(req *app.SearchOptions) (*api.ChatMembers, error) {

	ctx := req.Context.Context
	cte, plan, err := searchChatMembersQuery(req) // selectChatMemberQuery(req) // selectMemberQuery(req)
	if err != nil {
		return nil, err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res api.ChatMembers
	err = fetchMemberRows(
		rows, plan, &res, req.GetSize(),
	)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// Query of the chat history messages ; back in time
func (c *sqlxRepository) GetHistory(req *app.SearchOptions) (*api.ChatMessages, error) {

	ctx := req.Context.Context
	cte, err := getHistoryQuery(req, false)
	if err != nil {
		return nil, err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var page api.ChatMessages
	err = cte.scanRows(rows, &page)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

func (c *sqlxRepository) GetContactChatHistory(req *app.SearchOptions) (*api.GetContactChatHistoryResponse, error) {

	ctx := req.Context.Context
	cte, err := getContactHistoryQuery(req, false)
	if err != nil {
		return nil, err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var page api.GetContactChatHistoryResponse
	err = cte.scanRows(rows, req, &page)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// Query of the chat history updates ; forward offset
func (c *sqlxRepository) GetUpdates(req *app.SearchOptions) (*api.ChatMessages, error) {

	ctx := req.Context.Context
	cte, err := getHistoryQuery(req, true)
	if err != nil {
		return nil, err
	}

	query, args, err := cte.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := c.db.QueryContext(
		ctx, query, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var page api.ChatMessages
	err = cte.scanRows(rows, &page)
	if err != nil {
		return nil, err
	}

	return &page, nil
}

// --------------------------------------- //

type dataField[TRow any] func(node TRow) any
type dataFetch[TRow any] []dataField[TRow]

// unsafe: internal usage only
func ident(rel ...string) string {
	return strings.Join(rel, ".")
}

// WITH query AS (SELECT c.id FROM chat.conversation c)
func searchDialogQuery(req *app.SearchOptions, ctx *SELECT) (*CTE, error) {

	var params struct {
		Q      string
		ID     []string // UUID
		Mode   uint8    // NOT implemented
		Peer   *api.Peer
		Date   *api.Timerange
		Online *bool
	}

	// Arguments
	// ( q: string )
	if v := req.Term; v != "" {
		params.Q = v
	}
	// ( mode: access )
	if v := req.Access; auth.NONE < v {
		params.Mode = v
	}
	for param, value := range req.Filter {
		if value == nil {
			continue // ommitted
		}
		switch param {
		// ( id: id )
		case "id":
			{
				switch data := value.(type) {
				case []string:
					params.ID = data
				case string:
					if data == "" {
						break
					}
					params.ID = []string{data}
				default:
					return nil, errors.BadRequest(
						"chat.dialogs.args.id.error",
						"dialogs( id:%v ) convert %[1]T into [id!]",
						value,
					)
				}
			}
		// ( date: timerange )
		case "date":
			{
				switch data := value.(type) {
				case *api.Timerange:
					params.Date = data
				default:
					return nil, errors.BadRequest(
						"chat.dialogs.args.date.error",
						"dialogs( date:%v ) convert %[1]T into timerange( since:date, until:date )",
						value,
					)
				}
			}
		// ( peer: peer )
		case "peer":
			{
				switch data := value.(type) {
				case *api.Peer:
					params.Peer = data
				default:
					return nil, errors.BadRequest(
						"chat.dialogs.args.peer.error",
						"dialogs( peer:%v ) convert %[1]T into peer( id:string, type:string, name:string )",
						value,
					)
				}
			}
		// ( closed: bool )
		case "online":
			{
				switch data := value.(type) {
				case *wrapperspb.BoolValue:
					{
						if data == nil {
							break // omitted
						}
						is := data.GetValue()
						params.Online = &is
					}
				case *bool:
					params.Online = data
				case bool:
					params.Online = &data
				default:
					return nil, errors.BadRequest(
						"chat.dialogs.args.online.error",
						"dialogs( online: %v ) convert %[1]T into bool",
						value,
					)
				}
			}
		default:
			return nil, errors.BadRequest(
				"chat.dialogs.args.error",
				"dialogs( %s: ? ) no such argument",
				param,
			)
		}
	}
	// APPLY
	const (
		dialogAs      = "c" // chat.conversation
		memberAs      = "m" // chat.channel
		userBotAs     = "b" // flow.acr_routing_scheme
		userAgentAs   = "u" // directory.wbt_auth
		userContactAs = "x" // chat.client
		// FROM
		left = dialogAs
	)
	var (
		// ok  bool                                        = true // DC(!) // once // has query ?
		cte = postgres.PGSQL.
			Select("DISTINCT c.id").                    // id
			From("chat.conversation " + left).          // threads
			Where(ident(left, "domain_id") + " = :pdc") // domain.dc
		// JOINs alias[ed]
		join = map[string]Sqlizer{}
		// INNER JOIN chat.channel AS m
		joinMember = func() bool {
			const as = memberAs // right
			if _, ok := join[as]; ok {
				return false // already done !
			}
			expr := fmt.Sprintf(
				"JOIN chat.channel %s ON %[1]s.conversation_id = %s.id",
				as, left,
			)
			cte = cte.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return true
		}
		// INNER JOIN flow.acr_routing_scheme AS b
		joinUserBot = func() bool {
			const as = userBotAs // right
			if _, ok := join[as]; ok {
				return false // already done !
			}
			expr := fmt.Sprintf(
				"JOIN flow.acr_routing_scheme %s ON %[1]s.id = (%s.props->>'flow')::::int8",
				as, left,
			)
			cte = cte.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return true
		}
		// LEFT JOIN directory.wbt_auth AS u
		joinUserAgent = func() bool {
			const as = userAgentAs // right
			if _, ok := join[as]; ok {
				return false // already done !
			}
			_ = joinMember() // MUST: ensure
			const left = memberAs
			expr := fmt.Sprintf(
				"LEFT JOIN directory.wbt_auth %[1]s ON %[2]s.internal AND %[1]s.id = %[2]s.user_id",
				as, left,
			)
			cte = cte.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return true
		}
		// LEFT JOIN chat.client AS x
		joinUserContact = func() bool {
			const as = userContactAs // right
			if _, ok := join[as]; ok {
				return false // already done !
			}
			_ = joinMember() // MUST: ensure
			const left = memberAs
			expr := fmt.Sprintf(
				"LEFT JOIN chat.client %[1]s ON NOT %[2]s.internal AND %[1]s.id = %[2]s.user_id",
				as, left,
			)
			cte = cte.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return true
		}
	)
	// ( id: [id!] )
	switch len(params.ID) {
	case 0: // omitted
	case 1:
		{
			var id pgtype.UUID
			err := id.Set(params.ID[0])
			if err != nil {
				// ERR: invalid UUID
				err = errors.BadRequest(
					"chat.dialogs.args.id.error",
					"dialogs( id: %s ) convert %[1]T into id!",
					params.ID[0],
				)
				return nil, err
			}
			// ok = true // has query !
			ctx.Params.set("thread_id", &id)
			cte = cte.Where(left + ".conversation_id = :thread_id")
		}
	default:
		{
			var id pgtype.UUIDArray
			err := id.Set(params.ID)
			if err != nil {
				// ERR: invalid UUID
				err = errors.BadRequest(
					"chat.dialogs.args.id.error",
					"dialogs( id: %v ) convert %[1]T into [id!]",
					params.ID,
				)
				return nil, err
			}
			// ok = true // has query !
			ctx.Params.set("thread_id", &id)
			cte = cte.Where(left + ".conversation_id = ANY(:thread_id)")
		}
	}
	// ( date: timerange )
	if vs := params.Date; vs != nil {
		if 0 < vs.Since {
			var since pgtype.Timestamp
			_ = since.Set(app.EpochtimeDate(vs.Since, app.TimePrecision).UTC())
			// ok = true // has query !
			ctx.Params.set("since", &since)
			cte = cte.Where(left + ".created_at >= :since")
		}
		if 0 < vs.Until {
			var until pgtype.Timestamp
			_ = until.Set(app.EpochtimeDate(vs.Until, app.TimePrecision).UTC())
			// ok = true // has query !
			ctx.Params.set("until", &until)
			cte = cte.Where("coalesce(" + left + ".closed_at, " + left + ".created_at) < :until")
		}
	}
	// ( peer: peer )
	if vs := params.Peer; vs != nil {
		if typeOf := vs.Type; typeOf != "" {
			switch vs.Type {
			case "bot": // flow::schema
				{
					var oid int64
					if vs.Id != "" {
						if oid, _ = strconv.ParseInt(vs.Id, 10, 64); oid < 1 {
							return nil, errors.BadRequest(
								"chat.peer.bot.id.error",
								"( peer:bot( id: %s )) invalid id",
								vs.Id,
							)
						}
					}
					if oid > 0 {
						ctx.Params.set("peer.id", vs.Id) // oid)
						cte = cte.Where(left + ".props->>'flow' = :peer.id")
					}
					if vs.Name != "" && vs.Name != "*" {
						_ = joinUserBot() // INNER JOIN flow.acr_flow_scheme AS b
						ctx.Params.set("peer.cn", app.Substring(vs.Name))
						cte = cte.Where(userBotAs + ".name::::text ILIKE :peer.cn COLLATE \"default\"")
					}
				}
			case "user": // user::webitel
				{
					var oid int64
					if vs.Id != "" {
						if oid, _ = strconv.ParseInt(vs.Id, 10, 64); oid < 1 {
							return nil, errors.BadRequest(
								"chat.peer.user.id.error",
								"( peer:user( id: %s )) invalid id",
								vs.Id,
							)
						}
					}
					if oid > 0 {
						_ = joinUserAgent() // LEFT JOIN directory.wbt_auth AS u
						ctx.Params.set("peer.id", oid)
						cte = cte.Where(userAgentAs + ".id = :peer.id")
					}
					if vs.Name != "" && vs.Name != "*" {
						_ = joinUserAgent() // LEFT JOIN directory.wbt_auth AS u
						ctx.Params.set("peer.cn", app.Substring(vs.Name))
						cte = cte.Where(fmt.Sprintf(
							"coalesce(%[1]s.name ,%[1]s.auth::::text) ILIKE :peer.cn COLLATE \"default\"",
							userAgentAs,
						))
					}
				}
			default: // contact::external
				{
					if vs.Id != "" && vs.Id != "*" {
						_ = joinUserContact() // LEFT JOIN chat.client AS x
						ctx.Params.set("peer.id", app.Substring(vs.Id))
						cte = cte.Where(userContactAs + ".external_id ILIKE :peer.id")
					}
					if vs.Name != "" && vs.Name != "*" {
						_ = joinUserContact() // LEFT JOIN chat.client AS x
						ctx.Params.set("peer.cn", app.Substring(vs.Name))
						cte = cte.Where(ident(userContactAs, "first_name") + " ILIKE :peer.cn COLLATE \"default\"")
					}
				}
			}
		} // else { // id|name
		//
		// }
	}
	// ( closed: bool )
	if is := params.Online; is != nil {
		if *(is) {
			cte = cte.Where(left + ".closed_at ISNULL") // active ?
		} else {
			cte = cte.Where(left + ".closed_at NOTNULL") // closed ?
		}
	}

	query := CTE{
		Name: "query",
		Cols: []string{"id"}, // thread_id
		Expr: cte,
	}
	ctx.With(query)
	return &query, nil
}

func fetchPeerRow(value **api.Peer) any {
	return DecodeText(func(src []byte) error {

		res := *(value) // cache
		*(value) = nil  // NULLify

		if len(src) == 0 {
			return nil // NULL
		}

		if res == nil {
			// ALLOC
			res = new(api.Peer)
		}

		var (
			ok  bool // false
			str pgtype.Text
			row = []TextDecoder{
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Id = str.String
					ok = ok || (str.String != "" && str.String != "0") // && str.Status == pgtype.Present
					return nil
				}),
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Type = str.String
					ok = ok || (str.String != "" && str.String != "unknown") // && str.Status == pgtype.Present
					return nil
				}),
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Name = str.String
					ok = ok || (str.String != "" && str.String != "[deleted]") // && str.Status == pgtype.Present
					return nil
				}),
			}
			raw = pgtype.NewCompositeTextScanner(nil, src)
		)

		var err error
		for _, col := range row {

			raw.ScanDecoder(col)

			err = raw.Err()
			if err != nil {
				return err
			}
		}

		if ok {
			*(value) = res
		}

		return nil
	})
}

func fetchQueueRow(value **api.Peer) any {
	return DecodeText(func(src []byte) error {

		res := *(value) // cache
		*(value) = nil  // NULLify

		if len(src) == 0 {
			return nil // NULL
		}

		if res == nil {
			// ALLOC
			res = new(api.Peer)
		}

		var (
			ok  bool // false
			str pgtype.Text
			row = []TextDecoder{
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Id = str.String
					ok = ok || (str.String != "" && str.String != "0") // && str.Status == pgtype.Present
					return nil
				}),
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Type = str.String
					ok = ok || (str.String != "" && str.String != "unknown") // && str.Status == pgtype.Present
					return nil
				}),
				DecodeText(func(src []byte) error {
					err := str.DecodeText(nil, src)
					if err != nil {
						return err
					}
					res.Name = str.String
					ok = ok || (str.String != "" && str.String != "unknown") // && str.Status == pgtype.Present
					return nil
				}),
			}
			raw = pgtype.NewCompositeTextScanner(nil, src)
		)

		var err error
		for _, col := range row {

			raw.ScanDecoder(col)

			err = raw.Err()
			if err != nil {
				return err
			}
		}

		if ok {
			*(value) = res
		}

		return nil
	})
}

func fetchInvitedRow(value **api.Chat_Invite) any {
	return DecodeText(func(src []byte) error {

		res := *(value) // cache
		*(value) = nil  // NULLify

		if len(src) == 0 {
			return nil // NULL
		}

		if res == nil {
			res = new(api.Chat_Invite)
		}

		var (
			ok  bool // false
			row = []TextDecoder{
				DecodeText(func(src []byte) error {
					var date pgtype.Timestamptz
					err := date.DecodeText(nil, src)
					if err != nil {
						return err
					}
					// if date.Status == pgtype.Present {
					res.Date = app.DateEpochtime(date.Time, app.TimePrecision)
					// }
					ok = ok || !date.Time.IsZero()
					return nil
				}),
				DecodeText(func(src []byte) error {
					var from pgtype.UUID
					err := from.DecodeText(nil, src)
					if err != nil {
						return err
					}
					if from.Status == pgtype.Present {
						res.From = hex.EncodeToString(from.Bytes[:])
						ok = true
					}
					// ok = ok || from.Status == pgtype.Present
					return nil
				}),
			}
			raw = pgtype.NewCompositeTextScanner(nil, src)
		)

		var err error
		for _, col := range row {

			raw.ScanDecoder(col)

			err = raw.Err()
			if err != nil {
				return err
			}
		}

		if ok {
			*(value) = res
		}

		return nil
	})
}

func fetchMemberRows(rows *sql.Rows, plan dataFetch[*api.Chat], into *api.ChatMembers, limit int) (err error) {
	var (
		node *api.Chat
		heap []api.Chat

		page = into.Data // input
		data []*api.Chat // output

		eval = make([]any, len(plan))
	)

	if 0 < limit {
		data = make([]*api.Chat, 0, limit)
	}

	if n := limit - len(page); 1 < n {
		heap = make([]api.Chat, n) // mempage; tidy
	}

	var r, c int // [r]ow, [c]olumn
	for rows.Next() {
		// LIMIT
		if 0 < limit && len(data) == limit {
			into.Next = true
			if into.Page < 1 {
				into.Page = 1
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
			node = new(api.Chat)
		}

		// [BIND] data fields to scan row
		c = 0
		for _, bind := range plan {
			// if !raw.Next() { /// .ScanValue calls .Next(!)
			// 	break
			// }

			df := bind(node)
			if df != nil {
				eval[c] = df
				c++
				continue
			}
			// (df == nil)
			// omit; pseudo calc
		}

		err = rows.Scan(eval[0:c]...)
		if err != nil {
			break
		}

		data = append(data, node)
		r++ // advance
	}

	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		return err
	}

	if !into.Next && into.Page <= 1 {
		// The first page with NO more results !
		into.Page = 0 // Hide: NO paging !
	}

	into.Data = data
	return nil
}

func fetchFileRow(value **api.File) any {
	return DecodeText(func(src []byte) error {

		res := *(value) // Value
		*(value) = nil  // NULL
		if len(src) == 0 {
			return nil // NULL
		}

		var (
			ok   bool // false
			text pgtype.Text
			int8 pgtype.Int8
			row  = []TextDecoder{
				// id
				DecodeText(func(src []byte) error {
					err := int8.DecodeText(nil, src)
					if err == nil && int8.Status == pgtype.Present {
						res.Id = strconv.FormatInt(int8.Int, 10) // int8.Int
						ok = true
					}
					return err
				}),
				// size
				DecodeText(func(src []byte) error {
					err := int8.DecodeText(nil, src)
					if err == nil && int8.Status == pgtype.Present {
						res.Size = int8.Int
						ok = true
					}
					return err
				}),
				// type
				DecodeText(func(src []byte) error {
					err := text.DecodeText(nil, src)
					if err == nil && text.Status == pgtype.Present {
						res.Type = text.String
						ok = true
					}
					return err
				}),
				// name
				DecodeText(func(src []byte) error {
					err := text.DecodeText(nil, src)
					if err == nil && text.Status == pgtype.Present {
						res.Name = text.String
						ok = true
					}
					return err
				}),
			}
			raw = pgtype.NewCompositeTextScanner(nil, src)
		)

		if res == nil {
			res = new(api.File)
		}

		var err error
		for _, col := range row {

			raw.ScanDecoder(col)

			err = raw.Err()
			if err != nil {
				return err
			}
		}

		if ok {
			*(value) = res
		}

		return nil
	})
}

func fetchContactFileRow(value **api.MessageFile) any {
	return DecodeText(func(src []byte) error {

		res := *(value) // Value
		*(value) = nil  // NULL
		if len(src) == 0 {
			return nil // NULL
		}

		var (
			ok   bool // false
			text pgtype.Text
			int8 pgtype.Int8
			row  = []TextDecoder{
				// id
				DecodeText(func(src []byte) error {
					err := int8.DecodeText(nil, src)
					if err == nil && int8.Status == pgtype.Present {
						res.Id = strconv.FormatInt(int8.Int, 10) // int8.Int
						ok = true
					}
					return err
				}),
				// size
				DecodeText(func(src []byte) error {
					err := int8.DecodeText(nil, src)
					if err == nil && int8.Status == pgtype.Present {
						res.Size = int8.Int
						ok = true
					}
					return err
				}),
				// type
				DecodeText(func(src []byte) error {
					err := text.DecodeText(nil, src)
					if err == nil && text.Status == pgtype.Present {
						res.Type = text.String
						ok = true
					}
					return err
				}),
				// name
				DecodeText(func(src []byte) error {
					err := text.DecodeText(nil, src)
					if err == nil && text.Status == pgtype.Present {
						res.Name = text.String
						ok = true
					}
					return err
				}),
				DecodeText(func(src []byte) error {
					err := text.DecodeText(nil, src)
					if err == nil && text.Status == pgtype.Present {
						res.Url = text.String
						ok = true
					}
					return err
				}),
			}
			raw = pgtype.NewCompositeTextScanner(nil, src)
		)

		if res == nil {
			res = new(api.MessageFile)
		}

		var err error
		for _, col := range row {

			raw.ScanDecoder(col)

			err = raw.Err()
			if err != nil {
				return err
			}
		}

		if ok {
			*(value) = res
		}

		return nil
	})
}

type queryChannelArgs struct {
	// REQUIRED. Mandatory(!)
	DC int64
	// // Type of the channel peer
	// Type []string
	// VIA text gateway transport.
	Via *api.Peer
	// Peer *-like participant(s).
	Peer *api.Peer
	// Date timerange within ...
	Date *api.Timerange
	// Online participants.
	// <nil> -- any; whatevent
	// <true> -- connected; active
	// <false> -- disconnected; kicked
	Online *bool
	// Conversation thread IDs
	ThreadID []string
	// Participants chat IDs
	MemberID []string
}

// [WITH] channel AS ( SELECT channel FULL JOIN invite UNION ALL conversation )
func selectChannelQuery(args queryChannelArgs) (ctx *SELECT, err error) {
	// alias(es)
	const (
		threadAs = "c"
		memberAs = "c"
		inviteAs = "e"
	)

	var (
		// FROM chat.conversation AS c
		threadExpr sq.And // strings.Builder
		// FROM chat.channel AS c
		memberExpr sq.And // strings.Builder
		// FROM chat.invite AS e
		inviteExpr sq.And // strings.Builder
	)

	ctx = &SELECT{
		Params: params{
			"pdc": args.DC,
		},
	}
	// Mandatory(:pdc) filter !
	for _, from := range []struct {
		Alias string
		Where *sq.And // *[]Sqlizer
	}{
		{threadAs, &threadExpr}, // conversation
		{memberAs, &memberExpr}, // channel
		{inviteAs, &inviteExpr}, // invite
	} {
		*(from.Where) = append(
			*(from.Where), sq.Expr(
				ident(from.Alias, "domain_id")+" = :pdc",
			),
		)
	}

	// for _, typo := range args.Type {
	switch args.Peer.GetType() { // typo {
	// none
	case "", "*":
		break // continue
	// internal
	case "bot":
		// UNION ALL chat.conversation
	case "user":
		// UNION ALL chat.channel WHERE (internal)
		// UNION ALL chat.invite
	// external:
	default:
		// UNION ALL chat.channel WHERE NOT (internal)
	}
	// }

	if vs := args.ThreadID; len(vs) > 0 {
		var threadID pgtype.UUIDArray // pgtype.TextArray
		err := threadID.Set(vs)
		if err != nil {
			return nil, err
		}
		if len(threadID.Elements) == 1 {
			ctx.Params.set("thread.id", &threadID.Elements[0])
			threadExpr = append(threadExpr,
				sq.Expr(ident(threadAs, "id")+" = :thread.id"),
			)
			memberExpr = append(memberExpr,
				sq.Expr(ident(memberAs, "conversation_id")+" = :thread.id"),
			)
			inviteExpr = append(inviteExpr,
				sq.Expr(ident(inviteAs, "conversation_id")+" = :thread.id"),
			)
		} else {
			ctx.Params.set("thread.id", &threadID)
			threadExpr = append(threadExpr,
				sq.Expr(ident(threadAs, "id")+" = ANY(:thread.id)"),
			)
			memberExpr = append(memberExpr,
				sq.Expr(ident(memberAs, "conversation_id")+" = ANY(:thread.id)"),
			)
			inviteExpr = append(inviteExpr,
				sq.Expr(ident(inviteAs, "conversation_id")+" = ANY(:thread.id)"),
			)
		}
	}

	if vs := args.MemberID; len(vs) > 0 {
		var memberID pgtype.UUIDArray // pgtype.TextArray
		err := memberID.Set(vs)
		if err != nil {
			return nil, err
		}
		if len(memberID.Elements) == 1 {
			ctx.Params.set("member.id", &memberID.Elements[0])
			threadExpr = append(threadExpr,
				sq.Expr(ident(threadAs, "id")+" = :member.id"),
			)
			memberExpr = append(memberExpr,
				sq.Expr(ident(memberAs, "id")+" = :member.id"),
			)
			inviteExpr = append(inviteExpr,
				sq.Expr(ident(inviteAs, "id")+" = :member.id"),
			)
		} else {
			ctx.Params.set("member.id", &memberID)
			threadExpr = append(threadExpr,
				sq.Expr(ident(threadAs, "id")+" = ANY(:member.id)"),
			)
			memberExpr = append(memberExpr,
				sq.Expr(ident(memberAs, "id")+" = ANY(:member.id)"),
			)
			inviteExpr = append(inviteExpr,
				sq.Expr(ident(inviteAs, "id")+" = ANY(:member.id)"),
			)
		}
	}

	if vs := args.Online; vs != nil {
		// closed_at
		const (
			active = " ISNULL"
			closed = " NOTNULL"
		)
		state := closed
		if *(vs) {
			state = active
		}
		threadExpr = append(threadExpr,
			sq.Expr(ident(threadAs, "closed_at")+state),
		)
		memberExpr = append(memberExpr,
			sq.Expr(ident(memberAs, "closed_at")+state),
		)
		inviteExpr = append(inviteExpr,
			sq.Expr(ident(inviteAs, "closed_at")+state),
		)
	}

	query := catalogChannelSQL
	for part, where := range map[string]Sqlizer{
		"{{$filter.thread}}": threadExpr,
		"{{$filter.member}}": memberExpr,
		"{{$filter.invite}}": inviteExpr,
	} {
		expr, _, err := where.ToSql()
		if err != nil {
			return nil, err
		}
		query = strings.ReplaceAll(
			query, part, " WHERE "+CompactSQL(expr),
		)
	}

	ctx.With(CTE{
		Name: "channel",
		Expr: sq.Expr(query),
	})

	return ctx, nil
}

func selectChatMemberQuery(req *app.SearchOptions) (ctx *SELECT, plan dataFetch[*api.Chat], err error) {

	queryArgs := queryChannelArgs{
		DC: req.Authorization.Creds.Dc,
	}

	for param, value := range req.Filter {
		switch param {
		case "thread.id": //, "chat.id":
			{
				switch data := value.(type) {
				case []string:
					queryArgs.ThreadID = data
				case string:
					if data == "" {
						break // switch
					}
					queryArgs.ThreadID = []string{data}
				default:
					return nil, nil, errors.BadRequest(
						"catalog.chat.thread.id.error",
						"chat( thread: [id!] ) convert %T value %[1]v into id",
						value,
					)
				}
			}
		case "peer":
			{
				switch data := value.(type) {
				case *api.Peer:
					queryArgs.Peer = data
				default:
					return nil, nil, errors.BadRequest(
						"catalog.chat.peer.input",
						"chat( peer: %[1]v ) convert %[1]T into *Peer",
						value,
					)
				}
			}
		case "via":
			{
				switch data := value.(type) {
				case *api.Peer:
					queryArgs.Peer = data
				default:
					return nil, nil, errors.BadRequest(
						"catalog.chat.via.input",
						"chat( via: %[1]v ) convert %[1]T into *Peer",
						value,
					)
				}
			}
		}
	}
	// WITH channel AS (..SELECT..)
	ctx, err = selectChannelQuery(queryArgs)
	if err != nil {
		return nil, nil, err
	}

	const (
		left = "c" // alias
	)
	ctx.Query = postgres.PGSQL.
		Select(
			// core:id
			ident(left, "id"),
		).
		From(
			"channel "+left,
		).
		OrderBy(
			// "coalesce("+ident(left, "join")+", ("+ident(left, "invite")+").date) DESC",
			ident(left, "thread_id"), // ASC
			ident(left, "leg"),       // ASC
		)

	plan = dataFetch[*api.Chat]{
		// core:id
		func(node *api.Chat) any {
			return DecodeText(func(src []byte) error {
				var id pgtype.UUID
				err := id.DecodeText(nil, src)
				if err == nil && id.Status == pgtype.Present {
					node.Id = hex.EncodeToString(id.Bytes[:])
				}
				return err
			})
		},
	}

	// Arguments
	const (
		aliasVia         = "via" // chat.bot
		aliasPeerBot     = "b"   // flow.acr_routing_scheme
		aliasPeerUser    = "u"   // directory.wbt_auth
		aliasPeerContact = "x"   // chat.client
	)

	var (
		// date pgtype.Timestamptz
		join = map[string]Sqlizer{}
		// LEFT JOIN chat.bot AS via
		joinGate = func() (as string, ok bool) {
			as = aliasVia // RIGHT
			if _, ok := join[as]; ok {
				return as, false // already done
			}
			expr := fmt.Sprintf(
				"LEFT JOIN chat.bot %s ON %[1]s.id = %s.via",
				as, left,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return as, true
		}
		// LEFT JOIN flow.acr_routing_scheme AS b
		joinPeerBot = func() (as string, ok bool) {
			as = aliasPeerBot // RIGHT
			if _, ok := join[as]; ok {
				return as, false // already done
			}
			expr := fmt.Sprintf(
				"LEFT JOIN flow.acr_routing_scheme %[1]s ON %[2]s.type = 'bot' AND %[1]s.id = %[2]s.user_id",
				as, left,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return as, true
		}
		// LEFT JOIN directory.wbt_auth AS u
		joinPeerUser = func() (as string, ok bool) {
			as = aliasPeerUser // RIGHT
			if _, ok := join[as]; ok {
				return as, false // already done
			}
			expr := fmt.Sprintf(
				"LEFT JOIN directory.wbt_auth %[1]s ON %[2]s.type = 'user' AND %[1]s.id = %[2]s.user_id",
				as, left,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return as, true
		}
		// LEFT JOIN chat.client AS x
		joinPeerContact = func() (as string, ok bool) {
			as = aliasPeerContact // RIGHT
			if _, ok := join[as]; ok {
				return as, false // already done
			}
			expr := fmt.Sprintf(
				"LEFT JOIN chat.client %[1]s ON %[2]s.type != ALL('{bot,user}') AND %[1]s.id = %[2]s.user_id",
				as, left,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join[as] = sq.Expr(expr)
			return as, true
		}
		// ----------------------------
		cols = map[string]Sqlizer{}
		// (gate) via
		columnVia = func() bool {
			const alias = "via"
			if _, ok := cols[alias]; ok {
				return false // duplicate; ignore
			}
			right, _ := joinGate() // MUST: ensure
			expr := fmt.Sprintf(
				"LEFT JOIN LATERAL (SELECT %[1]s.via id, coalesce(%[2]s.provider,'unknown') \"type\", coalesce(%[2]s.name,'[deleted]') \"name\"	WHERE %[1]s.via NOTNULL) gate ON true",
				left, right,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join["gate"] = sq.Expr(expr)

			expr = "(gate)" + alias
			ctx.Query = ctx.Query.Column(expr)
			cols[alias] = sq.Expr(expr)

			plan = append(plan, func(node *api.Chat) any {
				return fetchPeerRow(&node.Via)
			})

			return true
		}
		columnPeer = func() bool {
			const alias = "peer"
			if _, ok := cols[alias]; ok {
				return false // duplicate; ignore
			}
			// MUST: ensure
			_, _ = joinPeerBot()
			_, _ = joinPeerUser()
			_, _ = joinPeerContact()
			expr := CompactSQL(fmt.Sprintf(`LEFT JOIN LATERAL
(
	SELECT
--	%[1]s.user_id id
		coalesce(
	--- external:id ---
		%[4]s.external_id
	--- internal:id ---
	, %[1]s.user_id::::text
	) id
	, %[1]s.type
	, coalesce(
	--- flow:scheme ---
		%[2]s.name::::text
	--- user:agent ---
	, %[3]s.name
	, %[3]s.auth::::text
	--- contact:ext ---
	, %[4]s.name
	--- unknown ---
	, '[deleted]'
	) "name"
) peer ON true`,
				left,
				aliasPeerBot,
				aliasPeerUser,
				aliasPeerContact,
			))
			ctx.Query = ctx.Query.JoinClause(expr)
			join["peer"] = sq.Expr(expr)

			expr = "(peer)" // + alias
			ctx.Query = ctx.Query.Column(expr)
			cols[alias] = sq.Expr(expr)

			plan = append(plan, func(node *api.Chat) any {
				return fetchPeerRow(&node.Peer)
			})

			return true
		}
		columnInvite = func() bool {
			const alias = "invite"
			if _, ok := cols[alias]; ok {
				return false // duplicate; ignore
			}

			expr := ident(left, "invite") // + alias
			ctx.Query = ctx.Query.Column(expr)
			cols[alias] = sq.Expr(expr)

			plan = append(plan, func(node *api.Chat) any {
				return fetchInvitedRow(&node.Invite)
			})

			return true
		}
	)

	for _, field := range req.Fields {
		switch field {

		case "id": // core(!)
		// ---------- [PSEUDO] ---------- //
		case "leg":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "leg"), // chat_id
				)
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						// TODO: node.Leg = '@' + int(leg) // A..B..C..D..
						return nil
					})
				})
			}
		case "chat_id":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "thread_id"), // chat_id
				)
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						// TODO node.Chat.Id = UUID(src)
						return nil // dbx.ScanTimestamp(&node.Left)
					})
				})
			}
		// ------------------------------ //
		case "via":
			columnVia()
		case "peer":
			columnPeer()
		case "left":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "left"),
				)
				plan = append(plan, func(node *api.Chat) any {
					return postgres.Epochtime{Value: &node.Left}
				})
			}
		case "join":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "join"),
				)
				plan = append(plan, func(node *api.Chat) any {
					return postgres.Epochtime{Value: &node.Join}
				})
			}
		case "invite":
			columnInvite()
		case "context":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "context"),
				)
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						return dbx.ScanJSONBytes(&node.Context)(src)
					})
				})
			}
		default:
			err = errors.BadRequest(
				"members.query.fields.input",
				"members{ %s }; input: no such field",
				field,
			)
			return // ctx, plan, err
		}
	}

	for _, field := range req.Order {
		switch field {
		case "id":
		default:
			err = errors.BadRequest(
				"members.query.sort.input",
				"members( sort: [%s] ) input: no field support",
				field,
			)
			return // ctx, plan, err
		}
	}

	// [OFFSET|LIMIT]: paging
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			ctx.Query = ctx.Query.Offset((uint64)((page - 1) * size))
		}
		// LIMIT (size+1) -- to indicate whether there are more result entries
		ctx.Query = ctx.Query.Limit((uint64)(size + 1))
	}

	return // ctx, plan, nil
}

var (
	catalogChannelSQL = CompactSQL(`SELECT
  row_number() over
  (
    partition by
      coalesce(m.thread_id) -- c.chat_id -- chat_id
    order by
      coalesce(m.join, (m.invite).date) -- ASC -- coalesce(c.join, c.req) -- OLDest..to..NEWest; -- join
  )
  leg -- index within unique(thread)
, *
FROM
(
SELECT
------- identity ------
	coalesce(c.domain_id, r.domain_id) dc
, coalesce(c.id, r.id) id -- [FROM]
, coalesce(c.conversation_id, r.conversation_id) thread_id -- [TO]
--------- from --------
, (case when not c.internal then c.type else 'user' end) "type"
--, coalesce(c.internal, true) internal -- NULL means chat.invite !
, coalesce(c.user_id, r.user_id) user_id -- user.oid
---------- via --------
, c."connection"::::int8 via -- gate.id
-------- request ------
, (invite) -- .*
--  , r.inviter_channel_id from_id -- invited from.id
--  , r.created_at req -- requested
-------- timing -------
, c.created_at join -- accepted
-- , coalesce(c.closed_at, r.closed_at) left -- closed
, (case when c.id isnull then r.closed_at else c.closed_at end) left -- closed
------- context -------
, c.host
, coalesce(c.props, r.props) context
from
-- chat.channel c
(
	select
		c.*
	from
		chat.channel c
	----- filter -----
	{{$filter.member}}
	------------------
) c
full join
(
	select
		e.*
	from
		chat.invite e
	----- filter -----
	{{$filter.invite}}
	------------------
)
r on r.id = c.id -- using(id)
left join lateral
(
	select
		r.created_at date
	, r.inviter_channel_id "from"
	where
		r.id notnull
)
invite on true
--  where
--  --c.id isnull -- invited -but- ignored
--    (case when c.id isnull then r.closed_at else c.closed_at end) isnull -- left ?
UNION ALL --------------------------------
select
	c.domain_id dc
, c.id
, c.id thread_id
, 'bot' "type"
--, true internal
, (c.props->>'flow')::::int8 user_id
, NULL::::int8 via
, NULL::::record invite
, c.created_at join
, c.closed_at left
, h.node_id host
, c.props context
from
	chat.conversation c 
left join
	chat.conversation_node h
	on h.conversation_id = c.id
----- filter -----
{{$filter.thread}}
------------------
)
m -- member
`,
	)

	catalogParticipantSQL = CompactSQL(`SELECT
  row_number() over
  (
    partition by
      coalesce(m.thread_id) -- c.chat_id -- chat_id
    order by
      coalesce(m.join, (m.invite).date) -- ASC -- coalesce(c.join, c.req) -- OLDest..to..NEWest; -- join
  )
  leg -- index within unique(thread)
, *
FROM
(
SELECT
------- identity ------
	coalesce(c.domain_id, r.domain_id) dc
, coalesce(c.id, r.id) id -- [FROM]
, coalesce(c.conversation_id, r.conversation_id) thread_id -- [TO]
--------- from --------
, (case when not c.internal then c.type else 'user' end) "type"
--, coalesce(c.internal, true) internal -- NULL means chat.invite !
, coalesce(c.user_id, r.user_id) user_id -- user.oid
---------- via --------
, c."connection"::::int8 via -- gate.id
-------- request ------
, (invite) -- .*
--  , r.inviter_channel_id from_id -- invited from.id
--  , r.created_at req -- requested
-------- timing -------
, c.created_at join -- accepted
-- , coalesce(c.closed_at, r.closed_at) left -- closed
, (case when c.id isnull then r.closed_at else c.closed_at end) left -- closed
------- context -------
, c.host
, coalesce(c.props, r.props) context
from
	chat.channel c
----- filter -----
join
  query q
  on c.conversation_id = q.id
------------------
full join
(
	select
		e.*
	from
		chat.invite e
	--- filter ---
  join
    query q
    on e.conversation_id = q.id
)
r on r.id = c.id -- using(id)
left join lateral
(
	select
		r.created_at date
	, r.inviter_channel_id "from"
	where
		r.id notnull
)
invite on true
--  where
--  --c.id isnull -- invited -but- ignored
--    (case when c.id isnull then r.closed_at else c.closed_at end) isnull -- left ?
UNION ALL --------------------------------
select
	c.domain_id dc
, c.id
, c.id thread_id
, 'bot' "type"
--, true internal
, (c.props->>'flow')::::int8 user_id
, NULL::::int8 via
, NULL::::record invite
, c.created_at join
, c.closed_at left
, h.node_id host
, c.props context
from
	chat.conversation c 
----- filter -----
join
	query q
	on c.id = q.id
------------------
left join
	chat.conversation_node h
	on h.conversation_id = c.id
)
m -- member
`,
	)
)
