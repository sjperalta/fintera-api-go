-- Migration: Remove analytics_cache (only if this migration was the one that created it)
DROP TABLE IF EXISTS analytics_cache CASCADE;
