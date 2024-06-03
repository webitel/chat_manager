
ALTER TABLE chat.message
  ADD content jsonb NULL
;

COMMENT ON COLUMN chat.message.keyboard IS 'Message content associated';
