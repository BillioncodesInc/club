package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"gorm.io/gorm"
)

// CookieStoreMessage is the repository for cached cookie store messages
type CookieStoreMessage struct {
	DB *gorm.DB
}

// UpsertMessages replaces all cached messages for a given store+folder with new ones.
// This is called by the background automation to refresh cached data.
func (r *CookieStoreMessage) UpsertMessages(
	ctx context.Context,
	storeID uuid.UUID,
	folder string,
	messages []database.CookieStoreMessage,
) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing messages for this store+folder
		if err := tx.Table(database.COOKIE_STORE_MESSAGE_TABLE).
			Where("cookie_store_id = ? AND folder = ?", storeID, folder).
			Delete(&database.CookieStoreMessage{}).Error; err != nil {
			return err
		}

		// Insert new messages
		if len(messages) > 0 {
			now := time.Now()
			for i := range messages {
				messages[i].ID = uuid.New()
				messages[i].CookieStoreID = storeID
				messages[i].Folder = folder
				messages[i].CreatedAt = &now
				messages[i].ScrapedAt = &now
			}
			if err := tx.Table(database.COOKIE_STORE_MESSAGE_TABLE).
				Create(&messages).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetMessages returns cached messages for a given store+folder with pagination
func (r *CookieStoreMessage) GetMessages(
	ctx context.Context,
	storeID uuid.UUID,
	folder string,
	limit int,
	skip int,
) ([]database.CookieStoreMessage, int, error) {
	var total int64
	db := r.DB.WithContext(ctx).Table(database.COOKIE_STORE_MESSAGE_TABLE).
		Where("cookie_store_id = ? AND folder = ?", storeID, folder)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []database.CookieStoreMessage
	if err := db.Order("date DESC, created_at DESC").
		Offset(skip).Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, int(total), nil
}

// DeleteByStoreID deletes all cached messages for a cookie store
func (r *CookieStoreMessage) DeleteByStoreID(ctx context.Context, storeID uuid.UUID) error {
	return r.DB.WithContext(ctx).Table(database.COOKIE_STORE_MESSAGE_TABLE).
		Where("cookie_store_id = ?", storeID).
		Delete(&database.CookieStoreMessage{}).Error
}

// HasCachedMessages checks if there are any cached messages for a store+folder
func (r *CookieStoreMessage) HasCachedMessages(
	ctx context.Context,
	storeID uuid.UUID,
	folder string,
) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Table(database.COOKIE_STORE_MESSAGE_TABLE).
		Where("cookie_store_id = ? AND folder = ?", storeID, folder).
		Count(&count).Error
	return count > 0, err
}
