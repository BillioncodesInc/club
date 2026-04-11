package service

// ============================================================
// WEBHOOK RETRY ENHANCEMENT
// ============================================================
// Adds exponential backoff retry (3 attempts: 1s, 5s, 30s)
// and an in-memory delivery log for debugging.
// ============================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/phishingclub/phishingclub/model"
)

// WebhookDeliveryStatus represents the status of a webhook delivery attempt
type WebhookDeliveryStatus string

const (
	WebhookDeliveryPending  WebhookDeliveryStatus = "pending"
	WebhookDeliverySuccess  WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed   WebhookDeliveryStatus = "failed"
	WebhookDeliveryRetrying WebhookDeliveryStatus = "retrying"
)

// WebhookDeliveryLog represents a single webhook delivery attempt
type WebhookDeliveryLog struct {
	ID            string                `json:"id"`
	WebhookName   string                `json:"webhookName"`
	WebhookURL    string                `json:"webhookUrl"`
	Event         string                `json:"event"`
	Status        WebhookDeliveryStatus `json:"status"`
	StatusCode    int                   `json:"statusCode"`
	ResponseBody  string                `json:"responseBody"`
	Attempt       int                   `json:"attempt"`
	MaxAttempts   int                   `json:"maxAttempts"`
	Error         string                `json:"error,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
	LastAttemptAt time.Time             `json:"lastAttemptAt"`
	NextRetryAt   *time.Time            `json:"nextRetryAt,omitempty"`
}

// WebhookDeliveryTracker tracks webhook delivery attempts in memory
type WebhookDeliveryTracker struct {
	mu      sync.RWMutex
	logs    []WebhookDeliveryLog
	maxLogs int
}

// NewWebhookDeliveryTracker creates a new tracker with a max log size
func NewWebhookDeliveryTracker(maxLogs int) *WebhookDeliveryTracker {
	return &WebhookDeliveryTracker{
		logs:    make([]WebhookDeliveryLog, 0, maxLogs),
		maxLogs: maxLogs,
	}
}

// Add adds a delivery log entry
func (t *WebhookDeliveryTracker) Add(log WebhookDeliveryLog) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.logs) >= t.maxLogs {
		// Remove oldest entries (keep last 80%)
		cutoff := t.maxLogs / 5
		t.logs = t.logs[cutoff:]
	}
	t.logs = append(t.logs, log)
}

// Update updates an existing log entry by ID
func (t *WebhookDeliveryTracker) Update(id string, updater func(*WebhookDeliveryLog)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := len(t.logs) - 1; i >= 0; i-- {
		if t.logs[i].ID == id {
			updater(&t.logs[i])
			return
		}
	}
}

// GetRecent returns the most recent N delivery logs
func (t *WebhookDeliveryTracker) GetRecent(limit int) []WebhookDeliveryLog {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if limit > len(t.logs) {
		limit = len(t.logs)
	}
	result := make([]WebhookDeliveryLog, limit)
	copy(result, t.logs[len(t.logs)-limit:])
	// Reverse so newest is first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// GetRecentJSON returns the most recent N delivery logs as JSON bytes
func (t *WebhookDeliveryTracker) GetRecentJSON(limit int) ([]byte, error) {
	logs := t.GetRecent(limit)
	return json.Marshal(logs)
}

// GetStats returns delivery statistics
func (t *WebhookDeliveryTracker) GetStats() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stats := map[string]int{
		"total":    len(t.logs),
		"success":  0,
		"failed":   0,
		"retrying": 0,
	}
	for _, log := range t.logs {
		switch log.Status {
		case WebhookDeliverySuccess:
			stats["success"]++
		case WebhookDeliveryFailed:
			stats["failed"]++
		case WebhookDeliveryRetrying:
			stats["retrying"]++
		}
	}
	return stats
}

// retryDelays defines the backoff delays for each retry attempt
var retryDelays = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	30 * time.Second,
}

const maxRetryAttempts = 3

// SendWithRetry sends a webhook with exponential backoff retry.
// It fires the retries in a background goroutine so the caller is not blocked.
func (w *Webhook) SendWithRetry(
	webhook *model.Webhook,
	request *WebhookRequest,
	tracker *WebhookDeliveryTracker,
) {
	webhookName := ""
	if name, err := webhook.Name.Get(); err == nil {
		webhookName = name.String()
	}
	webhookURL := ""
	if url, err := webhook.URL.Get(); err == nil {
		webhookURL = url.String()
	}

	logID := fmt.Sprintf("%s-%s-%d", webhookName, request.Event, time.Now().UnixNano())
	entry := WebhookDeliveryLog{
		ID:            logID,
		WebhookName:   webhookName,
		WebhookURL:    webhookURL,
		Event:         request.Event,
		Status:        WebhookDeliveryPending,
		Attempt:       0,
		MaxAttempts:   maxRetryAttempts,
		CreatedAt:     time.Now(),
		LastAttemptAt: time.Now(),
	}
	if tracker != nil {
		tracker.Add(entry)
	}

	go func() {
		for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
			if tracker != nil {
				tracker.Update(logID, func(l *WebhookDeliveryLog) {
					l.Attempt = attempt
					l.LastAttemptAt = time.Now()
					if attempt > 1 {
						l.Status = WebhookDeliveryRetrying
					}
				})
			}

			data, err := w.Send(context.Background(), webhook, request)
			if err == nil {
				statusCode := 0
				responseBody := ""
				if data != nil {
					if code, ok := data["code"].(int); ok {
						statusCode = code
					}
					if body, ok := data["body"].(string); ok {
						responseBody = body
					}
				}
				// Success if status code is 2xx
				if statusCode >= 200 && statusCode < 300 {
					if tracker != nil {
						tracker.Update(logID, func(l *WebhookDeliveryLog) {
							l.Status = WebhookDeliverySuccess
							l.StatusCode = statusCode
							l.ResponseBody = truncateString(responseBody, 500)
						})
					}
					w.Logger.Debugw("webhook delivered successfully",
						"webhook", webhookName,
						"event", request.Event,
						"attempt", attempt,
						"statusCode", statusCode,
					)
					return
				}
				// Non-2xx response - treat as failure, retry
				errMsg := fmt.Sprintf("HTTP %d: %s", statusCode, truncateString(responseBody, 200))
				w.Logger.Warnw("webhook delivery got non-2xx response",
					"webhook", webhookName,
					"event", request.Event,
					"attempt", attempt,
					"statusCode", statusCode,
				)
				if attempt < maxRetryAttempts {
					delay := retryDelays[attempt-1]
					nextRetry := time.Now().Add(delay)
					if tracker != nil {
						tracker.Update(logID, func(l *WebhookDeliveryLog) {
							l.Status = WebhookDeliveryRetrying
							l.StatusCode = statusCode
							l.Error = errMsg
							l.NextRetryAt = &nextRetry
						})
					}
					time.Sleep(delay)
					continue
				}
				// Final attempt failed
				if tracker != nil {
					tracker.Update(logID, func(l *WebhookDeliveryLog) {
						l.Status = WebhookDeliveryFailed
						l.StatusCode = statusCode
						l.Error = errMsg
						l.ResponseBody = truncateString(responseBody, 500)
					})
				}
				return
			}

			// Network error
			w.Logger.Warnw("webhook delivery failed",
				"webhook", webhookName,
				"event", request.Event,
				"attempt", attempt,
				"error", err,
			)
			if attempt < maxRetryAttempts {
				delay := retryDelays[attempt-1]
				nextRetry := time.Now().Add(delay)
				if tracker != nil {
					tracker.Update(logID, func(l *WebhookDeliveryLog) {
						l.Status = WebhookDeliveryRetrying
						l.Error = err.Error()
						l.NextRetryAt = &nextRetry
					})
				}
				time.Sleep(delay)
				continue
			}
			// Final attempt failed
			if tracker != nil {
				tracker.Update(logID, func(l *WebhookDeliveryLog) {
					l.Status = WebhookDeliveryFailed
					l.Error = err.Error()
				})
			}
		}
	}()
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
