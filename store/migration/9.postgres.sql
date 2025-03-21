-- Public name of the agent that will be displayed in chats
ALTER TABLE chat.channel
ADD COLUMN public_name TEXT NULL;

-- Add a new column 'type' of type TEXT to the 'chat.client' table, allowing NULL values.
ALTER TABLE chat.client ADD COLUMN "type" TEXT NULL;

-- Fix bug with incorrect type in chat.channel 'type' = 'messenger'. This referrer type was set to a broadcast.
UPDATE
    chat.channel ch
SET
    "type" = x.type
FROM LATERAL (
    SELECT
        x.type
    FROM
        chat.channel x
    WHERE
        x.user_id = ch.user_id
        AND x.closed_cause != 'broadcast_end'
        AND x.type != ch.type
    ORDER BY
        x.created_at DESC
    LIMIT 1
) x
WHERE
    ch.type = 'messenger';

-- Setting the correct values in the 'type' field in the chat.client table.
UPDATE
    chat.client cli
SET
    "type" = x.type
FROM LATERAL (
    SELECT
        x.type
    FROM
        chat.channel x
    WHERE
        x.user_id = cli.id
    ORDER BY
        x.created_at DESC
    LIMIT 1
) x;
