package service

import (
	"bytes"
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
// Domain Auto-Rotation & Reputation Monitoring Service
// Ported from Evilginx's domain_rotator.go and blacklist_evasion.go
// ============================================================================

// DomainRotatorConfig holds the configuration for domain rotation
type DomainRotatorConfig struct {
	Enabled           bool          `json:"enabled"`
	DomainPool        []DomainEntry `json:"domainPool"`
	CooldownMinutes   int           `json:"cooldownMinutes"`   // min time between rotations
	AutoRotateOnBlock bool          `json:"autoRotateOnBlock"` // rotate when ASN blocker threshold hit
	ReputationCheck   bool          `json:"reputationCheck"`   // periodically check domain reputation
	CheckIntervalMin  int           `json:"checkIntervalMin"`  // reputation check interval in minutes
	NotifyOnRotation  bool          `json:"notifyOnRotation"`  // send Telegram notification on rotation
}

// DomainEntry represents a domain in the rotation pool
type DomainEntry struct {
	Domain      string    `json:"domain"`
	Status      string    `json:"status"`      // "active", "standby", "burned", "cooldown"
	AddedAt     time.Time `json:"addedAt"`
	LastUsedAt  time.Time `json:"lastUsedAt"`
	BurnedAt    time.Time `json:"burnedAt,omitempty"`
	BurnReason  string    `json:"burnReason,omitempty"`
	Reputation  *ReputationInfo `json:"reputation,omitempty"`
}

// ReputationInfo holds reputation data for a domain
type ReputationInfo struct {
	Score           int       `json:"score"`           // 0-100, higher is better
	IsBlacklisted   bool      `json:"isBlacklisted"`
	GoogleSafeBrowsing bool   `json:"googleSafeBrowsing"`
	PhishTank       bool      `json:"phishTank"`
	VirusTotal      int       `json:"virusTotal"`      // number of detections
	LastChecked     time.Time `json:"lastChecked"`
	CheckError      string    `json:"checkError,omitempty"`
}

// RotationResult captures what happened during a rotation
type RotationResult struct {
	OldDomain    string    `json:"oldDomain"`
	NewDomain    string    `json:"newDomain"`
	RotatedAt    time.Time `json:"rotatedAt"`
	Reason       string    `json:"reason"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	CampaignsUpdated int  `json:"campaignsUpdated"`
}

// RotatorStatus is the current status of the domain rotator
type RotatorStatus struct {
	CurrentDomain  string        `json:"currentDomain"`
	PoolSize       int           `json:"poolSize"`
	DomainPool     []DomainEntry `json:"domainPool"`
	IsReady        bool          `json:"isReady"`
	LastRotation   time.Time     `json:"lastRotation"`
	CooldownActive bool          `json:"cooldownActive"`
	RotationCount  int           `json:"rotationCount"`
}

// DomainRotator manages automatic domain rotation
type DomainRotator struct {
	Common
	OptionRepository   *repository.Option
	CampaignRepository *repository.Campaign
	TelegramService    *Telegram
	config             *DomainRotatorConfig
	mu                 sync.Mutex
	lastRotation       time.Time
	rotationCount      int
	stopChan           chan struct{}
	httpClient         *http.Client
}

// NewDomainRotatorService creates a new domain rotation service
func NewDomainRotatorService(
	logger *zap.SugaredLogger,
	optionRepo *repository.Option,
	campaignRepo *repository.Campaign,
	telegramSvc *Telegram,
) *DomainRotator {
	svc := &DomainRotator{
		Common: Common{
			Logger: logger,
		},
		OptionRepository:   optionRepo,
		CampaignRepository: campaignRepo,
		TelegramService:    telegramSvc,
		stopChan:           make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	svc.loadConfigFromDB()

	// start reputation monitoring if enabled
	if svc.config.ReputationCheck {
		go svc.reputationMonitorLoop()
	}

	return svc
}

// loadConfigFromDB loads the domain rotator configuration
func (dr *DomainRotator) loadConfigFromDB() {
	ctx := context.Background()
	opt, err := dr.OptionRepository.GetByKey(ctx, data.OptionKeyDomainRotatorConfig)
	if err != nil {
		dr.Logger.Debugw("no domain rotator config found, using defaults")
		dr.config = &DomainRotatorConfig{
			Enabled:          false,
			CooldownMinutes:  30,
			ReputationCheck:  false,
			CheckIntervalMin: 60,
			NotifyOnRotation: true,
		}
		return
	}

	var config DomainRotatorConfig
	if err := json.Unmarshal([]byte(opt.Value.String()), &config); err != nil {
		dr.Logger.Errorw("failed to unmarshal domain rotator config", "error", err)
		dr.config = &DomainRotatorConfig{Enabled: false}
		return
	}

	dr.config = &config
	dr.Logger.Infow("loaded domain rotator config",
		"enabled", config.Enabled,
		"poolSize", len(config.DomainPool),
	)
}

// saveConfigToDB persists the domain rotator configuration
func (dr *DomainRotator) saveConfigToDB() error {
	jsonData, err := json.Marshal(dr.config)
	if err != nil {
		return fmt.Errorf("failed to marshal domain rotator config: %w", err)
	}
	ctx := context.Background()
	return dr.OptionRepository.UpsertByKey(ctx, data.OptionKeyDomainRotatorConfig, string(jsonData))
}

// UpdateConfig updates the domain rotator configuration
func (dr *DomainRotator) UpdateConfig(
	ctx context.Context,
	session *model.Session,
	config *DomainRotatorConfig,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		dr.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	dr.mu.Lock()
	dr.config = config
	dr.mu.Unlock()

	return dr.saveConfigToDB()
}

// GetConfig returns the current configuration
func (dr *DomainRotator) GetConfig() *DomainRotatorConfig {
	return dr.config
}

// GetStatus returns the current rotation status
func (dr *DomainRotator) GetStatus() *RotatorStatus {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	currentDomain := ""
	for _, d := range dr.config.DomainPool {
		if d.Status == "active" {
			currentDomain = d.Domain
			break
		}
	}

	cooldownDuration := time.Duration(dr.config.CooldownMinutes) * time.Minute
	return &RotatorStatus{
		CurrentDomain:  currentDomain,
		PoolSize:       len(dr.config.DomainPool),
		DomainPool:     dr.config.DomainPool,
		IsReady:        len(dr.config.DomainPool) > 1,
		LastRotation:   dr.lastRotation,
		CooldownActive: time.Since(dr.lastRotation) < cooldownDuration,
		RotationCount:  dr.rotationCount,
	}
}

// AddDomain adds a domain to the rotation pool
func (dr *DomainRotator) AddDomain(
	ctx context.Context,
	session *model.Session,
	domain string,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		dr.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	dr.mu.Lock()
	defer dr.mu.Unlock()

	// check for duplicates
	for _, d := range dr.config.DomainPool {
		if d.Domain == domain {
			return fmt.Errorf("domain '%s' already in pool", domain)
		}
	}

	status := "standby"
	if len(dr.config.DomainPool) == 0 {
		status = "active"
	}

	dr.config.DomainPool = append(dr.config.DomainPool, DomainEntry{
		Domain:  domain,
		Status:  status,
		AddedAt: time.Now(),
	})

	return dr.saveConfigToDB()
}

// RemoveDomain removes a domain from the rotation pool
func (dr *DomainRotator) RemoveDomain(
	ctx context.Context,
	session *model.Session,
	domain string,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		dr.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	dr.mu.Lock()
	defer dr.mu.Unlock()

	var newPool []DomainEntry
	found := false
	for _, d := range dr.config.DomainPool {
		if d.Domain == domain {
			found = true
			continue
		}
		newPool = append(newPool, d)
	}

	if !found {
		return fmt.Errorf("domain '%s' not found in pool", domain)
	}

	dr.config.DomainPool = newPool
	return dr.saveConfigToDB()
}

// RotateDomain performs a domain rotation
func (dr *DomainRotator) RotateDomain(
	ctx context.Context,
	session *model.Session,
	reason string,
) (*RotationResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		dr.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	return dr.doRotation(reason)
}

// TriggerAutoRotation is called by the ASN blocker when threshold is exceeded
func (dr *DomainRotator) TriggerAutoRotation(reason string) (*RotationResult, error) {
	if !dr.config.Enabled || !dr.config.AutoRotateOnBlock {
		return nil, fmt.Errorf("auto-rotation not enabled")
	}

	// check cooldown
	cooldownDuration := time.Duration(dr.config.CooldownMinutes) * time.Minute
	if time.Since(dr.lastRotation) < cooldownDuration {
		return nil, fmt.Errorf("rotation on cooldown (%.0f min remaining)",
			(cooldownDuration - time.Since(dr.lastRotation)).Minutes())
	}

	return dr.doRotation(reason)
}

// doRotation performs the actual domain rotation
func (dr *DomainRotator) doRotation(reason string) (*RotationResult, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	result := &RotationResult{
		Reason: reason,
	}

	// find current active domain
	var oldDomain string
	var oldIdx int = -1
	for i, d := range dr.config.DomainPool {
		if d.Status == "active" {
			oldDomain = d.Domain
			oldIdx = i
			break
		}
	}

	if oldDomain == "" {
		result.Error = "no active domain found"
		return result, fmt.Errorf("no active domain found")
	}
	result.OldDomain = oldDomain

	// find next available standby domain
	var newDomain string
	var newIdx int = -1
	for i, d := range dr.config.DomainPool {
		if d.Status == "standby" {
			newDomain = d.Domain
			newIdx = i
			break
		}
	}

	if newDomain == "" {
		result.Error = "no standby domain available"
		return result, fmt.Errorf("no standby domain available in pool")
	}
	result.NewDomain = newDomain

	dr.Logger.Warnw("=== STARTING DOMAIN ROTATION ===",
		"old", oldDomain,
		"new", newDomain,
		"reason", reason,
	)

	// Step 1: Mark old domain as burned/cooldown
	dr.config.DomainPool[oldIdx].Status = "burned"
	dr.config.DomainPool[oldIdx].BurnedAt = time.Now()
	dr.config.DomainPool[oldIdx].BurnReason = reason

	// Step 2: Activate new domain
	dr.config.DomainPool[newIdx].Status = "active"
	dr.config.DomainPool[newIdx].LastUsedAt = time.Now()

	// Step 3: Save config
	if err := dr.saveConfigToDB(); err != nil {
		dr.Logger.Errorw("failed to save rotation config", "error", err)
		result.Error = err.Error()
		return result, err
	}

	dr.lastRotation = time.Now()
	dr.rotationCount++
	result.RotatedAt = dr.lastRotation
	result.Success = true

	dr.Logger.Warnw("=== DOMAIN ROTATION COMPLETE ===",
		"new", newDomain,
		"rotationCount", dr.rotationCount,
	)

	// Step 4: Send Telegram notification if enabled
	if dr.config.NotifyOnRotation && dr.TelegramService != nil {
		ctx := context.Background()
		dr.TelegramService.Notify(
			ctx,
			"domain_rotation",
			fmt.Sprintf("Domain Rotation: %s -> %s (reason: %s, #%d)", oldDomain, newDomain, reason, dr.rotationCount),
			"",
			nil,
		)
	}

	return result, nil
}

// ============================================================================
// Reputation Monitoring
// ============================================================================

// CheckDomainReputation checks the reputation of a domain
func (dr *DomainRotator) CheckDomainReputation(domain string) (*ReputationInfo, error) {
	rep := &ReputationInfo{
		Score:       100, // start with perfect score
		LastChecked: time.Now(),
	}

	// Check 1: DNS resolution (domain must resolve)
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		rep.Score -= 50
		rep.CheckError = "DNS resolution failed"
	}

	// Check 2: Google Safe Browsing (via transparency report)
	gsb, err := dr.checkGoogleSafeBrowsing(domain)
	if err == nil && gsb {
		rep.GoogleSafeBrowsing = true
		rep.IsBlacklisted = true
		rep.Score -= 80
	}

	// Check 3: HTTP connectivity
	resp, err := dr.httpClient.Get("https://" + domain)
	if err != nil {
		rep.Score -= 10
	} else {
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			rep.Score -= 20
		}
	}

	// Clamp score
	if rep.Score < 0 {
		rep.Score = 0
	}

	return rep, nil
}

// checkGoogleSafeBrowsing checks if a domain is flagged by Google Safe Browsing
// using the official GSB Lookup API v4 (threatMatches:find).
// Returns true if the domain is flagged, false otherwise.
// Requires a Google API key stored in the options table under OptionKeyGSBApiKey.
func (dr *DomainRotator) checkGoogleSafeBrowsing(domain string) (bool, error) {
	// load API key from options
	apiKey, err := dr.getGSBApiKey()
	if err != nil || apiKey == "" {
		// no API key configured, fall back to transparency report heuristic
		return dr.checkGSBTransparencyReport(domain)
	}

	// build the Lookup API v4 request
	reqBody := gsbLookupRequest{
		Client: gsbClient{
			ClientID:      "phishingclub",
			ClientVersion: "1.0",
		},
		ThreatInfo: gsbThreatInfo{
			ThreatTypes:      []string{"SOCIAL_ENGINEERING", "MALWARE", "UNWANTED_SOFTWARE"},
			PlatformTypes:    []string{"ANY_PLATFORM"},
			ThreatEntryTypes: []string{"URL"},
			ThreatEntries: []gsbThreatEntry{
				{URL: "https://" + domain + "/"},
				{URL: "http://" + domain + "/"},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal GSB request: %w", err)
	}

	endpoint := fmt.Sprintf("https://safebrowsing.googleapis.com/v4/threatMatches:find?key=%s", apiKey)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return false, fmt.Errorf("failed to create GSB request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := dr.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("GSB API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("GSB API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result gsbLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// empty body {} means no matches - this is OK
		return false, nil
	}

	// if matches array is non-empty, the domain is flagged
	if len(result.Matches) > 0 {
		for _, m := range result.Matches {
			dr.Logger.Warnw("GSB: domain flagged",
				"domain", domain,
				"threatType", m.ThreatType,
				"platformType", m.PlatformType,
				"url", m.Threat.URL,
			)
		}
		return true, nil
	}

	return false, nil
}

// getGSBApiKey retrieves the Google Safe Browsing API key from the options table
func (dr *DomainRotator) getGSBApiKey() (string, error) {
	ctx := context.Background()
	opt, err := dr.OptionRepository.GetByKey(ctx, data.OptionKeyGSBApiKey)
	if err != nil {
		return "", err
	}
	return opt.Value.String(), nil
}

// checkGSBTransparencyReport is a fallback heuristic using the Google Transparency Report
// when no API key is configured. It checks if the response body contains threat indicators.
func (dr *DomainRotator) checkGSBTransparencyReport(domain string) (bool, error) {
	url := fmt.Sprintf("https://transparencyreport.google.com/transparencyreport/api/v3/safebrowsing/status?site=%s", domain)

	resp, err := dr.httpClient.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// The Transparency Report API returns a JSON-like response with )]}' prefix.
	// A clean domain typically has status "1" (no unsafe content).
	// Status "2", "3", "4" or "5" indicate varying degrees of unsafe content.
	bodyStr := string(body)
	// strip the XSSI protection prefix
	bodyStr = strings.TrimPrefix(bodyStr, ")]}'\n")
	bodyStr = strings.TrimPrefix(bodyStr, ")]}'")

	// look for unsafe indicators in the response
	// status values: 1=no unsafe content, 2=some pages unsafe, 3=site is unsafe
	if strings.Contains(bodyStr, `"2"`) || strings.Contains(bodyStr, `"3"`) ||
		strings.Contains(bodyStr, `"4"`) || strings.Contains(bodyStr, `"5"`) {
		dr.Logger.Warnw("GSB Transparency Report: domain may be flagged",
			"domain", domain,
			"response", bodyStr,
		)
		return true, nil
	}

	return false, nil
}

// ─── GSB Lookup API v4 Types ────────────────────────────────────────

type gsbLookupRequest struct {
	Client     gsbClient     `json:"client"`
	ThreatInfo gsbThreatInfo `json:"threatInfo"`
}

type gsbClient struct {
	ClientID      string `json:"clientId"`
	ClientVersion string `json:"clientVersion"`
}

type gsbThreatInfo struct {
	ThreatTypes      []string         `json:"threatTypes"`
	PlatformTypes    []string         `json:"platformTypes"`
	ThreatEntryTypes []string         `json:"threatEntryTypes"`
	ThreatEntries    []gsbThreatEntry `json:"threatEntries"`
}

type gsbThreatEntry struct {
	URL string `json:"url"`
}

type gsbLookupResponse struct {
	Matches []gsbMatch `json:"matches"`
}

type gsbMatch struct {
	ThreatType      string         `json:"threatType"`
	PlatformType    string         `json:"platformType"`
	ThreatEntryType string         `json:"threatEntryType"`
	Threat          gsbThreatEntry `json:"threat"`
	CacheDuration   string         `json:"cacheDuration"`
}

// reputationMonitorLoop periodically checks domain reputation
func (dr *DomainRotator) reputationMonitorLoop() {
	interval := time.Duration(dr.config.CheckIntervalMin) * time.Minute
	if interval < 5*time.Minute {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dr.checkAllDomainReputations()
		case <-dr.stopChan:
			return
		}
	}
}

// checkAllDomainReputations checks reputation for all domains in the pool
func (dr *DomainRotator) checkAllDomainReputations() {
	dr.mu.Lock()
	domains := make([]DomainEntry, len(dr.config.DomainPool))
	copy(domains, dr.config.DomainPool)
	dr.mu.Unlock()

	for i, d := range domains {
		if d.Status == "burned" {
			continue
		}

		rep, err := dr.CheckDomainReputation(d.Domain)
		if err != nil {
			dr.Logger.Warnw("reputation check failed", "domain", d.Domain, "error", err)
			continue
		}

		dr.mu.Lock()
		if i < len(dr.config.DomainPool) {
			dr.config.DomainPool[i].Reputation = rep
		}
		dr.mu.Unlock()

		// if active domain is blacklisted, trigger auto-rotation
		if d.Status == "active" && rep.IsBlacklisted && dr.config.AutoRotateOnBlock {
			dr.Logger.Warnw("active domain blacklisted, triggering rotation",
				"domain", d.Domain,
				"score", rep.Score,
			)

			reason := fmt.Sprintf("domain '%s' blacklisted (score: %d)", d.Domain, rep.Score)
			if _, err := dr.TriggerAutoRotation(reason); err != nil {
				dr.Logger.Errorw("auto-rotation failed", "error", err)
			}
		}

		// small delay between checks to avoid rate limiting
		time.Sleep(2 * time.Second)
	}

	// save updated reputations
	dr.mu.Lock()
	dr.saveConfigToDB()
	dr.mu.Unlock()
}

// Stop stops the reputation monitoring loop
func (dr *DomainRotator) Stop() {
	close(dr.stopChan)
}

// ============================================================================
// Subdomain Generation (from Evilginx blacklist_evasion.go)
// ============================================================================

// GenerateSubdomain generates a random-looking subdomain for a base domain
func (dr *DomainRotator) GenerateSubdomain(baseDomain string) string {
	// Common patterns that look legitimate
	prefixes := []string{
		"auth", "login", "secure", "account", "verify", "portal",
		"mail", "webmail", "sso", "id", "identity", "signin",
		"app", "my", "access", "connect", "cloud",
	}

	suffixes := []string{
		"", "-v2", "-prod", "-us", "-eu", "-01", "-web", "-api",
	}

	// Simple deterministic selection based on domain hash
	hash := 0
	for _, c := range baseDomain {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}

	prefix := prefixes[hash%len(prefixes)]
	suffix := suffixes[(hash/len(prefixes))%len(suffixes)]

	return fmt.Sprintf("%s%s.%s", prefix, suffix, baseDomain)
}

// GetDomainAge returns how long a domain has been in the pool
func (dr *DomainRotator) GetDomainAge(domain string) time.Duration {
	for _, d := range dr.config.DomainPool {
		if d.Domain == domain {
			return time.Since(d.AddedAt)
		}
	}
	return 0
}

// GetActiveDomain returns the currently active domain
func (dr *DomainRotator) GetActiveDomain() string {
	for _, d := range dr.config.DomainPool {
		if d.Status == "active" {
			return d.Domain
		}
	}
	return ""
}

// GetAvailableDomainCount returns the number of standby domains
func (dr *DomainRotator) GetAvailableDomainCount() int {
	count := 0
	for _, d := range dr.config.DomainPool {
		if d.Status == "standby" {
			count++
		}
	}
	return count
}

// BurnDomain manually marks a domain as burned
func (dr *DomainRotator) BurnDomain(domain, reason string) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	for i, d := range dr.config.DomainPool {
		if d.Domain == domain {
			dr.config.DomainPool[i].Status = "burned"
			dr.config.DomainPool[i].BurnedAt = time.Now()
			dr.config.DomainPool[i].BurnReason = reason
			return dr.saveConfigToDB()
		}
	}
	return fmt.Errorf("domain '%s' not found in pool", domain)
}

// RecoverDomain moves a burned domain back to standby
func (dr *DomainRotator) RecoverDomain(domain string) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	for i, d := range dr.config.DomainPool {
		if d.Domain == domain && d.Status == "burned" {
			dr.config.DomainPool[i].Status = "standby"
			dr.config.DomainPool[i].BurnedAt = time.Time{}
			dr.config.DomainPool[i].BurnReason = ""
			return dr.saveConfigToDB()
		}
	}
	return fmt.Errorf("domain '%s' not found or not burned", domain)
}

// Helper to check if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
