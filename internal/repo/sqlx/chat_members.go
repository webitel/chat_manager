package sqlxrepo

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	api "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	dbx "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/chat_manager/store/postgres"
)

type chatMemberQ struct {
	// columns []Sqlizer
	member struct {
		from     string // "e"
		query    sq.SelectBuilder
		joinGate string // bot
		joinUser string // directory.wbt_auth
		joinPeer string // chat.client
		alias    string // "c"
	}
	invite struct {
		from     string // "e"
		query    sq.SelectBuilder
		joinUser string // user
		alias    string // "r"
	}
	JOIN          // FROM member JOIN invite
	WHERE         Sqlizer
	joinInviteRow string // LEFT JOIN LATERAL (invite)
}

func (q *chatMemberQ) memberJoinGate() string {
	alias := q.member.joinGate
	if alias != "" {
		return alias
	}
	// once
	alias = "g"
	left := q.member.from
	q.member.joinGate = alias
	q.member.query = q.member.query.JoinClause(fmt.Sprintf(
		// INNER; MUST
		"JOIN chat.bot %[2]s ON %[2]s.id = (%[1]s.\"connection\"::::int8)",
		left, alias,
	))
	return alias
}

func (q *chatMemberQ) memberJoinUser() string {
	alias := q.member.joinUser
	if alias != "" {
		return alias
	}
	// once
	alias = "u"
	left := q.member.from
	q.member.joinUser = alias
	q.member.query = q.member.query.JoinClause(fmt.Sprintf(
		"LEFT JOIN directory.wbt_auth %[2]s ON %[1]s.internal AND %[2]s.id = %[1]s.user_id",
		left, alias,
	))
	return alias
}

func (q *chatMemberQ) memberJoinPeer() string {
	alias := q.member.joinPeer
	if alias != "" {
		return alias
	}
	// once
	alias = "x"
	left := q.member.from
	q.member.joinPeer = alias
	q.member.query = q.member.query.JoinClause(fmt.Sprintf(
		"LEFT JOIN chat.client %[2]s ON NOT %[1]s.internal AND %[2]s.id = %[1]s.user_id",
		left, alias,
	))
	return alias
}

func (q *chatMemberQ) inviteJoinUser() string {
	alias := q.invite.joinUser
	if alias != "" {
		return alias
	}
	// once
	alias = "u"
	left := q.invite.from
	q.invite.joinUser = alias
	q.invite.query = q.invite.query.JoinClause(fmt.Sprintf(
		// INNER; MUST
		"JOIN directory.wbt_auth %[2]s ON %[2]s.id = %[1]s.user_id",
		left, alias,
	))
	return alias
}

func (q *chatMemberQ) inviteJoinRow() string {
	alias := q.joinInviteRow
	if alias != "" {
		return alias
	}
	// once
	alias = "invite"
	q.joinInviteRow = alias
	// see: q.Select() method
	return alias
}

func (q *chatMemberQ) Select(columns ...string) sq.SelectBuilder {

	var (
		left  = q.member.alias // "c" -- chat
		right = q.invite.alias // "r" -- request
	)
	// FROM (SELECT chat.channel) c
	cte := postgres.PGSQL.Select().FromSelect(
		q.member.query, left,
	)
	// FULL JOIN (SELECT chat.invite) r
	cte = cte.JoinClause(&JOIN{
		Kind:  q.JOIN.Kind, // "FULL JOIN",
		Table: q.invite.query.Prefix("(").Suffix(")"),
		Alias: right,
		Pred:  sq.Expr(fmt.Sprintf("%[2]s.id = %[1]s.id", left, right)),
	})

	// SELECT column(s)..
	for _, column := range columns {
		switch column {

		// // :pdc
		// case "dc": // int8
		// 	cte = cte.Column("coalesce(c.domain_id, r.domain_id) dc")

		case "id": // uuid
			cte = cte.Column("coalesce(c.id, r.id) id")
		case "dc": // int8
			cte = cte.Column("coalesce(c.domain_id, r.domain_id) dc")
		case "via": // int8
			cte = cte.Column("c.\"connection\"::::int8 via")
		case "type": // (peer).type::text
			cte = cte.Column("(case when NOT c.internal then c.type else 'user' end) \"type\"") // chat.invite IS 'user' !
		case "user_id": // (peer).oid::int8
			cte = cte.Column("coalesce(c.user_id, r.user_id) user_id")
		case "title":
			cte = cte.Column("coalesce(c.name, r.title) title")

		case "thread_id": // uuid
			cte = cte.Column("coalesce(c.conversation_id, r.conversation_id) thread_id")

		case "left": // timestamptz
			cte = cte.Column("(case when c.id isnull then r.closed_at else c.closed_at end) left")
		case "join": // timestamptz
			cte = cte.Column("c.created_at join")
		case "invite": // ROW(date::timestamptx,chat_id::uuid)
			_ = q.inviteJoinRow() // LEFT JOIN LATERAL (invite) ON true
			cte = cte.Column("(invite)")

		case "host":
			cte = cte.Column("c.host")
		case "context":
			cte = cte.Column("coalesce(c.props, r.props) context")
		default:
			cte = cte.Column(column)
		}
	}

	// LEFT JOIN LATERAL (invite)
	if alias := q.joinInviteRow; alias != "" {
		cte = cte.JoinClause(&JOIN{
			Kind: "LEFT JOIN LATERAL",
			Table: sq.Expr(CompactSQL(fmt.Sprintf(`(
					SELECT
						%[1]s.created_at date,
						%[1]s.inviter_channel_id chat_id
					WHERE
						%[1]s.id NOTNULL
				)`, q.invite.alias,
			))),
			Alias: alias,
			Pred:  sq.Expr("true"),
		})
	}

	return cte
}

