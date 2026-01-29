-- Consolidated Migration for FinteraAPI Database
-- This migration creates all tables and indexes needed for the application

-- ============================================
-- USERS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    encrypted_password VARCHAR(255) NOT NULL,
    reset_password_token VARCHAR(255),
    reset_password_sent_at TIMESTAMP,
    remember_created_at TIMESTAMP,
    confirmation_token VARCHAR(255),
    confirmed_at TIMESTAMP,
    confirmation_sent_at TIMESTAMP,
    unconfirmed_email VARCHAR(255),
    role VARCHAR(50) DEFAULT 'user',
    full_name VARCHAR(255),
    phone VARCHAR(50),
    status VARCHAR(50) DEFAULT 'active',
    identity VARCHAR(255) UNIQUE,
    rtn VARCHAR(255) UNIQUE,
    discarded_at TIMESTAMP,
    recovery_code VARCHAR(255),
    recovery_code_sent_at TIMESTAMP,
    address VARCHAR(255),
    created_by BIGINT,
    note VARCHAR(255),
    credit_score INTEGER DEFAULT 0,
    locale VARCHAR(10) DEFAULT 'es',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_discarded_at ON users(discarded_at);
CREATE INDEX IF NOT EXISTS idx_users_rtn ON users(rtn);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_identity ON users(identity);

-- ============================================
-- REFRESH TOKENS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token VARCHAR(255),
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- ============================================
-- PROJECTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    project_type VARCHAR(50) DEFAULT 'residential',
    address VARCHAR(255) NOT NULL,
    lot_count INTEGER NOT NULL,
    price_per_square_unit NUMERIC(10,2) NOT NULL,
    interest_rate NUMERIC(5,2) NOT NULL,
    guid VARCHAR(255) NOT NULL,
    commission_rate NUMERIC(5,2) DEFAULT 0,
    measurement_unit VARCHAR(20) DEFAULT 'm2',
    delivery_date DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- LOTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS lots (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'available',
    length NUMERIC(10,2) NOT NULL,
    width NUMERIC(10,2) NOT NULL,
    price NUMERIC(15,2) NOT NULL,
    address VARCHAR(255),
    measurement_unit VARCHAR(20),
    override_price NUMERIC(15,2),
    registration_number VARCHAR(255),
    note TEXT,
    override_area NUMERIC(10,2),
    north TEXT,
    east TEXT,
    west TEXT,
    south TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_lots_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_lots_project_id ON lots(project_id);
CREATE INDEX IF NOT EXISTS idx_lots_status ON lots(status);
CREATE INDEX IF NOT EXISTS idx_lots_registration_number ON lots(registration_number);

-- ============================================
-- CONTRACTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS contracts (
    id BIGSERIAL PRIMARY KEY,
    lot_id BIGINT NOT NULL,
    creator_id BIGINT,
    applicant_user_id BIGINT NOT NULL,
    payment_term INTEGER NOT NULL,
    financing_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    amount NUMERIC(15,2),
    balance NUMERIC(15,2),
    down_payment NUMERIC(15,2),
    reserve_amount NUMERIC(15,2),
    currency VARCHAR(10) DEFAULT 'HNL' NOT NULL,
    approved_at TIMESTAMP,
    active BOOLEAN DEFAULT FALSE,
    note TEXT,
    rejection_reason TEXT,
    document_paths TEXT,
    closed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_contracts_lot FOREIGN KEY (lot_id) REFERENCES lots(id),
    CONSTRAINT fk_contracts_creator FOREIGN KEY (creator_id) REFERENCES users(id),
    CONSTRAINT fk_contracts_applicant FOREIGN KEY (applicant_user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_contracts_lot_id ON contracts(lot_id);
CREATE INDEX IF NOT EXISTS idx_contracts_creator_id ON contracts(creator_id);
CREATE INDEX IF NOT EXISTS idx_contracts_applicant_user_id ON contracts(applicant_user_id);
CREATE INDEX IF NOT EXISTS idx_contracts_status ON contracts(status);
CREATE INDEX IF NOT EXISTS idx_contracts_active ON contracts(active);
CREATE INDEX IF NOT EXISTS idx_contracts_approved_at ON contracts(approved_at);

-- ============================================
-- PAYMENTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    contract_id BIGINT NOT NULL,
    amount NUMERIC(10,2) NOT NULL,
    paid_amount NUMERIC(15,2) DEFAULT 0,
    due_date DATE NOT NULL,
    payment_date DATE,
    status VARCHAR(50) DEFAULT 'pending' NOT NULL,
    payment_type VARCHAR(50) DEFAULT 'installment',
    description VARCHAR(255),
    interest_amount NUMERIC(10,2),
    approved_at TIMESTAMP,
    document_path VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_payments_contract FOREIGN KEY (contract_id) REFERENCES contracts(id)
);
CREATE INDEX IF NOT EXISTS idx_payments_contract_id ON payments(contract_id);
CREATE INDEX IF NOT EXISTS idx_payments_due_date ON payments(due_date);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_approved_at ON payments(approved_at);

-- ============================================
-- CONTRACT LEDGER ENTRIES TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS contract_ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    contract_id BIGINT NOT NULL,
    payment_id BIGINT,
    amount NUMERIC(15,2) NOT NULL,
    description VARCHAR(255) NOT NULL,
    entry_type VARCHAR(50) NOT NULL,
    entry_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_ledger_contract FOREIGN KEY (contract_id) REFERENCES contracts(id),
    CONSTRAINT fk_ledger_payment FOREIGN KEY (payment_id) REFERENCES payments(id)
);
CREATE INDEX IF NOT EXISTS idx_ledger_contract_id ON contract_ledger_entries(contract_id);
CREATE INDEX IF NOT EXISTS idx_ledger_payment_id ON contract_ledger_entries(payment_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entry_type ON contract_ledger_entries(entry_type);

-- ============================================
-- NOTIFICATIONS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    message VARCHAR(255) NOT NULL,
    notification_type VARCHAR(50),
    read_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_notification_type ON notifications(notification_type);
CREATE INDEX IF NOT EXISTS idx_notifications_read_at ON notifications(read_at);

-- ============================================
-- AUDIT LOGS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL,
    entity VARCHAR(50) NOT NULL,
    entity_id BIGINT,
    details TEXT,
    ip_address VARCHAR(45),
    user_agent VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_audit_logs_user FOREIGN KEY (user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- ============================================
-- ANALYTICS CACHE TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS analytics_cache (
    id BIGSERIAL PRIMARY KEY,
    cache_key VARCHAR(255) NOT NULL,
    project_id BIGINT,
    data JSONB NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_analytics_cache_key_project ON analytics_cache(cache_key, project_id);
CREATE INDEX IF NOT EXISTS idx_analytics_cache_expires_at ON analytics_cache(expires_at);
