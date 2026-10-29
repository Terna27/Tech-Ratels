package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func membershipHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func (h *MemberHandler) Card(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	card, err := h.authService.MemberCard(r.Context(), GetCurrentUser(r.Context()).ID)
	if err != nil {
		ErrorJSON(w, 400, "CARD_UNAVAILABLE", "Membership card unavailable", nil)
		return
	}
	png, err := qrcode.Encode(h.authService.CardVerificationURL(card.VerificationPath), qrcode.Medium, 320)
	if err != nil {
		ErrorJSON(w, 500, "QR_UNAVAILABLE", "Could not generate verification QR", nil)
		return
	}
	data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	if chi.URLParam(r, "format") == "download" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="ratels-membership-card.html"`)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src https: data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
		_ = cardTemplate.Execute(w, struct {
			Card *service.MembershipCard
			QR   template.URL
			URL  string
		}{card, template.URL(data), h.authService.CardVerificationURL(card.VerificationPath)})
		return
	}
	JSON(w, 200, struct {
		*service.MembershipCard
		QR string `json:"qr_image_data"`
	}{card, data})
}

var cardTemplate = template.Must(template.New("card").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><meta name="referrer" content="no-referrer"><title>Ratels membership card</title><style>body{font:18px system-ui;background:#f0f4f1;color:#123e2a;padding:24px}article{max-width:560px;margin:auto;background:white;border:3px solid #065f46;border-radius:20px;padding:24px}img{object-fit:cover} .photo{width:100px;height:100px;border-radius:12px}.qr{width:180px;height:180px;float:right}p{overflow-wrap:anywhere}@media(max-width:480px){.qr{float:none;display:block;margin:0 auto 20px}}@media print{body{background:white;padding:0}article{break-inside:avoid}}</style><article><h1>Nigerian Ratels</h1><p>Digital membership card</p><img class="qr" alt="Scan to check current membership" src="{{.QR}}">{{if .Card.Photo}}<img class="photo" alt="Member photo" referrerpolicy="no-referrer" src="{{.Card.Photo}}">{{end}}<h2>{{.Card.Name}}</h2><p>{{.Card.Reference}}</p><p>Status at download: <strong>{{.Card.Status}}</strong></p><p>Verify current status before relying on this card.</p><p><a href="{{.URL}}">Check current membership</a></p></article></html>`))

func (h *MemberHandler) VerifyCard(w http.ResponseWriter, r *http.Request) {
	membershipHeaders(w)
	if ok, retry := h.verificationLimiter.Allow(security.GetClientIP(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		ErrorJSON(w, 429, "RATE_LIMITED", "Too many verification requests", nil)
		return
	}
	out, err := h.authService.VerifyMembership(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		ErrorJSON(w, 404, "MEMBERSHIP_NOT_FOUND", "Membership verification link not found", nil)
		return
	}
	JSON(w, 200, out)
}
func (h *MemberHandler) MembershipStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		ErrorJSON(w, 400, "INVALID_REQUEST", "Invalid status request", nil)
		return
	}
	err := h.authService.ChangeMembershipStatus(r.Context(), GetCurrentUser(r.Context()).ID, chi.URLParam(r, "id"), in.Status, in.Reason)
	if err != nil {
		code := 400
		if errors.Is(err, service.ErrMembershipForbidden) {
			code = 403
		}
		ErrorJSON(w, code, "MEMBERSHIP_STATUS_REFUSED", err.Error(), nil)
		return
	}
	JSON(w, 200, map[string]string{"status": in.Status})
}
