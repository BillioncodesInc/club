package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	OPEN_REDIRECT_TABLE = "open_redirects"
)

// OpenRedirect is a gorm data model for open redirect URLs
// used to bypass email security gateways and Google Safe Browsing
type OpenRedirect struct {
	ID        *uuid.UUID `gorm:"primary_key;not null;unique;type:uuid"`
	CreatedAt *time.Time `gorm:"not null;index;"`
	UpdatedAt *time.Time `gorm:"not null;index"`
	CompanyID *uuid.UUID `gorm:"index;type:uuid"`

	// The base URL of the open redirect (e.g., https://www.google.com/url?q=)
	BaseURL string `gorm:"not null;type:text"`

	// Human-readable name/label
	Name string `gorm:"not null;type:varchar(255)"`

	// The platform/source (e.g., "google", "microsoft", "slack", "custom")
	Platform string `gorm:"not null;type:varchar(64);index"`

	// The parameter name that accepts the redirect target (e.g., "q", "url", "redirect_uri")
	ParamName string `gorm:"not null;type:varchar(64)"`

	// Whether this redirect has been tested and confirmed working.
	// Nullable to preserve nil=unknown ("not tested yet") semantics; defaults to false on write.
	IsVerified *bool `gorm:"default:false;index"`

	// Last time the redirect was tested
	LastTestedAt *time.Time `gorm:"index"`

	// HTTP status code returned during last test (301, 302, etc.)
	LastStatusCode int `gorm:"default:0"`

	// Whether to use this redirect with the proxy domain
	UseWithProxy bool `gorm:"not null;default:false"`

	// The proxy ID to associate with (optional)
	ProxyID *uuid.UUID `gorm:"type:uuid;index"`

	// Notes/description
	Notes string `gorm:"type:text"`

	// could has-one
	Company *Company
	Proxy   *Proxy
}

func (e *OpenRedirect) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(e)
}
