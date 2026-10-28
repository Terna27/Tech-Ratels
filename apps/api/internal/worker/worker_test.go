package worker_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/migrator"
	"ratels-api/internal/payment"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
	"ratels-api/internal/storage"
	"ratels-api/internal/worker"
)

type workerTestEnv struct {
	pool        *pgxpool.Pool
	repo        *repository.Repository
	notifSvc    *service.NotificationService
	emailSender *delivery.DevSafeEmailSender
	smsSender   *delivery.DevSafeSMSSender
	worker      *worker.OutboxWorker
}

func setupWorkerTestEnv(t *testing.T) *workerTestEnv {
	t.Helper()
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	_ = migrator.RunMigrations(ctx, pool)
	repo := repository.NewRepository(pool)
	auditSvc := service.NewAuditService(repo)
	emailSender := delivery.NewDevSafeEmailSender()
	smsSender := delivery.NewDevSafeSMSSender()
	notifSvc := service.NewNotificationService(repo, auditSvc, emailSender, smsSender, "http://localhost:3000")

	wCfg := &worker.WorkerConfig{
		PollInterval: 100 * time.Millisecond,
		SchedulePoll: 500 * time.Millisecond,
		BatchSize:    10,
	}
	w := worker.NewOutboxWorker(repo, notifSvc, emailSender, smsSender, wCfg)

	return &workerTestEnv{
		pool:        pool,
		repo:        repo,
		notifSvc:    notifSvc,
		emailSender: emailSender,
		smsSender:   smsSender,
		worker:      w,
	}
}

func createWorkerUser(t *testing.T, env *workerTestEnv, email string) db.User {
	t.Helper()
	ctx := context.Background()
	userID := security.GenerateUUID()
	user, err := env.repo.Queries().CreateUser(ctx, db.CreateUserParams{
		ID:                  userID,
		Email:               service.TextFromString(email),
		Phone:               service.TextFromString(fmt.Sprintf("+23480%08d", time.Now().UnixNano()%100000000)),
		PasswordHash:        "testhash",
		DisplayName:         "Worker Test User",
		MembershipReference: fmt.Sprintf("WKR-%s", userID[:6]),
		Status:              "active",
		PrimaryRole:         "member",
		IsVerified:          true,
	})
	if err != nil {
		t.Fatalf("failed to create worker test user: %v", err)
	}
	return user
}

