-- Change default currency for pricing plans to CNY (RMB)
-- PostgreSQL 15+

ALTER TABLE IF EXISTS pricing_plans
    ALTER COLUMN price_currency SET DEFAULT 'CNY';

