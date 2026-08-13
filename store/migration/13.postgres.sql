create index "idx_chat_invite_conversation_domain"
on "chat"."invite" using btree("conversation_id", "domain_id");

create index "idx_call_center_member_attempt_member_call_id_channel"
on "call_center"."cc_member_attempt" using btree("member_call_id") where "channel" = 'chat';

create index "idx_chat_channel_domain_open" on chat.channel using btree("domain_id") where closed_at is null;
