package sqlxrepo

import (

	"fmt"
	"context"
	"database/sql"

	"github.com/webitel/chat_manager/app" // as model
)

func (s *sqlxRepository) GetSession(ctx context.Context, chatID string) (chat *app.Session, err error) {

	// panic("not implemented")

	rows, err := s.db.QueryContext(ctx, psqlChatSessionQ, chatID)
	
	if err != nil {
		// Disclose DB schema specific errors !
		return nil, err
	}

	defer rows.Close()

	room := &app.Session{}
	room.Members, err = channelFetch(rows)

	if err != nil {
		return nil, err
	}

	for i, member := range room.Members {
		if member.Chat.ID == chatID {
			room.Channel = member // view for requested chatID as member
			room.Members = append(room.Members[:i], room.Members[i+1:]...)
			break
		}
	}

	if room.Channel == nil {
		return nil, nil // Not Found !
	}

	return room, nil // OK !
}

func channelFetch(rows *sql.Rows) ([]*app.Channel, error) {

	// region: prepare column => target bindings projection
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var (
		
		obj *app.Channel // cursor: current entry target object
		dst = make([]interface{}, len(cols), len(cols))

		list []*app.Channel
		proj = make([]func() interface{}, len(cols), len(cols))
		// // target := func(bind interface{}) {
		// // 	dst = append(dst, bind)
		// // }

		// omit = ScanFunc(
		// 	func(_ interface{}) error {
		// 		return nil
		// 	}
		// )
	)

	for i, col := range cols {
		switch col {
		
		case "dc":              proj[i] = func() interface{} { return &obj.DomainID }
		// case "domain":          proj[i] = func() interface{} { return omit }

		case "chat_id":         proj[i] = func() interface{} { return &obj.Chat.ID }
		case "room_id":         proj[i] = func() interface{} { return &obj.Chat.Invite }
		
		// case "leg":             
		// 
		case "chat_channel":    proj[i] = func() interface{} { return &obj.Chat.Channel }
		case "chat_contact":    proj[i] = func() interface{} { return &obj.Chat.Contact }
		// TO: this chat channel .User owner
		case "user_id":         proj[i] = func() interface{} { return &obj.User.ID }
		case "user_channel":    proj[i] = func() interface{} { return &obj.User.Channel }
		case "user_contact":    proj[i] = func() interface{} { return &obj.User.Contact }
		case "user_name":       proj[i] = func() interface{} { return &obj.User.FirstName }
		// FROM: chat room title name
		// case "title":              

		case "created_at":      proj[i] = func() interface{} { return ScanTimestamp(&obj.Created) }
		case "updated_at":      proj[i] = func() interface{} { return ScanTimestamp(&obj.Updated) }
		case "joined_at":       proj[i] = func() interface{} { return ScanTimestamp(&obj.Joined) }
		case "closed_at":       proj[i] = func() interface{} { return ScanTimestamp(&obj.Closed) }

		// case "closed_cause":    target(&m.ClosedCause)

		// case "flow_bridge":     target(&m.FlowBridge)

		// case "props":           target(ScanProperties(&m.Properties))

		default:

			return nil, fmt.Errorf("sql: scan %T column %q not implemented", obj, col)

		}
	}
	// endregion

	// region: fetch result entries
	for rows.Next() {
		// alloc NEW entry memory
		obj = &app.Channel{
			Chat: &app.Chat{},
			User: &app.User{},
		}
		// rebind NEW entry attributes
		for i, bind := range proj {
			dst[i] = bind()
		}
		// perform: fetch row columns ...
		err = rows.Scan(dst...)

		if err != nil {
			break
		}

		list = append(list, obj)
	}
	// endregion

	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		return nil, err
	}

	return list, nil
}



const psqlSessionQ =
`SELECT m.*
  FROM chat.channels AS c, chat.channels AS m
WHERE c.chat = $1 AND m.session = c.session
`

