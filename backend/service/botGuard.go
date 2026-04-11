package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

// BotGuardConfig holds configuration for bot detection.
// Field names match the frontend UI exactly.
type BotGuardConfig struct {
	Enabled             bool   `json:"enabled"`
	JsChallenge         bool   `json:"jsChallenge"`
	BehaviorAnalysis    bool   `json:"behaviorAnalysis"`
	FingerprintCheck    bool   `json:"fingerprintCheck"`
	MinInteractionTime  int    `json:"minInteractionTime"`
	MaxRequestRate      int    `json:"maxRequestRate"`
	ChallengeDifficulty string `json:"challengeDifficulty"`
	BlockHeadless       bool   `json:"blockHeadless"`
	BlockTor            bool   `json:"blockTor"`
	BlockVPN            bool   `json:"blockVPN"`
	WhitelistedIPs      string `json:"whitelistedIPs"`
	// UseTurnstile enables Cloudflare Turnstile as an additional verification layer
	UseTurnstile bool `json:"useTurnstile"`
	// BlockSecurityCrawlers blocks known security scanner and GSB crawler IPs
	BlockSecurityCrawlers bool `json:"blockSecurityCrawlers"`
}

// BotSession tracks a visitor's behavior for bot detection
type BotSession struct {
	ID              string    `json:"id"`
	IP              string    `json:"ip"`
	FirstSeen       time.Time `json:"firstSeen"`
	LastSeen        time.Time `json:"lastSeen"`
	RequestCount    int       `json:"requestCount"`
	JSVerified      bool      `json:"jsVerified"`
	ChallengeResult string    `json:"challengeResult"`
	ThreatScore     int       `json:"threatScore"`
	UserAgent       string    `json:"userAgent"`
	Fingerprint     string    `json:"fingerprint"`
	IsBot           bool      `json:"isBot"`
	Reason          string    `json:"reason"`
	Counted         bool      `json:"-"` // whether this session has been counted in stats
	CountedAsBot    bool      `json:"-"` // tracks what it was counted as, to handle status changes
}

// BotCheckResult is the result of a bot check
type BotCheckResult struct {
	Allowed     bool   `json:"allowed"`
	ThreatScore int    `json:"threatScore"`
	Reason      string `json:"reason"`
	Challenge   string `json:"challenge,omitempty"`
	SessionID   string `json:"sessionId"`
}

// JSChallenge is a JavaScript challenge for bot verification
type JSChallenge struct {
	ID        string `json:"id"`
	Script    string `json:"script"`
	Expected  string `json:"-"`
	ExpiresAt time.Time
}

// BotGuardStats tracks cumulative statistics
type BotGuardStats struct {
	TotalSessions    int `json:"totalSessions"`
	PassedSessions   int `json:"passedSessions"`
	BlockedSessions  int `json:"blockedSessions"`
	ChallengeSent    int `json:"challengesSent"`
	ChallengePassed  int `json:"challengesPassed"`
	ChallengeFailed  int `json:"challengesFailed"`
}

// BotGuard provides comprehensive bot detection and anti-automation protection
type BotGuard struct {
	Common
	Logger           *zap.SugaredLogger
	OptionRepository *repository.Option
	TurnstileService *Turnstile
	config           *BotGuardConfig
	sessions         map[string]*BotSession
	challenges       map[string]*JSChallenge
	stats            BotGuardStats
	whitelistedNets  []*net.IPNet
	whitelistedIPs   []net.IP
	crawlerNets      []*net.IPNet // known security scanner/crawler CIDR ranges
	crawlerLastFetch time.Time    // last time dynamic crawler IPs were fetched
	mu               sync.RWMutex
}

// DefaultBotGuardConfig returns sensible defaults
func DefaultBotGuardConfig() *BotGuardConfig {
	return &BotGuardConfig{
		Enabled:             false,
		JsChallenge:         true,
		BehaviorAnalysis:    true,
		FingerprintCheck:    true,
		MinInteractionTime:  2000,
		MaxRequestRate:      30,
		ChallengeDifficulty: "medium",
		BlockHeadless:       true,
		BlockTor:            false,
		BlockVPN:            false,
		WhitelistedIPs:      "",
		UseTurnstile:        false,
		BlockSecurityCrawlers: true,
	}
}

