package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// CookieExportFormat defines the format for cookie export
type CookieExportFormat string

const (
	CookieExportFormatJSON     CookieExportFormat = "json"
	CookieExportFormatNetscape CookieExportFormat = "netscape"
)

// ExportedCookie represents a cookie in browser extension format (Cookie Editor, EditThisCookie)
type ExportedCookie struct {
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Domain         string  `json:"domain"`
	Path           string  `json:"path"`
	ExpirationDate float64 `json:"expirationDate,omitempty"`
	HttpOnly       bool    `json:"httpOnly"`
	Secure         bool    `json:"secure"`
	SameSite       string  `json:"sameSite,omitempty"`
	HostOnly       bool    `json:"hostOnly"`
	Session        bool    `json:"session"`
	StoreId        string  `json:"storeId,omitempty"`
}

// CookieExport provides cookie export functionality.
// It is stateless and works purely in-memory; no filesystem writes.
type CookieExport struct {
	Common
}

// NewCookieExportService creates a new cookie export service
func NewCookieExportService(logger *zap.SugaredLogger) *CookieExport {
	return &CookieExport{
		Common: Common{
			Logger: logger,
		},
	}
}

// ExportCookiesFromCapturedData converts captured cookie data from a proxy session
// into a browser-importable format. The capturedData is expected to be the cookie
// bundle from ProxyHandler.createCookieBundle().
// Returns filename, content bytes, and error.
func (ce *CookieExport) ExportCookiesFromCapturedData(
	capturedData map[string]interface{},
	targetDomain string,
	sessionID string,
	format CookieExportFormat,
) (string, []byte, error) {

	cookies := ce.extractCookiesFromCapturedData(capturedData, targetDomain)

	if len(cookies) == 0 {
		return "", nil, fmt.Errorf("no cookies found in captured data")
	}

	var content []byte
	var ext string
	var err error

	switch format {
	case CookieExportFormatNetscape:
		content, err = ce.formatAsNetscape(cookies)
		ext = "txt"
	default:
		content, err = ce.formatAsJSON(cookies)
		ext = "json"
	}

	if err != nil {
		return "", nil, fmt.Errorf("failed to format cookies: %w", err)
	}

	// generate filename
	sanitizedDomain := strings.ReplaceAll(targetDomain, ".", "_")
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	filename := fmt.Sprintf("cookies_%s_%s.%s", sanitizedDomain, shortID, ext)

	return filename, content, nil
}

// extractCookiesFromCapturedData extracts cookies from the captured data map
func (ce *CookieExport) extractCookiesFromCapturedData(
	capturedData map[string]interface{},
	targetDomain string,
) []ExportedCookie {

	var cookies []ExportedCookie

	// the captured data from PhishingClub's cookie bundle has this structure:
	// { "cookies": { "captureName": { "cookieName": "cookieValue", ... } }, ... }
	cookiesRaw, ok := capturedData["cookies"]
	if !ok {
		// try flat structure: { "cookieName": "cookieValue", ... }
		cookies = ce.extractFlatCookies(capturedData, targetDomain)
		return cookies
	}

	cookiesMap, ok := cookiesRaw.(map[string]interface{})
	if !ok {
		return cookies
	}

	for _, captureData := range cookiesMap {
		captureMap, ok := captureData.(map[string]interface{})
		if !ok {
			if captureStrMap, ok := captureData.(map[string]string); ok {
				for name, value := range captureStrMap {
					cookies = append(cookies, ce.createExportedCookie(name, value, targetDomain))
				}
			}
			continue
		}

		for name, value := range captureMap {
			valueStr := fmt.Sprintf("%v", value)
			cookies = append(cookies, ce.createExportedCookie(name, valueStr, targetDomain))
		}
	}

	return cookies
}

// extractFlatCookies extracts cookies from a flat key-value map
func (ce *CookieExport) extractFlatCookies(data map[string]interface{}, targetDomain string) []ExportedCookie {
	var cookies []ExportedCookie

	// skip metadata keys
	skipKeys := map[string]bool{
		"capture_type":     true,
		"cookie_count":     true,
		"bundle_time":      true,
		"target_domain":    true,
		"session_complete": true,
	}

	for key, value := range data {
		if skipKeys[key] {
			continue
		}
		valueStr := fmt.Sprintf("%v", value)
		cookies = append(cookies, ce.createExportedCookie(key, valueStr, targetDomain))
	}

	return cookies
}

// createExportedCookie creates an ExportedCookie with sensible defaults
func (ce *CookieExport) createExportedCookie(name, value, domain string) ExportedCookie {
	cookie := ExportedCookie{
		Name:     name,
		Value:    value,
		Domain:   domain,
		Path:     "/",
		HttpOnly: false,
		StoreId:  "0",
		SameSite: "no_restriction",
		Secure:   true, // default to secure for captured auth cookies
	}

	// detect host-only based on domain format
	if len(domain) > 0 && domain[0] == '.' {
		cookie.HostOnly = false
	} else {
		cookie.HostOnly = true
	}

	// set a long expiration (5 years) for captured cookies
	cookie.Session = false
	cookie.ExpirationDate = float64(time.Now().Add(5 * 365 * 24 * time.Hour).Unix())

	return cookie
}

// formatAsJSON formats cookies as a JSON array compatible with browser extensions
func (ce *CookieExport) formatAsJSON(cookies []ExportedCookie) ([]byte, error) {
	return json.MarshalIndent(cookies, "", "    ")
}

// formatAsNetscape formats cookies in Netscape/Mozilla cookie file format
func (ce *CookieExport) formatAsNetscape(cookies []ExportedCookie) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString("# Netscape HTTP Cookie File\n")
	sb.WriteString("# https://curl.se/docs/http-cookies.html\n")
	sb.WriteString("# This file was generated by Phishing Club\n\n")

	for _, c := range cookies {
		includeSubdomains := "FALSE"
		if !c.HostOnly {
			includeSubdomains = "TRUE"
		}

		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}

		expiry := int64(c.ExpirationDate)
		if c.Session {
			expiry = 0
		}

		domain := c.Domain
		if !c.HostOnly && !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}

		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			domain,
			includeSubdomains,
			c.Path,
			secure,
			expiry,
			c.Name,
			c.Value,
		))
	}

	return []byte(sb.String()), nil
}
