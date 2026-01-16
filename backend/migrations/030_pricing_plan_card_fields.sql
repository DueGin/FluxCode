-- Pricing plan card fields for richer UI (icon/badge/tagline)
-- PostgreSQL 15+

ALTER TABLE IF EXISTS pricing_plans
    ADD COLUMN IF NOT EXISTS icon_url TEXT,
    ADD COLUMN IF NOT EXISTS badge_text TEXT,
    ADD COLUMN IF NOT EXISTS tagline TEXT;