// NewBotGuardService creates a new BotGuard service
func NewBotGuardService(logger *zap.SugaredLogger, optionRepo *repository.Option, turnstileService *Turnstile) *BotGuard {
	bg := &BotGuard{
		Logger:           logger,
		OptionRepository: optionRepo,
		TurnstileService: turnstileService,
		config:           DefaultBotGuardConfig(),
		sessions:         make(map[string]*BotSession),
		challenges:       make(map[string]*JSChallenge),
	}

	// load config from database
	bg.loadConfigFromDB()

	// initialize known security scanner/crawler IP ranges
	bg.loadStaticCrawlerNets()
	go bg.fetchDynamicCrawlerNets()

	return bg
}

// loadConfigFromDB loads the BotGuard configuration from the options table
func (bg *BotGuard) loadConfigFromDB() {
	ctx := context.Background()
	opt, err := bg.OptionRepository.GetByKey(ctx, data.OptionKeyBotGuardConfig)
	if err != nil {
		bg.Logger.Debugw("no bot guard config found, using defaults")
		return
	}

	var config BotGuardConfig
	if err := json.Unmarshal([]byte(opt.Value.String()), &config); err != nil {
		bg.Logger.Errorw("failed to unmarshal bot guard config", "error", err)
		return
	}

	bg.config = &config
	bg.parseWhitelist()
	bg.Logger.Infow("loaded bot guard config", "enabled", config.Enabled)
}

// parseWhitelist parses the whitelisted IPs/CIDRs from config
func (bg *BotGuard) parseWhitelist() {
	bg.whitelistedNets = nil
	bg.whitelistedIPs = nil

	if bg.config.WhitelistedIPs == "" {
		return
	}

	lines := strings.Split(bg.config.WhitelistedIPs, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "/") {
			_, cidr, err := net.ParseCIDR(line)
			if err == nil {
				bg.whitelistedNets = append(bg.whitelistedNets, cidr)
			}
		} else {
			ip := net.ParseIP(line)
			if ip != nil {
				bg.whitelistedIPs = append(bg.whitelistedIPs, ip)
			}
		}
	}
}

