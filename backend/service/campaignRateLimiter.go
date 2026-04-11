package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CampaignRateLimiter provides per-campaign send rate limiting.
// It uses a token bucket algorithm where tokens are replenished at
// the configured rate, and each send consumes one token.
type CampaignRateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*rateBucket
	logger   *zap.SugaredLogger
	defaults RateLimitConfig
}

// RateLimitConfig defines rate limiting parameters for a campaign
type RateLimitConfig struct {
	// MaxPerMinute is the maximum number of emails per minute (0 = unlimited)
	MaxPerMinute int `json:"maxPerMinute"`
	// MaxPerHour is the maximum number of emails per hour (0 = unlimited)
	MaxPerHour int `json:"maxPerHour"`
	// MaxPerDay is the maximum number of emails per day (0 = unlimited)
	MaxPerDay int `json:"maxPerDay"`
	// BurstSize is the maximum burst size (defaults to MaxPerMinute)
	BurstSize int `json:"burstSize"`
	// Enabled controls whether rate limiting is active
	Enabled bool `json:"enabled"`
}

// DefaultRateLimitConfig returns sensible defaults
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxPerMinute: 30,
		MaxPerHour:   500,
		MaxPerDay:    5000,
		BurstSize:    10,
		Enabled:      true,
	}
}

type rateBucket struct {
	config      RateLimitConfig
	minuteCount int
	hourCount   int
	dayCount    int
	minuteReset time.Time
	hourReset   time.Time
	dayReset    time.Time
	lastSend    time.Time
}

// NewCampaignRateLimiter creates a new rate limiter
func NewCampaignRateLimiter(logger *zap.SugaredLogger) *CampaignRateLimiter {
	return &CampaignRateLimiter{
		buckets:  make(map[string]*rateBucket),
		logger:   logger,
		defaults: DefaultRateLimitConfig(),
	}
}

// SetConfig sets the rate limit config for a specific campaign
func (rl *CampaignRateLimiter) SetConfig(campaignID uuid.UUID, config RateLimitConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	key := campaignID.String()
	if bucket, exists := rl.buckets[key]; exists {
		bucket.config = config
	} else {
		now := time.Now()
		rl.buckets[key] = &rateBucket{
			config:      config,
			minuteReset: now.Add(time.Minute),
			hourReset:   now.Add(time.Hour),
			dayReset:    now.Add(24 * time.Hour),
		}
	}
}

// GetConfig returns the current rate limit config for a campaign
func (rl *CampaignRateLimiter) GetConfig(campaignID uuid.UUID) RateLimitConfig {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	key := campaignID.String()
	if bucket, exists := rl.buckets[key]; exists {
		return bucket.config
	}
	return rl.defaults
}

// WaitForSlot blocks until a send slot is available for the given campaign.
// Returns the recommended delay before sending.
// If ctx is cancelled, returns immediately with the context error.
func (rl *CampaignRateLimiter) WaitForSlot(ctx context.Context, campaignID uuid.UUID) error {
	key := campaignID.String()

	for {
		delay := rl.getDelay(key)
		if delay == 0 {
			return nil
		}

		rl.logger.Debugw("rate limiter: waiting for slot",
			"campaignID", campaignID.String(),
			"delay", delay.String(),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Try again after the delay
		}
	}
}

// RecordSend records that an email was sent for the given campaign
func (rl *CampaignRateLimiter) RecordSend(campaignID uuid.UUID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	key := campaignID.String()
	bucket, exists := rl.buckets[key]
	if !exists {
		now := time.Now()
		bucket = &rateBucket{
			config:      rl.defaults,
			minuteReset: now.Add(time.Minute),
			hourReset:   now.Add(time.Hour),
			dayReset:    now.Add(24 * time.Hour),
		}
		rl.buckets[key] = bucket
	}
	now := time.Now()
	bucket.lastSend = now
	bucket.minuteCount++
	bucket.hourCount++
	bucket.dayCount++
}

// RemoveCampaign removes rate limiting state for a campaign (when it completes)
func (rl *CampaignRateLimiter) RemoveCampaign(campaignID uuid.UUID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, campaignID.String())
}

// GetStats returns current rate limiting stats for a campaign
func (rl *CampaignRateLimiter) GetStats(campaignID uuid.UUID) map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	key := campaignID.String()
	bucket, exists := rl.buckets[key]
	if !exists {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	rl.resetExpiredCounters(bucket)
	return map[string]interface{}{
		"enabled":      bucket.config.Enabled,
		"maxPerMinute": bucket.config.MaxPerMinute,
		"maxPerHour":   bucket.config.MaxPerHour,
		"maxPerDay":    bucket.config.MaxPerDay,
		"minuteCount":  bucket.minuteCount,
		"hourCount":    bucket.hourCount,
		"dayCount":     bucket.dayCount,
		"lastSend":     bucket.lastSend,
	}
}

// getDelay calculates how long to wait before the next send is allowed
func (rl *CampaignRateLimiter) getDelay(key string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	if !exists {
		return 0
	}
	if !bucket.config.Enabled {
		return 0
	}

	now := time.Now()
	rl.resetExpiredCounters(bucket)

	// Check daily limit
	if bucket.config.MaxPerDay > 0 && bucket.dayCount >= bucket.config.MaxPerDay {
		return bucket.dayReset.Sub(now)
	}

	// Check hourly limit
	if bucket.config.MaxPerHour > 0 && bucket.hourCount >= bucket.config.MaxPerHour {
		return bucket.hourReset.Sub(now)
	}

	// Check per-minute limit
	if bucket.config.MaxPerMinute > 0 && bucket.minuteCount >= bucket.config.MaxPerMinute {
		return bucket.minuteReset.Sub(now)
	}

	return 0
}

// resetExpiredCounters resets counters whose time windows have expired
func (rl *CampaignRateLimiter) resetExpiredCounters(bucket *rateBucket) {
	now := time.Now()
	if now.After(bucket.minuteReset) {
		bucket.minuteCount = 0
		bucket.minuteReset = now.Add(time.Minute)
	}
	if now.After(bucket.hourReset) {
		bucket.hourCount = 0
		bucket.hourReset = now.Add(time.Hour)
	}
	if now.After(bucket.dayReset) {
		bucket.dayCount = 0
		bucket.dayReset = now.Add(24 * time.Hour)
	}
}

// Cleanup removes stale campaign entries (campaigns that haven't sent in 24h)
func (rl *CampaignRateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-24 * time.Hour)
	for key, bucket := range rl.buckets {
		if bucket.lastSend.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}
