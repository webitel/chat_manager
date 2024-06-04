
ALTER TABLE chat.message
  ADD content jsonb NULL
;

COMMENT ON COLUMN chat.message.content IS 'Message content associated';
