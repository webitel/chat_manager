update chat.bot
   set metadata = jsonb_set(coalesce(metadata, '{}'::jsonb), '{allow_origin}', '"*"'::jsonb)
 where provider = 'webchat'
   and coalesce(metadata ->> 'allow_origin', '') !~ '[^,[:space:]]';
