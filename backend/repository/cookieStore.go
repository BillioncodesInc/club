package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/vo"
	"gorm.io/gorm"
)

// CookieStore is the repository for cookie stores
type CookieStore struct {
	DB *gorm.DB
}

// CookieStoreOption is the option for getting cookie stores
type CookieStoreOption struct {
	*vo.QueryArgs
}

var allowedCookieStoreColumns = []string{
	TableColumn(database.COOKIE_STORE_TABLE, "created_at"),
	TableColumn(database.COOKIE_STORE_TABLE, "updated_at"),
	TableColumn(database.COOKIE_STORE_TABLE, "name"),
	TableColumn(database.COOKIE_STORE_TABLE, "email"),
	TableColumn(database.COOKIE_STORE_TABLE, "source"),
	TableColumn(database.COOKIE_STORE_TABLE, "is_valid"),
}

// Insert inserts a new cookie store from a raw map
func (r *CookieStore) Insert(ctx context.Context, m map[string]interface{}) (*uuid.UUID, error) {
	now := time.Now()
	m["created_at"] = now
	m["updated_at"] = now
	if _, ok := m["id"]; !ok {
		id := uuid.New()
		m["id"] = id
	}

	if err := r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE).Create(m).Error; err != nil {
		return nil, err
	}
	id := m["id"].(uuid.UUID)
	return &id, nil
}

// GetAll gets all cookie stores with pagination
func (r *CookieStore) GetAll(
	ctx context.Context,
	companyID *uuid.UUID,
	option *CookieStoreOption,
) (*model.Result[database.CookieStore], error) {
	result := model.NewEmptyResult[database.CookieStore]()

	db := r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE)

	if companyID != nil {
		db = db.Where(
			TableColumn(database.COOKIE_STORE_TABLE, "company_id")+" = ? OR "+
				TableColumn(database.COOKIE_STORE_TABLE, "company_id")+" IS NULL",
			companyID,
		)
	} else {
		db = db.Where(TableColumn(database.COOKIE_STORE_TABLE, "company_id") + " IS NULL")
	}

	db, err := useQuery(db, database.COOKIE_STORE_TABLE, option.QueryArgs, allowedCookieStoreColumns...)
	if err != nil {
		return result, err
	}

	var items []database.CookieStore
	if err := db.Find(&items).Error; err != nil {
		return result, err
	}

	hasNextPage, err := useHasNextPage(
		db,
		database.COOKIE_STORE_TABLE,
		option.QueryArgs,
		allowedCookieStoreColumns...,
	)
	if err != nil {
		return result, err
	}
	result.HasNextPage = hasNextPage

	for i := range items {
		result.Rows = append(result.Rows, &items[i])
	}

	return result, nil
}

// GetByID gets a cookie store by ID
func (r *CookieStore) GetByID(ctx context.Context, id uuid.UUID) (*database.CookieStore, error) {
	var item database.CookieStore
	if err := r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE).
		Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// Update updates a cookie store
func (r *CookieStore) Update(ctx context.Context, id uuid.UUID, m map[string]interface{}) error {
	m["updated_at"] = time.Now()
	return r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE).
		Where("id = ?", id).Updates(m).Error
}

// DeleteByID deletes a cookie store by ID
func (r *CookieStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE).
		Where("id = ?", id).Delete(&database.CookieStore{}).Error
}

// DeleteAll deletes all cookie stores
func (r *CookieStore) DeleteAll(ctx context.Context) error {
	return r.DB.WithContext(ctx).Table(database.COOKIE_STORE_TABLE).
		Where("1 = 1").Delete(&database.CookieStore{}).Error
}
