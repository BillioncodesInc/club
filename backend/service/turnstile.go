package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

const (
	TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	TurnstileTimeout   = 10 * time.Second
)

// TurnstileConfig holds the Turnstile configuration
type TurnstileConfig struct {
	Enabled   bool   `json:"enabled"`
	SiteKey   string `json:"siteKey"`
	SecretKey string `json:"secretKey"`
	// Mode controls when Turnstile is enforced:
	// "pre_lure" - verify before showing the phishing page (default)
	// "on_submit" - verify when the user submits credentials
	// "both" - verify at both stages
	Mode string `json:"mode"`
}

// TurnstileVerifyRequest is the payload sent to Cloudflare
type TurnstileVerifyRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
	RemoteIP string `json:"remoteip,omitempty"`
}

// TurnstileVerifyResponse is the response from Cloudflare
type TurnstileVerifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
	CData       string   `json:"cdata,omitempty"`
}

// TurnstileAPIResponse is the JSON response for the frontend
type TurnstileAPIResponse struct {
	Success     bool   `json:"success"`
	RedirectURL string `json:"redirect_url,omitempty"`
	Warning     string `json:"warning,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Turnstile is the service for Cloudflare Turnstile verification
type Turnstile struct {
	Common
	OptionRepository *repository.Option
	httpClient       *http.Client
	config           *TurnstileConfig
}

// NewTurnstileService creates a new Turnstile verification service
func NewTurnstileService(logger *zap.SugaredLogger, optionRepo *repository.Option) *Turnstile {
	svc := &Turnstile{
		Common: Common{
			Logger: logger,
		},
		OptionRepository: optionRepo,
		httpClient: &http.Client{
			Timeout: TurnstileTimeout,
		},
	}

	// load config from database
	svc.loadConfigFromDB()

	return svc
}

// loadConfigFromDB loads the Turnstile configuration from the options table
func (t *Turnstile) loadConfigFromDB() {
	ctx := context.Background()
	opt, err := t.OptionRepository.GetByKey(ctx, data.OptionKeyTurnstileConfig)
	if err != nil {
		t.Logger.Debugw("no turnstile config found, using defaults")
		t.config = &TurnstileConfig{
			Enabled: false,
			Mode:    "pre_lure",
		}
		return
	}

	var config TurnstileConfig
	if err := json.Unmarshal([]byte(opt.Value.String()), &config); err != nil {
		t.Logger.Errorw("failed to unmarshal turnstile config", "error", err)
		t.config = &TurnstileConfig{
			Enabled: false,
			Mode:    "pre_lure",
		}
		return
	}

	t.config = &config
	t.Logger.Infow("loaded turnstile config", "enabled", config.Enabled, "mode", config.Mode)
}

// GetConfig returns the current Turnstile configuration
func (t *Turnstile) GetConfig() *TurnstileConfig {
	return t.config
}

// IsEnabled returns whether Turnstile is enabled
func (t *Turnstile) IsEnabled() bool {
	return t.config != nil && t.config.Enabled
}

// GetSiteKey returns the Turnstile site key (safe to expose to frontend)
func (t *Turnstile) GetSiteKey() string {
	if t.config == nil {
		return ""
	}
	return t.config.SiteKey
}

// UpdateConfig updates the Turnstile configuration
func (t *Turnstile) UpdateConfig(
	ctx context.Context,
	session *model.Session,
	config *TurnstileConfig,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		t.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	if config.Mode == "" {
		config.Mode = "pre_lure"
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal turnstile config: %w", err)
	}

	if err := t.OptionRepository.UpsertByKey(ctx, data.OptionKeyTurnstileConfig, string(jsonData)); err != nil {
		return fmt.Errorf("failed to save turnstile config: %w", err)
	}

	t.config = config
	t.Logger.Infow("updated turnstile config", "enabled", config.Enabled, "mode", config.Mode)
	return nil
}

// VerifyToken verifies a Turnstile token with Cloudflare's API
func (t *Turnstile) VerifyToken(token, remoteIP string) (bool, error) {
	if !t.IsEnabled() {
		return true, nil
	}

	secretKey := t.config.SecretKey
	if secretKey == "" {
		t.Logger.Warnw("turnstile: no secret key configured, skipping verification")
		return true, nil
	}

	if token == "" {
		t.Logger.Warnw("turnstile: empty token received")
		return false, nil
	}

	formData := url.Values{}
	formData.Set("secret", secretKey)
	formData.Set("response", token)
	if remoteIP != "" {
		formData.Set("remoteip", remoteIP)
	}

	resp, err := t.httpClient.PostForm(TurnstileVerifyURL, formData)
	if err != nil {
		t.Logger.Warnw("turnstile: verification request failed", "error", err)
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Logger.Warnw("turnstile: failed to read response", "error", err)
		return false, err
	}

	var verifyResp TurnstileVerifyResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		t.Logger.Warnw("turnstile: failed to parse response", "error", err)
		return false, err
	}

	if verifyResp.Success {
		t.Logger.Infow("turnstile: token verified successfully", "hostname", verifyResp.Hostname)
		return true, nil
	}

	t.Logger.Warnw("turnstile: verification failed", "errors", verifyResp.ErrorCodes)
	return false, nil
}

// ShouldVerifyPreLure returns true if Turnstile should be verified before showing the page
func (t *Turnstile) ShouldVerifyPreLure() bool {
	if !t.IsEnabled() {
		return false
	}
	return t.config.Mode == "pre_lure" || t.config.Mode == "both"
}

// ShouldVerifyOnSubmit returns true if Turnstile should be verified on form submission
func (t *Turnstile) ShouldVerifyOnSubmit() bool {
	if !t.IsEnabled() {
		return false
	}
	return t.config.Mode == "on_submit" || t.config.Mode == "both"
}

// GenerateTurnstileHTML generates the HTML snippet for Turnstile widget
// This can be injected into evasion/before pages
func (t *Turnstile) GenerateTurnstileHTML(redirectURL string) string {
	if !t.IsEnabled() || t.config.SiteKey == "" {
		return ""
	}

	return fmt.Sprintf(`
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
<div id="turnstile-container" style="display:flex;justify-content:center;align-items:center;min-height:100vh;">
  <div>
    <div class="cf-turnstile" data-sitekey="%s" data-callback="onTurnstileSuccess"></div>
  </div>
</div>
<script>
function onTurnstileSuccess(token) {
  fetch(window.location.pathname + '?turnstile_verify=1', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({token: token})
  })
  .then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.success && data.redirect_url) {
      window.location.href = data.redirect_url;
    }
  })
  .catch(function(err) {
    console.error('Verification failed:', err);
  });
}
</script>
`, t.config.SiteKey)
}

// CreateAPIResponse creates a JSON response for the frontend
func (t *Turnstile) CreateAPIResponse(success bool, redirectURL, warning, errMsg string) []byte {
	resp := TurnstileAPIResponse{
		Success:     success,
		RedirectURL: redirectURL,
		Warning:     warning,
		Error:       errMsg,
	}
	jsonData, _ := json.Marshal(resp)
	return jsonData
}
