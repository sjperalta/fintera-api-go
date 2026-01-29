-- SQL Script to Close Contracts with Zero Balance
-- Run this to fix any contracts that are approved but have balance = 0

UPDATE contracts
SET 
    status = 'closed',
    closed_at = NOW(),
    active = FALSE,
    updated_at = NOW()
WHERE 
    status = 'approved' 
    AND balance <= 0
    AND closed_at IS NULL;

-- Show affected contracts
SELECT 
    c.id,
    c.status,
    c.balance,
    u.full_name as applicant_name,
    l.name as lot_name
FROM contracts c
JOIN users u ON c.applicant_user_id = u.id  
JOIN lots l ON c.lot_id = l.id
WHERE c.status = 'closed' AND c.closed_at IS NOT NULL
ORDER BY c.closed_at DESC
LIMIT 10;
