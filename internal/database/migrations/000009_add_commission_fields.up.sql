ALTER TABLE projects ADD COLUMN commission_rate_direct DECIMAL(5, 2) DEFAULT 4;
ALTER TABLE projects ADD COLUMN commission_rate_bank DECIMAL(5, 2) DEFAULT 6;
ALTER TABLE projects ADD COLUMN commission_rate_cash DECIMAL(5, 2) DEFAULT 7;

ALTER TABLE contracts ADD COLUMN commission_amount DECIMAL(15, 2) DEFAULT 0;
