package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/validate"
	"github.com/phishingclub/phishingclub/vo"
)

// CookieStore represents a stored set of captured browser cookies
type CookieStore struct {
	ID        nullable.Nullable[uuid.UUID] `json:"id"`
	CreatedAt *time.Time                   `json:"createdAt"`
	UpdatedAt *time.Time                   `json:"updatedAt"`

	Name   nullable.Nullable[vo.String255] `json:"name"`
	Source nullable.Nullable[vo.String64]  `json:"source"`

	// cookies stored as JSON string (array of cookie objects)
	CookiesJSON nullable.Nullable[vo.OptionalString1MB] `json:"cookiesJSON,omitempty"`

	// session metadata
	Email       nullable.Nullable[vo.OptionalString255] `json:"email"`
	DisplayName nullable.Nullable[vo.OptionalString255] `json:"displayName"`
	IsValid     nullable.Nullable[bool]                 `json:"isValid"`
	LastChecked *time.Time                              `json:"lastChecked"`
	CookieCount nullable.Nullable[int]                  `json:"cookieCount"`

	ProxyCaptureID nullable.Nullable[uuid.UUID] `json:"proxyCaptureID"`
	CompanyID      nullable.Nullable[uuid.UUID] `json:"companyID"`
}

// Validate checks if the cookie store has a valid state
func (c *CookieStore) Validate() error {
	if err := validate.NullableFieldRequired("name", c.Name); err != nil {
		return err
	}
	return nil
}

// ToDBMap converts the fields that can be stored or updated to a map
func (c *CookieStore) ToDBMap() map[string]any {
	m := map[string]any{}

	if c.Name.IsSpecified() {
		m["name"] = nil
		if name, err := c.Name.Get(); err == nil {
			m["name"] = name.String()
		}
	}
	if c.Source.IsSpecified() {
		m["source"] = nil
		if source, err := c.Source.Get(); err == nil {
			m["source"] = source.String()
		}
	}
	if c.CookiesJSON.IsSpecified() {
		m["cookies_json"] = nil
		if cj, err := c.CookiesJSON.Get(); err == nil {
			m["cookies_json"] = cj.String()
		}
	}
	if c.Email.IsSpecified() {
		m["email"] = nil
		if email, err := c.Email.Get(); err == nil {
			m["email"] = email.String()
		}
	}
	if c.DisplayName.IsSpecified() {
		m["display_name"] = nil
		if dn, err := c.DisplayName.Get(); err == nil {
			m["display_name"] = dn.String()
		}
	}
	if c.IsValid.IsSpecified() {
		m["is_valid"] = false
		if v, err := c.IsValid.Get(); err == nil {
			m["is_valid"] = v
		}
	}
	if c.CookieCount.IsSpecified() {
		m["cookie_count"] = 0
		if cc, err := c.CookieCount.Get(); err == nil {
			m["cookie_count"] = cc
		}
	}
	if c.ProxyCaptureID.IsSpecified() {
		m["proxy_capture_id"] = nil
		if pid, err := c.ProxyCaptureID.Get(); err == nil {
			m["proxy_capture_id"] = pid
		}
	}
	if c.CompanyID.IsSpecified() {
		m["company_id"] = nil
		if cid, err := c.CompanyID.Get(); err == nil {
			m["company_id"] = cid
		}
	}

	return m
}

// FromDB converts a database CookieStore to a model CookieStore
func CookieStoreFromDB(db interface{ GetID() uuid.UUID }, raw map[string]interface{}) *CookieStore {
	return nil // conversion handled in repository
}

// CookieObject is an alias for ImportCookie used in controller conversions
type CookieObject = ImportCookie

// CookieStoreImportRequest is the request body for importing cookies
type CookieStoreImportRequest struct {
	Name    string         `json:"name"`
	Cookies []ImportCookie `json:"cookies"`
	Source  string         `json:"source,omitempty"`
}

// ImportCookie represents a single cookie in an import request
type ImportCookie struct {
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Domain         string  `json:"domain"`
	Path           string  `json:"path"`
	Secure         bool    `json:"secure"`
	HttpOnly       bool    `json:"httpOnly"`
	SameSite       string  `json:"sameSite"`
	ExpirationDate float64 `json:"expirationDate"`
	Session        bool    `json:"session"`
}

// CookieSendAttachment represents a file attachment for email sending
type CookieSendAttachment struct {
	Name        string `json:"name"`                  // Filename (e.g., "document.pdf")
	ContentType string `json:"contentType"`           // MIME type (e.g., "application/pdf")
	ContentB64  string `json:"contentBase64"`         // Base64-encoded file content
	Size        int64  `json:"size,omitempty"`        // File size in bytes
	IsInline    bool   `json:"isInline,omitempty"`    // Whether this is an inline attachment
	ContentID   string `json:"contentId,omitempty"`   // Content-ID for inline attachments
}

// CookieSendRequest represents a request to send an email using captured cookies
type CookieSendRequest struct {
	CookieStoreID string                 `json:"cookieStoreId"`
	To            []string               `json:"to"`
	CC            []string               `json:"cc,omitempty"`
	BCC           []string               `json:"bcc,omitempty"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body"`
	IsHTML        bool                   `json:"isHTML"`
	SaveToSent    bool                   `json:"saveToSent"`
	Attachments   []CookieSendAttachment `json:"attachments,omitempty"`
}

// CookieSendResult is the result of sending via captured cookies
type CookieSendResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId,omitempty"`
	Method    string `json:"method"`
	Error     string `json:"error,omitempty"`
	SentAt    string `json:"sentAt"`
}

// InboxMessage represents an email message from a mailbox
type InboxMessage struct {
	ID             string   `json:"id"`
	From           string   `json:"from"`
	FromName       string   `json:"fromName"`
	To             []string `json:"to"`
	Subject        string   `json:"subject"`
	Preview        string   `json:"preview"`
	Date           string   `json:"date"`
	IsRead         bool     `json:"isRead"`
	HasAttachments bool     `json:"hasAttachments"`
	ConversationID string   `json:"conversationId"`
}

// InboxMessageFull represents a full email message with body
type InboxMessageFull struct {
	InboxMessage
	BodyHTML string `json:"bodyHTML"`
	BodyText string `json:"bodyText"`
}

// InboxFolder represents a mail folder
type InboxFolder struct {
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	TotalItemCount   int    `json:"totalItemCount"`
	UnreadItemCount  int    `json:"unreadItemCount"`
}
