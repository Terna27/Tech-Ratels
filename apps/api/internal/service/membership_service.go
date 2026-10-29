package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
)

var ErrInvalidProfile = errors.New("invalid profile: check name, field lengths, HTTPS photo URL and state/LGA")
var ErrMembershipForbidden = errors.New("only a current Super Admin may change another ordinary member's status")
var ErrInvalidMembershipStatus = errors.New("choose a valid membership status, supply a reason up to 1000 characters, and verify the contact before activation")
var ErrMembershipNotFound = errors.New("membership verification link not found")

type ProfileUpdate struct {
	DisplayName  *string `json:"display_name"`
	HomeStateID  *string `json:"home_state_id"`
	HomeLGAID    *string `json:"home_lga_id"`
	Address      *string `json:"address"`
	Bio          *string `json:"bio"`
	AvatarURL    *string `json:"avatar_url"`
	PublicBadges *bool   `json:"is_public_badges"`
}

func validProfileText(s string, max int) bool {
	if len(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
func (s *AuthService) EditMemberProfile(ctx context.Context, id string, in ProfileUpdate) (*db.MemberProfile, error) {
	for _, field := range []struct {
		p   *string
		max int
	}{{in.DisplayName, 100}, {in.Address, 255}, {in.Bio, 2000}, {in.AvatarURL, 255}, {in.HomeStateID, 36}, {in.HomeLGAID, 36}} {
		if field.p != nil && !validProfileText(*field.p, field.max) {
			return nil, ErrInvalidProfile
		}
	}
	if in.DisplayName != nil && strings.TrimSpace(*in.DisplayName) == "" {
		return nil, ErrInvalidProfile
	}
	if in.AvatarURL != nil && *in.AvatarURL != "" {
		u, e := url.Parse(*in.AvatarURL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return nil, ErrInvalidProfile
		}
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err = tx.QueryRow(ctx, `SELECT status FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, ErrAccountInactive
	}
	_, err = tx.Exec(ctx, `INSERT INTO member_profiles(id,user_id) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, security.GenerateUUID(), id)
	if err != nil {
		return nil, err
	}
	q := db.New(tx)
	p, err := q.GetMemberProfileByUserID(ctx, id)
	if err != nil {
		return nil, err
	}
	state, lga := p.HomeStateID.String, p.HomeLgaID.String
	if in.HomeStateID != nil {
		state = strings.TrimSpace(*in.HomeStateID)
		if state != p.HomeStateID.String && in.HomeLGAID == nil {
			lga = ""
		}
	}
	if in.HomeLGAID != nil {
		lga = strings.TrimSpace(*in.HomeLGAID)
	}
	var valid bool
	if state != "" {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM states WHERE id=$1)`, state).Scan(&valid); err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrInvalidProfile
		}
	}
	if lga != "" {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM lgas WHERE id=$1 AND state_id=$2)`, lga, state).Scan(&valid); err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrInvalidProfile
		}
	}
	// COALESCE retains omitted fields; explicit empty strings clear optional values.
	_, err = tx.Exec(ctx, `UPDATE member_profiles SET home_state_id=NULLIF($2,''),home_lga_id=NULLIF($3,''),address=COALESCE($4,address),bio=COALESCE($5,bio),avatar_url=COALESCE($6,avatar_url),is_public_badges=COALESCE($7,is_public_badges),updated_at=now(),version=version+1 WHERE user_id=$1`, id, state, lga, in.Address, in.Bio, in.AvatarURL, in.PublicBadges)
	if err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		_, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,updated_at=now(),version=version+1 WHERE id=$1`, id, strings.TrimSpace(*in.DisplayName))
		if err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,details) SELECT $1,id,primary_role,'edit_own_member_profile','member_profile',id,'{"source":"self_service"}' FROM users WHERE id=$2`, security.GenerateUUID(), id)
	if err != nil {
		return nil, err
	}
	p, err = q.GetMemberProfileByUserID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}

type MembershipCard struct {
	Name             string `json:"display_name"`
	Reference        string `json:"membership_reference"`
	Status           string `json:"status"`
	Photo            string `json:"photo_url,omitempty"`
	VerificationPath string `json:"verification_path"`
}

