package service

import (
	"strings"

	"github.com/phishingclub/phishingclub/model"
)

// deduplicateRecipientsBatch removes duplicate recipients from a batch
// before the DB check loop, using email as the dedup key.
// It keeps the first occurrence of each email address.
func deduplicateRecipientsBatch(recipients []*model.Recipient) []*model.Recipient {
	if len(recipients) <= 1 {
		return recipients
	}

	seen := make(map[string]bool, len(recipients))
	result := make([]*model.Recipient, 0, len(recipients))

	for _, r := range recipients {
		email, err := r.Email.Get()
		if err != nil {
			// Keep recipients without valid email - they'll fail validation later
			result = append(result, r)
			continue
		}

		key := strings.ToLower(strings.TrimSpace(email.String()))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, r)
	}

	return result
}
