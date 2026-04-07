package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

// ============================================================================
// Live Map Dashboard Service
// Provides real-time geographic visualization of campaign events.
//
// Key design decisions:
// - Events are deduplicated per unique session (IP+UA hash), NOT per-request.
//   A single browser session generates many sub-requests (CSS, JS, images, etc.)
//   but should only appear as ONE event on the map.
// - Bot traffic is tagged separately so the frontend can filter/display it differently.
// - Time-based filtering is supported so the frontend can request events for
//   specific time windows (last 15min, 1hr, 24hr, etc.).
// - Cleanup routines run automatically to prevent unbounded memory growth.
// ============================================================================

// MapEvent represents a single event on the live map
type MapEvent struct {
	ID          string    `json:"id"`
	SessionKey  string    `json:"-"` // internal dedup key, not sent to frontend
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	City        string    `json:"city"`
	Region      string    `json:"region"`
	Country     string    `json:"country"`
	CountryCode string    `json:"countryCode"`
	IPAddress   string    `json:"ipAddress"`
	UserAgent   string    `json:"userAgent"`
	EventType   string    `json:"eventType"`
	CampaignID  string    `json:"campaignId"`
	Timestamp   time.Time `json:"timestamp"`
	IsBot       bool      `json:"isBot"`
}

// MapStats holds aggregate statistics for the map
type MapStats struct {
	TotalEvents     int            `json:"totalEvents"`
	UniqueVisitors  int            `json:"uniqueVisitors"`
	ActiveCountries int            `json:"activeCountries"`
	ActiveCities    int            `json:"activeCities"`
	BotEvents       int            `json:"botEvents"`
	RealEvents      int            `json:"realEvents"`
	EventsByType    map[string]int `json:"eventsByType"`
	EventsByCountry map[string]int `json:"eventsByCountry"`
	RecentEvents    []*MapEvent    `json:"recentEvents"`
}

