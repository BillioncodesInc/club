package database

import (
	"time"

	"github.com/google/uuid"
)

const (
	OPENGRAPH_CONFIG_TABLE = "opengraph_configs"
)

// OpenGraphConfig stores OpenGraph meta tag configuration per proxy.
// When a visitor or link preview bot accesses a proxy domain, these tags
// are injected into the HTML <head> to control how the link appears
// in social media previews, messaging apps, and other platforms.
type OpenGraphConfig struct {
	ID        *uuid.UUID `gorm:"primary_key;not null;unique;type:uuid" json:"id"`
	CreatedAt *time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt *time.Time `gorm:"not null" json:"updatedAt"`

	// ProxyID links this config to a specific proxy configuration.
	ProxyID *uuid.UUID `gorm:"uniqueIndex;not null;type:uuid" json:"proxyId"`
	Proxy   *Proxy     `gorm:"foreignKey:ProxyID" json:"-"`

	// OGTitle is the og:title meta tag value (page title in previews).
	OGTitle string `gorm:"type:text;default:''" json:"ogTitle"`

	// OGDescription is the og:description meta tag value (description in previews).
	OGDescription string `gorm:"type:text;default:''" json:"ogDescription"`

	// OGImage is the og:image meta tag value (preview image URL).
	OGImage string `gorm:"type:text;default:''" json:"ogImage"`

	// OGURL is the og:url meta tag value (canonical URL shown in previews).
	OGURL string `gorm:"type:text;default:''" json:"ogUrl"`

	// OGType is the og:type meta tag value (e.g., "website", "article").
	OGType string `gorm:"type:text;default:'website'" json:"ogType"`

	// OGSiteName is the og:site_name meta tag value (site name in previews).
	OGSiteName string `gorm:"type:text;default:''" json:"ogSiteName"`

	// TwitterCard is the twitter:card meta tag value (e.g., "summary_large_image").
	TwitterCard string `gorm:"type:text;default:'summary_large_image'" json:"twitterCard"`

	// Favicon is an optional custom favicon URL for the proxy domain.
	Favicon string `gorm:"type:text;default:''" json:"favicon"`
}

func (OpenGraphConfig) TableName() string {
	return OPENGRAPH_CONFIG_TABLE
}
