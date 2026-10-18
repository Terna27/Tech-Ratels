package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ratels-api/internal/calendar"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrEventNotPublished   = errors.New("event is not published for registration")
	ErrEventAtCapacity     = errors.New("event has reached maximum registration capacity")
	ErrAlreadyRegistered   = errors.New("user is already registered for this event")
	ErrNotRegistered       = errors.New("user is not registered for this event")
	ErrScopeUnauthorized   = errors.New("user is unauthorized for this geographic scope")
)

type CleanupService struct {
	repo  *repository.Repository
	audit *AuditService
}

func NewCleanupService(repo *repository.Repository, audit *AuditService) *CleanupService {
	return &CleanupService{repo: repo, audit: audit}
}

type CreateEventRequest struct {
	CampaignID         *string   `json:"campaign_id"`
	ProgrammeType      string    `json:"programme_type"`
	Title              string    `json:"title"`
	StateID            string    `json:"state_id"`
	LGAID              string    `json:"lga_id"`
	Address            string    `json:"address"`
	Landmark           string    `json:"landmark"`
	Latitude           *float64  `json:"latitude"`
	Longitude          *float64  `json:"longitude"`
	StartTime          time.Time `json:"start_time"`
	EndTime            time.Time `json:"end_time"`
	Capacity           int32     `json:"capacity"`
	PublicContact      string    `json:"public_contact"`
	AccessibilityNotes *string   `json:"accessibility_notes"`
	SafetyNotes        *string   `json:"safety_notes"`
	Status             string    `json:"status"` // draft, submitted, published
}

type RescheduleEventRequest struct {
	NewStartTime time.Time `json:"new_start_time"`
	NewEndTime   time.Time `json:"new_end_time"`
	Reason       string    `json:"reason"`
}

func (s *CleanupService) GetCurrentCampaign(ctx context.Context) (*db.CleanupCampaign, error) {
	now := time.Now()
	year := int32(now.Year())
	month := int32(now.Month())

	campaign, err := s.repo.Queries().GetCleanupCampaignByYearMonth(ctx, db.GetCleanupCampaignByYearMonthParams{
		Year:  year,
		Month: month,
	})
	if err == nil && campaign.ID != "" {
		return &campaign, nil
	}

	// Calculate last Saturday of Lagos
	schedDate := calendar.CalculateLastSaturdayOfLagos(int(year), int(month))
	title := fmt.Sprintf("%s %d National Clean-up Campaign", now.Month().String(), year)
	desc := "Official nationwide monthly environmental cleanup on the last Saturday of the month across all 36 States and the FCT."

	created, err := s.repo.Queries().CreateCleanupCampaign(ctx, db.CreateCleanupCampaignParams{
		ID:            security.GenerateUUID(),
		Year:          year,
		Month:         month,
		ScheduledDate: TimestamptzFromTime(schedDate),
		Title:         title,
		Description:   TextFromString(desc),
		Status:        "scheduled",
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *CleanupService) ListEvents(ctx context.Context, stateFilter *string, publishedOnly bool) (interface{}, error) {
	if publishedOnly {
		if stateFilter != nil && *stateFilter != "" {
			return s.repo.Queries().ListPublishedEventsByState(ctx, *stateFilter)
		}
		return s.repo.Queries().ListPublishedEvents(ctx)
	}

	if stateFilter != nil && *stateFilter != "" {
		return s.repo.Queries().ListEventsByState(ctx, *stateFilter)
	}
	return s.repo.Queries().ListAllEvents(ctx)
}

func (s *CleanupService) GetEvent(ctx context.Context, eventID string) (*db.EventInstance, error) {
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	return &event, nil
}

func (s *CleanupService) CreateEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, req CreateEventRequest) (*db.EventInstance, error) {
	// Enforce geographic scoping for State Coordinators
	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != req.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole == "lga_representative" {
		if actorJurisdictionID == nil || *actorJurisdictionID != req.LGAID {
			return nil, ErrScopeUnauthorized
		}
	}

	// Status gating
	status := "draft"
	if req.Status == "published" && (actorRole == "national_admin" || actorRole == "state_coordinator") {
		status = "published"
	} else if req.Status == "submitted" {
		status = "submitted"
	}

	if req.Capacity <= 0 {
		req.Capacity = 100
	}
	if req.ProgrammeType == "" {
		req.ProgrammeType = "cleanup"
	}

	eventID := security.GenerateUUID()
	created, err := s.repo.Queries().CreateEventInstance(ctx, db.CreateEventInstanceParams{
		ID:                 eventID,
		CampaignID:         Text(req.CampaignID),
		ProgrammeType:      req.ProgrammeType,
		Title:              req.Title,
		StateID:            req.StateID,
		LgaID:              req.LGAID,
		Address:            req.Address,
		Landmark:           req.Landmark,
		Latitude:           Float8(req.Latitude),
		Longitude:          Float8(req.Longitude),
		StartTime:          TimestamptzFromTime(req.StartTime),
		EndTime:            TimestamptzFromTime(req.EndTime),
		Capacity:           req.Capacity,
		CoordinatorID:      actorID,
		PublicContact:      req.PublicContact,
		AccessibilityNotes: Text(req.AccessibilityNotes),
		SafetyNotes:        Text(req.SafetyNotes),
		Status:             status,
	})
	if err != nil {
		return nil, err
	}

	action := "create_event_draft"
	if status == "published" {
		action = "publish_event"
	}
	scopeType := "state"
	_ = s.audit.Log(ctx, &actorID, &actorRole, action, "event", eventID, &scopeType, &req.StateID, &req.Title, nil)

	return &created, nil
}

func (s *CleanupService) PublishEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, eventID string) (*db.EventInstance, error) {
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}

	// Scope verification
	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != event.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole != "national_admin" {
		return nil, errors.New("unauthorized to publish activities")
	}

	updated, err := s.repo.Queries().UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:              eventID,
		Status:          "published",
		RejectionReason: Text(nil),
	})
	if err != nil {
		return nil, err
	}

	scopeType := "state"
	details := fmt.Sprintf("Published venue %s", updated.Title)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "publish_event", "event", eventID, &scopeType, &updated.StateID, &details, nil)

	return &updated, nil
}

