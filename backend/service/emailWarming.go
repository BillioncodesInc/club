package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// EmailWarming implements gradual volume increase for new sending domains/IPs
// to build sender reputation before full campaign launches.
//
// Ported from: ghostsenderintegration/ghost-sender-node/services/warming-engine.js
//
// This service does NOT duplicate existing campaign scheduling — it provides
// a warming schedule calculator and status tracker that works alongside
// the existing Campaign.SendNextBatch() flow.
type EmailWarming struct {
	Common
	OptionService *Option
}

// WarmingPlan represents a warming schedule for a sender
type WarmingPlan struct {
	SenderID    string         `json:"senderID"`
	SenderType  string         `json:"senderType"` // "smtp" or "api"
	SenderName  string         `json:"senderName"`
	StartDate   time.Time      `json:"startDate"`
	CurrentDay  int            `json:"currentDay"`
	TotalDays   int            `json:"totalDays"`
	DailyLimit  int            `json:"dailyLimit"`  // current day's limit
	TotalSent   int            `json:"totalSent"`   // total sent in warming period
	Schedule    []WarmingDay   `json:"schedule"`
	Status      string         `json:"status"` // "active", "completed", "paused"
}

// WarmingDay represents a single day in the warming schedule
type WarmingDay struct {
	Day       int    `json:"day"`
	Volume    int    `json:"volume"`
	Interval  string `json:"interval"` // time between sends
	Status    string `json:"status"`   // "pending", "in_progress", "completed"
	SentCount int    `json:"sentCount"`
}

// WarmingConfig holds configuration for a warming plan
type WarmingConfig struct {
	StartVolume    int     `json:"startVolume"`    // emails per day on day 1 (default: 5)
	MaxVolume      int     `json:"maxVolume"`      // target daily volume (default: 500)
	GrowthRate     float64 `json:"growthRate"`     // daily growth multiplier (default: 1.5)
	DaysToComplete int     `json:"daysToComplete"` // max warming days (default: 21)
}

// DefaultWarmingConfig returns the default warming configuration
func DefaultWarmingConfig() WarmingConfig {
	return WarmingConfig{
		StartVolume:    5,
		MaxVolume:      500,
		GrowthRate:     1.5,
		DaysToComplete: 21,
	}
}

// optionKey returns the option key for storing warming plan state
func warmingOptionKey(senderID string) string {
	return fmt.Sprintf("warming_plan_%s", senderID)
}

// GenerateWarmingPlan creates a warming schedule for a sender
func (e *EmailWarming) GenerateWarmingPlan(
	ctx context.Context,
	session *model.Session,
	senderID uuid.UUID,
	senderType string,
	senderName string,
	config WarmingConfig,
) (*WarmingPlan, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	if config.StartVolume <= 0 {
		config.StartVolume = 5
	}
	if config.MaxVolume <= 0 {
		config.MaxVolume = 500
	}
	if config.GrowthRate <= 1.0 {
		config.GrowthRate = 1.5
	}
	if config.DaysToComplete <= 0 {
		config.DaysToComplete = 21
	}

	schedule := make([]WarmingDay, 0)
	currentVolume := float64(config.StartVolume)

	for day := 1; day <= config.DaysToComplete; day++ {
		volume := int(math.Min(currentVolume, float64(config.MaxVolume)))

		// Calculate interval between sends to spread them throughout the day
		var interval string
		if volume <= 10 {
			interval = "2h"
		} else if volume <= 50 {
			interval = "30m"
		} else if volume <= 200 {
			interval = "10m"
		} else {
			interval = "5m"
		}

		schedule = append(schedule, WarmingDay{
			Day:       day,
			Volume:    volume,
			Interval:  interval,
			Status:    "pending",
			SentCount: 0,
		})

		if volume >= config.MaxVolume {
			break // reached target volume
		}

		currentVolume *= config.GrowthRate
	}

	plan := &WarmingPlan{
		SenderID:   senderID.String(),
		SenderType: senderType,
		SenderName: senderName,
		StartDate:  time.Now(),
		CurrentDay: 1,
		TotalDays:  len(schedule),
		DailyLimit: schedule[0].Volume,
		TotalSent:  0,
		Schedule:   schedule,
		Status:     "active",
	}

	return plan, nil
}

// GetCurrentDayLimit returns the sending limit for the current day of warming
func (e *EmailWarming) GetCurrentDayLimit(plan *WarmingPlan) int {
	if plan.Status != "active" {
		return 0
	}

	daysSinceStart := int(time.Since(plan.StartDate).Hours()/24) + 1
	if daysSinceStart > len(plan.Schedule) {
		return plan.Schedule[len(plan.Schedule)-1].Volume // use max volume
	}

	return plan.Schedule[daysSinceStart-1].Volume
}

// GetWarmingStatus returns a summary of the warming progress
func (e *EmailWarming) GetWarmingStatus(plan *WarmingPlan) map[string]interface{} {
	daysSinceStart := int(time.Since(plan.StartDate).Hours()/24) + 1
	progress := float64(daysSinceStart) / float64(plan.TotalDays) * 100
	if progress > 100 {
		progress = 100
	}

	currentLimit := e.GetCurrentDayLimit(plan)

	return map[string]interface{}{
		"senderID":     plan.SenderID,
		"senderType":   plan.SenderType,
		"senderName":   plan.SenderName,
		"status":       plan.Status,
		"currentDay":   daysSinceStart,
		"totalDays":    plan.TotalDays,
		"dailyLimit":   currentLimit,
		"totalSent":    plan.TotalSent,
		"progress":     fmt.Sprintf("%.1f%%", progress),
		"startDate":    plan.StartDate.Format(time.RFC3339),
		"isCompleted":  daysSinceStart >= plan.TotalDays,
	}
}

// CalculateOptimalSchedule returns a recommended warming schedule based on
// the target volume and sender reputation factors
func (e *EmailWarming) CalculateOptimalSchedule(
	targetDailyVolume int,
	isNewDomain bool,
	hasExistingReputation bool,
) WarmingConfig {
	config := DefaultWarmingConfig()
	config.MaxVolume = targetDailyVolume

	if isNewDomain && !hasExistingReputation {
		// Brand new domain: slow and steady
		config.StartVolume = 2
		config.GrowthRate = 1.3
		config.DaysToComplete = 30
	} else if isNewDomain && hasExistingReputation {
		// New domain but established sender: moderate pace
		config.StartVolume = 10
		config.GrowthRate = 1.5
		config.DaysToComplete = 21
	} else {
		// Existing domain, just increasing volume: faster pace
		config.StartVolume = 20
		config.GrowthRate = 1.8
		config.DaysToComplete = 14
	}

	return config
}
