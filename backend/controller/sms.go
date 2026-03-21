package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// SMS is the controller for SMS campaign features
type SMS struct {
	Common
	SMSService *service.SMS
}

// GetConfig returns the current SMS configuration
func (s *SMS) GetConfig(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	config, err := s.SMSService.GetConfig(g.Request.Context(), nil)
	if !s.handleErrors(g, err) {
		return
	}

	// Mask sensitive fields for display
	if config.TwilioAuthToken != "" {
		config.TwilioAuthToken = maskString(config.TwilioAuthToken)
	}
	if config.TextBeeAPIKey != "" {
		config.TextBeeAPIKey = maskString(config.TextBeeAPIKey)
	}

	s.Response.OK(g, config)
}

// SaveConfig saves the SMS configuration
func (s *SMS) SaveConfig(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	var config service.SMSConfig
	if ok := s.handleParseRequest(g, &config); !ok {
		return
	}

	// If masked values are sent back, fetch originals
	existing, _ := s.SMSService.GetConfig(g.Request.Context(), nil)
	if existing != nil {
		if isMasked(config.TwilioAuthToken) {
			config.TwilioAuthToken = existing.TwilioAuthToken
		}
		if isMasked(config.TextBeeAPIKey) {
			config.TextBeeAPIKey = existing.TextBeeAPIKey
		}
	}

	err := s.SMSService.SaveConfig(g.Request.Context(), nil, &config)
	if !s.handleErrors(g, err) {
		return
	}

	s.Response.OK(g, map[string]string{"status": "updated"})
}

// Send sends a single SMS message
func (s *SMS) Send(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	var req service.SMSSendRequest
	if ok := s.handleParseRequest(g, &req); !ok {
		return
	}

	result, err := s.SMSService.Send(g.Request.Context(), nil, &req)
	if !s.handleErrors(g, err) {
		return
	}

	s.Response.OK(g, result)
}

// SendBulk sends SMS to multiple recipients
func (s *SMS) SendBulk(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		Recipients []service.SMSSendRequest `json:"recipients"`
		DelayMs    int                      `json:"delayMs"`
	}
	if ok := s.handleParseRequest(g, &req); !ok {
		return
	}

	result, err := s.SMSService.SendBulk(g.Request.Context(), nil, req.Recipients, req.DelayMs)
	if !s.handleErrors(g, err) {
		return
	}

	s.Response.OK(g, result)
}

// TestConnection tests the SMS provider connection
func (s *SMS) TestConnection(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	result, err := s.SMSService.TestConnection(g.Request.Context(), nil)
	if !s.handleErrors(g, err) {
		return
	}

	s.Response.OK(g, result)
}

// GetProviders returns available SMS providers
func (s *SMS) GetProviders(g *gin.Context) {
	_, _, ok := s.handleSession(g)
	if !ok {
		return
	}

	providers := []map[string]string{
		{"id": "twilio", "name": "Twilio", "description": "Twilio SMS API - requires Account SID, Auth Token, and From Number"},
		{"id": "textbee", "name": "TextBee", "description": "TextBee SMS API - uses Android device as SMS gateway"},
	}

	s.Response.OK(g, providers)
}

// maskString masks a string showing only last 4 chars
func maskString(str string) string {
	if len(str) <= 4 {
		return "****"
	}
	return "****" + str[len(str)-4:]
}

// isMasked checks if a string is a masked value
func isMasked(str string) bool {
	return len(str) > 4 && str[:4] == "****"
}
