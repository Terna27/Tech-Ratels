package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
)

var (
	ErrNotificationNotFound  = errors.New("notification not found")
	ErrTemplateNotFound      = errors.New("notification template not found")
	ErrOutboxNotFound        = errors.New("outbox delivery record not found")
	ErrInvalidChannel        = errors.New("invalid delivery channel")
	ErrRecipientOptedOut     = errors.New("recipient has opted out of this notification channel or topic")
	ErrScheduleNotFound      = errors.New("notification schedule not found")
	ErrInvalidPhoneFormat    = errors.New("invalid Nigerian phone number format")
)

// Lagos timezone constant
const LagosTimeZone = "Africa/Lagos"

type NotificationService struct {
	repo        *repository.Repository
	audit       *AuditService
	emailSender delivery.EmailSender
	smsSender   delivery.SMSSender
	appBaseURL  string
}

func NewNotificationService(
	repo *repository.Repository,
	audit *AuditService,
	emailSender delivery.EmailSender,
	smsSender delivery.SMSSender,
	appBaseURL string,
) *NotificationService {
	if appBaseURL == "" {
		appBaseURL = "https://ratels-web.onrender.com"
	}
	return &NotificationService{
		repo:        repo,
		audit:       audit,
		emailSender: emailSender,
		smsSender:   smsSender,
		appBaseURL:  strings.TrimRight(appBaseURL, "/"),
	}
}

// -------------------------------------------------------------
// DTOs
// -------------------------------------------------------------

