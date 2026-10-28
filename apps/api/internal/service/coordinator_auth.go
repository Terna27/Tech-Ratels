package service

import (
	"context"
	"errors"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/repository"
)

var (
	ErrCoordinatorUnauthorized   = errors.New("COORDINATOR_UNAUTHORIZED: active approved coordinator appointment required")
	ErrSelfVerificationNotAllowed = errors.New("ANTI_SELF_VERIFICATION: coordinators cannot verify their own attendance")
	ErrCheckinWindowClosed       = errors.New("CHECKIN_WINDOW_CLOSED: attendance verification is only permitted during or immediately following the event")
	ErrAlreadyVerified           = errors.New("ALREADY_VERIFIED: member attendance has already been verified for this event")
	ErrInvalidCheckinCode        = errors.New("INVALID_CHECKIN_CODE: no matching registration found for this event and code")
	ErrEventCancelled            = errors.New("EVENT_CANCELLED: event has been cancelled")
	ErrClaimAlreadyReversed      = errors.New("CLAIM_ALREADY_REVERSED: participation claim is already reversed")
	ErrClaimNotApproved          = errors.New("CLAIM_NOT_APPROVED: only approved or verified claims can be reversed")
)

// AuthorizeCoordinatorForJurisdiction performs database-backed authorization verification.
// An actor is authorized ONLY if they have an active approved appointment covering the target jurisdiction
// that has reached its effective start date and has not expired or been suspended/revoked/replaced,
// or if they hold nationwide Super Admin authority.
func AuthorizeCoordinatorForJurisdiction(ctx context.Context, repo *repository.Repository, userID string, stateID, lgaID string) (*db.Appointment, error) {
	// 1. Check if user is Super Admin in users table
	user, err := repo.Queries().GetUserByID(ctx, userID)
	if err == nil && user.PrimaryRole == "super_admin" {
		// Super Admin has nationwide authority
		return &db.Appointment{
			ID:               "super_admin_root",
			UserID:           userID,
			RoleTitle:        "Super Administrator",
			RoleType:         "super_admin",
			JurisdictionType: "national",
			Status:           "active",
		}, nil
	}

	// 2. Fetch all active approved appointments for user that have reached start_date and not expired
	apps, err := repo.Queries().ListActiveApprovedAppointmentsByUser(ctx, userID)
	if err != nil || len(apps) == 0 {
		return nil, ErrCoordinatorUnauthorized
	}

	// Resolve target state and zone
	var targetState *db.State
	var targetZoneID string
	var targetZoneCode string

	if stateID != "" {
		st, err := repo.Queries().GetStateByID(ctx, stateID)
		if err == nil && st.ID != "" {
			targetState = &st
			targetZoneID = st.ZoneID
			if z, zErr := repo.Queries().GetZoneByID(ctx, st.ZoneID); zErr == nil {
				targetZoneCode = z.Code
			}
		}
	}

	// 3. Evaluate each active appointment against target jurisdiction
	for _, app := range apps {
		switch app.RoleType {
		case "super_admin", "national_admin":
			// Nationwide coverage
			return &db.Appointment{
				ID:               app.ID,
				UserID:           app.UserID,
				RoleTitle:        app.RoleTitle,
				RoleType:         app.RoleType,
				JurisdictionType: app.JurisdictionType,
				JurisdictionID:   app.JurisdictionID,
				StartDate:        app.StartDate,
				EndDate:          app.EndDate,
				Status:           app.Status,
			}, nil

		case "zonal_coordinator":
			if app.JurisdictionType == "zone" && app.JurisdictionID.Valid {
				jid := app.JurisdictionID.String
				if jid == targetZoneID || (targetZoneCode != "" && jid == targetZoneCode) {
					return &db.Appointment{
						ID:               app.ID,
						UserID:           app.UserID,
						RoleTitle:        app.RoleTitle,
						RoleType:         app.RoleType,
						JurisdictionType: app.JurisdictionType,
						JurisdictionID:   app.JurisdictionID,
						StartDate:        app.StartDate,
						EndDate:          app.EndDate,
						Status:           app.Status,
					}, nil
				}
			}

		case "state_coordinator":
			if app.JurisdictionType == "state" && app.JurisdictionID.Valid {
				jid := app.JurisdictionID.String
				if jid == stateID || (targetState != nil && jid == targetState.Code) {
					return &db.Appointment{
						ID:               app.ID,
						UserID:           app.UserID,
						RoleTitle:        app.RoleTitle,
						RoleType:         app.RoleType,
						JurisdictionType: app.JurisdictionType,
						JurisdictionID:   app.JurisdictionID,
						StartDate:        app.StartDate,
						EndDate:          app.EndDate,
						Status:           app.Status,
					}, nil
				}
			}

		case "lga_coordinator", "lga_representative":
			if app.JurisdictionType == "lga" && app.JurisdictionID.Valid {
				jid := app.JurisdictionID.String
				if jid == lgaID {
					return &db.Appointment{
						ID:               app.ID,
						UserID:           app.UserID,
						RoleTitle:        app.RoleTitle,
						RoleType:         app.RoleType,
						JurisdictionType: app.JurisdictionType,
						JurisdictionID:   app.JurisdictionID,
						StartDate:        app.StartDate,
						EndDate:          app.EndDate,
						Status:           app.Status,
					}, nil
				}
			}
		}
	}

	return nil, ErrScopeUnauthorized
}

// AuthorizeCoordinatorForEvent loads an event instance and verifies the actor's appointment jurisdiction covers it.
func AuthorizeCoordinatorForEvent(ctx context.Context, repo *repository.Repository, userID, eventID string) (*db.EventInstance, *db.Appointment, error) {
	event, err := repo.Queries().GetEventInstanceByID(ctx, eventID)
	if err != nil {
		return nil, nil, ErrEventNotFound
	}

	app, err := AuthorizeCoordinatorForJurisdiction(ctx, repo, userID, event.StateID, event.LgaID)
	if err != nil {
		return nil, nil, err
	}

	return &event, app, nil
}

// IsWithinCheckinWindow validates if current time falls within an event's active check-in window.
// Standard window: from 4 hours before StartTime to 24 hours after EndTime.
func IsWithinCheckinWindow(now time.Time, startTime, endTime time.Time) bool {
	windowStart := startTime.Add(-4 * time.Hour)
	windowEnd := endTime.Add(24 * time.Hour)
	return (now.Equal(windowStart) || now.After(windowStart)) && (now.Equal(windowEnd) || now.Before(windowEnd))
}
