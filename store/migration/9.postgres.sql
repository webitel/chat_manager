-- Public name of the agent that will be displayed in chats
ALTER TABLE chat.channel
ADD COLUMN public_name TEXT NULL;
