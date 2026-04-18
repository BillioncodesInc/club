package service

import (
	"context"

	"github.com/phishingclub/phishingclub/database"
)

// ValidateViaTokenExchangePublic is a public wrapper around validateViaTokenExchange
// for use by controllers that need to trigger token exchange explicitly.
func (s *CookieStoreService) ValidateViaTokenExchangePublic(
	ctx context.Context,
	store *database.CookieStore,
) (email, displayName string, valid bool) {
	return s.validateViaTokenExchange(ctx, store)
}
