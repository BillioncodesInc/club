package controller

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/service"
)

// Telegram is the controller for Telegram notification settings.
type Telegram struct {
	Common
	TelegramService *service.Telegram
}

// GetSettings returns the current Telegram integration settings.
func (c *Telegram) GetSettings(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	// check permissions (mirrors SaveSettings / Test)
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	settings, err := c.TelegramService.GetSettings(g.Request.Context())
	if err != nil {
		// return empty defaults if not configured yet
		c.Response.OK(g, &service.TelegramSettings{
			DataLevel: "full",
		})
		return
	}

	// mask the bot token for security
	masked := *settings
	if len(masked.BotToken) > 8 {
		masked.BotToken = masked.BotToken[:4] + "..." + masked.BotToken[len(masked.BotToken)-4:]
	}
	c.Response.OK(g, masked)
}

// SaveSettings saves the Telegram integration settings.
func (c *Telegram) SaveSettings(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	// check permissions
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	body, err := io.ReadAll(g.Request.Body)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	var settings service.TelegramSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		c.Response.BadRequest(g)
		return
	}

	if err := c.TelegramService.SaveSettings(g.Request.Context(), &settings); err != nil {
		c.Response.ServerError(g)
		return
	}

	c.Response.OK(g, gin.H{"message": "Telegram settings saved"})
}

// Test sends a test message to verify the Telegram configuration.
func (c *Telegram) Test(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	body, err := io.ReadAll(g.Request.Body)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	var settings service.TelegramSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		c.Response.BadRequest(g)
		return
	}

	if settings.BotToken == "" || settings.ChatID == "" {
		g.JSON(http.StatusBadRequest, gin.H{"error": "botToken and chatID are required"})
		return
	}

	if err := c.TelegramService.SendTestMessage(&settings); err != nil {
		g.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.Response.OK(g, gin.H{"message": "Test message sent successfully"})
}
