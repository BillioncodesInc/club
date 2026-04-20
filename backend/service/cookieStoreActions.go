package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/database"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// v1.0.56 – Phase 2: Cookie Store message-action endpoints
//
// These methods wrap Microsoft Graph API calls for single-message actions
// (mark read/unread, flag/unflag, delete, move) and attachment download,
// plus bulk operations that iterate over a list of message IDs with a
// bounded goroutine pool.
//
// Graceful degradation: when no access token is available (e.g. an OWA-only
// cookie session without a refresh token), these methods return a
// user-visible error that the controller surfaces as a toast. They do NOT
// panic and do NOT break existing flows.

// ErrGraphUnavailable indicates the action requires a Graph API token
// that could not be obtained for this session.
var ErrGraphUnavailable = fmt.Errorf("action not supported for this session")

// MessageActionResult is the result of a single-message action.
type MessageActionResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId"`
	Error     string `json:"error,omitempty"`
	Method    string `json:"method"`
}

// BulkMessageActionRequest is a bulk request body.
type BulkMessageActionRequest struct {
	Action              string   `json:"action"`
	MessageIDs          []string `json:"messageIds"`
	DestinationFolderID string   `json:"destinationFolderId,omitempty"`
	IsRead              *bool    `json:"isRead,omitempty"`
	Flagged             *bool    `json:"flagged,omitempty"`
}

// BulkMessageActionResult aggregates per-message outcomes.
type BulkMessageActionResult struct {
	Action    string                `json:"action"`
	Total     int                   `json:"total"`
	Succeeded int                   `json:"succeeded"`
	Failed    int                   `json:"failed"`
	Results   []MessageActionResult `json:"results"`
}

// --- Internal helper: resolve a Graph access token (or empty) ---
func (s *CookieStoreService) resolveAccessToken(ctx context.Context, store *database.CookieStore) string {
	if token := s.getOrRefreshAccessToken(ctx, store); token != "" {
		return token
	}
	// Browser fallback: try to get a fresh token via a headless browser session.
	if s.BrowserSession != nil {
		browserResult, err := s.BrowserSession.ValidateAndGetToken(ctx, store.CookiesJSON)
		if err == nil && browserResult != nil && browserResult.Valid && browserResult.AccessToken != "" {
			s.cacheAccessToken(ctx, store.ID, browserResult)
			return browserResult.AccessToken
		}
	}
	return ""
}

// --- Authorization helper ---
func (s *CookieStoreService) authorizeAndLoadStore(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
) (*database.CookieStore, error) {
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
	return store, nil
}

// --- PATCH /me/messages/{id} with a JSON body ---
func (s *CookieStoreService) graphPatchMessage(
	ctx context.Context,
	accessToken, messageID string,
	body map[string]interface{},
) error {
	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s", messageID)
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("Graph API PATCH failed (HTTP %d): %s", resp.StatusCode, string(respBody))
}

// --- 1. Mark Read / Unread ---
func (s *CookieStoreService) MarkMessageRead(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
	isRead bool,
) error {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		s.Logger.Warnw("MarkMessageRead: no access token available", "storeID", storeID)
		return ErrGraphUnavailable
	}
	if err := s.graphPatchMessage(ctx, token, messageID, map[string]interface{}{"isRead": isRead}); err != nil {
		s.Logger.Warnw("MarkMessageRead failed", "storeID", storeID, "messageID", messageID, "error", err)
		return err
	}
	return nil
}

// --- 2. Flag / Unflag ---
func (s *CookieStoreService) FlagMessage(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
	flagged bool,
) error {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		s.Logger.Warnw("FlagMessage: no access token available", "storeID", storeID)
		return ErrGraphUnavailable
	}
	flagStatus := "notFlagged"
	if flagged {
		flagStatus = "flagged"
	}
	body := map[string]interface{}{
		"flag": map[string]interface{}{
			"flagStatus": flagStatus,
		},
	}
	if err := s.graphPatchMessage(ctx, token, messageID, body); err != nil {
		s.Logger.Warnw("FlagMessage failed", "storeID", storeID, "messageID", messageID, "error", err)
		return err
	}
	return nil
}

