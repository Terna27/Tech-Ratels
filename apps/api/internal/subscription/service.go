// Package subscription implements manual, verified marketplace store billing.
package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/payment"
)

var ErrForbidden = errors.New("store owner or super-admin access required")
var ErrDisabled = errors.New("store subscription checkout is disabled or authorized Paystack credentials are missing")
var ErrConflict = errors.New("billing request conflicts with an existing checkout or scheduled term")

type Plan struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Monthly int64  `json:"monthly_kobo"`
	Yearly  int64  `json:"yearly_kobo"`
	Limit   *int   `json:"product_limit"`
}

func Plans() []Plan {
	n := 30
	return []Plan{{"basic", "Basic Store", 200000, 2000000, &n}, {"business", "Business Store", 500000, 5000000, nil}}
}
func Price(plan, interval string) (int64, error) {
	for _, p := range Plans() {
		if p.Code == plan {
			switch interval {
			case "monthly":
				return p.Monthly, nil
			case "yearly":
				return p.Yearly, nil
			}
		}
	}
	return 0, errors.New("choose basic or business and monthly or yearly")
}

// PeriodEnd clamps the day to the final day of the target calendar month (Jan 31 -> Feb 28).
func PeriodEnd(start time.Time, interval string) time.Time {
	start = start.UTC()
	y, m, d := start.Date()
	if interval == "yearly" {
		y++
	} else {
		m++
	}
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > last {
		d = last
	}
	return time.Date(y, m, d, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), time.UTC)
}

type Service struct {
	Pool            *pgxpool.Pool
	Gateway         payment.PaymentGateway
	Email           delivery.EmailSender
	Enabled         bool
	CheckoutEnabled bool
	Frontend        string
}

func New(pool *pgxpool.Pool, gateway payment.PaymentGateway, email delivery.EmailSender, enabled, checkout bool, frontend string) *Service {
	return &Service{pool, gateway, email, enabled, checkout, strings.TrimRight(frontend, "/")}
}

