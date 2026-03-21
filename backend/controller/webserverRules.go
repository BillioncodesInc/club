package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type WebServerRules struct {
	Common
	Service *service.WebServerRulesGenerator
}

func (c *WebServerRules) Generate(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var config service.RulesConfig
	if !c.handleParseRequest(g, &config) {
		return
	}

	result, err := c.Service.Generate(session, &config)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

func (c *WebServerRules) GetServers(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session
	c.Response.OK(g, c.Service.GetSupportedServers())
}
