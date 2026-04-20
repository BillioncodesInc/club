package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// v1.0.43 – Cookie Store Enhancements: Bulk Ops, Rotation, Reply/Forward

// --- Bulk Operations ---

// BulkDeleteRequest holds IDs for bulk deletion
type BulkDeleteRequest struct {
	IDs []string `json:"ids"`
}

// BulkDeleteResult reports the outcome of a bulk delete
type BulkDeleteResult struct {
	Deleted int      `json:"deleted"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// BulkDelete deletes multiple cookie stores by ID
func (s *CookieStoreService) BulkDelete(
	ctx context.Context,
	session *model.Session,
	ids []string,
) (*BulkDeleteResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	result := &BulkDeleteResult{}
	for _, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("invalid ID: %s", idStr))
			continue
		}
		if err := s.CookieStoreRepo.DeleteByID(ctx, id); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("failed to delete %s: %s", idStr, err.Error()))
			continue
		}
		result.Deleted++
	}
	return result, nil
}

// BulkRevalidateResult reports the outcome of a bulk revalidation
type BulkRevalidateResult struct {
	Total   int                   `json:"total"`
	Valid   int                   `json:"valid"`
	Invalid int                   `json:"invalid"`
	Errors  int                   `json:"errors"`
	Results []BulkRevalidateEntry `json:"results"`
}

// BulkRevalidateEntry is one entry in the bulk revalidate result
type BulkRevalidateEntry struct {
	ID      string `json:"id"`
	Email   string `json:"email,omitempty"`
	IsValid bool   `json:"isValid"`
	Error   string `json:"error,omitempty"`
}

// BulkRevalidate re-checks multiple cookie sessions concurrently
func (s *CookieStoreService) BulkRevalidate(
	ctx context.Context,
	session *model.Session,
	ids []string,
) (*BulkRevalidateResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	result := &BulkRevalidateResult{Total: len(ids)}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to 5
	sem := make(chan struct{}, 5)

	for _, idStr := range ids {
		wg.Add(1)
		// Acquire the semaphore BEFORE spawning the goroutine so we bound
		// the number of live goroutines (not just the number running work).
		sem <- struct{}{}
		go func(idStr string) {
			defer wg.Done()
			defer func() { <-sem }()

			entry := BulkRevalidateEntry{ID: idStr}
			id, err := uuid.Parse(idStr)
			if err != nil {
				entry.Error = "invalid ID"
				mu.Lock()
				result.Errors++
				result.Results = append(result.Results, entry)
				mu.Unlock()
				return
			}

			if err := s.validateAndUpdate(ctx, id); err != nil {
				entry.Error = err.Error()
				mu.Lock()
				result.Errors++
				result.Results = append(result.Results, entry)
				mu.Unlock()
				return
			}

			store, err := s.CookieStoreRepo.GetByID(ctx, id)
			if err != nil {
				entry.Error = err.Error()
				mu.Lock()
				result.Errors++
				result.Results = append(result.Results, entry)
				mu.Unlock()
				return
			}

			entry.Email = store.Email
			entry.IsValid = store.IsValid

			mu.Lock()
			if store.IsValid {
				result.Valid++
			} else {
				result.Invalid++
			}
			result.Results = append(result.Results, entry)
			mu.Unlock()
		}(idStr)
	}

	wg.Wait()
	return result, nil
}

// --- Cookie Rotation ---

// CookieRotationConfig holds rotation settings for a campaign
type CookieRotationConfig struct {
	CookieStoreIDs []string `json:"cookieStoreIds"` // pool of cookie stores to rotate through
	Strategy       string   `json:"strategy"`       // "round_robin", "random", "least_used"
	MaxPerStore    int      `json:"maxPerStore"`    // max emails per store before rotating (0 = unlimited)
}

// CookieRotator manages round-robin / random rotation across cookie stores
type CookieRotator struct {
	mu       sync.RWMutex
	configs  map[string]*CookieRotationConfig // campaignID -> config
	counters map[string]map[string]int        // campaignID -> storeID -> send count
	indexes  map[string]int                   // campaignID -> current round-robin index
}

// NewCookieRotator creates a new rotator
func NewCookieRotator() *CookieRotator {
	return &CookieRotator{
		configs:  make(map[string]*CookieRotationConfig),
		counters: make(map[string]map[string]int),
		indexes:  make(map[string]int),
	}
}

// SetConfig sets the rotation config for a campaign
func (r *CookieRotator) SetConfig(campaignID string, config *CookieRotationConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[campaignID] = config
	if _, ok := r.counters[campaignID]; !ok {
		r.counters[campaignID] = make(map[string]int)
	}
}

// GetConfig returns the rotation config for a campaign
func (r *CookieRotator) GetConfig(campaignID string) *CookieRotationConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configs[campaignID]
}

// NextStore selects the next cookie store ID for a campaign
func (r *CookieRotator) NextStore(campaignID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, ok := r.configs[campaignID]
	if !ok || len(config.CookieStoreIDs) == 0 {
		return "", fmt.Errorf("no rotation config for campaign %s", campaignID)
	}

	counters := r.counters[campaignID]
	if counters == nil {
		counters = make(map[string]int)
		r.counters[campaignID] = counters
	}

	switch config.Strategy {
	case "random":
		available := r.availableStores(config, counters)
		if len(available) == 0 {
			// Reset counters and try again
			r.counters[campaignID] = make(map[string]int)
			counters = r.counters[campaignID]
			available = r.availableStores(config, counters)
		}
		if len(available) == 0 {
			return "", fmt.Errorf("no available cookie stores")
		}
		chosen := available[rand.Intn(len(available))]
		counters[chosen]++
		return chosen, nil

	case "least_used":
		available := r.availableStores(config, counters)
		if len(available) == 0 {
			r.counters[campaignID] = make(map[string]int)
			counters = r.counters[campaignID]
			available = r.availableStores(config, counters)
		}
		if len(available) == 0 {
			return "", fmt.Errorf("no available cookie stores")
		}
		// Find the one with the lowest count
		minCount := -1
		chosen := available[0]
		for _, id := range available {
			c := counters[id]
			if minCount == -1 || c < minCount {
				minCount = c
				chosen = id
			}
		}
		counters[chosen]++
		return chosen, nil

	default: // round_robin
		idx := r.indexes[campaignID] % len(config.CookieStoreIDs)
		storeID := config.CookieStoreIDs[idx]
		if config.MaxPerStore > 0 && counters[storeID] >= config.MaxPerStore {
			// Find next available
			for i := 0; i < len(config.CookieStoreIDs); i++ {
				candidate := config.CookieStoreIDs[(idx+i)%len(config.CookieStoreIDs)]
				if config.MaxPerStore == 0 || counters[candidate] < config.MaxPerStore {
					storeID = candidate
					r.indexes[campaignID] = (idx + i + 1) % len(config.CookieStoreIDs)
					break
				}
			}
		} else {
			r.indexes[campaignID] = (idx + 1) % len(config.CookieStoreIDs)
		}
		counters[storeID]++
		return storeID, nil
	}
}

// availableStores returns store IDs that haven't exceeded MaxPerStore
func (r *CookieRotator) availableStores(config *CookieRotationConfig, counters map[string]int) []string {
	if config.MaxPerStore <= 0 {
		return config.CookieStoreIDs
	}
	var available []string
	for _, id := range config.CookieStoreIDs {
		if counters[id] < config.MaxPerStore {
			available = append(available, id)
		}
	}
	return available
}

// GetStats returns rotation stats for a campaign
func (r *CookieRotator) GetStats(campaignID string) map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	config := r.configs[campaignID]
	counters := r.counters[campaignID]
	stats := map[string]interface{}{
		"campaignId": campaignID,
		"strategy":   "",
		"stores":     []interface{}{},
		"totalSent":  0,
	}
	if config != nil {
		stats["strategy"] = config.Strategy
		stats["maxPerStore"] = config.MaxPerStore
	}
	total := 0
	stores := []map[string]interface{}{}
	if counters != nil {
		for id, count := range counters {
			stores = append(stores, map[string]interface{}{
				"storeId": id,
				"sent":    count,
			})
			total += count
		}
	}
	stats["stores"] = stores
	stats["totalSent"] = total
	return stats
}

// --- Reply / Forward ---

// ReplyRequest holds parameters for replying to a message
type ReplyRequest struct {
	CookieStoreID string `json:"cookieStoreId"`
	MessageID     string `json:"messageId"`
	Body          string `json:"body"`
	IsHTML        bool   `json:"isHTML"`
	ReplyAll      bool   `json:"replyAll"`
}

// ForwardRequest holds parameters for forwarding a message
type ForwardRequest struct {
	CookieStoreID string   `json:"cookieStoreId"`
	MessageID     string   `json:"messageId"`
	To            []string `json:"to"`
	Body          string   `json:"body"`
	IsHTML        bool     `json:"isHTML"`
}

// ReplyToMessage replies to an email using Graph API
func (s *CookieStoreService) ReplyToMessage(
	ctx context.Context,
	session *model.Session,
	req *ReplyRequest,
) (*model.CookieSendResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	storeID, err := uuid.Parse(req.CookieStoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid cookie store ID: %s", req.CookieStoreID)
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("cookie store not found: %s", req.CookieStoreID)
	}

	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken == "" {
		// Try browser fallback
		if s.BrowserSession != nil {
			browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
			if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
				accessToken = browserResult.AccessToken
				s.cacheAccessToken(ctx, store.ID, browserResult)
			}
		}
	}

	if accessToken == "" {
		return &model.CookieSendResult{
			Success: false,
			Error:   "no valid access token available for reply",
			Method:  "graph_api",
			SentAt:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	contentType := "Text"
	if req.IsHTML {
		contentType = "HTML"
	}

	// Build reply payload
	payload := map[string]interface{}{
		"comment": req.Body,
		"message": map[string]interface{}{
			"body": map[string]interface{}{
				"contentType": contentType,
				"content":     req.Body,
			},
		},
	}

	endpoint := "reply"
	if req.ReplyAll {
		endpoint = "replyAll"
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/%s", req.MessageID, endpoint)
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		s.Logger.Infow("reply sent via Graph API",
			"messageId", req.MessageID,
			"replyAll", req.ReplyAll,
		)
		return &model.CookieSendResult{
			Success:   true,
			Method:    "graph_api",
			MessageID: fmt.Sprintf("reply-%d", time.Now().UnixMilli()),
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &model.CookieSendResult{
		Success: false, Method: "graph_api",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ForwardMessage forwards an email using Graph API
func (s *CookieStoreService) ForwardMessage(
	ctx context.Context,
	session *model.Session,
	req *ForwardRequest,
) (*model.CookieSendResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	storeID, err := uuid.Parse(req.CookieStoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid cookie store ID: %s", req.CookieStoreID)
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("cookie store not found: %s", req.CookieStoreID)
	}

	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken == "" {
		if s.BrowserSession != nil {
			browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
			if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
				accessToken = browserResult.AccessToken
				s.cacheAccessToken(ctx, store.ID, browserResult)
			}
		}
	}

	if accessToken == "" {
		return &model.CookieSendResult{
			Success: false,
			Error:   "no valid access token available for forward",
			Method:  "graph_api",
			SentAt:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	toRecipients := make([]map[string]interface{}, len(req.To))
	for i, email := range req.To {
		toRecipients[i] = map[string]interface{}{
			"emailAddress": map[string]string{"address": email},
		}
	}

	contentType := "Text"
	if req.IsHTML {
		contentType = "HTML"
	}

	payload := map[string]interface{}{
		"comment":      req.Body,
		"toRecipients": toRecipients,
		"message": map[string]interface{}{
			"body": map[string]interface{}{
				"contentType": contentType,
				"content":     req.Body,
			},
		},
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/forward", req.MessageID)
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		s.Logger.Infow("message forwarded via Graph API",
			"messageId", req.MessageID,
			"to", req.To,
		)
		return &model.CookieSendResult{
			Success:   true,
			Method:    "graph_api",
			MessageID: fmt.Sprintf("forward-%d", time.Now().UnixMilli()),
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &model.CookieSendResult{
		Success: false, Method: "graph_api",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
