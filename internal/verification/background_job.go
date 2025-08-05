package verification

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	cron "github.com/robfig/cron/v3"
)

// BackgroundVerificationJob handles continuous domain verification
type BackgroundVerificationJob struct {
	db         database.Database
	cron       *cron.Cron
	running    bool
	mu         sync.RWMutex
	config     *BackgroundJobConfig
	notifyFunc NotificationFunc
	stopChan   chan struct{}
	doneChan   chan struct{}
}

// BackgroundJobConfig contains configuration for the background verification job
type BackgroundJobConfig struct {
	// CronSchedule defines when to run verification (default: "0 0 2 * * *" - daily at 2 AM)
	CronSchedule string

	// MaxConcurrentVerifications limits parallel verifications (default: 10)
	MaxConcurrentVerifications int

	// VerificationTimeout is the timeout for each verification attempt (default: 30s)
	VerificationTimeout time.Duration

	// FailureThreshold is the number of consecutive failures before marking as failed (default: 3)
	FailureThreshold int

	// RetryDelay is the delay between verification attempts (default: 1s)
	RetryDelay time.Duration

	// NotificationCooldown is the minimum time between failure notifications (default: 24h)
	NotificationCooldown time.Duration

	// CleanupInterval is how often to clean up old verification records (default: 7 days)
	CleanupInterval time.Duration
}

// NotificationFunc is called when domain verification fails repeatedly
type NotificationFunc func(ctx context.Context, domain string, failures int, lastError error)

// DefaultBackgroundJobConfig returns a sensible default configuration
func DefaultBackgroundJobConfig() *BackgroundJobConfig {
	return &BackgroundJobConfig{
		CronSchedule:               "0 0 2 * * *", // Daily at 2 AM (with seconds)
		MaxConcurrentVerifications: 10,
		VerificationTimeout:        30 * time.Second,
		FailureThreshold:           3,
		RetryDelay:                 time.Second,
		NotificationCooldown:       24 * time.Hour,
		CleanupInterval:            7 * 24 * time.Hour, // 7 days
	}
}