// isWhitelisted checks if an IP is in the whitelist
func (bg *BotGuard) isWhitelisted(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, wip := range bg.whitelistedIPs {
		if wip.Equal(ip) {
			return true
		}
	}

	for _, cidr := range bg.whitelistedNets {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// CheckRequest evaluates an HTTP request for bot indicators
func (bg *BotGuard) CheckRequest(r *http.Request) *BotCheckResult {
	if !bg.config.Enabled {
		return &BotCheckResult{Allowed: true, ThreatScore: 0, Reason: "disabled"}
	}

	ip := extractIP(r)

	// check whitelist first
	if bg.isWhitelisted(ip) {
		return &BotCheckResult{Allowed: true, ThreatScore: 0, Reason: "whitelisted"}
	}

	ua := r.UserAgent()
	sessionID := bg.getOrCreateSession(ip, ua)

	bg.mu.Lock()
	session := bg.sessions[sessionID]
	session.RequestCount++
	session.LastSeen = time.Now()
	bg.mu.Unlock()

	score := 0
	reasons := []string{}

	// 1. Rate limiting check
	if session.RequestCount > bg.config.MaxRequestRate {
		score += 30
		reasons = append(reasons, "rate_limit_exceeded")
	}

	// 2. User-Agent analysis
	uaScore, uaReason := bg.analyzeUserAgent(ua)
	score += uaScore
	if uaReason != "" {
		reasons = append(reasons, uaReason)
	}

	// 3. Headless browser detection
	if bg.config.BlockHeadless {
		headlessScore, headlessReason := bg.detectHeadless(ua, r)
		score += headlessScore
		if headlessReason != "" {
			reasons = append(reasons, headlessReason)
		}
	}

	// 4. Header anomaly detection
	headerScore, headerReason := bg.analyzeHeaders(r)
	score += headerScore
	if headerReason != "" {
		reasons = append(reasons, headerReason)
	}

	// 5. Behavior analysis
	if bg.config.BehaviorAnalysis {
		behaviorScore, behaviorReason := bg.analyzeBehavior(session)
		score += behaviorScore
		if behaviorReason != "" {
			reasons = append(reasons, behaviorReason)
		}
	}

	// 6. JS verification check
	if bg.config.JsChallenge && !session.JSVerified {
		score += 15
		reasons = append(reasons, "js_not_verified")
	}

	// 7. Minimum interaction time check
	if bg.config.MinInteractionTime > 0 {
		elapsed := time.Since(session.FirstSeen)
		if elapsed < time.Duration(bg.config.MinInteractionTime)*time.Millisecond && session.RequestCount > 3 {
			score += 20
			reasons = append(reasons, "too_fast_interaction")
		}
	}

	// 8. Known security crawler/scanner IP check
	if bg.config.BlockSecurityCrawlers {
		crawlerScore, crawlerReason := bg.checkCrawlerIP(ip)
		score += crawlerScore
		if crawlerReason != "" {
			reasons = append(reasons, crawlerReason)
		}
	}

	// determine threshold based on challenge difficulty
	threshold := bg.getThreshold()

	session.ThreatScore = score
	session.IsBot = score >= threshold

	// update stats - only count each session once; handle status changes
	bg.mu.Lock()
	if !session.Counted {
		// First evaluation for this session
		session.Counted = true
		session.CountedAsBot = session.IsBot
		bg.stats.TotalSessions++
		if session.IsBot {
			bg.stats.BlockedSessions++
		} else {
			bg.stats.PassedSessions++
		}
	} else if session.CountedAsBot != session.IsBot {
		// Session status changed (e.g., passed JS challenge)
		if session.IsBot {
			bg.stats.BlockedSessions++
			bg.stats.PassedSessions--
		} else {
			bg.stats.PassedSessions++
			bg.stats.BlockedSessions--
		}
		session.CountedAsBot = session.IsBot
	}
	bg.mu.Unlock()

	result := &BotCheckResult{
		Allowed:     !session.IsBot,
		ThreatScore: score,
		SessionID:   sessionID,
		Reason:      strings.Join(reasons, ", "),
	}

	// Issue JS challenge if score is borderline
	if bg.config.JsChallenge && score >= threshold/2 && score < threshold && !session.JSVerified {
		challenge := bg.generateChallenge()
		result.Challenge = challenge.Script
		bg.mu.Lock()
		bg.stats.ChallengeSent++
		bg.mu.Unlock()
	}

	return result
}

// getThreshold returns the threat score threshold based on difficulty
func (bg *BotGuard) getThreshold() int {
	switch bg.config.ChallengeDifficulty {
	case "low":
		return 70
	case "high":
		return 30
	default: // "medium"
		return 50
	}
}

// detectHeadless checks for headless browser indicators
func (bg *BotGuard) detectHeadless(ua string, r *http.Request) (int, string) {
	lower := strings.ToLower(ua)

	headlessPatterns := []string{
		"headlesschrome", "headless", "phantomjs",
		"selenium", "puppeteer", "playwright",
		"webdriver",
	}

	for _, pattern := range headlessPatterns {
		if strings.Contains(lower, pattern) {
			return 60, "headless_browser:" + pattern
		}
	}

	// Check for webdriver header
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		// Not necessarily headless, but worth noting
	}

	return 0, ""
}

// VerifyChallenge verifies a JS challenge response
func (bg *BotGuard) VerifyChallenge(sessionID, challengeID, answer string) bool {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	challenge, ok := bg.challenges[challengeID]
	if !ok || time.Now().After(challenge.ExpiresAt) {
		delete(bg.challenges, challengeID)
		bg.stats.ChallengeFailed++
		return false
	}

	if answer == challenge.Expected {
		if session, ok := bg.sessions[sessionID]; ok {
			session.JSVerified = true
			session.ThreatScore = maxInt(0, session.ThreatScore-30)
		}
		delete(bg.challenges, challengeID)
		bg.stats.ChallengePassed++
		return true
	}

	bg.stats.ChallengeFailed++
	return false
}

// VerifyFingerprint stores a device fingerprint for a session
func (bg *BotGuard) VerifyFingerprint(sessionID, fingerprint string) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if session, ok := bg.sessions[sessionID]; ok {
		session.Fingerprint = fingerprint
	}
}

