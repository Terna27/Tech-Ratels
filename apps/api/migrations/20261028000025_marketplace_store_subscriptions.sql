-- +goose Up
CREATE TABLE marketplace_subscription_settings (
 id boolean PRIMARY KEY DEFAULT true CHECK(id), enabled boolean NOT NULL DEFAULT false,
 enabled_at timestamptz
);
INSERT INTO marketplace_subscription_settings(id) VALUES(true);
CREATE TABLE marketplace_subscription_plans (
 code text PRIMARY KEY CHECK(code IN ('basic','business')), name text NOT NULL,
 monthly_kobo bigint NOT NULL CHECK(monthly_kobo>0), yearly_kobo bigint NOT NULL CHECK(yearly_kobo>0),
 product_limit integer CHECK(product_limit>0)
);
INSERT INTO marketplace_subscription_plans VALUES
 ('basic','Basic Store',200000,2000000,30),('business','Business Store',500000,5000000,NULL);
CREATE TABLE marketplace_store_subscriptions (
 business_id varchar(36) PRIMARY KEY REFERENCES marketplace_businesses(id),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','active','expired','cancelled','suspended')),
 legacy_store boolean NOT NULL DEFAULT false, cancelled_at timestamptz,
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO marketplace_store_subscriptions(business_id,legacy_store) SELECT id,true FROM marketplace_businesses;
CREATE TABLE marketplace_subscription_payments (
 reference text PRIMARY KEY, business_id varchar(36) NOT NULL REFERENCES marketplace_businesses(id),
 plan_code text NOT NULL REFERENCES marketplace_subscription_plans(code),
 interval text NOT NULL CHECK(interval IN ('monthly','yearly')),
 amount_kobo bigint NOT NULL CHECK(amount_kobo>0), currency text NOT NULL DEFAULT 'NGN' CHECK(currency='NGN'),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','successful','failed')),
 idempotency_key text NOT NULL CHECK(length(idempotency_key) BETWEEN 1 AND 128),
 checkout_url text, paid_at timestamptz, fees_kobo bigint CHECK(fees_kobo>=0),
 refunded_kobo bigint NOT NULL DEFAULT 0 CHECK(refunded_kobo>=0 AND refunded_kobo<=amount_kobo),
 created_at timestamptz NOT NULL DEFAULT now(), verified_at timestamptz,
 UNIQUE(business_id,idempotency_key)
);
CREATE INDEX marketplace_subscription_payments_status ON marketplace_subscription_payments(status,created_at);
-- Each successful payment buys exactly one non-overlapping term. Current entitlement is derived by UTC time.
CREATE TABLE marketplace_subscription_terms (
 reference text PRIMARY KEY REFERENCES marketplace_subscription_payments(reference),
 business_id varchar(36) NOT NULL REFERENCES marketplace_store_subscriptions(business_id),
 plan_code text NOT NULL REFERENCES marketplace_subscription_plans(code),
 starts_at timestamptz NOT NULL, expires_at timestamptz NOT NULL CHECK(expires_at>starts_at),
 voided boolean NOT NULL DEFAULT false
);
CREATE INDEX marketplace_subscription_terms_business ON marketplace_subscription_terms(business_id,expires_at);
CREATE TABLE marketplace_subscription_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, business_id varchar(36) REFERENCES marketplace_businesses(id),
 reference text, actor_id varchar(36), action text NOT NULL, detail text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_subscription_emails (
 event_key text PRIMARY KEY, business_id varchar(36) NOT NULL REFERENCES marketplace_businesses(id),
 subject text NOT NULL, body text NOT NULL, attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(), sent_at timestamptz, provider_message_id text
);
-- +goose StatementBegin
CREATE FUNCTION marketplace_subscription_product_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE lim integer; entitled boolean; legacy boolean; enabled_on timestamptz; state text;
BEGIN
 IF NOT (SELECT enabled FROM marketplace_subscription_settings WHERE id) THEN RETURN NEW; END IF;
 -- Serialize all product writes and billing changes for this store, including parallel inserts.
 PERFORM id FROM marketplace_businesses WHERE id=NEW.business_id FOR UPDATE;
 SELECT s.status,s.legacy_store, cfg.enabled_at INTO state,legacy,enabled_on
 FROM marketplace_subscription_settings cfg LEFT JOIN marketplace_store_subscriptions s ON s.business_id=NEW.business_id WHERE cfg.id;
 IF TG_OP='UPDATE' AND (to_jsonb(NEW)-ARRAY['stock_quantity','updated_at','version'])=(to_jsonb(OLD)-ARRAY['stock_quantity','updated_at','version']) THEN RETURN NEW; END IF;
 IF NEW.status='archived' THEN RETURN NEW; END IF;
 SELECT true,p.product_limit INTO entitled,lim FROM marketplace_subscription_terms t
 JOIN marketplace_subscription_plans p ON p.code=t.plan_code
 WHERE t.business_id=NEW.business_id AND NOT t.voided AND t.starts_at<=now() AND t.expires_at>now()
 ORDER BY t.starts_at DESC LIMIT 1;
 IF state='suspended' OR (NOT coalesce(entitled,false) AND NOT (coalesce(legacy,false) AND now()<enabled_on+interval '30 days')) THEN
 RAISE EXCEPTION 'An active store subscription is required; existing orders can still be fulfilled' USING ERRCODE='P7001'; END IF;
 -- A prepaid Basic downgrade reserves the lower ceiling to avoid exceeding it at transition.
 IF EXISTS(SELECT 1 FROM marketplace_subscription_terms WHERE business_id=NEW.business_id AND NOT voided AND plan_code='basic' AND starts_at>now()) OR EXISTS(SELECT 1 FROM marketplace_subscription_payments WHERE business_id=NEW.business_id AND plan_code='basic' AND status='pending') THEN lim:=30; END IF;
 IF NEW.is_featured AND (lim IS NOT NULL OR (NOT coalesce(entitled,false) AND NOT (coalesce(legacy,false) AND now()<enabled_on+interval '30 days'))) THEN
 RAISE EXCEPTION 'Featured eligibility requires Business Store' USING ERRCODE='P7003'; END IF;
 IF lim IS NOT NULL AND (TG_OP='INSERT' OR OLD.status='archived') AND
 (SELECT count(*) FROM marketplace_products WHERE business_id=NEW.business_id AND status<>'archived' AND id<>NEW.id)>=lim THEN
 RAISE EXCEPTION 'Basic Store allows at most 30 draft or published products; archive a product or upgrade' USING ERRCODE='P7002'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER marketplace_subscription_product_guard BEFORE INSERT OR UPDATE ON marketplace_products
FOR EACH ROW EXECUTE FUNCTION marketplace_subscription_product_guard();
-- +goose StatementBegin
CREATE FUNCTION marketplace_subscription_term_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM id FROM marketplace_businesses WHERE id=NEW.business_id FOR UPDATE;
 IF NOT NEW.voided AND EXISTS(SELECT 1 FROM marketplace_subscription_terms WHERE business_id=NEW.business_id AND reference<>NEW.reference AND NOT voided AND starts_at<NEW.expires_at AND expires_at>NEW.starts_at) THEN
 RAISE EXCEPTION 'Store subscription terms must not overlap' USING ERRCODE='P7004'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER marketplace_subscription_term_guard BEFORE INSERT OR UPDATE ON marketplace_subscription_terms
FOR EACH ROW EXECUTE FUNCTION marketplace_subscription_term_guard();
-- +goose StatementBegin
CREATE FUNCTION marketplace_subscription_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE state text; grace boolean;
BEGIN
 IF NOT (SELECT enabled FROM marketplace_subscription_settings WHERE id) THEN RETURN NEW; END IF;
 PERFORM id FROM marketplace_businesses WHERE id=NEW.business_id FOR UPDATE;
 SELECT s.status,coalesce(s.legacy_store,false) AND now()<cfg.enabled_at+interval '30 days' INTO state,grace
 FROM marketplace_subscription_settings cfg LEFT JOIN marketplace_store_subscriptions s ON s.business_id=NEW.business_id WHERE cfg.id;
 IF state='suspended' OR (NOT coalesce(grace,false) AND NOT EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=NEW.business_id AND NOT t.voided AND t.starts_at<=now() AND t.expires_at>now())) THEN
 RAISE EXCEPTION 'This store is not accepting new orders until its subscription is renewed' USING ERRCODE='P7005'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER marketplace_subscription_order_guard BEFORE INSERT ON marketplace_order_items
FOR EACH ROW EXECUTE FUNCTION marketplace_subscription_order_guard();
-- +goose Down
DROP TRIGGER IF EXISTS marketplace_subscription_order_guard ON marketplace_order_items;
DROP FUNCTION IF EXISTS marketplace_subscription_order_guard();
DROP TRIGGER IF EXISTS marketplace_subscription_term_guard ON marketplace_subscription_terms;
DROP FUNCTION IF EXISTS marketplace_subscription_term_guard();
DROP TRIGGER IF EXISTS marketplace_subscription_product_guard ON marketplace_products;
DROP FUNCTION IF EXISTS marketplace_subscription_product_guard();
DROP TABLE marketplace_subscription_emails;
DROP TABLE marketplace_subscription_audit;
DROP TABLE marketplace_subscription_terms;
DROP TABLE marketplace_subscription_payments;
DROP TABLE marketplace_store_subscriptions;
DROP TABLE marketplace_subscription_plans;
DROP TABLE marketplace_subscription_settings;
