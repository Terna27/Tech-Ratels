package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"ratels-api/internal/db"
	"ratels-api/internal/delivery"
	"ratels-api/internal/repository"
	"ratels-api/internal/security"
	"ratels-api/internal/service"
)

type WorkerConfig struct {
	PollInterval    time.Duration
	SchedulePoll    time.Duration
	BatchSize       int32
	LeaseDuration   time.Duration
	MaxRetryJitter  time.Duration
	WorkerID        string
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval:   3 * time.Second,
		SchedulePoll:   30 * time.Second,
		BatchSize:      10,
		LeaseDuration:  5 * time.Minute,
		MaxRetryJitter: 5 * time.Second,
		WorkerID:       fmt.Sprintf("worker-%s", security.GenerateUUID()[:8]),
	}
}

type OutboxWorker struct {
	repo         *repository.Repository
	notifService *service.NotificationService
	emailSender  delivery.EmailSender
	smsSender    delivery.SMSSender
	config       WorkerConfig
	cancelFunc   context.CancelFunc
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

func NewOutboxWorker(
	repo *repository.Repository,
	notifService *service.NotificationService,
	emailSender delivery.EmailSender,
	smsSender delivery.SMSSender,
	cfg *WorkerConfig,
) *OutboxWorker {
	config := DefaultWorkerConfig()
	if cfg != nil {
		if cfg.PollInterval > 0 {
			config.PollInterval = cfg.PollInterval
		}
		if cfg.SchedulePoll > 0 {
			config.SchedulePoll = cfg.SchedulePoll
		}
		if cfg.BatchSize > 0 {
			config.BatchSize = cfg.BatchSize
		}
		if cfg.LeaseDuration > 0 {
			config.LeaseDuration = cfg.LeaseDuration
		}
		if cfg.WorkerID != "" {
			config.WorkerID = cfg.WorkerID
		}
	}

	return &OutboxWorker{
		repo:         repo,
		notifService: notifService,
		emailSender:  emailSender,
		smsSender:    smsSender,
		config:       config,
	}
}

// Start launches the background worker goroutines.
func (w *OutboxWorker) Start(parentCtx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	w.cancelFunc = cancel
	w.running = true
	w.mu.Unlock()

	log.Printf("[OUTBOX_WORKER] Started background worker (ID: %s, poll: %s, schedulePoll: %s)",
		w.config.WorkerID, w.config.PollInterval, w.config.SchedulePoll)

	// 1. Outbox processing loop
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processOutboxBatch(ctx)
			}
		}
	}()

	// 2. Schedule reminders checking loop
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.SchedulePoll)
		defer ticker.Stop()

		// Run once on startup
		if w.notifService != nil {
			_, _ = w.notifService.ProcessDueSchedules(ctx)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if w.notifService != nil {
					_, err := w.notifService.ProcessDueSchedules(ctx)
					if err != nil && !errors.Is(err, context.Canceled) {
						log.Printf("[OUTBOX_WORKER] Schedule reminder check error: %v", err)
					}
				}
			}
		}
	}()
}

// Stop gracefully signals background goroutines to stop and waits for completion.
func (w *OutboxWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	w.mu.Unlock()

	w.wg.Wait()
	log.Printf("[OUTBOX_WORKER] Worker %s stopped gracefully", w.config.WorkerID)
}

// ProcessBatchSync processes a single batch synchronously (useful for integration tests).
func (w *OutboxWorker) ProcessBatchSync(ctx context.Context) int {
	return w.processOutboxBatch(ctx)
}

func (w *OutboxWorker) processOutboxBatch(ctx context.Context) int {
	now := time.Now()
	leasedUntil := now.Add(w.config.LeaseDuration)
	leaseToken := fmt.Sprintf("%s:%s", w.config.WorkerID, security.GenerateUUID())

	items, err := w.repo.Queries().ClaimOutboxBatch(ctx, db.ClaimOutboxBatchParams{
		LeaseToken:    pgtype.Text{String: leaseToken, Valid: true},
		LeasedUntil:   pgtype.Timestamptz{Time: leasedUntil, Valid: true},
		NextAttemptAt: pgtype.Timestamptz{Time: now, Valid: true},
		Limit:         w.config.BatchSize,
	})
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("[OUTBOX_WORKER] Error claiming outbox batch: %v", err)
		}
		return 0
	}

	if len(items) == 0 {
		return 0
	}

	for _, item := range items {
		w.processItem(ctx, item)
	}

	return len(items)
}

