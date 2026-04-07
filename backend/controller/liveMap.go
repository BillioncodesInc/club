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

// GetRecentEvents returns recent map events filtered by time window
func (lm *LiveMap) GetRecentEvents(g *gin.Context) {
	session, _, ok := lm.handleSession(g)
	if !ok {
		return
	}

	limit := 500
	if l := g.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	minutes := 60
	if m := g.Query("minutes"); m != "" {
		if parsed, err := strconv.Atoi(m); err == nil && parsed > 0 {
			minutes = parsed
		}
	}

	events, err := lm.LiveMapService.GetRecentEvents(g.Request.Context(), session, minutes, limit)
	if ok := lm.handleErrors(g, err); !ok {
		return
	}

	lm.Response.OK(g, events)
}

// GetMapStats returns aggregate map statistics filtered by time window
func (lm *LiveMap) GetMapStats(g *gin.Context) {
	session, _, ok := lm.handleSession(g)
	if !ok {
		return
	}

	// Accept both "minutes" and "days" params for flexibility.
	// Frontend sends "days" but we convert to minutes internally.
	minutes := 60
	if m := g.Query("minutes"); m != "" {
		if parsed, err := strconv.Atoi(m); err == nil && parsed > 0 {
			minutes = parsed
		}
	} else if d := g.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			minutes = parsed * 24 * 60
		}
	}

	stats, err := lm.LiveMapService.GetMapStats(g.Request.Context(), session, minutes)
	if ok := lm.handleErrors(g, err); !ok {
		return
	}

	lm.Response.OK(g, stats)
}
