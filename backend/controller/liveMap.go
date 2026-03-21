package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// LiveMap is a controller for the live map dashboard
type LiveMap struct {
	Common
	LiveMapService *service.LiveMap
}

// GetRecentEvents returns recent map events
func (lm *LiveMap) GetRecentEvents(g *gin.Context) {
	session, _, ok := lm.handleSession(g)
	if !ok {
		return
	}

	limit := 50
	if l := g.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := lm.LiveMapService.GetRecentEvents(g.Request.Context(), session, limit)
	if ok := lm.handleErrors(g, err); !ok {
		return
	}

	lm.Response.OK(g, events)
}

// GetMapStats returns aggregate map statistics
func (lm *LiveMap) GetMapStats(g *gin.Context) {
	session, _, ok := lm.handleSession(g)
	if !ok {
		return
	}

	stats, err := lm.LiveMapService.GetMapStats(g.Request.Context(), session)
	if ok := lm.handleErrors(g, err); !ok {
		return
	}

	lm.Response.OK(g, stats)
}
