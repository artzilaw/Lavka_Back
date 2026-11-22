ALTER TABLE users
    ADD COLUMN IF NOT EXISTS balance BIGINT NOT NULL DEFAULT 1000 CHECK (balance >= 0);

ALTER TABLE ads
    ADD COLUMN IF NOT EXISTS promotion_tier VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS promotion_priority INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS promoted_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ads_promotion ON ads(promotion_priority DESC, promoted_until DESC);

CREATE TABLE IF NOT EXISTS ad_price_history (
    id BIGSERIAL PRIMARY KEY,
    ad_id BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    old_price BIGINT NOT NULL,
    new_price BIGINT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ad_price_history_ad_id ON ad_price_history(ad_id, changed_at ASC);

CREATE TABLE IF NOT EXISTS wallet_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('topup', 'promotion')),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_id ON wallet_transactions(user_id, created_at DESC);
