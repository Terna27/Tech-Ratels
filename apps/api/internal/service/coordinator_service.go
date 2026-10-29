package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

type CoordinatorService struct{ repo *repository.Repository }

func NewCoordinatorService(repo *repository.Repository) *CoordinatorService {
	return &CoordinatorService{repo}
}

type CoordinatorMember struct {
	ID        string  `json:"id"`
	Name      string  `json:"display_name"`
	Reference string  `json:"membership_reference"`
	Status    string  `json:"status"`
	Verified  bool    `json:"is_verified"`
	StateID   *string `json:"home_state_id"`
	LGAID     *string `json:"home_lga_id"`
	State     *string `json:"state_name"`
	LGA       *string `json:"lga_name"`
	Address   *string `json:"address"`
}
type CoordinatorMembers struct {
	Members []CoordinatorMember `json:"members"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	Limit   int                 `json:"limit"`
}

const operationalMemberFilter = `($1::boolean OR p.home_state_id=ANY($2::text[]) OR p.home_lga_id=ANY($3::text[]))`
const operationalEventFilter = `($1::boolean OR e.state_id=ANY($2::text[]) OR e.lga_id=ANY($3::text[]))`

func (s *CoordinatorService) Members(ctx context.Context, u *db.User, search string, page, limit int) (*CoordinatorMembers, error) {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if page > 1000000 {
		return nil, errors.New("page is too large")
	}
	result := &CoordinatorMembers{Members: []CoordinatorMember{}, Page: page, Limit: limit}
	filter := operationalMemberFilter + ` AND u.primary_role='member' AND NOT EXISTS(SELECT 1 FROM appointments a WHERE a.user_id=u.id AND a.status='active' AND a.start_date<=now() AND (a.end_date IS NULL OR a.end_date>now())) AND ($4='' OR u.display_name ILIKE '%'||$4||'%' OR u.membership_reference ILIKE '%'||$4||'%')`
	args := []interface{}{scope.National, scope.States, scope.LGAs, strings.TrimSpace(search)}
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM users u LEFT JOIN member_profiles p ON p.user_id=u.id WHERE `+filter, args...).Scan(&result.Total)
	if err != nil {
		return nil, err
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := s.repo.Pool().Query(ctx, `SELECT u.id,u.display_name,u.membership_reference,u.status,u.is_verified,p.home_state_id,p.home_lga_id,st.name,l.name,p.address FROM users u LEFT JOIN member_profiles p ON p.user_id=u.id LEFT JOIN states st ON st.id=p.home_state_id LEFT JOIN lgas l ON l.id=p.home_lga_id WHERE `+filter+` ORDER BY u.display_name,u.id LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m CoordinatorMember
		if err = rows.Scan(&m.ID, &m.Name, &m.Reference, &m.Status, &m.Verified, &m.StateID, &m.LGAID, &m.State, &m.LGA, &m.Address); err != nil {
			return nil, err
		}
		result.Members = append(result.Members, m)
	}
	return result, rows.Err()
}

type MemberCorrection struct {
	StateID string `json:"home_state_id"`
	LGAID   string `json:"home_lga_id"`
	Address string `json:"address"`
	Reason  string `json:"reason"`
}

func (s *CoordinatorService) CorrectMember(ctx context.Context, u *db.User, id string, in MemberCorrection) error {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return err
	}
	if u.ID == id || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 || len(in.Address) > 255 {
		return ErrMemberCorrection
	}
	lga, err := s.repo.Queries().GetLGAByID(ctx, in.LGAID)
	if err != nil || lga.StateID != in.StateID || !scope.Covers(in.StateID, in.LGAID, "") {
		return ErrScopeUnauthorized
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var oldState, oldLGA, address *string
	var role string
	err = tx.QueryRow(ctx, `SELECT u.primary_role,p.home_state_id,p.home_lga_id,p.address FROM users u JOIN member_profiles p ON p.user_id=u.id WHERE u.id=$1 FOR UPDATE OF u,p`, id).Scan(&role, &oldState, &oldLGA, &address)
	if err != nil {
		return err
	}
	var appointed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM appointments WHERE user_id=$1 AND status='active' AND start_date<=now() AND (end_date IS NULL OR end_date>now()))`, id).Scan(&appointed)
	if err != nil {
		return err
	}
	state, lgaID := "", ""
	if oldState != nil {
		state = *oldState
	}
	if oldLGA != nil {
		lgaID = *oldLGA
	}
	if role != "member" || appointed || !scope.Covers(state, lgaID, "") {
		return ErrMemberCorrection
	}
	_, err = tx.Exec(ctx, `UPDATE member_profiles SET home_state_id=$2,home_lga_id=$3,address=$4,updated_at=now(),version=version+1 WHERE user_id=$1`, id, in.StateID, in.LGAID, strings.TrimSpace(in.Address))
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]interface{}{"reason": strings.TrimSpace(in.Reason), "previous_state_id": oldState, "previous_lga_id": oldLGA, "state_id": in.StateID, "lga_id": in.LGAID, "address_changed": address == nil || *address != in.Address})
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,scope_type,scope_id,details) VALUES($1,$2,$3,'correct_member_geography','member_profile',$4,'lga',$5,$6)`, security.GenerateUUID(), u.ID, u.PrimaryRole, id, in.LGAID, string(detail))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *CoordinatorService) Events(ctx context.Context, u *db.User, state string) ([]map[string]interface{}, error) {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return nil, err
	}
	if state != "" && !scope.AllowsStateFilter(state) {
		return nil, ErrScopeUnauthorized
	}
	rows, err := s.repo.Pool().Query(ctx, `SELECT to_jsonb(e)||jsonb_build_object('state_name',st.name,'lga_name',l.name) FROM event_instances e JOIN states st ON st.id=e.state_id JOIN lgas l ON l.id=e.lga_id WHERE `+operationalEventFilter+` AND ($4='' OR e.state_id=$4) ORDER BY e.start_time DESC LIMIT 500`, scope.National, scope.States, scope.LGAs, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row map[string]interface{}
		if err = json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type OperationalSummary struct {
	Members       int64 `json:"members"`
	Events        int64 `json:"events"`
	Drafts        int64 `json:"draft_events"`
	Registrations int64 `json:"registrations"`
	Attendance    int64 `json:"verified_attendance"`
	PendingClaims int64 `json:"pending_claims"`
	Requests      int64 `json:"inventory_requests"`
	Inbound       int64 `json:"inbound_dispatches"`
}
type OperationalReport struct {
	Scope     *OperationalScope        `json:"scope"`
	Summary   OperationalSummary       `json:"summary"`
	Geography []map[string]interface{} `json:"geography"`
	From      time.Time                `json:"from"`
	Until     time.Time                `json:"until"`
}

func (s *CoordinatorService) Report(ctx context.Context, u *db.User, from, until time.Time) (*OperationalReport, error) {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return nil, err
	}
	if from.IsZero() {
		from = time.Now().UTC().AddDate(0, -1, 0)
	}
	if until.IsZero() {
		until = time.Now().UTC().AddDate(0, 0, 1)
	}
	if !until.After(from) || until.Sub(from) > 366*24*time.Hour {
		return nil, errors.New("report range must be positive and no longer than 366 days")
	}
	r := &OperationalReport{Scope: scope, Geography: []map[string]interface{}{}, From: from.UTC(), Until: until.UTC()}
	args := []interface{}{scope.National, scope.States, scope.LGAs, from, until}
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM users u LEFT JOIN member_profiles p ON p.user_id=u.id WHERE `+operationalMemberFilter, scope.National, scope.States, scope.LGAs).Scan(&r.Summary.Members)
	if err != nil {
		return nil, err
	}
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE e.status='draft'),coalesce(sum((SELECT count(*) FROM event_registrations reg WHERE reg.event_id=e.id)),0),coalesce(sum((SELECT count(*) FROM event_registrations reg WHERE reg.event_id=e.id AND reg.verified_at IS NOT NULL)),0) FROM event_instances e WHERE `+operationalEventFilter+` AND e.start_time >=$4 AND e.start_time<$5`, args...).Scan(&r.Summary.Events, &r.Summary.Drafts, &r.Summary.Registrations, &r.Summary.Attendance)
	if err != nil {
		return nil, err
	}
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM participation_claims c JOIN event_instances e ON e.id=c.event_id WHERE `+operationalEventFilter+` AND c.status='pending' AND e.start_time >=$4 AND e.start_time<$5`, args...).Scan(&r.Summary.PendingClaims)
	if err != nil {
		return nil, err
	}
	locFilter := `($1::boolean OR loc.lga_id=ANY($3::text[]) OR (loc.lga_id IS NULL AND loc.state_id=ANY($2::text[])) OR (loc.lga_id IS NULL AND loc.state_id IS NULL AND loc.zone_id=ANY($4::text[])))`
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM material_requests req JOIN storage_locations loc ON loc.id=req.destination_location_id WHERE `+locFilter, scope.National, scope.States, scope.LGAs, scope.Zones).Scan(&r.Summary.Requests)
	if err != nil {
		return nil, err
	}
	err = s.repo.Pool().QueryRow(ctx, `SELECT count(*) FROM material_transfers t JOIN storage_locations loc ON loc.id=t.destination_location_id WHERE t.status='dispatched' AND `+locFilter, scope.National, scope.States, scope.LGAs, scope.Zones).Scan(&r.Summary.Inbound)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.Pool().Query(ctx, `SELECT st.id,st.name,l.id,l.name,(SELECT count(*) FROM users u JOIN member_profiles p ON p.user_id=u.id WHERE p.home_lga_id=l.id),(SELECT count(*) FROM event_instances e WHERE e.lga_id=l.id AND e.start_time>=$4 AND e.start_time<$5),(SELECT count(*) FROM event_registrations reg JOIN event_instances e ON e.id=reg.event_id WHERE e.lga_id=l.id AND reg.verified_at IS NOT NULL AND e.start_time>=$4 AND e.start_time<$5) FROM lgas l JOIN states st ON st.id=l.state_id WHERE ($1::boolean OR st.id=ANY($2::text[]) OR l.id=ANY($3::text[])) ORDER BY st.name,l.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var state, stateName, lga, lgaName string
		var members, events, attendance int64
		if err = rows.Scan(&state, &stateName, &lga, &lgaName, &members, &events, &attendance); err != nil {
			return nil, err
		}
		r.Geography = append(r.Geography, map[string]interface{}{"state_id": state, "state_name": stateName, "lga_id": lga, "lga_name": lgaName, "members": members, "events": events, "verified_attendance": attendance})
	}
	return r, rows.Err()
}

func (s *CoordinatorService) AuditReportExport(ctx context.Context, u *db.User, from, until time.Time) error {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return err
	}
	kind := "regional"
	if scope.National {
		kind = "national"
	}
	detail, _ := json.Marshal(map[string]string{"from": from.Format(time.RFC3339), "until": until.Format(time.RFC3339)})
	_, err = s.repo.Pool().Exec(ctx, `INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,scope_type,details) VALUES($1,$2,$3,'export_coordinator_report','coordinator_report',$1,$4,$5)`, security.GenerateUUID(), u.ID, u.PrimaryRole, kind, string(detail))
	return err
}