func (s *CleanupService) RescheduleEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, eventID string, req RescheduleEventRequest) (*db.EventInstance, error) {
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}

	if actorRole == "state_coordinator" {
		if actorJurisdictionID == nil || *actorJurisdictionID != event.StateID {
			return nil, ErrScopeUnauthorized
		}
	} else if actorRole != "national_admin" {
		return nil, errors.New("unauthorized to reschedule events")
	}

	oldStart := event.StartTime
	updated, err := s.repo.Queries().RescheduleEvent(ctx, db.RescheduleEventParams{
		ID:                eventID,
		StartTime:         TimestamptzFromTime(req.NewStartTime),
		EndTime:           TimestamptzFromTime(req.NewEndTime),
		RescheduledFrom:   oldStart,
		RescheduledReason: TextFromString(req.Reason),
	})
	if err != nil {
		return nil, err
	}

	scopeType := "state"
	details := fmt.Sprintf("Rescheduled event to %s: %s", req.NewStartTime.Format(time.RFC3339), req.Reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "reschedule_event", "event", eventID, &scopeType, &updated.StateID, &details, nil)

	return &updated, nil
}

func (s *CleanupService) RegisterForEvent(ctx context.Context, userID, eventID string) (*db.EventRegistration, error) {
	var registration db.EventRegistration

	err := s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Row-level lock FOR UPDATE
		event, err := q.GetEventInstanceForUpdate(ctx, eventID)
		if err != nil {
			return ErrEventNotFound
		}

		if event.Status != "published" {
			return ErrEventNotPublished
		}

		if event.RegisteredCount >= event.Capacity {
			return ErrEventAtCapacity
		}

		// Check existing registration
		existing, err := q.GetEventRegistration(ctx, db.GetEventRegistrationParams{
			EventID: eventID,
			UserID:  userID,
		})
		if err == nil && existing.ID != "" {
			if existing.Status == "registered" {
				return ErrAlreadyRegistered
			}
			// Re-activate cancelled registration
			updated, err := q.UpdateEventRegistrationStatus(ctx, db.UpdateEventRegistrationStatusParams{
				EventID:     eventID,
				UserID:      userID,
				Status:      "registered",
				CancelledAt: Timestamptz(nil),
			})
			if err != nil {
				return err
			}
			_, _ = q.IncrementRegisteredCount(ctx, eventID)
			registration = updated
			return nil
		}

		// Create new registration
		regID := security.GenerateUUID()
		created, err := q.CreateEventRegistration(ctx, db.CreateEventRegistrationParams{
			ID:      regID,
			EventID: eventID,
			UserID:  userID,
		})
		if err != nil {
			return err
		}

		_, err = q.IncrementRegisteredCount(ctx, eventID)
		if err != nil {
			return err
		}

		registration = created
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &registration, nil
}

func (s *CleanupService) CancelRegistration(ctx context.Context, userID, eventID string) error {
	return s.repo.WithTx(ctx, func(q *db.Queries) error {
		existing, err := q.GetEventRegistration(ctx, db.GetEventRegistrationParams{
			EventID: eventID,
			UserID:  userID,
		})
		if err != nil || existing.Status != "registered" {
			return ErrNotRegistered
		}

		now := time.Now()
		_, err = q.UpdateEventRegistrationStatus(ctx, db.UpdateEventRegistrationStatusParams{
			EventID:     eventID,
			UserID:      userID,
			Status:      "cancelled",
			CancelledAt: TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		_, _ = q.DecrementRegisteredCount(ctx, eventID)
		return nil
	})
}
