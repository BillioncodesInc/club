package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
)

// BrowserSessionService handles browser-based cookie validation and token acquisition.
type BrowserSessionService struct {
	Logger *zap.SugaredLogger
	mu     sync.Mutex
}

// NewBrowserSessionService creates a new BrowserSessionService
func NewBrowserSessionService(logger *zap.SugaredLogger) *BrowserSessionService {
	return &BrowserSessionService{
		Logger: logger,
	}
}

// BrowserSessionResult holds the result of a browser-based session validation
type BrowserSessionResult struct {
	Valid       bool   `json:"valid"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	AccessToken string `json:"accessToken"`
	Error       string `json:"error,omitempty"`
}

// cookieEntry represents a single cookie from the stored JSON
type cookieEntry struct {
	Name           string      `json:"name"`
	Value          string      `json:"value"`
	Domain         string      `json:"domain"`
	Path           string      `json:"path"`
	Secure         interface{} `json:"secure"`
	HttpOnly       interface{} `json:"httpOnly"`
	SameSite       string      `json:"sameSite"`
	ExpirationDate interface{} `json:"expirationDate"`
}

// launchBrowser creates and returns a headless Chrome browser instance
func (b *BrowserSessionService) launchBrowser(ctx context.Context, timeout time.Duration) (*rod.Browser, func(), error) {
	chromePath := os.Getenv("CHROME_PATH")
	if chromePath == "" {
		chromePath = "/usr/bin/chromium"
	}

	l := launcher.New().
		Bin(chromePath).
		Headless(true).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars", "").
		Set("no-first-run", "").
		Set("no-default-browser-check", "").
		Set("disable-gpu", "").
		Set("window-size", "1920,1080").
		NoSandbox(true)

	wsURL, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to launch Chrome: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Chrome: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	browser = browser.Context(timeoutCtx)

	cleanup := func() {
		cancel()
		browser.Close()
	}

	return browser, cleanup, nil
}

// establishSSOSession injects cookies, navigates to login.live.com, then to Outlook
// Returns the page with Outlook loaded and the intercepted Bearer token
func (b *BrowserSessionService) establishSSOSession(browser *rod.Browser, cookies []cookieEntry) (*rod.Page, string, string, string, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies
	b.Logger.Infow("injecting cookies", "count", len(cookies))
	if err := b.injectCookies(page, cookies); err != nil {
		b.Logger.Warnw("some cookies failed to inject", "error", err)
	}

	// Set up network interception to capture Bearer tokens from Outlook API calls
	var capturedToken string
	var capturedSubstrateToken string
	var capturedEmail string
	var capturedDisplayName string
	var tokenMu sync.Mutex

	// Enable network events
	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		b.Logger.Warnw("failed to enable network events", "error", err)
	}

	// Listen for network requests that contain Authorization: Bearer headers
	waitEvents := page.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			reqURL := e.Request.URL

			for key, val := range e.Request.Headers {
				if !strings.EqualFold(key, "Authorization") {
					continue
				}
				authVal := val.String()
				if !strings.HasPrefix(authVal, "Bearer ") {
					continue
				}
				bearerToken := strings.TrimPrefix(authVal, "Bearer ")

				// Log ALL Bearer tokens for debugging
				isJWT := strings.Contains(bearerToken, ".")
				tokenPrefix := bearerToken
				if len(tokenPrefix) > 30 {
					tokenPrefix = tokenPrefix[:30]
				}

				b.Logger.Debugw("intercepted Bearer token",
					"url", reqURL,
					"isJWT", isJWT,
					"tokenLen", len(bearerToken),
					"prefix", tokenPrefix,
				)

				if !isJWT {
					continue
				}

				tokenMu.Lock()
				// Prioritize Graph API tokens
				if strings.Contains(reqURL, "graph.microsoft.com") {
					capturedToken = bearerToken
					b.Logger.Infow("captured Graph API Bearer token", "url", reqURL, "tokenLen", len(bearerToken))
				} else if strings.Contains(reqURL, "substrate.office.com") && capturedSubstrateToken == "" {
					capturedSubstrateToken = bearerToken
					b.Logger.Infow("captured substrate Bearer token", "url", reqURL, "tokenLen", len(bearerToken))
				} else if capturedToken == "" {
					// Any other JWT from Outlook APIs
					isOutlookAPI := strings.Contains(reqURL, "outlook.office.com") ||
						strings.Contains(reqURL, "outlook.office365.com") ||
						(strings.Contains(reqURL, "outlook.live.com") && strings.Contains(reqURL, "/owa/"))
					if isOutlookAPI {
						capturedToken = bearerToken
						b.Logger.Infow("captured Outlook API Bearer token", "url", reqURL, "tokenLen", len(bearerToken))
					}
				}
				tokenMu.Unlock()
			}
		},
		func(e *proto.NetworkResponseReceived) {
			reqURL := e.Response.URL

			// Capture tokens from OAuth token endpoint responses
			isTokenEndpoint := strings.Contains(reqURL, "/oauth2/v2.0/token") ||
				strings.Contains(reqURL, "/consumers/oauth2/v2.0/token")

			if isTokenEndpoint {
				b.Logger.Infow("intercepted token endpoint response", "url", reqURL, "status", e.Response.Status)
				body, bodyErr := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
				if bodyErr != nil {
					b.Logger.Debugw("failed to get token response body", "error", bodyErr)
					return
				}

				var tokenResp struct {
					AccessToken string `json:"access_token"`
					ExpiresIn   int    `json:"expires_in"`
					Scope       string `json:"scope"`
				}
				if json.Unmarshal([]byte(body.Body), &tokenResp) == nil && tokenResp.AccessToken != "" {
					if strings.Contains(tokenResp.AccessToken, ".") {
						scope := strings.ToLower(tokenResp.Scope)
						tokenMu.Lock()
						// Prefer tokens with Mail scope
						if strings.Contains(scope, "mail") {
							capturedToken = tokenResp.AccessToken
							b.Logger.Infow("captured JWT with Mail scope from token endpoint",
								"scope", tokenResp.Scope,
								"expiresIn", tokenResp.ExpiresIn,
							)
						} else if capturedToken == "" {
							capturedToken = tokenResp.AccessToken
							b.Logger.Infow("captured JWT from token endpoint",
								"scope", tokenResp.Scope,
								"expiresIn", tokenResp.ExpiresIn,
							)
						}
						tokenMu.Unlock()
					}
				}
			}

			// Capture email from profile responses
			if strings.Contains(reqURL, "graph.microsoft.com") && strings.Contains(reqURL, "/me") {
				body, bodyErr := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
				if bodyErr == nil {
					var profile struct {
						Mail        string `json:"mail"`
						DisplayName string `json:"displayName"`
						UPN         string `json:"userPrincipalName"`
					}
					if json.Unmarshal([]byte(body.Body), &profile) == nil {
						tokenMu.Lock()
						if profile.Mail != "" {
							capturedEmail = profile.Mail
						} else if profile.UPN != "" {
							capturedEmail = profile.UPN
						}
						if profile.DisplayName != "" {
							capturedDisplayName = profile.DisplayName
						}
						tokenMu.Unlock()
					}
				}
			}
		},
	)
	_ = waitEvents

	// Step 1: Navigate to login.live.com to establish SSO session
	b.Logger.Infow("navigating to login.live.com to establish SSO session")
	if err := page.Navigate("https://login.live.com/"); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to navigate to login.live.com: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("login.live.com load timeout, continuing", "error", err)
	}
	time.Sleep(3 * time.Second)

	currentURL := page.MustInfo().URL
	b.Logger.Infow("after login.live.com navigation", "currentURL", currentURL)

	// Check if SSO failed
	if strings.Contains(currentURL, "login.live.com") && !strings.Contains(currentURL, "account.microsoft.com") {
		el, err := page.Element("#i0116")
		if err == nil && el != nil {
			b.Logger.Warnw("SSO session not established - cookies may be expired")
			return page, "", "", "", fmt.Errorf("cookies did not establish SSO session - session may be expired")
		}
	}

	// Step 2: Navigate to Outlook
	b.Logger.Infow("navigating to outlook.live.com/mail/ to trigger token acquisition")
	if err := page.Navigate("https://outlook.live.com/mail/"); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to navigate to outlook.live.com: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("outlook.live.com load timeout, continuing", "error", err)
	}

	// Wait for Outlook to fully load and make API calls (which we intercept)
	b.Logger.Infow("waiting for Outlook API calls to capture Bearer token...")
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		tokenMu.Lock()
		hasToken := capturedToken != ""
		tokenMu.Unlock()
		if hasToken {
			b.Logger.Infow("Bearer token captured from network interception")
			break
		}
		if i == 5 {
			page.Eval(`() => { window.scrollTo(0, 100); }`)
		}
	}

	// Check final page state
	finalURL := page.MustInfo().URL
	pageTitle, _ := page.Eval(`() => document.title`)
	title := ""
	if pageTitle != nil {
		title = pageTitle.Value.String()
	}
	b.Logger.Infow("final page state", "url", finalURL, "title", title)

	// Step 3: If no token from network interception, try forcing MSAL.js to acquire a Graph token
	tokenMu.Lock()
	needMSAL := capturedToken == ""
	tokenMu.Unlock()

	if needMSAL {
		b.Logger.Infow("no token from network, attempting MSAL.js acquireTokenSilent for Graph scopes")
		graphToken := b.forceAcquireGraphToken(page)
		if graphToken != "" {
			tokenMu.Lock()
			capturedToken = graphToken
			tokenMu.Unlock()
			b.Logger.Infow("acquired Graph token via MSAL.js acquireTokenSilent", "tokenLen", len(graphToken))
		}
	}

	// Step 4: If still no token, try extracting from MSAL cache with strict filtering
	tokenMu.Lock()
	if capturedToken == "" {
		b.Logger.Infow("trying MSAL cache extraction with strict scope filtering")
		token := b.extractJWTFromMSALCache(page)
		if token != "" {
			capturedToken = token
		}
	}
	// If we have a substrate token but no Graph/Outlook token, use substrate as fallback
	if capturedToken == "" && capturedSubstrateToken != "" {
		b.Logger.Infow("using substrate token as fallback")
		capturedToken = capturedSubstrateToken
	}
	tokenMu.Unlock()

	// Extract email and display name
	if capturedEmail == "" || capturedDisplayName == "" {
		email, name := b.extractAccountInfo(page)
		if email != "" && capturedEmail == "" {
			capturedEmail = email
		}
		if name != "" && capturedDisplayName == "" {
			capturedDisplayName = name
		}
	}

	// Try to extract display name from page title ("Mail - Jenessa Crook - Outlook")
	if capturedDisplayName == "" && strings.Contains(title, " - ") {
		parts := strings.Split(title, " - ")
		if len(parts) >= 2 {
			capturedDisplayName = strings.TrimSpace(parts[1])
		}
	}

	// Try to extract email from DOM
	if capturedEmail == "" {
		capturedEmail = b.extractEmailFromDOM(page)
	}

	tokenMu.Lock()
	defer tokenMu.Unlock()

	return page, capturedToken, capturedEmail, capturedDisplayName, nil
}

// forceAcquireGraphToken uses MSAL.js acquireTokenSilent to get a Graph-scoped token
func (b *BrowserSessionService) forceAcquireGraphToken(page *rod.Page) string {
	// This script finds the MSAL PublicClientApplication instance and calls acquireTokenSilent
	// with Graph API scopes (Mail.Read, Mail.Send, User.Read)
	script := `() => {
		return new Promise((resolve) => {
			try {
				// Find MSAL account from cache
				let account = null;
				const storages = [sessionStorage, localStorage];
				for (const storage of storages) {
					for (let i = 0; i < storage.length; i++) {
						const key = storage.key(i);
						if (key && key.includes('.account.') && !key.includes('accesstoken')) {
							try {
								const val = JSON.parse(storage.getItem(key));
								if (val && val.username && val.homeAccountId) {
									account = val;
									break;
								}
							} catch(e) {}
						}
					}
					if (account) break;
				}

				if (!account) {
					resolve(JSON.stringify({error: 'no MSAL account found in cache'}));
					return;
				}

				// Find the MSAL client ID from cache keys
				let clientId = null;
				for (const storage of storages) {
					for (let i = 0; i < storage.length; i++) {
						const key = storage.key(i);
						if (key && key.includes('accesstoken') && key.includes(account.homeAccountId)) {
							// Key format: homeAccountId-environment-credentialType-clientId-realm-target
							const parts = key.split('-');
							// Find the part that looks like a GUID (client ID)
							for (const part of parts) {
								if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(part)) {
									clientId = part;
									break;
								}
							}
							if (clientId) break;
						}
					}
					if (clientId) break;
				}

				if (!clientId) {
					resolve(JSON.stringify({error: 'no MSAL client ID found'}));
					return;
				}

				// Try to use the existing MSAL instance on the page
				// Outlook web stores it in various global variables
				const msalConfig = {
					auth: {
						clientId: clientId,
						authority: 'https://login.microsoftonline.com/consumers',
						redirectUri: window.location.origin
					},
					cache: {
						cacheLocation: 'sessionStorage'
					}
				};

				// Check if msal is available globally
				if (typeof msal !== 'undefined' && msal.PublicClientApplication) {
					const pca = new msal.PublicClientApplication(msalConfig);
					const msalAccount = {
						homeAccountId: account.homeAccountId,
						environment: account.environment || 'login.microsoftonline.com',
						tenantId: account.realm || 'consumers',
						username: account.username,
						localAccountId: account.localAccountId || account.homeAccountId.split('.')[0]
					};

					pca.acquireTokenSilent({
						scopes: ['https://graph.microsoft.com/Mail.Read', 'https://graph.microsoft.com/Mail.Send', 'https://graph.microsoft.com/User.Read'],
						account: msalAccount,
						forceRefresh: false
					}).then(response => {
						resolve(JSON.stringify({token: response.accessToken, scopes: response.scopes}));
					}).catch(err => {
						// Try with just openid profile
						pca.acquireTokenSilent({
							scopes: ['https://graph.microsoft.com/.default'],
							account: msalAccount,
							forceRefresh: false
						}).then(response => {
							resolve(JSON.stringify({token: response.accessToken, scopes: response.scopes}));
						}).catch(err2 => {
							resolve(JSON.stringify({error: 'acquireTokenSilent failed: ' + err2.message, account: account.username, clientId: clientId}));
						});
					});
				} else {
					// MSAL library not available as global, try to find it via webpack modules
					resolve(JSON.stringify({error: 'MSAL not available globally', account: account.username, clientId: clientId}));
				}
			} catch(e) {
				resolve(JSON.stringify({error: 'exception: ' + e.message}));
			}

			// Timeout after 15 seconds
			setTimeout(() => resolve(JSON.stringify({error: 'timeout'})), 15000);
		});
	}`

	result, err := page.Eval(script)
	if err != nil {
		b.Logger.Warnw("forceAcquireGraphToken eval failed", "error", err)
		return ""
	}
	if result == nil {
		return ""
	}

	val := result.Value.String()
	b.Logger.Infow("forceAcquireGraphToken result", "result", val)

	var resp struct {
		Token    string   `json:"token"`
		Scopes   []string `json:"scopes"`
		Error    string   `json:"error"`
		Account  string   `json:"account"`
		ClientID string   `json:"clientId"`
	}
	if json.Unmarshal([]byte(val), &resp) != nil {
		return ""
	}

	if resp.Token != "" && strings.Contains(resp.Token, ".") {
		b.Logger.Infow("successfully acquired Graph token via MSAL.js",
			"scopes", resp.Scopes,
			"tokenLen", len(resp.Token),
		)
		return resp.Token
	}

	if resp.Error != "" {
		b.Logger.Warnw("MSAL.js acquireTokenSilent failed",
			"error", resp.Error,
			"account", resp.Account,
			"clientId", resp.ClientID,
		)
	}

	return ""
}

// ValidateAndGetToken uses headless Chrome to validate cookies and obtain an access token.
func (b *BrowserSessionService) ValidateAndGetToken(ctx context.Context, cookiesJSON string) (*BrowserSessionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("starting browser-based cookie validation")

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	if len(cookies) == 0 {
		return &BrowserSessionResult{Valid: false, Error: "no cookies to inject"}, nil
	}

	browser, cleanup, err := b.launchBrowser(ctx, 120*time.Second)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	_, token, email, displayName, err := b.establishSSOSession(browser, cookies)
	if err != nil {
		return &BrowserSessionResult{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	result := &BrowserSessionResult{
		Valid:       token != "",
		Email:       email,
		DisplayName: displayName,
		AccessToken: token,
	}

	if !result.Valid {
		// Even without a token, if we got email/displayName the session is valid
		if email != "" || displayName != "" {
			result.Valid = true
			result.Error = "session valid but no API token acquired"
		} else {
			result.Error = "could not capture a valid JWT access token from Outlook"
		}
	}

	b.Logger.Infow("browser session validation complete",
		"valid", result.Valid,
		"email", result.Email,
		"displayName", result.DisplayName,
		"hasToken", result.AccessToken != "",
		"tokenLen", len(result.AccessToken),
	)

	return result, nil
}

// injectCookies injects cookies into the browser page for all relevant Microsoft domains
func (b *BrowserSessionService) injectCookies(page *rod.Page, cookies []cookieEntry) error {
	var lastErr error
	injected := 0

	for _, c := range cookies {
		if c.Name == "" || c.Value == "" {
			continue
		}

		domain := c.Domain
		if domain == "" {
			continue
		}

		// Normalize domain
		if strings.HasPrefix(domain, ".") {
			domain = domain[1:]
		}

		path := c.Path
		if path == "" {
			path = "/"
		}

		secure := false
		switch v := c.Secure.(type) {
		case bool:
			secure = v
		case string:
			secure = v == "true" || v == "1"
		case float64:
			secure = v != 0
		}

		httpOnly := false
		switch v := c.HttpOnly.(type) {
		case bool:
			httpOnly = v
		case string:
			httpOnly = v == "true" || v == "1"
		case float64:
			httpOnly = v != 0
		}

		var expires proto.TimeSinceEpoch
		switch v := c.ExpirationDate.(type) {
		case float64:
			if v > 0 {
				expires = proto.TimeSinceEpoch(v)
			}
		case string:
			// Skip if empty
		}

		sameSite := proto.NetworkCookieSameSiteNone
		switch strings.ToLower(c.SameSite) {
		case "strict":
			sameSite = proto.NetworkCookieSameSiteStrict
		case "lax":
			sameSite = proto.NetworkCookieSameSiteLax
		case "none", "no_restriction":
			sameSite = proto.NetworkCookieSameSiteNone
		}

		scheme := "https"
		if !secure {
			scheme = "http"
		}
		cookieURL := fmt.Sprintf("%s://%s%s", scheme, domain, path)

		_, setErr := proto.NetworkSetCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     path,
			Secure:   secure,
			HTTPOnly: httpOnly,
			SameSite: sameSite,
			URL:      cookieURL,
			Expires:  expires,
		}.Call(page)

		if setErr != nil {
			lastErr = setErr
			b.Logger.Debugw("failed to inject cookie", "name", c.Name, "domain", c.Domain, "error", setErr)
		} else {
			injected++
		}
	}

	b.Logger.Infow("cookies injected", "injected", injected, "total", len(cookies))
	return lastErr
}

// extractJWTFromMSALCache tries to extract a JWT access token from MSAL.js cache
// Only returns tokens that are actual JWTs (contain dots), with strict scope filtering
func (b *BrowserSessionService) extractJWTFromMSALCache(page *rod.Page) string {
	script := `() => {
		try {
			const tokens = [];
			const storages = [sessionStorage, localStorage];
			for (const storage of storages) {
				for (let i = 0; i < storage.length; i++) {
					const key = storage.key(i);
					if (key && key.includes('accesstoken')) {
						try {
							const val = JSON.parse(storage.getItem(key));
							if (val && val.secret) {
								tokens.push({
									secret: val.secret,
									scope: val.target || '',
									realm: val.realm || '',
									credentialType: val.credentialType || '',
									key: key
								});
							}
						} catch(e) {}
					}
				}
			}
			return JSON.stringify(tokens);
		} catch(e) {
			return '[]';
		}
	}`

	result, err := page.Eval(script)
	if err != nil || result == nil {
		return ""
	}

	var tokens []struct {
		Secret         string `json:"secret"`
		Scope          string `json:"scope"`
		Realm          string `json:"realm"`
		CredentialType string `json:"credentialType"`
		Key            string `json:"key"`
	}

	if err := json.Unmarshal([]byte(result.Value.String()), &tokens); err != nil {
		return ""
	}

	b.Logger.Infow("found tokens in MSAL cache", "count", len(tokens))

	// Log all tokens for debugging
	for i, t := range tokens {
		isJWT := strings.Contains(t.Secret, ".")
		b.Logger.Infow("MSAL cache token",
			"index", i,
			"scope", t.Scope,
			"realm", t.Realm,
			"isJWT", isJWT,
			"tokenLen", len(t.Secret),
		)
	}

	// Priority 1: JWT tokens with explicit Mail scope (for Graph API)
	for _, t := range tokens {
		if !strings.Contains(t.Secret, ".") {
			continue
		}
		scope := strings.ToLower(t.Scope)
		if strings.Contains(scope, "mail.read") || strings.Contains(scope, "mail.send") {
			b.Logger.Infow("selected JWT with Mail scope from MSAL cache",
				"scope", t.Scope,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	// Priority 2: JWT tokens scoped for substrate.office.com (Outlook's actual API)
	for _, t := range tokens {
		if !strings.Contains(t.Secret, ".") {
			continue
		}
		scope := strings.ToLower(t.Scope)
		if strings.Contains(scope, "substrate.office.com") {
			b.Logger.Infow("selected JWT with substrate scope from MSAL cache",
				"scope", t.Scope,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	// Priority 3: JWT tokens scoped for outlook.office.com
	for _, t := range tokens {
		if !strings.Contains(t.Secret, ".") {
			continue
		}
		scope := strings.ToLower(t.Scope)
		if strings.Contains(scope, "outlook.office.com") || strings.Contains(scope, "outlook.office365.com") {
			b.Logger.Infow("selected JWT with Outlook scope from MSAL cache",
				"scope", t.Scope,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	// Priority 4: Any JWT token that is NOT for augloop or ads
	for _, t := range tokens {
		if !strings.Contains(t.Secret, ".") {
			continue
		}
		scope := strings.ToLower(t.Scope)
		if strings.Contains(scope, "augloop") || strings.Contains(scope, "ads.") {
			continue // Skip augloop and ads tokens
		}
		b.Logger.Infow("selected JWT token from MSAL cache (fallback)",
			"scope", t.Scope,
			"tokenLen", len(t.Secret),
		)
		return t.Secret
	}

	// Priority 5: Last resort - any JWT
	for _, t := range tokens {
		if strings.Contains(t.Secret, ".") {
			b.Logger.Warnw("using last-resort JWT from MSAL cache (may be augloop/ads)",
				"scope", t.Scope,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	b.Logger.Warnw("no JWT tokens found in MSAL cache")
	return ""
}

// extractAccountInfo extracts email and display name from MSAL cache
func (b *BrowserSessionService) extractAccountInfo(page *rod.Page) (email, displayName string) {
	script := `() => {
		try {
			const storages = [sessionStorage, localStorage];
			for (const storage of storages) {
				for (let i = 0; i < storage.length; i++) {
					const key = storage.key(i);
					if (key && (key.includes('.account.') || key.includes('idtoken'))) {
						try {
							const val = JSON.parse(storage.getItem(key));
							if (val) {
								const email = val.username || val.preferred_username || '';
								const name = val.name || '';
								if (email || name) {
									return JSON.stringify({email: email, name: name});
								}
							}
						} catch(e) {}
					}
				}
			}
		} catch(e) {}
		return '';
	}`

	result, err := page.Eval(script)
	if err != nil || result == nil {
		return "", ""
	}
	val := result.Value.String()
	if val == "" {
		return "", ""
	}

	var data struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if json.Unmarshal([]byte(val), &data) == nil {
		return data.Email, data.Name
	}
	return "", ""
}

// extractEmailFromDOM tries to extract the email from the Outlook page DOM
func (b *BrowserSessionService) extractEmailFromDOM(page *rod.Page) string {
	selectors := []string{
		`[data-testid="mectrl_currentAccount_secondary"]`,
		`#mectrl_currentAccount_secondary`,
		`#O365_MainLink_Me`,
	}

	for _, sel := range selectors {
		el, err := page.Element(sel)
		if err != nil || el == nil {
			continue
		}
		text, err := el.Text()
		if err != nil || text == "" {
			continue
		}
		text = strings.TrimSpace(text)
		if strings.Contains(text, "@") {
			return text
		}
	}

	result, err := page.Eval(`() => {
		try {
			const meBtn = document.querySelector('[data-testid="mectrl_currentAccount_secondary"]');
			if (meBtn) return meBtn.textContent.trim();
			const allBtns = document.querySelectorAll('button[aria-label]');
			for (const btn of allBtns) {
				const label = btn.getAttribute('aria-label');
				if (label && label.includes('@')) {
					const match = label.match(/[\w.-]+@[\w.-]+/);
					if (match) return match[0];
				}
			}
		} catch(e) {}
		return '';
	}`)
	if err == nil && result != nil {
		val := result.Value.String()
		if val != "" && strings.Contains(val, "@") {
			return val
		}
	}

	return ""
}

