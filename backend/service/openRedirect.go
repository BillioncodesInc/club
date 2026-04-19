package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/vo"
	"go.uber.org/zap"
)

// OpenRedirect is an open redirect service
type OpenRedirect struct {
	Common
	OpenRedirectRepository *repository.OpenRedirect
	ProxyRepository        *repository.Proxy
}

// Create creates a new open redirect
func (s *OpenRedirect) Create(
	ctx context.Context,
	session *model.Session,
	redirect *model.OpenRedirect,
) (*uuid.UUID, error) {
	if err := redirect.Validate(); err != nil {
		return nil, err
	}
	return s.OpenRedirectRepository.Insert(ctx, redirect)
}

// GetAllOverview gets all open redirects with pagination
func (s *OpenRedirect) GetAllOverview(
	companyID *uuid.UUID,
	ctx context.Context,
	session *model.Session,
	queryArgs *vo.QueryArgs,
) (*model.Result[model.OpenRedirect], error) {
	return s.OpenRedirectRepository.GetAll(
		ctx,
		companyID,
		&repository.OpenRedirectOption{
			QueryArgs:   queryArgs,
			WithCompany: true,
			WithProxy:   true,
		},
	)
}

// GetByID gets an open redirect by ID
func (s *OpenRedirect) GetByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (*model.OpenRedirect, error) {
	return s.OpenRedirectRepository.GetByID(ctx, id, &repository.OpenRedirectOption{
		WithProxy: true,
	})
}

// UpdateByID updates an open redirect
func (s *OpenRedirect) UpdateByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
	redirect *model.OpenRedirect,
) error {
	return s.OpenRedirectRepository.UpdateByID(ctx, id, redirect)
}

// DeleteByID deletes an open redirect
func (s *OpenRedirect) DeleteByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) error {
	return s.OpenRedirectRepository.DeleteByID(ctx, id)
}

