-- 000003_favorites.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_ads_price;
DROP TABLE IF EXISTS favorites;

COMMIT;
