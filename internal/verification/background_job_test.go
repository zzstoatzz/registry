package verification

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
)

const (
	defaultCronScheduleBG   = "0 0 2 * * *"
	testDomainBG            = "example.com"
	errMsgGetDomainVerifyBG = "Failed to get domain verification: %v"
)

// mockDatabase is a mock implementation of the Database interface for testing
type mockDatabase struct {
	mu                  sync.RWMutex
	verifiedDomains     []string
	domainVerifications map[string]*model.DomainVerification
	cleanupCount        int
}

func newMockDatabase() *mockDatabase {
	return &mockDatabase{
		domainVerifications: make(map[string]*model.DomainVerification),
	}
}

func (m *mockDatabase) List(ctx context.Context, filter map[string]any, cursor string, limit int) ([]*model.Server, string, error) {
	return nil, "", nil
}

func (m *mockDatabase) GetByID(ctx context.Context, id string) (*model.ServerDetail, error) {
	return nil, database.ErrNotFound
}

func (m *mockDatabase) Publish(ctx context.Context, serverDetail *model.ServerDetail) error {
	return nil
}

func (m *mockDatabase) StoreVerificationToken(ctx context.Context, serverID string, token *model.VerificationToken) error {
	return nil
}

func (m *mockDatabase) GetVerificationToken(ctx context.Context, serverID string) (*model.VerificationToken, error) {
	return nil, database.ErrNotFound
}

func (m *mockDatabase) ImportSeed(ctx context.Context, seedFilePath string) error {
	return nil
}

func (m *mockDatabase) Close() error {
	return nil
}

func (m *mockDatabase) GetVerifiedDomains(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.verifiedDomains...), nil
}

func (m *mockDatabase) GetDomainVerification(ctx context.Context, domain string) (*model.DomainVerification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dv, exists := m.domainVerifications[domain]
	if !exists {
		return nil, database.ErrNotFound
	}

	// Return a copy to avoid race conditions
	result := *dv
	return &result, nil
}

func (m *mockDatabase) UpdateDomainVerification(ctx context.Context, domainVerification *model.DomainVerification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy to avoid sharing memory
	dv := *domainVerification
	m.domainVerifications[domainVerification.Domain] = &dv
	return nil
}

func (m *mockDatabase) CleanupOldVerifications(ctx context.Context, before time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupCount++
	return 1, nil
}

func (m *mockDatabase) addVerifiedDomain(domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.verifiedDomains = append(m.verifiedDomains, domain)
	now := time.Now()
	m.domainVerifications[domain] = &model.DomainVerification{
		Domain:              domain,
		Status:              model.VerificationStatusVerified,
		CreatedAt:           now,
		LastVerified:        now,
		ConsecutiveFailures: 0,
	}
}

// mockNotificationFunc captures notifications for testing
type mockNotificationFunc struct {
	mu            sync.RWMutex
	notifications []notificationRecord
}

type notificationRecord struct {
	domain   string
	failures int
	err      error
}

func (m *mockNotificationFunc) notify(ctx context.Context, domain string, failures int, lastError error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notifications = append(m.notifications, notificationRecord{
		domain:   domain,
		failures: failures,
		err:      lastError,
	})
}

func (m *mockNotificationFunc) getNotifications() []notificationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]notificationRecord{}, m.notifications...)
}

func TestNewBackgroundVerificationJob(t *testing.T) {
	db := newMockDatabase()
	config := DefaultBackgroundJobConfig()
	mockNotify := &mockNotificationFunc{}

	job := NewBackgroundVerificationJob(db, config, mockNotify.notify)

	if job == nil {
		t.Fatal("NewBackgroundVerificationJob returned nil")
	}

	if job.db != db {
		t.Error("Database not set correctly")
	}

	if job.config != config {
		t.Error("Config not set correctly")
	}

	if job.running {
		t.Error("Job should not be running initially")
	}
}

