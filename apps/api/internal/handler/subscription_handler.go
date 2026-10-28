package handler

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"ratels-api/internal/security"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"ratels-api/internal/payment"
	"ratels-api/internal/subscription"
)

type SubscriptionHandler struct {
	Service *subscription.Service
	limiter *security.DownloadRateLimiter
}

func (h *SubscriptionHandler) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := security.GetClientIP(r)
		if u := GetCurrentUser(r.Context()); u != nil {
			key = u.ID
		}
		if ok, retry := h.limiter.Allow(key); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			ErrorJSON(w, 429, "RATE_LIMIT_EXCEEDED", "Too many subscription requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (h *SubscriptionHandler) Routes(r chi.Router) {
	h.limiter = security.NewDownloadRateLimiter(60, time.Minute)
	r.Get("/subscriptions/plans", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, 200, map[string]interface{}{"plans": subscription.Plans(), "enabled": h.Service.Enabled, "checkout_enabled": h.Service.CheckoutEnabled})
	})
	r.Post("/subscriptions/webhooks/paystack", h.Webhook)
	r.Route("/seller/businesses/{businessID}/subscription", func(r chi.Router) {
		r.Use(RequireAuth)
		r.Use(h.limit)
		r.Get("/", h.Status)
		r.Get("/payments", h.History)
		r.Post("/checkout", h.Checkout)
		r.Post("/renew", h.Checkout)
		r.Post("/change-plan", h.Checkout)
		r.Post("/cancel", h.Cancel)
	})
	r.With(RequireAuth, h.limit).Post("/subscriptions/payments/{reference}/verify", h.Verify)
	r.With(RequireAuth, h.limit).Post("/subscriptions/payments/{reference}/retry-checkout", h.RetryCheckout)
	r.Route("/admin/subscriptions", func(r chi.Router) {
		r.Use(RequireRole("super_admin"))
		r.Get("/", h.Report)
		r.Get("/export", h.Report)
		r.Post("/payments/{reference}/reconcile", h.Verify)
		r.Post("/{businessID}/suspend", h.Suspend)
		r.Post("/{businessID}/resume", h.Suspend)
	})
}
func subscriptionError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, subscription.ErrForbidden) {
		status = 403
	}
	if errors.Is(err, subscription.ErrDisabled) {
		status = 503
	}
	if errors.Is(err, subscription.ErrConflict) {
		status = 409
	}
	if errors.Is(err, pgx.ErrNoRows) {
		status = 404
	}
	ErrorJSON(w, status, "SUBSCRIPTION_ERROR", err.Error(), nil)
}
func (h *SubscriptionHandler) Status(w http.ResponseWriter, r *http.Request) {
	d, err := h.Service.Dashboard(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "businessID"))
	if err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, d)
}
func (h *SubscriptionHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var in subscription.CheckoutInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		subscriptionError(w, err)
		return
	}
	d, err := h.Service.Checkout(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "businessID"), in)
	if err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, d)
}
func (h *SubscriptionHandler) Verify(w http.ResponseWriter, r *http.Request) {
	d, err := h.Service.Verify(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "reference"), false)
	if err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, d)
}
func (h *SubscriptionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.Service.Cancel(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "businessID")); err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, map[string]string{"status": "cancelled"})
}
func (h *SubscriptionHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		subscriptionError(w, err)
		return
	}
	suspend := true
	if len(r.URL.Path) >= 7 && r.URL.Path[len(r.URL.Path)-7:] == "/resume" {
		suspend = false
	}
	if err := h.Service.Suspend(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "businessID"), in.Reason, suspend); err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, map[string]bool{"suspended": suspend})
}
func (h *SubscriptionHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		subscriptionError(w, err)
		return
	}
	err = h.Service.Webhook(r.Context(), r.Header.Get("x-paystack-signature"), body)
	if err != nil {
		if errors.Is(err, payment.ErrInvalidSignature) {
			ErrorJSON(w, 401, "INVALID_SIGNATURE", "Invalid webhook signature", nil)
		} else {
			ErrorJSON(w, 503, "RECONCILIATION_RETRY", "Payment reconciliation will be retried", nil)
		}
		return
	}
	w.WriteHeader(200)
}
func (h *SubscriptionHandler) Report(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d, err := h.Service.Report(r.Context(), GetCurrentUser(r.Context()), q.Get("search"), q.Get("plan"), q.Get("status"), strings.HasSuffix(r.URL.Path, "/export"))
	if err != nil {
		subscriptionError(w, err)
		return
	}
	if r.URL.Path[len(r.URL.Path)-7:] == "/export" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="store-subscription-revenue.csv"`)
		c := csv.NewWriter(w)
		defer c.Flush()
		_ = c.Write([]string{"reference", "business_id", "plan", "interval", "status", "gross_kobo", "refunds_kobo", "fees_kobo", "net_before_unknown_fees_kobo"})
		for _, p := range d.Payments {
			fees := "unknown"
			net := "0"
			gross := "0"
			refund := "0"
			if p.Status == "successful" {
				gross = strconv.FormatInt(p.Amount, 10)
				refund = strconv.FormatInt(p.Refunded, 10)
				n := p.Amount - p.Refunded
				if p.Fees != nil {
					fees = strconv.FormatInt(*p.Fees, 10)
					n -= *p.Fees
				}
				net = strconv.FormatInt(n, 10)
			}
			_ = c.Write([]string{p.Reference, p.BusinessID, p.Plan, p.Interval, p.Status, gross, refund, fees, net})
		}
		return
	}
	JSON(w, 200, d)
}

func (h *SubscriptionHandler) RetryCheckout(w http.ResponseWriter, r *http.Request) {
	d, err := h.Service.RetryCheckout(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "reference"))
	if err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, d)
}

func (h *SubscriptionHandler) History(w http.ResponseWriter, r *http.Request) {
	d, err := h.Service.Dashboard(r.Context(), GetCurrentUser(r.Context()), chi.URLParam(r, "businessID"))
	if err != nil {
		subscriptionError(w, err)
		return
	}
	JSON(w, 200, d.Payments)
}