// --- 3. Delete ---
func (s *CookieStoreService) DeleteMessage(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
) error {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		s.Logger.Warnw("DeleteMessage: no access token available", "storeID", storeID)
		return ErrGraphUnavailable
	}

	// Primary path: DELETE /me/messages/{id} (moves to Deleted Items)
	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s", messageID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		s.Logger.Warnw("DeleteMessage primary DELETE failed, trying move fallback",
			"storeID", storeID, "messageID", messageID, "status", resp.StatusCode)
	}

	// Fallback: POST /me/messages/{id}/move with destinationId=deleteditems
	_, moveErr := s.moveMessageViaGraph(ctx, token, messageID, "deleteditems")
	if moveErr != nil {
		return fmt.Errorf("delete failed and move-to-deleteditems fallback also failed: %w", moveErr)
	}
	return nil
}

// --- 4. Move ---
func (s *CookieStoreService) MoveMessage(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
	destinationFolderID string,
) (string, error) {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return "", err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		s.Logger.Warnw("MoveMessage: no access token available", "storeID", storeID)
		return "", ErrGraphUnavailable
	}
	newID, err := s.moveMessageViaGraph(ctx, token, messageID, destinationFolderID)
	if err != nil {
		s.Logger.Warnw("MoveMessage failed", "storeID", storeID, "messageID", messageID,
			"destinationFolderID", destinationFolderID, "error", err)
		return "", err
	}
	return newID, nil
}