func TestNewBackgroundVerificationJobWithDefaults(t *testing.T) {
	db := newMockDatabase()

	job := NewBackgroundVerificationJob(db, nil, nil)

	if job == nil {
		t.Fatal("NewBackgroundVerificationJob returned nil")
	}

	if job.config == nil {
		t.Error("Default config not set")
	}

	if job.config.CronSchedule != defaultCronScheduleBG {
		t.Errorf("Default cron schedule = %s, want %s", job.config.CronSchedule, defaultCronScheduleBG)
	}
}

func TestDefaultBackgroundJobConfig(t *testing.T) {
	config := DefaultBackgroundJobConfig()

	if config == nil {
		t.Fatal("DefaultBackgroundJobConfig returned nil")
	}

	if config.CronSchedule != defaultCronScheduleBG {
		t.Errorf("CronSchedule = %s, want %s", config.CronSchedule, defaultCronScheduleBG)
	}

	if config.MaxConcurrentVerifications != 10 {
		t.Errorf("MaxConcurrentVerifications = %d, want %d", config.MaxConcurrentVerifications, 10)
	}

	if config.VerificationTimeout != 30*time.Second {
		t.Errorf("VerificationTimeout = %v, want %v", config.VerificationTimeout, 30*time.Second)
	}

	if config.FailureThreshold != 3 {
		t.Errorf("FailureThreshold = %d, want %d", config.FailureThreshold, 3)
	}
}

func TestBackgroundJobStartStop(t *testing.T) {
	db := newMockDatabase()
	config := &BackgroundJobConfig{
		CronSchedule:               "0 0 * * * *", // Every minute for testing (with seconds)
		MaxConcurrentVerifications: 1,
		VerificationTimeout:        1 * time.Second,
		FailureThreshold:           1,
		RetryDelay:                 100 * time.Millisecond,
		NotificationCooldown:       1 * time.Second,
		CleanupInterval:            1 * time.Hour,
	}
	mockNotify := &mockNotificationFunc{}

	job := NewBackgroundVerificationJob(db, config, mockNotify.notify)
	ctx := context.Background()

	// Test starting the job
	err := job.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start job: %v", err)
	}

	if !job.IsRunning() {
		t.Error("Job should be running after start")
	}

	// Test that starting again fails
	err = job.Start(ctx)
	if err == nil {
		t.Error("Starting already running job should fail")
	}

	// Test stopping the job
	err = job.Stop()
	if err != nil {
		t.Fatalf("Failed to stop job: %v", err)
	}

	if job.IsRunning() {
		t.Error("Job should not be running after stop")
	}

	// Test that stopping again fails
	err = job.Stop()
	if err == nil {
		t.Error("Stopping already stopped job should fail")
	}
}

func TestRunNow(t *testing.T) {
	db := newMockDatabase()
	db.addVerifiedDomain(testDomainBG)

	mockNotify := &mockNotificationFunc{}
	job := NewBackgroundVerificationJob(db, DefaultBackgroundJobConfig(), mockNotify.notify)

	ctx := context.Background()
	err := job.RunNow(ctx)
	if err != nil {
		t.Errorf("RunNow failed: %v", err)
	}
}

func TestRunNowWithNoDomains(t *testing.T) {
	db := newMockDatabase()
	mockNotify := &mockNotificationFunc{}
	job := NewBackgroundVerificationJob(db, DefaultBackgroundJobConfig(), mockNotify.notify)

	ctx := context.Background()
	err := job.RunNow(ctx)
	if err != nil {
		t.Errorf("RunNow with no domains failed: %v", err)
	}
}

func TestGetStatus(t *testing.T) {
	db := newMockDatabase()
	config := DefaultBackgroundJobConfig()
	mockNotify := &mockNotificationFunc{}

	job := NewBackgroundVerificationJob(db, config, mockNotify.notify)

	status := job.GetStatus()
	if status == nil {
		t.Fatal("GetStatus returned nil")
	}

	if status["running"] != false {
		t.Error("Status should show not running initially")
	}

	if status["cron_schedule"] != config.CronSchedule {
		t.Errorf("Status cron_schedule = %v, want %v", status["cron_schedule"], config.CronSchedule)
	}

	// Start job and check status again
	ctx := context.Background()
	err := job.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start job: %v", err)
	}
	defer job.Stop()

	status = job.GetStatus()
	if status["running"] != true {
		t.Error("Status should show running after start")
	}

	if status["cron_entries"] == nil {
		t.Error("Status should include cron_entries when running")
	}
}

