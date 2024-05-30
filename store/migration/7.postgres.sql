alter table chat.bot
    alter column flow_id drop not null;

alter table chat.bot
    alter column flow_id set default null;