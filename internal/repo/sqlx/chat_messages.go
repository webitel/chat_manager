package sqlxrepo

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/micro/micro/v3/service/errors"
	pb "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	"github.com/webitel/chat_manager/internal/repo/sqlx/proto"
	dbx "github.com/webitel/chat_manager/store/database"
	"github.com/webitel/chat_manager/store/postgres"
)

func isZeroUUID(uuid [16]byte) bool {
	const max = 16
	for e := 0; e < max && uuid[e] != 0; e++ {
		return false // FOUND: non-zero(!) byte
	}
	return true // ALL(16) are zero(!)
}

type chatMessagesArgs struct {
	// ----- Input ------ //
	// Search term: message.text
	Q string
	// [D]omain[C]omponent primary ID.
	// Mandatory(!)
	DC int64
	// Self peer.id
	// If non-zero return common dialog(s) ONLY(!)
	Self int64
	// Peer, as a member of dialog(s).
	Peer *pb.Peer
	// Includes the history of ONLY those dialogs
	// whose member channel(s) contain a specified set of variables
	Group map[string]string
	// ----- Output ----- //
	// Fields to return into result.
	Fields []string
	// Offset messages history options.
	Offset struct {
		// Message ID
		Id int64
		// Message Date
		Date *time.Time
	}
	// Limit count of messages to return.
	Limit int
	// Make sure that chat connected with case
	CaseId int64
	// Make sure that chat connected with contact
	ContactId int64
	// Exclude messages filter options
	Exclude struct {
		// Kind of message(s) to exclude
		Kind []string
	}
}

type contactChatMessagesArgs struct {
	// ----- Input ------ //
	// Search term: message.text
	Q string
	// [D]omain[C]omponent primary ID.
	// Mandatory(!)
	DC int64
	// Possible chat ID
	Peer *pb.Peer
	// Required contact ID
	ContactId string
	// Required contact ID
	Closed bool
	// Includes the history of ONLY those dialogs
	// whose member channel(s) contain a specified set of variables
	Group map[string]string
	// ----- Output ----- //
	// Fields to return into result.
	Fields []string
	// Offset messages history options.
	Offset struct {
		// Message ID
		Id int64
		// Message Date
		Date *time.Time
	}
	// Limit count of messages to return.
	Limit int
	// Page
	Page int
}

func (e *chatMessagesArgs) indexField(name string, alias ...string) int {
	var a, c = 0, len(alias)
	var i, n = 0, len(e.Fields)
	for ; i < n && e.Fields[i] != name; i++ {
		// match: by <name>
		for a = 0; a < c && e.Fields[i] != alias[a]; a++ {
			// match: by <alias>
		}
		if a < c {
			return i
		}
	}
	if i < n {
		return i
	}
	return -1
}

func (e *contactChatMessagesArgs) indexField(name string, alias ...string) int {
	var a, c = 0, len(alias)
	var i, n = 0, len(e.Fields)
	for ; i < n && e.Fields[i] != name; i++ {
		// match: by <name>
		for a = 0; a < c && e.Fields[i] != alias[a]; a++ {
			// match: by <alias>
		}
		if a < c {
			return i
		}
	}
	if i < n {
		return i
	}
	return -1
}

type chatMessagesQuery struct {
	Input chatMessagesArgs
	SELECT
	plan dataFetch[*pb.Message]
}

type contactChatMessagesQuery struct {
	Input contactChatMessagesArgs
	SELECT
	plan dataFetch[*pb.ChatMessage]
}

func getMessagesInput(req *app.SearchOptions) (args chatMessagesArgs, err error) {

	args.Q = req.Term
	if app.IsPresent(args.Q) {
		args.Q = "" // clear; ignore
	} else {
		for _, kindOf := range []string{
			"media", "file", "text",
		} {
			if strings.HasPrefix(
				args.Q, kindOf+":",
			) {
				args.Q = "*" + args.Q + "*"
				break
			}
		}
	}
	args.DC = req.Context.Creds.Dc
	args.Limit = req.GetSize()
	args.Fields = app.FieldsFunc(
		req.Fields, // app.InlineFields,
		app.SelectFields(
			// application: default
			[]string{
				"id",
				"from", // sender; user
				"date",
				"edit",
				"kind", // custom message.type classifier.
				"text",
				"file",
			},
			// operational
			[]string{
				"chat",   // chat dialog, that this message belongs to ..
				"sender", // chat member, on behalf of the "chat" (dialog)
				"context",
				"postback", // Quick Reply button Click[ed].
				"keyboard", // Quick Replies. Button(s)
			},
		),
	)

	inputPeer := func(input *pb.Peer) error {
		if args.Peer != nil {
			return errors.BadRequest(
				"messages.query.peer.input",
				"messages( peer: peer! ); input: ambiguous; oneof{ chat, peer }",
			)
		}
		if input.Id == "" {
			return errors.BadRequest(
				"messages.query.peer.input",
				"messages( peer( id: string! ) ); input: required but missing",
			)
		}
		if input.Type == "" {
			return errors.BadRequest(
				"messages.query.peer.input",
				"messages( peer( type: string! ) ); input: required but missing",
			)
		}
		args.Peer = input
		return nil // OK
	}

	for param, input := range req.Filter {
		switch param {
		case "self":
			args.Self = req.Creds.UserId
		case "peer":
			{
				switch data := input.(type) {
				case *pb.Peer:
					err = inputPeer(data)
				default:
					err = errors.BadRequest(
						"messages.query.peer.input",
						"messages( peer: peer! ); input: convert %T into Peer",
						input,
					)
				}
				if err != nil {
					return // args, err
				}
			}
		case "group":
			{
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
						"messages.query.group.input",
						"messages( group: {variables} ); input: convert %T into variables",
						input,
					)
				}
			}
		case "chat.id":
			{
				var chatId uuid.UUID
				switch data := input.(type) {
				case uuid.UUID:
					chatId = data
				case string:
					if data == "" {
						break // omitted
					}
					chatId, err = uuid.Parse(data)
					if err != nil {
						err = errors.BadRequest(
							"messages.query.peer.chat.input",
							"messages( chat: id! ); input: %v",
							err,
						)
					}
				default:
					err = errors.BadRequest(
						"messages.query.chat.input",
						"messages( chat: id ); input: convert %T into ID",
						input,
					)
				}
				if err == nil && !isZeroUUID(chatId) {
					err = inputPeer(&pb.Peer{
						Type: "chat",
						Id:   hex.EncodeToString(chatId[:]),
					})
				}
				if err != nil {
					return // args, err
				}
			}
		case "case.id":
			{
				var caseId int64
				switch data := input.(type) {
				case int64:
					caseId = data
				case int:
					caseId = int64(data)
				case string:
					caseId, err = strconv.ParseInt(data, 10, 64)
					if err != nil {
						err = errors.InternalServerError(
							"messages.query.chat.input",
							err.Error(),
						)
					}
				default:
					err = errors.BadRequest(
						"messages.query.chat.input",
						"messages( case: id ); input: convert %T into ID",
						input,
					)
				}
				args.CaseId = caseId
			}
		case "offset":
			{
				switch data := input.(type) {
				case *pb.ChatMessagesRequest_Offset:
					{
						if data.GetId() > 0 {
							args.Offset.Id = data.Id
						} else if data.GetDate() > 0 {
							date := app.EpochtimeDate(
								data.Date, app.TimePrecision, // (milli)
							)
							args.Offset.Date = &date
						} else {
							// IGNORE(?)
						}
					}
				// case int64:
				// case int:
				default:
					err = errors.BadRequest(
						"messages.query.offset.input",
						"messages( offset: ! ); input: convert %T into Offset",
						input,
					)
					return // args, err
				}
			}
		case "exclude.kind":
			{
				switch data := input.(type) {
				case string:
					{
						data = strings.TrimSpace(data)
						if data == "" {
							// ignore: empty
							break
						}
						args.Exclude.Kind = []string{data}
					}
				case []string:
					{
						// explode by: ',' or ' '
						data = app.FieldsFunc(
							// data, app.InlineFields, // case-ignore (lower)
							data, func(inline string) []string { // case-exact !!!
								return strings.FieldsFunc(inline, func(c rune) bool {
									return c == ',' || unicode.IsSpace(c)
								})
							},
						)
						for i, n := 0, len(data); i < n; i++ {
							data[i] = strings.TrimSpace(data[i])
							if data[i] == "" {
								// ignore: empty
								data = append(data[:i], data[i+1:]...)
								n--
								i--
							}
						}
						if len(data) == 0 {
							// ignore: none
							break
						}
						args.Exclude.Kind = data
					}
				default:
					err = errors.BadRequest(
						"messages.query.exclude.kind.input",
						"messages( exclude.kind: [string!] ); input: convert %T into []string",
						input,
					)
				}
			}
		default:
			err = errors.BadRequest(
				"messages.query.args.error",
				"messages( %s: ? ); input: no such argument",
				param,
			)
			return // args, err
		}
	}

	if args.Peer.GetId() == "" {
		err = errors.BadRequest(
			"messages.peer.id.required",
			"messages( peer.id: string! ); input: required",
		)
		return // nil, err
	}

	if args.Peer.GetType() == "" {
		err = errors.BadRequest(
			"messages.peer.type.required",
			"messages( peer.type: string! ); input: required",
		)
		return // nil, err
	}

	return // args, nil
}