// GetConfig returns the current BotGuard configuration
func (bg *BotGuard) GetConfig() *BotGuardConfig {
	return bg.config
}

// UpdateConfig updates the BotGuard configuration and persists to DB
func (bg *BotGuard) UpdateConfig(cfg *BotGuardConfig) error {
	bg.mu.Lock()
	bg.config = cfg
	bg.parseWhitelist()
	bg.mu.Unlock()

	// persist to database
	if bg.OptionRepository != nil {
		jsonData, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal bot guard config: %w", err)
		}
		ctx := context.Background()
		if err := bg.OptionRepository.UpsertByKey(ctx, data.OptionKeyBotGuardConfig, string(jsonData)); err != nil {
			return fmt.Errorf("failed to save bot guard config: %w", err)
		}
	}

	bg.Logger.Infow("updated bot guard config", "enabled", cfg.Enabled)
	return nil
}

// GetSessionStats returns stats about tracked sessions
func (bg *BotGuard) GetSessionStats() map[string]interface{} {
	bg.mu.RLock()
	defer bg.mu.RUnlock()

	return map[string]interface{}{
		"totalSessions":    bg.stats.TotalSessions,
		"passedSessions":   bg.stats.PassedSessions,
		"blockedSessions":  bg.stats.BlockedSessions,
		"challengesSent":   bg.stats.ChallengeSent,
		"challengesPassed": bg.stats.ChallengePassed,
		"challengesFailed": bg.stats.ChallengeFailed,
	}
}

// CleanupExpired removes expired sessions and challenges
func (bg *BotGuard) CleanupExpired() {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	timeout := 30 * time.Minute
	now := time.Now()

	for id, s := range bg.sessions {
		if now.Sub(s.LastSeen) > timeout {
			delete(bg.sessions, id)
		}
	}
	for id, c := range bg.challenges {
		if now.After(c.ExpiresAt) {
			delete(bg.challenges, id)
		}
	}
}

// ShouldUseTurnstile returns whether Turnstile should be used as additional verification
func (bg *BotGuard) ShouldUseTurnstile() bool {
	return bg.config.Enabled && bg.config.UseTurnstile && bg.TurnstileService != nil && bg.TurnstileService.IsEnabled()
}

// --- internal helpers ---

func (bg *BotGuard) getOrCreateSession(ip, ua string) string {
	hash := sha256.Sum256([]byte(ip + "|" + ua))
	sessionID := hex.EncodeToString(hash[:16])

	bg.mu.Lock()
	defer bg.mu.Unlock()

	if _, ok := bg.sessions[sessionID]; !ok {
		bg.sessions[sessionID] = &BotSession{
			ID:        sessionID,
			IP:        ip,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			UserAgent: ua,
		}
	}
	return sessionID
}

func (bg *BotGuard) analyzeUserAgent(ua string) (int, string) {
	lower := strings.ToLower(ua)

	// Empty or missing UA
	if ua == "" {
		return 40, "empty_user_agent"
	}

	// Known bot patterns
	botPatterns := []string{
		"bot", "crawler", "spider", "scraper", "curl", "wget",
		"python-requests", "go-http-client", "httpie", "postman",
		"phantomjs", "headlesschrome", "selenium", "puppeteer",
		"nessus", "nikto", "nmap", "masscan", "qualys", "acunetix",
		"burpsuite", "zap", "sqlmap", "dirbuster", "gobuster",
	}
	for _, pattern := range botPatterns {
		if strings.Contains(lower, pattern) {
			return 50, "known_bot_ua:" + pattern
		}
	}

	// Security vendor patterns
	securityPatterns := []string{
		"barracuda", "proofpoint", "mimecast", "fireeye", "sophos",
		"forcepoint", "ironport", "messagelabs", "symantec",
	}
	for _, pattern := range securityPatterns {
		if strings.Contains(lower, pattern) {
			return 60, "security_vendor_ua:" + pattern
		}
	}

	return 0, ""
}

