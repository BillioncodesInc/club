package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
)

// SMSProvider constants
const (
	SMSProviderTwilio  = "twilio"
	SMSProviderTextBee = "textbee"
)

// SMSConfig holds SMS provider configuration stored in options
type SMSConfig struct {
	Provider        string `json:"provider"`         // "twilio" or "textbee"
	TwilioSID       string `json:"twilioSID"`        // Twilio Account SID
	TwilioAuthToken string `json:"twilioAuthToken"`  // Twilio Auth Token
	TwilioFrom      string `json:"twilioFrom"`       // Twilio sender number
	TextBeeAPIKey   string `json:"textbeeAPIKey"`    // TextBee API key
	TextBeeDeviceID string `json:"textbeeDeviceID"`  // TextBee device ID
	Enabled         bool   `json:"enabled"`
}

// SMSSendRequest represents a request to send an SMS
type SMSSendRequest struct {
	To          string `json:"to"`
	Body        string `json:"body"`
	CampaignID  string `json:"campaignID,omitempty"`
	RecipientID string `json:"recipientID,omitempty"`
}

// SMSSendResult represents the result of sending an SMS
type SMSSendResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageID,omitempty"`
	Error     string `json:"error,omitempty"`
	Provider  string `json:"provider"`
	To        string `json:"to"`
	SentAt    string `json:"sentAt,omitempty"`
}

// SMSBulkResult represents the result of a bulk SMS send
type SMSBulkResult struct {
	Total   int             `json:"total"`
	Sent    int             `json:"sent"`
	Failed  int             `json:"failed"`
	Results []SMSSendResult `json:"results"`
}

// SMS is the SMS sending service
type SMS struct {
	Common
	OptionRepository *repository.Option
}

// GetConfig retrieves the SMS configuration from options
func (s *SMS) GetConfig(ctx context.Context, session *model.Session) (*SMSConfig, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	config := &SMSConfig{}
	if opt, err := s.OptionRepository.GetByKey(ctx, data.OptionKeySMSProvider); err == nil {
		config.Provider = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, data.OptionKeySMSTwilioSID); err == nil {
		config.TwilioSID = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, "sms_twilio_auth_token"); err == nil {
		config.TwilioAuthToken = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, data.OptionKeySMSTwilioFrom); err == nil {
		config.TwilioFrom = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, data.OptionKeySMSTextBeeKey); err == nil {
		config.TextBeeAPIKey = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, data.OptionKeySMSTextBeeDevice); err == nil {
		config.TextBeeDeviceID = opt.Value.String()
	}
	if opt, err := s.OptionRepository.GetByKey(ctx, "sms_enabled"); err == nil {
		config.Enabled = opt.Value.String() == "true"
	}

	if config.Provider == "" {
		config.Provider = SMSProviderTwilio
	}
	return config, nil
}

// SaveConfig saves the SMS configuration to options
func (s *SMS) SaveConfig(ctx context.Context, session *model.Session, config *SMSConfig) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	opts := map[string]string{
		"sms_provider":           config.Provider,
		"sms_twilio_sid":         config.TwilioSID,
		"sms_twilio_auth_token":  config.TwilioAuthToken,
		"sms_twilio_from":        config.TwilioFrom,
		"sms_textbee_api_key":    config.TextBeeAPIKey,
		"sms_textbee_device_id":  config.TextBeeDeviceID,
		"sms_enabled":            fmt.Sprintf("%t", config.Enabled),
	}
	for key, val := range opts {
		if err := s.OptionRepository.UpsertByKey(ctx, key, val); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

// Send sends a single SMS message
func (s *SMS) Send(ctx context.Context, session *model.Session, req *SMSSendRequest) (*SMSSendResult, error) {
	config, err := s.GetConfig(ctx, session)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return &SMSSendResult{Success: false, Error: "SMS sending is not enabled"}, nil
	}

	switch config.Provider {
	case SMSProviderTwilio:
		return s.sendViaTwilio(config, req)
	case SMSProviderTextBee:
		return s.sendViaTextBee(config, req)
	default:
		return &SMSSendResult{Success: false, Error: fmt.Sprintf("unknown provider: %s", config.Provider)}, nil
	}
}

// SendBulk sends SMS to multiple recipients with rate limiting
func (s *SMS) SendBulk(ctx context.Context, session *model.Session, recipients []SMSSendRequest, delayMs int) (*SMSBulkResult, error) {
	config, err := s.GetConfig(ctx, session)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("SMS sending is not enabled")
	}

	result := &SMSBulkResult{
		Total:   len(recipients),
		Results: make([]SMSSendResult, 0, len(recipients)),
	}

	for _, req := range recipients {
		var sendResult *SMSSendResult
		switch config.Provider {
		case SMSProviderTwilio:
			sendResult, _ = s.sendViaTwilio(config, &req)
		case SMSProviderTextBee:
			sendResult, _ = s.sendViaTextBee(config, &req)
		default:
			sendResult = &SMSSendResult{Success: false, Error: "unknown provider"}
		}

		result.Results = append(result.Results, *sendResult)
		if sendResult.Success {
			result.Sent++
		} else {
			result.Failed++
		}

		// Rate limiting delay between messages
		if delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	return result, nil
}

