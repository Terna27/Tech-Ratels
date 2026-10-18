package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrSuperAdminRequired         = errors.New("SUPER_ADMIN_REQUIRED: Only a Super Admin may appoint, activate, replace, suspend or revoke zonal and state coordinators, or alter their geographic scope and dates")
	ErrSelfModificationProhibited = errors.New("SELF_MODIFICATION_PROHIBITED: Coordinators and administrators cannot modify, promote, or elevate their own appointments")
)

type LeadershipService struct {
	repo  *repository.Repository
	audit *AuditService
}

func NewLeadershipService(repo *repository.Repository, audit *AuditService) *LeadershipService {
	return &LeadershipService{repo: repo, audit: audit}
}

type CreateAppointmentRequest struct {
	UserID                   string     `json:"user_id"`
	RoleTitle                string     `json:"role_title"`
	RoleType                 string     `json:"role_type"` // super_admin, national_admin, zonal_coordinator, state_coordinator, lga_representative
	JurisdictionType         string     `json:"jurisdiction_type"` // national, zone, state, lga
	JurisdictionID           *string    `json:"jurisdiction_id"`
	ReportingToAppointmentID *string    `json:"reporting_to_appointment_id"`
	StartDate                time.Time  `json:"start_date"`
	EndDate                  *time.Time `json:"end_date"`
	ApprovalReference        string     `json:"approval_reference"`
	PublicContact            *string    `json:"public_contact"`
}

type ReplaceAppointmentRequest struct {
	NewUserID         string    `json:"new_user_id"`
	EffectiveDate     time.Time `json:"effective_date"`
	Reason            string    `json:"reason"`
	ApprovalReference string    `json:"approval_reference"`
	PublicContact     *string   `json:"public_contact"`
}

type UpdateScopeDatesRequest struct {
	JurisdictionType string     `json:"jurisdiction_type"`
	JurisdictionID   *string    `json:"jurisdiction_id"`
	StartDate        time.Time  `json:"start_date"`
	EndDate          *time.Time `json:"end_date"`
	Reason           string     `json:"reason"`
}

func (s *LeadershipService) ListPublicDirectory(ctx context.Context) ([]db.ListPublicLeadershipDirectoryRow, error) {
	return s.repo.Queries().ListPublicLeadershipDirectory(ctx)
}

func (s *LeadershipService) ListAppointments(ctx context.Context) ([]db.ListAppointmentsRow, error) {
	return s.repo.Queries().ListAppointments(ctx)
}

func (s *LeadershipService) CreateAppointment(ctx context.Context, actorID string, actorRole string, req CreateAppointmentRequest) (*db.Appointment, error) {
	// Prevent self-promotion or self-appointment
	if req.UserID == actorID {
		return nil, ErrSelfModificationProhibited
	}

	// Super Admin authority is strictly required for zonal coordinators, state coordinators, national admins, and super admins
	if req.RoleType == "zonal_coordinator" || req.RoleType == "state_coordinator" || req.RoleType == "super_admin" || req.RoleType == "national_admin" {
		if actorRole != "super_admin" {
			return nil, ErrSuperAdminRequired
		}
	}

	// Verify target user exists
	targetUser, err := s.repo.Queries().GetUserByID(ctx, req.UserID)
	if err != nil || targetUser.ID == "" {
		return nil, errors.New("target user not found")
	}

	// Validate role_type
	validRoles := map[string]bool{
		"super_admin":        true,
		"national_admin":     true,
		"zonal_coordinator":  true,
		"state_coordinator":  true,
		"lga_representative": true,
	}
	if !validRoles[req.RoleType] {
		return nil, fmt.Errorf("invalid role_type: %s", req.RoleType)
	}

	// Validate jurisdiction
	if req.JurisdictionType == "national" && req.JurisdictionID != nil {
		return nil, errors.New("national jurisdiction cannot have a jurisdiction_id")
	}
	if req.JurisdictionType != "national" && (req.JurisdictionID == nil || *req.JurisdictionID == "") {
		return nil, fmt.Errorf("jurisdiction_id required for jurisdiction_type %s", req.JurisdictionType)
	}

	appID := security.GenerateUUID()
	var appointment db.Appointment

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		app, err := q.CreateAppointment(ctx, db.CreateAppointmentParams{
			ID:                       appID,
			UserID:                   req.UserID,
			RoleTitle:                req.RoleTitle,
			RoleType:                 req.RoleType,
			JurisdictionType:         req.JurisdictionType,
			JurisdictionID:           Text(req.JurisdictionID),
			ReportingToAppointmentID: Text(req.ReportingToAppointmentID),
			StartDate:                TimestamptzFromTime(req.StartDate),
			EndDate:                  Timestamptz(req.EndDate),
			Status:                   "active",
			ApprovalReference:        req.ApprovalReference,
			PublicContact:            Text(req.PublicContact),
		})
		if err != nil {
			return err
		}
		appointment = app

		// Record appointment history with appointee tracking
		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   appID,
			Action:          "created",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  Text(nil),
			NewStatus:       "active",
			Reason:          TextFromString(req.ApprovalReference),
			PreviousUserID:  Text(nil),
			NewUserID:       TextFromString(req.UserID),
			EffectiveDate:   TimestamptzFromTime(req.StartDate),
		})
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Appointed to %s (%s)", req.RoleTitle, req.RoleType)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "create_appointment", "appointment", appID, &req.JurisdictionType, req.JurisdictionID, &details, nil)

	return &appointment, nil
}

