CREATE INDEX IF NOT EXISTS channel_domain_id_internal_type_user_id_index
ON chat.channel(domain_id, internal, type, user_id)
WHERE NOT internal;