func selectChatQuery(ctx *SELECT, req *app.SearchOptions) (plan dataFetch[*api.Chat], err error) {

	const (
		left  = "c"       // alias
		table = "channel" // CTE; View
	)

	ctx.Query = postgres.PGSQL.
		Select(
			// core:id
			ident(left, "id"),
		).
		From(
			table+" "+left,
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
		joinGateVia = func() (as string, ok bool) {
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
			right, _ := joinGateVia() // MUST: ensure
			expr := fmt.Sprintf(
				// NULLIF(via, 0) -- invalidate VIA('0') portal's gateway.id
				"LEFT JOIN LATERAL (SELECT %[1]s.via id, coalesce(%[2]s.provider,'unknown') \"type\", coalesce(%[2]s.name,'[deleted]') \"name\"	WHERE NULLIF(%[1]s.via, 0) NOTNULL) gate ON true",
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
		--%[1]s.user_id id
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
		columnQueue = func() bool {
			as := "q"
			column := "queue"
			if _, ok := cols[column]; ok {
				return false // duplicate; ignore
			}
			expr := fmt.Sprintf(
				`LEFT JOIN LATERAL (SELECT %[1]s.id, %[1]s.strategy, %[1]s.name
							FROM call_center.cc_member_attempt_history m
								LEFT JOIN call_center.cc_queue %[1]s ON m.queue_id = %[1]s.id
							WHERE m.member_call_id = %[2]s.thread_id::::varchar
							ORDER BY %[2]s."join" desc
							LIMIT 1) %[3]s ON true`,
				as, left, column,
			)
			ctx.Query = ctx.Query.JoinClause(expr)
			join[column] = sq.Expr(expr)

			expr = "(queue)" // + alias
			ctx.Query = ctx.Query.Column(expr)
			cols[column] = sq.Expr(expr)

			plan = append(plan, func(node *api.Chat) any {
				return fetchQueueRow(&node.Queue)
			})

			return true
		}
	)

	var no bool // [n]o[o]utput; USED for some column(s) to pass thru value without fetch
	for _, field := range req.Fields {
		if no = (field[0] == '!'); no {
			field = field[1:]
		}
		switch field {

		case "id": // core(!)
		// ---------- [PSEUDO] ---------- //
		case "dc":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "dc"),
				)
				fetch := !no
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						if !fetch {
							return nil
						}
						return postgres.Int8{Value: &node.Dc}.DecodeText(nil, src)
					})
				})
			}
		case "leg":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "leg"),
				)
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						// TODO: node.Leg = '@' + int(leg) // A..B..C..D..
						return nil
					})
				})
			}
		case "title":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "title"),
				)
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						// NOTE: ignore; USED for dialogs ONLY !
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
				fetch := !no
				plan = append(plan, func(node *api.Chat) any {
					return DecodeText(func(src []byte) error {
						if !fetch {
							return nil
						}
						return dbx.ScanJSONBytes(&node.Context)(src)
					})
				})
			}
		case "queue":
			{
				columnQueue()
			}
		default:
			err = errors.BadRequest(
				"chat.query.fields.error",
				"chat{ %s } no such field",
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
				"chat.query.sort.error",
				"chat( sort: %s ) no support",
				field,
			)
			return // ctx, plan, err
		}
	}

	// // [OFFSET|LIMIT]: paging
	// if size := req.GetSize(); size > 0 {
	// 	// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
	// 	if page := req.GetPage(); page > 1 {
	// 		ctx.Query = ctx.Query.Offset((uint64)((page - 1) * size))
	// 	}
	// 	// LIMIT (size+1) -- to indicate whether there are more result entries
	// 	ctx.Query = ctx.Query.Limit((uint64)(size + 1))
	// }

	return // ctx, plan, nil
}

