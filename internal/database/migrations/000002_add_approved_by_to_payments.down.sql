-- Migration: Remove approved_by_user_id from payments table
ALTER TABLE payments DROP CONSTRAINT IF EXISTS fk_payments_approved_by;
DROP INDEX IF EXISTS idx_payments_approved_by_user_id;
ALTER TABLE payments DROP COLUMN IF EXISTS approved_by_user_id;
