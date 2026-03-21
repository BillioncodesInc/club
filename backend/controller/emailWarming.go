package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/service"
)

// EmailWarming is the controller for email warming features
type EmailWarming struct {
	Common
	EmailWarmingService *service.EmailWarming
}

// GeneratePlanRequest is the request body for generating a warming plan
type GeneratePlanRequest struct {
	SenderID    string                 `json:"senderID"`
	SenderType  string                 `json:"senderType"` // "smtp" or "api"
	SenderName  string                 `json:"senderName"`
	Config      service.WarmingConfig  `json:"config"`
}

// CalculateScheduleRequest is the request body for calculating optimal schedule
type CalculateScheduleRequest struct {
	TargetDailyVolume     int  `json:"targetDailyVolume"`
	IsNewDomain           bool `json:"isNewDomain"`
	HasExistingReputation bool `json:"hasExistingReputation"`
}

// GeneratePlan generates a warming plan for a sender
func (e *EmailWarming) GeneratePlan(g *gin.Context) {
	session, _, ok := e.handleSession(g)
	if !ok {
		return
	}

	var req GeneratePlanRequest
	if ok := e.handleParseRequest(g, &req); !ok {
		return
	}

	senderID, err := uuid.Parse(req.SenderID)
	if err != nil {
		e.Response.BadRequestMessage(g, "Invalid sender ID")
		return
	}

	plan, err := e.EmailWarmingService.GenerateWarmingPlan(
		g.Request.Context(),
		session,
		senderID,
		req.SenderType,
		req.SenderName,
		req.Config,
	)
	if !e.handleErrors(g, err) {
		return
	}

	e.Response.OK(g, plan)
}

// CalculateSchedule returns a recommended warming schedule
func (e *EmailWarming) CalculateSchedule(g *gin.Context) {
	_, _, ok := e.handleSession(g)
	if !ok {
		return
	}

	var req CalculateScheduleRequest
	if ok := e.handleParseRequest(g, &req); !ok {
		return
	}

	config := e.EmailWarmingService.CalculateOptimalSchedule(
		req.TargetDailyVolume,
		req.IsNewDomain,
		req.HasExistingReputation,
	)

	// Also generate a preview plan
	plan, err := e.EmailWarmingService.GenerateWarmingPlan(
		g.Request.Context(),
		nil, // no session needed for preview
		uuid.Nil,
		"preview",
		"Preview",
		config,
	)
	if err != nil {
		e.Response.OK(g, map[string]interface{}{
			"config": config,
		})
		return
	}

	e.Response.OK(g, map[string]interface{}{
		"config":   config,
		"schedule": plan.Schedule,
	})
}
