package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// EnhancedHeaders is the controller for enhanced email header generation
type EnhancedHeaders struct {
	Common
	EnhancedHeadersService *service.EnhancedHeaders
}

// GenerateHeadersRequest is the request body for generating headers
type GenerateHeadersRequest struct {
	FromDomain string `json:"fromDomain"`
	FromEmail  string `json:"fromEmail"`
	ToEmail    string `json:"toEmail"`
	Profile    string `json:"profile"` // "exchange", "google", "generic", or "all"
}

// Generate generates email headers for the specified profile
func (e *EnhancedHeaders) Generate(g *gin.Context) {
	session, _, ok := e.handleSession(g)
	if !ok {
		return
	}

	var req GenerateHeadersRequest
	if ok := e.handleParseRequest(g, &req); !ok {
		return
	}

	if req.FromDomain == "" || req.FromEmail == "" {
		e.Response.BadRequestMessage(g, "fromDomain and fromEmail are required")
		return
	}

	ctx := g.Request.Context()

	switch req.Profile {
	case "exchange":
		profile, err := e.EnhancedHeadersService.GenerateExchangeHeaders(ctx, session, req.FromDomain, req.FromEmail, req.ToEmail)
		if !e.handleErrors(g, err) {
			return
		}
		e.Response.OK(g, profile)

	case "google":
		profile, err := e.EnhancedHeadersService.GenerateGoogleHeaders(ctx, session, req.FromDomain, req.FromEmail, req.ToEmail)
		if !e.handleErrors(g, err) {
			return
		}
		e.Response.OK(g, profile)

	case "generic":
		profile, err := e.EnhancedHeadersService.GenerateGenericHeaders(ctx, session, req.FromDomain, req.FromEmail, req.ToEmail)
		if !e.handleErrors(g, err) {
			return
		}
		e.Response.OK(g, profile)

	default:
		// Return all profiles
		profiles, err := e.EnhancedHeadersService.GenerateAllProfiles(ctx, session, req.FromDomain, req.FromEmail, req.ToEmail)
		if !e.handleErrors(g, err) {
			return
		}
		e.Response.OK(g, profiles)
	}
}
