package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

// CookieHealthMonitor periodically validates cookie store sessions
// and sends notifications when sessions expire or become invalid.
type CookieHealthMonitor struct {
	mu              sync.RWMutex
	logger          *zap.SugaredLogger
	cookieStore     *CookieStoreService
	cookieStoreRepo *repository.CookieStore
	telegram        *Telegram
	checkInterval   time.Duration
	sessionHealth   map[string]*CookieSessionHealth
	running         bool
	stopCh          chan struct{}
}

// CookieSessionHealth tracks the health of a cookie store session
type CookieSessionHealth struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Source           string     `json:"source"`
	Status           string     `json:"status"`
	LastChecked      time.Time  `json:"lastChecked"`
	LastValid        time.Time  `json:"lastValid"`
	ConsecutiveFails int        `json:"consecutiveFails"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	ErrorMessage     string     `json:"errorMessage,omitempty"`
	NotifiedExpiry   bool       `json:"notifiedExpiry"`
}

// NewCookieHealthMonitor creates a new health monitor
func NewCookieHealthMonitor(
	logger *zap.SugaredLogger,
	cookieStore *CookieStoreService,
	cookieStoreRepo *repository.CookieStore,
	telegram *Telegram,
) *CookieHealthMonitor {
	return &CookieHealthMonitor{
		logger:          logger,
		cookieStore:     cookieStore,
		cookieStoreRepo: cookieStoreRepo,
		telegram:        telegram,
		checkInterval:   30 * time.Minute,
		sessionHealth:   make(map[string]*CookieSessionHealth),
		stopCh:          make(chan struct{}),
	}
}

// Start begins the periodic health check loop
func (m *CookieHealthMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Infow("cookie health monitor started",
		"interval", m.checkInterval.String(),
	)

	go func() {
		// Initial check after 2 minutes (give server time to start)
		timer := time.NewTimer(2 * time.Minute)
		defer timer.Stop()

		for {
			select {
			case <-m.stopCh:
				m.logger.Info("cookie health monitor stopped")
				return
			case <-timer.C:
				m.runHealthCheck()
				timer.Reset(m.checkInterval)
			}
		}
	}()
}

// Stop stops the health check loop
func (m *CookieHealthMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		close(m.stopCh)
		m.running = false
	}
}

// GetAllHealth returns the health status of all monitored sessions
func (m *CookieHealthMonitor) GetAllHealth() []CookieSessionHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]CookieSessionHealth, 0, len(m.sessionHealth))
	for _, h := range m.sessionHealth {
		result = append(result, *h)
	}
	return result
}

// GetHealth returns the health status of a specific session
func (m *CookieHealthMonitor) GetHealth(id uuid.UUID) *CookieSessionHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, exists := m.sessionHealth[id.String()]; exists {
		return h
	}
	return nil
}

// GetSummary returns a summary of session health
func (m *CookieHealthMonitor) GetSummary() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	summary := map[string]int{
		"total":   len(m.sessionHealth),
		"active":  0,
		"expired": 0,
		"invalid": 0,
		"unknown": 0,
	}
	for _, h := range m.sessionHealth {
		switch h.Status {
		case "active":
			summary["active"]++
		case "expired":
			summary["expired"]++
		case "invalid":
			summary["invalid"]++
		default:
			summary["unknown"]++
		}
	}
	return summary
}

// runHealthCheck validates all cookie store sessions
func (m *CookieHealthMonitor) runHealthCheck() {
	m.logger.Debug("running cookie health check")
	ctx := context.Background()

	// Get all cookie stores via repository (no auth needed for background task)
	stores, err := m.cookieStoreRepo.GetAll(ctx, nil, &repository.CookieStoreOption{})
	if err != nil {
		m.logger.Errorw("cookie health check: failed to get stores", "error", err)
		return
	}

	if stores == nil || stores.Rows == nil {
		return
	}

	checkedCount := 0
	expiredCount := 0
	activeCount := 0

	for _, store := range stores.Rows {
		key := store.ID.String()

		// Get or create health entry
		m.mu.Lock()
		health, exists := m.sessionHealth[key]
		if !exists {
			health = &CookieSessionHealth{
				ID:     store.ID,
				Email:  store.Email,
				Source: store.Source,
				Status: "unknown",
			}
			m.sessionHealth[key] = health
		}
		m.mu.Unlock()

		// Validate the session using the existing validateAndUpdate flow
		isValid, errMsg := m.validateSession(ctx, store)

		m.mu.Lock()
		health.LastChecked = time.Now()
		if isValid {
			health.Status = "active"
			health.LastValid = time.Now()
			health.ConsecutiveFails = 0
			health.ErrorMessage = ""
			health.NotifiedExpiry = false
			activeCount++
		} else {
			health.ConsecutiveFails++
			health.ErrorMessage = errMsg

			if health.ConsecutiveFails >= 2 {
				// Mark as expired after 2 consecutive failures
				if health.Status != "expired" {
					health.Status = "expired"
					// Send notification if not already notified
					if !health.NotifiedExpiry {
						health.NotifiedExpiry = true
						go m.sendExpiryNotification(health)
					}
				}
				expiredCount++
			} else {
				// First failure - might be transient
				health.Status = "invalid"
			}
		}
		m.mu.Unlock()

		checkedCount++
	}

	m.logger.Infow("cookie health check completed",
		"checked", checkedCount,
		"active", activeCount,
		"expired", expiredCount,
	)
}

// validateSession checks if a cookie store session is still valid.
// It delegates to the CookieStoreService's validateAndUpdate method which
// tries token exchange, browser automation, and cookie-based validation.
func (m *CookieHealthMonitor) validateSession(ctx context.Context, store *database.CookieStore) (bool, string) {
	// Use the existing validateAndUpdate flow which updates the DB record
	err := m.cookieStore.validateAndUpdate(ctx, store.ID)
	if err != nil {
		return false, fmt.Sprintf("validation error: %s", err.Error())
	}

	// Re-read the store to check if it was marked valid
	updated, err := m.cookieStoreRepo.GetByID(ctx, store.ID)
	if err != nil {
		return false, fmt.Sprintf("failed to read updated store: %s", err.Error())
	}

	if !updated.IsValid {
		return false, "session validation failed (cookies expired or invalid)"
	}

	return true, ""
}

// sendExpiryNotification sends a Telegram notification about session expiry
func (m *CookieHealthMonitor) sendExpiryNotification(health *CookieSessionHealth) {
	if m.telegram == nil {
		return
	}

	message := fmt.Sprintf(
		"⚠️ <b>Cookie Session Expired</b>\n\n"+
			"📧 Email: <code>%s</code>\n"+
			"🔌 Source: %s\n"+
			"❌ Error: %s\n"+
			"🕐 Last Valid: %s\n"+
			"📊 Failed Checks: %d\n\n"+
			"<i>Please refresh or re-import this session.</i>",
		health.Email,
		health.Source,
		health.ErrorMessage,
		health.LastValid.Format("2006-01-02 15:04:05"),
		health.ConsecutiveFails,
	)

	m.logger.Infow("sending cookie expiry notification",
		"email", health.Email,
		"source", health.Source,
	)

	// Use the Telegram service to send the notification
	ctx := context.Background()
	settings, err := m.telegram.GetSettings(ctx)
	if err != nil || !settings.Enabled || settings.BotToken == "" || settings.ChatID == "" {
		m.logger.Debugw("telegram not configured, skipping cookie expiry notification")
		return
	}

	if err := m.telegram.sendMessage(settings, message); err != nil {
		m.logger.Errorw("failed to send cookie expiry telegram notification", "error", err)
	}
}

// Cleanup removes health entries for sessions that no longer exist
func (m *CookieHealthMonitor) Cleanup(ctx context.Context) {
	stores, err := m.cookieStoreRepo.GetAll(ctx, nil, &repository.CookieStoreOption{})
	if err != nil {
		return
	}

	existingIDs := make(map[string]bool)
	if stores != nil && stores.Rows != nil {
		for _, store := range stores.Rows {
			existingIDs[store.ID.String()] = true
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.sessionHealth {
		if !existingIDs[key] {
			delete(m.sessionHealth, key)
		}
	}
}