func getContactMessagesInput(req *app.SearchOptions) (args contactChatMessagesArgs, err error) {

	args.Q = req.Term
	if app.IsPresent(args.Q) {
		args.Q = "" // clear; ignore
	} else {
		for _, kindOf := range []string{
			"media", "file", "text",
		} {
			if strings.HasPrefix(
				args.Q, kindOf+":",
			) {
				args.Q = "*" + args.Q + "*"
				break
			}
		}
	}
	args.DC = req.Context.Creds.Dc
	args.Page = req.GetPage()

	inputPeer := func(input *pb.Peer) error {
		if args.Peer != nil {
			return errors.BadRequest(
				"contact.messages.query.peer.input",
				"contact.messages( peer: peer! ); input: ambiguous; oneof{ chat, peer }",
			)
		}
		if input.Id == "" {
			return errors.BadRequest(
				"contact.messages.query.peer.input",
				"contact.messages( peer( id: string! ) ); input: required but missing",
			)
		}
		if input.Type == "" {
			return errors.BadRequest(
				"contact.messages.query.peer.input",
				"contact.messages( peer( type: string! ) ); input: required but missing",
			)
		}
		args.Peer = input
		return nil // OK
	}

	for param, input := range req.Filter {
		switch param {
		case "contact.id":
			{
				switch data := input.(type) {
				case string:
					if data == "" {
						err = errors.BadRequest(
							"contact.messages.query.contact_id.input.nil",
							"contact.messages( contact: id! ); input: nil",
							input,
						)
					}
					args.ContactId = data

				default:
					err = errors.BadRequest(
						"contact.messages.query.contact_id.input",
						"contact.messages( contact: id! ); input: convert %T into string",
						input,
					)
				}
				if err != nil {
					return // args, err
				}
			}
		case "group":
			{
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
						"contact.messages.query.group.input",
						"contact.messages( group: {variables} ); input: convert %T into variables",
						input,
					)
				}
			}
		case "chat.id":
			{
				var chatId uuid.UUID
				switch data := input.(type) {
				case uuid.UUID:
					chatId = data
				case string:
					if data == "" {
						break // omitted
					}
					chatId, err = uuid.Parse(data)
					if err != nil {
						err = errors.BadRequest(
							"contact.messages.query.peer.chat.input",
							"messages( chat: id! ); input: %v",
							err,
						)
					}
				default:
					err = errors.BadRequest(
						"contact.messages.query.chat.input",
						"contact.messages( chat: id ); input: convert %T into ID",
						input,
					)
				}
				if err == nil && !isZeroUUID(chatId) {
					err = inputPeer(&pb.Peer{
						Type: "chat",
						Id:   hex.EncodeToString(chatId[:]),
					})
				}
				if err != nil {
					return // args, err
				}
			}
		case "offset":
			{
				switch data := input.(type) {
				case *pb.ChatMessagesRequest_Offset:
					{
						if data.GetId() > 0 {
							args.Offset.Id = data.Id
						} else if data.GetDate() > 0 {
							date := app.EpochtimeDate(
								data.Date, app.TimePrecision, // (milli)
							)
							args.Offset.Date = &date
						} else {
							// IGNORE(?)
						}
					}
				// case int64:
				// case int:
				default:
					err = errors.BadRequest(
						"contact.messages.query.offset.input",
						"contact.messages( offset: ! ); input: convert %T into Offset",
						input,
					)
					return // args, err
				}
			}
		default:
			err = errors.BadRequest(
				"contact.messages.query.args.error",
				"contact.messages( %s: ? ); input: no such argument",
				param,
			)
			return // args, err
		}
	}

	if args.ContactId == "" {
		err = errors.BadRequest(
			"contact.messages.query.peer.input",
			"messages( peer( type: string! ) ); input: required but missing",
		)
	}

	// split fields for merged and unmerged history
	// if peer not found then user wants to see all merged messages by contact
	if args.Peer == nil {
		// Provide default sizing
		args.Limit = req.GetSize()
		args.Closed = true
		args.Fields = app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// application: default
				[]string{
					"id",
					"from", // sender; user
					"date",
					"edit",
					"text",
					"file",
					"chat", // chat dialog, that this message belongs to ..
				},
				// operational
				[]string{
					// "kind",
					"sender", // chat member, on behalf of the "chat" (dialog)
					"context",
					"postback", // Quick Reply button Click[ed].
					"keyboard", // Quick Replies. Button(s)
				},
			),
		)
	} else {
		// Provide
		var size int
		if req.Size <= 0 {
			// get all history
			size = 0
		} else {
			// sizing enabled, set
			size = req.GetSize()
		}
		args.Limit = size
		args.Closed = false
		args.Fields = app.FieldsFunc(
			req.Fields, // app.InlineFields,
			app.SelectFields(
				// application: default
				[]string{
					"id",
					"from", // sender; user
					"date",
					"edit",
					"text",
					"file",
				},
				// operational
				[]string{
					"chat",   // chat dialog, that this message belongs to ..
					"sender", // chat member, on behalf of the "chat" (dialog)
					"context",
					"postback", // Quick Reply button Click[ed].
					"keyboard", // Quick Replies. Button(s)
				},
			),
		)
	}

	return // args, nil
}

