CREATE FUNCTION catalog_metadata_variant(metadata jsonb,settings jsonb) RETURNS jsonb
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT coalesce(
  CASE WHEN jsonb_typeof(metadata->coalesce(settings->>'metadata_language','en'))='object'
    THEN metadata->coalesce(settings->>'metadata_language','en') END,
  CASE WHEN jsonb_typeof(metadata->'')='object' THEN metadata->'' END,
  (SELECT value FROM jsonb_each(CASE WHEN jsonb_typeof(metadata)='object' THEN metadata ELSE '{}'::jsonb END)
    WHERE jsonb_typeof(value)='object' ORDER BY key LIMIT 1),
  '{}'::jsonb
 )
$$;
CREATE FUNCTION catalog_display_title(kind text,metadata jsonb,fallback text,settings jsonb) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT CASE WHEN kind='anime' THEN coalesce(
  (SELECT nullif(t->>'text','')
   FROM jsonb_array_elements(
    CASE WHEN jsonb_typeof(catalog_metadata_variant(metadata,settings)->'titles')='array'
      THEN catalog_metadata_variant(metadata,settings)->'titles' ELSE '[]'::jsonb END
   ) WITH ORDINALITY AS aliases(t,position)
   WHERE coalesce(t->>'text','')<>''
   ORDER BY CASE
    WHEN t->>'language'=CASE coalesce(nullif(settings->>'anime_title',''),'romaji')
      WHEN 'native' THEN 'ja' WHEN 'romaji' THEN 'x-jat' WHEN 'english' THEN 'en'
      ELSE coalesce(settings->>'metadata_language','en') END
      AND coalesce(t->>'type','') IN('official','main','') THEN 0
    WHEN t->>'type'='main' THEN 1
    WHEN t->>'language'='en' THEN 2 ELSE 3 END,position
   LIMIT 1),fallback)
 ELSE coalesce(nullif(catalog_metadata_variant(metadata,settings)->>'title',''),
               nullif(catalog_metadata_variant(metadata,settings)->>'name',''),fallback) END
$$;