func (bg *BotGuard) analyzeHeaders(r *http.Request) (int, string) {
	score := 0
	reason := ""

	// Missing common headers
	if r.Header.Get("Accept") == "" {
		score += 5
	}
	if r.Header.Get("Accept-Language") == "" {
		score += 10
		reason = "missing_accept_language"
	}
	if r.Header.Get("Accept-Encoding") == "" {
		score += 5
	}

	// Suspicious header combinations
	if r.Header.Get("X-Forwarded-For") != "" && r.Header.Get("Via") != "" {
		score += 10
		reason = "proxy_headers_detected"
	}

	return score, reason
}

func (bg *BotGuard) analyzeBehavior(session *BotSession) (int, string) {
	elapsed := time.Since(session.FirstSeen)

	// Too many requests too fast
	if elapsed < time.Second*10 && session.RequestCount > 20 {
		return 40, "rapid_fire_requests"
	}

	// Perfectly regular intervals (bot-like)
	if elapsed > time.Minute && session.RequestCount > 10 {
		avgInterval := elapsed / time.Duration(session.RequestCount)
		if avgInterval > 900*time.Millisecond && avgInterval < 1100*time.Millisecond {
			return 25, "regular_interval_pattern"
		}
	}

	return 0, ""
}

func (bg *BotGuard) generateChallenge() *JSChallenge {
	a, _ := rand.Int(rand.Reader, big.NewInt(1000))
	b, _ := rand.Int(rand.Reader, big.NewInt(1000))
	expected := a.Int64() + b.Int64()

	id := generateRandomID(16)
	challenge := &JSChallenge{
		ID:        id,
		Script:    fmt.Sprintf("(function(){return %d+%d;})()", a.Int64(), b.Int64()),
		Expected:  fmt.Sprintf("%d", expected),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	bg.mu.Lock()
	bg.challenges[id] = challenge
	bg.mu.Unlock()

	return challenge
}

func extractIP(r *http.Request) string {
	// prefer CF-Connecting-IP for Cloudflare setups
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		// use the rightmost non-private IP for security
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			parsed := net.ParseIP(ip)
			if parsed != nil && !parsed.IsPrivate() && !parsed.IsLoopback() {
				return ip
			}
		}
		// fallback to first IP
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func generateRandomID(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─── Security Crawler / GSB Scanner IP Detection ──────────────────────────────

// checkCrawlerIP checks if the given IP belongs to a known security scanner or crawler network.
// Returns a high score if matched, effectively blocking the request.
func (bg *BotGuard) checkCrawlerIP(ipStr string) (int, string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0, ""
	}

	bg.mu.RLock()
	defer bg.mu.RUnlock()

	for _, cidr := range bg.crawlerNets {
		if cidr.Contains(ip) {
			return 100, "security_crawler_ip:" + cidr.String()
		}
	}

	// Also check via reverse DNS for Google crawlers
	if bg.isGoogleCrawler(ipStr) {
		return 100, "google_crawler_rdns:" + ipStr
	}

	return 0, ""
}

// isGoogleCrawler performs reverse DNS lookup to detect Google crawlers.
// Google crawlers resolve to *.googlebot.com or *.google.com hostnames.
func (bg *BotGuard) isGoogleCrawler(ipStr string) bool {
	names, err := net.LookupAddr(ipStr)
	if err != nil || len(names) == 0 {
		return false
	}

	hostname := strings.ToLower(strings.TrimSuffix(names[0], "."))

	// Check if hostname belongs to Google crawler domains
	googleDomains := []string{
		".googlebot.com",
		".google.com",
		".googleusercontent.com",
	}

	for _, domain := range googleDomains {
		if strings.HasSuffix(hostname, domain) {
			// Verify with forward DNS to prevent spoofing
			addrs, err := net.LookupHost(names[0])
			if err != nil {
				return false
			}
			for _, addr := range addrs {
				if addr == ipStr {
					return true
				}
			}
		}
	}

	return false
}

// loadStaticCrawlerNets loads well-known security scanner IP ranges.
// These are CIDRs of services known to scan for phishing pages.
func (bg *BotGuard) loadStaticCrawlerNets() {
	// Known security scanner and anti-phishing service IP ranges
	staticCIDRs := []string{
		// --- Microsoft Defender / SmartScreen ---
		"40.76.0.0/14",
		"40.80.0.0/12",
		"40.96.0.0/12",
		"40.112.0.0/13",
		"40.120.0.0/14",
		"40.124.0.0/16",
		"40.125.0.0/17",
		"52.96.0.0/12",
		"52.112.0.0/14",
		"52.120.0.0/14",
		"52.132.0.0/14",
		"52.136.0.0/13",
		"52.145.0.0/16",
		"52.146.0.0/15",
		"52.148.0.0/14",
		"52.152.0.0/13",
		"52.160.0.0/11",

		// --- Proofpoint URL Defense ---
		"67.231.148.0/22",
		"67.231.152.0/22",
		"148.163.128.0/17",

		// --- Barracuda Networks ---
		"64.235.144.0/20",
		"209.222.80.0/21",

		// --- Mimecast ---
		"91.220.42.0/24",
		"207.211.30.0/24",

		// --- PhishTank / OpenDNS ---
		"208.67.216.0/21",

		// --- VirusTotal scanners ---
		"74.125.0.0/16",

		// --- Mandiant / FireEye ---
		"34.68.34.64/27",
		"8.34.210.32/27",

		// NOTE: Cloudflare CIDRs intentionally excluded - they are CDN edge IPs,
		// not scanner IPs. Including them would block all traffic when proxy is
		// not behind CF but receives CF-routed scanner traffic.

		// --- Sophos ---
		"62.73.128.0/18",

		// --- Forcepoint ---
		"15.230.56.0/24",

		// --- URLScan.io ---
		"54.187.174.169/32",

		// --- Sucuri ---
		"192.88.134.0/23",
		"185.93.228.0/22",

		// --- Netcraft ---
		"194.72.238.0/24",
		"46.37.160.0/19",

		// --- ESET ---
		"91.228.166.0/23",
		"91.228.167.0/24",

		// --- Kaspersky ---
		"77.74.176.0/21",
		"93.159.228.0/22",

		// --- Trend Micro ---
		"150.70.0.0/16",
		"216.104.0.0/16",
	}

	bg.mu.Lock()
	defer bg.mu.Unlock()

	bg.crawlerNets = make([]*net.IPNet, 0, len(staticCIDRs))
	for _, cidr := range staticCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			bg.Logger.Warnw("failed to parse static crawler CIDR", "cidr", cidr, "error", err)
			continue
		}
		bg.crawlerNets = append(bg.crawlerNets, ipNet)
	}

	bg.Logger.Infow("loaded static crawler IP ranges", "count", len(bg.crawlerNets))
}

