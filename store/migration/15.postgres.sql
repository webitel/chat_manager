-- WTEL-10766: deleting a user clears Created by / Modified by instead of failing.

alter table chat.bot alter column created_by drop not null, alter column updated_by drop not null;

alter table chat.bot drop constraint bot_created_dc_fk;
alter table chat.bot
	add constraint bot_created_dc_fk
		foreign key (created_by, dc) references directory.wbt_user (id, dc)
			on delete set null (created_by)
			not valid;
alter table chat.bot validate constraint bot_created_dc_fk;

alter table chat.bot drop constraint bot_updated_dc_fk;
alter table chat.bot
	add constraint bot_updated_dc_fk
		foreign key (updated_by, dc) references directory.wbt_user (id, dc)
			on delete set null (updated_by)
			not valid;
alter table chat.bot validate constraint bot_updated_dc_fk;
