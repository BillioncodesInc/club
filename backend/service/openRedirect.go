package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/vo"
	"go.uber.org/zap"
)

// maxRedirectTestBodyBytes caps how much of a 200 response body we read when
// looking for meta refresh / JS redirect signals. 64 KiB is enough to cover
// even heavily-padded landing pages without slurping multi-MB payloads.
const maxRedirectTestBodyBytes = 64 * 1024

// maxRedirectTestHops is the maximum number of HTTP hops we follow before
// giving up. Well-behaved redirect chains are <= 3 hops; 10 is generous.
const maxRedirectTestHops = 10

// errTooManyRedirects sentinel returned by CheckRedirect when we hit the cap.
var errTooManyRedirects = errors.New("too many redirects")

// metaRefreshRe matches `<meta http-equiv="refresh" content="N;url=..."/>`
// with optional whitespace / quoting variations.
var metaRefreshRe = regexp.MustCompile(
	`(?is)<meta[^>]+http-equiv\s*=\s*["']?refresh["']?[^>]+content\s*=\s*["']?\s*\d+\s*[;,]\s*url\s*=\s*([^"'>\s]+)`,
)

// jsLocationRe matches common JS-driven redirects. We accept several forms:
//
//	window.location = "…"
//	window.location.href = "…"
//	window.location.replace("…")
//	location.replace("…")
//	document.location = "…"
var jsLocationRe = regexp.MustCompile(
	`(?is)(?:window\.|document\.)?location(?:\.href|\.replace)?\s*(?:=|\(\s*)\s*["']([^"']+)["']`,
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
	ae := NewAuditEvent("OpenRedirect.Create", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	if err := redirect.Validate(); err != nil {
		return nil, errs.Wrap(err)
	}
	id, err := s.OpenRedirectRepository.Insert(ctx, redirect)
	if err != nil {
		s.Logger.Errorw("failed to insert open redirect", "error", err)
		return nil, errs.Wrap(err)
	}
	ae.Details["id"] = id.String()
	s.AuditLogAuthorized(ae)
	return id, nil
}

// GetAllOverview gets all open redirects with pagination
func (s *OpenRedirect) GetAllOverview(
	companyID *uuid.UUID,
	ctx context.Context,
	session *model.Session,
	queryArgs *vo.QueryArgs,
) (*model.Result[model.OpenRedirect], error) {
	result := model.NewEmptyResult[model.OpenRedirect]()
	ae := NewAuditEvent("OpenRedirect.GetAllOverview", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return result, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return result, errs.ErrAuthorizationFailed
	}
	out, err := s.OpenRedirectRepository.GetAll(
		ctx,
		companyID,
		&repository.OpenRedirectOption{
			QueryArgs:   queryArgs,
			WithCompany: true,
			WithProxy:   true,
		},
	)
	if err != nil {
		s.Logger.Errorw("failed to get open redirects", "error", err)
		return result, errs.Wrap(err)
	}
	return out, nil
}

// GetByID gets an open redirect by ID
func (s *OpenRedirect) GetByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (*model.OpenRedirect, error) {
	ae := NewAuditEvent("OpenRedirect.GetByID", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	out, err := s.OpenRedirectRepository.GetByID(ctx, id, &repository.OpenRedirectOption{
		WithProxy: true,
	})
	if err != nil {
		s.Logger.Errorw("failed to get open redirect", "error", err)
		return out, errs.Wrap(err)
	}
	return out, nil
}

// UpdateByID updates an open redirect
func (s *OpenRedirect) UpdateByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
	redirect *model.OpenRedirect,
) error {
	ae := NewAuditEvent("OpenRedirect.UpdateByID", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return errs.ErrAuthorizationFailed
	}
	if err := s.OpenRedirectRepository.UpdateByID(ctx, id, redirect); err != nil {
		s.Logger.Errorw("failed to update open redirect", "error", err)
		return errs.Wrap(err)
	}
	s.AuditLogAuthorized(ae)
	return nil
}

// DeleteByID deletes an open redirect
func (s *OpenRedirect) DeleteByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) error {
	ae := NewAuditEvent("OpenRedirect.DeleteByID", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return errs.ErrAuthorizationFailed
	}
	if err := s.OpenRedirectRepository.DeleteByID(ctx, id); err != nil {
		s.Logger.Errorw("failed to delete open redirect", "error", err)
		return errs.Wrap(err)
	}
	s.AuditLogAuthorized(ae)
	return nil
}