func (w *OutboxWorker) processItem(ctx context.Context, item db.NotificationOutbox) {
	// Parse payload
	var payload map[string]any
	if len(item.Payload) > 0 {
		_ = json.Unmarshal(item.Payload, &payload)
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	// 1. Check preferences if recipient is a registered user
	if item.RecipientID.Valid && item.RecipientID.String != "" && w.notifService != nil {
		prefs, err := w.notifService.GetPreferences(ctx, item.RecipientID.String)
		if err == nil && !w.notifService.IsChannelAndTopicAllowed(prefs, item.Channel, item.EventType, false) {
			// User opted out
			_, _ = w.repo.Queries().MarkOutboxSkipped(ctx, db.MarkOutboxSkippedParams{
				ID:        item.ID,
				LastError: pgtype.Text{String: "Recipient opted out of this channel or topic in notification preferences", Valid: true},
			})
			_ = w.recordDelivery(ctx, item.ID, item.Channel, "preferences", "", "skipped", "opt_out", "Recipient opted out", int32(item.AttemptCount+1), "")
			return
		}
	}

	// 2. Dispatch based on channel
	var dispatchErr error
	var providerMsgID string
	var providerName string

	switch item.Channel {
	case "email":
		providerName, providerMsgID, dispatchErr = w.dispatchEmail(ctx, item, payload)
	case "sms":
		providerName, providerMsgID, dispatchErr = w.dispatchSMS(ctx, item, payload)
	case "in_app":
		providerName, providerMsgID, dispatchErr = w.dispatchInApp(ctx, item, payload)
	default:
		dispatchErr = fmt.Errorf("unsupported delivery channel: %s", item.Channel)
		providerName = "unknown"
	}

	if dispatchErr == nil {
		// Delivery accepted
		_, _ = w.repo.Queries().MarkOutboxDelivered(ctx, item.ID)
		_ = w.recordDelivery(ctx, item.ID, item.Channel, providerName, providerMsgID, "accepted", "", "", int32(item.AttemptCount+1), "")
		return
	}

	// Handle delivery failure
	category := categorizeDeliveryError(dispatchErr)
	errMsg := dispatchErr.Error()
	attemptCount := item.AttemptCount + 1

	_ = w.recordDelivery(ctx, item.ID, item.Channel, providerName, "", "failed", category, errMsg, int32(attemptCount), "")

	if attemptCount >= item.MaxAttempts || category == "provider_unconfigured" {
		// Fail fast if unconfigured or max attempts reached -> dead letter
		_, _ = w.repo.Queries().MarkOutboxDeadLetter(ctx, db.MarkOutboxDeadLetterParams{
			ID:                item.ID,
			LastError:         pgtype.Text{String: errMsg, Valid: true},
			LastErrorCategory: pgtype.Text{String: category, Valid: true},
		})
	} else {
		// Calculate exponential backoff with jitter
		// base = 5s * 2^attempt, plus random jitter
		backoffSeconds := int64(5 * (1 << item.AttemptCount))
		if backoffSeconds > 3600 {
			backoffSeconds = 3600
		}
		jitter := rand.Int63n(5)
		nextAttempt := time.Now().Add(time.Duration(backoffSeconds+jitter) * time.Second)

		_, _ = w.repo.Queries().MarkOutboxRetry(ctx, db.MarkOutboxRetryParams{
			ID:                item.ID,
			NextAttemptAt:     pgtype.Timestamptz{Time: nextAttempt, Valid: true},
			LastError:         pgtype.Text{String: errMsg, Valid: true},
			LastErrorCategory: pgtype.Text{String: category, Valid: true},
		})
	}
}

func (w *OutboxWorker) dispatchEmail(ctx context.Context, item db.NotificationOutbox, payload map[string]any) (string, string, error) {
	if w.emailSender == nil {
		return "none", "", delivery.ErrBrevoNotConfigured
	}

	providerName := w.emailSender.ProviderName()

	recipientEmail := ""
	if item.RecipientEmail.Valid {
		recipientEmail = item.RecipientEmail.String
	}
	if recipientEmail == "" {
		return providerName, "", errors.New("missing recipient email address")
	}

	recipientName := "Ratels Member"
	if item.RecipientName.Valid && item.RecipientName.String != "" {
		recipientName = item.RecipientName.String
	}

	// Extract subject & body from payload or template
	subject, htmlBody, textBody := w.resolveEmailContent(ctx, item.EventType, recipientName, payload)

	msgID, err := w.emailSender.SendNotificationEmail(ctx, recipientEmail, recipientName, subject, htmlBody, textBody)
	return providerName, msgID, err
}

func (w *OutboxWorker) dispatchSMS(ctx context.Context, item db.NotificationOutbox, payload map[string]any) (string, string, error) {
	if w.smsSender == nil {
		return "none", "", delivery.ErrTermiiNotConfigured
	}

	providerName := w.smsSender.ProviderName()

	phone := ""
	if item.RecipientPhone.Valid {
		phone = item.RecipientPhone.String
	}
	if phone == "" {
		return providerName, "", errors.New("missing recipient phone number")
	}

	message := w.resolveSMSContent(ctx, item.EventType, payload)
	if message == "" {
		return providerName, "", errors.New("empty SMS message content")
	}

	msgID, err := w.smsSender.SendSMS(ctx, phone, message)
	return providerName, msgID, err
}

func (w *OutboxWorker) dispatchInApp(ctx context.Context, item db.NotificationOutbox, payload map[string]any) (string, string, error) {
	if !item.RecipientID.Valid || item.RecipientID.String == "" {
		return "in_app", "", errors.New("in-app notification requires a recipient user ID")
	}

	title, ok := payload["title"].(string)
	if !ok || title == "" {
		title = "Notification"
	}
	body, _ := payload["body"].(string)
	actionURL, _ := payload["action_url"].(string)
	relType, _ := payload["related_resource_type"].(string)
	relID, _ := payload["related_resource_id"].(string)

	var actURLPtr *string
	if actionURL != "" {
		actURLPtr = &actionURL
	}
	var relTypePtr *string
	if relType != "" {
		relTypePtr = &relType
	}
	var relIDPtr *string
	if relID != "" {
		relIDPtr = &relID
	}

	notif, err := w.notifService.CreateInAppNotification(ctx, nil, service.CreateInAppNotificationInput{
		RecipientID:         item.RecipientID.String,
		NotificationType:    item.EventType,
		Title:               title,
		Body:                body,
		RelatedResourceType: relTypePtr,
		RelatedResourceID:   relIDPtr,
		ActionURL:           actURLPtr,
		Priority:            "normal",
	})
	if err != nil {
		return "in_app", "", err
	}

	return "in_app", notif.ID, nil
}

func (w *OutboxWorker) resolveEmailContent(ctx context.Context, eventType, recipientName string, payload map[string]any) (string, string, string) {
	// If explicit subject and content passed in payload, prefer them
	if subj, ok := payload["subject"].(string); ok && subj != "" {
		html, _ := payload["html_content"].(string)
		text, _ := payload["text_content"].(string)
		if text == "" {
			text = html
		}
		return subj, html, text
	}

	// Try loading template from DB
	templateCode := fmt.Sprintf("%s_email", eventType)
	tpl, err := w.repo.Queries().GetNotificationTemplateByCode(ctx, templateCode)
	if err == nil {
		dataMap := make(map[string]string)
		dataMap["RecipientName"] = recipientName
		for k, v := range payload {
			dataMap[k] = fmt.Sprintf("%v", v)
		}
		title := service.RenderSafeTemplate(tpl.TitleTemplate, dataMap)
		body := service.RenderSafeTemplate(tpl.BodyTemplate, dataMap)
		htmlBody := fmt.Sprintf(`<p>%s</p>`, body)
		return title, htmlBody, body
	}

	// Fallback generic content
	title := fmt.Sprintf("Nigerian Ratels Notification: %s", strings.ReplaceAll(eventType, "_", " "))
	body := fmt.Sprintf("Hello %s,\n\nYou have a new notification regarding %s.", recipientName, eventType)
	htmlBody := fmt.Sprintf("<p>%s</p>", body)
	return title, htmlBody, body
}

func (w *OutboxWorker) resolveSMSContent(ctx context.Context, eventType string, payload map[string]any) string {
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}

	templateCode := fmt.Sprintf("%s_sms", eventType)
	tpl, err := w.repo.Queries().GetNotificationTemplateByCode(ctx, templateCode)
	if err == nil {
		dataMap := make(map[string]string)
		for k, v := range payload {
			dataMap[k] = fmt.Sprintf("%v", v)
		}
		return service.RenderSafeTemplate(tpl.BodyTemplate, dataMap)
	}

	return fmt.Sprintf("Ratels Alert: New update regarding %s.", eventType)
}