// if `updates` true - query history forward to get updates from some `offset` state (chat.top message)
// otherwise - will query history back in time ..
func getHistoryQuery(req *app.SearchOptions, updates bool) (ctx chatMessagesQuery, err error) {

	ctx.Input, err = getMessagesInput(req)
	if err != nil {
		return // nil, err
	}

	// default: history (back in time)
	var (
		offsetOp = "<"    // backward offset
		resOrder = "DESC" // NEWest..to..OLDest
	)

	if updates {
		// get difference from offset
		if q := ctx.Input.Q; q != "" {
			// NO SEARCH AVAILABLE
			ctx.Input.Q = ""
		}
		// if ctx.Input.Peer.GetId() == "" {
		// 	// getMessagesInput(REQUIRE) ;
		// }
		if ctx.Input.Offset.Id < 1 && ctx.Input.Offset.Date == nil {
			// REQUIRED
			dummy := req.Localtime()
			ctx.Input.Offset.Date = &dummy
		}
		offsetOp = ">"   // forward offset
		resOrder = "ASC" // OLDest..to..NEWest
	}

	ctx.Params = params{
		"pdc": ctx.Input.DC,
	}

	var left string
	// region: ----- resolve: thread(s) -----
	left = "t"
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "id"),
		).
		From(
			"chat.conversation " + left,
		).
		Where(
			ident(left, "domain_id") + " = :pdc",
		).
		GroupBy(
			ident(left, "id"),
		).
		OrderBy(
			ident(left, "created_at") + " DESC",
		)

	if ctx.Input.Self > 0 {
		ctx.Params.set("self", ctx.Input.Self)
		ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
			// INNER; dialog(s) in common with current user, that has [been] joined !
			"JOIN %[2]s %[3]s ON %[3]s.internal AND %[3]s.user_id = :self AND %[3]s.conversation_id = %[1]s.id",
			left, "chat.channel", "self",
		))
	}

	var (
		// aliasBot     string // "b"
		aliasChat string // "c"
		// aliasUser    string // "u"
		aliasContact string // "x"
		// LEFT JOIN flow.acr_routing_scheme AS b
		/*joinPeerBot = func() string {
			if aliasBot != "" {
				return aliasBot
			}
			// once
			aliasBot = "b"
			ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
				"LEFT JOIN flow.acr_routing_scheme %[2]s ON %[2]s.id = (%[1]s.props->>'flow')::::int8",
				left, aliasBot,
			))
			return aliasBot
		}*/
		// JOIN chat.channel AS c
		joinChatPeer = func() string {
			if aliasChat != "" {
				return aliasChat
			}
			// once
			aliasChat = "c"
			ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
				"JOIN chat.channel %[2]s ON %[2]s.conversation_id = %[1]s.id",
				left, aliasChat,
			))
			return aliasChat
		}
		// LEFT JOIN directory.wbt_auth AS u
		/*joinPeerUser = func() string {
			if aliasUser != "" {
				return aliasUser
			}
			// once
			aliasUser = "u"
			left, right := joinPeerChat(), aliasUser
			ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
				"LEFT JOIN directory.wbt_auth %[2]s ON %[1]s.internal AND %[2]s.id = %[1]s.user_id",
				left, right,
			))
			return aliasUser
		}*/
		// LEFT JOIN chat.client AS x
		joinPeerContact = func() string {
			if aliasContact != "" {
				return aliasContact
			}
			// once
			aliasContact = "u"
			left, right := joinChatPeer(), aliasContact
			ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
				"LEFT JOIN chat.client %[2]s ON NOT %[1]s.internal AND %[2]s.id = %[1]s.user_id",
				left, right,
			))
			return aliasContact
		}
	)

	switch ctx.Input.Peer.Type {
	case "chat":
		{
			if caseId := ctx.Input.CaseId; caseId != 0 { // required connection between case and chat
				ctx.Params.set("case.id", &caseId)
				properStringUuid, err := uuid.Parse(ctx.Input.Peer.Id)
				if err != nil {
					return ctx, err
				}
				ctx.Params.set("chat.id", properStringUuid.String())
				ctx.Query = ctx.Query.Where(ident(left, "id") + " = ANY (SELECT communication_id::::uuid FROM cases.case_communication WHERE communication_id = :chat.id AND case_id = :case.id )")
			} else if contactId := ctx.Input.ContactId; contactId != 0 { // required connection between contact and chat
				// TODO
			} else {
				var chatId pgtype.UUID
				err = chatId.Set(ctx.Input.Peer.Id)
				if err != nil {
					panic("messages.chat.id: " + err.Error())
				}
				ctx.Params.set("chat.id", &chatId)
				ctx.Query = ctx.Query.Where(
					ident(left, "id") + " = :chat.id",
				)
			}
		}
	case "user":
		{
			relChat := joinChatPeer()
			ctx.Params.set("peer.id", ctx.Input.Peer.Id)
			ctx.Query = ctx.Query.Where(sq.And{
				sq.Expr(ident(relChat, "internal")), // true
				sq.Expr(ident(relChat, "user_id") + "::::text LIKE :peer.id"),
			})
		}
	case "bot":
		{
			relBot := left // joinPeerBot()
			ctx.Params.set("peer.id", ctx.Input.Peer.Id)
			ctx.Query = ctx.Query.Where(
				ident(relBot, "props->>'flow'") + " LIKE :peer.id",
			)
		}
	default:
		// case "user":
		// case "bot": // useless(!) Agent CANNOT text with bot !
		relChat := joinChatPeer()
		relPeer := joinPeerContact()
		ctx.Params.set("peer.id", ctx.Input.Peer.Id)
		ctx.Params.set("peer.type", ctx.Input.Peer.Type)
		ctx.Query = ctx.Query.Where(sq.And{
			sq.Expr(ident(relChat, "type") + " = :peer.type"),
			sq.Expr(ident(relPeer, "external_id") + " LIKE :peer.id"),
		})
	}
	if q := ctx.Input.Q; q != "" {
		ctx.Params.set("q", app.Substring(q))
		ctx.Query = ctx.Query.JoinClause(CompactSQL(fmt.Sprintf(
			`JOIN LATERAL
			(
				SELECT
					true
				FROM
					chat.message m
				WHERE
					m.conversation_id = %[1]s.id
					AND FORMAT
					(
						'%%s%%s%%s'
					, '; media::' || m.file_type
					, '; file::' || m.file_name
					, '; text::' || m.text
					)
					ILIKE :q COLLATE "default"
				LIMIT 1
			) m ON true`,
			left,
		)))
	}
	// Includes the history of ONLY those dialogs
	// whose member channel(s) contain a specified set of variables
	if len(ctx.Input.Group) > 0 {
		group := pgtype.JSONB{
			Bytes: dbx.NullJSONBytes(ctx.Input.Group),
		}
		// set: status.Present
		group.Set(group.Bytes)
		ctx.Params.set("group", &group)
		// t.props
		expr := ident(left, "props")
		// coalesce(c.props,t.props)
		if aliasChat != "" {
			expr = fmt.Sprintf(
				"coalesce(%s.props,%s.props)",
				aliasChat, left,
			)
		}
		// @>  Does the first JSON value contain the second ?
		ctx.Query = ctx.Query.Where(
			expr + "@>:group",
		)
	}

	const (
		threadView = "thread"
	)
	ctx.With(CTE{
		Name: threadView,
		Expr: ctx.Query,
	})
	// endregion: ----- resolve: thread(s) -----

	// region: ----- select: message(s) -----
	left = "m"
	threadAlias := "q"
	ctx.Query = postgres.PGSQL.
		Select(
			// mandatory(!)
			ident(left, "id"),
		).
		From(
			"chat.message " + left,
		).
		JoinClause(fmt.Sprintf(
			"JOIN %[2]s %[3]s ON %[1]s.conversation_id = %[3]s.id",
			left, threadView, threadAlias,
		)).
		OrderBy(
			ident(left, "id") + " " + resOrder,
		)
	// mandatory(!)
	ctx.plan = append(ctx.plan,
		// "id"
		func(node *pb.Message) any {
			return postgres.Int8{Value: &node.Id}
		},
	)

	var (
		cols   []string
		column = func(name string) bool {
			var e, n = 0, len(cols)
			for ; e < n && cols[e] != name; e++ {
				// lookup: already selected ?
			}
			if e < n {
				// FOUND; selected !
				return false
			}
			cols = append(cols, name)
			return true
		}
		scanContent = func(node *pb.Message) any {
			return DecodeText(func(src []byte) error {
				// JSONB
				if len(src) == 0 {
					return nil // NULL
				}
				var data proto.Content
				err := protojsonCodec.Unmarshal(src, &data)
				if err != nil {
					return err
				}
				var e, n = 0, len(cols)
				for fd, pull := range map[string]func(){
					"postback": func() { node.Postback = data.Postback },
					"keyboard": func() { node.Keyboard = data.Keyboard },
					// "contact",
				} {
					for e = 0; e < n && cols[e] != fd; e++ {
						// lookup: column requested ?
					}
					if e == n {
						// NOT FOUND; skip !
						continue
					}
					pull()
				}
				return nil // OK
			})
		}
	)
	fields := ctx.Input.Fields // req.Fields
	if ctx.Input.indexField("sender") < 0 {
		fields = append(fields, "sender") // REFERENCE sender_chat.from
	}
	for _, field := range fields {
		switch field {
		case "id":
			// mandatory(!)
		case "date":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "created_at") + " date",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return postgres.Epochtime{
							Value: &node.Date, Precision: app.TimePrecision,
						}
					},
				)
			}
		case "edit":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "updated_at") + " edit",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return postgres.Epochtime{
							Precision: app.TimePrecision,
							Value:     &node.Edit,
						}
					},
				)
			}
		case "from":
			// TODO: below ...
		case "chat":
			{
				if !column("chat_id") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "conversation_id") + " chat_id",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						// return postgres.Int8{&node.Chat}
						return DecodeText(func(src []byte) error {

							chat := node.Chat
							node.Chat = nil // NULLify

							var id pgtype.UUID
							err := id.DecodeText(nil, src)
							if err != nil || id.Status != pgtype.Present {
								return err // err|nil
							}

							if chat == nil {
								chat = new(pb.Chat)
							}
							*(chat) = pb.Chat{
								Id: hex.EncodeToString(id.Bytes[:]),
							}

							node.Chat = chat
							return nil
						})
					},
				)
			}
		case "sender":
			{
				if !column("sender_chat_id") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(fmt.Sprintf(
					"coalesce(%[1]s.channel_id, %[1]s.conversation_id) sender_chat_id",
					left,
				))
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						// return postgres.Int8{&node.SenderChat}
						return DecodeText(func(src []byte) error {

							sender := node.Sender
							node.Sender = nil // NULLify

							var id pgtype.UUID
							err := id.DecodeText(nil, src)
							if err != nil || id.Status != pgtype.Present {
								return err // err|nil
							}

							if sender == nil {
								sender = new(pb.Chat)
							}
							*(sender) = pb.Chat{
								Id: hex.EncodeToString(id.Bytes[:]),
							}

							node.Sender = sender
							return nil
						})
					},
				)
			}
		case "kind":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "variables->>'kind'"),
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return postgres.Text{Value: &node.Kind}
					},
				)
			}
		case "text":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "text"),
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return postgres.Text{Value: &node.Text}
					},
				)
			}
		case "file":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.JoinClause(
					CompactSQL(fmt.Sprintf(
						`LEFT JOIN LATERAL(
						SELECT
							%[1]s.file_id id,
							%[1]s.file_size size,
							%[1]s.file_type "type",
							%[1]s.file_name "name",
							%[1]s.file_url "url"
						WHERE
						%[1]s.file_id NOTNULL OR %[1]s.file_url NOTNULL
					) %[2]s ON true`,
						left, "file",
					)))
				ctx.Query = ctx.Query.Column(
					"(file)", // ROW(file)
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return fetchFileRow(&node.File)
					},
				)
			}
		case "postback", "keyboard":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				if !column("content") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "content"),
				)
				// once for all "content" -related fields..
				ctx.plan = append(
					ctx.plan, scanContent,
				)
			}
		case "context":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "variables") + " context",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.Message) any {
						return DecodeText(func(src []byte) error {
							return dbx.ScanJSONBytes(&node.Context)(src)
						})
					},
				)
			}
		default:
			err = errors.BadRequest(
				"messages.query.fields.error",
				"messages{ %s }; input: no such field",
				field,
			)
			return // ctx, err
		}
	}
	// PATTERN: Q
	if q := ctx.Input.Q; q != "" {
		// ctx.Params.set("q", app.Substring(q)) // DONE: above(!)
		ctx.Query = ctx.Query.Where(CompactSQL(fmt.Sprintf(
			`FORMAT
			(
				'%%s%%s%%s'
			, '; media::' || %[1]s.file_type
			, '; file::' || %[1]s.file_name
			, '; text::' || %[1]s.text
			)
			ILIKE :q COLLATE "default"`,
			left,
		)))
	}
	// OFFSET
	if ctx.Input.Offset.Id > 0 {
		ctx.Params.set(
			"offset.id", ctx.Input.Offset.Id,
		)
		ctx.Query = ctx.Query.Where(fmt.Sprintf(
			"%s %s :offset.id",
			ident(left, "id"), offsetOp,
		))
	} else if ctx.Input.Offset.Date != nil {
		var date pgtype.Timestamp
		err = date.Set(
			ctx.Input.Offset.Date.UTC(),
		)
		if err != nil {
			return // ctx, err
		}
		ctx.Params.set(
			"offset.date", &date,
		)
		ctx.Query = ctx.Query.Where(fmt.Sprintf(
			"%s AT TIME ZONE 'UTC' %s :offset.date",
			ident(left, "created_at"), offsetOp,
		))
	}
	// EXCLUDE message.kind[of] records
	if len(ctx.Input.Exclude.Kind) > 0 {
		var (
			expr      = "<> ALL(:kind.not)"
			param any = ctx.Input.Exclude.Kind
		)
		if len(ctx.Input.Exclude.Kind) == 1 {
			expr = "<> :kind.not"
			param = ctx.Input.Exclude.Kind[0]
		}
		ctx.Params.set(
			"kind.not", param,
		)
		ctx.Query = ctx.Query.Where(fmt.Sprintf(
			"coalesce(%s,'') %s",
			ident(left, "variables->>'kind'"), expr,
		))
	}
	// LIMIT
	if ctx.Input.Limit > 0 {
		ctx.Query = ctx.Query.Limit(
			uint64(ctx.Input.Limit),
		)
	}

	const messageView = "message"
	ctx.With(CTE{
		Name: messageView,
		Expr: ctx.Query,
	})
	// endregion: ----- select: message(s) -----

	// region: ----- select: sender(s) -----
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "sender_chat_id") + " id",
		).
		From(
			messageView + " " + left,
		).
		GroupBy(
			ident(left, "sender_chat_id"),
		)

	const (
		senderChatView = "sender_chat"
	)
	ctx.With(CTE{
		Name: senderChatView,
		Expr: ctx.Query,
	})

	left = "c"
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "id"),
			ident(left, "type"),
			ident(left, "name"),
			"array_agg("+ident(left, "chat_id")+") chat_id",
		).
		FromSelect(
			postgres.PGSQL.
				Select(
					"c.id chat_id",
					"'bot' \"type\"",
					"c.props->>'flow' id", // text
					"b.name::::text \"name\"",
				).
				From(
					"chat.conversation c",
				).
				JoinClause(fmt.Sprintf(
					// INNER
					"JOIN %[2]s %[3]s ON %[1]s.id = %[3]s.id",
					"c", senderChatView, "q",
				)).
				JoinClause(fmt.Sprintf(
					"LEFT JOIN %[2]s %[3]s ON %[3]s.id = (%[1]s.props->>'flow')::::int8",
					"c", "flow.acr_routing_scheme", "b",
				)).
				Suffix(
					"UNION ALL",
				).
				SuffixExpr(postgres.PGSQL.
					Select(
						"c.id chat_id",
						"(case when not c.internal then c.type else 'user' end) \"type\"",
						"(case when c.internal then c.user_id::::text else x.external_id end) id", // NULL -if- deleted
						"coalesce(x.name, u.chat_name, u.name, u.username::::text, c.name, '[deleted]') \"name\"",
					).
					From(
						"chat.channel c",
					).
					JoinClause(fmt.Sprintf( // INNER
						"JOIN %[2]s %[3]s ON %[1]s.id = %[3]s.id",
						"c", senderChatView, "q",
					)).
					JoinClause(fmt.Sprintf( // external
						"LEFT JOIN %[2]s %[3]s ON NOT %[1]s.internal AND %[3]s.id = %[1]s.user_id",
						"c", "chat.client", "x",
					)).
					JoinClause(fmt.Sprintf( // internal
						"LEFT JOIN %[2]s %[3]s ON %[1]s.internal AND %[3]s.id = %[1]s.user_id",
						"c", "directory.wbt_user", "u",
					)),
				),
			left,
		).
		GroupBy(
			ident(left, "id"),
			ident(left, "type"),
			ident(left, "name"),
		)

	const (
		senderView = "sender"
	)
	ctx.With(CTE{
		Name: senderView,
		Expr: ctx.Query,
	})
	// endregion: ----- select: sender(s) -----

	// region: ----- select: result(page) -----
	ctx.Query = postgres.PGSQL.
		Select(
			fmt.Sprintf(
				"ARRAY( SELECT %[2]s from %[1]s %[2]s ) peers",
				senderView, "c",
			),
			fmt.Sprintf(
				"ARRAY( SELECT %[2]s from %[1]s %[2]s ) messages",
				messageView, "m",
			),
		)
	// endregion: ----- select: result(page) -----

	return // ctx, nil
}

