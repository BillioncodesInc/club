package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/service"
)

// ExportFromStore exports cookies from a cookie store entry.
// GET /api/v1/cookie-store/:id/export?format=json|netscape|header|console
func (c *CookieStoreController) ExportFromStore(g *gin.Context) {
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

	format := service.CookieExportFormat(g.DefaultQuery("format", "json"))

	// Fetch the cookie store
	store, err := c.Service.GetByID(g.Request.Context(), nil, storeID)
	if err != nil {
		c.Response.NotFound(g)
		return
	}

	if store == nil {
		c.Response.NotFound(g)
		return
	}

	// We need the raw CookiesJSON which is not exposed via the normal GetByID
	// Use the repo directly through the service
	rawStore, err := c.Service.GetRawByID(g.Request.Context(), storeID)
	if err != nil || rawStore == nil {
		c.Response.NotFound(g)
		return
	}

	// Use the CookieExport service to format
	exportService := &service.CookieExport{}
	filename, content, err := exportService.ExportFromCookieStore(
		rawStore.CookiesJSON,
		rawStore.Name,
		rawStore.ID.String(),
		format,
	)
	if err != nil {
		g.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Set download headers
	contentType := "application/json"
	switch format {
	case service.CookieExportFormatNetscape, service.CookieExportFormatHeader:
		contentType = "text/plain"
	case service.CookieExportFormatConsole:
		contentType = "application/javascript"
	}

	g.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	g.Data(http.StatusOK, contentType, content)
}
