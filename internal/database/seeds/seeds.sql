-- Enhanced Seed Data for FinteraAPI - Realistic Contract Data
-- Password for all users: "password123"

-- ============================================
-- 1. USERS
-- ============================================

INSERT INTO users (email, encrypted_password, role, full_name, phone, identity, rtn, credit_score, confirmed_at, status, created_at, updated_at)
VALUES 
('admin@example.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'admin', 'Administrador', '+50431848112', '0000000000000', '00000000000000', 100, NOW(), 'active', NOW(), NOW()),
('vendedor@example.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'seller', 'Juan Perez', '+50498586221', '0506199100444', '05061991004441', 85, NOW(), 'active', NOW(), NOW()),
('cristofer@gmail.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'seller', 'Cristofer Hernandez', '7863346828', '0412199500777', '04121995007771', 92, NOW(), 'active', NOW(), NOW()),
('alexander.ramos@example.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'user', 'Alexander Ramos', '98765432', '0304199200567', '03041992005671', 88, NOW(), 'active', NOW(), NOW()),
('milton@example.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'user', 'Milton Suazo', '31848112', '0612199000234', '06121990002341', 82, NOW(), 'active', NOW(), NOW()),
('antonio.perez@gmail.com', '$2a$10$fztkch9sFwGAczx1lOpDBeDGqgR//wM.WcHKp08mA3OC1Db9ufW9i', 'user', 'Antonio Perez Martinez', '318481123', '0801198500123', '08011985001231', 75, NOW(), 'active', NOW(), NOW())
ON CONFLICT (email) DO NOTHING;

-- ============================================
-- 2. PROJECTS
-- ============================================

INSERT INTO projects (name, description, project_type, address, lot_count, price_per_square_unit, interest_rate, guid, commission_rate, measurement_unit, created_at, updated_at)
VALUES 
('Portal Mar de Plata', 'Proyecto residencial frente al mar en Colonia Costarica, Omoa.', 'residential', 'Colonia Costarica, Omoa', 5, 2500.00, 5.00, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 5.00, 'm2', NOW(), NOW()),
('Ciudad Jaguar', 'Residencial exclusivo en Cieneguita, Puerto Cortes.', 'residential', 'Cieneguita, Puerto Cortes, Honduras', 5, 2500.00, 5.00, 'b1ffcd88-8d0a-3ff7-aa5c-5cc8bd270b22', 5.00, 'm2', NOW(), NOW())
ON CONFLICT DO NOTHING;

-- ============================================
-- 3. LOTS
-- ============================================

-- Portal Mar de Plata lots
WITH p AS (SELECT id FROM projects WHERE name = 'Portal Mar de Plata' LIMIT 1)
INSERT INTO lots (project_id, name, length, width, price, status, created_at, updated_at)
SELECT p.id, 'Lote ' || i, 20.00, 15.00, 750000.00, 
       CASE WHEN i = 1 THEN 'fully_paid' WHEN i = 2 THEN 'reserved' ELSE 'available' END,
       NOW(), NOW()
FROM p, generate_series(1, 5) i
WHERE NOT EXISTS (SELECT 1 FROM lots l WHERE l.name = 'Lote ' || i AND l.project_id = p.id);

-- Ciudad Jaguar lots  
WITH p AS (SELECT id FROM projects WHERE name = 'Ciudad Jaguar' LIMIT 1)
INSERT INTO lots (project_id, name, length, width, price, status, created_at, updated_at)
SELECT p.id, 'Lote ' || (i + 10), 25.00, 18.00, 1125000.00,
       CASE WHEN i = 1 THEN 'financed' ELSE 'available' END,
       NOW(), NOW()
FROM p, generate_series(1, 5) i
WHERE NOT EXISTS (SELECT 1 FROM lots l WHERE l.name = 'Lote ' || (i + 10) AND l.project_id = p.id);

-- ============================================
-- 4. CONTRACTS WITH PAYMENT SCHEDULES & LEDGER
-- ============================================

-- Contract 1: Alexander Ramos - APPROVED, with ongoing payments
DO $$
DECLARE
    v_contract_id BIGINT;
    v_lot_id BIGINT;
    v_user_id BIGINT;
    v_seller_id BIGINT;
    v_amount NUMERIC := 1125000.00;
    v_reserve NUMERIC := 50000.00;
    v_down NUMERIC := 250000.00;
    v_balance NUMERIC;
    v_monthly NUMERIC;
    v_approved_date TIMESTAMP := NOW() - INTERVAL '3 months';
BEGIN
    -- Get IDs
    SELECT id INTO v_lot_id FROM lots WHERE name = 'Lote 11' LIMIT 1;
    SELECT id INTO v_user_id FROM users WHERE email = 'alexander.ramos@example.com' LIMIT 1;
    SELECT id INTO v_seller_id FROM users WHERE email = 'cristofer@gmail.com' LIMIT 1;
    
    -- Create contract
    INSERT INTO contracts (lot_id, creator_id, applicant_user_id, payment_term, financing_type, reserve_amount, down_payment, 
                          status, currency, amount, balance, approved_at, active, created_at, updated_at)
    VALUES (v_lot_id, v_seller_id, v_user_id, 24, 'direct', v_reserve, v_down, 
            'approved', 'HNL', v_amount, 0, v_approved_date, TRUE, v_approved_date, NOW())
    RETURNING id INTO v_contract_id;
    
    -- Initial ledger entry
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, -v_amount, 'Monto Inicial del Contrato', 'initial', v_approved_date, v_approved_date, v_approved_date);
    
    -- Reserve payment ledger
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, v_reserve, 'Pago de Reserva', 'reservation', v_approved_date, v_approved_date, v_approved_date);
    
    -- Down payment ledger
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, v_down, 'Prima/Enganche', 'down_payment', v_approved_date, v_approved_date, v_approved_date);
    
    -- Calculate monthly payment
    v_monthly := (v_amount - v_reserve - v_down) / 24.0;
    
    -- Payment 1: Reserve (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (
        v_contract_id,
        v_reserve,
        v_reserve,
        v_approved_date,
        v_approved_date,
        'paid',
        'reservation',
        'Pago de Reserva',
        v_approved_date,
        NOW(), NOW()
    );
    
    -- Payment 2: Down Payment (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (
        v_contract_id,
        v_down,
        v_down,
        v_approved_date + INTERVAL '7 days',
        v_approved_date + INTERVAL '7 days',
        'paid',
        'down_payment',
        'Prima/Enganche',
        v_approved_date + INTERVAL '7 days',
        NOW(), NOW()
    );
    
    -- Payments 3-26: Monthly installments (first 3 paid, rest pending)
    FOR i IN 1..24 LOOP
        INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
        VALUES (
            v_contract_id,
            v_monthly,
            CASE WHEN i <= 3 THEN v_monthly ELSE 0 END,
            v_approved_date + (i || ' months')::INTERVAL,
            CASE WHEN i <= 3 THEN v_approved_date + (i || ' months')::INTERVAL ELSE NULL END,
            CASE WHEN i <= 3 THEN 'paid' ELSE 'pending' END,
            'installment',
            'Cuota Mensual #' || i,
            CASE WHEN i <= 3 THEN v_approved_date + (i || ' months')::INTERVAL ELSE NULL END,
            NOW(), NOW()
        );
        
        -- Ledger for paid installments
        IF i <= 3 THEN
            INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
            VALUES (v_contract_id, v_monthly, 'Pago Cuota #' || i, 'payment', 
                   v_approved_date + (i || ' months')::INTERVAL,
                   v_approved_date + (i || ' months')::INTERVAL, 
                   v_approved_date + (i || ' months')::INTERVAL);
        END IF;
    END LOOP;
    
    -- Calculate final balance: -amount + reserve + down_payment + (3 monthly payments) (debt is negative)
    -- Logic matches ContractService: Balance starts at -Amount and increases with payments
    v_balance := -v_amount + v_reserve + v_down + (v_monthly * 3);
    
    -- Update contract balance
    UPDATE contracts SET balance = v_balance WHERE id = v_contract_id;
END $$;

-- Contract 2: Milton Suazo - CLOSED (fully paid, balance = 0)
DO $$
DECLARE
    v_contract_id BIGINT;
    v_lot_id BIGINT;
    v_user_id BIGINT;
    v_seller_id BIGINT;
    v_amount NUMERIC := 750000.00;
    v_reserve NUMERIC := 50000.00;
    v_down NUMERIC := 150000.00;
    v_monthly NUMERIC;
    v_approved_date TIMESTAMP := NOW() - INTERVAL '14 months';
    v_closed_date TIMESTAMP := NOW() - INTERVAL '2 months';
BEGIN
    SELECT id INTO v_lot_id FROM lots WHERE name = 'Lote 1' LIMIT 1;
    SELECT id INTO v_user_id FROM users WHERE email = 'milton@example.com' LIMIT 1;
    SELECT id INTO v_seller_id FROM users WHERE email = 'vendedor@example.com' LIMIT 1;
    
    -- Create contract WITH balance = 0 and status = closed
    INSERT INTO contracts (lot_id, creator_id, applicant_user_id, payment_term, financing_type, reserve_amount, down_payment,
                          status, currency, amount, balance, approved_at, active, closed_at, created_at, updated_at)
    VALUES (v_lot_id, v_seller_id, v_user_id, 12, 'direct', v_reserve, v_down,
            'closed', 'HNL', v_amount, 0, v_approved_date, FALSE, v_closed_date, v_approved_date, NOW())
    RETURNING id INTO v_contract_id;
    
    -- Initial ledger
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, -v_amount, 'Monto Inicial del Contrato', 'initial', v_approved_date, v_approved_date, v_approved_date);
    
    -- Reserve & down payment ledgers
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES
        (v_contract_id, v_reserve, 'Pago de Reserva', 'reservation', v_approved_date, v_approved_date, v_approved_date),
        (v_contract_id, v_down, 'Prima/Enganche', 'down_payment', v_approved_date, v_approved_date, v_approved_date);

    -- Payment 1: Reserve (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (
        v_contract_id,
        v_reserve,
        v_reserve,
        v_approved_date,
        v_approved_date,
        'paid',
        'reservation',
        'Pago de Reserva',
        v_approved_date,
        NOW(), NOW()
    );
    
    -- Payment 2: Down Payment (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (
        v_contract_id,
        v_down,
        v_down,
        v_approved_date + INTERVAL '7 days',
        v_approved_date + INTERVAL '7 days',
        'paid',
        'down_payment',
        'Prima/Enganche',
        v_approved_date + INTERVAL '7 days',
        NOW(), NOW()
    );
    
    v_monthly := FLOOR((v_amount - v_reserve - v_down) / 12.0);
    
    -- All 12 payments PAID
    FOR i IN 1..12 LOOP
        DECLARE
            v_payment_amount NUMERIC;
        BEGIN
            -- First payment gets the remainder to avoid cents in others
            IF i = 1 THEN
                 v_payment_amount := (v_amount - v_reserve - v_down) - (v_monthly * 11);
            ELSE
                 v_payment_amount := v_monthly;
            END IF;
            
            INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
            VALUES (
                v_contract_id,
                v_payment_amount,
                v_payment_amount,
                v_approved_date + (i || ' months')::INTERVAL,
                v_approved_date + (i || ' months')::INTERVAL,
                'paid',
                'installment',
                'Cuota Mensual #' || i,
                v_approved_date + (i || ' months')::INTERVAL,
                NOW(), NOW()
            );
            
            INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
            VALUES (v_contract_id, v_payment_amount, 'Pago Cuota #' || i, 'payment',
                   v_approved_date + (i || ' months')::INTERVAL,
                   v_approved_date + (i || ' months')::INTERVAL,
                   v_approved_date + (i || ' months')::INTERVAL);
        END;
    END LOOP;
END $$;

-- Contract 3: Antonio Perez - PENDING (no payments yet)
DO $$
DECLARE
    v_contract_id BIGINT;
    v_lot_id BIGINT;
    v_user_id BIGINT;
    v_amount NUMERIC := 750000.00;
BEGIN
    SELECT id INTO v_lot_id FROM lots WHERE name = 'Lote 2' LIMIT 1;
    SELECT id INTO v_user_id FROM users WHERE email = 'antonio.perez@gmail.com' LIMIT 1;
    
    -- Initial balance is negative total amount (debt)
    INSERT INTO contracts (lot_id, applicant_user_id, payment_term, financing_type, reserve_amount, down_payment,
                          status, currency, amount, balance, created_at, updated_at)
    VALUES (v_lot_id, v_user_id, 18, 'direct', 50000.00, 100000.00,
            'pending', 'HNL', v_amount, -v_amount, NOW() - INTERVAL '5 days', NOW())
    RETURNING id INTO v_contract_id;
    
    -- No payments or ledger entries for pending contracts
END $$;

-- ============================================
-- 5. NOTIFICATIONS
-- ============================================

INSERT INTO notifications (user_id, title, message, notification_type, created_at, updated_at)
SELECT 
    (SELECT id FROM users WHERE role = 'admin' LIMIT 1),
    'Nueva solicitud de contrato',
    'Antonio Perez Martinez ha solicitado un contrato',
    'contract_pending',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '5 days';

INSERT INTO notifications (user_id, title, message, notification_type, read_at, created_at, updated_at)
SELECT 
    (SELECT id FROM users WHERE email = 'alexander.ramos@example.com'),
    'Contrato aprobado',
    'Tu solicitud de contrato ha sido aprobada',
    'contract_approved',
    NOW() - INTERVAL '2 months',
    NOW() - INTERVAL '3 months',
    NOW() - INTERVAL '2 months';