func (w *OutboxWorker) recordDelivery(
	ctx context.Context,
	outboxID, channel, provider, providerMsgRef, status, failureCategory, errMsg string,
	attemptNumber int32,
	responsePayload string,
) error {
	var ref pgtype.Text
	if providerMsgRef != "" {
		ref = pgtype.Text{String: providerMsgRef, Valid: true}
	}
	var failCat pgtype.Text
	if failureCategory != "" {
		failCat = pgtype.Text{String: failureCategory, Valid: true}
	}
	var errText pgtype.Text
	if errMsg != "" {
		errText = pgtype.Text{String: errMsg, Valid: true}
	}
	var respPay pgtype.Text
	if responsePayload != "" {
		respPay = pgtype.Text{String: responsePayload, Valid: true}
	}

	_, err := w.repo.Queries().CreateDeliveryRecord(ctx, db.CreateDeliveryRecordParams{
		ID:                 security.GenerateUUID(),
		OutboxID:           outboxID,
		Provider:           provider,
		Channel:            channel,
		ProviderMessageRef: ref,
		Status:             status,
		FailureCategory:    failCat,
		ErrorMessage:       errText,
		AttemptNumber:      attemptNumber,
		ResponsePayload:    respPay,
		CreatedAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	return err
}

func categorizeDeliveryError(err error) string {
	if err == nil {
		return ""
	}
	errStr := strings.ToLower(err.Error())

	if errors.Is(err, delivery.ErrBrevoNotConfigured) || errors.Is(err, delivery.ErrTermiiNotConfigured) || strings.Contains(errStr, "not configured") {
		return "provider_unconfigured"
	}
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "api key") || strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
		return "auth_error"
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return "timeout"
	}
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		return "rate_limited"
	}
	if strings.Contains(errStr, "phone") || strings.Contains(errStr, "format") || strings.Contains(errStr, "invalid") || strings.Contains(errStr, "400") {
		return "validation_error"
	}
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") {
		return "network"
	}
	return "provider_error"
}
