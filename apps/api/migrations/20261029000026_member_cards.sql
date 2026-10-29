-- +goose Up
-- Existing non-empty references are preserved; repair only unusable legacy blanks.
UPDATE users SET membership_reference='RATEL-'||replace(id,'-','') WHERE btrim(membership_reference)='';
ALTER TABLE users ADD CONSTRAINT membership_reference_nonempty CHECK (btrim(membership_reference)<>'');
CREATE TABLE member_card_links (
 user_id varchar(36) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 token varchar(64) NOT NULL UNIQUE CHECK(token ~ '^[0-9a-f]{64}$'),
 created_at timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION preserve_membership_reference() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.membership_reference IS DISTINCT FROM OLD.membership_reference THEN
  RAISE EXCEPTION 'membership reference is immutable';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER stable_membership_reference BEFORE UPDATE OF membership_reference ON users FOR EACH ROW EXECUTE FUNCTION preserve_membership_reference();
-- +goose Down
DROP TRIGGER IF EXISTS stable_membership_reference ON users;
DROP FUNCTION IF EXISTS preserve_membership_reference();
DROP TABLE IF EXISTS member_card_links;
ALTER TABLE users DROP CONSTRAINT IF EXISTS membership_reference_nonempty;