func (s *LeadershipService) ReplaceAppointment(ctx context.Context, actorID, actorRole, oldAppointmentID string, req ReplaceAppointmentRequest) (*db.Appointment, error) {
	if actorRole != "super_admin" {
		return nil, ErrSuperAdminRequired
	}
	if req.NewUserID == actorID {
		return nil, ErrSelfModificationProhibited
	}

	oldApp, err := s.repo.Queries().GetAppointmentByID(ctx, oldAppointmentID)
	if err != nil {
		return nil, errors.New("appointment to replace not found")
	}

	targetUser, err := s.repo.Queries().GetUserByID(ctx, req.NewUserID)
	if err != nil || targetUser.ID == "" {
		return nil, errors.New("new user not found")
	}

	if req.EffectiveDate.IsZero() {
		req.EffectiveDate = time.Now()
	}

	if req.ApprovalReference == "" {
		req.ApprovalReference = fmt.Sprintf("REPLACE-%d", time.Now().Unix())
	}

	newAppID := security.GenerateUUID()
	var newApp db.Appointment

	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		// 1. Terminate previous appointment
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:      oldAppointmentID,
			Status:  "replaced",
			EndDate: TimestamptzFromTime(req.EffectiveDate),
		})
		if err != nil {
			return err
		}

		// 2. Create new appointment with identical jurisdiction
		created, err := q.CreateAppointment(ctx, db.CreateAppointmentParams{
			ID:                       newAppID,
			UserID:                   req.NewUserID,
			RoleTitle:                oldApp.RoleTitle,
			RoleType:                 oldApp.RoleType,
			JurisdictionType:         oldApp.JurisdictionType,
			JurisdictionID:           oldApp.JurisdictionID,
			ReportingToAppointmentID: oldApp.ReportingToAppointmentID,
			StartDate:                TimestamptzFromTime(req.EffectiveDate),
			EndDate:                  pgtype.Timestamptz{Valid: false},
			Status:                   "active",
			ApprovalReference:        req.ApprovalReference,
			PublicContact:            Text(req.PublicContact),
		})
		if err != nil {
			return err
		}
		newApp = created

		// 3. Record appointment history for the old appointment
		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   oldAppointmentID,
			Action:          "replaced",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  TextFromString(oldApp.Status),
			NewStatus:       "replaced",
			Reason:          TextFromString(req.Reason),
			PreviousUserID:  TextFromString(oldApp.UserID),
			NewUserID:       TextFromString(req.NewUserID),
			EffectiveDate:   TimestamptzFromTime(req.EffectiveDate),
		})
		if err != nil {
			return err
		}

		// 4. Record appointment history for the new replacement appointment
		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   newAppID,
			Action:          "appointed_replacement",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  Text(nil),
			NewStatus:       "active",
			Reason:          TextFromString(req.Reason),
			PreviousUserID:  TextFromString(oldApp.UserID),
			NewUserID:       TextFromString(req.NewUserID),
			EffectiveDate:   TimestamptzFromTime(req.EffectiveDate),
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Replaced %s on %s with %s (%s). Reason: %s", oldApp.UserID, oldAppointmentID, req.NewUserID, newAppID, req.Reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "replace_appointment", "appointment", newAppID, &oldApp.JurisdictionType, StringPtrFromText(oldApp.JurisdictionID), &details, nil)

	return &newApp, nil
}