func selectChatMember(req searchChatArgs, params params) (cte chatMemberQ, err error) {

	// FROM

	const left = "e"
	cte.member.from = left
	cte.member.alias = "c" // chat
	cte.invite.from = left
	cte.invite.alias = "r" // request
	// ( SELECT FROM chat.channel AS e )
	cte.member.query = postgres.PGSQL.
		Select().From("chat.channel " + cte.member.from)
	// ( SELECT FROM chat.invite AS e )
	cte.invite.query = postgres.PGSQL.
		Select().From("chat.invite " + cte.invite.from)

	cte.JOIN.Kind = "FULL JOIN"
	cte.JOIN.Pred = sq.Expr(
		ident(cte.member.alias, "id") + " = " +
			ident(cte.invite.alias, "id"),
	)

	// WHERE

	var (
		where = sq.And{
			// sq.Expr(ident(left, "domain_id") + " = :pdc"),
		}
		// id = make([]pgtype.UUID, 0, len(req.MemberID) + len(req.ThreadID))
	)
	if _, has := params["pdc"]; has {
		where = append(where, // AND
			sq.Expr(ident(left, "domain_id")+" = :pdc"),
		)
	}
	// chat( id: [id!] )
	if n := len(req.ID); n > 0 {
		var id pgtype.UUIDArray
		err = id.Set(req.ID)
		if err != nil {
			return // nil, err
		}
		compare, value := " = ANY(:chat_id)", any(&id)
		if n > 1 {
			compare, value = " = :chat_id", &id.Elements[0]
		}
		params.set("chat_id", value)
		where = append(where, sq.Or{ // OR
			sq.Expr(ident(left, "id") + compare),              // member_id
			sq.Expr(ident(left, "conversation_id") + compare), // thread_id
		})
	}
	pred := Sqlizer(where)
	if len(where) == 1 {
		pred = where[0]
	}
	cte.member.query =
		cte.member.query.Where(
			pred,
		)
	cte.invite.query =
		cte.invite.query.Where(
			pred,
		)
	// chat( via: peer )
	if req.Via != nil {
		// via( presence: bool )
		var (
			via = api.Peer{
				Id:   req.Via.Id,
				Type: req.Via.Type,
				Name: req.Via.Name,
			}
			presence bool // false
			joinGate bool // false
		)
		for e, sv := range []*string{
			&via.Id, &via.Type, &via.Name,
		} {
			if *(sv) == "*" {
				*(sv) = ""
				presence = true
				continue
			}
			if e > 0 { // NOT (id)
				joinGate = joinGate || *(sv) != ""
			}
		}
		// via( id: int )
		if via.Id != "" {
			oid, _ := strconv.ParseInt(
				via.Id, 10, 64,
			)
			if oid < 1 {
				err = errors.BadRequest(
					"chat.query.via.id.input",
					"chat( via: %s ) invalid id",
					via.Id,
				)
				return // cte, err
			}
			params.set("via.id", oid)
			cte.member.query = cte.member.query.Where(
				left + ".\"connection\"::::int8 = :via.id",
			)
			cte.JOIN.Kind = "LEFT JOIN"
		}
		// via( type|name )
		if joinGate {
			gate := cte.memberJoinGate()
			if via.Type != "" {
				params.set("via.type", app.Substring(via.Type))
				cte.member.query = cte.member.query.Where(
					gate + ".provider::::text ILIKE :via.type COLLATE \"default\"",
				)
			}
			if via.Name != "" {
				params.set("via.name", app.Substring(via.Name))
				cte.member.query = cte.member.query.Where(
					gate + ".name ILIKE :via.name COLLATE \"default\"",
				)
			}
		} else if presence {
			// via( * )
			cte.member.query = cte.member.query.Where(
				left + ".\"connection\" NOTNULL",
			)
		}
	}
	// chat( peer: peer )
	if req.Peer != nil {
		var (
			peer = api.Peer{
				Id:   req.Peer.Id,
				Type: req.Peer.Type,
				Name: req.Peer.Name,
			}
			presence bool // false
			joinPeer bool // false
		)
		for e, sv := range []*string{
			&peer.Id, &peer.Type, &peer.Name,
		} {
			if *(sv) == "*" {
				*(sv) = ""
				presence = true
				continue
			}
			if e > 0 { // NOT (id)
				joinPeer = joinPeer || *(sv) != ""
			}
		}
		// BY: peer( type: string )
		typeOf := strings.ToLower(req.Peer.Type)
		switch typeOf {
		case "bot": // chat.conversation
			// invalidate(!)
			cte.member.query = cte.member.query.Where(
				sq.Expr("false"),
			)
			cte.invite.query = cte.invite.query.Where(
				sq.Expr("false"),
			)
			// TODO: searchChatMemberFlowCTE(!)
		case "user": // chat.(channel|invite)
			{
				// user( id: int )
				if peer.Id != "" {
					oid, _ := strconv.ParseInt(
						peer.Id, 10, 64,
					)
					if oid < 1 {
						err = errors.BadRequest(
							"chat.query.user.id.input",
							"chat( user: %s ) invalid id",
							peer.Id,
						)
						return // cte, err
					}
					params.set("peer.id", oid)
					cte.member.query = cte.member.query.Where(fmt.Sprintf(
						"%[1]s.internal AND %[1]s.user_id = :peer.id",
						left,
					))
					cte.invite.query = cte.invite.query.Where(fmt.Sprintf(
						"%[1]s.user_id = :peer.id",
						left,
					))
				}
				if peer.Name != "" {
					right := cte.memberJoinUser()
					params.set("peer.name", app.Substring(peer.Name))
					cte.member.query = cte.member.query.Where(fmt.Sprintf(
						"coalesce(%[1]s.name, %[1]s.auth::::text) ILIKE :peer.name COLLATE \"default\"",
						right,
					))
				} else if presence {
					cte.member.query = cte.member.query.Where(
						sq.Expr(ident(left, "internal")),
					)
				}
			}
		default: // ONLY chat.channel
			// peer( type: string )
			if typeOf != "" {
				// FIXME: substrings ?
				params.set("peer.type", app.Substring(req.Peer.Type))
				cte.member.query = cte.member.query.Where(fmt.Sprintf(
					"(case when %[1]s.internal then 'user' else %[1]s.type end) ILIKE :peer.type COLLATE \"default\"",
					left,
				))
				cte.invite.query = cte.invite.query.Where(
					"'user' ILIKE :peer.type COLLATE \"default\"",
				)
			}
			// peer( id: string )
			if peer.Id != "" {
				// const (
				// 	internalAlias = "u"
				// 	externalAlias = "x"
				// )
				right := cte.memberJoinPeer()
				params.set("peer.id", peer.Id)
				cte.member.query = cte.member.query.Where(fmt.Sprintf(
					"coalesce(%s.external_id, %s.user_id::::text) LIKE :peer.id",
					right, left,
				))

				cte.invite.query = cte.invite.query.Where(fmt.Sprintf(
					"%s.user_id::::text LIKE :peer.id",
					left,
				))
			}
			// peer( name: string )
			if peer.Name != "" {
				internal := cte.memberJoinUser()
				external := cte.memberJoinPeer()
				params.set("peer.name", app.Substring(peer.Name))
				cte.member.query = cte.member.query.Where(fmt.Sprintf(
					"coalesce(%[1]s.name, %[2]s.name, %[2]s.auth::::text, 'deleted') ILIKE :peer.name COLLATE \"default\"",
					external, internal,
				))

				internal = cte.inviteJoinUser()
				cte.invite.query = cte.invite.query.Where(fmt.Sprintf(
					"coalesce(%[1]s.name, %[1]s.auth::::text, 'deleted') ILIKE :peer.name COLLATE \"default\"",
					internal,
				))
			} else if presence {
				// ALLWAYS !
			}
		}
	}
	// chat( date: timerange )
	if req.Date != nil {
		if 0 < req.Date.Since {
			var since pgtype.Timestamp
			_ = since.Set(app.EpochtimeDate(
				req.Date.Since, app.TimePrecision,
			).UTC())
			// ok = true // has query !
			params.set("since", &since)
			cte.member.query = cte.member.query.Where(left + ".created_at >= :since")
			cte.invite.query = cte.invite.query.Where(left + ".created_at >= :since")
		}
		if 0 < req.Date.Until {
			var until pgtype.Timestamp
			_ = until.Set(app.EpochtimeDate(
				req.Date.Until, app.TimePrecision,
			).UTC())
			// ok = true // has query !
			params.set("until", &until)
			cte.member.query = cte.member.query.Where(fmt.Sprintf(
				"coalesce(%[1]s.closed_at, %[1]s.created_at) < :until",
				left,
			))
			cte.invite.query = cte.invite.query.Where(fmt.Sprintf(
				"coalesce(%[1]s.closed_at, %[1]s.created_at) < :until",
				left,
			))
		}
	}
	// chat( self: bool )
	if req.Self > 0 {
		// Filter members of dialogs ONLY in common with current user
		params.set("self", req.Self)
		cte.member.query =
			cte.member.query.JoinClause(fmt.Sprintf(
				"JOIN %[2]s %[3]s ON %[3]s.internal AND %[3]s.user_id = :self AND %[3]s.conversation_id = %[1]s.conversation_id",
				left, "chat.channel", "self",
			))
		cte.invite.query =
			cte.invite.query.JoinClause(fmt.Sprintf(
				"JOIN %[2]s %[3]s ON %[3]s.user_id = :self AND %[3]s.conversation_id = %[1]s.conversation_id",
				left, "chat.invite", "self",
			))
	}
	// chat( group: {variables} )
	if len(req.Group) > 0 {
		group := pgtype.JSONB{
			Bytes: dbx.NullJSONBytes(req.Group),
		}
		// set( status: present )
		group.Set(group.Bytes)
		params.set("group", &group)
		cte.member.query = cte.member.query.Where(
			ident(left, "props") + "@>:group",
		)
		cte.invite.query = cte.invite.query.Where(
			ident(left, "props") + "@>:group",
		)
	}
	// chat( online: bool )
	if req.Online != nil {
		const (
			active = " ISNULL"
			closed = " NOTNULL"
		)
		is := active
		if !(*(req.Online)) {
			is = closed
		}
		cte.member.query = cte.member.query.Where(
			ident(left, "closed_at") + is,
		)
		cte.invite.query = cte.invite.query.Where(
			ident(left, "closed_at") + is,
		)
	}
	// chat( joined: bool )
	if req.Joined != nil {
		const (
			// chat.channel
			IS  = "LEFT JOIN"  // " NOTNULL"
			NOT = "RIGHT JOIN" // " ISNULL"
		)
		join := IS
		if !(*(req.Joined)) {
			// FROM chat.channel c
			// RIGHT JOIN chat.invite r
			// WHERE c.id ISNULL
			join = NOT
			cond := cte.WHERE
			cte.WHERE = sq.Expr(
				ident(cte.member.alias, "id") + " ISNULL",
			)
			if cond != nil {
				cte.WHERE = sq.And{
					cte.WHERE, cond,
				}
			}
		}
		cte.JOIN.Kind = join
	}

	return
}

