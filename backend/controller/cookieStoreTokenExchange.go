package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TokenExchangeRequest is the request body for token exchange
type TokenExchangeRequest struct {
	Scope string `json:"scope"` // "outlook" or "graph" (default: "graph")
}

// TokenExchange triggers a token exchange for a cookie store session.
// POST /api/v1/cookie-store/:id/token-exchange
// This attempts to extract a refresh token from the stored cookies and
// exchange it for an access token via Microsoft OAuth2.
func (c *CookieStoreController) TokenExchange(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	idStr := g.Param("id")
	storeID, err := uuid.Parse(idStr)
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid cookie store ID")
		return
	}

	var req TokenExchangeRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		// Default to graph scope
		req.Scope = "graph"
	}
	if req.Scope == "" {
		req.Scope = "graph"
	}

	// Fetch the raw store to get cookies
	rawStore, err := c.Service.GetRawByID(g.Request.Context(), storeID)
	if err != nil || rawStore == nil {
		c.Response.NotFound(g)
		return
	}

	// Attempt token exchange using the service's validateViaTokenExchange
	// which extracts refresh tokens from MSRT cookies and exchanges them
	email, displayName, valid := c.Service.ValidateViaTokenExchangePublic(g.Request.Context(), rawStore)

	result := map[string]interface{}{
		"success":     valid,
		"email":       email,
		"displayName": displayName,
		"storeId":     storeID.String(),
		"scope":       req.Scope,
	}

	if valid {
		result["message"] = "Token exchange successful. Access token cached for this session."
	} else {
		result["message"] = "Token exchange failed. No valid refresh token found in cookies."
	}

	c.Response.OK(g, result)
}
