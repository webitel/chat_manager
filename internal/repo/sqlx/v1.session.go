package sqlxrepo

import (
	"context"
	"database/sql"
	"fmt"

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

		case "dc":
			proj[i] = func() interface{} { return &obj.DomainID }
		// case "domain":          proj[i] = func() interface{} { return omit }

		case "chat_id":
			proj[i] = func() interface{} { return &obj.Chat.ID }
		case "room_id":
			proj[i] = func() interface{} { return &obj.Chat.Invite }

		// case "leg":
		//
		case "chat_channel":
			proj[i] = func() interface{} { return &obj.Chat.Channel }
		case "chat_contact":
			proj[i] = func() interface{} { return &obj.Chat.Contact }
		// TO: this chat channel .User owner
		case "user_id":
			proj[i] = func() interface{} { return &obj.User.ID }
		case "user_channel":
			proj[i] = func() interface{} { return &obj.User.Channel }
		case "user_contact":
			proj[i] = func() interface{} { return &obj.User.Contact }
		case "user_name":
			proj[i] = func() interface{} { return &obj.User.FirstName }
		// FROM: chat room title name
		// case "title":
		case "props":
			proj[i] = func() interface{} { return ScanMetadata(&obj.Variables) }

		case "created_at":
			proj[i] = func() interface{} { return ScanTimestamp(&obj.Created) }
		case "updated_at":
			proj[i] = func() interface{} { return ScanTimestamp(&obj.Updated) }
		case "joined_at":
			proj[i] = func() interface{} { return ScanTimestamp(&obj.Joined) }
		case "closed_at":
			proj[i] = func() interface{} { return ScanTimestamp(&obj.Closed) }

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

// Select CHAT session with all it's member channels
// on behalf of given single, unique member channel ID
//
// $1 - chatID; Unique CHAT member channel ID
var psqlChatSessionQ = CompactSQL(
	`WITH session AS
(
	select
		id
	from
	(
		select
			$1
		from
			chat.conversation
		where
			id = $1
		union all
		(
			select
				conversation_id
			from
				chat.channel
			where
				id = $1
			union -- all
			select
				conversation_id
			from
				chat.invite
			where
				id = $1
		)
	) chat(id)
	limit 1
)
, channel as (
	-- @chatflow
	select

		chat.domain_id      as dc,
		channel.id          as room_id,
		channel.id          as chat_id,
--    true                as internal,
		'chatflow'          as chat_channel,
		chat.connection||coalesce('@'||service.node_id,'') as chat_contact,
		(false)             as flow_bridge, -- chat.flow_bridge,
		-- bot.flow_id         as user_id,
		coalesce(bot.flow_id, flow.id) user_id,
		'bot'               as user_channel,
		-- bot.flow_id::text   as user_contact,
		(channel.props->>'flow') user_contact,
		-- bot.name            as user_name,
		coalesce(bot.name, flow.name::text) user_name,
		-- coalesce(contact.name, nullif(account.name, ''), account.username, chat.name) as chat_title,
		channel.props,

		chat.created_at + interval '1 millisecond' as created_at,
		null as joined_at,
		channel.updated_at,
		coalesce(chat.closed_at, channel.closed_at) as closed_at

	from chat.conversation as channel
	join chat.channel as chat on chat.conversation_id = channel.id and chat.connection similar to '\d+'
																		-- MUST chat@gateway as originator, created earlier this chat.conversation
																		and chat.created_at <= channel.created_at + '3 millisecond'
	left join chat.bot on (chat.connection::int8) = bot.id -- type:portal has no gateway assigned !
	join flow.acr_routing_scheme flow ON flow.id = (channel.props->>'flow')::int8 and flow.domain_id = channel.domain_id
	left join chat.conversation_node as service on channel.id = service.conversation_id
--  -- to be able to resolve CHAT title !
--  left join chat.client as contact on not chat.internal and chat.user_id = contact.id -- external
--  left join directory.wbt_user as account on chat.internal and chat.user_id = account.id -- internal

	where channel.id = (select id from session)

	union all
	-- @channel
	select

		channel.domain_id                                         as dc,
		channel.conversation_id                                   as room_id,
		channel.id                                                as chat_id,
--    coalesce((account.dc = channel.domain_id), false)         as internal,
		coalesce(nullif(channel.type, 'webitel'), 'websocket')    as chat_channel,
		coalesce(channel.connection||coalesce('@'||channel.host,''), 'engine') as chat_contact,
		channel.flow_bridge,
		channel.user_id,
		coalesce(nullif(channel.type, 'webitel'), 'user')         as user_channel,
		coalesce(contact.external_id, account.username)           as user_contact,
		coalesce(contact.name, account.chat_name, account.name, account.username)    as user_name,
		-- channel.name                                              as chat_title,
		channel.props,

		channel.created_at,
		channel.joined_at,
		channel.updated_at,
		channel.closed_at

	from chat.channel
	left join chat.client as contact on not (channel.internal) and channel.user_id = contact.id -- external
	left join directory.wbt_user as account on channel.internal and channel.user_id = account.id -- internal

	where channel.conversation_id = (select id from session)

	union all
	-- invite(s) as live, pending channels too !..
	select

		invite.domain_id                         as dc,
		invite.conversation_id                   as room_id,
		invite.id                                as chat_id,
--    (true)                                   as internal,
		'websocket'                              as chat_channel,
		'engine'                                 as chat_contact,
		(invite.inviter_channel_id isnull)       as flow_bridge,
		invite.user_id                           as user_id,
		'user'                                   as user_channel,
		account.username                         as user_contact,
		coalesce(account.chat_name, account.name, account.username) as user_name,
		-- invite.title                             as chat_title,
		invite.props,

		invite.created_at,
		null as joined_at,
		invite.created_at as updated_at,
		null as closed_at

	from chat.invite
	left join directory.wbt_user as account on invite.user_id = account.id -- internal
	where invite.conversation_id = (select id from session) and invite.closed_at isnull
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
	chat.props,

	chat.created_at,
	chat.joined_at,
	chat.updated_at,
	chat.closed_at

from channel as chat
-- left join chat.profile as gate on chat.user_channel = 'bot' and chat.user_id = contact.id -- external
-- left join chat.client as contact on chat.user_channel and chat.user_id = contact.id -- external
-- left join directory.wbt_user as account on chat.user_channel = 'user' and chat.user_id = account.id -- internal
-- -- join channel as room on chat.room_id = room.chat_id
-- -- -- join directory.wbt_domain as srv on a.dc = srv.dc
-- -- where chat.room_id = (select room_id from channel where chat_id = $1)
--   -- and room.closed_at isnull -- chat.closed_at isnull

-- order by chat.room_id, chat.created_at asc
order by chat.created_at asc`)