// Configure only enables enforcement explicitly. Existing-store grace begins at first enablement.
func (s *Service) Configure(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var enabledAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT enabled_at FROM marketplace_subscription_settings WHERE id FOR UPDATE`).Scan(&enabledAt); err != nil {
		return err
	}
	if s.Enabled && enabledAt == nil {
		_, err = tx.Exec(ctx, `INSERT INTO marketplace_store_subscriptions(business_id,legacy_store) SELECT id,true FROM marketplace_businesses ON CONFLICT(business_id) DO UPDATE SET legacy_store=true`)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE marketplace_subscription_settings SET enabled=$1,enabled_at=CASE WHEN $1 THEN coalesce(enabled_at,now()) ELSE enabled_at END WHERE id`, s.Enabled)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) authorize(ctx context.Context, u *db.User, business string) error {
	if u == nil {
		return ErrForbidden
	}
	var owner string
	if err := s.Pool.QueryRow(ctx, `SELECT owner_user_id FROM marketplace_businesses WHERE id=$1`, business).Scan(&owner); err != nil {
		return ErrForbidden
	}
	if owner != u.ID && u.PrimaryRole != "super_admin" {
		return ErrForbidden
	}
	return nil
}
func lockStore(ctx context.Context, tx pgx.Tx, b string) error {
	var id string
	return tx.QueryRow(ctx, `SELECT id FROM marketplace_businesses WHERE id=$1 FOR UPDATE`, b).Scan(&id)
}
func audit(ctx context.Context, tx pgx.Tx, b, ref, actor, action, detail string) error {
	_, err := tx.Exec(ctx, `INSERT INTO marketplace_subscription_audit(business_id,reference,actor_id,action,detail) VALUES($1,$2,$3,$4,$5)`, b, ref, actor, action, detail)
	return err
}
func email(ctx context.Context, tx pgx.Tx, key, b, subject, body string) error {
	_, err := tx.Exec(ctx, `INSERT INTO marketplace_subscription_emails(event_key,business_id,subject,body) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, key, b, subject, body)
	return err
}

type CheckoutInput struct {
	Plan     string `json:"plan"`
	Interval string `json:"interval"`
	Key      string `json:"idempotency_key"`
}
type Checkout struct {
	Reference string `json:"reference"`
	URL       string `json:"checkout_url"`
	Status    string `json:"status"`
}

func (s *Service) Checkout(ctx context.Context, u *db.User, b string, in CheckoutInput) (*Checkout, error) {
	if err := s.authorize(ctx, u, b); err != nil {
		return nil, err
	}
	if !s.Enabled || !s.CheckoutEnabled {
		return nil, ErrDisabled
	}
	amount, err := Price(in.Plan, in.Interval)
	if err != nil {
		return nil, err
	}
	if len(in.Key) < 1 || len(in.Key) > 128 {
		return nil, errors.New("idempotency_key must contain 1 to 128 characters")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockStore(ctx, tx, b); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO marketplace_store_subscriptions(business_id) VALUES($1) ON CONFLICT DO NOTHING`, b)
	if err != nil {
		return nil, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT status FROM marketplace_store_subscriptions WHERE business_id=$1`, b).Scan(&state)
	if err != nil {
		return nil, err
	}
	if state == "suspended" {
		return nil, errors.New("subscription is suspended; contact an administrator")
	}
	out := &Checkout{}
	var oldPlan, oldInterval string
	var checkoutURL *string
	err = tx.QueryRow(ctx, `SELECT reference,checkout_url,status,plan_code,interval FROM marketplace_subscription_payments WHERE business_id=$1 AND idempotency_key=$2`, b, in.Key).Scan(&out.Reference, &checkoutURL, &out.Status, &oldPlan, &oldInterval)
	if err == nil {
		if oldPlan != in.Plan || oldInterval != in.Interval {
			return nil, ErrConflict
		}
		if checkoutURL != nil {
			out.URL = *checkoutURL
			return out, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else {
		var blocked bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_subscription_payments WHERE business_id=$1 AND status='pending') OR EXISTS(SELECT 1 FROM marketplace_subscription_terms WHERE business_id=$1 AND starts_at>now() AND NOT voided)`, b).Scan(&blocked)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, ErrConflict
		}
		if in.Plan == "basic" {
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM marketplace_products WHERE business_id=$1 AND status<>'archived'`, b).Scan(&count); err != nil {
				return nil, err
			}
			if count > 30 {
				return nil, errors.New("archive products until at most 30 remain before choosing Basic Store")
			}
		}
		out.Reference = "sub_" + uuid.NewString()
		out.Status = "pending"
		_, err = tx.Exec(ctx, `INSERT INTO marketplace_subscription_payments(reference,business_id,plan_code,interval,amount_kobo,idempotency_key) VALUES($1,$2,$3,$4,$5,$6)`, out.Reference, b, in.Plan, in.Interval, amount, in.Key)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	// A durable reference is recorded before calling Paystack. A retry reuses exactly that reference.
	var customerEmail string
	err = s.Pool.QueryRow(ctx, `SELECT coalesce(nullif(u.email,''),b.contact_email) FROM users u JOIN marketplace_businesses b ON b.owner_user_id=u.id WHERE b.id=$1`, b).Scan(&customerEmail)
	if err != nil {
		return nil, err
	}
	initialized, err := s.Gateway.InitializePayment(ctx, payment.InitializePaymentInput{OrderReference: out.Reference, AmountKobo: amount, Currency: "NGN", CustomerEmail: customerEmail, CallbackURL: s.Frontend + "/marketplace/subscriptions/confirmation", Metadata: map[string]interface{}{"purpose": "store_subscription", "business_id": b}})
	if err != nil {
		return nil, fmt.Errorf("checkout initialization failed; retry this request or reconcile its reference: %w", err)
	}
	if initialized.PaymentReference != out.Reference || initialized.Provider != "paystack" {
		return nil, payment.ErrReferenceMismatch
	}
	parsed, e := url.Parse(initialized.CheckoutURL)
	if e != nil || parsed.Scheme != "https" || parsed.Hostname() != "checkout.paystack.com" || parsed.User != nil {
		return nil, errors.New("invalid secure Paystack checkout URL")
	}
	out.URL = initialized.CheckoutURL
	_, err = s.Pool.Exec(ctx, `UPDATE marketplace_subscription_payments SET checkout_url=$2 WHERE reference=$1`, out.Reference, out.URL)
	return out, err
}

type Payment struct {
	Reference   string     `json:"reference"`
	BusinessID  string     `json:"business_id"`
	Plan        string     `json:"plan"`
	Interval    string     `json:"interval"`
	Amount      int64      `json:"amount_kobo"`
	Status      string     `json:"status"`
	Created     time.Time  `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at"`
	Fees        *int64     `json:"fees_kobo"`
	Refunded    int64      `json:"refunded_kobo"`
	CheckoutURL *string    `json:"checkout_url"`
}

