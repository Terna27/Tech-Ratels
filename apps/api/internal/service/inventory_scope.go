package service

import (
	"context"
	"encoding/json"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

// Internal/background reads may omit the actor. Every HTTP inventory route sets
// it from the authenticated session before calling the existing service.
func filterInventoryRows[T any](ctx context.Context, repo *repository.Repository, rows []T, locations func(T) []string, owner func(T) string) ([]T, error) {
	actor := operationalActor(ctx)
	if actor == nil {
		return rows, nil
	}
	scope, err := ResolveOperationalScope(ctx, repo, actor.ID)
	if err != nil && owner == nil {
		return nil, err
	}
	out := make([]T, 0)
	cache := map[string]bool{}
	for _, row := range rows {
		allowed := owner != nil && owner(row) == actor.ID
		if !allowed && scope != nil {
			for _, id := range locations(row) {
				v, ok := cache[id]
				if !ok {
					loc, e := repo.Queries().GetStorageLocationByID(ctx, id)
					v = e == nil && scope.CoversLocation(&loc)
					cache[id] = v
				}
				if v {
					allowed = true
					break
				}
			}
		}
		if allowed {
			out = append(out, row)
		}
	}
	return out, nil
}
func (s *InventoryService) authorizeLocation(ctx context.Context, actor *db.User, id string) error {
	scope, err := operationalScopeForActor(ctx, s.repo, actor)
	if err != nil {
		return ErrUnauthorizedInventoryAction
	}
	loc, err := s.repo.Queries().GetStorageLocationByID(ctx, id)
	if err != nil {
		return ErrLocationNotFound
	}
	if !scope.CoversLocation(&loc) {
		return ErrJurisdictionMismatch
	}
	return nil
}
func (s *InventoryService) authorizeCatalogue(ctx context.Context, actor *db.User) error {
	scope, err := operationalScopeForActor(ctx, s.repo, actor)
	if err != nil || (!scope.National && len(scope.States) == 0 && len(scope.Zones) == 0) {
		return ErrUnauthorizedInventoryAction
	}
	return nil
}
func (s *InventoryService) authorizeRequestRead(ctx context.Context, id string) error {
	actor := operationalActor(ctx)
	if actor == nil {
		return nil
	}
	r, err := s.repo.Queries().GetMaterialRequestByID(ctx, id)
	if err != nil {
		return ErrRequestNotFound
	}
	if r.RequestingUserID == actor.ID {
		return nil
	}
	return s.authorizeLocation(ctx, actor, r.DestinationLocationID)
}
func (s *InventoryService) authorizeTransferRead(ctx context.Context, id string) error {
	actor := operationalActor(ctx)
	if actor == nil {
		return nil
	}
	t, err := s.repo.Queries().GetMaterialTransferByID(ctx, id)
	if err != nil {
		return ErrTransferNotFound
	}
	if s.authorizeLocation(ctx, actor, t.SourceLocationID) == nil || s.authorizeLocation(ctx, actor, t.DestinationLocationID) == nil {
		return nil
	}
	return ErrJurisdictionMismatch
}
func (s *InventoryService) scopedLocationIDs(ctx context.Context) ([]string, error) {
	rows, err := s.ListLocations(ctx, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

func (s *InventoryService) withOperationalAudit(ctx context.Context, u *db.User, action, resource, id string, fn func(*db.Queries) error) error {
	scope, err := operationalScopeForActor(ctx, s.repo, u)
	if err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]interface{}{"authorized_scope": scope})
	return s.repo.WithTx(ctx, func(q *db.Queries) error {
		if err := fn(q); err != nil {
			return err
		}
		_, err := q.CreateAuditEvent(ctx, db.CreateAuditEventParams{ID: security.GenerateUUID(), ActorID: TextFromString(u.ID), ActorRole: TextFromString(u.PrimaryRole), Action: action, ResourceType: resource, ResourceID: id, Details: TextFromString(string(detail))})
		return err
	})
}

// Dispatchers may select another region's destination by its name/reference,
// without receiving warehouse addresses, custodians, balances or stock ledgers.
func (s *InventoryService) TransferDestinations(ctx context.Context, u *db.User) ([]map[string]string, error) {
	if _, err := operationalScopeForActor(ctx, s.repo, u); err != nil {
		return nil, err
	}
	rows, err := s.repo.Pool().Query(ctx, `SELECT id,name,code,location_type FROM storage_locations WHERE is_active ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, name, code, kind string
		if err = rows.Scan(&id, &name, &code, &kind); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"id": id, "name": name, "code": code, "location_type": kind})
	}
	return out, rows.Err()
}
