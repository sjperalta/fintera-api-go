-- Migration: Add max_payment_date to contracts (for bank/cash: date by which customer will pay the rest)
ALTER TABLE contracts ADD COLUMN IF NOT EXISTS max_payment_date DATE;