// moveMessageViaGraph issues a POST /me/messages/{id}/move and returns the new message ID.
func (s *CookieStoreService) moveMessageViaGraph(
	ctx context.Context,
	accessToken, messageID, destinationFolderID string,
) (string, error) {
	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/messages/%s/move", messageID)
	payload, _ := json.Marshal(map[string]interface{}{
		"destinationId": destinationFolderID,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Graph API move failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	// Response is the newly-moved message; extract its id when present.
	var moved struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &moved)
	return moved.ID, nil
}

// --- 5. Bulk actions ---
func (s *CookieStoreService) BulkMessageAction(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	req *BulkMessageActionRequest,
) (*BulkMessageActionResult, error) {
	// Authorize once up front. Per-ID calls go through the single-action
	// methods which each re-authorize — that's fine and matches the pattern
	// used by BulkRevalidate.
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	result := &BulkMessageActionResult{
		Action: req.Action,
		Total:  len(req.MessageIDs),
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, msgID := range req.MessageIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(messageID string) {
			defer wg.Done()
			defer func() { <-sem }()

			entry := MessageActionResult{MessageID: messageID, Method: "graph_api"}
			var actionErr error
			switch req.Action {
			case "markRead":
				actionErr = s.MarkMessageRead(ctx, session, storeID, messageID, true)
			case "markUnread":
				actionErr = s.MarkMessageRead(ctx, session, storeID, messageID, false)
			case "flag":
				actionErr = s.FlagMessage(ctx, session, storeID, messageID, true)
			case "unflag":
				actionErr = s.FlagMessage(ctx, session, storeID, messageID, false)
			case "delete":
				actionErr = s.DeleteMessage(ctx, session, storeID, messageID)
			case "archive":
				_, actionErr = s.MoveMessage(ctx, session, storeID, messageID, "archive")
			case "move":
				dest := req.DestinationFolderID
				if dest == "" {
					actionErr = fmt.Errorf("destinationFolderId is required for move")
				} else {
					_, actionErr = s.MoveMessage(ctx, session, storeID, messageID, dest)
				}
			default:
				actionErr = fmt.Errorf("unknown action: %s", req.Action)
			}

			if actionErr != nil {
				entry.Error = actionErr.Error()
			} else {
				entry.Success = true
			}
			mu.Lock()
			if entry.Success {
				result.Succeeded++
			} else {
				result.Failed++
			}
			result.Results = append(result.Results, entry)
			mu.Unlock()
		}(msgID)
	}
	wg.Wait()
	return result, nil
}

// --- 6. Attachment download ---
//
// DownloadAttachment streams the raw bytes of a single message attachment
// through the writer. It returns filename and MIME type for the caller to
// set response headers. If no Graph token is available, returns
// ErrGraphUnavailable.
func (s *CookieStoreService) DownloadAttachment(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID, attachmentID string,
	w io.Writer,
) (filename, contentType string, err error) {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return "", "", err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		s.Logger.Warnw("DownloadAttachment: no access token available", "storeID", storeID)
		return "", "", ErrGraphUnavailable
	}

	// First grab attachment metadata to learn filename + contentType.
	metaURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages/%s/attachments/%s?$select=name,contentType,size",
		messageID, attachmentID,
	)
	metaReq, _ := http.NewRequestWithContext(ctx, "GET", metaURL, nil)
	metaReq.Header.Set("Authorization", "Bearer "+token)
	metaReq.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	metaResp, metaErr := client.Do(metaReq)
	if metaErr == nil && metaResp != nil {
		defer metaResp.Body.Close()
		if metaResp.StatusCode == 200 {
			var meta struct {
				Name        string `json:"name"`
				ContentType string `json:"contentType"`
			}
			_ = json.NewDecoder(metaResp.Body).Decode(&meta)
			filename = meta.Name
			contentType = meta.ContentType
		}
	}

	// Now fetch the raw bytes via $value.
	rawURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages/%s/attachments/%s/$value",
		messageID, attachmentID,
	)
	rawReq, rawReqErr := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if rawReqErr != nil {
		return filename, contentType, rawReqErr
	}
	rawReq.Header.Set("Authorization", "Bearer "+token)

	rawResp, rawErr := client.Do(rawReq)
	if rawErr != nil {
		return filename, contentType, rawErr
	}
	defer rawResp.Body.Close()
	if rawResp.StatusCode < 200 || rawResp.StatusCode >= 300 {
		body, _ := io.ReadAll(rawResp.Body)
		return filename, contentType, fmt.Errorf("attachment download failed (HTTP %d): %s",
			rawResp.StatusCode, string(body))
	}

	// If the metadata call didn't populate content type, fall back to the response header.
	if contentType == "" {
		contentType = rawResp.Header.Get("Content-Type")
	}
	if filename == "" {
		filename = "attachment"
	}

	if _, err := io.Copy(w, rawResp.Body); err != nil {
		return filename, contentType, err
	}
	return filename, contentType, nil
}

// --- Attachment metadata (listed on the message) ---
//
// GetMessageAttachments returns the list of attachments for a message so the
// UI can render a download link per attachment.
func (s *CookieStoreService) GetMessageAttachments(
	ctx context.Context,
	session *model.Session,
	storeID uuid.UUID,
	messageID string,
) ([]model.InboxAttachmentInfo, error) {
	store, err := s.authorizeAndLoadStore(ctx, session, storeID)
	if err != nil {
		return nil, err
	}
	token := s.resolveAccessToken(ctx, store)
	if token == "" {
		return nil, ErrGraphUnavailable
	}

	apiURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages/%s/attachments?$select=id,name,contentType,size,isInline",
		messageID,
	)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Graph API attachments list failed (HTTP %d)", resp.StatusCode)
	}

	var data struct {
		Value []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			ContentType string `json:"contentType"`
			Size        int64  `json:"size"`
			IsInline    bool   `json:"isInline"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]model.InboxAttachmentInfo, 0, len(data.Value))
	for _, a := range data.Value {
		out = append(out, model.InboxAttachmentInfo{
			ID:          a.ID,
			Name:        a.Name,
			ContentType: a.ContentType,
			Size:        a.Size,
			IsInline:    a.IsInline,
		})
	}
	return out, nil
}