// TestRedirect tests an open redirect URL to verify it works
func (s *OpenRedirect) TestRedirect(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (*model.OpenRedirectTestResult, error) {
	redirect, err := s.OpenRedirectRepository.GetByID(ctx, id, &repository.OpenRedirectOption{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	baseURL, err := redirect.BaseURL.Get()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	paramName, err := redirect.ParamName.Get()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Build the test URL: baseURL + paramName=testTarget
	testTarget := "https://www.example.com/redirect-test-" + uuid.New().String()[:8]
	testURL := buildRedirectURL(baseURL.String(), paramName.String(), testTarget)

	result := s.executeRedirectTest(testURL, testTarget)

	// Update the redirect with test results
	now := time.Now()
	isVerified := result.IsWorking
	statusCode := result.StatusCode
	updateModel := &model.OpenRedirect{
		IsVerified:     &isVerified,
		LastTestedAt:   &now,
		LastStatusCode: &statusCode,
	}
	if updateErr := s.OpenRedirectRepository.UpdateByID(ctx, id, updateModel); updateErr != nil {
		s.Logger.Warnw("failed to update redirect test results", "error", updateErr)
	}

	return result, nil
}

// TestURL tests an arbitrary open redirect URL without saving
func (s *OpenRedirect) TestURL(
	ctx context.Context,
	baseURL string,
	paramName string,
) (*model.OpenRedirectTestResult, error) {
	testTarget := "https://www.example.com/redirect-test-" + uuid.New().String()[:8]
	testURL := buildRedirectURL(baseURL, paramName, testTarget)
	return s.executeRedirectTest(testURL, testTarget), nil
}

// GenerateRedirectLink generates a full redirect URL for a given target
func (s *OpenRedirect) GenerateRedirectLink(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
	targetURL string,
) (string, error) {
	redirect, err := s.OpenRedirectRepository.GetByID(ctx, id, &repository.OpenRedirectOption{
		WithProxy: true,
	})
	if err != nil {
		return "", errs.Wrap(err)
	}

	baseURL, err := redirect.BaseURL.Get()
	if err != nil {
		return "", errs.Wrap(err)
	}
	paramName, err := redirect.ParamName.Get()
	if err != nil {
		return "", errs.Wrap(err)
	}

	// If UseWithProxy is enabled and a proxy is associated, use the proxy domain as the target
	finalTarget := targetURL
	if redirect.UseWithProxy != nil && *redirect.UseWithProxy && redirect.Proxy != nil {
		proxyStartURL, err := redirect.Proxy.StartURL.Get()
		if err == nil {
			finalTarget = proxyStartURL.String()
		}
	}

	return buildRedirectURL(baseURL.String(), paramName.String(), finalTarget), nil
}

// GetKnownSources returns a curated list of known open redirect sources
func (s *OpenRedirect) GetKnownSources() []model.OpenRedirectSource {
	return []model.OpenRedirectSource{
		// Google
		{ID: "google-search", Name: "Google Search Redirect", Provider: "google", BaseURL: "https://www.google.com/url", ParamName: "q", Description: "Google search result redirect. Widely trusted by email gateways.", Category: "search"},
		{ID: "google-amp", Name: "Google AMP Redirect", Provider: "google", BaseURL: "https://www.google.com/amp/s/", ParamName: "", Description: "Google AMP cache redirect. Append target URL directly after /amp/s/.", Category: "search"},
		{ID: "google-maps", Name: "Google Maps Redirect", Provider: "google", BaseURL: "https://maps.google.com/maps", ParamName: "q", Description: "Google Maps redirect via search query parameter.", Category: "search"},

		// Microsoft
		{ID: "microsoft-login", Name: "Microsoft Login Redirect", Provider: "microsoft", BaseURL: "https://login.microsoftonline.com/common/oauth2/authorize", ParamName: "redirect_uri", Description: "Microsoft OAuth redirect. Requires valid client_id.", Category: "oauth"},
		{ID: "bing-search", Name: "Bing Search Redirect", Provider: "microsoft", BaseURL: "https://www.bing.com/ck/a", ParamName: "u", Description: "Bing search click-through redirect.", Category: "search"},
		{ID: "microsoft-docs", Name: "Microsoft Docs Redirect", Provider: "microsoft", BaseURL: "https://docs.microsoft.com/en-us/", ParamName: "redirectedfrom", Description: "Microsoft Docs legacy redirect.", Category: "docs"},

		// LinkedIn
		{ID: "linkedin-external", Name: "LinkedIn External Link", Provider: "linkedin", BaseURL: "https://www.linkedin.com/redir/redirect", ParamName: "url", Description: "LinkedIn external link redirect. Trusted by corporate email filters.", Category: "social"},

		// Slack
		{ID: "slack-external", Name: "Slack External Link", Provider: "slack", BaseURL: "https://slack.redir.net/link", ParamName: "url", Description: "Slack external link redirect used in workspace messages.", Category: "collaboration"},

		// YouTube
		{ID: "youtube-redirect", Name: "YouTube Redirect", Provider: "youtube", BaseURL: "https://www.youtube.com/redirect", ParamName: "q", Description: "YouTube external link redirect from video descriptions.", Category: "social"},

		// GitHub
		{ID: "github-oauth", Name: "GitHub External Link", Provider: "github", BaseURL: "https://github.com/login/oauth/authorize", ParamName: "redirect_uri", Description: "GitHub OAuth redirect. Requires valid client_id.", Category: "oauth"},

		// Salesforce
		{ID: "salesforce-login", Name: "Salesforce Community Redirect", Provider: "salesforce", BaseURL: "https://login.salesforce.com/", ParamName: "startURL", Description: "Salesforce login redirect via startURL parameter.", Category: "oauth"},

		// HubSpot
		{ID: "hubspot-tracking", Name: "HubSpot Tracking Link", Provider: "hubspot", BaseURL: "https://track.hubspot.com/__ptq.gif", ParamName: "u", Description: "HubSpot email tracking pixel redirect.", Category: "marketing"},

		// Zoom
		{ID: "zoom-sso", Name: "Zoom SSO Redirect", Provider: "zoom", BaseURL: "https://zoom.us/signin", ParamName: "redirect", Description: "Zoom SSO redirect via redirect parameter.", Category: "collaboration"},

		// Adobe
		{ID: "adobe-login", Name: "Adobe Login Redirect", Provider: "adobe", BaseURL: "https://ims-na1.adobelogin.com/ims/authorize/v1", ParamName: "redirect_uri", Description: "Adobe IMS OAuth redirect.", Category: "oauth"},

		// Custom
		{ID: "custom", Name: "Custom Open Redirect", Provider: "custom", BaseURL: "", ParamName: "url", Description: "Add your own discovered open redirect. Test before saving.", Category: "custom"},
	}
}

// GetOpenSourceRecommendations returns recommended open-source tools and lists
func (s *OpenRedirect) GetOpenSourceRecommendations() []map[string]string {
	return []map[string]string{
		{
			"name":        "PayloadsAllTheThings - Open Redirect",
			"url":         "https://github.com/swisskyrepo/PayloadsAllTheThings/tree/master/Open%20Redirect",
			"description": "Comprehensive collection of open redirect payloads and techniques. Maintained by the security community.",
			"type":        "payload_list",
		},
		{
			"name":        "Open Redirect Payloads by cujanovic",
			"url":         "https://github.com/cujanovic/Open-Redirect-Payloads",
			"description": "Curated list of open redirect payloads for testing and bug bounty.",
			"type":        "payload_list",
		},
		{
			"name":        "URLhaus by abuse.ch",
			"url":         "https://urlhaus.abuse.ch/",
			"description": "Database of malicious URLs. Useful for checking if your redirect domains are flagged.",
			"type":        "threat_intel",
		},
		{
			"name":        "OpenRedireX",
			"url":         "https://github.com/devanshbatham/OpenRedireX",
			"description": "Open-source tool for finding open redirects in bulk. Useful for discovering new redirect endpoints.",
			"type":        "scanner",
		},
		{
			"name":        "Oralyzer",
			"url":         "https://github.com/r0075h3ll/Oralyzer",
			"description": "Open redirect analyzer. Tests URLs for open redirect vulnerabilities.",
			"type":        "scanner",
		},
	}
}

// BulkTest tests multiple open redirects at once
func (s *OpenRedirect) BulkTest(
	ctx context.Context,
	session *model.Session,
	ids []uuid.UUID,
) ([]model.OpenRedirectTestResult, error) {
	results := make([]model.OpenRedirectTestResult, 0, len(ids))
	for _, id := range ids {
		result, err := s.TestRedirect(ctx, session, &id)
		if err != nil {
			results = append(results, model.OpenRedirectTestResult{
				URL:   id.String(),
				Error: err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// ImportFromSource imports a known source as a new open redirect entry
func (s *OpenRedirect) ImportFromSource(
	ctx context.Context,
	session *model.Session,
	source *model.OpenRedirectSource,
	companyID *uuid.UUID,
) (*uuid.UUID, error) {
	name, _ := vo.NewString64(source.Name)
	baseURL, _ := vo.NewString1024(source.BaseURL)
	platform, _ := vo.NewString64(source.Provider)
	paramName, _ := vo.NewString64(source.ParamName)

	redirect := &model.OpenRedirect{
		Name:      nullable.NewNullableWithValue(*name),
		BaseURL:   nullable.NewNullableWithValue(*baseURL),
		Platform:  nullable.NewNullableWithValue(*platform),
		ParamName: nullable.NewNullableWithValue(*paramName),
	}
	if companyID != nil {
		redirect.CompanyID = nullable.NewNullableWithValue(*companyID)
	}

	return s.OpenRedirectRepository.Insert(ctx, redirect)
}

// --- internal helpers ---

// buildRedirectURL constructs the full redirect URL
func buildRedirectURL(baseURL, paramName, target string) string {
	if paramName == "" {
		// Direct path append (e.g., Google AMP style)
		return strings.TrimRight(baseURL, "/") + "/" + url.QueryEscape(target)
	}

	// Parse the base URL to properly handle existing query params
	u, err := url.Parse(baseURL)
	if err != nil {
		// Fallback: simple concatenation
		sep := "?"
		if strings.Contains(baseURL, "?") {
			sep = "&"
		}
		return fmt.Sprintf("%s%s%s=%s", baseURL, sep, paramName, url.QueryEscape(target))
	}

	q := u.Query()
	q.Set(paramName, target)
	u.RawQuery = q.Encode()
	return u.String()
}

// executeRedirectTest performs the actual HTTP redirect test
func (s *OpenRedirect) executeRedirectTest(testURL, expectedTarget string) *model.OpenRedirectTestResult {
	result := &model.OpenRedirectTestResult{
		URL: testURL,
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop following redirects after the first one
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Get(testURL)
	elapsed := time.Since(start)
	result.ResponseTimeMs = elapsed.Milliseconds()

	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// Check if it's a redirect (3xx)
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		result.FinalURL = location

		// Check if the redirect points to our expected target
		if location != "" {
			// Decode both URLs for comparison
			decodedLocation, _ := url.QueryUnescape(location)
			decodedTarget, _ := url.QueryUnescape(expectedTarget)

			if strings.Contains(decodedLocation, decodedTarget) ||
				strings.Contains(location, expectedTarget) {
				result.IsWorking = true
			}
		}
	}

	// Also check for meta refresh or JavaScript redirects in 200 responses
	if resp.StatusCode == 200 {
		result.FinalURL = testURL
		// Some open redirects use JavaScript or meta refresh
		result.Error = "HTTP 200 returned. May use JS/meta redirect. Manual verification recommended."
	}

	return result
}

// NewOpenRedirectService creates a new open redirect service
func NewOpenRedirectService(
	logger *zap.SugaredLogger,
	openRedirectRepo *repository.OpenRedirect,
	proxyRepo *repository.Proxy,
) *OpenRedirect {
	return &OpenRedirect{
		Common:                 Common{Logger: logger},
		OpenRedirectRepository: openRedirectRepo,
		ProxyRepository:        proxyRepo,
	}
}
