package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"ratels-api/internal/service"
)

type RefundWorkerConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

func DefaultRefundWorkerConfig() RefundWorkerConfig {
	return RefundWorkerConfig{
		PollInterval: 10 * time.Second,
		BatchSize:    25,
	}
}

type RefundWorker struct {
	marketplaceService *service.MarketplaceService
	config             RefundWorkerConfig
	cancelFunc         context.CancelFunc
	wg                 sync.WaitGroup
	running            bool
	mu                 sync.Mutex
}

func NewRefundWorker(marketplaceService *service.MarketplaceService, cfg *RefundWorkerConfig) *RefundWorker {
	config := DefaultRefundWorkerConfig()
	if cfg != nil {
		if cfg.PollInterval > 0 {
			config.PollInterval = cfg.PollInterval
		}
		if cfg.BatchSize > 0 {
			config.BatchSize = cfg.BatchSize
		}
	}
	return &RefundWorker{
		marketplaceService: marketplaceService,
		config:             config,
	}
}

func (w *RefundWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.running = true

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.PollInterval)
		defer ticker.Stop()

		// Run immediately once on start to catch up any pending refunds from server restart
		w.runCycle(workerCtx)

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				w.runCycle(workerCtx)
			}
		}
	}()
}

func (w *RefundWorker) runCycle(ctx context.Context) {
	result, err := w.marketplaceService.ProcessPendingRefunds(ctx, w.config.BatchSize)
	if err != nil {
		log.Printf("[REFUND_WORKER] Error processing pending refunds: %v\n", err)
		return
	}
	if result.SettledCount > 0 || result.RetriedCount > 0 || result.FailedCount > 0 {
		log.Printf("[REFUND_WORKER] Pending refunds cycle completed: %d processed, %d settled, %d retried, %d failed\n",
			result.ProcessedCount, result.SettledCount, result.RetriedCount, result.FailedCount)
	}
}

func (w *RefundWorker) Stop() {
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
}
