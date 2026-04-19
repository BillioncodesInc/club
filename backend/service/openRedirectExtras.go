package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/vo"
)

// OpenRedirectStats holds aggregate statistics
type OpenRedirectStats struct {
	Total      int64            `json:"total"`
	Verified   int64            `json:"verified"`
	Unverified int64            `json:"unverified"`
	WithProxy  int64            `json:"withProxy"`
	Platforms  map[string]int64 `json:"platforms"`
}

// GetStats returns aggregate statistics for open redirects
func (s *OpenRedirect) GetStats(ctx context.Context, companyID *uuid.UUID) (*OpenRedirectStats, error) {
	queryArgs := &vo.QueryArgs{
		Offset:  0,
		Limit:   10000,
		OrderBy: "created_at",
		Desc:    true,
	}

	result, err := s.OpenRedirectRepository.GetAll(ctx, companyID, &repository.OpenRedirectOption{
		QueryArgs: queryArgs,
	})
	if err != nil {
		return nil, err
	}

	stats := &OpenRedirectStats{
		Total:     int64(len(result.Rows)),
		Platforms: make(map[string]int64),
	}

	for _, r := range result.Rows {
		if r.IsVerified != nil && *r.IsVerified {
			stats.Verified++
		} else {
			stats.Unverified++
		}
		if r.UseWithProxy != nil && *r.UseWithProxy {
			stats.WithProxy++
		}
		platform := "unknown"
		if r.Platform.IsSpecified() {
			p, _ := r.Platform.Get()
			platform = p.String()
		}
		stats.Platforms[platform]++
	}

	return stats, nil
}

// ToggleActive toggles the UseWithProxy flag for an open redirect
func (s *OpenRedirect) ToggleActive(ctx context.Context, id uuid.UUID, companyID *uuid.UUID) (map[string]interface{}, error) {
	idPtr := &id
	redirect, err := s.OpenRedirectRepository.GetByID(ctx, idPtr, &repository.OpenRedirectOption{})
	if err != nil {
		return nil, err
	}

	// Toggle the UseWithProxy flag
	newValue := true
	if redirect.UseWithProxy != nil && *redirect.UseWithProxy {
		newValue = false
	}
	redirect.UseWithProxy = &newValue

	err = s.OpenRedirectRepository.UpdateByID(ctx, idPtr, redirect)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":           id.String(),
		"useWithProxy": newValue,
	}, nil
}