// TestRedirect tests an open redirect URL to verify it works
func (s *OpenRedirect) TestRedirect(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (*model.OpenRedirectTestResult, error) {
	ae := NewAuditEvent("OpenRedirect.TestRedirect", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	redirect, err := s.OpenRedirectRepository.GetByID(ctx, id, &repository.OpenRedirectOption{})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	baseURL, err := redirect.BaseURL.Get()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	// ParamName may be empty/"path" for path-based redirects
	paramNameStr := ""
	paramName, err := redirect.ParamName.Get()
	if err == nil {
		paramNameStr = paramName.String()
		if paramNameStr == "path" {
			paramNameStr = ""
		}
	}

	// Build the test URL: baseURL + paramName=testTarget
	testTarget := "https://www.example.com/redirect-test-" + uuid.New().String()[:8]
	testURL := buildRedirectURL(baseURL.String(), paramNameStr, testTarget)

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

	s.AuditLogAuthorized(ae)
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
	ae := NewAuditEvent("OpenRedirect.GenerateRedirectLink", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return "", errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return "", errs.ErrAuthorizationFailed
	}
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
	// ParamName may be empty/"path" for path-based redirects
	paramNameStr := ""
	paramName, err := redirect.ParamName.Get()
	if err == nil {
		paramNameStr = paramName.String()
		if paramNameStr == "path" {
			paramNameStr = ""
		}
	}

	// If UseWithProxy is enabled and a proxy is associated, use the proxy domain as the target
	finalTarget := targetURL
	if redirect.UseWithProxy != nil && *redirect.UseWithProxy && redirect.Proxy != nil {
		proxyStartURL, err := redirect.Proxy.StartURL.Get()
		if err == nil {
			finalTarget = proxyStartURL.String()
		}
	}

	return buildRedirectURL(baseURL.String(), paramNameStr, finalTarget), nil
}

// GetKnownSources returns a curated list of known open redirect sources
// Sources verified from: Microsoft Security Blog, Cofense PDC, SANS ISC, LevelBlue/SpiderLabs,
// PayloadsAllTheThings, Hornetsecurity, and public bug bounty reports.
func (s *OpenRedirect) GetKnownSources() []model.OpenRedirectSource {
	return []model.OpenRedirectSource{
		// ── Google Services ──────────────────────────────────────────────
		{ID: "google-search", Name: "Google Search Redirect", Provider: "google", BaseURL: "https://www.google.com/url", ParamName: "q", Description: "Google search result redirect. Widely trusted by all email gateways and security filters. Append &sa=D&source=docs for higher trust.", Category: "search"},
		{ID: "google-amp", Name: "Google AMP Cache", Provider: "google", BaseURL: "https://www.google.com/amp/s/", ParamName: "", Description: "Google AMP cache redirect. Path-based: append target domain directly after /amp/s/. Bypasses most SEGs.", Category: "cloud"},
		{ID: "google-maps", Name: "Google Maps Redirect", Provider: "google", BaseURL: "https://maps.google.com/maps", ParamName: "q", Description: "Google Maps redirect via search query parameter. Trusted domain, rarely blocked.", Category: "cloud"},
		{ID: "google-travel", Name: "Google Travel Redirect", Provider: "google", BaseURL: "https://www.google.com/travel/flights", ParamName: "redirect_url", Description: "Google Travel open redirect. Actively exploited in phishing campaigns per SANS ISC (2025).", Category: "cloud"},
		{ID: "google-notifications", Name: "Google Notifications", Provider: "google", BaseURL: "https://notifications.google.com/g/p/", ParamName: "link", Description: "Google Notifications redirect. Abused since Q4 2023 for Meta/Instagram phishing per LevelBlue SpiderLabs.", Category: "cloud"},
		{ID: "google-weblight", Name: "Google Web Light", Provider: "google", BaseURL: "https://googleweblight.com/", ParamName: "lite_url", Description: "Google Web Light service for fast mobile browsing. Redirects to any URL via lite_url parameter.", Category: "cloud"},
		{ID: "google-doubleclick", Name: "Google DoubleClick", Provider: "google", BaseURL: "https://ad.doubleclick.net/ddm/trackclk/", ParamName: "dc_rdto", Description: "Google DoubleClick ad tracking redirect. Owned by Google, highly trusted by email filters.", Category: "marketing"},

		// ── Microsoft Services ────────────────────────────────────────────
		{ID: "microsoft-oauth", Name: "Microsoft Entra ID OAuth", Provider: "microsoft", BaseURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", ParamName: "redirect_uri", Description: "Microsoft Entra ID OAuth redirect. Use scope=invalid&prompt=none to force error redirect. Per Microsoft Security Blog (March 2026).", Category: "oauth"},
		{ID: "microsoft-live", Name: "Microsoft Live Login", Provider: "microsoft", BaseURL: "https://login.live.com/login.srf", ParamName: "wreply", Description: "Microsoft Live login redirect via wreply parameter. Classic open redirect on Microsoft consumer auth.", Category: "oauth"},
		{ID: "bing-click", Name: "Bing Click Tracking", Provider: "microsoft", BaseURL: "https://www.bing.com/ck/a", ParamName: "u", Description: "Bing search click-through redirect. Parameter u takes base64-encoded URL (a1{base64}). Per LevelBlue SpiderLabs.", Category: "search"},
		{ID: "microsoft-docs", Name: "Microsoft Docs Legacy", Provider: "microsoft", BaseURL: "https://docs.microsoft.com/en-us/", ParamName: "redirectedfrom", Description: "Microsoft Docs legacy redirect via redirectedfrom parameter.", Category: "cloud"},

		// ── LinkedIn ──────────────────────────────────────────────────────
		{ID: "linkedin-slink", Name: "LinkedIn Smart Link", Provider: "linkedin", BaseURL: "https://www.linkedin.com/slink", ParamName: "code", Description: "LinkedIn Smart Link (slink) redirect. Requires LinkedIn campaign code. Bypasses all major SEGs per Cofense PDC.", Category: "social"},
		{ID: "linkedin-redirect", Name: "LinkedIn External Redirect", Provider: "linkedin", BaseURL: "https://www.linkedin.com/redir/redirect", ParamName: "url", Description: "LinkedIn external link redirect. Trusted by corporate email filters and security gateways.", Category: "social"},

		// ── Meta / Facebook ───────────────────────────────────────────────
		{ID: "facebook-external", Name: "Facebook External Link", Provider: "facebook", BaseURL: "https://l.facebook.com/l.php", ParamName: "u", Description: "Facebook external link redirect. Requires valid h= hash parameter for some targets.", Category: "social"},
		{ID: "facebook-business", Name: "Facebook Business Redirect", Provider: "facebook", BaseURL: "https://business.facebook.com/", ParamName: "redirect_url", Description: "Facebook Business portfolio redirect. Open redirect on account setup flow per bug bounty reports.", Category: "social"},

		// ── YouTube ───────────────────────────────────────────────────────
		{ID: "youtube-redirect", Name: "YouTube Redirect", Provider: "youtube", BaseURL: "https://www.youtube.com/redirect", ParamName: "q", Description: "YouTube external link redirect from video descriptions. Add &event=video_description for legitimacy. Per Hornetsecurity.", Category: "social"},

		// ── Slack ─────────────────────────────────────────────────────────
		{ID: "slack-redirect", Name: "Slack External Redirect", Provider: "slack", BaseURL: "https://slack-redir.net/link", ParamName: "url", Description: "Slack external link redirect used in workspace messages. Trusted by enterprise email filters. Per Okta threat intel.", Category: "collaboration"},

		// ── Zoom ──────────────────────────────────────────────────────────
		{ID: "zoom-sso", Name: "Zoom SSO Redirect", Provider: "zoom", BaseURL: "https://zoom.us/signin", ParamName: "redirect", Description: "Zoom SSO redirect via redirect parameter. Zoom domains highly trusted in enterprise environments.", Category: "collaboration"},
		{ID: "zoom-docs", Name: "Zoom Docs Redirect", Provider: "zoom", BaseURL: "https://zoom.us/docs/", ParamName: "redirect_url", Description: "Zoom Docs redirect. Used in phishing campaigns delivering AITM payloads per Sublime Security.", Category: "collaboration"},

		// ── GitHub ────────────────────────────────────────────────────────
		{ID: "github-oauth", Name: "GitHub OAuth Redirect", Provider: "github", BaseURL: "https://github.com/login/oauth/authorize", ParamName: "redirect_uri", Description: "GitHub OAuth redirect. Requires registered OAuth app client_id.", Category: "oauth"},

		// ── Adobe ─────────────────────────────────────────────────────────
		{ID: "adobe-ims", Name: "Adobe IMS OAuth", Provider: "adobe", BaseURL: "https://ims-na1.adobelogin.com/ims/authorize/v1", ParamName: "redirect_uri", Description: "Adobe IMS OAuth redirect. Adobe domains pass SPF/DKIM/DMARC checks per IronScales.", Category: "oauth"},
		{ID: "adobe-campaign", Name: "Adobe Campaign Redirect", Provider: "adobe", BaseURL: "https://campaign.adobe.com/r/", ParamName: "url", Description: "Adobe Campaign email tracking redirect. Used in image-based phishing per LevelBlue SpiderLabs.", Category: "marketing"},

		// ── Salesforce ────────────────────────────────────────────────────
		{ID: "salesforce-login", Name: "Salesforce Login Redirect", Provider: "salesforce", BaseURL: "https://login.salesforce.com/", ParamName: "startURL", Description: "Salesforce login redirect via startURL parameter. Trusted enterprise domain.", Category: "oauth"},
		{ID: "salesforce-krux", Name: "Salesforce Krux Beacon", Provider: "salesforce", BaseURL: "https://beacon.krxd.net/", ParamName: "url", Description: "Krux (Salesforce DMP) beacon redirect. Used in phishing redirections per LevelBlue.", Category: "marketing"},

		// ── HubSpot ───────────────────────────────────────────────────────
		{ID: "hubspot-tracking", Name: "HubSpot Tracking Redirect", Provider: "hubspot", BaseURL: "https://track.hubspot.com/__ptq.gif", ParamName: "redirect_url", Description: "HubSpot email tracking pixel redirect. Widely used in marketing, trusted by email filters.", Category: "marketing"},

		// ── Marketing Platforms ───────────────────────────────────────────
		{ID: "mailjet-redirect", Name: "Mailjet Tracking Redirect", Provider: "mailjet", BaseURL: "https://mjt.lu/lnk/", ParamName: "", Description: "Mailjet email tracking redirect. Path-based redirect via tracking ID. Per LevelBlue SpiderLabs.", Category: "marketing"},
		{ID: "constant-contact", Name: "Constant Contact Redirect", Provider: "constant-contact", BaseURL: "https://r20.rs6.net/tn.jsp", ParamName: "p", Description: "Constant Contact email marketing redirect. Used in multi-hop phishing chains per LevelBlue.", Category: "marketing"},

		// ── Social Platforms ──────────────────────────────────────────────
		{ID: "vk-away", Name: "VK External Redirect", Provider: "vk", BaseURL: "https://vk.com/away.php", ParamName: "to", Description: "VKontakte external link redirect. Russian social platform, used in targeted phishing per LevelBlue.", Category: "social"},
		{ID: "medium-redirect", Name: "Medium Global Identity", Provider: "medium", BaseURL: "https://medium.com/m/global-identity", ParamName: "redirectUrl", Description: "Medium global identity redirect. Publishing platform trusted by content filters.", Category: "social"},

		// ── Search Engines ────────────────────────────────────────────────
		{ID: "baidu-link", Name: "Baidu Search Redirect", Provider: "baidu", BaseURL: "https://www.baidu.com/link", ParamName: "url", Description: "Baidu search click-through redirect. Encoded URL parameter. Per LevelBlue SpiderLabs.", Category: "search"},

		// ── Enterprise / Collaboration ────────────────────────────────────
		{ID: "atlassian-login", Name: "Atlassian Login Redirect", Provider: "atlassian", BaseURL: "https://id.atlassian.com/login", ParamName: "continue", Description: "Atlassian (Jira/Confluence) login redirect via continue parameter. Enterprise trusted domain.", Category: "collaboration"},

		// ── Search (additions) ────────────────────────────────────────────
		{ID: "duckduckgo-l", Name: "DuckDuckGo Outbound Link", Provider: "duckduckgo", BaseURL: "https://duckduckgo.com/l/", ParamName: "uddg", Description: "DuckDuckGo outbound click redirect. The uddg parameter takes the URL-encoded target. Documented in Open Redirect payload collections.", Category: "search"},
		{ID: "yahoo-search", Name: "Yahoo Search Click Redirect", Provider: "yahoo", BaseURL: "https://r.search.yahoo.com/", ParamName: "RU", Description: "Yahoo search result click redirect (r.search.yahoo.com). Takes RU parameter with URL-encoded target. Trusted by many SEGs.", Category: "search"},
		{ID: "yandex-clck", Name: "Yandex Click Redirect", Provider: "yandex", BaseURL: "https://yandex.ru/clck/jsredir", ParamName: "url", Description: "Yandex search click-through redirect. Russian search engine, commonly trusted for legitimate traffic. Manual verification recommended.", Category: "search"},

		// ── Social (additions) ────────────────────────────────────────────
		{ID: "reddit-outbound", Name: "Reddit Outbound Click", Provider: "reddit", BaseURL: "https://out.reddit.com/", ParamName: "url", Description: "Reddit outbound click tracking redirect. Used when users click links in posts/comments. Documented in bug bounty reports.", Category: "social"},
		{ID: "instagram-l", Name: "Instagram External Link", Provider: "instagram", BaseURL: "https://l.instagram.com/", ParamName: "u", Description: "Instagram external link redirect (l.instagram.com). Mirror of Facebook's l.php. May require a valid e= hash for some targets.", Category: "social"},
		{ID: "pinterest-offsite", Name: "Pinterest Offsite Redirect", Provider: "pinterest", BaseURL: "https://www.pinterest.com/offsite/", ParamName: "url", Description: "Pinterest offsite link redirect. Used for pin outbound clicks. Generally trusted by corporate filters.", Category: "social"},
		{ID: "medium-r", Name: "Medium Outbound Redirect", Provider: "medium", BaseURL: "https://medium.com/r/", ParamName: "url", Description: "Medium outbound link redirect (separate from global-identity). Takes url parameter. Publishing platform trusted by most content filters.", Category: "social"},
		{ID: "quora-leavingsite", Name: "Quora Leaving Site Redirect", Provider: "quora", BaseURL: "https://www.quora.com/leavingsite", ParamName: "url", Description: "Quora outbound-link interstitial redirect. Documented in multiple bug bounty disclosures.", Category: "social"},

		// ── OAuth / Login (additions) ─────────────────────────────────────
		{ID: "github-return", Name: "GitHub Login Return Redirect", Provider: "github", BaseURL: "https://github.com/login", ParamName: "return_to", Description: "GitHub login page return_to redirect. Validates origin but accepts same-site targets. Manual verification recommended per target.", Category: "oauth"},
		{ID: "gitlab-signin", Name: "GitLab Sign-In Redirect", Provider: "gitlab", BaseURL: "https://gitlab.com/users/sign_in", ParamName: "redirect_to", Description: "GitLab sign-in page redirect_to parameter. Validates host but has historically accepted certain bypass patterns.", Category: "oauth"},
		{ID: "notion-signup", Name: "Notion Signup Redirect", Provider: "notion", BaseURL: "https://www.notion.so/signup", ParamName: "redirect", Description: "Notion signup page redirect parameter. Trusted SaaS domain widely allowlisted in enterprises.", Category: "oauth"},
		{ID: "salesforce-frontdoor", Name: "Salesforce Frontdoor Redirect", Provider: "salesforce", BaseURL: "https://login.salesforce.com/secur/frontdoor.jsp", ParamName: "retURL", Description: "Salesforce frontdoor.jsp retURL redirect. Classic enterprise redirect pattern; frequently referenced in SaaS redirect chain reports.", Category: "oauth"},
		{ID: "stackoverflow-logout", Name: "StackOverflow Logout Redirect", Provider: "stackoverflow", BaseURL: "https://stackoverflow.com/users/logout", ParamName: "returnUrl", Description: "StackOverflow logout returnUrl redirect. Documented as open redirect in prior security advisories.", Category: "oauth"},
		{ID: "azure-devops-signin", Name: "Azure DevOps Sign-In Redirect", Provider: "microsoft", BaseURL: "https://dev.azure.com/_signedIn", ParamName: "redirect_uri", Description: "Azure DevOps signed-in redirect_uri parameter. Microsoft-trusted domain, passes most SEGs.", Category: "oauth"},

		// ── Cloud / File Sharing (additions) ──────────────────────────────
		{ID: "dropbox-l-ce", Name: "Dropbox External Redirect", Provider: "dropbox", BaseURL: "https://www.dropbox.com/l/ce/", ParamName: "url", Description: "Dropbox outbound-link redirect (l/ce path). Documented in phishing chain research. Trusted cloud storage domain.", Category: "cloud"},

		// ── Collaboration (additions) ─────────────────────────────────────
		{ID: "slack-redir-param", Name: "Slack Workspace Redirect", Provider: "slack", BaseURL: "https://slack.com/", ParamName: "redir", Description: "Slack workspace redirect parameter. Trusted enterprise collaboration domain. Manual verification per target recommended.", Category: "collaboration"},
		{ID: "zoom-logout", Name: "Zoom Logout Return Redirect", Provider: "zoom", BaseURL: "https://zoom.us/saml/logout", ParamName: "returnto", Description: "Zoom SAML logout returnto parameter. Enterprise-trusted, used in multi-stage phishing per public threat reports.", Category: "collaboration"},

		// ── Shorteners (additions) ────────────────────────────────────────
		{ID: "twitter-tco", Name: "Twitter/X t.co Shortener", Provider: "twitter", BaseURL: "https://t.co/", ParamName: "", Description: "Twitter/X t.co link shortener. Path-based (t.co/SHORTID). Generated via Twitter API; cannot be crafted directly but is a trusted hop.", Category: "shortener"},
		{ID: "bitly-shortener", Name: "Bit.ly Shortener", Provider: "bitly", BaseURL: "https://bit.ly/", ParamName: "", Description: "Bit.ly URL shortener. Path-based hop (bit.ly/SHORTID). Requires account to generate links. Widely allowlisted.", Category: "shortener"},

		// ── Marketing (additions) ─────────────────────────────────────────
		{ID: "sendgrid-click", Name: "SendGrid Click Tracking", Provider: "sendgrid", BaseURL: "https://u.sendgrid.com/wf/click", ParamName: "upn", Description: "SendGrid email click-tracking redirect. The upn parameter encodes the target. Marketing-trusted domain.", Category: "marketing"},
		{ID: "mailchimp-track", Name: "Mailchimp Click Tracking", Provider: "mailchimp", BaseURL: "https://mailchi.mp/", ParamName: "u", Description: "Mailchimp (mailchi.mp) tracking redirect. Marketing platform trusted by email filters.", Category: "marketing"},
		{ID: "campaignmonitor-r", Name: "Campaign Monitor Redirect", Provider: "campaignmonitor", BaseURL: "https://createsend1.com/t/", ParamName: "", Description: "Campaign Monitor (createsend1.com) tracking redirect. Path-based. Email marketing trusted domain.", Category: "marketing"},

		// ── Custom ────────────────────────────────────────────────────────
		{ID: "custom", Name: "Custom Open Redirect", Provider: "custom", BaseURL: "", ParamName: "url", Description: "Add your own discovered open redirect endpoint. Test before importing.", Category: "custom"},
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
	ae := NewAuditEvent("OpenRedirect.BulkTest", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
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
	ae := NewAuditEvent("OpenRedirect.ImportFromSource", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		s.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	name, err := vo.NewString64(source.Name)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	baseURL, err := vo.NewString1024(source.BaseURL)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	platform, err := vo.NewString64(source.Provider)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	redirect := &model.OpenRedirect{
		Name:     nullable.NewNullableWithValue(*name),
		BaseURL:  nullable.NewNullableWithValue(*baseURL),
		Platform: nullable.NewNullableWithValue(*platform),
	}

	// ParamName is optional for path-based redirects (e.g., Google AMP /amp/s/)
	if source.ParamName != "" {
		paramName, err := vo.NewString64(source.ParamName)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		redirect.ParamName = nullable.NewNullableWithValue(*paramName)
	} else {
		// For path-based redirects, store "path" as the param type
		pathParam, _ := vo.NewString64("path")
		redirect.ParamName = nullable.NewNullableWithValue(*pathParam)
	}

	if companyID != nil {
		redirect.CompanyID = nullable.NewNullableWithValue(*companyID)
	}

	id, err := s.OpenRedirectRepository.Insert(ctx, redirect)
	if err != nil {
		s.Logger.Errorw("failed to import open redirect from source", "error", err)
		return nil, errs.Wrap(err)
	}
	ae.Details["id"] = id.String()
	s.AuditLogAuthorized(ae)
	return id, nil
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

// executeRedirectTest performs the actual HTTP redirect test.
//
// The logic handles three real-world flavours of open redirect:
//
//  1. HTTP 30x with a Location header (the classic case).
//  2. HTTP 200 with a <meta http-equiv="refresh"> tag (very common on
//     Google /url, DuckDuckGo /l/, LinkedIn, etc.).
//  3. HTTP 200 with a JavaScript-driven navigation (window.location = …).
//
// The function follows up to maxRedirectTestHops redirects using a custom
// CheckRedirect hook that records each hop. At the final response it
// parses the body (capped to maxRedirectTestBodyBytes) looking for
// meta-refresh or JS navigation pointing at the expected target.
//
// Status values:
//
//	"working" — the redirect reached (or clearly points at) the target
//	"warning" — endpoint returned 200 and mentions the target, but no
//	            automatic redirect was detected; operator should verify
//	"failed"  — nothing useful happened
func (s *OpenRedirect) executeRedirectTest(testURL, expectedTarget string) *model.OpenRedirectTestResult {
	result := &model.OpenRedirectTestResult{
		URL:    testURL,
		Status: "failed",
	}

	// SSRF guard: reject non-public targets before any HTTP call
	if err := validatePublicURL(testURL); err != nil {
		result.Error = err.Error()
		return result
	}

	hops := make([]model.OpenRedirectTestResultHop, 0, 4)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Record the previous hop (the 3xx response) before following.
			if len(via) > 0 {
				prev := via[len(via)-1]
				hops = append(hops, model.OpenRedirectTestResultHop{
					URL:      prev.URL.String(),
					Location: req.URL.String(),
					// StatusCode for the prior hop is unknown from this hook;
					// set it to 0 as a placeholder — the final hop's status
					// (and the working/target match below) is what matters.
				})
			}
			if len(via) >= maxRedirectTestHops {
				return errTooManyRedirects
			}
			// Still apply SSRF guard at each hop — any Location header
			// pointing at a private/loopback address must be refused.
			if err := validatePublicURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Get(testURL)
	elapsed := time.Since(start)
	result.ResponseTimeMs = elapsed.Milliseconds()

	if err != nil {
		// http.Client returns a *url.Error wrapping CheckRedirect errors; if
		// the last response is attached, read it so we can still report a
		// status code (useful for operators debugging blocked targets).
		if resp != nil {
			result.StatusCode = resp.StatusCode
			resp.Body.Close() //nolint:errcheck
		}
		result.Error = err.Error()
		result.Hops = hops
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	finalURL := resp.Request.URL.String()
	result.FinalURL = finalURL

	// Append the terminal hop.
	hops = append(hops, model.OpenRedirectTestResultHop{
		URL:        finalURL,
		StatusCode: resp.StatusCode,
	})
	result.Hops = hops

	decodedTarget, _ := url.QueryUnescape(expectedTarget)

	// Case 1: the client already followed a chain of 3xx hops and landed
	// on the target (or a URL containing the target). That's the strongest
	// signal — mark as working/http.
	if urlMatchesTarget(finalURL, expectedTarget, decodedTarget) {
		result.IsWorking = true
		result.Status = "working"
		result.RedirectMethod = "http"
		return result
	}

	// Case 2: 200 OK with a meta-refresh or JS-driven redirect. Parse the
	// body (capped) and try to extract a target URL.
	if resp.StatusCode == 200 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxRedirectTestBodyBytes))
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			result.Error = "failed to read response body: " + readErr.Error()
			return result
		}

		if metaURL := extractMetaRefreshURL(body); metaURL != "" {
			if urlMatchesTarget(metaURL, expectedTarget, decodedTarget) {
				result.IsWorking = true
				result.Status = "working"
				result.RedirectMethod = "meta"
				result.FinalURL = metaURL
				return result
			}
		}
		if jsURL := extractJSRedirectURL(body); jsURL != "" {
			if urlMatchesTarget(jsURL, expectedTarget, decodedTarget) {
				result.IsWorking = true
				result.Status = "working"
				result.RedirectMethod = "js"
				result.FinalURL = jsURL
				return result
			}
		}

		// Body didn't auto-redirect, but if it mentions the target (e.g.
		// "Click here to continue" link) flag as warning rather than
		// failed — this is how Google's interstitial /url page behaves
		// for some target URLs.
		if bodyMentionsTarget(body, expectedTarget, decodedTarget) {
			result.Status = "warning"
			result.RedirectMethod = "unknown"
			result.Error = "HTTP 200 returned without automatic redirect, but response body references the target. Manual verification recommended."
			return result
		}

		result.Error = "HTTP 200 returned. No meta-refresh or JS redirect detected. Manual verification recommended."
		return result
	}

	// Case 3: 3xx but Location didn't include our target (rare — maybe the
	// provider validated the target and redirected to an error page).
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		result.Error = fmt.Sprintf("HTTP %d returned but Location did not reach the expected target", resp.StatusCode)
		return result
	}

	// Everything else: bare failure.
	result.Error = fmt.Sprintf("HTTP %d returned", resp.StatusCode)
	return result
}

// urlMatchesTarget returns true when candidate contains either the raw or
// URL-decoded form of the expected test target. We match on substring
// because providers often append their own query params (e.g. ved=, usg=).
func urlMatchesTarget(candidate, expectedTarget, decodedTarget string) bool {
	if candidate == "" {
		return false
	}
	if strings.Contains(candidate, expectedTarget) {
		return true
	}
	if decodedTarget != "" && decodedTarget != expectedTarget && strings.Contains(candidate, decodedTarget) {
		return true
	}
	// Also try decoding the candidate once — some providers URL-encode
	// the whole Location (e.g. Google wraps the target in %3A/%2F).
	if decoded, err := url.QueryUnescape(candidate); err == nil && decoded != candidate {
		if strings.Contains(decoded, expectedTarget) {
			return true
		}
		if decodedTarget != "" && strings.Contains(decoded, decodedTarget) {
			return true
		}
	}
	return false
}

// extractMetaRefreshURL scans body for a <meta http-equiv="refresh" …> tag
// and returns the target URL, or "" if none is found.
func extractMetaRefreshURL(body []byte) string {
	m := metaRefreshRe.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// extractJSRedirectURL scans body for the first common JS-driven navigation
// (window.location = "…" / location.replace("…") / …) and returns the URL.
func extractJSRedirectURL(body []byte) string {
	m := jsLocationRe.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

// bodyMentionsTarget returns true when body contains either form of the
// expected target anywhere (attribute value, text node, etc.). Used to
// downgrade a 200-without-redirect into a "warning" rather than "failed"
// when the page clearly references our target.
func bodyMentionsTarget(body []byte, expectedTarget, decodedTarget string) bool {
	s := string(body)
	if strings.Contains(s, expectedTarget) {
		return true
	}
	if decodedTarget != "" && decodedTarget != expectedTarget && strings.Contains(s, decodedTarget) {
		return true
	}
	return false
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