const psqlChatSessionQ =
`WITH channel as (
    select
       chat.domain_id      as dc,
       channel.id          as room_id,
       channel.id          as chat_id,
--        true                as internal,
       'chatflow'          as chat_channel,
--        service.node_id     as chat_contact,
       chat.connection||coalesce('@'||service.node_id,'') as chat_contact, -- $profile_id[@workflow-$node_id]
--        'bot:'||bot.name    as chat_contact,
       false as flow_bridge, -- chat.flow_bridge,
       bot.schema_id       as user_id,
       'bot'               as user_channel,
       bot.schema_id::text as user_contact,
       bot.name            as user_name,
       coalesce(contact.name, nullif(account.name, ''), account.username, chat.name) as chat_title,
--        service.node_id||'-'||chat.connection as app,
--        service.node_id     as host,
       null as props, -- chat.props,

       chat.created_at + interval '1 millisecond' as created_at,
       null as joined_at,
       channel.updated_at,
       coalesce(chat.closed_at, channel.closed_at) as closed_at

    from chat.channel as chat
    join chat.profile as bot on chat.connection = bot.id::text
    join chat.conversation as channel on channel.id = chat.conversation_id
    left join chat.conversation_node as service on channel.id = service.conversation_id
    left join chat.client as contact on (chat.internal, chat.user_id) = (false, contact.id) -- external
    left join directory.wbt_user as account on (chat.internal, chat.user_id) = (true, account.id) -- internal

    union all

    select

        channel.domain_id                                         as dc,
        channel.conversation_id                                   as room_id,
        channel.id                                                as chat_id,

--         coalesce((account.dc = channel.domain_id), false)         as internal,
        coalesce(nullif(channel.type, 'webitel'), 'websocket')    as chat_channel,
--         coalesce(nullif(channel.type, 'webitel'), 'user')||':'||
--         coalesce(contact.external_id, account.username)           as chat_contact,
--         coalesce('webitel.chat.bot-'||channel.host, 'engine')     as chat_contact,
        coalesce(channel.connection||coalesce('@'||channel.host,''), 'engine') as chat_contact, -- $profile_id[@webitel.chat.bot-$node_id]
        channel.flow_bridge,
        channel.user_id                                           as user_id,
        coalesce(nullif(channel.type, 'webitel'), 'user')         as user_channel,
        coalesce(contact.external_id, account.username)           as user_contact,
        coalesce(contact.name, account.name, account.username)    as user_name,
        -- chat.name                                              as title,
        channel.name                                              as chat_title,
--         coalesce('gateway-' || channel.connection || '-' || channel.host,
--                 websocket.app||'@'||websocket.socket)             as app,
        -- coalesce('webitel.chat.bot-'||channel.host, 'engine')     as host,
        channel.props,

        channel.created_at,
        channel.joined_at,
        channel.updated_at,
        channel.closed_at
        -- coalesce(channel.closed_at, session.closed_at) as closed_at

    from chat.channel
    left join chat.client as contact on (channel.internal, channel.user_id) = (false, contact.id) -- external
    left join directory.wbt_user as account on (channel.internal, channel.user_id) = (true, account.id) -- internal
--     left join lateral (
--         select *
--         from directory.wbt_user_socket as session
--         where session.user_id = channel.user_id
--         order by updated_at desc
--         limit 1
--     ) as websocket on true
)
select
    -- chat.*
    chat.dc,
    -- srv.name as domain,
    chat.room_id,
    chat.chat_id,
    -- chr((64 + row_number() over members)::int) as leg,
    chat.chat_channel,
    chat.chat_contact,
    -- chat.flow_bridge,
    chat.user_id,
    chat.user_channel,
    chat.user_contact,
    chat.user_name, -- TO: this channel end-user
    -- -- chat.chat_title,
    -- (case when chat.user_name = room.chat_title then room.user_name else room.chat_title end) as chat_title, -- FROM: chatroom title
    -- chat.props,

    chat.created_at,
    chat.joined_at,
    chat.updated_at,
    chat.closed_at

from channel as chat
join channel as room on chat.room_id = room.chat_id
-- join directory.wbt_domain as srv on a.dc = srv.dc
where chat.room_id = (select room_id from channel where chat_id = $1)
  and room.closed_at isnull -- chat.closed_at isnull
order by chat.room_id, chat.created_at asc`

