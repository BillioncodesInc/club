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
	// CachedAt is the time this entry was inserted into the geo cache.
	// Used by CleanupGeoCache to age out stale entries via a per-entry TTL.
	CachedAt time.Time `json:"-"`
}

// geoCacheTTL controls how long a geo lookup stays cached.
// 5-minute TTL is a good balance between reducing upstream traffic and
// handling IP-reassignment / mobile-network churn.
const geoCacheTTL = 5 * time.Minute

// ============================================================================
// EventRing: bounded ring buffer of MapEvents.
//
// Prior implementation used `append([]*MapEvent{event}, recentEvents...)`
// followed by a slice to cap, which is O(n) per insert (copy the whole slice
// each time) => O(n^2) across n events. With maxRecentEvents=1000 that is
// ~1M copies per full cycle, which shows up under sustained campaign traffic.
//
// The ring stores events newest-first by iteration (see iter). Internally the
// backing slice is filled in insertion order with head pointing at the NEXT
// write slot; reads walk backwards from head to produce the newest-first view
// the rest of the code (and the API) expects.
// ============================================================================
type eventRing struct {
	buf  []*MapEvent
	head int  // next write index
	size int  // number of populated slots (<= cap)
	full bool // becomes true once we wrap around
}

func newEventRing(capacity int) *eventRing {
	if capacity <= 0 {
		capacity = 1
	}
	return &eventRing{buf: make([]*MapEvent, capacity)}
}

// push appends a new event in O(1). When the ring is full, the oldest entry
// is overwritten in place.
func (r *eventRing) push(e *MapEvent) {
	r.buf[r.head] = e
	r.head = (r.head + 1) % len(r.buf)
	if !r.full {
		r.size++
		if r.size == len(r.buf) {
			r.full = true
		}
	}
}

// iter walks events newest-first, calling fn for each. fn returns false to
// stop iteration early (used for time-window cutoff).
func (r *eventRing) iter(fn func(*MapEvent) bool) {
	if r.size == 0 {
		return
	}
	capN := len(r.buf)
	// newest is at (head-1) mod cap
	for i := 0; i < r.size; i++ {
		idx := (r.head - 1 - i + capN*2) % capN
		e := r.buf[idx]
		if e == nil {
			continue
		}
		if !fn(e) {
			return
		}
	}
}

// compact drops events older than the given cutoff from the ring by rebuilding
// the backing slice. Called from CleanupOldEvents. O(n).
func (r *eventRing) compact(cutoff time.Time) {
	if r.size == 0 {
		return
	}
	kept := make([]*MapEvent, 0, r.size)
	// iterate newest-first and keep those after cutoff
	r.iter(func(e *MapEvent) bool {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
			return true
		}
		// everything older than cutoff can be skipped; events are ordered.
		return false
	})
	// reset buffer, then push back oldest->newest so newest ends up at head-1
	for i := range r.buf {
		r.buf[i] = nil
	}
	r.head = 0
	r.size = 0
	r.full = false
	for i := len(kept) - 1; i >= 0; i-- {
		r.push(kept[i])
	}
}

