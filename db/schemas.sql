-- =============================================
-- 1. ENUMS & TYPES (Pour la rigidité des données)
-- =============================================

CREATE TYPE user_role AS ENUM ('admin', 'user'); -- Rôle système. Le métier (Bailleur/Locataire) est contextuel.
CREATE TYPE property_type AS ENUM ('long_term', 'seasonal'); -- [cite: 5]
CREATE TYPE sub_plan AS ENUM ('discovery', 'serenity', 'premium'); -- [cite: 31, 37, 46]
CREATE TYPE billing_freq AS ENUM ('monthly', 'yearly'); -- [cite: 39]
CREATE TYPE solvency_status AS ENUM ('pending', 'approved', 'rejected', 'insufficient_docs', 'cancelled');
CREATE TYPE escrow_status AS ENUM ('held', 'released', 'disputed', 'refunded'); -- [cite: 27, 58]

-- =============================================
-- 2. UTILISATEURS & AUTHENTIFICATION
-- =============================================

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255), -- Nullable for provisional accounts
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    phone_number VARCHAR(20),
    is_verified BOOLEAN DEFAULT FALSE, -- KYC de l'utilisateur lui-même
    stripe_customer_id VARCHAR(100), -- Pour les prélèvements abonnements/packs
    is_provisional BOOLEAN DEFAULT TRUE,
    last_context_used VARCHAR(50) DEFAULT 'owner', -- 'owner' or 'tenant'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Note : Pas de table séparée "Propriétaire" vs "Locataire".
-- Un user est "Propriétaire" s'il a une entrée dans la table 'properties'.
-- Un user est "Locataire" s'il a une entrée dans 'leases' ou 'bookings'.

-- =============================================
-- 3. ABONNEMENTS & CRÉDITS (Le "Cœur" du Business Model)
-- =============================================

CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    plan_type sub_plan NOT NULL, -- Discovery (Gratuit), Serenity (9.90), Premium (29.90)
    frequency billing_freq, -- Mensuel ou Annuel (2 mois offerts gérés par le billing engine)
    status VARCHAR(50) DEFAULT 'active', -- active, cancelled, past_due
    start_date DATE NOT NULL,
    end_date DATE, -- Null si renouvellement auto, ou date de fin prépayée
    max_properties_limit INT DEFAULT 0, -- 0 (Free), 1 (Serenity), 5 (Premium) [cite: 51]
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- TABLE CRITIQUE : Gestion des Crédits de Solvabilité ("Ledger")
-- Plutôt qu'un compteur simple, on utilise un journal de transactions pour gérer les packs et les resets mensuels.
CREATE TABLE credit_transactions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    amount INT NOT NULL, -- Positif (ajout) ou Négatif (consommation)
    transaction_type VARCHAR(50) NOT NULL, 
    -- Types: 'plan_renewal' (+20/+30), 'pack_purchase' (+20), 'check_usage' (-1), 'initial_free' (+3)
    description TEXT, -- Ex: "Pack Achat à l'acte", "Renouvellement Mensuel Premium"
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Vue pour obtenir le solde actuel instantanément
CREATE VIEW view_user_credit_balance AS
SELECT user_id, SUM(amount) as current_balance
FROM credit_transactions
GROUP BY user_id;

-- =============================================
-- 4. GESTION DES BIENS (LOGEMENTS)
-- =============================================

