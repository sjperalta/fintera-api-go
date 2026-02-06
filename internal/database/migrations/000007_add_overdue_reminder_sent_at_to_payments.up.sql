-- Migration: Track when overdue payment reminder email was last sent (avoids spamming users)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS overdue_reminder_sent_at TIMESTAMP;