// length returns the count of populated events.
func (r *eventRing) length() int { return r.size }

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
	recentEvents       *eventRing // O(1) push; replaces prior O(n^2) prepend+slice
	maxRecentEvents    int
	geoCache           sync.Map
	sessionDedup       sync.Map
	httpClient         *http.Client
	cleanupDone        chan struct{}
	stopOnce           sync.Once
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
	const ringCap = 1000
	lm := &LiveMap{
		Common: Common{
			Logger: logger,
		},
		OptionRepository:   optionRepo,
		CampaignRepository: campaignRepo,
		maxRecentEvents:    ringCap,
		recentEvents:       newEventRing(ringCap),
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

	// Bounded ring buffer push: O(1). Capacity is lm.maxRecentEvents (1000).
	lm.recentEvents.push(event)
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
		// Lock contract: we hold BotGuard.mu as an RLock for the duration of
		// this iteration and only perform read-only access on the session
		// entries. Same-package access to the unexported `sessions` map is
		// intentional; callers outside this package should use a public
		// accessor. Do NOT mutate any session under this RLock.
		lm.BotGuard.mu.RLock()
		var matched *BotSession
		for _, session := range lm.BotGuard.sessions {
			if session.IP == ip && session.UserAgent == ua {
				matched = session
				break
			}
		}
		lm.BotGuard.mu.RUnlock()
		if matched != nil {
			return matched.IsBot
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
	lm.recentEvents.iter(func(e *MapEvent) bool {
		if e.Timestamp.Before(cutoff) {
			return false
		}
		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			return false
		}
		return true
	})

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

	lm.recentEvents.iter(func(e *MapEvent) bool {
		if e.Timestamp.Before(cutoff) {
			return false
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
		return true
	})

	stats.ActiveCountries = len(countries)
	stats.ActiveCities = len(cities)
	stats.UniqueVisitors = len(uniqueVisitors)

	limit := 10
	if stats.TotalEvents < limit {
		limit = stats.TotalEvents
	}
	if limit > 0 {
		lm.recentEvents.iter(func(e *MapEvent) bool {
			if e.Timestamp.Before(cutoff) {
				return false
			}
			stats.RecentEvents = append(stats.RecentEvents, e)
			return len(stats.RecentEvents) < limit
		})
	}

	return stats, nil
}

// lookupGeoIP performs a GeoIP lookup for an IP address.
//
// Results are cached in-memory per IP for geoCacheTTL (5 minutes). The TTL
// matters because we also cache Local / private-IP shortcut responses and
// failed lookups would otherwise hit the upstream every time for the same
// traffic. The per-entry CachedAt timestamp lets CleanupGeoCache drop only
// expired entries rather than wipe the whole cache on each sweep.
func (lm *LiveMap) lookupGeoIP(ip string) (*GeoIPResponse, error) {
	if cached, ok := lm.geoCache.Load(ip); ok {
		if geo, castOK := cached.(*GeoIPResponse); castOK {
			if time.Since(geo.CachedAt) < geoCacheTTL {
				return geo, nil
			}
			// stale — fall through and refresh.
			lm.geoCache.Delete(ip)
		}
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil || parsedIP.IsPrivate() || parsedIP.IsLoopback() {
		local := &GeoIPResponse{
			IP:       ip,
			City:     "Local",
			Country:  "Local",
			CachedAt: time.Now(),
		}
		lm.geoCache.Store(ip, local)
		return local, nil
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
		CachedAt:    time.Now(),
	}

	lm.geoCache.Store(ip, geo)
	return geo, nil
}

// maskIP partially masks an IP address for privacy.
//
// For IPv4: keeps the first three octets (/24) and masks the last.
//   e.g. 203.0.113.42  -> 203.0.113.*
// For IPv6: keeps the first three 16-bit groups (/48) and masks the rest.
//   e.g. 2001:db8:1::1 -> 2001:db8:1:*
//
// Previous implementation iterated raw bytes and counted '.' only, which
// produced garbage for IPv6 (e.g. "::1" -> "::1" untouched since it has no
// dots, and "fe80::1" -> "fe80::1" likewise — never masking anything).
// The new version parses via net.ParseIP so the mask is always structurally
// correct regardless of input shape.
func maskIP(ip string) string {
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		// Not a valid IP (e.g. hostname or empty). Return the last two
		// characters masked as a conservative default, rather than leaking
		// the raw string.
		if len(ip) <= 2 {
			return "**"
		}
		return ip[:len(ip)-2] + "**"
	}
	if v4 := parsed.To4(); v4 != nil {
		// Canonicalize to dotted-quad and mask the last octet.
		return fmt.Sprintf("%d.%d.%d.*", v4[0], v4[1], v4[2])
	}
	// IPv6: parsed is 16 bytes. Keep first 48 bits (first 3 groups), mask rest.
	v6 := parsed.To16()
	if v6 == nil {
		return "**"
	}
	g0 := uint16(v6[0])<<8 | uint16(v6[1])
	g1 := uint16(v6[2])<<8 | uint16(v6[3])
	g2 := uint16(v6[4])<<8 | uint16(v6[5])
	return fmt.Sprintf("%x:%x:%x:*", g0, g1, g2)
}

// CleanupOldEvents removes events older than the specified duration
func (lm *LiveMap) CleanupOldEvents(maxAge time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	lm.recentEvents.compact(cutoff)
}

// CleanupGeoCache removes entries older than maxAge from the geo cache.
//
// Per-entry age is now tracked via GeoIPResponse.CachedAt, so this can be a
// precise expiry sweep instead of a blanket wipe. A non-positive maxAge is
// treated as "clear everything" for operator-triggered purges.
func (lm *LiveMap) CleanupGeoCache(maxAge time.Duration) {
	if maxAge <= 0 {
		lm.geoCache.Range(func(key, _ interface{}) bool {
			lm.geoCache.Delete(key)
			return true
		})
		return
	}
	cutoff := time.Now().Add(-maxAge)
	lm.geoCache.Range(func(key, value interface{}) bool {
		geo, ok := value.(*GeoIPResponse)
		if !ok || geo.CachedAt.Before(cutoff) {
			lm.geoCache.Delete(key)
		}
		return true
	})
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
	lm.stopOnce.Do(func() { close(lm.cleanupDone) })
}
