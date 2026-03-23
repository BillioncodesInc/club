package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/vo"
	"gorm.io/gorm"
)

var proxyCaptureAllowedColumns = assignTableToColumns(database.PROXY_CAPTURE_TABLE, []string{
	"created_at",
	"updated_at",
	"ip_address",
	"username",
	"phish_domain",
	"target_domain",
})

// ProxyCaptureOption is for eager loading and query options
type ProxyCaptureOption struct {
	*vo.QueryArgs
}

// ProxyCapture is a proxy capture repository
type ProxyCapture struct {
	DB *gorm.DB
}

// Insert inserts a proxy capture record
func (m *ProxyCapture) Insert(
	ctx context.Context,
	capture *database.ProxyCapture,
) (*uuid.UUID, error) {
	id := uuid.New()
	capture.ID = &id
	res := m.DB.Create(capture)
	if res.Error != nil {
		return nil, res.Error
	}
	return &id, nil
}

// GetAll gets all proxy captures with pagination
func (m *ProxyCapture) GetAll(
	ctx context.Context,
	options *ProxyCaptureOption,
) ([]database.ProxyCapture, bool, error) {
	var captures []database.ProxyCapture
	db := m.DB.Model(&database.ProxyCapture{})

	db, err := useQuery(db, database.PROXY_CAPTURE_TABLE, options.QueryArgs, proxyCaptureAllowedColumns...)
	if err != nil {
		return nil, false, err
	}

	dbRes := db.Find(&captures)
	if dbRes.Error != nil {
		return nil, false, dbRes.Error
	}

	hasNextPage, err := useHasNextPage(db, database.PROXY_CAPTURE_TABLE, options.QueryArgs, proxyCaptureAllowedColumns...)
	if err != nil {
		return nil, false, err
	}

	return captures, hasNextPage, nil
}

// GetByID gets a proxy capture by id
func (m *ProxyCapture) GetByID(
	ctx context.Context,
	id *uuid.UUID,
) (*database.ProxyCapture, error) {
	var capture database.ProxyCapture
	res := m.DB.Where("id = ?", id).First(&capture)
	if res.Error != nil {
		return nil, res.Error
	}
	return &capture, nil
}

// DeleteByID deletes a proxy capture by id
func (m *ProxyCapture) DeleteByID(
	ctx context.Context,
	id *uuid.UUID,
) error {
	res := m.DB.Delete(&database.ProxyCapture{}, id)
	return res.Error
}

// DeleteAll deletes all proxy captures
func (m *ProxyCapture) DeleteAll(ctx context.Context) error {
	res := m.DB.Where("1 = 1").Delete(&database.ProxyCapture{})
	return res.Error
}
