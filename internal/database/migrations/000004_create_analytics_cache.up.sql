-- Migration: Ensure analytics_cache table exists (for DBs created before it was in initial schema)
CREATE TABLE IF NOT EXISTS analytics_cache (
    id BIGSERIAL PRIMARY KEY,
    cache_key VARCHAR(255) NOT NULL,
    project_id BIGINT,
    data JSONB NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_analytics_cache_key_project ON analytics_cache(cache_key, project_id);
CREATE INDEX IF NOT EXISTS idx_analytics_cache_expires_at ON analytics_cache(expires_at);