// GeoIPResponse represents the response from a GeoIP lookup service
type GeoIPResponse struct {
	IP          string  `json:"ip"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	ASN         string  `json:"asn"`
}

// sessionDedup tracks when we last recorded each event type for a session
type sessionDedup struct {
	lastSeen map[string]time.Time
	mu       sync.Mutex
}

// LiveMap manages the live map dashboard data
type LiveMap struct {
	Common
	OptionRepository   *repository.Option
	CampaignRepository *repository.Campaign
	BotGuard           *BotGuard
	mu                 sync.RWMutex
	recentEvents       []*MapEvent
	maxRecentEvents    int
	geoCache           sync.Map
	sessionDedup       sync.Map
	httpClient         *http.Client
	cleanupDone        chan struct{}
}

// Dedup windows per event type
var eventDedupWindows = map[string]time.Duration{
	"proxy_visit":  10 * time.Minute,
	"visit":        5 * time.Minute,
	"proxy_submit": 2 * time.Minute,
	"proxy_cookie": 2 * time.Minute,
	"submit":       2 * time.Minute,
}

const defaultDedupWindow = 5 * time.Minute

// NewLiveMapService creates a new live map service
func NewLiveMapService(
	logger *zap.SugaredLogger,
	optionRepo *repository.Option,
	campaignRepo *repository.Campaign,
) *LiveMap {
	lm := &LiveMap{
		Common: Common{
			Logger: logger,
		},
		OptionRepository:   optionRepo,
		CampaignRepository: campaignRepo,
		maxRecentEvents:    1000,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		cleanupDone: make(chan struct{}),
	}
	go lm.runCleanup()
	return lm
}

// SetBotGuard sets the BotGuard reference for bot detection integration.
func (lm *LiveMap) SetBotGuard(bg *BotGuard) {
	lm.BotGuard = bg
}

// runCleanup periodically cleans up old events and dedup entries
func (lm *LiveMap) runCleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lm.CleanupOldEvents(24 * time.Hour)
			lm.CleanupSessionDedup()
		case <-lm.cleanupDone:
			return
		}
	}
}

// RecordEvent records a new event on the live map.
// Deduplicates per session (IP+UA) per event type within a time window.
func (lm *LiveMap) RecordEvent(
	ipAddress string,
	userAgent string,
	eventType string,
	campaignID string,
) {
	sessionKey := ipAddress + "|" + userAgent

	dedupWindow := defaultDedupWindow
	if w, ok := eventDedupWindows[eventType]; ok {
		dedupWindow = w
	}

	dedupVal, _ := lm.sessionDedup.LoadOrStore(sessionKey, &sessionDedup{
		lastSeen: make(map[string]time.Time),
	})
	dedup := dedupVal.(*sessionDedup)

	dedup.mu.Lock()
	if lastTime, exists := dedup.lastSeen[eventType]; exists {
		if time.Since(lastTime) < dedupWindow {
			dedup.mu.Unlock()
			return
		}
	}
	dedup.lastSeen[eventType] = time.Now()
	dedup.mu.Unlock()

	isBot := lm.checkIsBot(ipAddress, userAgent)

	geo, err := lm.lookupGeoIP(ipAddress)
	if err != nil {
		lm.Logger.Debugw("geo lookup failed", "ip", ipAddress, "error", err)
		return
	}

	event := &MapEvent{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		SessionKey:  sessionKey,
		Latitude:    geo.Latitude,
		Longitude:   geo.Longitude,
		City:        geo.City,
		Region:      geo.Region,
		Country:     geo.Country,
		CountryCode: geo.CountryCode,
		IPAddress:   maskIP(ipAddress),
		UserAgent:   userAgent,
		EventType:   eventType,
		CampaignID:  campaignID,
		Timestamp:   time.Now(),
		IsBot:       isBot,
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.recentEvents = append([]*MapEvent{event}, lm.recentEvents...)
	if len(lm.recentEvents) > lm.maxRecentEvents {
		lm.recentEvents = lm.recentEvents[:lm.maxRecentEvents]
	}
}

// checkIsBot checks if the given IP+UA is flagged as a bot
func (lm *LiveMap) checkIsBot(ip, ua string) bool {
	lower := strings.ToLower(ua)
	botPatterns := []string{
		"bot", "crawler", "spider", "scraper", "curl", "wget",
		"python-requests", "go-http-client", "httpie", "postman",
		"phantomjs", "headlesschrome", "selenium", "puppeteer",
		"nessus", "nikto", "nmap", "masscan", "qualys", "acunetix",
		"burpsuite", "zap", "sqlmap", "dirbuster", "gobuster",
		"barracuda", "proofpoint", "mimecast", "fireeye", "sophos",
		"forcepoint", "ironport", "messagelabs", "symantec",
	}
	for _, pattern := range botPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	if ua == "" {
		return true
	}

	if lm.BotGuard != nil {
		lm.BotGuard.mu.RLock()
		defer lm.BotGuard.mu.RUnlock()
		for _, session := range lm.BotGuard.sessions {
			if session.IP == ip && session.UserAgent == ua {
				return session.IsBot
			}
		}
	}

	return false
}

// GetRecentEvents returns recent map events filtered by time window
func (lm *LiveMap) GetRecentEvents(
	ctx context.Context,
	session *model.Session,
	minutes int,
	limit int,
) ([]*MapEvent, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		lm.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	var result []*MapEvent
	for _, e := range lm.recentEvents {
		if e.Timestamp.Before(cutoff) {
			break
		}
		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

// GetMapStats returns aggregate map statistics filtered by time window
func (lm *LiveMap) GetMapStats(
	ctx context.Context,
	session *model.Session,
	minutes int,
) (*MapStats, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		lm.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)

	stats := &MapStats{
		EventsByType:    make(map[string]int),
		EventsByCountry: make(map[string]int),
	}

	countries := make(map[string]bool)
	cities := make(map[string]bool)
	uniqueVisitors := make(map[string]bool)

	for _, e := range lm.recentEvents {
		if e.Timestamp.Before(cutoff) {
			break
		}

		stats.TotalEvents++
		stats.EventsByType[e.EventType]++
		if e.Country != "" {
			stats.EventsByCountry[e.Country]++
			countries[e.Country] = true
		}
		if e.City != "" && e.Country != "" {
			cities[e.City+","+e.Country] = true
		}
		uniqueVisitors[e.IPAddress] = true

		if e.IsBot {
			stats.BotEvents++
		} else {
			stats.RealEvents++
		}
	}

	stats.ActiveCountries = len(countries)
	stats.ActiveCities = len(cities)
	stats.UniqueVisitors = len(uniqueVisitors)

	limit := 10
	if stats.TotalEvents < limit {
		limit = stats.TotalEvents
	}
	if limit > 0 {
		for _, e := range lm.recentEvents {
			if e.Timestamp.Before(cutoff) {
				break
			}
			stats.RecentEvents = append(stats.RecentEvents, e)
			if len(stats.RecentEvents) >= limit {
				break
			}
		}
	}

	return stats, nil
}

// lookupGeoIP performs a GeoIP lookup for an IP address
func (lm *LiveMap) lookupGeoIP(ip string) (*GeoIPResponse, error) {
	if cached, ok := lm.geoCache.Load(ip); ok {
		return cached.(*GeoIPResponse), nil
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil || parsedIP.IsPrivate() || parsedIP.IsLoopback() {
		return &GeoIPResponse{
			IP:      ip,
			City:    "Local",
			Country: "Local",
		}, nil
	}

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,regionName,city,lat,lon,timezone,isp,org,as", ip)

	resp, err := lm.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("geo lookup failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read geo response: %w", err)
	}

	var apiResp struct {
		Status      string  `json:"status"`
		Message     string  `json:"message"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		RegionName  string  `json:"regionName"`
		City        string  `json:"city"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		Timezone    string  `json:"timezone"`
		ISP         string  `json:"isp"`
		Org         string  `json:"org"`
		AS          string  `json:"as"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse geo response: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("geo lookup failed: %s", apiResp.Message)
	}

	geo := &GeoIPResponse{
		IP:          ip,
		City:        apiResp.City,
		Region:      apiResp.RegionName,
		Country:     apiResp.Country,
		CountryCode: apiResp.CountryCode,
		Latitude:    apiResp.Lat,
		Longitude:   apiResp.Lon,
		Timezone:    apiResp.Timezone,
		ISP:         apiResp.ISP,
		Org:         apiResp.Org,
		ASN:         apiResp.AS,
	}

	lm.geoCache.Store(ip, geo)
	return geo, nil
}

