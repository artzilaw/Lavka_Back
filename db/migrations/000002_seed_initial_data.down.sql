-- 000002_seed_initial_data.down.sql

BEGIN;

DELETE FROM categories WHERE id BETWEEN 1 AND 11;

COMMIT;
