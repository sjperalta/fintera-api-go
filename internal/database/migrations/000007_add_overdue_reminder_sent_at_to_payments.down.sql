-- Migration: Remove overdue_reminder_sent_at from payments table
ALTER TABLE payments DROP COLUMN IF EXISTS overdue_reminder_sent_at;
