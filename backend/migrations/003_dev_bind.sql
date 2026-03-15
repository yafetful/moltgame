-- 003_dev_bind.sql
-- Dev-agent optional binding: add bound_agent_id to owner_accounts,
-- add bonus_grant chakra type.

-- 1. Add bound_agent_id column to owner_accounts (1:1 dev-agent binding)
ALTER TABLE owner_accounts
    ADD COLUMN IF NOT EXISTS bound_agent_id UUID REFERENCES agents(id);

-- Enforce one agent per owner (and one owner per agent via FK uniqueness)
CREATE UNIQUE INDEX IF NOT EXISTS idx_owner_bound_agent_unique
    ON owner_accounts(bound_agent_id) WHERE bound_agent_id IS NOT NULL;

-- 2. Add bonus_grant type to chakra_transactions
ALTER TABLE chakra_transactions
    DROP CONSTRAINT IF EXISTS chakra_tx_type_check;

ALTER TABLE chakra_transactions
    ADD CONSTRAINT chakra_tx_type_check CHECK (type IN (
        'entry_fee', 'prize', 'rake', 'check_in',
        'passive_regen', 'initial_grant', 'bonus_grant'
    ));
