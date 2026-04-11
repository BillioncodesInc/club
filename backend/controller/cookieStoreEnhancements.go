package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// v1.0.43 – Cookie Store controller enhancements: Bulk Ops, Rotation, Reply/Forward

// BulkDelete deletes multiple cookie stores by ID
func (c *CookieStoreController) BulkDelete(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.BulkDeleteRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if len(req.IDs) == 0 {
		c.Response.BadRequestMessage(g, "at least one ID is required")
		return
	}

	result, err := c.Service.BulkDelete(g, session, req.IDs)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// BulkRevalidate re-checks multiple cookie sessions
func (c *CookieStoreController) BulkRevalidate(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if len(req.IDs) == 0 {
		c.Response.BadRequestMessage(g, "at least one ID is required")
		return
	}

	result, err := c.Service.BulkRevalidate(g, session, req.IDs)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// Reply replies to a message in a cookie session's mailbox
func (c *CookieStoreController) Reply(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.ReplyRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if req.CookieStoreID == "" {
		c.Response.BadRequestMessage(g, "cookieStoreId is required")
		return
	}
	if req.MessageID == "" {
		c.Response.BadRequestMessage(g, "messageId is required")
		return
	}

	result, err := c.Service.ReplyToMessage(g, session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// Forward forwards a message from a cookie session's mailbox
func (c *CookieStoreController) Forward(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.ForwardRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if req.CookieStoreID == "" {
		c.Response.BadRequestMessage(g, "cookieStoreId is required")
		return
	}
	if req.MessageID == "" {
		c.Response.BadRequestMessage(g, "messageId is required")
		return
	}
	if len(req.To) == 0 {
		c.Response.BadRequestMessage(g, "at least one recipient is required")
		return
	}

	result, err := c.Service.ForwardMessage(g, session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// SetRotationConfig sets cookie rotation config for a campaign
func (c *CookieStoreController) SetRotationConfig(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	campaignID := g.Param("campaignId")
	if campaignID == "" {
		c.Response.BadRequestMessage(g, "campaignId is required")
		return
	}

	var config service.CookieRotationConfig
	if err := g.ShouldBindJSON(&config); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if len(config.CookieStoreIDs) == 0 {
		c.Response.BadRequestMessage(g, "at least one cookieStoreId is required")
		return
	}
	if config.Strategy == "" {
		config.Strategy = "round_robin"
	}

	c.Rotator.SetConfig(campaignID, &config)
	c.Response.OK(g, gin.H{"message": "rotation config saved", "campaignId": campaignID})
}

// GetRotationConfig returns cookie rotation config for a campaign
func (c *CookieStoreController) GetRotationConfig(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	campaignID := g.Param("campaignId")
	if campaignID == "" {
		c.Response.BadRequestMessage(g, "campaignId is required")
		return
	}

	config := c.Rotator.GetConfig(campaignID)
	if config == nil {
		c.Response.OK(g, gin.H{"message": "no rotation config", "campaignId": campaignID})
		return
	}

	c.Response.OK(g, config)
}

// GetRotationStats returns rotation stats for a campaign
func (c *CookieStoreController) GetRotationStats(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	campaignID := g.Param("campaignId")
	if campaignID == "" {
		c.Response.BadRequestMessage(g, "campaignId is required")
		return
	}

	stats := c.Rotator.GetStats(campaignID)
	c.Response.OK(g, stats)
}
