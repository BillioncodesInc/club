package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/service"
)

// OpenRedirectColumnsMap is a map between the frontend and the backend
var OpenRedirectColumnsMap = map[string]string{
	"created_at":     repository.TableColumn(database.OPEN_REDIRECT_TABLE, "created_at"),
	"updated_at":     repository.TableColumn(database.OPEN_REDIRECT_TABLE, "updated_at"),
	"name":           repository.TableColumn(database.OPEN_REDIRECT_TABLE, "name"),
	"platform":       repository.TableColumn(database.OPEN_REDIRECT_TABLE, "platform"),
	"is_verified":    repository.TableColumn(database.OPEN_REDIRECT_TABLE, "is_verified"),
	"use_with_proxy": repository.TableColumn(database.OPEN_REDIRECT_TABLE, "use_with_proxy"),
}

// OpenRedirectCtrl is an open redirect controller
type OpenRedirectCtrl struct {
	Common
	OpenRedirectService *service.OpenRedirect
}

// Create creates an open redirect
func (m *OpenRedirectCtrl) Create(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	var req model.OpenRedirect
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}
	id, err := m.OpenRedirectService.Create(g.Request.Context(), session, &req)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, map[string]string{"id": id.String()})
}

// GetOverview gets open redirects overview using pagination
func (m *OpenRedirectCtrl) GetOverview(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	queryArgs, ok := m.handleQueryArgs(g)
	if !ok {
		return
	}
	queryArgs.DefaultSortByUpdatedAt()
	companyID := companyIDFromRequestQuery(g)
	redirects, err := m.OpenRedirectService.GetAllOverview(companyID, g.Request.Context(), session, queryArgs)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, redirects)
}

// GetByID gets an open redirect by ID
func (m *OpenRedirectCtrl) GetByID(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequest(g)
		return
	}
	redirect, err := m.OpenRedirectService.GetByID(g.Request.Context(), session, &id)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, redirect)
}

// UpdateByID updates an open redirect
func (m *OpenRedirectCtrl) UpdateByID(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequest(g)
		return
	}
	var req model.OpenRedirect
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}
	err = m.OpenRedirectService.UpdateByID(g.Request.Context(), session, &id, &req)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, nil)
}

// DeleteByID deletes an open redirect
func (m *OpenRedirectCtrl) DeleteByID(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequest(g)
		return
	}
	err = m.OpenRedirectService.DeleteByID(g.Request.Context(), session, &id)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, nil)
}

// TestRedirect tests an open redirect by ID
func (m *OpenRedirectCtrl) TestRedirect(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequest(g)
		return
	}
	result, err := m.OpenRedirectService.TestRedirect(g.Request.Context(), session, &id)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, result)
}

// TestURL tests an arbitrary URL without saving
func (m *OpenRedirectCtrl) TestURL(g *gin.Context) {
	_, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	var req struct {
		BaseURL   string `json:"baseURL"`
		ParamName string `json:"paramName"`
	}
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}
	if req.BaseURL == "" || req.ParamName == "" {
		m.Response.BadRequestMessage(g, "baseURL and paramName are required")
		return
	}
	result, err := m.OpenRedirectService.TestURL(g.Request.Context(), req.BaseURL, req.ParamName)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, result)
}

// GenerateLink generates a redirect link for a target URL
func (m *OpenRedirectCtrl) GenerateLink(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		m.Response.BadRequest(g)
		return
	}
	var req struct {
		TargetURL string `json:"targetURL"`
	}
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}
	if req.TargetURL == "" {
		m.Response.BadRequestMessage(g, "targetURL is required")
		return
	}
	link, err := m.OpenRedirectService.GenerateRedirectLink(g.Request.Context(), session, &id, req.TargetURL)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, map[string]string{"redirectURL": link})
}

// GetKnownSources returns known open redirect sources
func (m *OpenRedirectCtrl) GetKnownSources(g *gin.Context) {
	_, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	m.Response.OK(g, m.OpenRedirectService.GetKnownSources())
}

// GetRecommendations returns open-source tool recommendations
func (m *OpenRedirectCtrl) GetRecommendations(g *gin.Context) {
	_, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	m.Response.OK(g, m.OpenRedirectService.GetOpenSourceRecommendations())
}

// ImportSource imports a known source as a new redirect entry.
// Accepts either {"source_id": "google-search"} or a full OpenRedirectSource object.
func (m *OpenRedirectCtrl) ImportSource(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		SourceID string `json:"source_id"`
	}
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}

	// Look up the source from the known sources list
	var source *model.OpenRedirectSource
	for _, s := range m.OpenRedirectService.GetKnownSources() {
		if s.ID == req.SourceID {
			source = &s
			break
		}
	}
	if source == nil {
		m.Response.BadRequestMessage(g, "unknown source ID: "+req.SourceID)
		return
	}

	companyID := companyIDFromRequestQuery(g)
	id, err := m.OpenRedirectService.ImportFromSource(g.Request.Context(), session, source, companyID)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, map[string]string{"id": id.String(), "imported": "1"})
}

// BulkTest tests multiple redirects at once
func (m *OpenRedirectCtrl) BulkTest(g *gin.Context) {
	session, _, ok := m.handleSession(g)
	if !ok {
		return
	}
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if ok := m.handleParseRequest(g, &req); !ok {
		return
	}
	if len(req.IDs) == 0 {
		m.Response.BadRequestMessage(g, "at least one ID is required")
		return
	}
	results, err := m.OpenRedirectService.BulkTest(g.Request.Context(), session, req.IDs)
	if ok := m.handleErrors(g, err); !ok {
		return
	}
	m.Response.OK(g, results)
}
