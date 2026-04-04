package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	COOKIE_STORE_TABLE = "cookie_stores"
)

// CookieStore is the gorm data model for stored captured cookies.
// Each record represents a set of captured browser cookies from a single
// Outlook/Microsoft session, captured either via the Chrome Extension,
// imported manually, or extracted from proxy captures.
type CookieStore struct {
	ID        uuid.UUID  `gorm:"primary_key;not null;unique;type:uuid" json:"id"`
	CreatedAt *time.Time `gorm:"not null;index;" json:"createdAt"`
	UpdatedAt *time.Time `gorm:"not null;index;" json:"updatedAt"`

	// human-readable label
	Name string `gorm:"not null;type:varchar(255);" json:"name"`

	// source of the cookies: "extension", "import", "proxy_capture"
	Source string `gorm:"not null;type:varchar(50);default:'import';" json:"source"`

	// the raw cookies stored as JSON array of cookie objects
	// each cookie: {name, value, domain, path, secure, httpOnly, sameSite, expirationDate}
	CookiesJSON string `gorm:"not null;type:text;" json:"-"`

	// session validation metadata
	Email       string     `gorm:"type:varchar(255);" json:"email"`
	DisplayName string     `gorm:"type:varchar(255);" json:"displayName"`
	IsValid     bool       `gorm:"not null;default:false;" json:"isValid"`
	LastChecked *time.Time `gorm:"index;" json:"lastChecked"`

	// how the session was validated: "cookie", "token_exchange", ""
	ValidationMethod string `gorm:"type:varchar(50);default:'';" json:"validationMethod"`

	// cached access token obtained via token exchange (from MSRT refresh token)
	// this is short-lived (typically 1 hour) and refreshed on demand
	AccessToken  string     `gorm:"type:text;" json:"-"`
	RefreshToken string     `gorm:"type:text;" json:"-"`
	TokenExpiry  *time.Time `json:"tokenExpiry,omitempty"`

	// cookie count for display
	CookieCount int `gorm:"not null;default:0;" json:"cookieCount"`

	// optional link to proxy capture that produced these cookies
	ProxyCaptureID *uuid.UUID `gorm:"index;" json:"proxyCaptureId"`

	// can belong-to a company
	CompanyID *uuid.UUID `gorm:"index;" json:"companyId"`
	Company   *Company   `gorm:"foreignkey:CompanyID;" json:"-"`
}

// Migrate runs extra migrations for the cookie_stores table
func (c *CookieStore) Migrate(db *gorm.DB) error {
	return nil
}
