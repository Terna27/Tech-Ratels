-- +goose Up
-- Marketplace customer reviews. Historical orders and existing products are preserved.
CREATE TABLE marketplace_product_reviews (
 id varchar(36) PRIMARY KEY,
 product_id varchar(36) NOT NULL REFERENCES marketplace_products(id) ON DELETE CASCADE,
 buyer_user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 order_id varchar(36) NOT NULL REFERENCES marketplace_orders(id),
 rating smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
 comment text NOT NULL CHECK (char_length(btrim(comment)) BETWEEN 10 AND 2000),
 images jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(images) = 'array' AND jsonb_array_length(images) <= 5),
 status varchar(20) NOT NULL DEFAULT 'published' CHECK (status IN ('published','hidden')),
 vendor_response text CHECK (vendor_response IS NULL OR char_length(vendor_response) <= 2000),
 vendor_response_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(product_id,buyer_user_id)
);
CREATE INDEX idx_marketplace_reviews_product ON marketplace_product_reviews(product_id, status, created_at DESC);
CREATE TABLE marketplace_review_reports (
 id varchar(36) PRIMARY KEY,
 review_id varchar(36) NOT NULL REFERENCES marketplace_product_reviews(id) ON DELETE CASCADE,
 reporter_user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 reason varchar(500) NOT NULL CHECK (char_length(btrim(reason)) >= 5),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(review_id,reporter_user_id)
);
CREATE INDEX idx_marketplace_review_reports_review ON marketplace_review_reports(review_id);

-- +goose Down
DROP TABLE IF EXISTS marketplace_review_reports;
DROP TABLE IF EXISTS marketplace_product_reviews;