// if `updates` true - query history forward to get updates from some `offset` state (chat.top message)
// otherwise - will query history back in time ..
func getContactHistoryQuery(req *app.SearchOptions, updates bool) (ctx contactChatMessagesQuery, err error) {
	ctx.Input, err = getContactMessagesInput(req)
	if err != nil {
		return // nil, err
	}

	// default: history (back in time)
	var (
		offsetOp = "<"    // backward offset
		resOrder = "DESC" // NEWest..to..OLDest
	)

	if updates {
		// get difference from offset
		if q := ctx.Input.Q; q != "" {
			// NO SEARCH AVAILABLE
			ctx.Input.Q = ""
		}
		if ctx.Input.Offset.Id < 1 && ctx.Input.Offset.Date == nil {
			// REQUIRED
			dummy := req.Localtime()
			ctx.Input.Offset.Date = &dummy
		}
		offsetOp = ">"   // forward offset
		resOrder = "ASC" // OLDest..to..NEWest
	}

	ctx.Params = params{
		"pdc": ctx.Input.DC,
	}

	var left string
	// region: ----- resolve: thread(s) -----
	left = "t"
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "id"), ident(left, "created_at"),
		).
		From(
			"chat.conversation " + left,
		).
		Where(
			ident(left, "domain_id") + " = :pdc",
		).
		GroupBy(
			ident(left, "id"),
		)

	var (
		aliasChat    string // "c"
		joinChatPeer = func() string {
			if aliasChat != "" {
				return aliasChat
			}
			// once
			aliasChat = "c"
			ctx.Query = ctx.Query.JoinClause(fmt.Sprintf(
				"JOIN chat.channel %[2]s ON %[2]s.conversation_id = %[1]s.id",
				left, aliasChat,
			))
			return aliasChat
		}
	)
	if ctId := ctx.Input.ContactId; ctId != "" {
		relChat := joinChatPeer()
		ctx.Params.set("contact.id", ctId)
		ctx.Query = ctx.Query.Where(
			sq.Expr(ident(relChat, "user_id") + " = any (SELECT user_id from contacts.contact_imclient where contact_id = :contact.id)"),
		).Where(sq.Expr(fmt.Sprintf("not %s.internal", relChat)))
	}
	if peer := ctx.Input.Peer; peer != nil {
		switch ctx.Input.Peer.Type {
		default:
			// case "chat":
			{
				var chatId pgtype.UUID
				err = chatId.Set(ctx.Input.Peer.Id)
				if err != nil {
					panic("messages.chat.id: " + err.Error())
				}
				ctx.Params.set("chat.id", &chatId)
				ctx.Query = ctx.Query.Where(sq.And{
					sq.Expr(ident(left, "id") + " = :chat.id"),
				},
				)
			}
		}
	}

	if ctx.Input.Closed {
		ctx.Query = ctx.Query.Where(ident(aliasChat, "closed_at NOTNULL"))
	}

	if q := ctx.Input.Q; q != "" {
		ctx.Params.set("q", app.Substring(q))
		ctx.Query = ctx.Query.JoinClause(CompactSQL(fmt.Sprintf(
			`JOIN LATERAL
			(
				SELECT
					true
				FROM
					chat.message m
				WHERE
					m.conversation_id = %[1]s.id
					AND FORMAT
					(
						'%%s%%s%%s'
					, '; media::' || m.file_type
					, '; file::' || m.file_name
					, '; text::' || m.text
					)
					ILIKE :q COLLATE "default"
				LIMIT 1
			) m ON true`,
			left,
		)))
	}
	// Includes the history of ONLY those dialogs
	// whose member channel(s) contain a specified set of variables
	if len(ctx.Input.Group) > 0 {
		group := pgtype.JSONB{
			Bytes: dbx.NullJSONBytes(ctx.Input.Group),
		}
		// set: status.Present
		group.Set(group.Bytes)
		ctx.Params.set("group", &group)
		// t.props
		expr := ident(left, "props")
		// coalesce(c.props,t.props)
		if aliasChat != "" {
			expr = fmt.Sprintf(
				"coalesce(%s.props,%s.props)",
				aliasChat, left,
			)
		}
		// @>  Does the first JSON value contain the second ?
		ctx.Query = ctx.Query.Where(
			expr + "@>:group",
		)
	}

	const (
		threadView = "thread"
	)
	ctx.With(CTE{
		Name: threadView,
		Expr: ctx.Query,
	})
	// endregion: ----- resolve: thread(s) -----

	// region: ----- select: message(s) -----
	left = "m"
	threadAlias := "q"
	ctx.Query = postgres.PGSQL.
		Select(
			// mandatory(!)
			ident(left, "id"),
		).
		From(
			"chat.message " + left,
		).
		JoinClause(fmt.Sprintf(
			"JOIN %[2]s %[3]s ON %[1]s.conversation_id = %[3]s.id",
			left, threadView, threadAlias,
		)).
		OrderBy(
			fmt.Sprintf("%s %s, %s %[2]s", ident(threadAlias, "created_at"), resOrder, ident(left, "id")),
		)
	// mandatory(!)
	ctx.plan = append(ctx.plan,
		// "id"
		func(node *pb.ChatMessage) any {
			return postgres.Int8{Value: &node.Id}
		},
	)

	var (
		cols   []string
		column = func(name string) bool {
			var e, n = 0, len(cols)
			for ; e < n && cols[e] != name; e++ {
				// lookup: already selected ?
			}
			if e < n {
				// FOUND; selected !
				return false
			}
			cols = append(cols, name)
			return true
		}
		scanContent = func(node *pb.ChatMessage) any {
			return DecodeText(func(src []byte) error {
				// JSONB
				if len(src) == 0 {
					return nil // NULL
				}
				var data proto.ContactMessageContent
				err := protojsonCodec.Unmarshal(src, &data)
				if err != nil {
					return err
				}
				var e, n = 0, len(cols)
				for fd, pull := range map[string]func(){
					"postback": func() { node.Postback = data.Postback },
					"keyboard": func() { node.Keyboard = data.Keyboard },
					// "contact",
				} {
					for e = 0; e < n && cols[e] != fd; e++ {
						// lookup: column requested ?
					}
					if e == n {
						// NOT FOUND; skip !
						continue
					}
					pull()
				}
				return nil // OK
			})
		}
	)
	fields := ctx.Input.Fields // req.Fields
	if ctx.Input.indexField("sender") < 0 {
		fields = append(fields, "sender") // REFERENCE sender_chat.from
	}
	for _, field := range fields {
		switch field {
		case "id":
			// mandatory(!)
		case "date":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "created_at") + " date",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						return postgres.Epochtime{
							Value: &node.Date, Precision: app.TimePrecision,
						}
					},
				)
			}
		case "edit":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "updated_at") + " edit",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						return postgres.Epochtime{
							Precision: app.TimePrecision,
							Value:     &node.Edit,
						}
					},
				)
			}
		case "from":
			// TODO: below ...
		case "chat":
			{
				if !column("chat_id") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "conversation_id") + " chat_id",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						// return postgres.Int8{&node.Chat}
						return DecodeText(func(src []byte) error {

							chat := node.Chat
							node.Chat = nil // NULLify

							var id pgtype.UUID
							err := id.DecodeText(nil, src)
							if err != nil || id.Status != pgtype.Present {
								return err // err|nil
							}

							if chat == nil {
								chat = new(pb.ContactChat)
							}
							*(chat) = pb.ContactChat{
								Id: hex.EncodeToString(id.Bytes[:]),
							}

							node.Chat = chat
							return nil
						})
					},
				)
			}
		case "sender":
			{
				if !column("sender_chat_id") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(fmt.Sprintf(
					"coalesce(%[1]s.channel_id, %[1]s.conversation_id) sender_chat_id",
					left,
				))
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						// return postgres.Int8{&node.SenderChat}
						return DecodeText(func(src []byte) error {

							sender := node.Sender
							node.Sender = nil // NULLify

							var id pgtype.UUID
							err := id.DecodeText(nil, src)
							if err != nil || id.Status != pgtype.Present {
								return err // err|nil
							}

							if sender == nil {
								sender = new(pb.ContactChat)
							}
							*(sender) = pb.ContactChat{
								Id: hex.EncodeToString(id.Bytes[:]),
							}

							node.Sender = sender
							return nil
						})
					},
				)
			}
		case "kind":
			{
				// if !column(field) {
				// 	break // switch; duplicate!
				// }
				// ctx.Query = ctx.Query.Column(
				// 	ident(left, "variables->>'kind'"),
				// )
				// ctx.plan = append(ctx.plan,
				// 	func(node *pb.ChatMessage) any {
				// 		return postgres.Text{Value: &node.Kind}
				// 	},
				// )
			}
		case "text":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "text"),
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						return postgres.Text{Value: &node.Text}
					},
				)
			}
		case "file":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.JoinClause(
					CompactSQL(fmt.Sprintf(
						`LEFT JOIN LATERAL(
						SELECT
							%[1]s.file_id id,
							%[1]s.file_size size,
							%[1]s.file_type "type",
							%[1]s.file_name "name",
							%[1]s.file_url "url"
						WHERE
						%[1]s.file_id NOTNULL OR m.file_url NOTNULL 
					) %[2]s ON true`,
						left, "file",
					)))
				ctx.Query = ctx.Query.Column(
					"(file)", // ROW(file)
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						return fetchContactFileRow(&node.File)
					},
				)
			}
		case "postback", "keyboard":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				if !column("content") {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "content"),
				)
				// once for all "content" -related fields..
				ctx.plan = append(
					ctx.plan, scanContent,
				)
			}
		case "context":
			{
				if !column(field) {
					break // switch; duplicate!
				}
				ctx.Query = ctx.Query.Column(
					ident(left, "variables") + " context",
				)
				ctx.plan = append(ctx.plan,
					func(node *pb.ChatMessage) any {
						return DecodeText(func(src []byte) error {
							return dbx.ScanJSONBytes(&node.Context)(src)
						})
					},
				)
			}
		default:
			err = errors.BadRequest(
				"messages.query.fields.error",
				"messages{ %s }; input: no such field",
				field,
			)
			return // ctx, err
		}
	}
	// PATTERN: Q
	if q := ctx.Input.Q; q != "" {
		// ctx.Params.set("q", app.Substring(q)) // DONE: above(!)
		ctx.Query = ctx.Query.Where(CompactSQL(fmt.Sprintf(
			`FORMAT
			(
				'%%s%%s%%s'
			, '; media::' || %[1]s.file_type
			, '; file::' || %[1]s.file_name
			, '; text::' || %[1]s.text
			)
			ILIKE :q COLLATE "default"`,
			left,
		)))
	}
	// OFFSET
	if ctx.Input.Offset.Id > 0 {
		ctx.Params.set(
			"offset.id", ctx.Input.Offset.Id,
		)
		ctx.Query = ctx.Query.Where(fmt.Sprintf(
			"%s %s :offset.id",
			ident(left, "id"), offsetOp,
		))
	} else if ctx.Input.Offset.Date != nil {
		var date pgtype.Timestamp
		err = date.Set(
			ctx.Input.Offset.Date.UTC(),
		)
		if err != nil {
			return // ctx, err
		}
		ctx.Params.set(
			"offset.date", &date,
		)
		ctx.Query = ctx.Query.Where(fmt.Sprintf(
			"%s AT TIME ZONE 'UTC' %s :offset.date",
			ident(left, "created_at"), offsetOp,
		))
	}
	// LIMIT
	if ctx.Input.Limit > 0 {
		ctx.Query = ctx.Query.Limit(
			uint64(ctx.Input.Limit + 1),
		)
	}
	// Paging
	if p := ctx.Input.Page; p > 1 {
		ctx.Query = ctx.Query.Offset(
			uint64((p - 1) * ctx.Input.Limit),
		)
	}

	const messageView = "message"
	ctx.With(CTE{
		Name: messageView,
		Expr: ctx.Query,
	})
	// endregion: ----- select: message(s) -----

	// region: ----- select: sender(s) -----
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "sender_chat_id") + " id",
		).
		From(
			messageView + " " + left,
		).
		GroupBy(
			ident(left, "sender_chat_id"),
		)

	const (
		senderChatView = "sender_chat"
	)
	ctx.With(CTE{
		Name: senderChatView,
		Expr: ctx.Query,
	})

	left = "c"
	ctx.Query = postgres.PGSQL.
		Select(
			ident(left, "id"),
			ident(left, "type"),
			ident(left, "name"),
			"array_agg("+ident(left, "chat_id")+") chat_id",
		).
		FromSelect(
			postgres.PGSQL.
				Select(
					"c.id chat_id",
					"'bot' \"type\"",
					"c.props->>'flow' id", // text
					"b.name::::text \"name\"",
				).
				From(
					"chat.conversation c",
				).
				JoinClause(fmt.Sprintf(
					// INNER
					"JOIN %[2]s %[3]s ON %[1]s.id = %[3]s.id",
					"c", senderChatView, "q",
				)).
				JoinClause(fmt.Sprintf(
					"LEFT JOIN %[2]s %[3]s ON %[3]s.id = (%[1]s.props->>'flow')::::int8",
					"c", "flow.acr_routing_scheme", "b",
				)).
				Suffix(
					"UNION ALL",
				).
				SuffixExpr(postgres.PGSQL.
					Select(
						"c.id chat_id",
						"(case when not c.internal then c.type else 'user' end) \"type\"",
						"(case when c.internal then c.user_id::::text else x.external_id end) id", // NULL -if- deleted
						"coalesce(x.name, u.name, u.auth::::text, '[deleted]') \"name\"",
					).
					From(
						"chat.channel c",
					).
					JoinClause(fmt.Sprintf( // INNER
						"JOIN %[2]s %[3]s ON %[1]s.id = %[3]s.id",
						"c", senderChatView, "q",
					)).
					JoinClause(fmt.Sprintf( // external
						"LEFT JOIN %[2]s %[3]s ON NOT %[1]s.internal AND %[3]s.id = %[1]s.user_id",
						"c", "chat.client", "x",
					)).
					JoinClause(fmt.Sprintf( // internal
						"LEFT JOIN %[2]s %[3]s ON %[1]s.internal AND %[3]s.id = %[1]s.user_id",
						"c", "directory.wbt_auth", "u",
					)),
				),
			left,
		).
		GroupBy(
			ident(left, "id"),
			ident(left, "type"),
			ident(left, "name"),
		)

	const (
		senderView = "sender"
	)
	ctx.With(CTE{
		Name: senderView,
		Expr: ctx.Query,
	})
	// endregion: ----- select: sender(s) -----

	// endregion: ----- select: chat(s) -----
	const (
		chatView = "chat"
	)
	includeChat := ctx.Input.indexField("chat") > 0
	if includeChat {
		left = "conv"
		ctx.Query = postgres.PGSQL.
			Select(
				ident(left, "id"),
				ident(left, "domain_id"),
				"gate.id",
				"gate.provider",
				"gate.name",
			).
			From("chat.channel ch").
			LeftJoin("chat.conversation conv ON ch.conversation_id = conv.id").
			LeftJoin("chat.bot gate ON ch.connection::::bigint = gate.id").
			Where(sq.Expr(ident(left, "id") + "= any (select distinct m.chat_id from message m)")).
			Where(sq.Expr("NOT ch.internal"))
		ctx.With(CTE{
			Name: chatView,
			Expr: ctx.Query,
		})
	}

	// endregion: ----- select: chat(s) -----

	// region: ----- select: result(page) -----
	if includeChat {
		ctx.Query = postgres.PGSQL.
			Select(
				fmt.Sprintf(
					"ARRAY( SELECT %[2]s from %[1]s %[2]s ) chats",
					chatView, "c",
				),
				fmt.Sprintf(
					"ARRAY( SELECT %[2]s from %[1]s %[2]s ) peers",
					senderView, "c",
				),
				fmt.Sprintf(
					"ARRAY( SELECT %[2]s from %[1]s %[2]s ) messages",
					messageView, "m",
				),
			)
	} else {
		ctx.Query = postgres.PGSQL.
			Select(
				fmt.Sprintf(
					"ARRAY( SELECT %[2]s from %[1]s %[2]s ) peers",
					senderView, "c",
				),
				fmt.Sprintf(
					"ARRAY( SELECT %[2]s from %[1]s %[2]s ) messages",
					messageView, "m",
				),
			)
	}
	// endregion: ----- select: result(page) -----

	return // ctx, nil
}

