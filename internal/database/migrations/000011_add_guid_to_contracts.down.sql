-- Remove unique index
DROP INDEX IF EXISTS idx_contracts_guid;

-- Remove guid column
ALTER TABLE contracts DROP COLUMN IF EXISTS guid;
