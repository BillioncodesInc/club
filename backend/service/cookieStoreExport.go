package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
)

// GetRawByID returns a cookie store by ID including the raw CookiesJSON field.
// This bypasses the normal authorization check and is intended for internal use
// (e.g., cookie export) where the caller has already verified the session.
func (s *CookieStoreService) GetRawByID(ctx context.Context, id uuid.UUID) (*database.CookieStore, error) {
	return s.CookieStoreRepo.GetByID(ctx, id)
}
