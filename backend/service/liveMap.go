package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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
// Provides real-time geographic visualization of campaign events
// Ported concept from Evilginx's evilfeed live map dashboard
// ============================================================================

// MapEvent represents a single event on the live map
type MapEvent struct {
	ID          string    `json:"id"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	City        string    `json:"city"`
	Region      string    `json:"region"`
	Country     string    `json:"country"`
	CountryCode string    `json:"countryCode"`
	IPAddress   string    `json:"ipAddress"`
	UserAgent   string    `json:"userAgent"`
	EventType   string    `json:"eventType"` // "visit", "click", "submit", "capture"
	CampaignID  string    `json:"campaignId"`
	Timestamp   time.Time `json:"timestamp"`
}

// MapStats holds aggregate statistics for the map
type MapStats struct {
	TotalEvents     int            `json:"totalEvents"`
	ActiveCountries int            `json:"activeCountries"`
	ActiveCities    int            `json:"activeCities"`
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

// LiveMap manages the live map dashboard data
type LiveMap struct {
	Common
	OptionRepository   *repository.Option
	CampaignRepository *repository.Campaign
	mu                 sync.RWMutex
	recentEvents       []*MapEvent
	maxRecentEvents    int
	geoCache           sync.Map // IP -> *GeoIPResponse
	httpClient         *http.Client
}

// NewLiveMapService creates a new live map service
func NewLiveMapService(
	logger *zap.SugaredLogger,
	optionRepo *repository.Option,
	campaignRepo *repository.Campaign,
) *LiveMap {
	return &LiveMap{
		Common: Common{
			Logger: logger,
		},
		OptionRepository:   optionRepo,
		CampaignRepository: campaignRepo,
		maxRecentEvents:         100,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RecordEvent records a new event on the live map
func (lm *LiveMap) RecordEvent(
	ipAddress string,
	userAgent string,
	eventType string,
	campaignID string,
) {
	// look up geo data
	geo, err := lm.lookupGeoIP(ipAddress)
	if err != nil {
		lm.Logger.Debugw("geo lookup failed", "ip", ipAddress, "error", err)
		return
	}

	event := &MapEvent{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
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
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.recentEvents = append([]*MapEvent{event}, lm.recentEvents...)
	if len(lm.recentEvents) > lm.maxRecentEvents {
		lm.recentEvents = lm.recentEvents[:lm.maxRecentEvents]
	}
}

// GetRecentEvents returns recent map events
func (lm *LiveMap) GetRecentEvents(
	ctx context.Context,
	session *model.Session,
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

	if limit <= 0 || limit > len(lm.recentEvents) {
		limit = len(lm.recentEvents)
	}

	result := make([]*MapEvent, limit)
	copy(result, lm.recentEvents[:limit])
	return result, nil
}

// GetMapStats returns aggregate map statistics
func (lm *LiveMap) GetMapStats(
	ctx context.Context,
	session *model.Session,
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

	stats := &MapStats{
		TotalEvents:     len(lm.recentEvents),
		EventsByType:    make(map[string]int),
		EventsByCountry: make(map[string]int),
	}

	countries := make(map[string]bool)
	cities := make(map[string]bool)

	for _, e := range lm.recentEvents {
		stats.EventsByType[e.EventType]++
		stats.EventsByCountry[e.Country]++
		countries[e.Country] = true
		cities[e.City+","+e.Country] = true
	}

	stats.ActiveCountries = len(countries)
	stats.ActiveCities = len(cities)

	// include last 10 events
	limit := 10
	if len(lm.recentEvents) < limit {
		limit = len(lm.recentEvents)
	}
	stats.RecentEvents = lm.recentEvents[:limit]

	return stats, nil
}

// lookupGeoIP performs a GeoIP lookup for an IP address
func (lm *LiveMap) lookupGeoIP(ip string) (*GeoIPResponse, error) {
	// check cache first
	if cached, ok := lm.geoCache.Load(ip); ok {
		return cached.(*GeoIPResponse), nil
	}

	// skip private IPs
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil || parsedIP.IsPrivate() || parsedIP.IsLoopback() {
		return &GeoIPResponse{
			IP:      ip,
			City:    "Local",
			Country: "Local",
		}, nil
	}

	// Use ip-api.com (free, no key required, 45 req/min)
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

	// cache the result
	lm.geoCache.Store(ip, geo)

	return geo, nil
}

// maskIP partially masks an IP address for privacy
// e.g., "192.168.1.100" -> "192.168.1.***"
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
	// Since sync.Map doesn't track insertion time, we just clear it periodically
	lm.geoCache = sync.Map{}
}
