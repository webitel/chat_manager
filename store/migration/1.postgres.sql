
create table chat.bot
(
	id bigint default nextval('chat.profile_id_seq'::regclass) not null
		constraint bot_pk
			primary key,
	dc bigint not null
		constraint bot_dc_fk
			references directory.wbt_domain
				on delete cascade,
	uri text not null,
	name text not null,
	flow_id bigint not null,
	enabled boolean default false not null,
	provider name not null,
	metadata jsonb,
	created_at timestamp default LOCALTIMESTAMP not null,
	created_by bigint not null
		constraint bot_created_id_fk
			references directory.wbt_user
				on delete set null,
	updated_at timestamp default LOCALTIMESTAMP not null,
	updated_by bigint not null
		constraint bot_updated_id_fk
			references directory.wbt_user
				on delete set null,
	constraint bot_flow_id_fk
		foreign key (flow_id, dc) references flow.acr_routing_scheme (id, domain_id),
	constraint bot_created_dc_fk
		foreign key (created_by, dc) references directory.wbt_user (id, dc),
	constraint bot_updated_dc_fk
		foreign key (updated_by, dc) references directory.wbt_user (id, dc)
);

-- alter table chat.bot
--    alter column id set default nextval('chat.profile_id_seq'::regclass);

comment on table chat.bot is 'Chat-BOT Profiles';

alter table chat.bot owner to opensips;

create unique index bot_uri_uindex
	on chat.bot (uri);

create unique index bot_dc
	on chat.bot (id) include (dc);

-------------------------- MIGRATE TABLES -----------------------------

with owner as (
  select dc, min(id) id from directory.wbt_user group by dc
)
insert into chat.bot (id, dc, uri, name, flow_id, enabled, provider, metadata, created_at, created_by, updated_at, updated_by)
select
    profile.id
  , domain_id   dc
  , '/'||url_id uri
  , name
  , schema_id   flow_id
  , true        enabled
  , type        provider
  , variables   metadata
  , created_at
  , owner.id    created_by
  , created_at  updated_at
  , owner.id    updated_by
from chat.profile
join owner on domain_id = dc
-- where
order by profile.id
;

-- alter table chat.profile rename to _profile;