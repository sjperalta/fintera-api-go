-- Migration: Track when "payment due tomorrow" reminder was sent (one reminder per payment)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS upcoming_reminder_sent_at TIMESTAMP;
