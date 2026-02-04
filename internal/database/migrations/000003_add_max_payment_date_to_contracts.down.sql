-- Migration: Remove max_payment_date from contracts
ALTER TABLE contracts DROP COLUMN IF EXISTS max_payment_date;