func TestOutboxWorker_EmailAndSMSDispatch(t *testing.T) {
	env := setupWorkerTestEnv(t)
	ctx := context.Background()

	user := createWorkerUser(t, env, fmt.Sprintf("worker_dispatch_%d@test.com", time.Now().UnixNano()))

	emailStr := "dispatch_target@test.com"
	nameStr := "Comrade Dispatch"
	phoneStr := "+2348099887766"

	// 1. Enqueue Email outbox item
	idempEmail := fmt.Sprintf("idemp_email_%d", time.Now().UnixNano())
	emailItem, err := env.notifSvc.EnqueueOutbox(ctx, nil, service.EnqueueOutboxInput{
		EventType:      "cleanup_reminder_7d",
		RecipientID:    &user.ID,
		RecipientEmail: &emailStr,
		RecipientName:  &nameStr,
		Channel:        "email",
		Payload: map[string]any{
			"EventTitle":    "Ikeja Cleanup Drive",
			"EventDate":     "Saturday, 25 Oct 2026",
			"EventLocation": "Ikeja Bus Terminal, Lagos",
		},
		IdempotencyKey: &idempEmail,
	})
	if err != nil {
		t.Fatalf("failed to enqueue email item: %v", err)
	}

	// 2. Enqueue SMS outbox item
	idempSMS := fmt.Sprintf("idemp_sms_%d", time.Now().UnixNano())
	smsItem, err := env.notifSvc.EnqueueOutbox(ctx, nil, service.EnqueueOutboxInput{
		EventType:      "cleanup_reminder_7d",
		RecipientID:    &user.ID,
		RecipientPhone: &phoneStr,
		RecipientName:  &nameStr,
		Channel:        "sms",
		Payload: map[string]any{
			"EventTitle":    "Ikeja Cleanup Drive",
			"EventDate":     "Saturday, 25 Oct 2026",
			"EventLocation": "Ikeja Bus Terminal, Lagos",
		},
		IdempotencyKey: &idempSMS,
	})
	if err != nil {
		t.Fatalf("failed to enqueue SMS item: %v", err)
	}

	// 3. Run worker synchronous batch processing
	processed := env.worker.ProcessBatchSync(ctx)
	if processed < 2 {
		t.Fatalf("expected at least 2 items processed, got %d", processed)
	}

	// 4. Verify email outbox item marked delivered
	emailCheck, err := env.repo.Queries().GetOutboxByID(ctx, emailItem.ID)
	if err != nil {
		t.Fatalf("failed to get email outbox item: %v", err)
	}
	if emailCheck.DeliveryState != "delivered" {
		t.Fatalf("expected email state to be 'delivered', got '%s'", emailCheck.DeliveryState)
	}

	// 5. Verify SMS outbox item marked delivered
	smsCheck, err := env.repo.Queries().GetOutboxByID(ctx, smsItem.ID)
	if err != nil {
		t.Fatalf("failed to get SMS outbox item: %v", err)
	}
	if smsCheck.DeliveryState != "delivered" {
		t.Fatalf("expected SMS state to be 'delivered', got '%s'", smsCheck.DeliveryState)
	}

	// 6. Verify delivery audit logs created
	deliveries, err := env.repo.Queries().ListDeliveriesAdmin(ctx, db.ListDeliveriesAdminParams{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("failed to list deliveries: %v", err)
	}
	if len(deliveries) < 2 {
		t.Fatalf("expected at least 2 delivery audit logs, got %d", len(deliveries))
	}

	// 7. Check sent messages in dev senders
	sentEmails := env.emailSender.GetSentEmails()
	if len(sentEmails) == 0 {
		t.Fatalf("expected at least 1 email recorded in dev sender")
	}
	sentSMS := env.smsSender.GetSentMessages()
	if len(sentSMS) == 0 {
		t.Fatalf("expected at least 1 SMS recorded in dev sender")
	}
}

func TestOutboxWorker_PreferenceOptOutSkipping(t *testing.T) {
	env := setupWorkerTestEnv(t)
	ctx := context.Background()

	user := createWorkerUser(t, env, fmt.Sprintf("worker_optout_%d@test.com", time.Now().UnixNano()))

	// Disable cleanup reminders for this user
	f := false
	_, err := env.notifSvc.UpdatePreferences(ctx, user.ID, service.UpdatePreferencesRequest{
		CleanupRemindersEnabled: &f,
	})
	if err != nil {
		t.Fatalf("failed to update preferences: %v", err)
	}

	emailStr := "optout_user@test.com"
	idemp := fmt.Sprintf("idemp_optout_%d", time.Now().UnixNano())

	item, err := env.notifSvc.EnqueueOutbox(ctx, nil, service.EnqueueOutboxInput{
		EventType:      "cleanup_reminder_7d",
		RecipientID:    &user.ID,
		RecipientEmail: &emailStr,
		Channel:        "email",
		Payload: map[string]any{
			"EventTitle": "Enugu Cleanup",
		},
		IdempotencyKey: &idemp,
	})
	if err != nil {
		t.Fatalf("failed to enqueue item: %v", err)
	}

	// Process batch
	env.worker.ProcessBatchSync(ctx)

	// Item should be marked skipped because of opt-out
	check, err := env.repo.Queries().GetOutboxByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to get outbox item: %v", err)
	}
	if check.DeliveryState != "skipped" {
		t.Fatalf("expected delivery state 'skipped', got '%s'", check.DeliveryState)
	}
}

func TestRefundWorker_Lifecycle(t *testing.T) {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := repository.NewRepository(pool)
	auditSvc := service.NewAuditService(repo)
	sandboxGateway := payment.NewSandboxGateway("http://localhost:3000")
	mockStorage := storage.NewDevSandboxStorageService("http://localhost:3000")
	mktSvc := service.NewMarketplaceService(repo, auditSvc, sandboxGateway, mockStorage)

	w := worker.NewRefundWorker(mktSvc, &worker.RefundWorkerConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
	})

	w.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	w.Stop()
}

func TestRefundWorker_UnconfiguredGatewaySafety(t *testing.T) {
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = "postgres://postgres@127.0.0.1:5432/ratels_migration_audit_db?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := repository.NewRepository(pool)
	auditSvc := service.NewAuditService(repo)
	unconfGateway := payment.NewUnconfiguredGateway()
	mockStorage := storage.NewDevSandboxStorageService("http://localhost:3000")
	mktSvc := service.NewMarketplaceService(repo, auditSvc, unconfGateway, mockStorage)

	w := worker.NewRefundWorker(mktSvc, &worker.RefundWorkerConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
	})

	w.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	w.Stop()

	// Verify that ProcessPendingRefunds returns safely without dispatching or erroring
	res, err := mktSvc.ProcessPendingRefunds(ctx, 10)
	if err != nil {
		t.Fatalf("expected nil error on unconfigured ProcessPendingRefunds, got: %v", err)
	}
	if res.SettledCount != 0 || res.RetriedCount != 0 || res.FailedCount != 0 {
		t.Errorf("expected 0 settled/retried/failed refunds on unconfigured gateway, got %+v", res)
	}
}
