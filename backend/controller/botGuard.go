package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type BotGuard struct {
	Common
	Service *service.BotGuard
}

func (c *BotGuard) GetConfig(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session
	c.Response.OK(g, c.Service.GetConfig())
}

func (c *BotGuard) UpdateConfig(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session

	var cfg service.BotGuardConfig
	if !c.handleParseRequest(g, &cfg) {
		return
	}

	if err := c.Service.UpdateConfig(&cfg); err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Response.OK(g, c.Service.GetConfig())
}

func (c *BotGuard) GetStats(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session
	c.Response.OK(g, c.Service.GetSessionStats())
}

func (c *BotGuard) Cleanup(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session
	c.Service.CleanupExpired()
	c.Response.OK(g, map[string]string{"status": "cleaned"})
}
