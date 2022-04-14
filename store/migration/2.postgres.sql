
alter table chat.conversation
 	add props jsonb;

-- May result with an error
-- if column does not exist
-- but that's what we need !
alter table chat.channel
 	drop column vars;