// NewBackgroundVerificationJob creates a new background verification job
func NewBackgroundVerificationJob(db database.Database, config *BackgroundJobConfig, notifyFunc NotificationFunc) *BackgroundVerificationJob {
	if config == nil {
		config = DefaultBackgroundJobConfig()
	}

	if notifyFunc == nil {
		notifyFunc = defaultNotificationFunc
	}

	cronWithSeconds := cron.New(cron.WithSeconds())

	return &BackgroundVerificationJob{
		db:         db,
		cron:       cronWithSeconds,
		config:     config,
		notifyFunc: notifyFunc,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Start begins the background verification job
func (bvj *BackgroundVerificationJob) Start(ctx context.Context) error {
	bvj.mu.Lock()
	defer bvj.mu.Unlock()

	if bvj.running {
		return fmt.Errorf("background verification job is already running")
	}

	// Add the main verification job
	_, err := bvj.cron.AddFunc(bvj.config.CronSchedule, func() {
		if err := bvj.runVerificationCycle(ctx); err != nil {
			log.Printf("Background verification cycle failed: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to schedule verification job: %w", err)
	}

	// Add cleanup job (run daily at 3 AM)
	_, err = bvj.cron.AddFunc("0 0 3 * * *", func() {
		if err := bvj.runCleanup(ctx); err != nil {
			log.Printf("Background cleanup failed: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to schedule cleanup job: %w", err)
	}

	bvj.cron.Start()
	bvj.running = true

	log.Printf("Background verification job started with schedule: %s", bvj.config.CronSchedule)

	// Start monitoring goroutine
	go bvj.monitor(ctx)

	return nil
}

// Stop gracefully stops the background verification job
func (bvj *BackgroundVerificationJob) Stop() error {
	bvj.mu.Lock()
	defer bvj.mu.Unlock()

	if !bvj.running {
		return fmt.Errorf("background verification job is not running")
	}

	log.Println("Stopping background verification job...")

	// Stop the cron scheduler
	bvj.cron.Stop()

	// Signal monitoring goroutine to stop
	close(bvj.stopChan)

	// Wait for monitoring goroutine to finish
	<-bvj.doneChan

	bvj.running = false
	log.Println("Background verification job stopped")

	return nil
}

// IsRunning returns whether the background job is currently running
func (bvj *BackgroundVerificationJob) IsRunning() bool {
	bvj.mu.RLock()
	defer bvj.mu.RUnlock()
	return bvj.running
}

// RunNow triggers an immediate verification cycle
func (bvj *BackgroundVerificationJob) RunNow(ctx context.Context) error {
	return bvj.runVerificationCycle(ctx)
}

// monitor runs in a separate goroutine to handle graceful shutdown
func (bvj *BackgroundVerificationJob) monitor(ctx context.Context) {
	defer close(bvj.doneChan)

	select {
	case <-ctx.Done():
		log.Println("Background verification job context canceled")
	case <-bvj.stopChan:
		log.Println("Background verification job stop signal received")
	}
}

// runVerificationCycle executes a complete verification cycle for all domains
func (bvj *BackgroundVerificationJob) runVerificationCycle(ctx context.Context) error {
	log.Println("Starting background domain verification cycle")
	startTime := time.Now()

	// Get all domains that need verification
	domains, err := bvj.db.GetVerifiedDomains(ctx)
	if err != nil {
		return fmt.Errorf("failed to get verified domains: %w", err)
	}

	if len(domains) == 0 {
		log.Println("No verified domains found for background verification")
		return nil
	}

	log.Printf("Found %d verified domains for background verification", len(domains))

	// Create semaphore for concurrent verification control
	semaphore := make(chan struct{}, bvj.config.MaxConcurrentVerifications)
	var wg sync.WaitGroup

	successCount := 0
	failureCount := 0
	var mu sync.Mutex

	// Process each domain
	for _, domain := range domains {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create verification context with timeout
			verifyCtx, cancel := context.WithTimeout(ctx, bvj.config.VerificationTimeout)
			defer cancel()

			success := bvj.verifyDomain(verifyCtx, domain)

			mu.Lock()
			if success {
				successCount++
			} else {
				failureCount++
			}
			mu.Unlock()
		}(domain)
	}

	// Wait for all verifications to complete
	wg.Wait()

	duration := time.Since(startTime)
	log.Printf("Background verification cycle completed in %v: %d successful, %d failed",
		duration, successCount, failureCount)

	return nil
}

// verifyDomain performs verification for a single domain
func (bvj *BackgroundVerificationJob) verifyDomain(ctx context.Context, domain string) bool {
	log.Printf("Starting background verification for domain: %s", domain)

	// Get existing domain verification record to fetch stored tokens
	domainVerification, err := bvj.db.GetDomainVerification(ctx, domain)
	if err != nil {
		log.Printf("Failed to get domain verification record for %s: %v", domain, err)
		return false
	}

	// Extract tokens for verification
	var dnsToken, httpToken string
	if domainVerification.DNSToken != "" {
		dnsToken = domainVerification.DNSToken
	}
	if domainVerification.HTTPToken != "" {
		httpToken = domainVerification.HTTPToken
	}

	// If no tokens are available, we can't verify
	if dnsToken == "" && httpToken == "" {
		log.Printf("No verification tokens found for domain %s", domain)
		return false
	}

	// Try both DNS and HTTP verification methods using stored tokens
	methods := []model.VerificationMethod{model.VerificationMethodDNS, model.VerificationMethodHTTP}
	var lastError error

	for _, method := range methods {
		var token string
		switch method {
		case model.VerificationMethodDNS:
			token = dnsToken
		case model.VerificationMethodHTTP:
			token = httpToken
		}

		// Skip this method if we don't have a token for it
		if token == "" {
			continue
		}

		success, err := bvj.runSingleVerification(ctx, domain, token, method)
		if err != nil {
			lastError = err
			log.Printf("%s verification failed for %s: %v", method, domain, err)
			continue
		}

		if success {
			return bvj.handleVerificationSuccess(ctx, domain, method)
		}
	}

	// All methods failed
	return bvj.handleVerificationFailure(ctx, domain, lastError)
}

// runSingleVerification performs a single verification attempt
func (bvj *BackgroundVerificationJob) runSingleVerification(
	ctx context.Context, domain, token string, method model.VerificationMethod,
) (bool, error) {
	switch method {
	case model.VerificationMethodDNS:
		// Create a custom config with the provided context's timeout
		config := DefaultDNSConfig()
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining > 0 {
				config.Timeout = remaining
			}
			// If remaining <= 0, keep the default timeout
		}
		result, err := VerifyDNSRecordWithConfig(ctx, domain, token, config)
		if err != nil {
			return false, err
		}
		return result.Success, nil

	case model.VerificationMethodHTTP:
		// Create a custom config with the provided context's timeout
		config := DefaultHTTPConfig()
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining > 0 {
				config.Timeout = remaining
			}
			// If remaining <= 0, keep the default timeout
		}
		result, err := VerifyHTTPChallengeWithConfig(ctx, domain, token, config)
		if err != nil {
			return false, err
		}
		return result.Success, nil

	default:
		return false, fmt.Errorf("unknown verification method: %s", method)
	}
}

// handleVerificationSuccess processes a successful verification
func (bvj *BackgroundVerificationJob) handleVerificationSuccess(ctx context.Context, domain string, method model.VerificationMethod) bool {
	err := bvj.updateVerificationSuccess(ctx, domain, method)
	if err != nil {
		log.Printf("Failed to update verification success for %s: %v", domain, err)
		return false
	}

	log.Printf("Background verification successful for domain %s using %s", domain, method)
	return true
}

// handleVerificationFailure processes a failed verification
func (bvj *BackgroundVerificationJob) handleVerificationFailure(ctx context.Context, domain string, lastError error) bool {
	err := bvj.updateVerificationFailure(ctx, domain, lastError)
	if err != nil {
		log.Printf("Failed to update verification failure for %s: %v", domain, err)
	} else {
		log.Printf("Background verification failed for domain %s", domain)
	}

	return false
}

// updateVerificationSuccess updates the domain verification record on successful verification
func (bvj *BackgroundVerificationJob) updateVerificationSuccess(ctx context.Context, domain string, method model.VerificationMethod) error {
	now := time.Now()

	domainVerification := &model.DomainVerification{
		Domain:               domain,
		Status:               model.VerificationStatusVerified,
		LastVerified:         now,
		LastSuccessfulMethod: method,
		ConsecutiveFailures:  0,                       // Reset failure count
		NextVerification:     now.Add(24 * time.Hour), // Next verification in 24 hours
	}

	return bvj.db.UpdateDomainVerification(ctx, domainVerification)
}

// updateVerificationFailure updates the domain verification record on failed verification
func (bvj *BackgroundVerificationJob) updateVerificationFailure(ctx context.Context, domain string, lastError error) error {
	// Get current domain verification record
	domainVerification, err := bvj.db.GetDomainVerification(ctx, domain)
	if err != nil {
		// If record doesn't exist, create a new one
		if errors.Is(err, database.ErrNotFound) {
			now := time.Now()
			domainVerification = &model.DomainVerification{
				Domain:    domain,
				Status:    model.VerificationStatusPending,
				CreatedAt: now,
			}
		} else {
			return fmt.Errorf("failed to get domain verification record: %w", err)
		}
	}

	now := time.Now()
	domainVerification.ConsecutiveFailures++
	domainVerification.LastVerificationAttempt = now
	if lastError != nil {
		domainVerification.LastError = lastError.Error()
	}

	// Check if we've exceeded the failure threshold
	if domainVerification.ConsecutiveFailures >= bvj.config.FailureThreshold {
		domainVerification.Status = model.VerificationStatusFailed

		// Send notification if cooldown period has passed
		if domainVerification.LastNotificationSent.IsZero() ||
			now.Sub(domainVerification.LastNotificationSent) >= bvj.config.NotificationCooldown {
			// Send notification when threshold is exceeded, regardless of whether we have a specific error
			bvj.notifyFunc(ctx, domain, domainVerification.ConsecutiveFailures, lastError)
			domainVerification.LastNotificationSent = now
		}
	}

	// Calculate next verification time (exponential backoff)
	nextVerification := now.Add(time.Duration(domainVerification.ConsecutiveFailures) * time.Hour)
	if nextVerification.Sub(now) > 24*time.Hour {
		nextVerification = now.Add(24 * time.Hour) // Cap at 24 hours
	}
	domainVerification.NextVerification = nextVerification

	return bvj.db.UpdateDomainVerification(ctx, domainVerification)
}

// runCleanup removes old verification records and performs maintenance
func (bvj *BackgroundVerificationJob) runCleanup(ctx context.Context) error {
	log.Println("Starting background verification cleanup")

	cutoffTime := time.Now().Add(-bvj.config.CleanupInterval)

	// Clean up old failed verification records
	count, err := bvj.db.CleanupOldVerifications(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup old verifications: %w", err)
	}

	log.Printf("Background cleanup completed: removed %d old verification records", count)
	return nil
}

// defaultNotificationFunc is a simple notification function that logs failures
func defaultNotificationFunc(ctx context.Context, domain string, failures int, lastError error) {
	if lastError != nil {
		log.Printf("ALERT: Domain %s has failed verification %d times consecutively. Last error: %v",
			domain, failures, lastError)
	} else {
		log.Printf("ALERT: Domain %s has failed verification %d times consecutively. No specific error available.",
			domain, failures)
	}
}

// GetStatus returns the current status of the background verification job
func (bvj *BackgroundVerificationJob) GetStatus() map[string]any {
	bvj.mu.RLock()
	defer bvj.mu.RUnlock()

	status := map[string]any{
		"running":       bvj.running,
		"cron_schedule": bvj.config.CronSchedule,
		"config":        bvj.config,
	}

	if bvj.running {
		status["cron_entries"] = len(bvj.cron.Entries())
	}

	return status
}
