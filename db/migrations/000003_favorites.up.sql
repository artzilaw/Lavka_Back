-- 000003_favorites.up.sql
-- Таблица избранных объявлений пользователей + индекс для поиска

BEGIN;

CREATE TABLE IF NOT EXISTS favorites (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ad_id      BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, ad_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_favorites_ad_id ON favorites(ad_id);
CREATE INDEX IF NOT EXISTS idx_ads_price ON ads(price);

COMMIT;
