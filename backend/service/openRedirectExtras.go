package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
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

// statsPageSize is the batch size used when paginating through open redirects
// for aggregate stats — avoids silently dropping rows in large tenants.
const statsPageSize = 500

// GetStats returns aggregate statistics for open redirects
func (s *OpenRedirect) GetStats(ctx context.Context, companyID *uuid.UUID) (*OpenRedirectStats, error) {
	stats := &OpenRedirectStats{
		Platforms: make(map[string]int64),
	}

	offset := 0
	for {
		queryArgs := &vo.QueryArgs{
			Offset:  offset,
			Limit:   statsPageSize,
			OrderBy: "created_at",
			Desc:    true,
		}

		result, err := s.OpenRedirectRepository.GetAll(ctx, companyID, &repository.OpenRedirectOption{
			QueryArgs: queryArgs,
		})
		if err != nil {
			return nil, errs.Wrap(err)
		}

		rowsInPage := len(result.Rows)
		if rowsInPage == 0 {
			break
		}

		stats.Total += int64(rowsInPage)
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

		if rowsInPage < statsPageSize {
			break
		}
		offset += rowsInPage
	}

	return stats, nil
}

// ToggleActive toggles the UseWithProxy flag for an open redirect
func (s *OpenRedirect) ToggleActive(ctx context.Context, session *model.Session, id uuid.UUID, companyID *uuid.UUID) (map[string]interface{}, error) {
	ae := NewAuditEvent("OpenRedirect.ToggleActive", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	idPtr := &id
	redirect, err := s.OpenRedirectRepository.GetByID(ctx, idPtr, &repository.OpenRedirectOption{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Toggle the UseWithProxy flag
	newValue := true
	if redirect.UseWithProxy != nil && *redirect.UseWithProxy {
		newValue = false
	}
	redirect.UseWithProxy = &newValue

	err = s.OpenRedirectRepository.UpdateByID(ctx, idPtr, redirect)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	s.AuditLogAuthorized(ae)
	return map[string]interface{}{
		"id":           id.String(),
		"useWithProxy": newValue,
	}, nil
}
