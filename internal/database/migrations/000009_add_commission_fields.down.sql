ALTER TABLE projects DROP COLUMN commission_rate_direct;
ALTER TABLE projects DROP COLUMN commission_rate_bank;
ALTER TABLE projects DROP COLUMN commission_rate_cash;

ALTER TABLE contracts DROP COLUMN commission_amount;