// WITH channel as (
//     select
//        chat.domain_id      as dc,
//        channel.id          as room_id,
//        channel.id          as chat_id,
// --        true                as internal,
//        'chatflow'          as chat_channel,
//        service.node_id     as chat_contact,
// --        'bot:'||bot.name    as chat_contact,
//        false as flow_bridge, -- chat.flow_bridge,
//        bot.schema_id       as user_id,
//        'bot'               as user_channel,
//        bot.schema_id::text as user_contact,
//        bot.name            as user_name,
//        coalesce(contact.name, nullif(account.name, ''), account.username, chat.name) as chat_title,
// --        service.node_id||'-'||chat.connection as app,
// --        service.node_id     as host,
//        null as props, -- chat.props,

//        chat.created_at + interval '1 millisecond' as created_at,
//        null as joined_at,
//        channel.updated_at,
//        coalesce(chat.closed_at, channel.closed_at) as closed_at

//     from chat.channel as chat
//     join chat.profile as bot on chat.connection = bot.id::text
//     join chat.conversation as channel on channel.id = chat.conversation_id
//     left join chat.conversation_node as service on channel.id = service.conversation_id
//     left join chat.client as contact on (chat.internal, chat.user_id) = (false, contact.id) -- external
//     left join directory.wbt_user as account on (chat.internal, chat.user_id) = (true, account.id) -- internal

//     union all

//     select

//         channel.domain_id                                         as dc,
//         channel.conversation_id                                   as room_id,
//         channel.id                                                as chat_id,

// --         coalesce((account.dc = channel.domain_id), false)         as internal,
//         coalesce(nullif(channel.type, 'webitel'), 'websocket')    as chat_channel,
// --         coalesce(nullif(channel.type, 'webitel'), 'user')||':'||
// --         coalesce(contact.external_id, account.username)           as chat_contact,
//         coalesce('webitel.chat.bot-'||channel.host, 'engine')     as chat_contact,
//         channel.flow_bridge,
//         channel.user_id                                           as user_id,
//         coalesce(nullif(channel.type, 'webitel'), 'user')         as user_channel,
//         coalesce(contact.external_id, account.username)           as user_contact,
//         coalesce(contact.name, account.name, account.username)    as user_name,
//         -- chat.name                                              as title,
//         channel.name                                              as chat_title,
// --         coalesce('gateway-' || channel.connection || '-' || channel.host,
// --                 websocket.app||'@'||websocket.socket)             as app,
//         -- coalesce('webitel.chat.bot-'||channel.host, 'engine')     as host,
//         channel.props,

//         channel.created_at,
//         channel.joined_at,
//         channel.updated_at,
//         channel.closed_at
//         -- coalesce(channel.closed_at, session.closed_at) as closed_at

//     from chat.channel
//     left join chat.client as contact on (channel.internal, channel.user_id) = (false, contact.id) -- external
//     left join directory.wbt_user as account on (channel.internal, channel.user_id) = (true, account.id) -- internal
// --     left join lateral (
// --         select *
// --         from directory.wbt_user_socket as session
// --         where session.user_id = channel.user_id
// --         order by updated_at desc
// --         limit 1
// --     ) as websocket on true
// )
// select
//     -- chat.*
//     chat.dc,
//     -- srv.name as domain,
//     chat.room_id,
//     chat.chat_id,
//     -- chr((64 + row_number() over members)::int) as leg,
//     chat.chat_channel,
//     chat.chat_contact,
//     chat.flow_bridge,
//     chat.user_id,
//     chat.user_channel,
//     chat.user_contact,
//     chat.user_name, -- TO: this channel end-user
//     -- chat.chat_title,
//     (case when chat.user_name = room.chat_title then room.user_name else room.chat_title end) as chat_title, -- FROM: chatroom title
//     chat.props,

//     chat.created_at,
//     chat.joined_at,
//     chat.updated_at,
//     chat.closed_at

// from channel as chat
// join channel as room on chat.room_id = room.chat_id
// -- join directory.wbt_domain as srv on a.dc = srv.dc
// where room.closed_at isnull -- chat.closed_at isnull
//   and chat.room_id = (select room_id from channel where chat_id = :cid)
// order by chat.room_id, chat.created_at asc
// ;
