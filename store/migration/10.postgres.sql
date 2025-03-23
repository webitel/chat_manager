-- Add a new column 'type' of type name to the 'chat.client' table.
ALTER TABLE chat.client ADD COLUMN "type" name;

-- Fix bug with incorrect type in chat.channel. This referrer type was set to a broadcast.
WITH outgoing AS (
    SELECT c.user_id, c.type
    FROM chat.channel c
    WHERE c.closed_cause = 'broadcast_end'
    GROUP BY c.user_id, c.type
    ORDER BY c.user_id
),
original AS (
    SELECT c.user_id, c.type
    FROM chat.channel c
    JOIN outgoing r USING (user_id)
    WHERE c.closed_cause != 'broadcast_end'
    GROUP BY c.user_id, c.type
    ORDER BY c.user_id
),
plan AS (
    SELECT 
        r.user_id, 
        r.type AS old, 
        w.type AS new
    FROM outgoing r
    JOIN original w ON r.user_id = w.user_id
    WHERE r.type != w.type
)
UPDATE
    chat.channel e 
SET
    "type" = w.new
FROM
    plan w
WHERE
    e.user_id = w.user_id 
    AND e.type = w.old
RETURNING e.*;

-- Setting the correct values in the 'type' field in the chat.client table.
-- If the 'type' field contains NULL then this is not the correct logic!!!
-- This can only happen when the client has no entries in the chat.channel.
UPDATE chat.client AS c
SET "type" = x.type
FROM (
    SELECT x.user_id, x.type
    FROM chat.channel x
  	WHERE not x.internal
  	GROUP BY x.user_id, x.type
) x
WHERE c.id = x.user_id;
