-- Migration: Add approved_by_user_id to payments table
ALTER TABLE payments ADD COLUMN IF NOT EXISTS approved_by_user_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_payments_approved_by_user_id ON payments(approved_by_user_id);
ALTER TABLE payments ADD CONSTRAINT fk_payments_approved_by FOREIGN KEY (approved_by_user_id) REFERENCES users(id);
