-- Add guid column to contracts table
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS guid VARCHAR(255);

-- Generate UUIDs for existing contracts
-- Note: pgcrypto extension might be needed, or we can update via application if logic is complex.
-- Assuming pgcrypto is available or we can use a shim.
-- If pgcrypto is not enabled, we can't easily generate UUIDs in SQL without a function.
-- Let's check if we can use gen_random_uuid() (Postgres 13+) or uuid_generate_v4() (uuid-ossp).
-- Safest bet for consolidated migration without knowing PG version: 
-- 1. Add column nullable first
-- 2. Update existing rows (we might need to trust the app to backfill or use a best-effort random string if UUID functions aren't available)
-- However, FinteraAPI seems to be using standard Postgres. `gen_random_uuid()` is standard in PG 13+.

-- Attempt to use gen_random_uuid() if available, otherwise just leave null and let app handle it? 
-- No, user wants it mandatory. 
-- Let's assume PG 13+ or pgcrypto.

UPDATE contracts SET guid = gen_random_uuid()::text WHERE guid IS NULL;

-- Make column not null
ALTER TABLE contracts ALTER COLUMN guid SET NOT NULL;

-- Add unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_contracts_guid ON contracts(guid);