const paymentColumns = `reference,business_id,plan_code,interval,amount_kobo,status,created_at,paid_at,fees_kobo,refunded_kobo,checkout_url`

func scanPayment(row pgx.Row) (p Payment, err error) {
	err = row.Scan(&p.Reference, &p.BusinessID, &p.Plan, &p.Interval, &p.Amount, &p.Status, &p.Created, &p.PaidAt, &p.Fees, &p.Refunded, &p.CheckoutURL)
	return
}
func (s *Service) Verify(ctx context.Context, u *db.User, ref string, webhook bool) (*Payment, error) {
	if !strings.HasPrefix(ref, "sub_") || len(ref) > 100 {
		return nil, errors.New("invalid subscription payment reference")
	}
	p, err := scanPayment(s.Pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM marketplace_subscription_payments WHERE reference=$1`, ref))
	if err != nil {
		return nil, err
	}
	if !webhook {
		if err = s.authorize(ctx, u, p.BusinessID); err != nil {
			return nil, err
		}
	}
	// Verification stays available after checkout is disabled, so settled payments can still be reconciled.
	v, err := s.Gateway.VerifyPayment(ctx, ref)
	if err != nil {
		_, _ = s.Pool.Exec(ctx, `UPDATE marketplace_subscription_payments SET verified_at=now() WHERE reference=$1`, ref)
		return nil, err
	}
	if err = ValidatePayment(p, v); err != nil {
		_, _ = s.Pool.Exec(ctx, `INSERT INTO marketplace_subscription_audit(business_id,reference,actor_id,action,detail) VALUES($1,$2,'paystack','reconciliation_rejected',$3)`, p.BusinessID, ref, err.Error())
		return nil, err
	}

	var verifiedProvider struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(v.RawResponse, &verifiedProvider)
	if lister, ok := s.Gateway.(interface {
		ListTransactionRefunds(context.Context, string) ([]payment.TransactionRefundSummary, error)
	}); ok && verifiedProvider.Data.ID.String() != "" {
		refunds, e := lister.ListTransactionRefunds(ctx, verifiedProvider.Data.ID.String())
		if e != nil {
			return nil, e
		}
		v.Refunds = refunds
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockStore(ctx, tx, p.BusinessID); err != nil {
		return nil, err
	}
	p, err = scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM marketplace_subscription_payments WHERE reference=$1 FOR UPDATE`, ref))
	if err != nil {
		return nil, err
	}
	var fees *int64
	var raw struct {
		Data struct {
			Fees *int64 `json:"fees"`
		} `json:"data"`
	}
	_ = json.Unmarshal(v.RawResponse, &raw)
	if raw.Data.Fees != nil && *raw.Data.Fees >= 0 {
		fees = raw.Data.Fees
	}
	refunded := int64(0)
	for _, r := range v.Refunds {
		if r.Status == "successful" {
			refunded += r.AmountKobo
		}
	}
	if refunded > p.Amount {
		return nil, errors.New("invalid refunded total")
	}
	if refunded < p.Refunded {
		refunded = p.Refunded
	}
	status := v.Status
	if p.Status == "successful" {
		status = "successful"
	}
	if status == "successful" && p.Status != "successful" {
		var end *time.Time
		err = tx.QueryRow(ctx, `SELECT max(expires_at) FROM marketplace_subscription_terms WHERE business_id=$1 AND NOT voided`, p.BusinessID).Scan(&end)
		if err != nil {
			return nil, err
		}
		start := time.Now().UTC()
		if end != nil && end.After(start) {
			start = *end
		}
		expires := PeriodEnd(start, p.Interval)
		_, err = tx.Exec(ctx, `INSERT INTO marketplace_subscription_terms(reference,business_id,plan_code,starts_at,expires_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, ref, p.BusinessID, p.Plan, start, expires)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE marketplace_store_subscriptions SET status=CASE WHEN status='suspended' THEN status ELSE 'active' END,cancelled_at=NULL,updated_at=now() WHERE business_id=$1`, p.BusinessID)
		if err != nil {
			return nil, err
		}
		event := "Subscription activation"
		if end != nil {
			event = "Successful renewal / plan change"
		}
		if err = email(ctx, tx, ref+":paid", p.BusinessID, event, fmt.Sprintf("Your %s %s store term is verified. Starts %s; expires %s UTC. Administrative store approval is still required. No automatic charges.", p.Plan, p.Interval, start.Format(time.RFC3339), expires.Format(time.RFC3339))); err != nil {
			return nil, err
		}
	}
	if status == "failed" && p.Status != "failed" {
		if err = email(ctx, tx, ref+":failed", p.BusinessID, "Store subscription payment failed", "Your payment was not successful. Verify it again or create a new checkout to retry."); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE marketplace_subscription_payments SET status=$2,paid_at=CASE WHEN $2='successful' THEN coalesce(paid_at,$3,now()) ELSE paid_at END,fees_kobo=coalesce($4,fees_kobo),refunded_kobo=$5,verified_at=now() WHERE reference=$1`, ref, status, v.PaidAt, fees, refunded)
	if err != nil {
		return nil, err
	}
	if refunded == p.Amount {
		_, err = tx.Exec(ctx, `UPDATE marketplace_subscription_terms SET voided=true WHERE reference=$1`, ref)
		if err != nil {
			return nil, err
		}
	}
	actor := "paystack"
	if u != nil {
		actor = u.ID
	}
	if err = audit(ctx, tx, p.BusinessID, ref, actor, "payment_reconciliation", status); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	result, err := scanPayment(s.Pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM marketplace_subscription_payments WHERE reference=$1`, ref))
	return &result, err
}
func ValidatePayment(p Payment, v *payment.VerifyPaymentOutput) error {
	if v == nil {
		return payment.ErrMalformedResponse
	}
	if v.PaymentReference != p.Reference {
		return payment.ErrReferenceMismatch
	}
	if v.AmountKobo != p.Amount {
		return payment.ErrAmountMismatch
	}
	if v.Currency != "NGN" {
		return payment.ErrCurrencyMismatch
	}
	if v.Status != "successful" && v.Status != "pending" && v.Status != "failed" {
		return payment.ErrMalformedResponse
	}
	return nil
}
func (s *Service) Webhook(ctx context.Context, signature string, body []byte) error {
	if !s.Gateway.VerifyWebhookSignature(signature, body) {
		return payment.ErrInvalidSignature
	}
	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference            string      `json:"reference"`
			TransactionReference string      `json:"transaction_reference"`
			Transaction          interface{} `json:"transaction"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}
	if event.Data.TransactionReference != "" {
		event.Data.Reference = event.Data.TransactionReference
	}
	if !strings.HasPrefix(event.Data.Reference, "sub_") {
		return nil
	}
	switch event.Event {
	case "charge.success", "charge.failed", "refund.processed", "refund.failed":
		_, err := s.Verify(ctx, nil, event.Data.Reference, true)
		return err
	}
	return nil
}

type Term struct {
	Reference string    `json:"reference"`
	Plan      string    `json:"plan"`
	Starts    time.Time `json:"starts_at"`
	Expires   time.Time `json:"expires_at"`
}
type Dashboard struct {
	BusinessID      string     `json:"business_id"`
	Store           string     `json:"store_name"`
	Status          string     `json:"status"`
	Current         *Term      `json:"current"`
	Scheduled       []Term     `json:"scheduled"`
	Payments        []Payment  `json:"payments"`
	Used            int        `json:"product_usage"`
	Limit           *int       `json:"product_limit"`
	Days            int        `json:"remaining_days"`
	GraceUntil      *time.Time `json:"grace_until"`
	Enabled         bool       `json:"enabled"`
	CheckoutEnabled bool       `json:"checkout_enabled"`
}

func (s *Service) Dashboard(ctx context.Context, u *db.User, b string) (*Dashboard, error) {
	if err := s.authorize(ctx, u, b); err != nil {
		return nil, err
	}
	d := &Dashboard{BusinessID: b, Enabled: s.Enabled, CheckoutEnabled: s.CheckoutEnabled, Status: "pending", Scheduled: []Term{}, Payments: []Payment{}}
	var legacy bool
	var enabledAt *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT b.name,coalesce(s.status,'pending'),coalesce(s.legacy_store,false),cfg.enabled_at,(SELECT count(*) FROM marketplace_products p WHERE p.business_id=b.id AND p.status<>'archived') FROM marketplace_businesses b CROSS JOIN marketplace_subscription_settings cfg LEFT JOIN marketplace_store_subscriptions s ON s.business_id=b.id WHERE b.id=$1`, b).Scan(&d.Store, &d.Status, &legacy, &enabledAt, &d.Used)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT reference,plan_code,starts_at,expires_at FROM marketplace_subscription_terms WHERE business_id=$1 AND NOT voided AND expires_at>now() ORDER BY starts_at`, b)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var t Term
		if err = rows.Scan(&t.Reference, &t.Plan, &t.Starts, &t.Expires); err != nil {
			return nil, err
		}
		if !t.Starts.After(now) {
			d.Current = &t
			d.Days = int(t.Expires.Sub(now).Hours() / 24)
			for _, p := range Plans() {
				if p.Code == t.Plan {
					d.Limit = p.Limit
				}
			}
		} else {
			d.Scheduled = append(d.Scheduled, t)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if legacy && enabledAt != nil {
		grace := enabledAt.AddDate(0, 0, 30)
		d.GraceUntil = &grace
	}
	if d.Status != "suspended" {
		if d.Current != nil {
			if d.Status != "cancelled" {
				d.Status = "active"
			}
		} else if d.GraceUntil != nil && d.GraceUntil.After(now) {
			d.Status = "grace"
		} else if d.Status != "pending" {
			d.Status = "expired"
		}
	}
	rows, err = s.Pool.Query(ctx, `SELECT `+paymentColumns+` FROM marketplace_subscription_payments WHERE business_id=$1 ORDER BY created_at DESC LIMIT 250`, b)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, e := scanPayment(rows)
		if e != nil {
			return nil, e
		}
		d.Payments = append(d.Payments, p)
	}
	return d, rows.Err()
}
func (s *Service) Cancel(ctx context.Context, u *db.User, b string) error {
	if err := s.authorize(ctx, u, b); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockStore(ctx, tx, b); err != nil {
		return err
	}
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_subscription_terms WHERE business_id=$1 AND starts_at>now() AND NOT voided) OR EXISTS(SELECT 1 FROM marketplace_subscription_payments WHERE business_id=$1 AND status='pending')`, b).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return errors.New("reconcile pending payments or contact support about the prepaid future term before cancellation")
	}
	tag, err := tx.Exec(ctx, `UPDATE marketplace_store_subscriptions SET status='cancelled',cancelled_at=now(),updated_at=now() WHERE business_id=$1 AND status NOT IN ('cancelled','suspended')`, b)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if err = email(ctx, tx, "cancel:"+b+":"+time.Now().UTC().Format("2006-01-02T15:04:05.000000000"), b, "Store subscription cancellation", "Your subscription is cancelled. Any current paid term remains usable until expiry. You will not be charged automatically."); err != nil {
		return err
	}
	if err = audit(ctx, tx, b, "", u.ID, "cancellation", "paid access retained until expiry"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) Suspend(ctx context.Context, u *db.User, b, reason string, suspend bool) error {
	if u == nil || u.PrimaryRole != "super_admin" {
		return ErrForbidden
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("an administrative reason is required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockStore(ctx, tx, b); err != nil {
		return err
	}
	state := "active"
	if suspend {
		state = "suspended"
	}
	_, err = tx.Exec(ctx, `INSERT INTO marketplace_store_subscriptions(business_id,status) VALUES($1,$2) ON CONFLICT(business_id) DO UPDATE SET status=$2,updated_at=now()`, b, state)
	if err != nil {
		return err
	}
	if err = audit(ctx, tx, b, "", u.ID, state, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Report struct {
	Gross       int64                    `json:"gross_kobo"`
	Refunds     int64                    `json:"refunds_kobo"`
	KnownFees   int64                    `json:"known_fees_kobo"`
	Net         int64                    `json:"net_before_unknown_fees_kobo"`
	UnknownFees int                      `json:"payments_with_unknown_fees"`
	Monthly     int64                    `json:"monthly_revenue_kobo"`
	Yearly      int64                    `json:"yearly_revenue_kobo"`
	ThisMonth   int64                    `json:"this_month_revenue_kobo"`
	Active      int                      `json:"active_paid_stores"`
	Expired     int                      `json:"expired_subscriptions"`
	Pending     int                      `json:"pending_payments"`
	Failed      int                      `json:"failed_payments"`
	Payments    []Payment                `json:"payments"`
	Stores      []map[string]interface{} `json:"stores"`
	Audit       []map[string]interface{} `json:"audit"`
}

func (s *Service) Report(ctx context.Context, u *db.User, search, plan, status string, all ...bool) (*Report, error) {
	if u == nil || u.PrimaryRole != "super_admin" {
		return nil, ErrForbidden
	}
	var rowLimit *int
	if len(all) == 0 || !all[0] {
		n := 1000
		rowLimit = &n
	}
	r := &Report{Payments: []Payment{}, Stores: []map[string]interface{}{}, Audit: []map[string]interface{}{}}
	err := s.Pool.QueryRow(ctx, `SELECT coalesce(sum(amount_kobo) FILTER(WHERE status='successful'),0),coalesce(sum(refunded_kobo) FILTER(WHERE status='successful'),0),coalesce(sum(fees_kobo) FILTER(WHERE status='successful'),0),count(*) FILTER(WHERE status='successful' AND fees_kobo IS NULL),coalesce(sum(amount_kobo-refunded_kobo) FILTER(WHERE status='successful' AND interval='monthly'),0),coalesce(sum(amount_kobo-refunded_kobo) FILTER(WHERE status='successful' AND interval='yearly'),0),coalesce(sum(amount_kobo-refunded_kobo) FILTER(WHERE status='successful' AND paid_at>=date_trunc('month',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),0),count(*) FILTER(WHERE status='pending'),count(*) FILTER(WHERE status='failed') FROM marketplace_subscription_payments`).Scan(&r.Gross, &r.Refunds, &r.KnownFees, &r.UnknownFees, &r.Monthly, &r.Yearly, &r.ThisMonth, &r.Pending, &r.Failed)
	if err != nil {
		return nil, err
	}
	r.Net = r.Gross - r.Refunds - r.KnownFees
	err = s.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE s.status<>'suspended' AND EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=s.business_id AND NOT t.voided AND t.starts_at<=now() AND t.expires_at>now())),count(*) FILTER(WHERE s.status<>'suspended' AND EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=s.business_id) AND NOT EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=s.business_id AND NOT t.voided AND t.expires_at>now())) FROM marketplace_store_subscriptions s`).Scan(&r.Active, &r.Expired)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT p.reference,p.business_id,p.plan_code,p.interval,p.amount_kobo,p.status,p.created_at,p.paid_at,p.fees_kobo,p.refunded_kobo,p.checkout_url FROM marketplace_subscription_payments p JOIN marketplace_businesses b ON b.id=p.business_id JOIN users u ON u.id=b.owner_user_id WHERE ($1='' OR b.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%' OR b.owner_user_id=$1) AND ($2='' OR p.plan_code=$2) AND ($3='' OR p.status=$3 OR
 ($3='suspended' AND EXISTS(SELECT 1 FROM marketplace_store_subscriptions s WHERE s.business_id=p.business_id AND s.status='suspended')) OR
 ($3='cancelled' AND EXISTS(SELECT 1 FROM marketplace_store_subscriptions s WHERE s.business_id=p.business_id AND s.status='cancelled')) OR
 ($3='active' AND EXISTS(SELECT 1 FROM marketplace_store_subscriptions s JOIN marketplace_subscription_terms t ON t.business_id=s.business_id WHERE s.business_id=p.business_id AND s.status NOT IN ('suspended','cancelled') AND NOT t.voided AND t.starts_at<=now() AND t.expires_at>now())) OR
 ($3='expired' AND EXISTS(SELECT 1 FROM marketplace_store_subscriptions s WHERE s.business_id=p.business_id AND s.status='expired')) OR
 ($3='grace' AND EXISTS(SELECT 1 FROM marketplace_store_subscriptions s CROSS JOIN marketplace_subscription_settings cfg WHERE s.business_id=p.business_id AND s.legacy_store AND s.status<>'suspended' AND now()<cfg.enabled_at+interval '30 days'))) ORDER BY p.created_at DESC LIMIT $4`, search, plan, status, rowLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		p, e := scanPayment(rows)
		if e != nil {
			return nil, e
		}
		r.Payments = append(r.Payments, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = s.Pool.Query(ctx, `SELECT b.id,b.name,b.owner_user_id,u.email,CASE WHEN s.status='suspended' THEN 'suspended' WHEN t.reference IS NOT NULL THEN CASE WHEN s.status='cancelled' THEN 'cancelled' ELSE 'active' END WHEN s.legacy_store AND now()<cfg.enabled_at+interval '30 days' THEN 'grace' WHEN s.status='pending' THEN 'pending' ELSE 'expired' END AS state,coalesce(t.plan_code,''),t.starts_at,t.expires_at FROM marketplace_store_subscriptions s JOIN marketplace_businesses b ON b.id=s.business_id JOIN users u ON u.id=b.owner_user_id CROSS JOIN marketplace_subscription_settings cfg LEFT JOIN LATERAL (SELECT * FROM marketplace_subscription_terms x WHERE x.business_id=s.business_id AND NOT x.voided AND x.starts_at<=now() AND x.expires_at>now() ORDER BY x.starts_at DESC LIMIT 1) t ON true WHERE ($1='' OR b.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%') AND ($2='' OR t.plan_code=$2) ORDER BY b.name LIMIT 1000`, search, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, owner, em, state, p string
		var start, end *time.Time
		if err = rows.Scan(&id, &name, &owner, &em, &state, &p, &start, &end); err != nil {
			return nil, err
		}
		if status == "" || status == state {
			r.Stores = append(r.Stores, map[string]interface{}{"business_id": id, "store_name": name, "owner_user_id": owner, "owner_email": em, "status": state, "plan": p, "starts_at": start, "expires_at": end})
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = s.Pool.Query(ctx, `SELECT a.id,a.business_id,a.reference,a.actor_id,a.action,a.detail,a.created_at FROM marketplace_subscription_audit a JOIN marketplace_businesses b ON b.id=a.business_id JOIN users u ON u.id=b.owner_user_id WHERE ($1='' OR b.name ILIKE '%'||$1||'%' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%') ORDER BY a.id DESC LIMIT 250`, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var b, ref, actor, action, detail string
		var at time.Time
		if err = rows.Scan(&id, &b, &ref, &actor, &action, &detail, &at); err != nil {
			return nil, err
		}
		r.Audit = append(r.Audit, map[string]interface{}{"id": id, "business_id": b, "reference": ref, "actor_id": actor, "action": action, "detail": detail, "created_at": at})
	}
	return r, rows.Err()
}

// Tick is safe on multiple Render instances: advisory transaction lock for lifecycle,
// SKIP LOCKED for mail delivery. Mail failures retry without changing payment state.
func (s *Service) Tick(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(707707)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	_, err = tx.Exec(ctx, `UPDATE marketplace_store_subscriptions s SET status='expired',updated_at=now() WHERE status IN ('active','cancelled') AND NOT EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=s.business_id AND NOT t.voided AND t.expires_at>now())`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO marketplace_subscription_emails(event_key,business_id,subject,body) SELECT t.reference||CASE WHEN t.expires_at<=now() THEN ':expired' ELSE ':reminder' END,t.business_id,CASE WHEN t.expires_at<=now() THEN 'Store subscription expired' ELSE 'Store subscription expires soon' END,'Your paid store term ends at '||t.expires_at::text||'. Renew from the seller subscription dashboard. Existing paid orders can still be fulfilled.' FROM marketplace_subscription_terms t WHERE NOT t.voided AND t.starts_at<=now() AND t.expires_at<=now()+interval '7 days' AND NOT EXISTS(SELECT 1 FROM marketplace_subscription_terms n WHERE n.business_id=t.business_id AND NOT n.voided AND n.expires_at>t.expires_at) ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	// Reconcile bounded pending/failed payments even when a webhook is lost. API also supports explicit retries.
	rows, err := s.Pool.Query(ctx, `SELECT reference FROM marketplace_subscription_payments WHERE status='pending' AND created_at<now()-interval '2 minutes' AND (verified_at IS NULL OR verified_at<now()-interval '10 minutes') ORDER BY created_at LIMIT 10`)
	if err != nil {
		return err
	}
	refs := []string{}
	for rows.Next() {
		var ref string
		if err = rows.Scan(&ref); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, ref)
	}
	rows.Close()
	for _, ref := range refs {
		_, _ = s.Verify(ctx, nil, ref, true)
	}
	if s.Email == nil {
		return nil
	}
	for i := 0; i < 20; i++ {
		if err = s.deliverOne(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) deliverOne(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var key, subject, body, address, name string
	err = tx.QueryRow(ctx, `SELECT e.event_key,e.subject,e.body,u.email,b.name FROM marketplace_subscription_emails e JOIN marketplace_businesses b ON b.id=e.business_id JOIN users u ON u.id=b.owner_user_id WHERE e.sent_at IS NULL AND e.next_attempt_at<=now() ORDER BY e.next_attempt_at LIMIT 1 FOR UPDATE OF e SKIP LOCKED`).Scan(&key, &subject, &body, &address, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	id, sendErr := s.Email.SendNotificationEmail(ctx, address, name, subject, "", body)
	if sendErr != nil {
		_, err = tx.Exec(ctx, `UPDATE marketplace_subscription_emails SET attempts=attempts+1,next_attempt_at=now()+interval '15 minutes' WHERE event_key=$1`, key)
	} else {
		_, err = tx.Exec(ctx, `UPDATE marketplace_subscription_emails SET attempts=attempts+1,sent_at=now(),provider_message_id=$2 WHERE event_key=$1`, key, id)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) Run(ctx context.Context) {
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				work, cancel := context.WithTimeout(ctx, 55*time.Second)
				_ = s.Tick(work)
				cancel()
			}
		}
	}()
}

func (s *Service) AnalyticsAllowed(ctx context.Context, u *db.User, b string) (bool, error) {
	if err := s.authorize(ctx, u, b); err != nil {
		return false, err
	}
	var allowed bool
	err := s.Pool.QueryRow(ctx, `SELECT NOT cfg.enabled OR (coalesce(sub.status,'')<>'suspended' AND ((coalesce(sub.legacy_store,false) AND now()<cfg.enabled_at+interval '30 days') OR EXISTS(SELECT 1 FROM marketplace_subscription_terms t WHERE t.business_id=$1 AND t.plan_code='business' AND NOT t.voided AND t.starts_at<=now() AND t.expires_at>now()))) FROM marketplace_subscription_settings cfg LEFT JOIN marketplace_store_subscriptions sub ON sub.business_id=$1 WHERE cfg.id`, b).Scan(&allowed)
	return allowed, err
}

func (s *Service) RetryCheckout(ctx context.Context, u *db.User, ref string) (*Checkout, error) {
	var business string
	var in CheckoutInput
	err := s.Pool.QueryRow(ctx, `SELECT business_id,plan_code,interval,idempotency_key FROM marketplace_subscription_payments WHERE reference=$1 AND status='pending'`, ref).Scan(&business, &in.Plan, &in.Interval, &in.Key)
	if err != nil {
		return nil, err
	}
	return s.Checkout(ctx, u, business, in)
}
