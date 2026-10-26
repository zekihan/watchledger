-- Episode inventories and lifecycle status are source facts, not translations.
CREATE FUNCTION catalog_progress_metadata(catalog_key text) RETURNS jsonb
LANGUAGE sql STABLE PARALLEL SAFE AS $$
 SELECT coalesce(
  (SELECT l.metadata FROM catalog_localizations l WHERE l.catalog_id=c.id
   ORDER BY l.fetched_at DESC,l.language LIMIT 1),
  c.metadata->'',c.metadata->'en',
  (SELECT value FROM jsonb_each(c.metadata) ORDER BY key LIMIT 1),
  '{}'::jsonb
 ) FROM catalog c WHERE c.id=catalog_key
$$;