func selectChatThread(req searchChatArgs, params params) (cte sq.SelectBuilder, err error) {

	cte = postgres.PGSQL.
		Select(
			// "c.domain_id dc",

			"c.id",
			"c.domain_id dc",
			"NULL::::int8 via",
			"'bot' \"type\"",
			"(c.props->>'flow')::::int8 user_id",
			"c.title",
			"c.id thread_id",

			"c.closed_at left",
			"c.created_at join",
			"NULL::::record invite",

			"h.node_id host",
			"c.props context",
		).
		From(
			"chat.conversation c",
		).
		JoinClause( // host
			"LEFT JOIN chat.conversation_node h ON h.conversation_id = c.id",
		).
		// Where(
		// 	"c.domain_id = :pdc",
		// ).
		OrderBy(
			"c.closed_at NOTNULL", // ONLINE FIRST
			"c.created_at DESC",   // NEWest..to..OLDest
		).
		Limit(
			64,
		)

	if _, has := params["pdc"]; has {
		cte = cte.Where("c.domain_id = :pdc")
	}

	// chat( id: [id!] )
	if vs := req.ThreadID; len(vs) > 0 {
		var id pgtype.UUIDArray
		err := id.Set(vs)
		if err != nil {
			return cte, err
		}
		expr, value := "ANY(:thread.id)", any(&id)
		if len(vs) == 1 {
			expr, value = ":thread.id", &id.Elements[0]
		}
		params.set("thread.id", value)
		cte = cte.Where("c.id = " + expr)
		cte = cte.Limit(uint64(len(vs)))
	}
	// chat( online: bool )
	if vs := req.Online; vs != nil {
		const (
			// closed_at
			active = " ISNULL"
			closed = " NOTNULL"
		)
		expr := closed
		if *(vs) {
			expr = active
		}
		cte = cte.Where("c.closed_at" + expr)
	}
	// chat( joined: bool )
	if vs := req.Joined; vs != nil {
		if !(*vs) { // NOT ?
			// NOTE: Allways JOINED !
			cte = cte.Where("false") // EXCLUDE
		}
	}

	return // cte, nil
}

