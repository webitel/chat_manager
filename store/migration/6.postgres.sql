
ALTER SEQUENCE chat.profile_id_seq
  OWNED BY chat.bot.id
;

DROP TABLE chat."_profile";
DROP TABLE chat.active_channel;

DROP SEQUENCE chat.bot_id_seq;
DROP SEQUENCE chat.channel_id_seq;
DROP SEQUENCE chat.conversation_id_seq;
DROP SEQUENCE chat.invite_id_seq;

-- chat.conversation_node --

ALTER TABLE chat.conversation_node
  DROP CONSTRAINT conversation_node_pk
, ALTER COLUMN conversation_id TYPE uuid USING conversation_id::uuid
;

-- chat.conversation_confirmation --

ALTER TABLE chat.conversation_confirmation
  DROP CONSTRAINT conversation_confirmation_pk
, ALTER COLUMN conversation_id TYPE uuid USING conversation_id::uuid
;

-- chat.invite --

DROP INDEX -- IF EXISTS
  chat.invite_conversation_id_user_id_uindex -- btree (conversation_id, user_id) WHERE (closed_at IS NULL)
;

ALTER TABLE chat.invite
  DROP CONSTRAINT invite_pk -- PRIMARY KEY (id)
, DROP CONSTRAINT invite_conversation_fk -- FOREIGN KEY (conversation_id) REFERENCES chat.conversation(id) ON DELETE CASCADE ON UPDATE CASCADE
, ALTER COLUMN id TYPE uuid USING id::uuid
, ALTER COLUMN conversation_id TYPE uuid USING conversation_id::uuid
, ALTER COLUMN inviter_channel_id TYPE uuid USING inviter_channel_id::uuid
;

-- chat.channel --

DROP INDEX -- IF EXISTS
  chat.channel_conversation_id_index -- btree (conversation_id)
, chat.channel_conversation_id_user_id_type_uindex -- btree (conversation_id, user_id, type) WHERE (closed_at IS NULL)
;

ALTER TABLE chat.channel
  DROP CONSTRAINT channel_pk -- PRIMARY KEY (id)
, DROP CONSTRAINT channel_conversation_fk -- FOREIGN KEY (conversation_id) REFERENCES chat.conversation(id) ON DELETE CASCADE ON UPDATE CASCADE
, ALTER COLUMN id TYPE uuid USING id::uuid
, ALTER COLUMN conversation_id TYPE uuid USING conversation_id::uuid
;

-- chat.message --

DROP INDEX -- IF EXISTS
  chat.message_sender_index -- btree (COALESCE(channel_id, conversation_id)) INCLUDE (created_at)
, chat.message_conversation_id_created_at_index -- btree (conversation_id, created_at DESC)
;

ALTER TABLE chat.message
  DROP CONSTRAINT message_conversation_fk -- FOREIGN KEY (conversation_id) REFERENCES chat.conversation(id) ON DELETE CASCADE ON UPDATE CASCADE
, ALTER COLUMN channel_id TYPE uuid USING channel_id::uuid
, ALTER COLUMN conversation_id TYPE uuid USING conversation_id::uuid
;

-- chat.conversation --

ALTER TABLE chat.conversation
  DROP CONSTRAINT conversation_pk
, ALTER COLUMN id TYPE uuid USING id::uuid
;

----------------------------------------------------------------------

-- chat.conversation --

ALTER TABLE chat.conversation
  ADD CONSTRAINT conversation_pk
    PRIMARY KEY (id)
;

-- chat.message --

ALTER TABLE chat.message
  ADD CONSTRAINT message_conversation_fk
    FOREIGN KEY (conversation_id)
    REFERENCES chat.conversation(id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
;

CREATE INDEX message_sender_index ON chat.message
  USING btree (COALESCE(channel_id, conversation_id)) INCLUDE (created_at)
;

CREATE INDEX message_conversation_id_created_at_index ON chat.message
  USING btree (conversation_id, created_at DESC)
;

-- chat.channel --

ALTER TABLE chat.channel
  ADD CONSTRAINT channel_pk
    PRIMARY KEY (id)
, ADD CONSTRAINT channel_conversation_fk
    FOREIGN KEY (conversation_id)
    REFERENCES chat.conversation(id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
;

CREATE INDEX channel_conversation_id_index ON chat.channel
  USING btree (conversation_id)
;
CREATE UNIQUE INDEX channel_conversation_id_user_id_type_uindex ON chat.channel
  USING btree (conversation_id, user_id, "type") WHERE (closed_at IS NULL)
;

-- chat.invite --

ALTER TABLE chat.invite
  ADD CONSTRAINT invite_pk
    PRIMARY KEY (id)
, ADD CONSTRAINT invite_conversation_fk
    FOREIGN KEY (conversation_id)
    REFERENCES chat.conversation(id)
    ON DELETE CASCADE
    ON UPDATE CASCADE
;

CREATE UNIQUE INDEX invite_conversation_id_user_id_uindex ON chat.invite
  USING btree (conversation_id, user_id) WHERE (closed_at IS NULL)
;

-- chat.conversation_confirmation --

ALTER TABLE chat.conversation_confirmation
  ADD CONSTRAINT conversation_confirmation_pk
    PRIMARY KEY (conversation_id)
;

-- chat.conversation_node --

ALTER TABLE chat.conversation_node
  ADD CONSTRAINT conversation_node_pk
    PRIMARY KEY (conversation_id)
;