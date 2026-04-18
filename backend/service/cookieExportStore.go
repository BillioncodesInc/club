package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportFromCookieStore exports cookies from a CookieStore's raw CookiesJSON.
// The cookiesJSON is expected to be a JSON array of cookie objects.
// Returns filename, content bytes, and error.
func (ce *CookieExport) ExportFromCookieStore(
	cookiesJSON string,
	storeName string,
	storeID string,
	format CookieExportFormat,
) (string, []byte, error) {
	if cookiesJSON == "" {
		return "", nil, fmt.Errorf("no cookies found in store")
	}

	// Parse the raw cookie JSON into ExportedCookie slice
	var rawCookies []map[string]interface{}
	if err := json.Unmarshal([]byte(cookiesJSON), &rawCookies); err != nil {
		return "", nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	if len(rawCookies) == 0 {
		return "", nil, fmt.Errorf("no cookies found in store")
	}

	cookies := make([]ExportedCookie, 0, len(rawCookies))
	for _, raw := range rawCookies {
		c := ExportedCookie{
			Path:     "/",
			Secure:   false,
			HttpOnly: false,
			SameSite: "unspecified",
			HostOnly: false,
			Session:  true,
			StoreId:  "0",
		}
		if name, ok := raw["name"].(string); ok {
			c.Name = name
		}
		if value, ok := raw["value"].(string); ok {
			c.Value = value
		}
		if domain, ok := raw["domain"].(string); ok {
			c.Domain = domain
		}
		if path, ok := raw["path"].(string); ok && path != "" {
			c.Path = path
		}
		if secure, ok := raw["secure"].(bool); ok {
			c.Secure = secure
		}
		if httpOnly, ok := raw["httpOnly"].(bool); ok {
			c.HttpOnly = httpOnly
		}
		if sameSite, ok := raw["sameSite"].(string); ok && sameSite != "" {
			c.SameSite = sameSite
		}
		if hostOnly, ok := raw["hostOnly"].(bool); ok {
			c.HostOnly = hostOnly
		}
		if exp, ok := raw["expirationDate"].(float64); ok && exp > 0 {
			c.ExpirationDate = exp
			c.Session = false
		}
		if c.Name != "" && c.Value != "" {
			cookies = append(cookies, c)
		}
	}

	if len(cookies) == 0 {
		return "", nil, fmt.Errorf("no valid cookies found in store")
	}

	var content []byte
	var ext string
	var err error

	switch format {
	case CookieExportFormatNetscape:
		content, err = ce.formatAsNetscape(cookies)
		ext = "txt"
	case CookieExportFormatHeader:
		content, err = ce.formatAsHeader(cookies)
		ext = "txt"
	case CookieExportFormatConsole:
		content, err = ce.formatAsConsole(cookies)
		ext = "js"
	default:
		content, err = ce.formatAsJSON(cookies)
		ext = "json"
	}

	if err != nil {
		return "", nil, fmt.Errorf("failed to format cookies: %w", err)
	}

	// Generate filename
	sanitizedName := strings.ReplaceAll(storeName, " ", "_")
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "_")
	shortID := storeID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	filename := fmt.Sprintf("cookies_%s_%s.%s", sanitizedName, shortID, ext)

	return filename, content, nil
}
