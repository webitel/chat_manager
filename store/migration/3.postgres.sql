
-- chat.client.id == directory.wbt_user.id; will cause a duplicate error !
-- FIX IT with chat.channel(user_id + type) !

drop index chat.channel_conversation_id_user_id_uindex;

create unique index channel_conversation_id_user_id_type_uindex
	on chat.channel (conversation_id, user_id, type)
	where (closed_at IS NULL);
