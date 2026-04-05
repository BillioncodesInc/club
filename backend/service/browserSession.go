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
	var capturedEmail string
	var capturedDisplayName string
	var tokenMu sync.Mutex

	// Enable network events
	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		b.Logger.Warnw("failed to enable network events", "error", err)
	}

	// Listen for network requests that contain Authorization: Bearer headers
	// This captures the actual token Outlook uses for its API calls
	waitEvents := page.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			reqURL := e.Request.URL

			// Look for Authorization header in requests to Outlook/Office APIs
			isOutlookAPI := strings.Contains(reqURL, "substrate.office.com") ||
				strings.Contains(reqURL, "outlook.office.com") ||
				strings.Contains(reqURL, "outlook.office365.com") ||
				strings.Contains(reqURL, "graph.microsoft.com") ||
				(strings.Contains(reqURL, "outlook.live.com") && strings.Contains(reqURL, "/owa/"))

			if isOutlookAPI {
				for key, val := range e.Request.Headers {
					if strings.EqualFold(key, "Authorization") {
						authVal := val.String()
						if strings.HasPrefix(authVal, "Bearer ") {
							bearerToken := strings.TrimPrefix(authVal, "Bearer ")
							// Only accept JWT tokens (contain dots)
							if strings.Contains(bearerToken, ".") {
								tokenMu.Lock()
								if capturedToken == "" {
									capturedToken = bearerToken
									b.Logger.Infow("captured Bearer token from Outlook API request",
										"url", reqURL,
										"tokenLen", len(bearerToken),
									)
								}
								tokenMu.Unlock()
							} else {
								b.Logger.Debugw("skipping non-JWT Bearer token",
									"url", reqURL,
									"tokenPrefix", bearerToken[:min(20, len(bearerToken))],
								)
							}
						}
					}
				}
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
					// Only accept JWT tokens
					if strings.Contains(tokenResp.AccessToken, ".") {
						tokenMu.Lock()
						if capturedToken == "" ||
							strings.Contains(tokenResp.Scope, "Mail") ||
							strings.Contains(tokenResp.Scope, "mail") {
							capturedToken = tokenResp.AccessToken
							b.Logger.Infow("captured JWT access token from token endpoint",
								"scope", tokenResp.Scope,
								"expiresIn", tokenResp.ExpiresIn,
							)
						}
						tokenMu.Unlock()
					}
				}
			}

			// Capture email from Graph API /me responses
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
	for i := 0; i < 20; i++ {
		time.Sleep(2 * time.Second)
		tokenMu.Lock()
		hasToken := capturedToken != ""
		tokenMu.Unlock()
		if hasToken {
			b.Logger.Infow("Bearer token captured successfully")
			break
		}
		if i == 5 {
			// After 10 seconds, try scrolling/clicking to trigger more API calls
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

	// If we still don't have a JWT token, try extracting from MSAL cache (but only JWTs)
	tokenMu.Lock()
	if capturedToken == "" {
		b.Logger.Infow("no Bearer token from network interception, trying MSAL cache extraction (JWT only)")
		token := b.extractJWTFromMSALCache(page)
		if token != "" {
			capturedToken = token
		}
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

	// Try to extract display name from page title
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
		result.Error = "could not capture a valid JWT access token from Outlook"
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
// Only returns tokens that are actual JWTs (contain dots), filtering out MSA compact tickets
func (b *BrowserSessionService) extractJWTFromMSALCache(page *rod.Page) string {
	script := `() => {
		try {
			const tokens = [];
			// Check sessionStorage
			for (let i = 0; i < sessionStorage.length; i++) {
				const key = sessionStorage.key(i);
				if (key && key.includes('accesstoken')) {
					try {
						const val = JSON.parse(sessionStorage.getItem(key));
						if (val && val.secret) {
							tokens.push({
								secret: val.secret,
								scope: val.target || '',
								realm: val.realm || '',
								credentialType: val.credentialType || ''
							});
						}
					} catch(e) {}
				}
			}
			// Check localStorage
			for (let i = 0; i < localStorage.length; i++) {
				const key = localStorage.key(i);
				if (key && key.includes('accesstoken')) {
					try {
						const val = JSON.parse(localStorage.getItem(key));
						if (val && val.secret) {
							tokens.push({
								secret: val.secret,
								scope: val.target || '',
								realm: val.realm || '',
								credentialType: val.credentialType || ''
							});
						}
					} catch(e) {}
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
	}

	if err := json.Unmarshal([]byte(result.Value.String()), &tokens); err != nil {
		return ""
	}

	b.Logger.Infow("found tokens in MSAL cache", "count", len(tokens))

	// First pass: look for JWT tokens with Mail/Outlook scope
	for _, t := range tokens {
		if !strings.Contains(t.Secret, ".") {
			continue // Skip non-JWT tokens (MSA compact tickets)
		}
		scope := strings.ToLower(t.Scope)
		if strings.Contains(scope, "mail") || strings.Contains(scope, "outlook") ||
			strings.Contains(scope, "graph") || strings.Contains(scope, "office") {
			b.Logger.Infow("found JWT token with mail scope in MSAL cache",
				"scope", t.Scope,
				"realm", t.Realm,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	// Second pass: any JWT token
	for _, t := range tokens {
		if strings.Contains(t.Secret, ".") {
			b.Logger.Infow("found JWT token in MSAL cache (no mail scope)",
				"scope", t.Scope,
				"realm", t.Realm,
				"tokenLen", len(t.Secret),
			)
			return t.Secret
		}
	}

	b.Logger.Warnw("no JWT tokens found in MSAL cache, only MSA compact tickets")
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
					if (key && (key.includes('account') || key.includes('idtoken'))) {
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
