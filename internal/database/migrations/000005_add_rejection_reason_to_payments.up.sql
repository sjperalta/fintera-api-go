-- Migration: Add rejection_reason to payments table (optional reason when admin rejects a payment)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS rejection_reason TEXT;
