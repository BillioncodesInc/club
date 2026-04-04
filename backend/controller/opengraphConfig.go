package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/service"
)

// OpenGraphConfig is the controller for OpenGraph configuration management.
type OpenGraphConfig struct {
	Common
	OpenGraphConfigRepository *repository.OpenGraphConfig
	OnConfigChanged           func() // callback to invalidate proxy cache
}

type openGraphConfigRequest struct {
	OGTitle       string `json:"ogTitle"`
	OGDescription string `json:"ogDescription"`
	OGImage       string `json:"ogImage"`
	OGURL         string `json:"ogUrl"`
	OGType        string `json:"ogType"`
	OGSiteName    string `json:"ogSiteName"`
	TwitterCard   string `json:"twitterCard"`
	Favicon       string `json:"favicon"`
}

// GetByProxyID returns the OpenGraph config for a specific proxy.
func (c *OpenGraphConfig) GetByProxyID(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	proxyIDStr := g.Param("proxyId")
	proxyID, err := uuid.Parse(proxyIDStr)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	config, err := c.OpenGraphConfigRepository.GetByProxyID(g.Request.Context(), &proxyID)
	if err != nil {
		// return empty config if none exists
		c.Response.OK(g, gin.H{
			"proxyId":       proxyIDStr,
			"ogTitle":       "",
			"ogDescription": "",
			"ogImage":       "",
			"ogUrl":         "",
			"ogType":        "website",
			"ogSiteName":    "",
			"twitterCard":   "summary_large_image",
			"favicon":       "",
		})
		return
	}

	c.Response.OK(g, config)
}

// Upsert creates or updates the OpenGraph config for a proxy.
func (c *OpenGraphConfig) Upsert(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	proxyIDStr := g.Param("proxyId")
	proxyID, err := uuid.Parse(proxyIDStr)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	var req openGraphConfigRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequest(g)
		return
	}

	now := time.Now()
	config := &database.OpenGraphConfig{
		ProxyID:       &proxyID,
		OGTitle:       req.OGTitle,
		OGDescription: req.OGDescription,
		OGImage:       req.OGImage,
		OGURL:         req.OGURL,
		OGType:        req.OGType,
		OGSiteName:    req.OGSiteName,
		TwitterCard:   req.TwitterCard,
		Favicon:       req.Favicon,
		UpdatedAt:     &now,
		CreatedAt:     &now,
	}

	saved, err := c.OpenGraphConfigRepository.Upsert(g.Request.Context(), config)
	if err != nil {
		c.Response.ServerError(g)
		return
	}

	// invalidate proxy cache
	if c.OnConfigChanged != nil {
		go c.OnConfigChanged()
	}

	c.Response.OK(g, saved)
}

// Delete removes the OpenGraph config for a proxy.
func (c *OpenGraphConfig) Delete(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	proxyIDStr := g.Param("proxyId")
	proxyID, err := uuid.Parse(proxyIDStr)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	if err := c.OpenGraphConfigRepository.DeleteByProxyID(g.Request.Context(), &proxyID); err != nil {
		c.Response.ServerError(g)
		return
	}

	// invalidate proxy cache
	if c.OnConfigChanged != nil {
		go c.OnConfigChanged()
	}

	c.Response.OK(g, gin.H{"message": "OpenGraph config deleted"})
}
