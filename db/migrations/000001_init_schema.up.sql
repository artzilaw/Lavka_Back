-- 000001_init_schema.up.sql
-- Нормализованная схема БД PostgreSQL для проекта Lavka (аналог Юлы)

BEGIN;

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(120) NOT NULL,
    email         VARCHAR(255) NOT NULL UNIQUE,
    pass_hash     BYTEA,
    avatar_path   TEXT NOT NULL DEFAULT '',
    rating        NUMERIC(3, 2) NOT NULL DEFAULT 0.00 CHECK (rating >= 0 AND rating <= 5),
    reviews_count INTEGER NOT NULL DEFAULT 0 CHECK (reviews_count >= 0),
    is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories (
    id    BIGSERIAL PRIMARY KEY,
    title VARCHAR(100) NOT NULL UNIQUE,
    slug  VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS ads (
    id              BIGSERIAL PRIMARY KEY,
    seller_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id     BIGINT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    title           VARCHAR(120) NOT NULL,
    description     TEXT NOT NULL,
    price           BIGINT NOT NULL DEFAULT 0 CHECK (price >= 0),
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'closed', 'sold', 'deleted')),
    location        VARCHAR(120) NOT NULL,
    views_count     BIGINT NOT NULL DEFAULT 0 CHECK (views_count >= 0),
    favorites_count BIGINT NOT NULL DEFAULT 0 CHECK (favorites_count >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ad_photos (
    id         BIGSERIAL PRIMARY KEY,
    ad_id      BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    position   SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cart_items (
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ad_id    BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0 AND quantity <= 99),
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, ad_id)
);

CREATE TABLE IF NOT EXISTS orders (
    id               BIGSERIAL PRIMARY KEY,
    buyer_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    total_price      BIGINT NOT NULL DEFAULT 0 CHECK (total_price >= 0),
    delivery_method  VARCHAR(20) NOT NULL DEFAULT 'delivery'
                     CHECK (delivery_method IN ('delivery', 'pickup')),
    delivery_address TEXT NOT NULL DEFAULT '',
    payment_method   VARCHAR(20) NOT NULL DEFAULT 'card'
                     CHECK (payment_method IN ('card', 'cash')),
    phone            VARCHAR(32) NOT NULL,
    comment          TEXT NOT NULL DEFAULT '',
    status           VARCHAR(20) NOT NULL DEFAULT 'confirmed'
                     CHECK (status IN ('pending', 'confirmed', 'completed', 'cancelled')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS order_items (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    ad_id       BIGINT NOT NULL REFERENCES ads(id) ON DELETE RESTRICT,
    seller_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_name VARCHAR(120) NOT NULL,
    title       VARCHAR(120) NOT NULL,
    price       BIGINT NOT NULL CHECK (price >= 0),
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    photo       TEXT NOT NULL DEFAULT ''
);

-- Индексы для ускорения выборок
CREATE INDEX IF NOT EXISTS idx_ads_seller_id ON ads(seller_id);
CREATE INDEX IF NOT EXISTS idx_ads_category_id ON ads(category_id);
CREATE INDEX IF NOT EXISTS idx_ads_status_created ON ads(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ad_photos_ad_id ON ad_photos(ad_id, position);
CREATE INDEX IF NOT EXISTS idx_orders_buyer_id ON orders(buyer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

COMMIT;
