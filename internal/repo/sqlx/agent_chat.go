package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat/messages"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/internal/repo/sqlx/proto"
	"github.com/webitel/chat_manager/store/postgres"
	"strconv"
	"time"
)

func (c *sqlxRepository) MarkChatAsProcessed(ctx context.Context, chatId string, agentId int64) (int64, error) {

	result, err := c.db.ExecContext(
		ctx, catalogMarkProcessedSQL, chatId, agentId,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (c *sqlxRepository) GetAgentChats(req *app.SearchOptions, res *messages.GetAgentChatsResponse) error {
	ctx := req.Context
	cte, plan, err := constructAgentChatQuery(req)
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

	err = fetchChatAgentRows(
		rows, plan, res, req.GetSize(),
	)
	res.Page = int32(req.GetPage())
	if err != nil {
		return err
	}

	return nil
}

type agentChatArgs struct {
	AgentId     int64
	Timerange   *pb.Timerange
	Closed      *bool
	Unprocessed *bool
}

func constructAgentChatQuery(req *app.SearchOptions) (ctx *SELECT, plan dataFetch[*messages.AgentChat], err error) {
	var (
		left = "main"
	)
	args, err := newAgentChatQueryArgs(req)
	if err != nil {
		return
	}
	ctx = &SELECT{
		Params: params{},
	}
	ctx.Query = postgres.PGSQL.Select().
		From("chat.conversation " + left)

	threadQ, re := selectAgentChatThread(args, ctx.Params)
	if err = re; err != nil {
		return // nil, nil, err
	}
	ctx.With(CTE{
		Name: "thread",
		Expr: threadQ,
	})

	var (
		queueAlias  string
		threadAlias string
		joinQueue   = func() string {
			alias := queueAlias
			if alias != "" {
				return alias
			}
			alias = "queue"
			queueAlias = alias
			ctx.Query = ctx.Query.JoinClause(CompactSQL(fmt.Sprintf(
				`LEFT JOIN LATERAL (SELECT q.id, q.strategy, q.name
                            FROM call_center.cc_member_attempt_history m
                                     LEFT JOIN call_center.cc_queue q ON m.queue_id = q.id
                            WHERE m.member_call_id = main.id::::varchar
                            AND m.agent_call_id = agent.id::::varchar
							AND m.id = (%s ->>'cc_attempt_id')::::int8
                            UNION ALL
                            SELECT aq.id, aq.strategy, aq.name
                            FROM call_center.cc_member_attempt m
                                     LEFT JOIN call_center.cc_queue aq ON m.queue_id = aq.id
                            WHERE m.member_call_id = main.id::::varchar
                            AND m.agent_call_id = agent.id::::varchar
							AND m.id = (%[1]s ->>'cc_attempt_id')::::int8
) queue ON true`,
				ident(threadAlias, "props"))))
			return alias
		}

		joinThread = func() string {
			alias := threadAlias
			if alias != "" {
				return alias
			}
			alias = "agent"
			threadAlias = alias
			ctx.Query = ctx.Query.JoinClause(CompactSQL(
				`JOIN thread agent ON agent.conversation_id = main.id`,
			))
			return alias
		}
	)
	// by default
	joinThread()
	for _, field := range req.Fields {
		switch field {
		case "id":
			ctx.Query = ctx.Query.Column(
				ident(left, "id"),
			)
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.Text{Value: &node.Id}
			})
		case "title":
			ctx.Query = ctx.Query.Column(
				ident(left, "title"),
			)
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.Text{Value: &node.Title}
			})
		case "gateway":
			ctx.Query = ctx.Query.Column(
				CompactSQL(`(SELECT ROW (via.id, via.provider, via.name)
							FROM chat.channel ext
									 LEFT JOIN chat.bot via ON via.id::::text = ext.connection
							WHERE ext.conversation_id = ` + ident(left, "id") + `
							  AND NOT ext.internal) gateway`),
			)
			plan = append(plan, func(node *messages.AgentChat) any {
				return fetchPeerRow(&node.Gateway)
			})
		case "created_at":
			ctx.Query = ctx.Query.Column(ident(threadAlias, "joined_at"))
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.Epochtime{
					Precision: app.TimePrecision,
					Value:     &node.StartedAt,
				}
			})
		case "closed_at":
			ctx.Query = ctx.Query.Column(ident(threadAlias, "closed_at"))
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.Epochtime{
					Precision: app.TimePrecision,
					Value:     &node.ClosedAt,
				}
			})
		case "closed_cause":
			ctx.Query = ctx.Query.Column(ident(threadAlias, "closed_cause"))
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.Text{Value: &node.CloseReason}
			})
		case "last_message":
			ctx.Query = ctx.Query.Column(`(SELECT ROW (m.id,
                   m.created_at,
                   (
                       -- bot
                       SELECT ROW (id, 'bot', name)
                       FROM flow.acr_routing_scheme
                       WHERE id::::text = main.props ->> 'flow'
                         AND m.channel_id ISNULL

                       UNION ALL
                       -- agent
                       SELECT ROW (id, 'agent', name)
                       FROM directory.wbt_user
                       WHERE id = ANY (SELECT user_id FROM chat.channel WHERE internal AND id = m.channel_id LIMIT 1)

                       UNION ALL
                       -- contact or user
                       SELECT ROW (coalesce(ct.id, c.id), (CASE WHEN ct.id ISNULL THEN 'client' ELSE 'contact' END), coalesce(ct.common_name, c.name))
                       FROM chat.client c
                                LEFT JOIN contacts.contact_imclient im ON im.user_id = c.id
                                LEFT JOIN contacts.contact ct ON im.contact_id = ct.id
                       WHERE c.id = ANY
                             (SELECT user_id FROM chat.channel WHERE NOT internal AND id = m.channel_id LIMIT 1)),
                   m.text,
                   (file),
                   content)
        FROM chat.message m
                 LEFT JOIN LATERAL (SELECT m.file_id, m.file_size, m.file_type, m.file_name, m.file_url
                                    WHERE m.file_id NOTNULL OR m.file_url NOTNULL) file ON true
        WHERE m.conversation_id = ` + ident(left, "id") + `
        ORDER BY m.id DESC
        LIMIT 1) `,
			)
			plan = append(plan, func(node *messages.AgentChat) any {
				return DecodeText(func(src []byte) error {

					res := node.LastMessage
					node.LastMessage = nil // NULL
					if len(src) == 0 {
						return nil // NULL
					}

					if res == nil {
						res = new(messages.Message)
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
							fetchPeerRow(&res.From).(DecodeText),
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
							// content
							DecodeText(func(src []byte) error {
								// JSONB
								if len(src) == 0 {
									return nil // NULL
								}
								var data proto.Content
								err := protojsonCodec.Unmarshal(src, &data)
								if err != nil {
									return err
								}
								// set of columns to expose
								cols := []string{
									"keyboard",
									"postback",
									// "contact",
									// "location",
								}
								var e, n = 0, len(cols)
								for fd, expose := range map[string]func(){
									"postback": func() { res.Postback = data.Postback },
									"keyboard": func() { res.Keyboard = data.Keyboard },
								} {
									for e = 0; e < n && cols[e] != fd; e++ {
										// lookup: column requested ?
									}
									if e == n {
										// NOT FOUND; skip !
										continue
									}
									expose()
								}
								return nil // OK
							}),
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
						node.LastMessage = res
					}

					return nil
				})
			})
		case "needs_processing":
			ctx.Query = ctx.Query.Column(fmt.Sprintf("(%s ->> '%s')::::bool needs_processing", ident(threadAlias, "props"), ChatNeedsProcessingVariable))
			plan = append(plan, func(node *messages.AgentChat) any {
				return postgres.BoolValue{Value: &node.UnprocessedClose}
			})
		case "contact":
			ctx.Query = ctx.Query.Column(CompactSQL(`(SELECT ROW (ct.id, null, ct.common_name)
                       FROM contacts.contact_imclient im
                                LEFT JOIN contacts.contact ct ON im.contact_id = ct.id
                       WHERE im.user_id = ANY
                             (SELECT user_id FROM chat.channel WHERE NOT internal AND conversation_id = ` + ident(left, "id") + ` LIMIT 1)) contact`),
			)
			plan = append(plan, func(node *messages.AgentChat) any {
				return fetchPeerRow(&node.Contact)
			})
		case "queue":
			joinQueue()
			ctx.Query = ctx.Query.Column("(queue)")
			plan = append(plan, func(node *messages.AgentChat) any {
				return fetchPeerRow(&node.Queue)
			})
		default:
			err = errors.BadRequest("sqlxrepo.construct_agent_chat_query.check_fields.unknown", fmt.Sprintf("unknown field %s", field))
			return
		}
	}
	if size := req.GetSize(); size > 0 {
		if page := req.GetPage(); page > 1 {
			ctx.Query = ctx.Query.Offset((uint64)((page - 1) * size))
		}
		ctx.Query = ctx.Query.Limit((uint64)(size + 1))
	}
	return
}

