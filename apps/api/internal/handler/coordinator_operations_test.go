package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"ratels-api/internal/db"
	"ratels-api/internal/service"
	"strings"
	"sync"
	"testing"
	"time"
)

func operationRequest(h *TestHarness, token, method, path string, body interface{}) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Router.ServeHTTP(w, r)
	return w
}
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("got %d want %d: %s", w.Code, want, w.Body.String())
	}
}
func operationalUser(t *testing.T, h *TestHarness, role string) (string, string) {
	t.Helper()
	return createTestUserWithRole(t, h, fmt.Sprintf("phase70_%d", time.Now().UnixNano()), "Phase70 "+role, role)
}
func home(t *testing.T, h *TestHarness, user string, state, lga string) {
	t.Helper()
	tag, err := h.Pool.Exec(context.Background(), `UPDATE member_profiles SET home_state_id=$2,home_lga_id=$3 WHERE user_id=$1`, user, state, lga)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("profile fixture %v %v", tag, err)
	}
}

func TestPhase70CoordinatorMembersScopeAndAudit(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	la, lga, kn, klga, _ := getLagosAndKanoData(t, h)
	id, token := operationalUser(t, h, "member")
	app := insertTestAppointment(t, h, id, "state_coordinator", "Operational Lagos", "state", &la.ID, "active", time.Now().Add(-time.Hour), nil)
	member, _ := operationalUser(t, h, "member")
	outside, _ := operationalUser(t, h, "member")
	home(t, h, member, la.ID, lga.ID)
	home(t, h, outside, kn.ID, klga.ID)
	t.Run("scoped directory and profile correction", func(t *testing.T) {
		w := operationRequest(h, token, "GET", "/coordinator/members", nil)
		assertStatus(t, w, 200)
		if !strings.Contains(w.Body.String(), member) || strings.Contains(w.Body.String(), outside) {
			t.Fatal("member directory leaked or omitted jurisdiction")
		}
		in := service.MemberCorrection{StateID: la.ID, LGAID: lga.ID, Address: "Confirmed local address", Reason: "Member confirmed correction"}
		assertStatus(t, operationRequest(h, token, "PATCH", "/coordinator/members/"+member+"/profile", in), 200)
		var n int
		if err := h.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor_id=$1 AND resource_id=$2 AND action='correct_member_geography'`, id, member).Scan(&n); err != nil || n != 1 {
			t.Fatal("correction audit missing", n, err)
		}
		in.StateID = kn.ID
		in.LGAID = klga.ID
		assertStatus(t, operationRequest(h, token, "PATCH", "/coordinator/members/"+member+"/profile", in), 403)
		in.StateID = la.ID
		in.LGAID = lga.ID
		assertStatus(t, operationRequest(h, token, "PATCH", "/coordinator/members/"+outside+"/profile", in), 403)
		assertStatus(t, operationRequest(h, token, "PATCH", "/coordinator/members/"+member+"/profile", map[string]string{"primary_role": "super_admin"}), 400)
	})
	t.Run("revoked and role-label-only actors denied", func(t *testing.T) {
		_, err := h.Pool.Exec(ctx, `UPDATE appointments SET status='suspended' WHERE id=$1`, app)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, operationRequest(h, token, "GET", "/coordinator/dashboard", nil), 403)
		_, err = h.Pool.Exec(ctx, `UPDATE appointments SET status='active' WHERE id=$1`, app)
		if err != nil {
			t.Fatal(err)
		}
		_, fake := operationalUser(t, h, "state_coordinator")
		assertStatus(t, operationRequest(h, fake, "GET", "/coordinator/dashboard", nil), 403)
		assertStatus(t, operationRequest(h, "", "GET", "/coordinator/members", nil), 401)
	})
	t.Run("cookie profile mutation requires CSRF", func(t *testing.T) {
		r := httptest.NewRequest("PATCH", "/api/v1/coordinator/members/"+member+"/profile", strings.NewReader(`{}`))
		r.AddCookie(&http.Cookie{Name: "ratels_session", Value: token})
		w := httptest.NewRecorder()
		h.Router.ServeHTTP(w, r)
		assertStatus(t, w, 403)
	})
	t.Run("regional overview and export are scoped", func(t *testing.T) {
		w := operationRequest(h, token, "GET", "/coordinator/reports", nil)
		assertStatus(t, w, 200)
		if strings.Contains(w.Body.String(), kn.Name) || strings.Contains(w.Body.String(), klga.ID) {
			t.Fatal("report exposes another state")
		}
		assertStatus(t, operationRequest(h, token, "GET", "/admin/overview", nil), 200)
		w = operationRequest(h, token, "GET", "/coordinator/reports/export", nil)
		assertStatus(t, w, 200)
		if !strings.Contains(w.Header().Get("Content-Type"), "csv") || strings.Contains(w.Body.String(), kn.Name) {
			t.Fatal("export not scoped")
		}
		assertStatus(t, operationRequest(h, token, "GET", "/coordinator/reports?from=2026-10-30&until=2026-10-01", nil), 400)
	})
}

func TestPhase70ZonalAndLGAScopes(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	la, lga, kn, _, zoneCode := getLagosAndKanoData(t, h)
	for _, role := range []string{"zonal_coordinator", "lga_coordinator", "lga_representative"} {
		t.Run(role, func(t *testing.T) {
			id, token := operationalUser(t, h, "member")
			kind, target := "lga", lga.ID
			if role == "zonal_coordinator" {
				kind = "zone"
				target = zoneCode
			}
			insertTestAppointment(t, h, id, role, "Operational jurisdiction", kind, &target, "active", time.Now().Add(-time.Hour), nil)
			w := operationRequest(h, token, "GET", "/coordinator/dashboard", nil)
			assertStatus(t, w, 200)
			var report service.OperationalReport
			if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.Scope.National {
				t.Fatal("regional actor acquired national scope")
			}
			for _, a := range report.Geography {
				if a["state_id"] == kn.ID {
					t.Fatal("cross-zone data exposed")
				}
				if role != "zonal_coordinator" && a["lga_id"] != lga.ID {
					t.Fatal("LGA scope broadened")
				}
			}
			assertStatus(t, operationRequest(h, token, "GET", "/admin/overview", nil), 200)
			assertStatus(t, operationRequest(h, token, "GET", "/coordinator/events?state_id="+kn.ID, nil), 403)
			_ = la
		})
	}
}

func TestPhase70ActivityDraftAndAppointmentAuthority(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	la, lga, kn, klga, _ := getLagosAndKanoData(t, h)
	actor, token := operationalUser(t, h, "member")
	insertTestAppointment(t, h, actor, "state_coordinator", "Scheduling coordinator", "state", &la.ID, "active", time.Now().Add(-time.Hour), nil)
	in := service.CreateEventRequest{Title: "Phase70 draft workflow", StateID: la.ID, LGAID: lga.ID, Address: "Local hall", Landmark: "Meeting point", StartTime: time.Now().Add(time.Hour), EndTime: time.Now().Add(2 * time.Hour), PublicContact: "coordinator@example.test", Status: "draft"}
	w := operationRequest(h, token, "POST", "/activities/events", in)
	assertStatus(t, w, 201)
	var event db.EventInstance
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, operationRequest(h, "", "GET", "/activities/events?published_only=false", nil), 401)
	assertStatus(t, operationRequest(h, "", "GET", "/activities/events/"+event.ID, nil), 404)
	assertStatus(t, operationRequest(h, token, "GET", "/activities/events/"+event.ID, nil), 200)
	assertStatus(t, operationRequest(h, token, "POST", "/activities/events/"+event.ID+"/publish", nil), 200)
	assertStatus(t, operationRequest(h, "", "GET", "/activities/events/"+event.ID, nil), 200)
	in.StateID = kn.ID
	in.LGAID = klga.ID
	assertStatus(t, operationRequest(h, token, "POST", "/activities/events", in), 403)
	in.StateID = la.ID
	assertStatus(t, operationRequest(h, token, "POST", "/activities/events", in), 400)
	_, national := operationalUser(t, h, "national_admin")
	candidate, _ := operationalUser(t, h, "member")
	payload := service.CreateAppointmentRequest{UserID: candidate, RoleTitle: "LGA officer", RoleType: "lga_coordinator", JurisdictionType: "lga", JurisdictionID: &lga.ID, StartDate: time.Now().Add(-time.Hour), ApprovalReference: "PHASE70-AUTH"}
	assertStatus(t, operationRequest(h, national, "POST", "/leadership/appointments", payload), 403)
	_, forgedErr := service.NewLeadershipService(h.Repo, service.NewAuditService(h.Repo)).CreateAppointment(context.Background(), actor, "super_admin", payload)
	if forgedErr != service.ErrSuperAdminRequired {
		t.Fatalf("forged actor role granted appointment authority: %v", forgedErr)
	}
}

func TestPhase70InventoryScopeAndReceiptConcurrency(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	la, lga, kn, klga, _ := getLagosAndKanoData(t, h)
	adminID, _ := operationalUser(t, h, "super_admin")
	admin, err := h.Repo.Queries().GetUserByID(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	actor, token := operationalUser(t, h, "member")
	insertTestAppointment(t, h, actor, "state_coordinator", "Local warehouse coordinator", "state", &la.ID, "active", time.Now().Add(-time.Hour), nil)
	mk := func(name string, state, lga string) *db.GetStorageLocationByIDRow {
		t.Helper()
		loc, e := h.InventoryService.CreateLocation(ctx, &admin, service.CreateLocationRequest{Code: fmt.Sprintf("P70-%s-%d", name, time.Now().UnixNano()), Name: name, LocationType: "lga", StateID: &state, LGAID: &lga})
		if e != nil {
			t.Fatal(e)
		}
		return loc
	}
	local := mk("Local warehouse", la.ID, lga.ID)
	foreign := mk("Foreign warehouse", kn.ID, klga.ID)

	wrongZone := kn.ZoneID
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/locations", service.CreateLocationRequest{Code: "P70-BAD-GEO", Name: "Mismatched geography", LocationType: "lga", ZoneID: &wrongZone, StateID: &la.ID, LGAID: &lga.ID}), 400)
	w := operationRequest(h, token, "GET", "/inventory/locations", nil)
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), local.ID) || strings.Contains(w.Body.String(), foreign.ID) {
		t.Fatal("warehouse list not scoped")
	}
	assertStatus(t, operationRequest(h, token, "GET", "/inventory/locations/"+foreign.ID, nil), 403)
	item, e := h.InventoryService.CreateItem(ctx, &admin, service.CreateItemRequest{SKU: fmt.Sprintf("PH70-%d", time.Now().UnixNano()), Name: "Safety bags", Category: "tools"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = h.InventoryService.ReceiveStock(ctx, &admin, service.ReceiveStockRequest{LocationID: foreign.ID, ItemID: item.ID, Quantity: 100, MovementType: "initial_receipt"})
	if e != nil {
		t.Fatal(e)
	}
	transfer, e := h.InventoryService.CreateTransfer(ctx, &admin, service.CreateTransferInput{SourceLocationID: foreign.ID, DestinationLocationID: local.ID, AutoDispatch: true, Items: []service.CreateTransferItemInput{{ItemID: item.ID, DispatchedQuantity: 10}}})
	if e != nil {
		t.Fatal(e)
	}
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/transfers", service.CreateTransferInput{SourceLocationID: foreign.ID, DestinationLocationID: local.ID, Items: []service.CreateTransferItemInput{{ItemID: item.ID, DispatchedQuantity: 1}}}), 403)
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- operationRequest(h, token, "POST", "/inventory/transfers/"+transfer.ID+"/receive", service.ConfirmReceiptInput{Items: []service.ReceiptItemInput{}})
		}()
	}
	wg.Wait()
	close(results)
	for r := range results {
		assertStatus(t, r, 200)
	}
	var qty int64
	if err = h.Pool.QueryRow(ctx, `SELECT on_hand_quantity FROM stock_balances WHERE location_id=$1 AND item_id=$2`, local.ID, item.ID).Scan(&qty); err != nil || qty != 10 {
		t.Fatal("receipt credited more than once", qty, err)
	}
	w = operationRequest(h, token, "GET", "/inventory/ledger?limit=100", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), foreign.ID) {
		t.Fatal("foreign stock ledger leaked")
	}
	requesterID, _ := operationalUser(t, h, "member")
	requester, _ := h.Repo.Queries().GetUserByID(ctx, requesterID)
	fakeLocal := la.ID
	foreignRequest, err := h.InventoryService.CreateMaterialRequest(ctx, &requester, service.CreateMaterialRequestInput{JurisdictionType: "state", JurisdictionID: &fakeLocal, DestinationLocationID: foreign.ID, IntendedUse: "Cross-region authorization test", Items: []service.MaterialRequestItemInput{{ItemID: item.ID, RequestedQuantity: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/requests/"+foreignRequest.ID+"/review", service.ReviewMaterialRequestInput{Action: "approve"}), 403)
	localRequest, err := h.InventoryService.CreateMaterialRequest(ctx, &requester, service.CreateMaterialRequestInput{JurisdictionType: "national", DestinationLocationID: local.ID, IntendedUse: "Local material workflow", Items: []service.MaterialRequestItemInput{{ItemID: item.ID, RequestedQuantity: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/requests/"+localRequest.ID+"/review", service.ReviewMaterialRequestInput{Action: "approve"}), 200)
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/requests/"+localRequest.ID+"/allocate", map[string]string{"supplying_location_id": foreign.ID}), 403)
	assertStatus(t, operationRequest(h, token, "POST", "/inventory/requests/"+localRequest.ID+"/allocate", map[string]string{"supplying_location_id": local.ID}), 200)
	var audits int
	if err = h.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE actor_id=$1 AND resource_id=$2 AND action='allocate_material_stock'`, actor, localRequest.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("allocation audit missing", audits, err)
	}

}

func TestPhase70ReportSnapshotScope(t *testing.T) {
	h := setupTestHarness(t)
	defer h.Cleanup()
	ctx := context.Background()
	la, _, kn, _, _ := getLagosAndKanoData(t, h)
	adminID, _ := operationalUser(t, h, "super_admin")
	admin, _ := h.Repo.Queries().GetUserByID(ctx, adminID)
	actor, token := operationalUser(t, h, "member")
	insertTestAppointment(t, h, actor, "state_coordinator", "Reporting coordinator", "state", &la.ID, "active", time.Now().Add(-time.Hour), nil)
	req := service.GenerateReportRequest{ReportType: "cleanups", Title: "Phase70 own report", ReportingPeriod: "2026", PeriodStart: "2026-01-01", PeriodEnd: "2026-12-31", ScopeType: "state", StateID: &la.ID}
	own, err := h.ReportService.GenerateSnapshot(ctx, &admin, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.StateID = &kn.ID
	req.Title = "Phase70 foreign confidential report"
	other, err := h.ReportService.GenerateSnapshot(ctx, &admin, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, operationRequest(h, token, "GET", "/admin/reports/"+own.SnapshotID, nil), 200)
	assertStatus(t, operationRequest(h, token, "GET", "/admin/reports/"+other.SnapshotID, nil), 403)
	w := operationRequest(h, token, "GET", "/admin/reports/", nil)
	assertStatus(t, w, 200)
	if strings.Contains(w.Body.String(), other.SnapshotID) {
		t.Fatal("foreign snapshot metadata leaked")
	}
	assertStatus(t, operationRequest(h, token, "POST", "/admin/reports/generate", req), 403)
	assertStatus(t, operationRequest(h, token, "GET", "/admin/reports/audit", nil), 403)
}
