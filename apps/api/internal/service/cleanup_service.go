package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"

	"ratels-api/internal/calendar"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrEventNotFound     = errors.New("event not found")
	ErrEventNotPublished = errors.New("event is not published for registration")
	ErrEventAtCapacity   = errors.New("event has reached maximum registration capacity")
	ErrAlreadyRegistered = errors.New("user is already registered for this event")
	ErrNotRegistered     = errors.New("user is not registered for this event")
	ErrScopeUnauthorized = errors.New("SCOPE_UNAUTHORIZED: coordinator appointment does not cover this geographic jurisdiction")
)

type CleanupService struct {
	repo  *repository.Repository
	audit *AuditService
	notif *NotificationService
}

func NewCleanupService(repo *repository.Repository, audit *AuditService) *CleanupService {
	return &CleanupService{repo: repo, audit: audit}
}

func (s *CleanupService) SetNotificationService(notif *NotificationService) {
	s.notif = notif
}

type CreateEventRequest struct {
	CampaignID         *string   `json:"campaign_id"`
	ProgrammeType      string    `json:"programme_type"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	ZoneID             *string   `json:"zone_id"`
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

type CancelEventRequest struct {
	Reason string `json:"reason"`
}

type VerifyAttendanceRequest struct {
	CheckinCode string  `json:"checkin_code"`
	UserID      *string `json:"user_id"`
	IsManual    bool    `json:"is_manual"`
}

// GenerateCheckinCode creates a cryptographically secure 64-bit attendance code.
func GenerateCheckinCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("secure check-in code generation failed: %w", err)
	}
	return fmt.Sprintf("RT-%X", b), nil
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

func (s *CleanupService) ListEventsByCoordinator(ctx context.Context, coordinatorID string) ([]db.ListEventsByCoordinatorRow, error) {
	return s.repo.Queries().ListEventsByCoordinator(ctx, coordinatorID)
}

func (s *CleanupService) GetEvent(ctx context.Context, eventID string) (*db.EventInstance, error) {
	event, err := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
	if err != nil {
		return nil, ErrEventNotFound
	}
	return &event, nil
}

func (s *CleanupService) CreateEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, req CreateEventRequest) (*db.EventInstance, error) {
	// Centralized database-backed authorization check: verifies active approved appointment
	app, err := AuthorizeCoordinatorForJurisdiction(ctx, s.repo, actorID, req.StateID, req.LGAID)
	if err != nil {
		// Fallback check if actorRole is super_admin or national_admin in user record
		u, uErr := s.repo.Queries().GetUserByID(ctx, actorID)
		if uErr == nil && (u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin") {
			app = &db.Appointment{
				ID:               "root_admin",
				UserID:           actorID,
				RoleTitle:        "National Administrator",
				RoleType:         u.PrimaryRole,
				JurisdictionType: "national",
				Status:           "active",
			}
		} else {
			return nil, err
		}
	}

	// Validate date & times
	if req.StartTime.IsZero() || req.EndTime.IsZero() {
		return nil, errors.New("start_time and end_time are required")
	}
	if req.EndTime.Before(req.StartTime) || req.EndTime.Equal(req.StartTime) {
		return nil, errors.New("end_time must be strictly after start_time")
	}

	// Resolve zone from state if not provided
	if req.ZoneID == nil || *req.ZoneID == "" {
		st, err := s.repo.Queries().GetStateByID(ctx, req.StateID)
		if err == nil && st.ZoneID != "" {
			req.ZoneID = &st.ZoneID
		}
	}

	// Approved coordinators can publish directly without Super Admin approval queue
	status := "published"
	if req.Status == "draft" {
		status = "draft"
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
		Description:        TextFromString(req.Description),
		ZoneID:             Text(req.ZoneID),
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
	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, action, "event", eventID, &scopeType, &req.StateID, &req.Title, nil)

	if status == "published" && s.notif != nil {
		_ = s.notif.ScheduleCleanupReminders(ctx, s.repo.Queries(), eventID, created.Version, created.StartTime.Time)
	}

	return &created, nil
}

func (s *CleanupService) PublishEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, eventID string) (*db.EventInstance, error) {
	event, app, err := AuthorizeCoordinatorForEvent(ctx, s.repo, actorID, eventID)
	_ = event
	if err != nil {
		// Fallback check for super_admin or national_admin user role
		u, uErr := s.repo.Queries().GetUserByID(ctx, actorID)
		if uErr == nil && (u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin") {
			ev, eErr := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
			if eErr != nil {
				return nil, ErrEventNotFound
			}
			event = &ev
			app = &db.Appointment{RoleType: u.PrimaryRole, JurisdictionType: "national"}
		} else {
			return nil, err
		}
	}

	updated, err := s.repo.Queries().UpdateEventStatus(ctx, db.UpdateEventStatusParams{
		ID:              eventID,
		Status:          "published",
		RejectionReason: Text(nil),
	})
	if err != nil {
		return nil, err
	}

	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	details := fmt.Sprintf("Published cleanup venue %s", updated.Title)
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, "publish_event", "event", eventID, &scopeType, &updated.StateID, &details, nil)

	if s.notif != nil {
		_ = s.notif.ScheduleCleanupReminders(ctx, s.repo.Queries(), eventID, updated.Version, updated.StartTime.Time)
	}

	return &updated, nil
}

func (s *CleanupService) RescheduleEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, eventID string, req RescheduleEventRequest) (*db.EventInstance, error) {
	event, app, err := AuthorizeCoordinatorForEvent(ctx, s.repo, actorID, eventID)
	_ = event
	if err != nil {
		u, uErr := s.repo.Queries().GetUserByID(ctx, actorID)
		if uErr == nil && (u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin") {
			ev, eErr := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
			if eErr != nil {
				return nil, ErrEventNotFound
			}
			event = &ev
			app = &db.Appointment{RoleType: u.PrimaryRole, JurisdictionType: "national"}
		} else {
			return nil, err
		}
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

	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	details := fmt.Sprintf("Rescheduled event to %s: %s", req.NewStartTime.Format(time.RFC3339), req.Reason)
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, "reschedule_event", "event", eventID, &scopeType, &updated.StateID, &details, nil)

	if s.notif != nil {
		_ = s.notif.ScheduleCleanupReminders(ctx, s.repo.Queries(), eventID, updated.Version, req.NewStartTime)
		s.notif.OnEventRescheduled(ctx, s.repo.Queries(), eventID, req.NewStartTime.Format("Monday, 02 Jan 2006 03:04 PM"), req.Reason)
	}

	return &updated, nil
}

func (s *CleanupService) CancelEvent(ctx context.Context, actorID, actorRole string, actorJurisdictionID *string, eventID string, req CancelEventRequest) (*db.EventInstance, error) {
	event, app, err := AuthorizeCoordinatorForEvent(ctx, s.repo, actorID, eventID)
	_ = event
	if err != nil {
		u, uErr := s.repo.Queries().GetUserByID(ctx, actorID)
		if uErr == nil && (u.PrimaryRole == "super_admin" || u.PrimaryRole == "national_admin") {
			ev, eErr := s.repo.Queries().GetEventInstanceByID(ctx, eventID)
			if eErr != nil {
				return nil, ErrEventNotFound
			}
			event = &ev
			app = &db.Appointment{RoleType: u.PrimaryRole, JurisdictionType: "national"}
		} else {
			return nil, err
		}
	}

	if event.Status == "cancelled" {
		return nil, ErrEventCancelled
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Event cancelled by coordinator"
	}

	updated, err := s.repo.Queries().CancelEventInstance(ctx, db.CancelEventInstanceParams{
		ID:              eventID,
		RejectionReason: TextFromString(reason),
	})
	if err != nil {
		return nil, err
	}

	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	details := fmt.Sprintf("Cancelled cleanup event: %s", reason)
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, "cancel_event", "event", eventID, &scopeType, &updated.StateID, &details, nil)

	if s.notif != nil {
		s.notif.OnEventCancelled(ctx, s.repo.Queries(), eventID, updated.Title, reason)
	}

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
			if existing.Status == "registered" || existing.Status == "attended" {
				return ErrAlreadyRegistered
			}
			// Re-activate cancelled registration with a persisted fresh code
			checkinCode, err := GenerateCheckinCode()
			if err != nil {
				return err
			}

			updated, err := q.ReactivateEventRegistration(
				ctx,
				db.ReactivateEventRegistrationParams{
					EventID:     eventID,
					UserID:      userID,
					CheckinCode: TextFromString(checkinCode),
				},
			)
			if err != nil {
				return err
			}
			if _, err := q.IncrementRegisteredCount(ctx, eventID); err != nil {
				return err
			}

			registration = updated
			return nil
		}

		// Generate unpredictable, high-entropy attendance credential
		checkinCode, err := GenerateCheckinCode()
		if err != nil {
			return err
		}
		regID := security.GenerateUUID()
		created, err := q.CreateEventRegistration(ctx, db.CreateEventRegistrationParams{
			ID:          regID,
			EventID:     eventID,
			UserID:      userID,
			CheckinCode: TextFromString(checkinCode),
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

func (s *CleanupService) GetMyRegistration(ctx context.Context, userID, eventID string) (*db.EventRegistration, error) {
	reg, err := s.repo.Queries().GetEventRegistration(ctx, db.GetEventRegistrationParams{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		return nil, ErrNotRegistered
	}
	return &reg, nil
}

func (s *CleanupService) ListMyRegistrations(ctx context.Context, userID string) ([]db.ListRegistrationsByUserRow, error) {
	return s.repo.Queries().ListRegistrationsByUser(ctx, userID)
}

func (s *CleanupService) VerifyAttendance(ctx context.Context, actorID string, eventID string, req VerifyAttendanceRequest) (*db.EventRegistration, *db.ParticipationClaim, error) {
	// Database-backed coordinator authorization for this specific event
	event, app, err := AuthorizeCoordinatorForEvent(ctx, s.repo, actorID, eventID)
	if err != nil {
		return nil, nil, err
	}

	cleanCode := strings.ToUpper(strings.TrimSpace(req.CheckinCode))
	if cleanCode == "" {
		return nil, nil, errors.New("check-in code is required")
	}

	// Fetch registration by code for this event
	var reg db.EventRegistration
	if req.UserID != nil && *req.UserID != "" {
		r, err := s.repo.Queries().GetEventRegistration(ctx, db.GetEventRegistrationParams{
			EventID: eventID,
			UserID:  *req.UserID,
		})
		if err != nil || !r.CheckinCode.Valid || strings.ToUpper(r.CheckinCode.String) != cleanCode {
			return nil, nil, ErrInvalidCheckinCode
		}
		reg = r
	} else {
		r, err := s.repo.Queries().GetEventRegistrationByCode(ctx, db.GetEventRegistrationByCodeParams{
			EventID:     eventID,
			CheckinCode: TextFromString(cleanCode),
		})
		if err != nil || r.ID == "" {
			return nil, nil, ErrInvalidCheckinCode
		}
		reg = r
	}

	// ANTI-SELF-VERIFICATION: Coordinator cannot verify their own attendance
	if reg.UserID == actorID {
		return nil, nil, ErrSelfVerificationNotAllowed
	}

	// Verify check-in window (4 hours before StartTime to 24 hours after EndTime)
	now := time.Now()
	if !IsWithinCheckinWindow(now, event.StartTime.Time, event.EndTime.Time) {
		return nil, nil, ErrCheckinWindowClosed
	}

	// Replay & Duplicate attendance prevention
	if reg.VerifiedAt.Valid || reg.Status == "attended" {
		return nil, nil, ErrAlreadyVerified
	}

	method := "qr_scan"
	if req.IsManual {
		method = "manual_code"
	}

	var updatedReg db.EventRegistration
	var claim db.ParticipationClaim

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// Update registration status to attended and record verifier
		ur, err := q.VerifyEventAttendanceByCode(ctx, db.VerifyEventAttendanceByCodeParams{
			EventID:            eventID,
			UserID:             reg.UserID,
			VerifiedByUserID:   TextFromString(actorID),
			VerificationMethod: TextFromString(method),
			CheckinCode:        TextFromString(cleanCode),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAlreadyVerified
			}
			return err
		}
		updatedReg = ur

		// Upsert verified participation claim
		claimID := security.GenerateUUID()
		c, err := q.UpsertVerifiedParticipationClaim(ctx, db.UpsertVerifiedParticipationClaimParams{
			ID:                 claimID,
			EventID:            eventID,
			UserID:             reg.UserID,
			Description:        "On-site verified attendance by authorized coordinator",
			EvidenceNotes:      TextFromString(fmt.Sprintf("Physical presence confirmed via %s on %s", method, now.Format(time.RFC3339))),
			ReviewedByUserID:   TextFromString(actorID),
			ReviewReason:       TextFromString("Physical presence verified at venue"),
			VerificationMethod: TextFromString(method),
		})
		if err != nil {
			return err
		}
		claim = c

		// Award badge if first verified participation
		verifiedCount, err := q.CountVerifiedParticipationsByUser(ctx, reg.UserID)
		if err == nil && verifiedCount == 1 {
			badge, err := q.GetBadgeByCode(ctx, "first-cleanup-ratel")
			if err == nil && badge.ID != "" {
				existingAward, _ := q.GetBadgeAward(ctx, db.GetBadgeAwardParams{
					UserID:  reg.UserID,
					BadgeID: badge.ID,
				})
				if existingAward.ID == "" {
					_, _ = q.CreateBadgeAward(ctx, db.CreateBadgeAwardParams{
						ID:                        security.GenerateUUID(),
						UserID:                    reg.UserID,
						BadgeID:                   badge.ID,
						QualifyingParticipationID: TextFromString(c.ID),
						IsPublic:                  true,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	effectiveRole := app.RoleType
	scopeType := app.JurisdictionType
	// Never log attendance credential code in audit details or logs
	auditDetails := fmt.Sprintf("Verified attendance for participant %s (method: %s)", reg.UserID, method)
	_ = s.audit.Log(ctx, &actorID, &effectiveRole, "verify_attendance", "event_registration", reg.ID, &scopeType, &event.StateID, &auditDetails, nil)

	if s.notif != nil {
		s.notif.OnAttendanceVerified(ctx, s.repo.Queries(), reg.UserID, event.Title, method)
	}

	return &updatedReg, &claim, nil
}

func (s *CleanupService) ListParticipants(ctx context.Context, actorID string, eventID string) ([]db.ListEventParticipantsRow, error) {
	_, _, err := AuthorizeCoordinatorForEvent(ctx, s.repo, actorID, eventID)
	if err != nil {
		u, uErr := s.repo.Queries().GetUserByID(ctx, actorID)
		if uErr != nil || (u.PrimaryRole != "super_admin" && u.PrimaryRole != "national_admin") {
			return nil, err
		}
	}

	return s.repo.Queries().ListEventParticipants(ctx, eventID)
}