type NotificationDTO struct {
	ID                  string     `json:"id"`
	RecipientID         string     `json:"recipient_id"`
	NotificationType    string     `json:"notification_type"`
	Title               string     `json:"title"`
	Body                string     `json:"body"`
	RelatedResourceType *string    `json:"related_resource_type,omitempty"`
	RelatedResourceID   *string    `json:"related_resource_id,omitempty"`
	ActionURL           *string    `json:"action_url,omitempty"`
	Priority            string     `json:"priority"`
	IsRead              bool       `json:"is_read"`
	ReadAt              *time.Time `json:"read_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
}

type PaginatedNotificationsResponse struct {
	Items       []NotificationDTO `json:"items"`
	Total       int64             `json:"total"`
	UnreadCount int64             `json:"unread_count"`
	Page        int32             `json:"page"`
	PageSize    int32             `json:"page_size"`
	TotalPages  int32             `json:"total_pages"`
}

type NotificationPreferencesDTO struct {
	UserID                         string     `json:"user_id"`
	InAppEnabled                   bool       `json:"in_app_enabled"`
	EmailEnabled                   bool       `json:"email_enabled"`
	SMSEnabled                     bool       `json:"sms_enabled"`
	CleanupRemindersEnabled        bool       `json:"cleanup_reminders_enabled"`
	LeadershipAlertsEnabled        bool       `json:"leadership_alerts_enabled"`
	ParticipationUpdatesEnabled    bool       `json:"participation_updates_enabled"`
	MarketplaceUpdatesEnabled      bool       `json:"marketplace_updates_enabled"`
	CareersUpdatesEnabled          bool       `json:"careers_updates_enabled"`
	CommunityProjectUpdatesEnabled bool       `json:"community_project_updates_enabled"`
	FinanceAlertsEnabled           bool       `json:"finance_alerts_enabled"`
	AnnouncementsEnabled           bool       `json:"announcements_enabled"`
	SMSConsentRecordedAt           *time.Time `json:"sms_consent_recorded_at,omitempty"`
	SMSConsentWithdrawnAt          *time.Time `json:"sms_consent_withdrawn_at,omitempty"`
	UpdatedAt                      time.Time  `json:"updated_at"`
}

type UpdatePreferencesRequest struct {
	InAppEnabled                   *bool `json:"in_app_enabled"`
	EmailEnabled                   *bool `json:"email_enabled"`
	SMSEnabled                     *bool `json:"sms_enabled"`
	CleanupRemindersEnabled        *bool `json:"cleanup_reminders_enabled"`
	LeadershipAlertsEnabled        *bool `json:"leadership_alerts_enabled"`
	ParticipationUpdatesEnabled    *bool `json:"participation_updates_enabled"`
	MarketplaceUpdatesEnabled      *bool `json:"marketplace_updates_enabled"`
	CareersUpdatesEnabled          *bool `json:"careers_updates_enabled"`
	CommunityProjectUpdatesEnabled *bool `json:"community_project_updates_enabled"`
	FinanceAlertsEnabled           *bool `json:"finance_alerts_enabled"`
	AnnouncementsEnabled           *bool `json:"announcements_enabled"`
}

type OutboxStatsDTO struct {
	TotalCount      int64 `json:"total_count"`
	PendingCount    int64 `json:"pending_count"`
	ProcessingCount int64 `json:"processing_count"`
	DeliveredCount  int64 `json:"delivered_count"`
	FailedCount     int64 `json:"failed_count"`
	SkippedCount    int64 `json:"skipped_count"`
}

type PaginatedOutboxResponse struct {
	Items      []db.NotificationOutbox `json:"items"`
	Total      int64                   `json:"total"`
	Page       int32                   `json:"page"`
	PageSize   int32                   `json:"page_size"`
	TotalPages int32                   `json:"total_pages"`
}

type PaginatedDeliveriesResponse struct {
	Items      []db.NotificationDelivery `json:"items"`
	Total      int64                     `json:"total"`
	Page       int32                     `json:"page"`
	PageSize   int32                     `json:"page_size"`
	TotalPages int32                     `json:"total_pages"`
}

type PaginatedSchedulesResponse struct {
	Items      []db.ListNotificationSchedulesAdminRow `json:"items"`
	Total      int64                                  `json:"total"`
	Page       int32                                  `json:"page"`
	PageSize   int32                                  `json:"page_size"`
	TotalPages int32                                  `json:"total_pages"`
}

type ProviderHealthDTO struct {
	BrevoProvider struct {
		Configured bool   `json:"configured"`
		Provider   string `json:"provider"`
		Status     string `json:"status"` // 'ready', 'sandbox', 'unconfigured'
	} `json:"brevo_provider"`
	TermiiProvider struct {
		Configured bool   `json:"configured"`
		Provider   string `json:"provider"`
		Status     string `json:"status"` // 'ready', 'sandbox', 'unconfigured'
	} `json:"termii_provider"`
	OutboxWorker struct {
		Status string `json:"status"` // 'active'
	} `json:"outbox_worker"`
	LagosTime string `json:"lagos_time"`
}

type CreateInAppNotificationInput struct {
	RecipientID         string
	NotificationType    string
	Title               string
	Body                string
	RelatedResourceType *string
	RelatedResourceID   *string
	ActionURL           *string
	Priority            string // 'low', 'normal', 'high', 'urgent'
	ExpiresAt           *time.Time
}

type EnqueueOutboxInput struct {
	EventType      string
	RecipientID    *string
	RecipientEmail *string
	RecipientPhone *string
	RecipientName  *string
	Channel        string // 'in_app', 'email', 'sms'
	Payload        map[string]any
	IdempotencyKey *string
	NextAttemptAt  *time.Time
}

// -------------------------------------------------------------
// Member In-App Notification Methods
// -------------------------------------------------------------

func (s *NotificationService) ListUserNotifications(ctx context.Context, userID string, unreadOnly *bool, notifType *string, page, pageSize int32) (*PaginatedNotificationsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var unreadParam pgtype.Bool
	if unreadOnly != nil {
		unreadParam = pgtype.Bool{Bool: *unreadOnly, Valid: true}
	}

	var typeParam pgtype.Text
	if notifType != nil && *notifType != "" {
		typeParam = pgtype.Text{String: *notifType, Valid: true}
	}

	rows, err := s.repo.Queries().ListNotificationsByRecipient(ctx, db.ListNotificationsByRecipientParams{
		RecipientID:      userID,
		UnreadOnly:       unreadParam,
		NotificationType: typeParam,
		Limit:            pageSize,
		Offset:           offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list notifications: %w", err)
	}

	total, err := s.repo.Queries().CountNotificationsByRecipient(ctx, db.CountNotificationsByRecipientParams{
		RecipientID:      userID,
		UnreadOnly:       unreadParam,
		NotificationType: typeParam,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count notifications: %w", err)
	}

	unreadCount, err := s.repo.Queries().CountUnreadNotifications(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread count: %w", err)
	}

	items := make([]NotificationDTO, len(rows))
	for i, r := range rows {
		items[i] = NotificationDTO{
			ID:                  r.ID,
			RecipientID:         r.RecipientID,
			NotificationType:    r.NotificationType,
			Title:               r.Title,
			Body:                r.Body,
			RelatedResourceType: StringPtrFromText(r.RelatedResourceType),
			RelatedResourceID:   StringPtrFromText(r.RelatedResourceID),
			ActionURL:           StringPtrFromText(r.ActionUrl),
			Priority:            r.Priority,
			IsRead:              r.IsRead,
			ReadAt:              TimeFromTimestamptz(r.ReadAt),
			CreatedAt:           r.CreatedAt.Time,
			ExpiresAt:           TimeFromTimestamptz(r.ExpiresAt),
		}
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginatedNotificationsResponse{
		Items:       items,
		Total:       total,
		UnreadCount: unreadCount,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
	}, nil
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	return s.repo.Queries().CountUnreadNotifications(ctx, userID)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notifID string) (*NotificationDTO, error) {
	row, err := s.repo.Queries().MarkNotificationRead(ctx, db.MarkNotificationReadParams{
		ID:          notifID,
		RecipientID: userID,
	})
	if err != nil {
		return nil, ErrNotificationNotFound
	}

	dto := &NotificationDTO{
		ID:                  row.ID,
		RecipientID:         row.RecipientID,
		NotificationType:    row.NotificationType,
		Title:               row.Title,
		Body:                row.Body,
		RelatedResourceType: StringPtrFromText(row.RelatedResourceType),
		RelatedResourceID:   StringPtrFromText(row.RelatedResourceID),
		ActionURL:           StringPtrFromText(row.ActionUrl),
		Priority:            row.Priority,
		IsRead:              row.IsRead,
		ReadAt:              TimeFromTimestamptz(row.ReadAt),
		CreatedAt:           row.CreatedAt.Time,
		ExpiresAt:           TimeFromTimestamptz(row.ExpiresAt),
	}
	return dto, nil
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	return s.repo.Queries().MarkAllNotificationsRead(ctx, userID)
}

func (s *NotificationService) DeleteNotification(ctx context.Context, userID, notifID string) error {
	return s.repo.Queries().DeleteNotification(ctx, db.DeleteNotificationParams{
		ID:          notifID,
		RecipientID: userID,
	})
}

// -------------------------------------------------------------
// Preferences Management
// -------------------------------------------------------------

func (s *NotificationService) GetPreferences(ctx context.Context, userID string) (*NotificationPreferencesDTO, error) {
	row, err := s.repo.Queries().GetNotificationPreferences(ctx, userID)
	if err != nil {
		// Return sensible defaults if not yet created in db
		now := time.Now()
		return &NotificationPreferencesDTO{
			UserID:                         userID,
			InAppEnabled:                   true,
			EmailEnabled:                   true,
			SMSEnabled:                     true,
			CleanupRemindersEnabled:        true,
			LeadershipAlertsEnabled:        true,
			ParticipationUpdatesEnabled:    true,
			MarketplaceUpdatesEnabled:      true,
			CareersUpdatesEnabled:          true,
			CommunityProjectUpdatesEnabled: true,
			FinanceAlertsEnabled:           true,
			AnnouncementsEnabled:           true,
			UpdatedAt:                      now,
		}, nil
	}

	return &NotificationPreferencesDTO{
		UserID:                         row.UserID,
		InAppEnabled:                   row.InAppEnabled,
		EmailEnabled:                   row.EmailEnabled,
		SMSEnabled:                     row.SmsEnabled,
		CleanupRemindersEnabled:        row.CleanupRemindersEnabled,
		LeadershipAlertsEnabled:        row.LeadershipAlertsEnabled,
		ParticipationUpdatesEnabled:    row.ParticipationUpdatesEnabled,
		MarketplaceUpdatesEnabled:      row.MarketplaceUpdatesEnabled,
		CareersUpdatesEnabled:          row.CareersUpdatesEnabled,
		CommunityProjectUpdatesEnabled: row.CommunityProjectUpdatesEnabled,
		FinanceAlertsEnabled:           row.FinanceAlertsEnabled,
		AnnouncementsEnabled:           row.AnnouncementsEnabled,
		SMSConsentRecordedAt:           TimeFromTimestamptz(row.SmsConsentRecordedAt),
		SMSConsentWithdrawnAt:          TimeFromTimestamptz(row.SmsConsentWithdrawnAt),
		UpdatedAt:                      row.UpdatedAt.Time,
	}, nil
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID string, req UpdatePreferencesRequest) (*NotificationPreferencesDTO, error) {
	current, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	inApp := current.InAppEnabled
	if req.InAppEnabled != nil {
		inApp = *req.InAppEnabled
	}

	email := current.EmailEnabled
	if req.EmailEnabled != nil {
		email = *req.EmailEnabled
	}

	sms := current.SMSEnabled
	var smsConsentRecordedAt pgtype.Timestamptz
	var smsConsentWithdrawnAt pgtype.Timestamptz

	if current.SMSConsentRecordedAt != nil {
		smsConsentRecordedAt = TimestamptzFromTime(*current.SMSConsentRecordedAt)
	}
	if current.SMSConsentWithdrawnAt != nil {
		smsConsentWithdrawnAt = TimestamptzFromTime(*current.SMSConsentWithdrawnAt)
	}

	if req.SMSEnabled != nil {
		newSMS := *req.SMSEnabled
		if newSMS != current.SMSEnabled {
			now := time.Now()
			if newSMS {
				smsConsentRecordedAt = TimestamptzFromTime(now)
				smsConsentWithdrawnAt = pgtype.Timestamptz{Valid: false}
			} else {
				smsConsentWithdrawnAt = TimestamptzFromTime(now)
			}
		}
		sms = newSMS
	}

	cleanup := current.CleanupRemindersEnabled
	if req.CleanupRemindersEnabled != nil {
		cleanup = *req.CleanupRemindersEnabled
	}

	leadership := current.LeadershipAlertsEnabled
	if req.LeadershipAlertsEnabled != nil {
		leadership = *req.LeadershipAlertsEnabled
	}

	participation := current.ParticipationUpdatesEnabled
	if req.ParticipationUpdatesEnabled != nil {
		participation = *req.ParticipationUpdatesEnabled
	}

	marketplace := current.MarketplaceUpdatesEnabled
	if req.MarketplaceUpdatesEnabled != nil {
		marketplace = *req.MarketplaceUpdatesEnabled
	}

	careers := current.CareersUpdatesEnabled
	if req.CareersUpdatesEnabled != nil {
		careers = *req.CareersUpdatesEnabled
	}

	community := current.CommunityProjectUpdatesEnabled
	if req.CommunityProjectUpdatesEnabled != nil {
		community = *req.CommunityProjectUpdatesEnabled
	}

	finance := current.FinanceAlertsEnabled
	if req.FinanceAlertsEnabled != nil {
		finance = *req.FinanceAlertsEnabled
	}

	announcements := current.AnnouncementsEnabled
	if req.AnnouncementsEnabled != nil {
		announcements = *req.AnnouncementsEnabled
	}

	row, err := s.repo.Queries().UpsertNotificationPreferences(ctx, db.UpsertNotificationPreferencesParams{
		UserID:                         userID,
		InAppEnabled:                   inApp,
		EmailEnabled:                   email,
		SmsEnabled:                     sms,
		CleanupRemindersEnabled:        cleanup,
		LeadershipAlertsEnabled:        leadership,
		ParticipationUpdatesEnabled:    participation,
		MarketplaceUpdatesEnabled:      marketplace,
		CareersUpdatesEnabled:          careers,
		CommunityProjectUpdatesEnabled: community,
		FinanceAlertsEnabled:           finance,
		AnnouncementsEnabled:           announcements,
		SmsConsentRecordedAt:           smsConsentRecordedAt,
		SmsConsentWithdrawnAt:          smsConsentWithdrawnAt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update preferences: %w", err)
	}

	return &NotificationPreferencesDTO{
		UserID:                         row.UserID,
		InAppEnabled:                   row.InAppEnabled,
		EmailEnabled:                   row.EmailEnabled,
		SMSEnabled:                     row.SmsEnabled,
		CleanupRemindersEnabled:        row.CleanupRemindersEnabled,
		LeadershipAlertsEnabled:        row.LeadershipAlertsEnabled,
		ParticipationUpdatesEnabled:    row.ParticipationUpdatesEnabled,
		MarketplaceUpdatesEnabled:      row.MarketplaceUpdatesEnabled,
		CareersUpdatesEnabled:          row.CareersUpdatesEnabled,
		CommunityProjectUpdatesEnabled: row.CommunityProjectUpdatesEnabled,
		FinanceAlertsEnabled:           row.FinanceAlertsEnabled,
		AnnouncementsEnabled:           row.AnnouncementsEnabled,
		SMSConsentRecordedAt:           TimeFromTimestamptz(row.SmsConsentRecordedAt),
		SMSConsentWithdrawnAt:          TimeFromTimestamptz(row.SmsConsentWithdrawnAt),
		UpdatedAt:                      row.UpdatedAt.Time,
	}, nil
}

// -------------------------------------------------------------
// Outbox & Notification Creation
// -------------------------------------------------------------

func (s *NotificationService) IsChannelAndTopicAllowed(prefs *NotificationPreferencesDTO, channel, eventType string, isMandatory bool) bool {
	if isMandatory {
		return true
	}
	if prefs == nil {
		return true
	}

	// 1. Channel check
	switch channel {
	case "in_app":
		if !prefs.InAppEnabled {
			return false
		}
	case "email":
		if !prefs.EmailEnabled {
			return false
		}
	case "sms":
		if !prefs.SMSEnabled {
			return false
		}
	}

	// 2. Topic check
	switch eventType {
	case "cleanup_reminder_7d", "cleanup_reminder_1d", "event_rescheduled":
		return prefs.CleanupRemindersEnabled
	case "leadership_assigned", "leadership_revoked":
		return prefs.LeadershipAlertsEnabled
	case "claim_approved", "claim_rejected", "media_approved", "media_rejected", "media_revision":
		return prefs.ParticipationUpdatesEnabled
	case "order_status":
		return prefs.MarketplaceUpdatesEnabled
	case "career_status":
		return prefs.CareersUpdatesEnabled
	case "project_status":
		return prefs.CommunityProjectUpdatesEnabled
	case "finance_action":
		return prefs.FinanceAlertsEnabled
	case "announcement":
		return prefs.AnnouncementsEnabled
	default:
		return true
	}
}

func (s *NotificationService) CreateInAppNotification(ctx context.Context, q *db.Queries, input CreateInAppNotificationInput) (*db.Notification, error) {
	if q == nil {
		q = s.repo.Queries()
	}

	priority := input.Priority
	if priority == "" {
		priority = "normal"
	}

	notifID := security.GenerateUUID()
	now := time.Now()

	row, err := q.CreateNotification(ctx, db.CreateNotificationParams{
		ID:                  notifID,
		RecipientID:         input.RecipientID,
		NotificationType:    input.NotificationType,
		Title:               input.Title,
		Body:                input.Body,
		RelatedResourceType: Text(input.RelatedResourceType),
		RelatedResourceID:   Text(input.RelatedResourceID),
		ActionUrl:           Text(input.ActionURL),
		Priority:            priority,
		IsRead:              false,
		ReadAt:              pgtype.Timestamptz{Valid: false},
		CreatedAt:           TimestamptzFromTime(now),
		ExpiresAt:           Timestamptz(input.ExpiresAt),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create in-app notification: %w", err)
	}

	return &row, nil
}

func (s *NotificationService) EnqueueOutbox(ctx context.Context, q *db.Queries, input EnqueueOutboxInput) (*db.NotificationOutbox, error) {
	if q == nil {
		q = s.repo.Queries()
	}

	payloadBytes, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	outboxID := security.GenerateUUID()
	now := time.Now()
	nextAttemptAt := now
	if input.NextAttemptAt != nil {
		nextAttemptAt = *input.NextAttemptAt
	}

	var recipID pgtype.Text
	if input.RecipientID != nil {
		recipID = pgtype.Text{String: *input.RecipientID, Valid: true}
	}
	var recipEmail pgtype.Text
	if input.RecipientEmail != nil {
		recipEmail = pgtype.Text{String: *input.RecipientEmail, Valid: true}
	}
	var recipPhone pgtype.Text
	if input.RecipientPhone != nil {
		recipPhone = pgtype.Text{String: *input.RecipientPhone, Valid: true}
	}
	var recipName pgtype.Text
	if input.RecipientName != nil {
		recipName = pgtype.Text{String: *input.RecipientName, Valid: true}
	}
	var idemp pgtype.Text
	if input.IdempotencyKey != nil {
		idemp = pgtype.Text{String: *input.IdempotencyKey, Valid: true}
	}

	row, err := q.EnqueueOutbox(ctx, db.EnqueueOutboxParams{
		ID:             outboxID,
		EventType:      input.EventType,
		RecipientID:    recipID,
		RecipientEmail: recipEmail,
		RecipientPhone: recipPhone,
		RecipientName:  recipName,
		Channel:        input.Channel,
		Payload:        payloadBytes,
		DeliveryState:  "pending",
		AttemptCount:   0,
		MaxAttempts:    5,
		NextAttemptAt:  TimestamptzFromTime(nextAttemptAt),
		IdempotencyKey: idemp,
		CreatedAt:      TimestamptzFromTime(now),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue outbox record: %w", err)
	}

	return &row, nil
}

// -------------------------------------------------------------
// Schedule Management (Cleanup Reminders in Lagos Timezone)
// -------------------------------------------------------------

func (s *NotificationService) ScheduleCleanupReminders(ctx context.Context, qtx *db.Queries, eventID string, scheduleVersion int32, startTime time.Time) error {
	if qtx == nil {
		qtx = s.repo.Queries()
	}

	loc, err := time.LoadLocation(LagosTimeZone)
	if err != nil {
		loc = time.FixedZone("WAT", 3600) // West Africa Time UTC+1
	}

	startTimeLagos := startTime.In(loc)
	now := time.Now().In(loc)

	// Cancel any previously pending schedules for this event
	_ = qtx.CancelPendingSchedulesForEvent(ctx, eventID)

	// Target 1: 7 days before
	target7d := startTimeLagos.AddDate(0, 0, -7)
	if target7d.After(now) {
		_, _ = qtx.CreateNotificationSchedule(ctx, db.CreateNotificationScheduleParams{
			ID:                   security.GenerateUUID(),
			ScheduleType:         "cleanup_reminder_7d",
			EventID:              eventID,
			EventScheduleVersion: scheduleVersion,
			TargetTime:           TimestamptzFromTime(target7d),
			Status:               "pending",
			CreatedAt:            TimestamptzFromTime(now),
			UpdatedAt:            TimestamptzFromTime(now),
		})
	}

	// Target 2: 1 day before
	target1d := startTimeLagos.AddDate(0, 0, -1)
	if target1d.After(now) {
		_, _ = qtx.CreateNotificationSchedule(ctx, db.CreateNotificationScheduleParams{
			ID:                   security.GenerateUUID(),
			ScheduleType:         "cleanup_reminder_1d",
			EventID:              eventID,
			EventScheduleVersion: scheduleVersion,
			TargetTime:           TimestamptzFromTime(target1d),
			Status:               "pending",
			CreatedAt:            TimestamptzFromTime(now),
			UpdatedAt:            TimestamptzFromTime(now),
		})
	}

	return nil
}

func (s *NotificationService) CancelCleanupRemindersForEvent(ctx context.Context, qtx *db.Queries, eventID string) error {
	if qtx == nil {
		qtx = s.repo.Queries()
	}
	return qtx.CancelPendingSchedulesForEvent(ctx, eventID)
}

// ProcessDueSchedules checks for pending reminder schedules that are due, enqueues notifications, and marks them dispatched.
func (s *NotificationService) ProcessDueSchedules(ctx context.Context) (int, error) {
	loc, err := time.LoadLocation(LagosTimeZone)
	if err != nil {
		loc = time.FixedZone("WAT", 3600)
	}
	nowLagos := time.Now().In(loc)

	schedules, err := s.repo.Queries().GetDueNotificationSchedules(ctx, db.GetDueNotificationSchedulesParams{
		TargetTime: TimestamptzFromTime(nowLagos),
		Limit:      50,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to query due schedules: %w", err)
	}

	count := 0
	for _, sched := range schedules {
		err := s.dispatchSchedule(ctx, sched)
		if err != nil {
			log.Printf("[NOTIFICATION_SCHEDULE] Error dispatching schedule %s: %v", sched.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

func (s *NotificationService) dispatchSchedule(ctx context.Context, sched db.NotificationSchedule) error {
	eventCtx, err := s.repo.Queries().GetEventNotificationContext(ctx, sched.EventID)
	if err != nil {
		return fmt.Errorf("failed to get event context: %w", err)
	}

	registeredUsers, err := s.repo.Queries().ListRegisteredUsersForEvent(ctx, sched.EventID)
	if err != nil {
		return fmt.Errorf("failed to list registered users: %w", err)
	}

	loc, _ := time.LoadLocation(LagosTimeZone)
	eventStartTime := eventCtx.StartTime.Time.In(loc)
	dateStr := eventStartTime.Format("Monday, 02 Jan 2006")
	timeStr := eventStartTime.Format("03:04 PM")

	daysWord := "7 days"
	if sched.ScheduleType == "cleanup_reminder_1d" {
		daysWord = "tomorrow"
	}

	for _, u := range registeredUsers {
		emailStr := StringVal(u.Email)
		phoneStr := StringVal(u.Phone)
		nameStr := u.DisplayName
		actionURL := fmt.Sprintf("%s/cleanups/%s", s.appBaseURL, eventCtx.ID)

		payload := map[string]any{
			"event_id":       eventCtx.ID,
			"event_title":    eventCtx.Title,
			"event_date":     dateStr,
			"start_time":     timeStr,
			"event_location": fmt.Sprintf("%s, %s State", eventCtx.Address, eventCtx.StateName),
			"recipient_name": nameStr,
			"action_url":     actionURL,
		}

		// Enqueue in-app
		title := fmt.Sprintf("Cleanup in %s: %s", daysWord, eventCtx.Title)
		body := fmt.Sprintf("Reminder: %s is scheduled for %s at %s (%s, %s State).", eventCtx.Title, dateStr, timeStr, eventCtx.Address, eventCtx.StateName)
		_, _ = s.CreateInAppNotification(ctx, nil, CreateInAppNotificationInput{
			RecipientID:         u.ID,
			NotificationType:    sched.ScheduleType,
			Title:               title,
			Body:                body,
			RelatedResourceType: strPtr("event"),
			RelatedResourceID:   &eventCtx.ID,
			ActionURL:           &actionURL,
			Priority:            "normal",
		})

		// Enqueue Email outbox
		if emailStr != "" {
			idempEmail := fmt.Sprintf("email:%s:%s:%s", sched.ScheduleType, sched.EventID, u.ID)
			_, _ = s.EnqueueOutbox(ctx, nil, EnqueueOutboxInput{
				EventType:      sched.ScheduleType,
				RecipientID:    &u.ID,
				RecipientEmail: &emailStr,
				RecipientName:  &nameStr,
				Channel:        "email",
				Payload:        payload,
				IdempotencyKey: &idempEmail,
			})
		}

		// Enqueue SMS outbox
		if phoneStr != "" {
			idempSMS := fmt.Sprintf("sms:%s:%s:%s", sched.ScheduleType, sched.EventID, u.ID)
			_, _ = s.EnqueueOutbox(ctx, nil, EnqueueOutboxInput{
				EventType:      sched.ScheduleType,
				RecipientID:    &u.ID,
				RecipientPhone: &phoneStr,
				RecipientName:  &nameStr,
				Channel:        "sms",
				Payload:        payload,
				IdempotencyKey: &idempSMS,
			})
		}
	}

	_, err = s.repo.Queries().MarkScheduleDispatched(ctx, sched.ID)
	return err
}

// -------------------------------------------------------------
// Admin Communications Management
// -------------------------------------------------------------

func (s *NotificationService) GetOverviewStats(ctx context.Context) (*OutboxStatsDTO, error) {
	row, err := s.repo.Queries().GetOutboxOverviewStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get outbox overview stats: %w", err)
	}

	return &OutboxStatsDTO{
		TotalCount:      row.TotalCount,
		PendingCount:    row.PendingCount,
		ProcessingCount: row.ProcessingCount,
		DeliveredCount:  row.DeliveredCount,
		FailedCount:     row.FailedCount,
		SkippedCount:    row.SkippedCount,
	}, nil
}

func (s *NotificationService) ListOutboxAdmin(ctx context.Context, deliveryState, channel *string, page, pageSize int32) (*PaginatedOutboxResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var stateParam pgtype.Text
	if deliveryState != nil && *deliveryState != "" {
		stateParam = pgtype.Text{String: *deliveryState, Valid: true}
	}
	var channelParam pgtype.Text
	if channel != nil && *channel != "" {
		channelParam = pgtype.Text{String: *channel, Valid: true}
	}

	rows, err := s.repo.Queries().ListOutboxAdmin(ctx, db.ListOutboxAdminParams{
		DeliveryState: stateParam,
		Channel:       channelParam,
		Limit:         pageSize,
		Offset:        offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list outbox items: %w", err)
	}

	total, err := s.repo.Queries().CountOutboxAdmin(ctx, db.CountOutboxAdminParams{
		DeliveryState: stateParam,
		Channel:       channelParam,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count outbox items: %w", err)
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginatedOutboxResponse{
		Items:      rows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *NotificationService) ListFailedOutboxAdmin(ctx context.Context, page, pageSize int32) (*PaginatedOutboxResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	rows, err := s.repo.Queries().ListFailedOutboxAdmin(ctx, db.ListFailedOutboxAdminParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list failed outbox: %w", err)
	}

	total, err := s.repo.Queries().CountFailedOutboxAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed outbox: %w", err)
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginatedOutboxResponse{
		Items:      rows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *NotificationService) RetryOutboxItem(ctx context.Context, adminUserID, outboxID string) (*db.NotificationOutbox, error) {
	item, err := s.repo.Queries().RetryOutboxItem(ctx, outboxID)
	if err != nil {
		return nil, ErrOutboxNotFound
	}

	if s.audit != nil {
		actionDesc := fmt.Sprintf("Retried outbox message ID %s (%s)", outboxID, item.EventType)
		_ = s.audit.Log(ctx, &adminUserID, strPtr("admin"), "outbox_retry", "notification_outbox", outboxID, nil, nil, &actionDesc, nil)
	}

	return &item, nil
}

func (s *NotificationService) ListDeliveriesAdmin(ctx context.Context, provider, channel, status *string, page, pageSize int32) (*PaginatedDeliveriesResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var provParam pgtype.Text
	if provider != nil && *provider != "" {
		provParam = pgtype.Text{String: *provider, Valid: true}
	}
	var chanParam pgtype.Text
	if channel != nil && *channel != "" {
		chanParam = pgtype.Text{String: *channel, Valid: true}
	}
	var statParam pgtype.Text
	if status != nil && *status != "" {
		statParam = pgtype.Text{String: *status, Valid: true}
	}

	rows, err := s.repo.Queries().ListDeliveriesAdmin(ctx, db.ListDeliveriesAdminParams{
		Provider: provParam,
		Channel:  chanParam,
		Status:   statParam,
		Limit:    pageSize,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list delivery logs: %w", err)
	}

	total, err := s.repo.Queries().CountDeliveriesAdmin(ctx, db.CountDeliveriesAdminParams{
		Provider: provParam,
		Channel:  chanParam,
		Status:   statParam,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count delivery logs: %w", err)
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginatedDeliveriesResponse{
		Items:      rows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *NotificationService) ListTemplatesAdmin(ctx context.Context) ([]db.NotificationTemplate, error) {
	return s.repo.Queries().ListNotificationTemplates(ctx)
}

func (s *NotificationService) UpdateTemplateAdmin(ctx context.Context, adminUserID, templateID, titleTemplate, bodyTemplate string, isActive bool) (*db.NotificationTemplate, error) {
	row, err := s.repo.Queries().UpdateNotificationTemplate(ctx, db.UpdateNotificationTemplateParams{
		ID:            templateID,
		TitleTemplate: titleTemplate,
		BodyTemplate:  bodyTemplate,
		IsActive:      isActive,
	})
	if err != nil {
		return nil, ErrTemplateNotFound
	}

	if s.audit != nil {
		desc := fmt.Sprintf("Updated notification template %s (version %d)", row.TemplateCode, row.Version)
		_ = s.audit.Log(ctx, &adminUserID, strPtr("admin"), "template_updated", "notification_template", templateID, nil, nil, &desc, nil)
	}

	return &row, nil
}

func (s *NotificationService) ListSchedulesAdmin(ctx context.Context, status *string, page, pageSize int32) (*PaginatedSchedulesResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var statParam pgtype.Text
	if status != nil && *status != "" {
		statParam = pgtype.Text{String: *status, Valid: true}
	}

	rows, err := s.repo.Queries().ListNotificationSchedulesAdmin(ctx, db.ListNotificationSchedulesAdminParams{
		Status: statParam,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list notification schedules: %w", err)
	}

	total, err := s.repo.Queries().CountNotificationSchedulesAdmin(ctx, statParam)
	if err != nil {
		return nil, fmt.Errorf("failed to count notification schedules: %w", err)
	}

	totalPages := int32(0)
	if total > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return &PaginatedSchedulesResponse{
		Items:      rows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *NotificationService) GetProviderHealth(ctx context.Context) (*ProviderHealthDTO, error) {
	dto := &ProviderHealthDTO{}

	// Brevo status
	if s.emailSender != nil {
		dto.BrevoProvider.Provider = s.emailSender.ProviderName()
		dto.BrevoProvider.Configured = s.emailSender.IsConfigured()
		switch dto.BrevoProvider.Provider {
		case "brevo":
			if dto.BrevoProvider.Configured {
				dto.BrevoProvider.Status = "ready"
			} else {
				dto.BrevoProvider.Status = "unconfigured"
			}
		case "dev_sandbox":
			dto.BrevoProvider.Status = "sandbox"
		default:
			dto.BrevoProvider.Status = "unconfigured"
		}
	} else {
		dto.BrevoProvider.Provider = "none"
		dto.BrevoProvider.Configured = false
		dto.BrevoProvider.Status = "unconfigured"
	}

	// Termii status
	if s.smsSender != nil {
		dto.TermiiProvider.Provider = s.smsSender.ProviderName()
		dto.TermiiProvider.Configured = s.smsSender.IsConfigured()
		switch dto.TermiiProvider.Provider {
		case "termii":
			if dto.TermiiProvider.Configured {
				dto.TermiiProvider.Status = "ready"
			} else {
				dto.TermiiProvider.Status = "unconfigured"
			}
		case "dev_sandbox":
			dto.TermiiProvider.Status = "sandbox"
		default:
			dto.TermiiProvider.Status = "unconfigured"
		}
	} else {
		dto.TermiiProvider.Provider = "none"
		dto.TermiiProvider.Configured = false
		dto.TermiiProvider.Status = "unconfigured"
	}

	dto.OutboxWorker.Status = "active"

	loc, err := time.LoadLocation(LagosTimeZone)
	if err != nil {
		loc = time.FixedZone("WAT", 3600)
	}
	dto.LagosTime = time.Now().In(loc).Format(time.RFC3339)

	return dto, nil
}

// -------------------------------------------------------------
// Cross-Module Notification Triggers
// -------------------------------------------------------------

func (s *NotificationService) OnLeadershipAssigned(ctx context.Context, qtx *db.Queries, targetUserID, title, jurisdiction string) {
	actionURL := fmt.Sprintf("%s/dashboard", s.appBaseURL)
	notifTitle := "Leadership Appointment Confirmed"
	notifBody := fmt.Sprintf("You have been appointed as %s for %s. Welcome to the Ratels leadership network!", title, jurisdiction)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         targetUserID,
		NotificationType:    "leadership_assigned",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("appointment"),
		ActionURL:           &actionURL,
		Priority:            "high",
	})
}

func (s *NotificationService) OnLeadershipRevoked(ctx context.Context, qtx *db.Queries, targetUserID, title, reason string) {
	actionURL := fmt.Sprintf("%s/dashboard", s.appBaseURL)
	notifTitle := "Leadership Role Status Update"
	notifBody := fmt.Sprintf("Your appointment as %s has concluded. Reason: %s", title, reason)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         targetUserID,
		NotificationType:    "leadership_revoked",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("appointment"),
		ActionURL:           &actionURL,
		Priority:            "high",
	})
}

func (s *NotificationService) OnParticipationClaimApproved(ctx context.Context, qtx *db.Queries, claimID, userID, eventTitle string) {
	actionURL := fmt.Sprintf("%s/profile", s.appBaseURL)
	notifTitle := "Attendance Verified!"
	notifBody := fmt.Sprintf("Your participation claim for %s has been verified. Cleanliness points have been added to your profile.", eventTitle)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         userID,
		NotificationType:    "claim_approved",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("claim"),
		RelatedResourceID:   &claimID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnParticipationClaimRejected(ctx context.Context, qtx *db.Queries, claimID, userID, eventTitle, reason string) {
	actionURL := fmt.Sprintf("%s/profile", s.appBaseURL)
	notifTitle := "Participation Claim Update"
	notifBody := fmt.Sprintf("Your claim for %s was not approved. Reason: %s", eventTitle, reason)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         userID,
		NotificationType:    "claim_rejected",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("claim"),
		RelatedResourceID:   &claimID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnMediaApproved(ctx context.Context, qtx *db.Queries, mediaID, uploaderID, eventTitle string) {
	actionURL := fmt.Sprintf("%s/gallery", s.appBaseURL)
	notifTitle := "Cleanup Media Approved"
	notifBody := fmt.Sprintf("Your uploaded media for %s has been approved and published to the public gallery.", eventTitle)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         uploaderID,
		NotificationType:    "media_approved",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("media"),
		RelatedResourceID:   &mediaID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnMediaRejected(ctx context.Context, qtx *db.Queries, mediaID, uploaderID, eventTitle, reason string) {
	actionURL := fmt.Sprintf("%s/dashboard", s.appBaseURL)
	notifTitle := "Media Submission Update"
	notifBody := fmt.Sprintf("Your uploaded media for %s could not be approved. Reason: %s", eventTitle, reason)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         uploaderID,
		NotificationType:    "media_rejected",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("media"),
		RelatedResourceID:   &mediaID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnMediaRevisionRequested(ctx context.Context, qtx *db.Queries, mediaID, uploaderID, eventTitle, instructions string) {
	actionURL := fmt.Sprintf("%s/dashboard", s.appBaseURL)
	notifTitle := "Media Revision Requested"
	notifBody := fmt.Sprintf("Moderators requested revisions on your media for %s: %s", eventTitle, instructions)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         uploaderID,
		NotificationType:    "media_revision",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("media"),
		RelatedResourceID:   &mediaID,
		ActionURL:           &actionURL,
		Priority:            "high",
	})
}

func (s *NotificationService) OnMarketplaceOrderStatus(ctx context.Context, qtx *db.Queries, userID, orderID, status, orderRef string) {
	actionURL := fmt.Sprintf("%s/marketplace/orders/%s", s.appBaseURL, orderID)
	notifTitle := fmt.Sprintf("Order %s: %s", orderRef, strings.ToUpper(status))
	notifBody := fmt.Sprintf("The status of your marketplace order #%s has updated to %s.", orderRef, status)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         userID,
		NotificationType:    "order_status",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("order"),
		RelatedResourceID:   &orderID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnCareerApplicationStatus(ctx context.Context, qtx *db.Queries, candidateUserID, appID, jobTitle, newStatus, notes string) {
	actionURL := fmt.Sprintf("%s/careers/applications", s.appBaseURL)
	notifTitle := fmt.Sprintf("Application Update: %s", jobTitle)
	notifBody := fmt.Sprintf("Your application for '%s' is now %s.", jobTitle, newStatus)
	if notes != "" {
		notifBody += fmt.Sprintf(" Notes: %s", notes)
	}

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         candidateUserID,
		NotificationType:    "career_status",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("application"),
		RelatedResourceID:   &appID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnCommunityProjectStatus(ctx context.Context, qtx *db.Queries, initiatorUserID, projectID, title, newStatus string) {
	actionURL := fmt.Sprintf("%s/projects/%s", s.appBaseURL, projectID)
	notifTitle := fmt.Sprintf("Project Status: %s", title)
	notifBody := fmt.Sprintf("Your community project proposal '%s' has moved to status: %s.", title, newStatus)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         initiatorUserID,
		NotificationType:    "project_status",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("project"),
		RelatedResourceID:   &projectID,
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
}

func (s *NotificationService) OnFinanceAction(ctx context.Context, qtx *db.Queries, targetUserID, actionType, reference, amount string) {
	actionURL := fmt.Sprintf("%s/finance", s.appBaseURL)
	notifTitle := fmt.Sprintf("Financial Notice: %s", actionType)
	notifBody := fmt.Sprintf("A financial action (%s) with reference %s amounting to ₦%s has been processed.", actionType, reference, amount)

	_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
		RecipientID:         targetUserID,
		NotificationType:    "finance_action",
		Title:               notifTitle,
		Body:                notifBody,
		RelatedResourceType: strPtr("finance"),
		ActionURL:           &actionURL,
		Priority:            "high",
	})
}

func (s *NotificationService) OnEventRescheduled(ctx context.Context, qtx *db.Queries, eventID, newDateStr, reason string) {
	eventCtx, err := s.repo.Queries().GetEventNotificationContext(ctx, eventID)
	if err != nil {
		return
	}

	users, err := s.repo.Queries().ListRegisteredUsersForEvent(ctx, eventID)
	if err != nil {
		return
	}

	actionURL := fmt.Sprintf("%s/cleanups/%s", s.appBaseURL, eventID)
	title := fmt.Sprintf("Rescheduled: %s", eventCtx.Title)
	body := fmt.Sprintf("Notice: %s has been rescheduled to %s. Reason: %s", eventCtx.Title, newDateStr, reason)

	for _, u := range users {
		_, _ = s.CreateInAppNotification(ctx, qtx, CreateInAppNotificationInput{
			RecipientID:         u.ID,
			NotificationType:    "event_rescheduled",
			Title:               title,
			Body:                body,
			RelatedResourceType: strPtr("event"),
			RelatedResourceID:   &eventID,
			ActionURL:           &actionURL,
			Priority:            "high",
		})
	}
}

// -------------------------------------------------------------
// Safe Template Rendering Helper
// -------------------------------------------------------------

func RenderSafeTemplate(templateStr string, data map[string]string) string {
	res := templateStr
	for k, v := range data {
		placeholder1 := fmt.Sprintf("{{.%s}}", k)
		placeholder2 := fmt.Sprintf("{{%s}}", k)
		res = strings.ReplaceAll(res, placeholder1, v)
		res = strings.ReplaceAll(res, placeholder2, v)
	}
	return res
}
