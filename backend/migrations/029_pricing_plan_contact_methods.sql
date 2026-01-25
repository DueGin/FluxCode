-- Add contact methods to pricing plans (wechat/qq/etc.)
-- PostgreSQL 15+

ALTER TABLE IF EXISTS pricing_plans
    ADD COLUMN IF NOT EXISTS contact_methods JSONB NOT NULL DEFAULT '[]';
