package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// Turnstile is a controller for Cloudflare Turnstile configuration
type Turnstile struct {
	Common
	TurnstileService *service.Turnstile
}

// GetConfig returns the current Turnstile configuration (with secret key masked)
func (t *Turnstile) GetConfig(g *gin.Context) {
	_, _, ok := t.handleSession(g)
	if !ok {
		return
	}

	config := t.TurnstileService.GetConfig()
	// mask the secret key for the response
	maskedConfig := &service.TurnstileConfig{
		Enabled: config.Enabled,
		SiteKey: config.SiteKey,
		Mode:    config.Mode,
	}
	if config.SecretKey != "" {
		maskedConfig.SecretKey = "***configured***"
	}

	t.Response.OK(g, maskedConfig)
}

// UpdateConfig updates the Turnstile configuration
func (t *Turnstile) UpdateConfig(g *gin.Context) {
	session, _, ok := t.handleSession(g)
	if !ok {
		return
	}

	var config service.TurnstileConfig
	if ok := t.handleParseRequest(g, &config); !ok {
		return
	}

	// if secret key is the masked value, keep the existing one
	if config.SecretKey == "***configured***" {
		existing := t.TurnstileService.GetConfig()
		config.SecretKey = existing.SecretKey
	}

	err := t.TurnstileService.UpdateConfig(g.Request.Context(), session, &config)
	if ok := t.handleErrors(g, err); !ok {
		return
	}

	t.Response.OK(g, gin.H{
		"message": "turnstile configuration updated",
	})
}

// TestVerification tests a Turnstile token verification
func (t *Turnstile) TestVerification(g *gin.Context) {
	_, _, ok := t.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		Token    string `json:"token"`
		RemoteIP string `json:"remoteIP"`
	}
	if ok := t.handleParseRequest(g, &req); !ok {
		return
	}

	success, err := t.TurnstileService.VerifyToken(req.Token, req.RemoteIP)
	if err != nil {
		t.Response.OK(g, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	t.Response.OK(g, gin.H{
		"success": success,
	})
}