func selectAgentChatThread(args *agentChatArgs, params params) (cte sq.SelectBuilder, err error) {
	if args == nil {
		err = errors.BadRequest("sqlxrepo.agent_chat.select_agent_chat_thread.check_args.args", "args required")
		return
	}
	cte = postgres.PGSQL.
		Select(
			"ch.conversation_id", "ch.closed_cause", "ch.props", "ch.closed_at", "ch.joined_at", "ch.id",
		).
		From(
			"chat.channel ch",
		).
		Where("ch.internal").
		Where("ch.user_id = :agent").
		OrderBy("ch.created_at DESC")
	if args.AgentId <= 0 {
		err = errors.BadRequest("sqlxrepo.agent_chat.select_agent_chat_thread.check_args.agent", "agent id required")
		return
	}
	params.set("agent", args.AgentId)
	if args.Timerange == nil || (args.Timerange.Since <= 0 && args.Timerange.Until <= 0) {
		err = errors.BadRequest("sqlxrepo.agent_chat.select_agent_chat_thread.check_args.timerange", "time range required")
		return
	}
	if args.Timerange.Since > 0 {
		params.set("dateFrom", time.UnixMilli(args.Timerange.Since))
		cte = cte.Where("ch.created_at >= :dateFrom")
	}
	if args.Timerange.Until > 0 {
		params.set("dateTo", time.UnixMilli(args.Timerange.Until))
		cte = cte.Where("ch.created_at <= :dateTo")
	}

	if args.Closed != nil {
		if *args.Closed {
			cte = cte.Where("ch.closed_at NOTNULL").
				// remove postprocessing case
				Where("NOT EXISTS (SELECT FROM call_center.cc_member_attempt a WHERE a.agent_call_id = ch.id::::varchar AND a.state != 'leaving')")
		} else {
			cte = cte.Where("ch.closed_at ISNULL")
		}
	}
	if args.Unprocessed != nil {
		if *args.Unprocessed {
			cte = cte.Where(fmt.Sprintf("(ch.props -> '%s' NOTNULL OR (ch.props -> '%[1]s')::::bool)", ChatNeedsProcessingVariable))
		} else {
			cte = cte.Where(fmt.Sprintf("(ch.props -> '%s' ISNULL OR NOT (ch.props -> '%[1]s')::::bool)", ChatNeedsProcessingVariable))
		}
	}
	return // cte, nil
}

