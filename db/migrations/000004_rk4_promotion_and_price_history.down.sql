DROP TABLE IF EXISTS wallet_transactions;
DROP TABLE IF EXISTS ad_price_history;

DROP INDEX IF EXISTS idx_ads_promotion;
ALTER TABLE ads
    DROP COLUMN IF EXISTS promoted_until,
    DROP COLUMN IF EXISTS promotion_priority,
    DROP COLUMN IF EXISTS promotion_tier;

ALTER TABLE users
    DROP COLUMN IF EXISTS balance;
