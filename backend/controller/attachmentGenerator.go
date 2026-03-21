package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type AttachmentGenerator struct {
	Common
	AttachmentGeneratorService *service.AttachmentGenerator
}

// Generate creates a dynamic attachment
func (c *AttachmentGenerator) Generate(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.AttachmentGenerateRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	attachment, err := c.AttachmentGeneratorService.Generate(&req)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, attachment)
}
