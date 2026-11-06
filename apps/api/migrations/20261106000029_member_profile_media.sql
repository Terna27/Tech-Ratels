-- +goose Up
-- Keep Phase 71 membership identifiers and legacy profile records unchanged.
CREATE TABLE IF NOT EXISTS member_profile_media (
 user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 avatar_key text,
 cover_key text,
 cover_url text,
 avatar_x smallint NOT NULL DEFAULT 50 CHECK(avatar_x BETWEEN 0 AND 100),
 avatar_y smallint NOT NULL DEFAULT 50 CHECK(avatar_y BETWEEN 0 AND 100),
 cover_x smallint NOT NULL DEFAULT 50 CHECK(cover_x BETWEEN 0 AND 100),
 cover_y smallint NOT NULL DEFAULT 50 CHECK(cover_y BETWEEN 0 AND 100),
 avatar_zoom smallint NOT NULL DEFAULT 100 CHECK(avatar_zoom BETWEEN 100 AND 200),
 cover_zoom smallint NOT NULL DEFAULT 100 CHECK(cover_zoom BETWEEN 100 AND 200),
 updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE IF EXISTS member_profile_media;
