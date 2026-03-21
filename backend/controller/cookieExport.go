package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/service"
)

// CookieExport is the controller for exporting captured cookies.
type CookieExport struct {
	Common
	CookieExportService *service.CookieExport
	CampaignService     *service.Campaign
}

// ExportByEventID exports cookies from a specific campaign event.
// GET /api/cookie-export/:eventID?format=json|netscape
// The eventID is used as a label; the caller must also provide campaignID as a query param.
func (c *CookieExport) ExportByEventID(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	eventID := g.Param("eventID")
	if eventID == "" {
		c.Response.BadRequest(g)
		return
	}

	campaignIDStr := g.Query("campaignId")
	if campaignIDStr == "" {
		c.Response.BadRequestMessage(g, "campaignId query parameter is required")
		return
	}

	format := service.CookieExportFormat(g.DefaultQuery("format", "json"))

	// parse campaign ID
	campaignUUID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid campaignId")
		return
	}

	// fetch events for this campaign and find the matching event
	events, err := c.CampaignService.GetEventsByCampaignID(
		g.Request.Context(), session, &campaignUUID, nil, nil, nil,
	)
	if err != nil {
		c.Response.NotFound(g)
		return
	}

	// find the event with matching ID
	var capturedData map[string]interface{}
	for _, event := range events.Rows {
		if event.ID != nil && event.ID.String() == eventID {
			if event.Data != nil {
				_ = json.Unmarshal([]byte(event.Data.String()), &capturedData)
			}
			break
		}
	}

	if len(capturedData) == 0 {
		g.JSON(http.StatusNotFound, gin.H{"error": "no captured data in this event"})
		return
	}

	// extract target domain from captured data
	targetDomain := ""
	if td, ok := capturedData["target_domain"].(string); ok {
		targetDomain = td
	}

	filename, content, err := c.CookieExportService.ExportCookiesFromCapturedData(
		capturedData,
		targetDomain,
		eventID,
		format,
	)
	if err != nil {
		g.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// set download headers
	contentType := "application/json"
	if format == service.CookieExportFormatNetscape {
		contentType = "text/plain"
	}

	g.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	g.Data(http.StatusOK, contentType, content)
}
