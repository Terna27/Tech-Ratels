package service

import (
	"context"
	"errors"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"slices"
)

// OperationalScope is always resolved from current database appointments. Role
// labels, client filters and cached JWT claims never grant regional authority.
type OperationalScope struct {
	National   bool     `json:"national"`
	Zones      []string `json:"zone_ids"`
	States     []string `json:"state_ids"`
	LGAs       []string `json:"lga_ids"`
	lgaStates  map[string]string
	stateZones map[string]string
}

func ResolveOperationalScope(ctx context.Context, repo *repository.Repository, userID string) (*OperationalScope, error) {
	u, err := repo.Queries().GetUserByID(ctx, userID)
	if err != nil || u.Status != "active" {
		return nil, ErrCoordinatorUnauthorized
	}
	scope := &OperationalScope{Zones: []string{}, States: []string{}, LGAs: []string{}, lgaStates: map[string]string{}, stateZones: map[string]string{}}
	if u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin" {
		scope.National = true
		return scope, nil
	}
	apps, err := repo.Queries().ListActiveApprovedAppointmentsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	requestedStates := []string{}
	requestedLGAs := []string{}
	for _, a := range apps {
		id := a.JurisdictionID.String
		switch a.RoleType {
		case "super_admin", "national_admin":
			if a.JurisdictionType == "national" {
				scope.National = true
				return scope, nil
			}
		case "zonal_coordinator":
			if a.JurisdictionType == "zone" {
				var zone string
				if repo.Pool().QueryRow(ctx, `SELECT id FROM zones WHERE id=$1 OR code=$1`, id).Scan(&zone) == nil {
					scope.Zones = append(scope.Zones, zone)
				}
			}
		case "state_coordinator":
			if a.JurisdictionType == "state" {
				var state string
				if repo.Pool().QueryRow(ctx, `SELECT id FROM states WHERE id=$1 OR code=$1`, id).Scan(&state) == nil {
					requestedStates = append(requestedStates, state)
				}
			}
		case "lga_coordinator", "lga_representative":
			if a.JurisdictionType == "lga" {
				requestedLGAs = append(requestedLGAs, id)
			}
		}
	}
	rows, err := repo.Pool().Query(ctx, `SELECT id,zone_id FROM states WHERE id=ANY($1::text[]) OR zone_id=ANY($2::text[]) ORDER BY id`, requestedStates, scope.Zones)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var state, zone string
		if err = rows.Scan(&state, &zone); err != nil {
			rows.Close()
			return nil, err
		}
		scope.States = append(scope.States, state)
		scope.stateZones[state] = zone
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = repo.Pool().Query(ctx, `SELECT id,state_id FROM lgas WHERE id=ANY($1::text[]) OR state_id=ANY($2::text[]) ORDER BY id`, requestedLGAs, scope.States)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var lga, state string
		if err = rows.Scan(&lga, &state); err != nil {
			return nil, err
		}
		scope.LGAs = append(scope.LGAs, lga)
		scope.lgaStates[lga] = state
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(scope.Zones)+len(scope.States)+len(scope.LGAs) == 0 {
		return nil, ErrCoordinatorUnauthorized
	}
	return scope, nil
}
func (s *OperationalScope) Covers(state, lga, zone string) bool {
	if s == nil {
		return false
	}
	if s.National {
		return true
	}
	if lga != "" {
		return slices.Contains(s.LGAs, lga) && (state == "" || s.lgaStates[lga] == state)
	}
	if state != "" {
		return slices.Contains(s.States, state) && (zone == "" || s.stateZones[state] == zone)
	}
	return zone != "" && slices.Contains(s.Zones, zone)
}
func (s *OperationalScope) CoversLocation(loc *db.GetStorageLocationByIDRow) bool {
	if loc == nil {
		return false
	}
	return s.Covers(loc.StateID.String, loc.LgaID.String, loc.ZoneID.String)
}
func (s *OperationalScope) AllowsStateFilter(state string) bool {
	if s.National || slices.Contains(s.States, state) {
		return true
	}
	for _, st := range s.lgaStates {
		if st == state {
			return true
		}
	}
	return false
}

type operationalActorKey struct{}

func WithOperationalActor(ctx context.Context, u *db.User) context.Context {
	return context.WithValue(ctx, operationalActorKey{}, u)
}
func operationalActor(ctx context.Context) *db.User {
	u, _ := ctx.Value(operationalActorKey{}).(*db.User)
	return u
}
func operationalScopeForActor(ctx context.Context, r *repository.Repository, u *db.User) (*OperationalScope, error) {
	if u == nil {
		return nil, ErrCoordinatorUnauthorized
	}
	return ResolveOperationalScope(ctx, r, u.ID)
}

var ErrMemberCorrection = errors.New("only ordinary members in your jurisdiction can be corrected; a reason and valid state/LGA are required")

func hasCurrentSuperAdminAuthority(ctx context.Context, repo *repository.Repository, id string) bool {
	u, err := repo.Queries().GetUserByID(ctx, id)
	if err != nil || u.Status != "active" {
		return false
	}
	if u.PrimaryRole == "super_admin" {
		return true
	}
	apps, err := repo.Queries().ListActiveApprovedAppointmentsByUser(ctx, id)
	if err != nil {
		return false
	}
	for _, app := range apps {
		if app.RoleType == "super_admin" && app.JurisdictionType == "national" {
			return true
		}
	}
	return false
}