// fetchDynamicCrawlerNets fetches Google's published crawler IP ranges
// from the official JSON endpoints and adds them to the crawler nets list.
// This runs on startup and refreshes every 24 hours.
func (bg *BotGuard) fetchDynamicCrawlerNets() {
	bg.refreshGoogleCrawlerIPs()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		bg.refreshGoogleCrawlerIPs()
	}
}

// refreshGoogleCrawlerIPs fetches the latest Google crawler IP ranges
func (bg *BotGuard) refreshGoogleCrawlerIPs() {
	endpoints := []string{
		"https://developers.google.com/static/search/apis/ipranges/googlebot.json",
		"https://developers.google.com/static/search/apis/ipranges/special-crawlers.json",
		"https://developers.google.com/static/search/apis/ipranges/user-triggered-fetchers.json",
		"https://developers.google.com/static/search/apis/ipranges/user-triggered-fetchers-google.json",
		"https://developers.google.com/static/search/apis/ipranges/user-triggered-agents.json",
	}

	var dynamicNets []*net.IPNet
	client := &http.Client{Timeout: 15 * time.Second}

	for _, endpoint := range endpoints {
		nets, err := bg.fetchIPRangesFromJSON(client, endpoint)
		if err != nil {
			bg.Logger.Warnw("failed to fetch Google crawler IPs",
				"endpoint", endpoint, "error", err)
			continue
		}
		dynamicNets = append(dynamicNets, nets...)
	}

	if len(dynamicNets) > 0 {
		bg.mu.Lock()
		// rebuild: static + dynamic
		bg.loadStaticCrawlerNetsLocked()
		bg.crawlerNets = append(bg.crawlerNets, dynamicNets...)
		bg.crawlerLastFetch = time.Now()
		bg.mu.Unlock()

		bg.Logger.Infow("refreshed Google crawler IP ranges",
			"dynamicCount", len(dynamicNets),
			"totalCount", len(bg.crawlerNets))
	}
}

