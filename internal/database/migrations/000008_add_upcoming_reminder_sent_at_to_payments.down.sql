-- Migration: Remove upcoming_reminder_sent_at from payments table
ALTER TABLE payments DROP COLUMN IF EXISTS upcoming_reminder_sent_at;
