package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type CapturedSessionSender struct {
	Common
	Service *service.CapturedSessionSender
}

func (c *CapturedSessionSender) Send(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.CapturedSendRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	result, err := c.Service.SendAsCapturedSession(g.Request.Context(), session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

func (c *CapturedSessionSender) Validate(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		AccessToken string `json:"accessToken"`
		Provider    string `json:"provider"`
	}
	if !c.handleParseRequest(g, &req) {
		return
	}

	info, err := c.Service.ValidateCapturedSession(g.Request.Context(), session, req.AccessToken, req.Provider)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, info)
}

func (c *CapturedSessionSender) GetProviders(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session
	c.Response.OK(g, c.Service.GetSupportedProviders())
}