func (s *LeadershipService) UpdateAppointmentScopeAndDates(ctx context.Context, actorID, actorRole, appointmentID string, req UpdateScopeDatesRequest) (*db.Appointment, error) {
	if actorRole != "super_admin" {
		return nil, ErrSuperAdminRequired
	}

	oldApp, err := s.repo.Queries().GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		return nil, errors.New("appointment not found")
	}

	var updated db.Appointment
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		u, err := q.UpdateAppointmentScopeAndDates(ctx, db.UpdateAppointmentScopeAndDatesParams{
			ID:               appointmentID,
			JurisdictionType: req.JurisdictionType,
			JurisdictionID:   Text(req.JurisdictionID),
			StartDate:        TimestamptzFromTime(req.StartDate),
			EndDate:          Timestamptz(req.EndDate),
		})
		if err != nil {
			return err
		}
		updated = u

		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   appointmentID,
			Action:          "scope_dates_modified",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  TextFromString(oldApp.Status),
			NewStatus:       oldApp.Status,
			Reason:          TextFromString(req.Reason),
			PreviousUserID:  TextFromString(oldApp.UserID),
			NewUserID:       TextFromString(oldApp.UserID),
			EffectiveDate:   TimestamptzFromTime(time.Now()),
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	details := fmt.Sprintf("Modified scope and dates for appointment %s (%s). Reason: %s", appointmentID, oldApp.RoleTitle, req.Reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "modify_appointment_scope", "appointment", appointmentID, &req.JurisdictionType, req.JurisdictionID, &details, nil)

	return &updated, nil
}

func (s *LeadershipService) RevokeAppointment(ctx context.Context, actorID, actorRole, appointmentID, reason string) error {
	oldApp, err := s.repo.Queries().GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		return errors.New("appointment not found")
	}

	// Zonal, State, National, Super Admin revocation strictly requires Super Admin authority
	if oldApp.RoleType == "zonal_coordinator" || oldApp.RoleType == "state_coordinator" || oldApp.RoleType == "national_admin" || oldApp.RoleType == "super_admin" {
		if actorRole != "super_admin" {
			return ErrSuperAdminRequired
		}
	}

	now := time.Now()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:      appointmentID,
			Status:  "revoked",
			EndDate: TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   appointmentID,
			Action:          "revoked",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  TextFromString(oldApp.Status),
			NewStatus:       "revoked",
			Reason:          TextFromString(reason),
			PreviousUserID:  TextFromString(oldApp.UserID),
			NewUserID:       Text(nil),
			EffectiveDate:   TimestamptzFromTime(now),
		})
		return err
	})

	if err != nil {
		return err
	}

	details := fmt.Sprintf("Revoked appointment %s (%s). Reason: %s", appointmentID, oldApp.RoleTitle, reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "revoke_appointment", "appointment", appointmentID, nil, nil, &details, nil)
	return nil
}

func (s *LeadershipService) SuspendAppointment(ctx context.Context, actorID, actorRole, appointmentID, reason string) error {
	if actorRole != "super_admin" {
		return ErrSuperAdminRequired
	}

	oldApp, err := s.repo.Queries().GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		return errors.New("appointment not found")
	}

	now := time.Now()
	err = s.repo.WithTx(ctx, func(q *db.Queries) error {
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:      appointmentID,
			Status:  "suspended",
			EndDate: TimestamptzFromTime(now),
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAppointmentHistory(ctx, db.CreateAppointmentHistoryParams{
			ID:              security.GenerateUUID(),
			AppointmentID:   appointmentID,
			Action:          "suspended",
			ChangedByUserID: TextFromString(actorID),
			PreviousStatus:  TextFromString(oldApp.Status),
			NewStatus:       "suspended",
			Reason:          TextFromString(reason),
			PreviousUserID:  TextFromString(oldApp.UserID),
			NewUserID:       Text(nil),
			EffectiveDate:   TimestamptzFromTime(now),
		})
		return err
	})

	if err != nil {
		return err
	}

	details := fmt.Sprintf("Suspended appointment %s (%s). Reason: %s", appointmentID, oldApp.RoleTitle, reason)
	_ = s.audit.Log(ctx, &actorID, &actorRole, "suspend_appointment", "appointment", appointmentID, nil, nil, &details, nil)
	return nil
}

func (s *LeadershipService) ListAppointmentHistory(ctx context.Context, appointmentID string) ([]db.ListAppointmentHistoryRow, error) {
	return s.repo.Queries().ListAppointmentHistory(ctx, appointmentID)
}