// loadStaticCrawlerNetsLocked reloads static CIDRs while holding the write lock.
// Called internally by refreshGoogleCrawlerIPs to rebuild the full list.
func (bg *BotGuard) loadStaticCrawlerNetsLocked() {
	staticCIDRs := []string{
		"40.76.0.0/14", "40.80.0.0/12", "40.96.0.0/12", "40.112.0.0/13",
		"40.120.0.0/14", "40.124.0.0/16", "40.125.0.0/17",
		"52.96.0.0/12", "52.112.0.0/14", "52.120.0.0/14", "52.132.0.0/14",
		"52.136.0.0/13", "52.145.0.0/16", "52.146.0.0/15", "52.148.0.0/14",
		"52.152.0.0/13", "52.160.0.0/11",
		"67.231.148.0/22", "67.231.152.0/22", "148.163.128.0/17",
		"64.235.144.0/20", "209.222.80.0/21",
		"91.220.42.0/24", "207.211.30.0/24",
		"208.67.216.0/21", "74.125.0.0/16",
		"34.68.34.64/27", "8.34.210.32/27",
		// NOTE: Cloudflare CIDRs intentionally excluded (see loadStaticCrawlerNets)
		"62.73.128.0/18", "15.230.56.0/24",
		"54.187.174.169/32",
		"192.88.134.0/23", "185.93.228.0/22",
		"194.72.238.0/24", "46.37.160.0/19",
		"91.228.166.0/23", "91.228.167.0/24",
		"77.74.176.0/21", "93.159.228.0/22",
		"150.70.0.0/16", "216.104.0.0/16",
	}

	bg.crawlerNets = make([]*net.IPNet, 0, len(staticCIDRs)+200)
	for _, cidr := range staticCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		bg.crawlerNets = append(bg.crawlerNets, ipNet)
	}
}

// googleIPRangesResponse represents the JSON structure returned by Google's IP range endpoints
type googleIPRangesResponse struct {
	Prefixes []googleIPPrefix `json:"prefixes"`
}

type googleIPPrefix struct {
	IPv4Prefix string `json:"ipv4Prefix,omitempty"`
	IPv6Prefix string `json:"ipv6Prefix,omitempty"`
}

// fetchIPRangesFromJSON fetches IP ranges from a Google JSON endpoint
func (bg *BotGuard) fetchIPRangesFromJSON(client *http.Client, endpoint string) ([]*net.IPNet, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result googleIPRangesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var nets []*net.IPNet
	for _, prefix := range result.Prefixes {
		cidr := prefix.IPv4Prefix
		if cidr == "" {
			cidr = prefix.IPv6Prefix
		}
		if cidr == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		nets = append(nets, ipNet)
	}

	return nets, nil
}
