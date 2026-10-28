package subscription

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/migrator"
	"ratels-api/internal/payment"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type providerFixture struct {
	mu                                   sync.Mutex
	amounts                              map[string]int64
	states                               map[string]string
	currencies                           map[string]string
	refunds                              map[string]int64
	ids                                  map[string]uint64
	failInit, failVerify, wrongReference bool
}

func provider(t *testing.T) (*payment.PaystackGateway, *providerFixture) {
	f := &providerFixture{amounts: map[string]int64{}, states: map[string]string{}, currencies: map[string]string{}, refunds: map[string]int64{}, ids: map[string]uint64{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer sk_test_subscriptions" {
			w.WriteHeader(401)
			return
		}

		if r.URL.Path == "/refund" {
			data := []map[string]interface{}{}
			for ref, id := range f.ids {
				if strconv.FormatUint(id, 10) == r.URL.Query().Get("transaction") && f.refunds[ref] > 0 {
					data = append(data, map[string]interface{}{"id": id, "transaction": id, "amount": f.refunds[ref], "currency": "NGN", "status": "processed"})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "data": data})
			return
		}
		if strings.HasSuffix(r.URL.Path, "initialize") {
			if f.failInit {
				w.WriteHeader(503)
				return
			}
			var body struct {
				Reference string                 `json:"reference"`
				Amount    int64                  `json:"amount"`
				Currency  string                 `json:"currency"`
				Metadata  map[string]interface{} `json:"metadata"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Currency != "NGN" || body.Metadata["purpose"] != "store_subscription" {
				t.Error("incorrect initialization")
			}
			if f.ids[body.Reference] == 0 {
				f.ids[body.Reference] = uint64(len(f.ids) + 1)
			}
			f.amounts[body.Reference] = body.Amount
			f.currencies[body.Reference] = "NGN"
			f.states[body.Reference] = "success"
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "data": map[string]interface{}{"reference": body.Reference, "authorization_url": "https://checkout.paystack.com/test", "access_code": "test"}})
			return
		}
		if f.failVerify {
			w.WriteHeader(503)
			return
		}
		ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		returned := ref
		if f.wrongReference {
			returned = "other"
		}
		refunds := []map[string]interface{}{}
		if f.refunds[ref] > 0 {
			refunds = append(refunds, map[string]interface{}{"id": 1, "amount": f.refunds[ref], "status": "processed"})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "data": map[string]interface{}{"id": f.ids[ref], "reference": returned, "amount": f.amounts[ref], "currency": f.currencies[ref], "status": f.states[ref], "paid_at": time.Now().UTC().Format(time.RFC3339), "fees": 3000, "refunds": refunds}})
	}))
	t.Cleanup(srv.Close)
	g := payment.NewPaystackGateway("sk_test_subscriptions", "")
	g.SetBaseURL(srv.URL)
	return g, f
}

type fixture struct {
	ctx                 context.Context
	pool                *pgxpool.Pool
	s                   *Service
	f                   *providerFixture
	owner, other, admin *db.User
	b                   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL integration tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.ConnConfig.Database, "test") {
		t.Fatal("integration tests require a database with 'test' in its name")
	}
	root, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	schema := "sub_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = root.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = root.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); root.Close() })
	c := cfg.Copy()
	c.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	g, p := provider(t)
	s := New(pool, g, delivery.NewDevSafeEmailSender(), true, true, "https://ratels.test")
	if err = s.Configure(ctx); err != nil {
		t.Fatal(err)
	}
	f := &fixture{ctx: ctx, pool: pool, s: s, f: p}
	f.owner = f.user(t, "member")
	f.other = f.user(t, "member")
	f.admin = f.user(t, "super_admin")
	f.b = f.store(t, f.owner)
	return f
}
func (f *fixture) user(t *testing.T, role string) *db.User {
	t.Helper()
	id := uuid.NewString()
	_, err := f.pool.Exec(f.ctx, `INSERT INTO users(id,email,password_hash,display_name,membership_reference,status,primary_role,is_verified) VALUES($1,$2,'test-only','Subscription Tester',$1,'active',$3,true)`, id, id+"@example.test", role)
	if err != nil {
		t.Fatal(err)
	}
	return &db.User{ID: id, PrimaryRole: role, IsVerified: true}
}
func (f *fixture) store(t *testing.T, u *db.User) string {
	t.Helper()
	id := uuid.NewString()
	_, err := f.pool.Exec(f.ctx, `INSERT INTO marketplace_businesses(id,owner_user_id,name,slug,category,ownership_type,contact_email,contact_phone) VALUES($1,$2,'Test store',$1,'tools','member_owned','store@example.test','123')`, id, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func (f *fixture) checkout(t *testing.T, plan, interval string) *Checkout {
	t.Helper()
	c, err := f.s.Checkout(f.ctx, f.owner, f.b, CheckoutInput{plan, interval, uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func (f *fixture) paid(t *testing.T, plan, interval string) *Payment {
	t.Helper()
	c := f.checkout(t, plan, interval)
	p, err := f.s.Verify(f.ctx, f.owner, c.Reference, false)
	if err != nil || p.Status != "successful" {
		t.Fatalf("%+v %v", p, err)
	}
	return p
}
func (f *fixture) product(status string) error {
	id := uuid.NewString()
	_, err := f.pool.Exec(f.ctx, `INSERT INTO marketplace_products(id,business_id,title,slug,category,sku,price_kobo,status) VALUES($1,$2,'Item',$1,'tools',$1,20000,$3)`, id, f.b, status)
	return err
}
func signed(body []byte) string {
	h := hmac.New(sha512.New, []byte("sk_test_subscriptions"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
func count(t *testing.T, f *fixture, sql string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func execSQL(t *testing.T, f *fixture, sql string, args ...interface{}) {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationPricingOwnershipAndApproval(t *testing.T) {
	for _, plan := range []string{"basic", "business"} {
		for _, interval := range []string{"monthly", "yearly"} {
			t.Run(plan+interval, func(t *testing.T) {
				f := newFixture(t)
				if _, err := f.s.Checkout(f.ctx, f.other, f.b, CheckoutInput{plan, interval, "bad"}); !errors.Is(err, ErrForbidden) {
					t.Fatal(err)
				}
				if _, err := f.s.Dashboard(f.ctx, nil, f.b); !errors.Is(err, ErrForbidden) {
					t.Fatal(err)
				}
				p := f.paid(t, plan, interval)
				want, _ := Price(plan, interval)
				if p.Amount != want {
					t.Fatal(p.Amount)
				}
				d, err := f.s.Dashboard(f.ctx, f.owner, f.b)
				if err != nil || d.Current == nil || d.Status != "active" {
					t.Fatalf("%+v %v", d, err)
				}
				var state string
				if err = f.pool.QueryRow(f.ctx, `SELECT status FROM marketplace_businesses WHERE id=$1`, f.b).Scan(&state); err != nil || state != "pending_review" {
					t.Fatal("payment bypassed approval", state, err)
				}
				if _, err = f.s.Report(f.ctx, f.owner, "", "", ""); !errors.Is(err, ErrForbidden) {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestIntegrationWebhookReplayAndValidation(t *testing.T) {
	f := newFixture(t)
	c := f.checkout(t, "basic", "monthly")
	body := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, c.Reference))
	if err := f.s.Webhook(f.ctx, "invalid", body); !errors.Is(err, payment.ErrInvalidSignature) {
		t.Fatal(err)
	}
	for _, kind := range []string{"amount", "currency", "reference"} {
		t.Run(kind, func(t *testing.T) {
			f.f.mu.Lock()
			switch kind {
			case "amount":
				f.f.amounts[c.Reference] = 1
			case "currency":
				f.f.currencies[c.Reference] = "USD"
			case "reference":
				f.f.wrongReference = true
			}
			f.f.mu.Unlock()
			if err := f.s.Webhook(f.ctx, signed(body), body); err == nil {
				t.Fatal("invalid payment activated")
			}
			if n := count(t, f, `SELECT count(*) FROM marketplace_subscription_terms`); n != 0 {
				t.Fatal(n)
			}
			f.f.mu.Lock()
			f.f.amounts[c.Reference] = 200000
			f.f.currencies[c.Reference] = "NGN"
			f.f.wrongReference = false
			f.f.mu.Unlock()
		})
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.s.Webhook(f.ctx, signed(body), body) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, f, `SELECT count(*) FROM marketplace_subscription_terms`); n != 1 {
		t.Fatal(n)
	}
	if n := count(t, f, `SELECT count(*) FROM marketplace_subscription_emails WHERE event_key=$1`, c.Reference+":paid"); n != 1 {
		t.Fatal(n)
	}
	if _, err := f.s.Verify(f.ctx, f.other, c.Reference, false); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
func TestIntegrationPaymentRecoveryAndIdempotency(t *testing.T) {
	f := newFixture(t)
	in := CheckoutInput{"basic", "monthly", "retry-key"}
	f.f.failInit = true
	if _, err := f.s.Checkout(f.ctx, f.owner, f.b, in); err == nil {
		t.Fatal("initialization error ignored")
	}
	f.f.failInit = false
	c, err := f.s.Checkout(f.ctx, f.owner, f.b, in)
	if err != nil {
		t.Fatal(err)
	}
	same, err := f.s.Checkout(f.ctx, f.owner, f.b, in)
	if err != nil || same.Reference != c.Reference {
		t.Fatal("retry duplicated", err)
	}
	if _, err = f.s.Checkout(f.ctx, f.owner, f.b, CheckoutInput{"business", "monthly", in.Key}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, state := range []string{"pending", "failed"} {
		f.f.states[c.Reference] = state
		p, e := f.s.Verify(f.ctx, f.owner, c.Reference, false)
		if e != nil || p.Status != state {
			t.Fatalf("%s %v %+v", state, e, p)
		}
		if n := count(t, f, `SELECT count(*) FROM marketplace_subscription_terms`); n != 0 {
			t.Fatal(n)
		}
	}
	f.f.states[c.Reference] = "success"
	f.f.failVerify = true
	if _, err = f.s.Verify(f.ctx, f.owner, c.Reference, false); err == nil {
		t.Fatal("provider error ignored")
	}
	f.f.failVerify = false
	if _, err = f.s.Verify(f.ctx, f.owner, c.Reference, false); err != nil {
		t.Fatal(err)
	}
}
func TestIntegrationBasicConcurrentProductLimit(t *testing.T) {
	f := newFixture(t)
	f.paid(t, "basic", "monthly")
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 45; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f.product("draft") == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 30 {
		t.Fatalf("created %d, want 30", ok.Load())
	}
	if err := f.product("published"); err == nil {
		t.Fatal("limit bypassed")
	}
	if err := f.product("archived"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE marketplace_products SET status='published' WHERE business_id=$1 AND status='archived'`, f.b); err == nil {
		t.Fatal("unarchive bypassed limit")
	}
	execSQL(t, f, `UPDATE marketplace_products SET status='archived' WHERE id=(SELECT id FROM marketplace_products WHERE business_id=$1 AND status='draft' LIMIT 1)`, f.b)
	if err := f.product("published"); err != nil {
		t.Fatal(err)
	}
}
func TestIntegrationBusinessDowngradeAndRenewal(t *testing.T) {
	f := newFixture(t)
	f.paid(t, "business", "monthly")
	for i := 0; i < 45; i++ {
		if err := f.product("draft"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.s.Checkout(f.ctx, f.owner, f.b, CheckoutInput{"basic", "yearly", "too-many"}); err == nil {
		t.Fatal("downgrade over limit accepted")
	}
	execSQL(t, f, `UPDATE marketplace_products SET status='archived' WHERE id IN (SELECT id FROM marketplace_products WHERE business_id=$1 LIMIT 15)`, f.b)
	f.paid(t, "basic", "yearly")
	d, err := f.s.Dashboard(f.ctx, f.owner, f.b)
	if err != nil || d.Current.Plan != "business" || len(d.Scheduled) != 1 || d.Scheduled[0].Plan != "basic" || !d.Scheduled[0].Starts.Equal(d.Current.Expires) {
		t.Fatalf("%+v %v", d, err)
	}
	if err = f.product("draft"); err == nil {
		t.Fatal("queued downgrade ceiling bypassed")
	}
	if err = f.s.Cancel(f.ctx, f.owner, f.b); err == nil {
		t.Fatal("silently cancelled future term")
	}
	if _, err = f.s.Checkout(f.ctx, f.owner, f.b, CheckoutInput{"business", "yearly", "third"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestIntegrationUpgradeAndNonoverlap(t *testing.T) {
	f := newFixture(t)
	p := f.paid(t, "basic", "monthly")
	f.paid(t, "business", "yearly")
	d, err := f.s.Dashboard(f.ctx, f.owner, f.b)
	if err != nil || d.Current.Plan != "basic" || len(d.Scheduled) != 1 || d.Scheduled[0].Plan != "business" {
		t.Fatalf("%+v %v", d, err)
	}
	if _, err = f.pool.Exec(f.ctx, `UPDATE marketplace_subscription_terms SET starts_at=now() WHERE reference<>$1`, p.Reference); err == nil {
		t.Fatal("overlap allowed")
	}
}
func TestIntegrationExpiryCancelSuspendAndRenew(t *testing.T) {
	f := newFixture(t)
	f.paid(t, "basic", "monthly")
	if err := f.s.Cancel(f.ctx, f.other, f.b); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := f.s.Cancel(f.ctx, f.owner, f.b); err != nil {
		t.Fatal(err)
	}
	if err := f.product("draft"); err != nil {
		t.Fatal("cancellation lost paid access", err)
	}
	if err := f.s.Suspend(f.ctx, f.admin, f.b, "reason", true); err != nil {
		t.Fatal(err)
	}
	if err := f.product("draft"); err == nil {
		t.Fatal("suspension ignored")
	}
	if err := f.s.Suspend(f.ctx, f.admin, f.b, "reason", false); err != nil {
		t.Fatal(err)
	}
	execSQL(t, f, `UPDATE marketplace_subscription_terms SET starts_at=now()-interval '2 months',expires_at=now()-interval '1 month' WHERE business_id=$1`, f.b)
	if err := f.s.Tick(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.product("published"); err == nil {
		t.Fatal("expiry ignored")
	}
	execSQL(t, f, `UPDATE marketplace_products SET stock_quantity=stock_quantity+1 WHERE business_id=$1`, f.b)
	d, err := f.s.Dashboard(f.ctx, f.owner, f.b)
	if err != nil || d.Status != "expired" {
		t.Fatalf("%+v %v", d, err)
	}
	f.paid(t, "basic", "monthly")
	d, err = f.s.Dashboard(f.ctx, f.owner, f.b)
	if err != nil || d.Status != "active" {
		t.Fatalf("%+v %v", d, err)
	}
}
func TestIntegrationLegacyGraceAndDisabledFeature(t *testing.T) {
	f := newFixture(t)
	if err := f.product("draft"); err == nil {
		t.Fatal("unpaid new store allowed")
	}
	execSQL(t, f, `INSERT INTO marketplace_store_subscriptions(business_id,legacy_store) VALUES($1,true)`, f.b)
	if err := f.product("draft"); err != nil {
		t.Fatal("grace ignored", err)
	}
	execSQL(t, f, `UPDATE marketplace_subscription_settings SET enabled_at=now()-interval '31 days'`)
	if err := f.product("draft"); err == nil {
		t.Fatal("grace never expired")
	}
	f.s.Enabled = false
	f.s.CheckoutEnabled = false
	if err := f.s.Configure(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.product("draft"); err != nil {
		t.Fatal("disabled feature changed marketplace", err)
	}
	if _, err := f.s.Checkout(f.ctx, f.owner, f.b, CheckoutInput{"basic", "monthly", "disabled"}); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
}

type failingEmail struct {
	calls int
	fail  bool
}

func (e *failingEmail) ProviderName() string { return "test" }
func (e *failingEmail) IsConfigured() bool   { return true }
func (e *failingEmail) SendNotificationEmail(context.Context, string, string, string, string, string) (string, error) {
	e.calls++
	if e.fail {
		return "", errors.New("temporary failure")
	}
	return "id", nil
}
func TestIntegrationReportsAndEmailRetries(t *testing.T) {
	f := newFixture(t)
	mail := &failingEmail{fail: true}
	f.s.Email = mail
	p := f.paid(t, "basic", "monthly")
	pending := f.checkout(t, "business", "yearly")
	r, err := f.s.Report(f.ctx, f.admin, "", "", "")
	if err != nil || r.Gross != 200000 || r.Pending != 1 || r.KnownFees != 3000 || r.Net != 197000 || r.Monthly != 200000 || r.Yearly != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if err = f.s.Tick(f.ctx); err != nil {
		t.Fatal(err)
	}
	if mail.calls != 1 {
		t.Fatal(mail.calls)
	}
	if n := count(t, f, `SELECT count(*) FROM marketplace_subscription_terms WHERE reference=$1`, p.Reference); n != 1 {
		t.Fatal("mail reversed payment")
	}
	mail.fail = false
	execSQL(t, f, `UPDATE marketplace_subscription_emails SET next_attempt_at=now()`)
	if err = f.s.Tick(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = f.s.Tick(f.ctx); err != nil {
		t.Fatal(err)
	}
	if mail.calls != 2 {
		t.Fatal("duplicate email", mail.calls)
	}
	f.f.states[pending.Reference] = "failed"
	if _, err = f.s.Verify(f.ctx, f.owner, pending.Reference, false); err != nil {
		t.Fatal(err)
	}
	f.f.refunds[p.Reference] = 100000
	if _, err = f.s.Verify(f.ctx, f.owner, p.Reference, false); err != nil {
		t.Fatal(err)
	}
	r, err = f.s.Report(f.ctx, f.admin, "", "", "")
	if err != nil || r.Gross != 200000 || r.Refunds != 100000 || r.Net != 97000 || r.Failed != 1 {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestIntegrationMigrationRollback(t *testing.T) {
	f := newFixture(t)
	raw, err := os.ReadFile("../../migrations/20261028000025_marketplace_store_subscriptions.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	execSQL(t, f, parts[1])
	if n := count(t, f, `SELECT count(*) FROM marketplace_businesses WHERE id=$1`, f.b); n != 1 {
		t.Fatal("rollback removed marketplace data")
	}
	execSQL(t, f, parts[0])
	var legacy bool
	if err = f.pool.QueryRow(f.ctx, `SELECT legacy_store FROM marketplace_store_subscriptions WHERE business_id=$1`, f.b).Scan(&legacy); err != nil || !legacy {
		t.Fatal("legacy store grace missing", err)
	}
}

func TestIntegrationGraceStartsAtFirstEnableAndAnalytics(t *testing.T) {
	f := newFixture(t)
	execSQL(t, f, `UPDATE marketplace_subscription_settings SET enabled=false,enabled_at=NULL`)
	old := f.store(t, f.other)
	if err := f.s.Configure(f.ctx); err != nil {
		t.Fatal(err)
	}
	var legacy bool
	if err := f.pool.QueryRow(f.ctx, `SELECT legacy_store FROM marketplace_store_subscriptions WHERE business_id=$1`, old).Scan(&legacy); err != nil || !legacy {
		t.Fatal("store created during disabled rollout lost grace", err)
	}
	allowed, err := f.s.AnalyticsAllowed(f.ctx, f.other, old)
	if err != nil || !allowed {
		t.Fatal("legacy analytics removed during grace", err)
	}
	execSQL(t, f, `UPDATE marketplace_store_subscriptions SET legacy_store=false WHERE business_id=$1`, f.b)
	f.paid(t, "basic", "monthly")
	allowed, err = f.s.AnalyticsAllowed(f.ctx, f.owner, f.b)
	if err != nil || allowed {
		t.Fatal("Basic analytics enabled", err)
	}
	if _, err = f.s.AnalyticsAllowed(f.ctx, f.other, f.b); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
func TestIntegrationExpiredStoreCannotTakeNewOrders(t *testing.T) {
	f := newFixture(t)
	f.paid(t, "business", "monthly")
	if err := f.product("published"); err != nil {
		t.Fatal(err)
	}
	allowed, err := f.s.AnalyticsAllowed(f.ctx, f.owner, f.b)
	if err != nil || !allowed {
		t.Fatal("Business analytics unavailable", err)
	}
	order := uuid.NewString()
	execSQL(t, f, `INSERT INTO marketplace_orders(id,order_number,buyer_name,buyer_email,buyer_phone,subtotal_kobo,total_kobo) VALUES($1,$1,'Buyer','buyer@example.test','123',20000,20000)`, order)
	insert := `INSERT INTO marketplace_order_items(id,order_id,product_id,business_id,title_snapshot,sku_snapshot,price_kobo_snapshot,quantity,subtotal_kobo) SELECT $1,$2,id,business_id,title,sku,price_kobo,1,price_kobo FROM marketplace_products WHERE business_id=$3 LIMIT 1`
	execSQL(t, f, insert, uuid.NewString(), order, f.b)
	execSQL(t, f, `UPDATE marketplace_subscription_terms SET starts_at=now()-interval '2 months',expires_at=now()-interval '1 month' WHERE business_id=$1`, f.b)
	if _, err = f.pool.Exec(f.ctx, insert, uuid.NewString(), order, f.b); err == nil {
		t.Fatal("expired store took a new order")
	}
	execSQL(t, f, `UPDATE marketplace_orders SET payment_status='paid',order_status='dispatched' WHERE id=$1`, order)
	execSQL(t, f, `UPDATE marketplace_products SET stock_quantity=stock_quantity+1 WHERE business_id=$1`, f.b)
}