func membershipStatus(status string, verified bool) string {
	switch status {
	case "closed":
		return "inactive"
	case "suspended", "revoked", "inactive":
		return status
	case "active":
		if verified {
			return "active"
		}
	}
	return "pending_verification"
}
func (s *AuthService) MemberCard(ctx context.Context, id string) (*MembershipCard, error) {
	u, err := s.repo.Queries().GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, ErrAccountInactive
	}
	candidate, err := security.GenerateSecureToken()
	if err != nil {
		return nil, err
	}
	var token string
	err = s.repo.Pool().QueryRow(ctx, `INSERT INTO member_card_links(user_id,token) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id RETURNING token`, id, candidate).Scan(&token)
	if err != nil {
		return nil, err
	}
	p, err := s.repo.Queries().GetMemberProfileByUserID(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return &MembershipCard{Name: u.DisplayName, Reference: u.MembershipReference, Status: membershipStatus(u.Status, u.IsVerified), Photo: p.AvatarUrl.String, VerificationPath: "/verify-membership/" + token}, nil
}

// Deliberate allowlist: never serialize users, profiles, internal IDs or badges publicly.
type PublicMembership struct {
	Reference string `json:"membership_reference"`
	Status    string `json:"status"`
	Name      string `json:"display_name,omitempty"`
}

func (s *AuthService) VerifyMembership(ctx context.Context, token string) (*PublicMembership, error) {
	if len(token) != 64 || strings.Trim(token, "0123456789abcdef") != "" {
		return nil, ErrMembershipNotFound
	}
	var ref, status, name string
	var verified bool
	err := s.repo.Pool().QueryRow(ctx, `SELECT u.membership_reference,u.status,u.display_name,u.is_verified FROM member_card_links c JOIN users u ON u.id=c.user_id WHERE c.token=$1`, token).Scan(&ref, &status, &name, &verified)
	if err != nil {
		return nil, ErrMembershipNotFound
	}
	status = membershipStatus(status, verified)
	out := &PublicMembership{Reference: ref, Status: status}
	if status == "active" {
		out.Name = name
	}
	return out, nil
}
func (s *AuthService) ChangeMembershipStatus(ctx context.Context, actorID, id, status, reason string) error {
	if actorID == id || !hasCurrentSuperAdminAuthority(ctx, s.repo, actorID) {
		return ErrMembershipForbidden
	}
	switch status {
	case "active", "inactive", "suspended", "revoked":
	default:
		return ErrInvalidMembershipStatus
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || !validProfileText(reason, 1000) {
		return ErrInvalidMembershipStatus
	}
	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Lock actor and target in deterministic order; status changes cannot manage officers.
	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE id=ANY($1::text[]) ORDER BY id FOR UPDATE`, []string{actorID, id})
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active' AND (primary_role='super_admin' OR EXISTS(SELECT 1 FROM appointments WHERE user_id=$1 AND role_type='super_admin' AND jurisdiction_type='national' AND status='active' AND start_date<=now() AND (end_date IS NULL OR end_date>now()))))`, actorID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrMembershipForbidden
	}
	var old, role string
	var verified bool
	err = tx.QueryRow(ctx, `SELECT status,primary_role,is_verified FROM users WHERE id=$1`, id).Scan(&old, &role, &verified)
	if err != nil {
		return err
	}
	var appointed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM appointments WHERE user_id=$1 AND status='active' AND start_date<=now() AND (end_date IS NULL OR end_date>now()))`, id).Scan(&appointed)
	if err != nil {
		return err
	}
	if role != "member" || appointed {
		return ErrMembershipForbidden
	}
	if status == "active" && !verified {
		return ErrInvalidMembershipStatus
	}
	if old == status {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE users SET status=$2,updated_at=now(),version=version+1 WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	// Suspension/revocation invalidates existing sessions permanently, including after reactivation.
	if status != "active" {
		_, err = tx.Exec(ctx, `UPDATE user_sessions SET is_revoked=true WHERE user_id=$1`, id)
		if err != nil {
			return err
		}
	}
	detail, _ := json.Marshal(map[string]string{"previous_status": old, "status": status, "reason": reason})
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,actor_id,actor_role,action,resource_type,resource_id,details) VALUES($1,$2,'super_admin','change_membership_status','user',$3,$4)`, security.GenerateUUID(), actorID, id, string(detail))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *AuthService) CardVerificationURL(path string) string {
	return strings.TrimRight(s.cfg.FrontendBaseURL, "/") + path
}