func TestUpdateVerificationSuccess(t *testing.T) {
	db := newMockDatabase()
	mockNotify := &mockNotificationFunc{}
	job := NewBackgroundVerificationJob(db, DefaultBackgroundJobConfig(), mockNotify.notify)

	ctx := context.Background()
	domain := testDomainBG
	method := model.VerificationMethodDNS

	err := job.updateVerificationSuccess(ctx, domain, method)
	if err != nil {
		t.Errorf("updateVerificationSuccess failed: %v", err)
	}

	// Verify the domain verification was updated
	dv, err := db.GetDomainVerification(ctx, domain)
	if err != nil {
		t.Fatalf(errMsgGetDomainVerifyBG, err)
	}

	if dv.Status != model.VerificationStatusVerified {
		t.Errorf("Status = %s, want %s", dv.Status, model.VerificationStatusVerified)
	}

	if dv.LastSuccessfulMethod != method {
		t.Errorf("LastSuccessfulMethod = %s, want %s", dv.LastSuccessfulMethod, method)
	}

	if dv.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want %d", dv.ConsecutiveFailures, 0)
	}
}

func TestUpdateVerificationFailure(t *testing.T) {
	db := newMockDatabase()
	config := &BackgroundJobConfig{
		FailureThreshold:     3,
		NotificationCooldown: 1 * time.Hour,
	}
	mockNotify := &mockNotificationFunc{}
	job := NewBackgroundVerificationJob(db, config, mockNotify.notify)

	ctx := context.Background()
	domain := testDomainBG
	testError := errors.New("verification failed")

	// Set up initial domain verification
	now := time.Now()
	initialDV := &model.DomainVerification{
		Domain:              domain,
		Status:              model.VerificationStatusVerified,
		CreatedAt:           now,
		ConsecutiveFailures: 0,
	}
	db.UpdateDomainVerification(ctx, initialDV)

	// First failure
	err := job.updateVerificationFailure(ctx, domain, testError)
	if err != nil {
		t.Errorf("updateVerificationFailure failed: %v", err)
	}

	dv, err := db.GetDomainVerification(ctx, domain)
	if err != nil {
		t.Fatalf(errMsgGetDomainVerifyBG, err)
	}

	if dv.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want %d", dv.ConsecutiveFailures, 1)
	}

	if dv.LastError != testError.Error() {
		t.Errorf("LastError = %s, want %s", dv.LastError, testError.Error())
	}

	// Third failure (should trigger notification)
	dv.ConsecutiveFailures = 2
	db.UpdateDomainVerification(ctx, dv)

	err = job.updateVerificationFailure(ctx, domain, testError)
	if err != nil {
		t.Errorf("updateVerificationFailure failed: %v", err)
	}

	dv, err = db.GetDomainVerification(ctx, domain)
	if err != nil {
		t.Fatalf(errMsgGetDomainVerifyBG, err)
	}

	if dv.Status != model.VerificationStatusFailed {
		t.Errorf("Status = %s, want %s", dv.Status, model.VerificationStatusFailed)
	}

	// Check notification was sent
	notifications := mockNotify.getNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	if len(notifications) > 0 {
		if notifications[0].domain != domain {
			t.Errorf("Notification domain = %s, want %s", notifications[0].domain, domain)
		}
		if notifications[0].failures != 3 {
			t.Errorf("Notification failures = %d, want %d", notifications[0].failures, 3)
		}
	}
}

func TestRunSingleVerification(t *testing.T) {
	db := newMockDatabase()
	job := NewBackgroundVerificationJob(db, DefaultBackgroundJobConfig(), nil)

	domain := testDomainBG
	token := "test-token"

	// Test unknown method
	success, err := job.runSingleVerification(domain, token, "unknown")
	if err == nil {
		t.Error("Expected error for unknown verification method")
	}
	if success {
		t.Error("Expected success to be false for unknown method")
	}
}
