package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetStats returns aggregate statistics for open redirects
func (m *OpenRedirectCtrl) GetStats(g *gin.Context) {
	_, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	companyID := companyIDFromRequestQuery(g)
	stats, err := m.OpenRedirectService.GetStats(g.Request.Context(), companyID)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, stats)
}

// ToggleActive toggles the UseWithProxy flag for an open redirect
func (m *OpenRedirectCtrl) ToggleActive(g *gin.Context) {
	_, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequestMessage(g, "Invalid ID")
		return
	}
	companyID := companyIDFromRequestQuery(g)
	result, err := m.OpenRedirectService.ToggleActive(g.Request.Context(), id, companyID)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, result)
}