// SendEmailViaBrowser sends an email using browser automation on Outlook web
func (b *BrowserSessionService) SendEmailViaBrowser(ctx context.Context, cookiesJSON string, to []string, subject, body string, isHTML bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("sending email via browser automation", "to", to, "subject", subject)

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, cleanup, err := b.launchBrowser(ctx, 120*time.Second)
	if err != nil {
		return err
	}
	defer cleanup()

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies and establish SSO
	b.injectCookies(page, cookies)
	if err := page.Navigate("https://login.live.com/"); err != nil {
		return fmt.Errorf("failed to navigate to login.live.com: %w", err)
	}
	page.WaitLoad()
	time.Sleep(3 * time.Second)

	// Navigate to compose
	composeURL := "https://outlook.live.com/mail/0/deeplink/compose"
	if err := page.Navigate(composeURL); err != nil {
		return fmt.Errorf("failed to navigate to compose: %w", err)
	}
	page.WaitLoad()
	time.Sleep(5 * time.Second)

	// Fill in the To field
	toField, err := page.Element(`[aria-label="To"]`)
	if err != nil {
		toField, err = page.Element(`input[aria-label="To"]`)
		if err != nil {
			return fmt.Errorf("could not find To field: %w", err)
		}
	}
	for _, recipient := range to {
		toField.Input(recipient)
		toField.Type(input.Enter)
		time.Sleep(500 * time.Millisecond)
	}

	// Fill in Subject
	subjectField, err := page.Element(`[aria-label="Add a subject"]`)
	if err != nil {
		subjectField, err = page.Element(`input[aria-label="Subject"]`)
		if err != nil {
			return fmt.Errorf("could not find Subject field: %w", err)
		}
	}
	subjectField.Input(subject)

	// Fill in Body
	bodyField, err := page.Element(`[aria-label="Message body, press Alt+F10 to exit"]`)
	if err != nil {
		bodyField, err = page.Element(`div[role="textbox"]`)
		if err != nil {
			return fmt.Errorf("could not find body field: %w", err)
		}
	}

	if isHTML {
		page.Eval(fmt.Sprintf(`(selector) => {
			const el = document.querySelector('[aria-label="Message body, press Alt+F10 to exit"]') || document.querySelector('div[role="textbox"]');
			if (el) el.innerHTML = %s;
		}`, jsonEscape(body)))
	} else {
		bodyField.Input(body)
	}

	time.Sleep(1 * time.Second)

	// Click Send button
	sendBtn, err := page.Element(`[aria-label="Send"]`)
	if err != nil {
		sendBtn, err = page.Element(`button[title="Send"]`)
		if err != nil {
			return fmt.Errorf("could not find Send button: %w", err)
		}
	}
	sendBtn.Click(proto.InputMouseButtonLeft, 1)

	time.Sleep(3 * time.Second)

	b.Logger.Infow("email sent via browser automation", "to", to, "subject", subject)
	return nil
}