func (ctx chatMessagesQuery) scanRows(rows *sql.Rows, into *pb.ChatMessages) error {

	type (
		UUID [16]byte
		peer struct {
			alias pb.Peer
			node  *pb.Peer
		}
		chat struct {
			id    UUID
			peer  *peer
			node  *pb.Chat
			alias pb.Chat
		}
	)

	var (
		chats     []*chat
		peers     []*peer
		outFrom   = !(ctx.Input.indexField("from") < 0)
		outSender = !(ctx.Input.indexField("sender") < 0)
		equalUUID = func(this, that UUID) bool {
			var e int
			const max = 16
			for ; e < max && this[e] == that[e]; e++ {
				// advance: bytes are equal(!)
			}
			return e == max
		}
		getSender = func(chatId UUID) *chat {
			var e, n = 0, len(chats)
			for ; e < n && !equalUUID(chatId, chats[e].id); e++ {
				// lookup: match by original chat( id: uuid! )
			}
			if e == n {
				panic(fmt.Errorf("chat( id: %x ); not fetched", chatId))
			}
			return chats[e]
		}
	)

	var (
		fetch = []any{
			// peers
			DecodeText(func(src []byte) error {
				// parse: array(row(sender))
				rows, err := pgtype.ParseUntypedTextArray(string(src))
				if err != nil {
					return err
				}

				var (
					node *pb.Peer
					heap []pb.Peer

					page = into.GetPeers() // input
					// data []*pb.Peer        // output

					// eval = make([]any, 4) // len(ctx.plan))
					size = len(rows.Elements)

					text   pgtype.Text
					chatId pgtype.UUIDArray

					scan = []TextDecoder{
						// id
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Id = text.String
							// ok = ok || text.String != "" // && str.Status == pgtype.Present
							return nil
						}),
						// type
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Type = text.String
							// ok = ok || text.String != "" // && str.Status == pgtype.Present
							return nil
						}),
						// name
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Name = text.String
							// ok = ok || text.String != "" // && str.Status == pgtype.Present
							return nil
						}),
						// chat_id
						DecodeText(func(src []byte) error {
							err := chatId.DecodeText(nil, src)
							if err != nil {
								return err
							}
							// ok = ok || text.String != "" // && str.Status == pgtype.Present
							return nil
						}),
					}
				)

				if 0 < size {
					// data = make([]*pb.Peer, 0, size)
					peers = make([]*peer, 0, size)
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

					for _, col := range scan {
						raw.ScanDecoder(col)
						err = raw.Err()
						if err != nil {
							return err
						}
					}

					// data = append(data, node)
					peer := &peer{
						node: node, alias: pb.Peer{
							Id: strconv.Itoa(len(peers) + 1),
						},
					}
					peers = append(peers, peer)
					if outFrom { // output: peers
						into.Peers = append(into.Peers, node)
					}

					for _, chatId := range chatId.Elements {
						// TODO: disclose peer(node).chat_id relation !
						chatRow := &pb.Chat{
							Id:   hex.EncodeToString(chatId.Bytes[:]),
							Peer: &peer.alias,
						}
						chatRef := &chat{
							// data
							peer: peer,
							node: chatRow,
							// refs
							id: chatId.Bytes,
							alias: pb.Chat{
								Id: strconv.Itoa(len(chats) + 1),
							},
						}
						chats = append(chats, chatRef)
						if outSender { // output: chats
							into.Chats = append(into.Chats, chatRow)
						}
					}
				}
				return nil
			}),
			// messages
			DecodeText(func(src []byte) error {
				// parse: array(row(message))
				rows, err := pgtype.ParseUntypedTextArray(
					string(src),
				)
				if err != nil {
					return err
				}

				var (
					node *pb.Message
					heap []pb.Message

					page = into.GetMessages() // input
					data []*pb.Message        // output

					eval = make([]any, len(ctx.plan))
					size = len(rows.Elements)
				)

				if 0 < size {
					data = make([]*pb.Message, 0, size)
				}

				if n := size - len(page); 1 < n {
					heap = make([]pb.Message, n) // mempage; tidy
				}

				var (
					c   int // column
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
						node = new(pb.Message)
					}
					// [BIND] data fields to scan row
					c = 0
					for _, bind := range ctx.plan {

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
					senderId := uuid.MustParse(node.Sender.Id)
					senderChat := getSender(UUID(senderId))
					node.From = &senderChat.peer.alias
					if outSender {
						node.Sender = &senderChat.alias
					} else {
						node.Sender = nil
					}
					data = append(data, node)
				}

				into.Messages = data
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

func (ctx contactChatMessagesQuery) scanRows(rows *sql.Rows, req *app.SearchOptions, into *pb.GetContactChatHistoryResponse) error {

	type (
		UUID [16]byte
		peer struct {
			alias pb.ChatPeer
			node  *pb.ChatPeer
		}
		chatPeer struct {
			id    UUID
			peer  *peer
			node  *pb.ContactChat
			alias pb.ContactChat
		}
		chat struct {
			id    UUID
			node  *pb.ContactChat
			alias pb.ContactChat
		}
	)

	var (
		chats     []*chat
		chatPeers []*chatPeer
		peers     []*peer
		outFrom   = !(ctx.Input.indexField("from") < 0)
		outSender = !(ctx.Input.indexField("sender") < 0)
		outChat   = !(ctx.Input.indexField("chat") < 0)
		equalUUID = func(this, that UUID) bool {
			var e int
			const max = 16
			for ; e < max && this[e] == that[e]; e++ {
				// advance: bytes are equal(!)
			}
			return e == max
		}
		getSender = func(chatId UUID) *chatPeer {
			var e, n = 0, len(chatPeers)
			for ; e < n && !equalUUID(chatId, chatPeers[e].id); e++ {
				// lookup: match by original chat( id: uuid! )
			}
			if e == n {
				panic(fmt.Errorf("chat( id: %x ); not fetched", chatId))
			}
			return chatPeers[e]
		}
		getChat = func(chatId UUID) *chat {
			fmt.Printf("%x", chatId)
			var e, n = 0, len(chats)
			for ; e < n && !equalUUID(chatId, chats[e].id); e++ {
				// lookup: match by original chat( id: uuid! )
			}
			if e == n {
				panic(fmt.Errorf("chat( id: %x ); not fetched", chatId))
			}
			return chats[e]
		}
	)

	var (
		fetch []any
	)

	if outChat {
		fetch = append(fetch,
			// chats
			DecodeText(func(src []byte) error {
				// parse: array(row(sender))
				rows, err := pgtype.ParseUntypedTextArray(string(src))
				if err != nil {
					return err
				}

				var (
					node *pb.ContactChat
					heap []pb.ContactChat

					page = into.GetChats() // input
					size = len(rows.Elements)

					text pgtype.Text

					scan = []TextDecoder{
						// id
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Id = text.String
							// ok = ok || text.String != "" // && str.Status == pgtype.Present
							return nil
						}),
						// domain
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							node.Dc, err = strconv.ParseInt(text.String, 10, 64)
							if err != nil {
								return err
							}
							return nil
						}),
						// gateway.id
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							if node.GetVia() == nil {
								node.Via = &pb.ChatPeer{}
							}
							node.Via.Id = text.String
							return nil
						}),
						// gateway.type
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							if node.GetVia() == nil {
								node.Via = &pb.ChatPeer{}
							}
							node.Via.Type = text.String
							return nil
						}),
						// gateway.name
						DecodeText(func(src []byte) error {
							err := text.DecodeText(nil, src)
							if err != nil {
								return err
							}
							if node.GetVia() == nil {
								node.Via = &pb.ChatPeer{}
							}
							node.Via.Name = text.String
							return nil
						}),
					}
				)

				if 0 < size {
					// data = make([]*pb.Peer, 0, size)
					peers = make([]*peer, 0, size)
				}

				if n := size - len(page); 1 < n {
					heap = make([]pb.ContactChat, n) // mempage; tidy
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
						node = new(pb.ContactChat)
					}

					for _, col := range scan {
						raw.ScanDecoder(col)
						err = raw.Err()
						if err != nil {
							return err
						}
					}

					into.Chats = append(into.Chats, node)
					chats = append(chats, &chat{
						id:   UUID(uuid.MustParse(node.Id)),
						node: node,
						alias: pb.ContactChat{
							Id: strconv.Itoa(len(chats) + 1),
						},
					})
				}
				return nil
			}))
	}
	fetch = append(fetch,
		// peers
		DecodeText(func(src []byte) error {
			// parse: array(row(sender))
			rows, err := pgtype.ParseUntypedTextArray(string(src))
			if err != nil {
				return err
			}

			var (
				node *pb.ChatPeer
				heap []pb.ChatPeer

				page = into.GetPeers() // input
				// data []*pb.Peer        // output

				// eval = make([]any, 4) // len(ctx.plan))
				size = len(rows.Elements)

				text   pgtype.Text
				chatId pgtype.UUIDArray

				scan = []TextDecoder{
					// id
					DecodeText(func(src []byte) error {
						err := text.DecodeText(nil, src)
						if err != nil {
							return err
						}
						node.Id = text.String
						// ok = ok || text.String != "" // && str.Status == pgtype.Present
						return nil
					}),
					// type
					DecodeText(func(src []byte) error {
						err := text.DecodeText(nil, src)
						if err != nil {
							return err
						}
						node.Type = text.String
						// ok = ok || text.String != "" // && str.Status == pgtype.Present
						return nil
					}),
					// name
					DecodeText(func(src []byte) error {
						err := text.DecodeText(nil, src)
						if err != nil {
							return err
						}
						node.Name = text.String
						// ok = ok || text.String != "" // && str.Status == pgtype.Present
						return nil
					}),
					// chat_id
					DecodeText(func(src []byte) error {
						err := chatId.DecodeText(nil, src)
						if err != nil {
							return err
						}
						// ok = ok || text.String != "" // && str.Status == pgtype.Present
						return nil
					}),
				}
			)

			if 0 < size {
				// data = make([]*pb.Peer, 0, size)
				peers = make([]*peer, 0, size)
			}

			if n := size - len(page); 1 < n {
				heap = make([]pb.ChatPeer, n) // mempage; tidy
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
					node = new(pb.ChatPeer)
				}

				for _, col := range scan {
					raw.ScanDecoder(col)
					err = raw.Err()
					if err != nil {
						return err
					}
				}

				// data = append(data, node)
				peer := &peer{
					node: node, alias: pb.ChatPeer{
						Id: strconv.Itoa(len(peers) + 1),
					},
				}
				peers = append(peers, peer)
				if outFrom { // output: peers
					into.Peers = append(into.Peers, node)
				}

				for _, chatId := range chatId.Elements {
					// TODO: disclose peer(node).chat_id relation !
					chatRow := &pb.ContactChat{
						Id:   hex.EncodeToString(chatId.Bytes[:]),
						Peer: &peer.alias,
					}
					chatRef := &chatPeer{
						// data
						peer: peer,
						node: chatRow,
						// refs
						id: chatId.Bytes,
						alias: pb.ContactChat{
							Id: strconv.Itoa(len(chatPeers) + 1),
						},
					}
					chatPeers = append(chatPeers, chatRef)
				}
			}
			return nil
		}),

		// messages
		DecodeText(func(src []byte) error {
			// parse: array(row(message))
			rows, err := pgtype.ParseUntypedTextArray(
				string(src),
			)
			if err != nil {
				return err
			}

			var (
				node *pb.ChatMessage
				heap []pb.ChatMessage

				page = into.GetMessages() // input
				data []*pb.ChatMessage    // output

				eval = make([]any, len(ctx.plan))
				size = len(rows.Elements)
			)

			if 0 < size {
				data = make([]*pb.ChatMessage, 0, size)
			}

			if n := size - len(page); 1 < n {
				heap = make([]pb.ChatMessage, n) // mempage; tidy
			}

			var (
				c   int // column
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
					node = new(pb.ChatMessage)
				}
				// [BIND] data fields to scan row
				c = 0
				for _, bind := range ctx.plan {

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
				channelId := uuid.MustParse(node.Sender.Id)
				senderChannel := getSender(UUID(channelId))
				if outFrom {
					node.From = &senderChannel.peer.alias
				} else {
					node.From = nil
				}
				if !outSender {
					node.Sender = nil
				}
				if outChat {
					chatId := uuid.MustParse(node.Chat.Id)
					senderChat := getChat(UUID(chatId))
					node.Chat = &senderChat.alias
				}

				data = append(data, node)
			}

			into.Messages = data
			return nil
		}))

	for rows.Next() {
		err := rows.Scan(fetch...)
		if err != nil {
			return err
		}
		// once; MUST: single row
		break
	}

	if len(into.Messages) > req.GetSize() {
		into.Next = true
		into.Messages = into.Messages[:len(into.Messages)-1]
	}
	into.Page = int32(req.GetPage())

	return rows.Err()
}