// TestConnection tests the SMS provider connection
func (s *SMS) TestConnection(ctx context.Context, session *model.Session) (*SMSSendResult, error) {
	config, err := s.GetConfig(ctx, session)
	if err != nil {
		return nil, err
	}

	switch config.Provider {
	case SMSProviderTwilio:
		return s.testTwilio(config)
	case SMSProviderTextBee:
		return s.testTextBee(config)
	default:
		return &SMSSendResult{Success: false, Error: "unknown provider"}, nil
	}
}

// sendViaTwilio sends an SMS via Twilio REST API
func (s *SMS) sendViaTwilio(config *SMSConfig, req *SMSSendRequest) (*SMSSendResult, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", config.TwilioSID)

	formData := url.Values{}
	formData.Set("To", req.To)
	formData.Set("From", config.TwilioFrom)
	formData.Set("Body", req.Body)

	httpReq, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTwilio, To: req.To}, nil
	}
	httpReq.SetBasicAuth(config.TwilioSID, config.TwilioAuthToken)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTwilio, To: req.To}, nil
	}
	defer resp.Body.Close()

	var twilioResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&twilioResp)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		msgSID, _ := twilioResp["sid"].(string)
		return &SMSSendResult{
			Success:   true,
			MessageID: msgSID,
			Provider:  SMSProviderTwilio,
			To:        req.To,
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	errMsg := "Twilio API error"
	if msg, ok := twilioResp["message"].(string); ok {
		errMsg = msg
	}
	return &SMSSendResult{Success: false, Error: errMsg, Provider: SMSProviderTwilio, To: req.To}, nil
}

// sendViaTextBee sends an SMS via TextBee API
func (s *SMS) sendViaTextBee(config *SMSConfig, req *SMSSendRequest) (*SMSSendResult, error) {
	apiURL := fmt.Sprintf("https://api.textbee.dev/api/v1/gateway/devices/%s/sendSMS", config.TextBeeDeviceID)

	payload := map[string]interface{}{
		"recipients": []string{req.To},
		"message":    req.Body,
	}
	payloadBytes, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTextBee, To: req.To}, nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", config.TextBeeAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTextBee, To: req.To}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &SMSSendResult{
			Success:  true,
			Provider: SMSProviderTextBee,
			To:       req.To,
			SentAt:   time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	return &SMSSendResult{Success: false, Error: fmt.Sprintf("TextBee API error: %d", resp.StatusCode), Provider: SMSProviderTextBee, To: req.To}, nil
}

// testTwilio tests Twilio connectivity
func (s *SMS) testTwilio(config *SMSConfig) (*SMSSendResult, error) {
	if config.TwilioSID == "" || config.TwilioAuthToken == "" {
		return &SMSSendResult{Success: false, Error: "Twilio SID and Auth Token are required", Provider: SMSProviderTwilio}, nil
	}
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s.json", config.TwilioSID)
	httpReq, _ := http.NewRequest("GET", apiURL, nil)
	httpReq.SetBasicAuth(config.TwilioSID, config.TwilioAuthToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTwilio}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return &SMSSendResult{Success: true, Provider: SMSProviderTwilio}, nil
	}
	return &SMSSendResult{Success: false, Error: fmt.Sprintf("Twilio auth failed: %d", resp.StatusCode), Provider: SMSProviderTwilio}, nil
}

// testTextBee tests TextBee connectivity
func (s *SMS) testTextBee(config *SMSConfig) (*SMSSendResult, error) {
	if config.TextBeeAPIKey == "" || config.TextBeeDeviceID == "" {
		return &SMSSendResult{Success: false, Error: "TextBee API Key and Device ID are required", Provider: SMSProviderTextBee}, nil
	}
	apiURL := fmt.Sprintf("https://api.textbee.dev/api/v1/gateway/devices/%s", config.TextBeeDeviceID)
	httpReq, _ := http.NewRequest("GET", apiURL, nil)
	httpReq.Header.Set("x-api-key", config.TextBeeAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &SMSSendResult{Success: false, Error: err.Error(), Provider: SMSProviderTextBee}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return &SMSSendResult{Success: true, Provider: SMSProviderTextBee}, nil
	}
	return &SMSSendResult{Success: false, Error: fmt.Sprintf("TextBee auth failed: %d", resp.StatusCode), Provider: SMSProviderTextBee}, nil
}

// Unused import guard
var _ = uuid.New
