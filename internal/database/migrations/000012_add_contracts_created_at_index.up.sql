-- Add index on contracts(created_at) for list default sort and date filters (start_date/end_date)
CREATE INDEX IF NOT EXISTS idx_contracts_created_at ON contracts(created_at);