func newAgentChatQueryArgs(req *app.SearchOptions) (*agentChatArgs, error) {
	if req == nil {
		return nil, nil
	}
	var args agentChatArgs
	for param, v := range req.Filter {
		switch param {
		case "agent":
			switch raw := v.(type) {
			case string:
				data, err := strconv.ParseInt(raw, 10, 64)
				if err != nil {
					return nil, err
				}
				if data <= 0 {
					return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.agent.string.invalid", "invalid agent")
				}
				args.AgentId = data
			case int64:
				if raw <= 0 {
					return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.agent.int.invalid", "invalid agent")
				}
				args.AgentId = raw
			default:
				return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.agent.type.unknown", "invalid argument for agent filter")
			}

		case "date":
			switch raw := v.(type) {
			case *pb.Timerange:
				if raw == nil {
					return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.date.timerange.nil", "invalid argument for date filter")
				}
				args.Timerange = raw
			default:
				return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.date.type.unknown", "invalid argument for date filter")
			}

		case "closed":
			switch raw := v.(type) {
			case *bool:
				if raw == nil {
					return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.closed.bool.nil", "invalid argument for close filter")
				}
				args.Closed = raw
			case bool:
				args.Closed = &raw
			default:
				return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.date.type.unknown", "invalid argument for close filter")
			}
		case "unprocessed":
			switch raw := v.(type) {
			case *bool:
				if raw == nil {
					return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.unprocessed.bool.nil", "invalid argument for unprocessed filter")
				}
				args.Unprocessed = raw
			case bool:
				args.Unprocessed = &raw
			default:
				return nil, errors.BadRequest("sqlxrepo.agent_chat.construct_agent_chat_args.unprocessed.type.unknown", "invalid argument for unprocessed filter")
			}

		}
	}

	return &args, nil
}

func fetchChatAgentRows(rows *sql.Rows, plan dataFetch[*messages.AgentChat], into *messages.GetAgentChatsResponse, limit int) (err error) {
	var (
		node *messages.AgentChat
		heap []messages.AgentChat

		page = into.GetItems()     // input
		data []*messages.AgentChat // output

		eval = make([]any, len(plan))
	)

	if 0 < limit {
		data = make([]*messages.AgentChat, 0, limit)
	}

	if n := limit - len(page); 1 < n {
		heap = make([]messages.AgentChat, n) // mempage; tidy
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
			node = new(messages.AgentChat)
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

	into.Items = data
	return nil
}

var (
	// $1 - conversation_id
	// $2 - agent that should process the chat
	catalogMarkProcessedSQL = CompactSQL(fmt.Sprintf(`
			UPDATE chat.channel SET props = props - '%s'
			WHERE conversation_id = $1 AND props->> '%[1]s' NOTNULL AND EXISTS(SELECT id FROM chat.channel WHERE conversation_id = $1 AND internal AND user_id = $2)
        `, ChatNeedsProcessingVariable))
)