CREATE TABLE properties (
    id SERIAL PRIMARY KEY,
    owner_id INT REFERENCES users(id),
    name TEXT, -- Titre de l'annonce (ex: "Joli Studio")
    address TEXT NOT NULL,
    rental_type property_type NOT NULL, -- Longue durée ou Saisonnier [cite: 5]
    details JSONB, -- Surface, nbr pièces, description
    rent_amount DECIMAL(10, 2),
    rent_charges_amount DECIMAL(10, 2) DEFAULT 0, -- Provisions sur charges
    deposit_amount DECIMAL(10, 2),
    is_furnished BOOLEAN DEFAULT FALSE, -- Meublé ou Non
    seasonal_price_per_night DECIMAL(10, 2), -- Prix nuitée saisonnier
    vacancy_credits INTEGER NOT NULL DEFAULT 20,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 5. MODULE VÉRIFICATION SOLVABILITÉ (Check Locataire)
-- =============================================

CREATE TABLE solvency_checks (
    id SERIAL PRIMARY KEY,
    initiator_owner_id INT REFERENCES users(id), -- Celui qui consomme le crédit
    candidate_id INT REFERENCES users(id), -- Lien vers le compte (provisoire ou non)
    token VARCHAR(255) UNIQUE,
    property_id INT REFERENCES properties(id),
    status solvency_status DEFAULT 'pending',
    credit_source VARCHAR(20), -- 'property' or 'global'
    score_result INT, -- Résultat de l'algo
    report_url VARCHAR(255), -- Lien vers le PDF généré
    documents_json JSONB, -- Liens vers fiches de paie stockées sécurisées
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 6. LOCATION LONGUE DURÉE (Bail & Loyer)
-- =============================================

CREATE TABLE leases (
    id SERIAL PRIMARY KEY,
    property_id INT REFERENCES properties(id),
    tenant_id INT REFERENCES users(id), -- Nullable for Drafts
    start_date DATE NOT NULL,
    end_date DATE,
    rent_amount DECIMAL(10, 2) NOT NULL,
    charges_amount DECIMAL(10, 2) DEFAULT 0, -- Fix: Store charges specific to lease
    deposit_amount DECIMAL(10, 2) NOT NULL,
    payment_day INT DEFAULT 5, -- Day of month (1-31)
    special_clauses JSONB, -- Array of strings
    lease_status VARCHAR(50) DEFAULT 'draft', -- draft, signed_waiting_deposit, active, terminated
    signature_status VARCHAR(50) DEFAULT 'draft', -- draft, pending, signed, rejected
    signature_envelope_id TEXT, -- ID from external provider (Yousign/DocuSign)
    contract_url VARCHAR(255), -- Bail signé électroniquement [cite: 25]
    escrow_deposit_status escrow_status DEFAULT 'held', -- Séquestre de la caution [cite: 27]
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Gestion des quittances et paiements récurrents
CREATE TABLE rent_payments (
    id SERIAL PRIMARY KEY,
    lease_id INT REFERENCES leases(id),
    amount DECIMAL(10, 2) NOT NULL,
    due_date DATE NOT NULL,
    payment_date TIMESTAMP, -- Null si impayé
    status VARCHAR(50) DEFAULT 'pending', -- pending, paid, failed
    receipt_url VARCHAR(255), -- Quittance générée automatiquement [cite: 28]
    is_sepa_direct_debit BOOLEAN DEFAULT TRUE -- [cite: 26]
);

-- =============================================
-- 7. LOCATION SAISONNIÈRE (Vacances)
-- =============================================

CREATE TABLE seasonal_bookings (
    id SERIAL PRIMARY KEY,
    property_id INT REFERENCES properties(id),
    tenant_id INT REFERENCES users(id),
    check_in_date DATE NOT NULL,
    check_out_date DATE NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL, -- Montant payé par le locataire
    platform_fee_percent DECIMAL(5,2) DEFAULT 3.5, -- 3.5% [cite: 60]
    commission_amount DECIMAL(10, 2) GENERATED ALWAYS AS (total_amount * 0.035) STORED,
    payout_amount DECIMAL(10, 2) GENERATED ALWAYS AS (total_amount * (1 - 0.035)) STORED, -- Net pour le propriétaire
    escrow_status escrow_status DEFAULT 'held', -- Fonds bloqués jusqu'au check-in 
    booking_status VARCHAR(50) DEFAULT 'confirmed',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 8. FLUX FINANCIERS GLOBAUX
-- =============================================

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id), -- Qui a payé ou reçu
    related_entity_type VARCHAR(50), -- 'subscription', 'pack_purchase', 'rent_payment', 'seasonal_booking'
    related_entity_id INT, -- ID de la commande/réservation
    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'EUR',
    direction VARCHAR(10), -- 'inbound' (Locataire -> Séculoc), 'outbound' (Séculoc -> Proprio)
    stripe_payment_intent_id VARCHAR(100),
    status VARCHAR(50) DEFAULT 'success',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 9. GESTION DES INVITATIONS & ONBOARDING LOCATAIRES
-- =============================================

CREATE TABLE lease_invitations (
    id SERIAL PRIMARY KEY,
    property_id INT NOT NULL REFERENCES properties(id),
    lease_id INT REFERENCES leases(id), -- Linked Draft Lease
    owner_id INT NOT NULL REFERENCES users(id), -- L'expéditeur
    tenant_email VARCHAR(255) NOT NULL, -- Le destinataire
    token VARCHAR(255) UNIQUE NOT NULL, -- Token sécurisé envoyé par mail
    status VARCHAR(50) DEFAULT 'pending', -- pending, accepted, expired, revoked
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