// maskIP partially masks an IP address for privacy
func maskIP(ip string) string {
	parts := make([]byte, 0, len(ip))
	dotCount := 0
	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' {
			dotCount++
		}
		if dotCount >= 3 && ip[i] != '.' {
			parts = append(parts, '*')
		} else {
			parts = append(parts, ip[i])
		}
	}
	return string(parts)
}

// CleanupOldEvents removes events older than the specified duration
func (lm *LiveMap) CleanupOldEvents(maxAge time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var filtered []*MapEvent
	for _, e := range lm.recentEvents {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	lm.recentEvents = filtered
}

// CleanupGeoCache removes old entries from the geo cache
func (lm *LiveMap) CleanupGeoCache(maxAge time.Duration) {
	lm.geoCache = sync.Map{}
}

// CleanupSessionDedup removes old dedup entries
func (lm *LiveMap) CleanupSessionDedup() {
	lm.sessionDedup.Range(func(key, value interface{}) bool {
		dedup := value.(*sessionDedup)
		dedup.mu.Lock()
		allExpired := true
		for eventType, lastTime := range dedup.lastSeen {
			window := defaultDedupWindow
			if w, ok := eventDedupWindows[eventType]; ok {
				window = w
			}
			if time.Since(lastTime) > window*2 {
				delete(dedup.lastSeen, eventType)
			} else {
				allExpired = false
			}
		}
		dedup.mu.Unlock()
		if allExpired {
			lm.sessionDedup.Delete(key)
		}
		return true
	})
}

// Stop gracefully stops the cleanup routine
func (lm *LiveMap) Stop() {
	close(lm.cleanupDone)
}
