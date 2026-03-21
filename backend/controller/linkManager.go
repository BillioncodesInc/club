package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type LinkManager struct {
	Common
	LinkManagerService *service.LinkManager
}

type linkRotateRequest struct {
	Codes  []string `json:"codes" binding:"required"`
	NewURL string   `json:"newUrl" binding:"required"`
}

// Shorten creates a shortened URL
func (c *LinkManager) Shorten(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.ShortenRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	link, err := c.LinkManagerService.Shorten(&req)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, link)
}

// Expand resolves a short code to the original URL
func (c *LinkManager) Expand(g *gin.Context) {
	code := g.Param("code")
	if code == "" {
		c.Response.BadRequestMessage(g, "code is required")
		return
	}

	originalURL, err := c.LinkManagerService.Expand(code)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	// Redirect to original URL
	g.Redirect(302, originalURL)
}

// TrackClick records a click and redirects
func (c *LinkManager) TrackClick(g *gin.Context) {
	code := g.Param("code")
	if code == "" {
		c.Response.BadRequestMessage(g, "code is required")
		return
	}

	ip := g.ClientIP()
	userAgent := g.GetHeader("User-Agent")
	referer := g.GetHeader("Referer")

	link, err := c.LinkManagerService.TrackClick(code, ip, userAgent, referer, "")
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	g.Redirect(302, link.OriginalURL)
}

// GetAnalytics returns analytics for a short link
func (c *LinkManager) GetAnalytics(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	code := g.Param("code")
	if code == "" {
		c.Response.BadRequestMessage(g, "code is required")
		return
	}

	analytics, err := c.LinkManagerService.GetAnalytics(code)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, analytics)
}

// RotateLinks changes destination URL for multiple short codes
func (c *LinkManager) RotateLinks(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req linkRotateRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	results := c.LinkManagerService.RotateLinks(req.Codes, req.NewURL)
	c.Response.OK(g, results)
}

// GetAllLinks returns all short links
func (c *LinkManager) GetAllLinks(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	campaignID := g.Query("campaignId")
	proxyID := g.Query("proxyId")
	links := c.LinkManagerService.GetAllLinks(campaignID, proxyID)
	c.Response.OK(g, links)
}

// DeleteLink removes a short link
func (c *LinkManager) DeleteLink(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	code := g.Param("code")
	if code == "" {
		c.Response.BadRequestMessage(g, "code is required")
		return
	}

	err := c.LinkManagerService.DeleteLink(code)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, gin.H{"deleted": true})
}
