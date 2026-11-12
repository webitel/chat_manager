-- WTEL-10766: deleting a user clears its references (Created by, Modified by, ...) instead of failing.

do $$
declare
    r    record;
    ucol name;
    def  text;
begin
    for r in
        select c.conname, c.conrelid, c.conrelid::regclass as tbl, c.conkey, c.confkey,
               pg_get_constraintdef(c.oid) as def
          from pg_constraint c
          join pg_class t on t.oid = c.conrelid
          join pg_namespace n on n.oid = t.relnamespace
         where c.confrelid = 'directory.wbt_user'::regclass
           and c.contype = 'f'
           and c.confdeltype in ('a', 'r')
           and n.nspname = 'chat'
    loop
        select a.attname into ucol
          from unnest(r.conkey, r.confkey) as k(col, refcol)
          join pg_attribute a on a.attrelid = r.conrelid and a.attnum = k.col
          join pg_attribute ra on ra.attrelid = 'directory.wbt_user'::regclass and ra.attnum = k.refcol
         where ra.attname = 'id';

        if ucol is null then
            continue;
        end if;

        if ucol in ('created_by', 'updated_by', 'deleted_by') then
            execute format('alter table %s alter column %I drop not null', r.tbl, ucol);
        end if;

        def := regexp_replace(r.def, ' NOT VALID$', '');
        def := regexp_replace(def, ' ON DELETE (NO ACTION|RESTRICT)', '');
        def := regexp_replace(def,
            '^(FOREIGN KEY \([^)]*\) REFERENCES [^(]+\([^)]*\)( MATCH \w+)?)',
            '\1 ON DELETE SET NULL (' || quote_ident(ucol) || ')');

        execute format('alter table %s drop constraint %I', r.tbl, r.conname);
        execute format('alter table %s add constraint %I %s not valid', r.tbl, r.conname, def);
        execute format('alter table %s validate constraint %I', r.tbl, r.conname);
    end loop;
end
$$;
