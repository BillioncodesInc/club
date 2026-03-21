package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type ContentBalancer struct {
	Common
	Service *service.ContentBalancer
}

func (c *ContentBalancer) Balance(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.BalanceRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	result, err := c.Service.Balance(session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

func (c *ContentBalancer) SpinContent(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	_ = session

	var req struct {
		Template string `json:"template"`
	}
	if !c.handleParseRequest(g, &req) {
		return
	}

	result := c.Service.SpinContent(req.Template)
	c.Response.OK(g, map[string]string{"result": result})
}