func searchChatMembersQuery(req *app.SearchOptions) (ctx *SELECT, plan dataFetch[*api.Chat], err error) {

	var args searchChatArgs
	args, err = searchChatRequest(req)
	if err != nil {
		return // nil, nil, err
	}

	ctx = &SELECT{
		Params: params{
			// // "date": &pgtype.Timestamp{
			// // 	Time:   req.Localtime().UTC(),
			// // 	Status: pgtype.Present,
			// // },
			// // "user": req.Authorization.Creds.UserId,
			// "pdc": req.Authorization.Creds.Dc,
		},
	}

	var (
		authN     = &req.Authorization
		endUser   = authN.Creds
		primaryDc int64 // 0 ; invalid
	)
	if endUser != nil {
		primaryDc = endUser.Dc
	}
	if authN.Native != nil && primaryDc < 1 {
		// Native service node client Authenticated !
		// Allow search in .. ANY domain !
	} else {
		// mandatory: filter !
		ctx.Params["pdc"] = primaryDc
	}

	threadQ, re := selectChatThread(args, ctx.Params)
	if err = re; err != nil {
		return // nil, nil, err
	}

	memberQ, re := selectChatMember(args, ctx.Params)
	if err = re; err != nil {
		return // nil, nil, err
	}

	memberQ.member.query = memberQ.member.query.Columns(
		ident(memberQ.member.from, "*"),
	)
	// filter( thread.id: uuid )
	memberQ.member.query = memberQ.member.query.Where(fmt.Sprintf(
		"%s.conversation_id = :thread.id",
		memberQ.member.from,
	))
	memberQ.invite.query = memberQ.invite.query.Columns(
		ident(memberQ.member.from, "*"),
	)
	// filter( thread.id: uuid )
	memberQ.invite.query = memberQ.invite.query.Where(fmt.Sprintf(
		"%s.conversation_id = :thread.id",
		memberQ.invite.from,
	))

	const cteThread = "thread"
	ctx.With(CTE{
		Name: cteThread,
		Expr: threadQ,
	})
	ctx.With(CTE{
		Name: "channel",
		Expr: postgres.PGSQL.
			Select(
				`row_number() over(
				partition by
					m.thread_id
				order by
					coalesce(m.join, (m.invite).date)
			) leg`,
				"m.*", // m.*
			).
			FromSelect(
				memberQ.
					Select(
						"id",
						"dc",
						"via",
						"type",
						"user_id",
						"title",
						"thread_id",
						"left",
						"join",
						"invite",
						"host",
						"context",
					).
					Suffix(
						"UNION ALL"+
							" SELECT * FROM "+
							cteThread,
					), // .
				// SuffixExpr(
				// 	threadQ,
				// ),
				"m",
			),
	})

	chatQ := *(req) // shallowcopy
	chatQ.Order = nil
	chatQ.Fields = []string{
		"id",
		// "leg",
		// "chat_id",
		"via",
		"peer",
		"left",
		"join",
		"invite",
		"context",
	}

	// WITH channel AS ( SELECT FROM chatQ UNION threadQ )
	// var plan dataFetch[*api.Chat]
	plan, err = selectChatQuery(ctx, &chatQ) // (req)
	if err != nil {
		return // ctx, nil, err
	}

	return // ctx, plan, nil
}
