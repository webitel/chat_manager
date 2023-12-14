package sqlxrepo

import (
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	api "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	dbx "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/chat_manager/store/postgres"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// searchChatArgs query arguments
type searchChatArgs struct {
	// REQUIRED. Mandatory(!)
	DC int64
	// Search term: TODO
	Q string
	// // Type of the channel peer
	// Type []string
	// VIA text gateway transport.
	Via *api.Peer
	// Peer *-like participant(s).
	Peer *api.Peer
	// Self filter dialogs in common with current user.
	Self int64
	// Date timerange within ...
	Date *api.Timerange
	// Online participants.
	// <nil> -- any; whatevent
	// <true> -- NOT disconnected; ( left: 0 )
	// <false> -- IS disconnected; ( left: + )
	Online *bool
	// Joined participants.
	// <nil> -- any; whatevent
	// <true> -- IS|WAS connected; ( join: + )
	// <false> -- NEVER connected; ( join: 0 )
	Joined *bool

	// Chat (thread|member) IDs
	// Combined
	ID []string
	// Conversation (thread) IDs
	ThreadID []string
	// Participants (member) IDs
	MemberID []string
	// Include chat dialogs ONLY whose
	// member channel(s) contain a specified SET of variables
	Group map[string]string
}

// searchChatRequest decode input req.Filter into supported query arguments
func searchChatRequest(req *app.SearchOptions) (args searchChatArgs, err error) {

	args.Q = req.Term
	args.DC = req.Authorization.Creds.Dc
	// helper func
	inputIDs := func(param string, value *[]string, input any) error {
		switch data := input.(type) {
		case []string:
			*(value) = data
		case string:
			if data == "" {
				break
			}
			*(value) = []string{data}
		default:
			err = errors.BadRequest(
				"chat.query.%[1]s.input",
				"chat( %[1]s: %[2]v ) convert %[2]T into [id!]",
				param, input,
			)
			return err
		}
		return nil
	}
	// input: decode
	for param, input := range req.Filter {
		switch param {
		case "id":
			{
				err = inputIDs(
					param, &args.ID, input,
				)
				if err != nil {
					return // err
				}
			}
		case "via":
			{
				switch data := input.(type) {
				case *api.Peer:
					args.Via = data
				default:
					err = errors.BadRequest(
						"chat.query.via.input",
						"chat( via: %[1]v ) convert %[1]T into *Peer",
						input,
					)
					return // err
				}
			}
		case "date":
			{
				switch data := input.(type) {
				case *api.Timerange:
					args.Date = data
				default:
					err = errors.BadRequest(
						"chat.query.date.input",
						"chat( date: %v ) convert %[1]T into *Timerange",
						input,
					)
					return // err
				}
			}
		case "peer":
			{
				switch data := input.(type) {
				case *api.Peer:
					args.Peer = data
				default:
					err = errors.BadRequest(
						"chat.query.peer.input",
						"chat( peer: %[1]v ) convert %[1]T into *Peer",
						input,
					)
					return // err
				}
			}
		case "self":
			args.Self = req.Creds.UserId
		case "group":
			switch data := input.(type) {
			case map[string]string:
				if len(data) > 0 {
					delete(data, "")
				}
				if len(data) > 0 {
					args.Group = data
				}
			default:
				err = errors.BadRequest(
					"chat.query.group.input",
					"chat( group: %[1]v ) convert %[1]T into variables",
					input,
				)
				return // err
			}
		case "online":
			{
				switch data := input.(type) {
				case *wrapperspb.BoolValue:
					{
						if data == nil {
							break // omitted
						}
						is := data.GetValue()
						args.Online = &is
					}
				case *bool:
					args.Online = data
				case bool:
					args.Online = &data
				default:
					err = errors.BadRequest(
						"chat.query.online.input",
						"chat( online: %v ) convert %[1]T into bool",
						input,
					)
					return // err
				}
			}
		case "joined":
			{
				switch data := input.(type) {
				case *wrapperspb.BoolValue:
					{
						if data == nil {
							break // omitted
						}
						is := data.GetValue()
						args.Joined = &is
					}
				case *bool:
					args.Joined = data
				case bool:
					args.Joined = &data
				default:
					err = errors.BadRequest(
						"chat.query.joined.input",
						"chat( joined: %v ) convert %[1]T into bool",
						input,
					)
					return // err
				}
			}
		// ID: extra granular ...
		case "thread.id": //, "chat.id":
			{
				err = inputIDs(
					param, &args.ThreadID, input,
				)
				if err != nil {
					return // err
				}
			}
		case "member.id":
			{
				err = inputIDs(
					param, &args.MemberID, input,
				)
				if err != nil {
					return // err
				}
			}
		default:
			// ERR: no such argument
		}
	}

	return // args, nil
}

func searchChatDialogsQuery(req *app.SearchOptions) (ctx *SELECT, plan dataFetch[*api.Dialog], err error) {

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
	if authN.Native != nil && primaryDc <= 0 {
		// Native service node client Authenticated !
		// Allow search in .. ANY domain !
		var e, n = 0, len(req.Fields)
		for ; e < n && req.Fields[e] != "dc"; e++ {
			// lookup: requested ?
		}
		if e == n {
			req.Fields = append(req.Fields, "dc")
		}
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
	// chat( id: [id!] ); NOTE: support lookup dialog by it's member ID
	// So, in such way we resolve thread.id(s) needs to be fetched !
	// if len(args.ID) > 0 {
	memberQ.member.query = memberQ.member.query.Columns(
		ident(memberQ.member.from, "id"), // FULL JOIN USING(id)
		ident(memberQ.member.from, "conversation_id"),
	)
	memberQ.invite.query = memberQ.invite.query.Columns(
		ident(memberQ.invite.from, "id"), // FULL JOIN USING(id)
		ident(memberQ.invite.from, "conversation_id"),
	)
	const cteLookup = "search"
	ctx.With(CTE{
		Name: cteLookup,
		Expr: memberQ.
			Select("thread_id").
			GroupBy("1"), // thread_id
		// GroupBy("coalesce(c.conversation_id, r.conversation_id)"), // thread_id
	})
	memberQ, err = selectChatMember(
		// WITHOUT filters from now on ..
		searchChatArgs{DC: args.DC}, ctx.Params,
	)
	if err != nil {
		return // nil, nil, err
	}
	// // INNER JOIN lookup
	// memberQ.member.query = memberQ.member.query.JoinClause(fmt.Sprintf(
	// 	"JOIN %s q ON %s.conversation_id = q.thread_id",
	// 	searchCTE, memberQ.member.from,
	// ))
	// // INNER JOIN lookup
	// memberQ.invite.query = memberQ.invite.query.JoinClause(fmt.Sprintf(
	// 	"JOIN %s q ON %s.conversation_id = q.thread_id",
	// 	searchCTE, memberQ.member.from,
	// ))
	// INNER JOIN lookup
	threadQ = threadQ.JoinClause(fmt.Sprintf(
		"JOIN %s q ON %s.id = q.thread_id",
		cteLookup, "c",
	))
	// }
	// [OFFSET|LIMIT]: paging
	if size := req.GetSize(); size > 0 {
		// OFFSET (page-1)*size -- omit same-sized previous page(s) from result
		if page := req.GetPage(); page > 1 {
			threadQ = threadQ.Offset(
				(uint64)((page - 1) * (size)),
			)
		}
		// LIMIT (size+1) -- to indicate whether there are more result entries
		threadQ = threadQ.Limit(
			(uint64)(size + 1),
		)
	}
	// [WITH] chat_bot AS
	const cteThread = "thread" // "chat_bot"
	ctx.With(CTE{
		Name: cteThread,
		Expr: threadQ,
	})

	// INNER JOIN lookup
	memberQ.member.query = memberQ.member.query.Columns(
		ident(memberQ.member.from, "*"),
	)
	memberQ.member.query = memberQ.member.query.JoinClause(fmt.Sprintf(
		"JOIN %s q ON %s.conversation_id = q.thread_id",
		cteThread, memberQ.member.from,
	))
	// INNER JOIN lookup
	memberQ.invite.query = memberQ.invite.query.Columns(
		ident(memberQ.member.from, "*"),
	)
	memberQ.invite.query = memberQ.invite.query.JoinClause(fmt.Sprintf(
		"JOIN %s q ON %s.conversation_id = q.thread_id",
		cteThread, memberQ.member.from,
	))

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
						"title", // +
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
		"!dc",
		"leg",
		"title",
		"chat_id", // ORDER BY, JOIN; thread_id
		"via",
		"peer",
		"left",
		"join",
		"invite",
		"!context", // '!' -- NO fetch ! for ( members: [node!] )
	}

	var chatPlan dataFetch[*api.Chat]
	chatPlan, err = selectChatQuery(ctx, &chatQ) // (req)
	if err != nil {
		return // ctx, nil, err
	}

	const (
		// WITH chat AS (..SELECT..)
		chatView = "chat" // VIEW
		// alias
		left = "c"
	)

	ctx.With(CTE{
		Name: chatView,
		Expr: ctx.Query,
	})

	ctx.Query = postgres.PGSQL.
		Select(
			// core:id
			ident(left, "thread_id"), // AS id
		).
		From(
			chatView + " " + left,
		).
		Where(
			// Leg[A]. Originator(!)
			ident(left, "leg") + " = 1",
		)
	// core:id
	plan = dataFetch[*api.Dialog]{
		func(node *api.Dialog) any {
			return ScanFunc(func(src interface{}) error {
				var id pgtype.UUID
				err := id.Scan(src)
				if err == nil && id.Status == pgtype.Present {
					node.Id = hex.EncodeToString(id.Bytes[:])
				}
				return err
			})
		},
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
	// Arguments
	var (
		// temporary
		// date pgtype.Timestamptz
		messageAlias   string
		joinTopMessage = func() string {
			alias := messageAlias
			if alias != "" {
				return alias
			}
			// once
			alias = "top"
			messageAlias = alias
			ctx.Query = ctx.Query.JoinClause(CompactSQL(
				`LEFT JOIN LATERAL
				(
					SELECT
						m.id
					, m.created_at "date"
					, coalesce(m.channel_id, m.conversation_id) "from"
					, m.text
					, (file)
					FROM
						chat.message m
					LEFT JOIN LATERAL
					(
						SELECT
							m.file_id
						, m.file_size
						, m.file_type
						, m.file_name
						WHERE
							m.file_id NOTNULL
					) file ON true
					WHERE
						m.conversation_id = c.thread_id
					ORDER BY
						m.conversation_id
					, m.id DESC -- NEWest..to..OLDest
					LIMIT
						1 -- TOP(1)
				)
				top ON true`,
			))
			return alias
		}
	)

	for _, field := range req.Fields {
		switch field {
		// ------ [IDENT] ------- //
		case "id": // core(!)
		case "dc":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "dc"), // ":pdc",
				)
				plan = append(plan, func(node *api.Dialog) any {
					return postgres.Int8{Value: &node.Dc}
				})
			}
		// ------ [TIMING] ------ //
		case "date":
			{
				// TODO: ensure JOIN chat.message AS top
				top := joinTopMessage()
				ctx.Query = ctx.Query.Column(
					fmt.Sprintf("coalesce(%s.left, %s.date)", left, top),
				)
				plan = append(plan, func(node *api.Dialog) any {
					return postgres.Epochtime{
						Precision: app.TimePrecision,
						Value:     &node.Date,
					}
				})
			}
		case "closed":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "left"), // closed
				)
				plan = append(plan, func(node *api.Dialog) any {
					return postgres.Epochtime{
						Precision: app.TimePrecision,
						Value:     &node.Closed,
					}
				})
			}
		case "started":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "join"), // started
				)
				plan = append(plan, func(node *api.Dialog) any {
					return postgres.Epochtime{
						Precision: app.TimePrecision,
						Value:     &node.Started,
					}
				})
			}
		// ------- [FROM] ------- //
		case "via":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "via"),
				)
				plan = append(plan, func(node *api.Dialog) any {
					return fetchPeerRow(&node.Via)
				})
			}
		case "from":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "peer"), // "from"
				)
				plan = append(plan, func(node *api.Dialog) any {
					return fetchPeerRow(&node.From)
				})
			}
		case "title":
			{
				ctx.Query = ctx.Query.Column(
					ident(left, "title"),
				)
				plan = append(plan, func(node *api.Dialog) any {
					return postgres.Text{Value: &node.Title}
				})
			}
		case "context":
			{
				{
					ctx.Query = ctx.Query.Column(
						ident(left, "context"),
					)
					plan = append(plan, func(node *api.Dialog) any {
						return dbx.ScanJSONBytes(&node.Context)
					})
				}
			}
		case "message":
			{
				// TODO: ensure JOIN chat.message AS top
				top := joinTopMessage()
				ctx.Query = ctx.Query.Column(
					"(" + top + ")", // ROW(message)
				)
				plan = append(plan, func(node *api.Dialog) any {
					return DecodeText(func(src []byte) error {

						res := node.Message
						node.Message = nil // NULL
						if len(src) == 0 {
							return nil // NULL
						}

						if res == nil {
							res = new(api.Message)
						}

						var (
							ok  bool // false
							row = []TextDecoder{
								// id
								DecodeText(func(src []byte) error {
									var mid pgtype.Int8
									err := mid.DecodeText(nil, src)
									if err != nil {
										return err
									}
									// if date.Status == pgtype.Present {
									res.Id = mid.Int
									// }
									ok = ok || mid.Int > 0
									return nil
								}),
								// date
								DecodeText(func(src []byte) error {
									var date pgtype.Timestamptz
									err := date.DecodeText(nil, src)
									if err != nil {
										return err
									}
									// if date.Status == pgtype.Present {
									res.Date = app.DateEpochtime(date.Time, app.TimePrecision)
									// }
									ok = ok || date.Status == pgtype.Present
									return nil
								}),
								// from
								DecodeText(func(src []byte) error {
									rel := res.Sender // Value
									res.Sender = nil  // NULL
									var from pgtype.UUID
									err := from.DecodeText(nil, src)
									if err != nil {
										return err
									}
									if from.Status == pgtype.Present {
										if rel == nil {
											rel = new(api.Chat)
										}
										*(rel) = api.Chat{
											Id: hex.EncodeToString(from.Bytes[:]),
										}
										res.Sender = rel
										ok = true
									}
									// ok = ok || from.Status == pgtype.Present
									return nil
								}),
								// text
								DecodeText(func(src []byte) error {
									var text pgtype.Text
									err := text.DecodeText(nil, src)
									if err != nil {
										return err
									}
									// if date.Status == pgtype.Present {
									res.Text = text.String
									// }
									ok = ok || res.Text != ""
									return nil
								}),
								// file
								fetchFileRow(&res.File).(DecodeText),
							}
							// row()::record
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
							node.Message = res
						}

						return nil
					})
				})
			}
		case "members":
			{
				ctx.Query = ctx.Query.JoinClause(CompactSQL(fmt.Sprintf(
					`LEFT JOIN LATERAL
					(
						SELECT ARRAY
						(
							SELECT -- distinct on (m.thread_id)
							(m) --m
							/*( m.id
							--, m.leg
							, m.via -- NULL
							, m.peer
							, m.left
							, m.join
							, m.invite
							--, m.context
							)*/
							FROM
								%[1]s m
							WHERE
								m.thread_id = c.thread_id
								AND m.join NOTNULL -- JOIN[ed] members ONLY !
							ORDER BY
							--m.thread_id,
							--m.leg
								m.left DESC NULLS FIRST -- FIRST active THAN kicked
						)
					)
					members(data) on true`,
					chatView,
				)))
				ctx.Query = ctx.Query.Column(
					"members.data", // participants
				)
				plan = append(plan, func(node *api.Dialog) any {
					return DecodeText(func(src []byte) error {
						page := node.Members // input: cache
						node.Members = nil   // NULL
						if len(src) == 0 {
							return nil // NULL
						}
						rows, err := pgtype.ParseUntypedTextArray(string(src))
						size := len(rows.Elements)
						if err != nil || size == 0 {
							return err
						}
						var (
							item *api.Chat                    // node
							heap []api.Chat                   // mempage
							data = make([]*api.Chat, 0, size) // result
						)

						if n := size - len(page); 1 < n {
							heap = make([]api.Chat, n) // mempage; tidy
						}

						// DECODE
						// var r, c int // [r]ow, [c]olumn
						for r, elem := range rows.Elements {
							// // LIMIT
							// if 0 < limit && limit == len(data) {
							// 	list.Next = true
							// 	if list.Page < 1 {
							// 		list.Page = 1
							// 	}
							// 	break
							// }
							// RECORD
							item = nil // NEW
							if r < len(page) {
								// [INTO] given page records
								// [NOTE] order matters !
								item = page[r]
							} else if len(heap) > 0 {
								item = &heap[0]
								heap = heap[1:]
							}
							// ALLOC
							if item == nil {
								item = new(api.Chat)
							}
							// DECODE
							raw := pgtype.NewCompositeTextScanner(nil, []byte(elem))
							// [c] -- column
							for _, bind := range chatPlan {
								// if !raw.Next() { /// .ScanValue calls .Next(!)
								// 	break
								// }

								df := bind(item)
								if df == nil {
									// omit; pseudo calc
									continue
								}
								// raw.ScanValue(df)
								raw.ScanDecoder(df.(TextDecoder))
								err = raw.Err()
								if err != nil {
									return err
								}
							}
							data = append(data, item)
						}
						node.Members = data // contact(1):(N)contact_label
						return nil
					})
				})
			}

		default:
			err = errors.BadRequest(
				"chat.dialogs.fields.error",
				"dialogs{ %s } no such field",
				field,
			)
			return // ctx, plan, err
		}
	}

	ctx.Query = ctx.Query.OrderBy(
		fmt.Sprintf("(%[1]s.left NOTNULL)", left),
		fmt.Sprintf("coalesce(%[1]s.left, top.date) DESC", left), // date
	)
	// for _, field := range req.Order {
	// 	switch field {
	// 	case "id":
	// 	default:
	// 		err = errors.BadRequest(
	// 			"dialogs.query.sort.input",
	// 			"dialogs( sort: %s ) input: no field support",
	// 			field,
	// 		)
	// 		return // ctx, plan, err
	// 	}
	// }

	return // ctx, plan, nil
}

func fetchDialogRows(rows *sql.Rows, plan dataFetch[*api.Dialog], into *api.ChatDialogs, limit int) (err error) {
	var (
		node *api.Dialog
		heap []api.Dialog

		page = into.GetData() // input
		data []*api.Dialog    // output

		eval = make([]any, len(plan))
	)

	if 0 < limit {
		data = make([]*api.Dialog, 0, limit)
	}

	if n := limit - len(page); 1 < n {
		heap = make([]api.Dialog, n) // mempage; tidy
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
			node = new(api.Dialog)
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
