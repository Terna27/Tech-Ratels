package service

import (
	"context"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

type AuditService struct {
	repo *repository.Repository
}

func NewAuditService(repo *repository.Repository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Log(ctx context.Context, actorID *string, actorRole *string, action, resourceType, resourceID string, scopeType, scopeID *string, details *string, ipAddress *string) error {
	_, err := s.repo.Queries().CreateAuditEvent(ctx, db.CreateAuditEventParams{
		ID:           security.GenerateUUID(),
		ActorID:      Text(actorID),
		ActorRole:    Text(actorRole),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ScopeType:    Text(scopeType),
		ScopeID:      Text(scopeID),
		Details:      Text(details),
		IpAddress:    Text(ipAddress),
	})
	return err
}
