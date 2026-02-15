-- Seed: Contract with Overdue Payments for testing the overdue schedule process
-- This creates an approved contract for Antonio Perez with payments that are past due.
-- Run after seeds.sql: psql "$DATABASE_URL" -f internal/database/seeds/overdue_contract_seed.sql

DO $$
DECLARE
    v_contract_id BIGINT;
    v_lot_id BIGINT;
    v_user_id BIGINT;
    v_seller_id BIGINT;
    v_project_id BIGINT;
    v_amount NUMERIC := 750000.00;
    v_reserve NUMERIC := 30000.00;
    v_down NUMERIC := 120000.00;
    v_monthly NUMERIC;
    v_approved_date TIMESTAMP := NOW() - INTERVAL '6 months';
BEGIN
    -- Get Portal Mar de Plata project
    SELECT id INTO v_project_id FROM projects WHERE name = 'Portal Mar de Plata' LIMIT 1;

    IF v_project_id IS NULL THEN
        RAISE NOTICE 'Project Portal Mar de Plata not found. Skipping overdue seed.';
        RETURN;
    END IF;

    -- Create lot only if it does not exist (idempotent)
    INSERT INTO lots (project_id, name, length, width, price, status, created_at, updated_at)
    SELECT v_project_id, 'Lote 99', 20.00, 15.00, 750000.00, 'available', NOW(), NOW()
    WHERE NOT EXISTS (SELECT 1 FROM lots WHERE project_id = v_project_id AND name = 'Lote 99')
    RETURNING id INTO v_lot_id;

    -- If lot already existed, fetch it
    IF v_lot_id IS NULL THEN
        SELECT id INTO v_lot_id FROM lots WHERE project_id = v_project_id AND name = 'Lote 99' LIMIT 1;
    END IF;

    SELECT id INTO v_user_id FROM users WHERE email = 'antonio.perez@gmail.com' LIMIT 1;
    SELECT id INTO v_seller_id FROM users WHERE email = 'vendedor@example.com' LIMIT 1;

    IF v_user_id IS NULL OR v_seller_id IS NULL THEN
        RAISE NOTICE 'Required users not found. Skipping overdue seed.';
        RETURN;
    END IF;

    -- Check if this specific overdue contract already exists
    SELECT id INTO v_contract_id FROM contracts WHERE lot_id = v_lot_id AND applicant_user_id = v_user_id LIMIT 1;

    IF v_contract_id IS NOT NULL THEN
        RAISE NOTICE 'Overdue contract for Antonio on Lote 99 already exists (ID: %). Skipping.', v_contract_id;
        RETURN;
    END IF;

    v_monthly := ROUND((v_amount - v_reserve - v_down) / 12.0, 2);

    -- Create approved contract (approved 6 months ago)
    INSERT INTO contracts (lot_id, creator_id, applicant_user_id, payment_term, financing_type, reserve_amount, down_payment,
                          status, currency, amount, balance, approved_at, active, created_at, updated_at)
    VALUES (v_lot_id, v_seller_id, v_user_id, 12, 'direct', v_reserve, v_down,
            'approved', 'HNL', v_amount, 0, v_approved_date, TRUE, v_approved_date, NOW())
    RETURNING id INTO v_contract_id;

    -- Mark lot as reserved
    UPDATE lots SET status = 'reserved' WHERE id = v_lot_id;

    -- Initial ledger entry (debt)
    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, -v_amount, 'Monto Inicial del Contrato', 'initial', v_approved_date, v_approved_date, v_approved_date);

    -- Reserve payment (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (v_contract_id, v_reserve, v_reserve, v_approved_date, v_approved_date, 'paid', 'reservation', 'Pago de Reserva', v_approved_date, NOW(), NOW());

    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, v_reserve, 'Pago de Reserva', 'reservation', v_approved_date, v_approved_date, v_approved_date);

    -- Down payment (PAID)
    INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
    VALUES (v_contract_id, v_down, v_down, v_approved_date + INTERVAL '7 days', v_approved_date + INTERVAL '7 days', 'paid', 'down_payment', 'Prima/Enganche', v_approved_date + INTERVAL '7 days', NOW(), NOW());

    INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
    VALUES (v_contract_id, v_down, 'Prima/Enganche', 'down_payment', v_approved_date + INTERVAL '7 days', v_approved_date + INTERVAL '7 days', v_approved_date + INTERVAL '7 days');

    -- 12 monthly installments:
    --   Months 1-2: PAID (4-5 months ago)
    --   Months 3-6: OVERDUE (due 1-4 months ago, still pending)
    --   Months 7-12: PENDING (future due dates)
    FOR i IN 1..12 LOOP
        DECLARE
            v_due DATE := (v_approved_date + (i || ' months')::INTERVAL)::DATE;
            v_is_paid BOOLEAN := (i <= 2);
            v_is_overdue BOOLEAN := (i > 2 AND v_due < CURRENT_DATE);
        BEGIN
            INSERT INTO payments (contract_id, amount, paid_amount, due_date, payment_date, status, payment_type, description, approved_at, created_at, updated_at)
            VALUES (
                v_contract_id,
                v_monthly,
                CASE WHEN v_is_paid THEN v_monthly ELSE 0 END,
                v_due,
                CASE WHEN v_is_paid THEN v_due ELSE NULL END,
                CASE
                    WHEN v_is_paid THEN 'paid'
                    WHEN v_is_overdue THEN 'pending'  -- The overdue job will mark these as overdue
                    ELSE 'pending'
                END,
                'installment',
                'Cuota Mensual #' || i,
                CASE WHEN v_is_paid THEN v_due::TIMESTAMP ELSE NULL END,
                NOW(), NOW()
            );

            -- Ledger entries only for paid installments
            IF v_is_paid THEN
                INSERT INTO contract_ledger_entries (contract_id, amount, description, entry_type, entry_date, created_at, updated_at)
                VALUES (v_contract_id, v_monthly, 'Pago Cuota #' || i, 'payment', v_due, v_due, v_due);
            END IF;
        END;
    END LOOP;

    -- Update contract balance: -amount + reserve + down + (2 paid installments)
    UPDATE contracts SET balance = (-v_amount + v_reserve + v_down + (v_monthly * 2)) WHERE id = v_contract_id;

    RAISE NOTICE 'Created overdue contract ID: %, with 4 overdue payments for user antonio.perez@gmail.com', v_contract_id;
END $$;
