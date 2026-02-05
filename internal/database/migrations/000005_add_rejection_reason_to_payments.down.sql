-- Migration: Remove rejection_reason from payments table
ALTER TABLE payments DROP COLUMN IF EXISTS rejection_reason;
