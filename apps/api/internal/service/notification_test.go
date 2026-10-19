package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ratels-api/internal/db"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

func createTestUser(t *testing.T, env *TestEnv, email, role string) db.User {
	t.Helper()
	ctx := context.Background()
	userID := security.GenerateUUID()

	user, err := env.Repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               service.TextFromString(email),
		Phone:               service.TextFromString(fmt.Sprintf("+23480%08d", time.Now().UnixNano()%100000000)),
		PasswordHash:        "testhash",
		DisplayName:         "Test User",
		MembershipReference: fmt.Sprintf("REF-%s", userID[:6]),
		Status:              "active",
		PrimaryRole:         role,
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

func TestNotificationService_InAppCRUD(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	user := createTestUser(t, env, fmt.Sprintf("notif_user_%d@test.com", time.Now().UnixNano()), "member")

	// 1. Initially unread count is 0
	unreadCount, err := env.NotificationService.GetUnreadCount(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get unread count: %v", err)
	}
	if unreadCount != 0 {
		t.Fatalf("expected 0 unread notifications, got %d", unreadCount)
	}

	// 2. Create in-app notification
	actionURL := "https://ratels.org/cleanups/123"
	notif, err := env.NotificationService.CreateInAppNotification(ctx, nil, service.CreateInAppNotificationInput{
		RecipientID:         user.ID,
		NotificationType:    "cleanup_reminder",
		Title:               "Upcoming Cleanup in Abuja",
		Body:                "Join us this Saturday for the central area cleanup.",
		RelatedResourceType: strPtr("event"),
		ActionURL:           &actionURL,
		Priority:            "normal",
	})
	if err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}
	if notif.ID == "" || notif.IsRead {
		t.Fatalf("expected unread notification, got is_read=%v", notif.IsRead)
	}

	// 3. Unread count should now be 1
	unreadCount, err = env.NotificationService.GetUnreadCount(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get unread count: %v", err)
	}
	if unreadCount != 1 {
		t.Fatalf("expected 1 unread notification, got %d", unreadCount)
	}

	// 4. List notifications
	listResp, err := env.NotificationService.ListUserNotifications(ctx, user.ID, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("failed to list notifications: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 notification in list, got total=%d len=%d", listResp.Total, len(listResp.Items))
	}
	if listResp.Items[0].Title != "Upcoming Cleanup in Abuja" {
		t.Fatalf("unexpected title: %s", listResp.Items[0].Title)
	}

	// 5. Mark as read
	readNotif, err := env.NotificationService.MarkAsRead(ctx, user.ID, notif.ID)
	if err != nil {
		t.Fatalf("failed to mark notification as read: %v", err)
	}
	if !readNotif.IsRead || readNotif.ReadAt == nil {
		t.Fatalf("expected notification to be marked as read with timestamp")
	}

	unreadCount, err = env.NotificationService.GetUnreadCount(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get unread count: %v", err)
	}
	if unreadCount != 0 {
		t.Fatalf("expected 0 unread notifications after mark as read, got %d", unreadCount)
	}

	// 6. Delete notification
	err = env.NotificationService.DeleteNotification(ctx, user.ID, notif.ID)
	if err != nil {
		t.Fatalf("failed to delete notification: %v", err)
	}

	listResp, err = env.NotificationService.ListUserNotifications(ctx, user.ID, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("failed to list notifications after delete: %v", err)
	}
	if listResp.Total != 0 {
		t.Fatalf("expected 0 notifications after delete, got %d", listResp.Total)
	}
}

func TestNotificationService_Preferences(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	user := createTestUser(t, env, fmt.Sprintf("pref_user_%d@test.com", time.Now().UnixNano()), "member")

	// 1. Initial preferences return sensible defaults
	prefs, err := env.NotificationService.GetPreferences(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to get initial preferences: %v", err)
	}
	if !prefs.InAppEnabled || !prefs.EmailEnabled || !prefs.SMSEnabled {
		t.Fatalf("expected all channels enabled by default")
	}

	// 2. Disable SMS and Cleanup Reminders
	f := false
	updated, err := env.NotificationService.UpdatePreferences(ctx, user.ID, service.UpdatePreferencesRequest{
		SMSEnabled:              &f,
		CleanupRemindersEnabled: &f,
	})
	if err != nil {
		t.Fatalf("failed to update preferences: %v", err)
	}
	if updated.SMSEnabled || updated.CleanupRemindersEnabled {
		t.Fatalf("expected SMS and cleanup reminders to be disabled")
	}
	if updated.SMSConsentWithdrawnAt == nil {
		t.Fatalf("expected SMS consent withdrawal timestamp to be recorded")
	}

	// 3. Re-enable SMS
	tr := true
	reUpdated, err := env.NotificationService.UpdatePreferences(ctx, user.ID, service.UpdatePreferencesRequest{
		SMSEnabled: &tr,
	})
	if err != nil {
		t.Fatalf("failed to re-enable SMS preferences: %v", err)
	}
	if !reUpdated.SMSEnabled {
		t.Fatalf("expected SMS to be re-enabled")
	}
	if reUpdated.SMSConsentRecordedAt == nil {
		t.Fatalf("expected SMS consent recording timestamp to be updated")
	}
}

func TestNotificationService_AdminOutboxOversight(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	user := createTestUser(t, env, fmt.Sprintf("outbox_admin_user_%d@test.com", time.Now().UnixNano()), "member")

	emailStr := "member@ratels.org"
	nameStr := "Comrade Ratel"
	idemp := fmt.Sprintf("test_idemp_%d", time.Now().UnixNano())

	// Enqueue outbox item
	item, err := env.NotificationService.EnqueueOutbox(ctx, nil, service.EnqueueOutboxInput{
		EventType:      "cleanup_reminder_7d",
		RecipientID:    &user.ID,
		RecipientEmail: &emailStr,
		RecipientName:  &nameStr,
		Channel:        "email",
		Payload: map[string]any{
			"EventTitle": "Lagos Beach Cleanup",
		},
		IdempotencyKey: &idemp,
	})
	if err != nil {
		t.Fatalf("failed to enqueue outbox: %v", err)
	}
	if item.DeliveryState != "pending" {
		t.Fatalf("expected pending delivery state, got %s", item.DeliveryState)
	}

	// Stats check
	stats, err := env.NotificationService.GetOverviewStats(ctx)
	if err != nil {
		t.Fatalf("failed to get outbox stats: %v", err)
	}
	if stats.PendingCount < 1 {
		t.Fatalf("expected at least 1 pending outbox item, got %d", stats.PendingCount)
	}

	// List admin outbox
	listResp, err := env.NotificationService.ListOutboxAdmin(ctx, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("failed to list admin outbox: %v", err)
	}
	if listResp.Total < 1 {
		t.Fatalf("expected at least 1 outbox item in admin list")
	}

	// Provider health check
	health, err := env.NotificationService.GetProviderHealth(ctx)
	if err != nil {
		t.Fatalf("failed to get provider health: %v", err)
	}
	if health.BrevoProvider.Provider == "" || health.TermiiProvider.Provider == "" {
		t.Fatalf("expected provider health details to be populated")
	}
}
