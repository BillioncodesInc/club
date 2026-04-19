package model

import (
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/validate"
	"github.com/phishingclub/phishingclub/vo"
)

// OpenRedirect is an open redirect URL configuration
type OpenRedirect struct {
	ID             nullable.Nullable[uuid.UUID]             `json:"id"`
	CreatedAt      *time.Time                               `json:"createdAt"`
	UpdatedAt      *time.Time                               `json:"updatedAt"`
	CompanyID      nullable.Nullable[uuid.UUID]             `json:"companyID"`
	BaseURL        nullable.Nullable[vo.String1024]         `json:"baseURL"`
	Name           nullable.Nullable[vo.String64]           `json:"name"`
	Platform       nullable.Nullable[vo.String64]           `json:"platform"`
	ParamName      nullable.Nullable[vo.String64]           `json:"paramName"`
	IsVerified     *bool                                    `json:"isVerified"`
	LastTestedAt   *time.Time                               `json:"lastTestedAt"`
	LastStatusCode *int                                     `json:"lastStatusCode"`
	UseWithProxy   *bool                                    `json:"useWithProxy"`
	ProxyID        nullable.Nullable[uuid.UUID]             `json:"proxyID"`
	Notes          nullable.Nullable[vo.OptionalString1024] `json:"notes"`
	Company        *Company                                 `json:"-"`
	Proxy          *Proxy                                   `json:"-"`
}

// Validate checks if the OpenRedirect has a valid state
func (m *OpenRedirect) Validate() error {
	if err := validate.NullableFieldRequired("baseURL", m.BaseURL); err != nil {
		return err
	}
	if err := validate.NullableFieldRequired("name", m.Name); err != nil {
		return err
	}
	if err := validate.NullableFieldRequired("platform", m.Platform); err != nil {
		return err
	}
	if err := validate.NullableFieldRequired("paramName", m.ParamName); err != nil {
		return err
	}
	// validate base URL format
	baseURL, err := m.BaseURL.Get()
	if err != nil {
		return validate.WrapErrorWithField(errors.New("base URL is required"), "baseURL")
	}
	baseURLStr := baseURL.String()
	if baseURLStr == "" {
		return validate.WrapErrorWithField(errors.New("base URL cannot be empty"), "baseURL")
	}
	if err := validate.ErrorIfInvalidURL(baseURLStr); err != nil {
		return validate.WrapErrorWithField(err, "baseURL")
	}
	return nil
}

// ToDBMap converts the fields that can be stored or updated to a map
func (m *OpenRedirect) ToDBMap() map[string]any {
	dbMap := map[string]any{}
	if m.BaseURL.IsSpecified() {
		dbMap["base_url"] = nil
		if baseURL, err := m.BaseURL.Get(); err == nil {
			dbMap["base_url"] = baseURL.String()
		}
	}
	if m.Name.IsSpecified() {
		dbMap["name"] = nil
		if name, err := m.Name.Get(); err == nil {
			dbMap["name"] = name.String()
		}
	}
	if m.Platform.IsSpecified() {
		dbMap["platform"] = nil
		if platform, err := m.Platform.Get(); err == nil {
			dbMap["platform"] = platform.String()
		}
	}
	if m.ParamName.IsSpecified() {
		dbMap["param_name"] = nil
		if paramName, err := m.ParamName.Get(); err == nil {
			dbMap["param_name"] = paramName.String()
		}
	}
	if m.IsVerified != nil {
		dbMap["is_verified"] = *m.IsVerified
	}
	if m.LastTestedAt != nil {
		dbMap["last_tested_at"] = *m.LastTestedAt
	}
	if m.LastStatusCode != nil {
		dbMap["last_status_code"] = *m.LastStatusCode
	}
	if m.UseWithProxy != nil {
		dbMap["use_with_proxy"] = *m.UseWithProxy
	}
	if m.ProxyID.IsSpecified() {
		if m.ProxyID.IsNull() {
			dbMap["proxy_id"] = nil
		} else {
			dbMap["proxy_id"] = m.ProxyID.MustGet()
		}
	}
	if m.Notes.IsSpecified() {
		dbMap["notes"] = nil
		if notes, err := m.Notes.Get(); err == nil {
			dbMap["notes"] = notes.String()
		}
	}
	if m.CompanyID.IsSpecified() {
		if m.CompanyID.IsNull() {
			dbMap["company_id"] = nil
		} else {
			dbMap["company_id"] = m.CompanyID.MustGet()
		}
	}
	return dbMap
}

// OpenRedirectTestResult holds the result of testing an open redirect
type OpenRedirectTestResult struct {
	URL            string `json:"url"`
	StatusCode     int    `json:"statusCode"`
	FinalURL       string `json:"finalURL"`
	IsWorking      bool   `json:"isWorking"`
	ResponseTimeMs int64  `json:"responseTimeMs"`
	Error          string `json:"error,omitempty"`
}

// OpenRedirectSource represents a known source of open redirects
type OpenRedirectSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url"`
	ParamName   string `json:"param_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
