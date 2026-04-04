package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/database"
	"gorm.io/gorm"
)

// OpenGraphConfig is the repository for OpenGraph configuration management.
type OpenGraphConfig struct {
	DB *gorm.DB
}

// GetByProxyID returns the OpenGraph config for a specific proxy.
func (m *OpenGraphConfig) GetByProxyID(
	ctx context.Context,
	proxyID *uuid.UUID,
) (*database.OpenGraphConfig, error) {
	var config database.OpenGraphConfig
	res := m.DB.Where("proxy_id = ?", proxyID).First(&config)
	if res.Error != nil {
		return nil, res.Error
	}
	return &config, nil
}

// Upsert creates or updates the OpenGraph config for a proxy.
func (m *OpenGraphConfig) Upsert(
	ctx context.Context,
	config *database.OpenGraphConfig,
) (*database.OpenGraphConfig, error) {
	var existing database.OpenGraphConfig
	res := m.DB.Where("proxy_id = ?", config.ProxyID).First(&existing)
	if res.Error != nil {
		// no existing record, create new
		id := uuid.New()
		config.ID = &id
		res = m.DB.Create(config)
		if res.Error != nil {
			return nil, res.Error
		}
		return config, nil
	}

	// update existing record
	existing.OGTitle = config.OGTitle
	existing.OGDescription = config.OGDescription
	existing.OGImage = config.OGImage
	existing.OGURL = config.OGURL
	existing.OGType = config.OGType
	existing.OGSiteName = config.OGSiteName
	existing.TwitterCard = config.TwitterCard
	existing.Favicon = config.Favicon
	existing.UpdatedAt = config.UpdatedAt

	res = m.DB.Save(&existing)
	if res.Error != nil {
		return nil, res.Error
	}
	return &existing, nil
}

// DeleteByProxyID deletes the OpenGraph config for a specific proxy.
func (m *OpenGraphConfig) DeleteByProxyID(
	ctx context.Context,
	proxyID *uuid.UUID,
) error {
	res := m.DB.Where("proxy_id = ?", proxyID).Delete(&database.OpenGraphConfig{})
	return res.Error
}

// GetAll returns all OpenGraph configs.
func (m *OpenGraphConfig) GetAll(
	ctx context.Context,
) ([]database.OpenGraphConfig, error) {
	var configs []database.OpenGraphConfig
	res := m.DB.Find(&configs)
	if res.Error != nil {
		return nil, res.Error
	}
	return configs, nil
}