// ReadInboxViaBrowser reads inbox messages using browser automation
func (b *BrowserSessionService) ReadInboxViaBrowser(ctx context.Context, cookiesJSON string) ([]map[string]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("reading inbox via browser automation")

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, cleanup, err := b.launchBrowser(ctx, 90*time.Second)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies and establish SSO
	b.injectCookies(page, cookies)
	page.Navigate("https://login.live.com/")
	page.WaitLoad()
	time.Sleep(3 * time.Second)

	// Navigate to inbox
	page.Navigate("https://outlook.live.com/mail/")
	page.WaitLoad()
	time.Sleep(5 * time.Second)

	// Extract messages from the page
	result, err := page.Eval(`() => {
		try {
			const messages = [];
			const items = document.querySelectorAll('[data-convid], [role="option"], [aria-label*="message"]');
			items.forEach(item => {
				const sender = item.querySelector('[data-testid="SenderName"]')?.textContent || 
				               item.querySelector('.OZZZK')?.textContent || '';
				const subject = item.querySelector('[data-testid="SubjectLine"]')?.textContent ||
				                item.querySelector('.jGG6V')?.textContent || '';
				const preview = item.querySelector('[data-testid="BodyPreview"]')?.textContent ||
				                item.querySelector('.Mc1Ri')?.textContent || '';
				const date = item.querySelector('[data-testid="DateLine"]')?.textContent ||
				             item.querySelector('.jHATS')?.textContent || '';
				if (sender || subject) {
					messages.push({sender, subject, preview, date});
				}
			});
			return JSON.stringify(messages);
		} catch(e) {
			return '[]';
		}
	}`)

	if err != nil {
		return nil, fmt.Errorf("failed to extract messages: %w", err)
	}

	var messages []map[string]string
	if result != nil {
		json.Unmarshal([]byte(result.Value.String()), &messages)
	}

	return messages, nil
}

// jsonEscape escapes a string for use in JavaScript
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// min returns the smaller of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseCookiesForDomains returns cookies filtered by Microsoft/Outlook domains
func ParseCookiesForDomains(cookiesJSON string) ([]cookieEntry, error) {
	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, err
	}

	msftDomains := []string{
		"live.com", "microsoft.com", "microsoftonline.com",
		"office.com", "office365.com", "outlook.com",
		"login.live.com", "outlook.live.com",
	}

	var filtered []cookieEntry
	for _, c := range cookies {
		domain := strings.TrimPrefix(c.Domain, ".")
		for _, d := range msftDomains {
			if strings.HasSuffix(domain, d) {
				filtered = append(filtered, c)
				break
			}
		}
	}

	return filtered, nil
}

// IsOutlookDomain checks if a URL belongs to an Outlook/Microsoft domain
func IsOutlookDomain(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	domains := []string{
		"live.com", "microsoft.com", "microsoftonline.com",
		"office.com", "office365.com", "outlook.com",
	}
	for _, d := range domains {
		if strings.HasSuffix(host, d) {
			return true
		}
	}
	return false
}
