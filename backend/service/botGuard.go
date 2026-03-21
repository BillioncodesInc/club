package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BotGuardConfig holds configuration for bot detection
type BotGuardConfig struct {
	Enabled              bool    `json:"enabled"`
	StrictMode           bool    `json:"strictMode"`
	RequireJS            bool    `json:"requireJS"`
	MaxRequestsPerMinute int     `json:"maxRequestsPerMinute"`
	ThreatScoreThreshold int     `json:"threatScoreThreshold"`
	ChallengeEnabled     bool    `json:"challengeEnabled"`
	FingerprintEnabled   bool    `json:"fingerprintEnabled"`
	BehaviorAnalysis     bool    `json:"behaviorAnalysis"`
	RateLimitBurst       int     `json:"rateLimitBurst"`
	SessionTimeout       float64 `json:"sessionTimeoutMinutes"`
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

// BotGuard provides comprehensive bot detection and anti-automation protection
type BotGuard struct {
	Common
	Logger     *zap.SugaredLogger
	config     *BotGuardConfig
	sessions   map[string]*BotSession
	challenges map[string]*JSChallenge
	mu         sync.RWMutex
}

// DefaultBotGuardConfig returns sensible defaults
func DefaultBotGuardConfig() *BotGuardConfig {
	return &BotGuardConfig{
		Enabled:              false,
		StrictMode:           false,
		RequireJS:            true,
		MaxRequestsPerMinute: 60,
		ThreatScoreThreshold: 50,
		ChallengeEnabled:     true,
		FingerprintEnabled:   true,
		BehaviorAnalysis:     true,
		RateLimitBurst:       10,
		SessionTimeout:       30,
	}
}

// NewBotGuardService creates a new BotGuard service
func NewBotGuardService(logger *zap.SugaredLogger) *BotGuard {
	return &BotGuard{
		Logger:     logger,
		config:     DefaultBotGuardConfig(),
		sessions:   make(map[string]*BotSession),
		challenges: make(map[string]*JSChallenge),
	}
}

// CheckRequest evaluates an HTTP request for bot indicators
func (bg *BotGuard) CheckRequest(r *http.Request) *BotCheckResult {
	if !bg.config.Enabled {
		return &BotCheckResult{Allowed: true, ThreatScore: 0, Reason: "disabled"}
	}

	ip := extractIP(r)
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
	if session.RequestCount > bg.config.MaxRequestsPerMinute {
		score += 30
		reasons = append(reasons, "rate_limit_exceeded")
	}

	// 2. User-Agent analysis
	uaScore, uaReason := bg.analyzeUserAgent(ua)
	score += uaScore
	if uaReason != "" {
		reasons = append(reasons, uaReason)
	}

	// 3. Header anomaly detection
	headerScore, headerReason := bg.analyzeHeaders(r)
	score += headerScore
	if headerReason != "" {
		reasons = append(reasons, headerReason)
	}

	// 4. Behavior analysis
	if bg.config.BehaviorAnalysis {
		behaviorScore, behaviorReason := bg.analyzeBehavior(session)
		score += behaviorScore
		if behaviorReason != "" {
			reasons = append(reasons, behaviorReason)
		}
	}

	// 5. JS verification check
	if bg.config.RequireJS && !session.JSVerified {
		score += 15
		reasons = append(reasons, "js_not_verified")
	}

	session.ThreatScore = score
	session.IsBot = score >= bg.config.ThreatScoreThreshold

	result := &BotCheckResult{
		Allowed:     !session.IsBot,
		ThreatScore: score,
		SessionID:   sessionID,
		Reason:      strings.Join(reasons, ", "),
	}

	// Issue JS challenge if score is borderline
	if bg.config.ChallengeEnabled && score >= bg.config.ThreatScoreThreshold/2 && score < bg.config.ThreatScoreThreshold && !session.JSVerified {
		challenge := bg.generateChallenge()
		result.Challenge = challenge.Script
	}

	return result
}

// VerifyChallenge verifies a JS challenge response
func (bg *BotGuard) VerifyChallenge(sessionID, challengeID, answer string) bool {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	challenge, ok := bg.challenges[challengeID]
	if !ok || time.Now().After(challenge.ExpiresAt) {
		delete(bg.challenges, challengeID)
		return false
	}

	if answer == challenge.Expected {
		if session, ok := bg.sessions[sessionID]; ok {
			session.JSVerified = true
			session.ThreatScore = max(0, session.ThreatScore-30)
		}
		delete(bg.challenges, challengeID)
		return true
	}
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

// UpdateConfig updates the BotGuard configuration
func (bg *BotGuard) UpdateConfig(cfg *BotGuardConfig) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.config = cfg
}

// GetSessionStats returns stats about tracked sessions
func (bg *BotGuard) GetSessionStats() map[string]interface{} {
	bg.mu.RLock()
	defer bg.mu.RUnlock()

	total := len(bg.sessions)
	bots := 0
	verified := 0
	for _, s := range bg.sessions {
		if s.IsBot {
			bots++
		}
		if s.JSVerified {
			verified++
		}
	}
	return map[string]interface{}{
		"totalSessions":    total,
		"detectedBots":     bots,
		"verifiedSessions": verified,
		"activeChallenges": len(bg.challenges),
	}
}

// CleanupExpired removes expired sessions and challenges
func (bg *BotGuard) CleanupExpired() {
	bg.mu.Lock()
	defer bg.mu.Unlock()

	timeout := time.Duration(bg.config.SessionTimeout) * time.Minute
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
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
