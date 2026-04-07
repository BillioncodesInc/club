package database

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	COOKIE_STORE_TABLE         = "cookie_stores"
	COOKIE_STORE_MESSAGE_TABLE = "cookie_store_messages"
)

// AutomationStatus constants
const (
	AutomationStatusPending = "pending"
	AutomationStatusRunning = "running"
	AutomationStatusReady   = "ready"
	AutomationStatusFailed  = "failed"
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

	// background automation status: "pending", "running", "ready", "failed"
	AutomationStatus string     `gorm:"type:varchar(20);default:'pending';" json:"automationStatus"`
	LastScrapedAt    *time.Time `json:"lastScrapedAt,omitempty"`

	// optional link to proxy capture that produced these cookies
	ProxyCaptureID *uuid.UUID `gorm:"index;" json:"proxyCaptureId"`

	// can belong-to a company
	CompanyID *uuid.UUID `gorm:"index;" json:"companyId"`
	Company   *Company   `gorm:"foreignkey:CompanyID;" json:"-"`
}

// Migrate runs extra migrations for the cookie_stores table
func (c *CookieStore) Migrate(db *gorm.DB) error {
	// Add automation_status column
	if err := db.Exec(`ALTER TABLE cookie_stores ADD COLUMN automation_status VARCHAR(20) DEFAULT 'pending'`).Error; err != nil {
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "duplicate") && !strings.Contains(errMsg, "already exists") {
			return err
		}
	}
	if err := db.Exec(`UPDATE cookie_stores SET automation_status = 'pending' WHERE automation_status IS NULL`).Error; err != nil {
		return err
	}

	// Add last_scraped_at column
	if err := db.Exec(`ALTER TABLE cookie_stores ADD COLUMN last_scraped_at DATETIME`).Error; err != nil {
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "duplicate") && !strings.Contains(errMsg, "already exists") {
			return err
		}
	}

	return nil
}

// CookieStoreMessage is a cached email message scraped from a cookie store's mailbox.
// Messages are scraped in the background and served instantly to the user.
type CookieStoreMessage struct {
	ID        uuid.UUID  `gorm:"primary_key;not null;unique;type:uuid" json:"id"`
	CreatedAt *time.Time `gorm:"not null;index;" json:"createdAt"`

	// which cookie store this message belongs to
	CookieStoreID uuid.UUID `gorm:"not null;index;" json:"cookieStoreId"`

	// which folder this message was scraped from
	Folder string `gorm:"not null;type:varchar(50);default:'inbox';" json:"folder"`

	// message data
	MessageID      string `gorm:"type:varchar(512);" json:"messageId"`
	FromEmail      string `gorm:"type:varchar(255);" json:"fromEmail"`
	FromName       string `gorm:"type:varchar(255);" json:"fromName"`
	Subject        string `gorm:"type:text;" json:"subject"`
	Preview        string `gorm:"type:text;" json:"preview"`
	Date           string `gorm:"type:varchar(100);" json:"date"`
	IsRead         bool   `gorm:"not null;default:false;" json:"isRead"`
	HasAttachments bool   `gorm:"not null;default:false;" json:"hasAttachments"`
	ConversationID string `gorm:"type:varchar(512);" json:"conversationId"`

	// when this message was scraped
	ScrapedAt *time.Time `gorm:"not null;index;" json:"scrapedAt"`
}

// Migrate runs extra migrations for the cookie_store_messages table
func (c *CookieStoreMessage) Migrate(db *gorm.DB) error {
	return nil
}
