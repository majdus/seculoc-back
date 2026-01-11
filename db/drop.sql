-- =============================================
-- DROP EVERYTHING (RESET)
-- =============================================

-- 1. Views
DROP VIEW IF EXISTS view_user_credit_balance CASCADE;

-- 2. Tables (Ordre inverse de création pour respecter les FK, ou CASCADE)
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS seasonal_bookings CASCADE;
DROP TABLE IF EXISTS rent_payments CASCADE;
DROP TABLE IF EXISTS leases CASCADE;
DROP TABLE IF EXISTS solvency_checks CASCADE;
DROP TABLE IF EXISTS properties CASCADE;
DROP TABLE IF EXISTS credit_transactions CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 3. Types (Enums)
DROP TYPE IF EXISTS escrow_status CASCADE;
DROP TYPE IF EXISTS solvency_status CASCADE;
DROP TYPE IF EXISTS billing_freq CASCADE;
DROP TYPE IF EXISTS sub_plan CASCADE;
DROP TYPE IF EXISTS property_type CASCADE;
DROP TYPE IF EXISTS user_role CASCADE;
