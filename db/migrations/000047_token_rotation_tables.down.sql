DROP INDEX IF EXISTS idx_token_rotation_events_result;
DROP INDEX IF EXISTS idx_token_rotation_events_source_time;
DROP TABLE IF EXISTS token_rotation_events;

DROP INDEX IF EXISTS uk_token_sources_org_provider_path;
DROP INDEX IF EXISTS idx_token_sources_next_check;
DROP INDEX IF EXISTS idx_token_sources_org_status;
DROP TABLE IF EXISTS token_sources;
