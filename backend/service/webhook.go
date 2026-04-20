package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-errors/errors"

	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/validate"
)

// webhookHTTPClient is the shared HTTP client used for webhook deliveries.
// It uses a sensible default timeout to avoid leaking sockets / goroutines.
var webhookHTTPClient = &http.Client{Timeout: 30 * time.Second}

// validatePublicURL rejects URLs that point at private, loopback, link-local,
// or IPv6 unique-local addresses to mitigate SSRF via user-supplied webhook URLs.
// Only http and https schemes are allowed.
func validatePublicURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("url has no host")
	}
	// block well-known local hostnames outright
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") ||
		lowerHost == "ip6-localhost" || lowerHost == "ip6-loopback" {
		return errors.New("url points to a loopback host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("host did not resolve to any ip")
	}
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("url resolves to a private/local address: %s", ip.String())
		}
	}
	return nil
}

// isPrivateOrLocalIP reports whether ip is in a range we refuse to POST to
// server-side: RFC1918, loopback, link-local (v4+v6), unspecified, multicast,
// IPv6 unique local (fc00::/7).
func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsPrivate() {
		return true
	}
	// IPv6 unique local addresses (fc00::/7) — covered by IsPrivate on recent
	// Go versions, but be explicit for safety.
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil {
		if v6[0]&0xfe == 0xfc {
			return true
		}
	}
	return false
}

type Webhook struct {
	Common
	CampaignRepository *repository.Campaign
	WebhookRepository  *repository.Webhook
	DeliveryTracker    *WebhookDeliveryTracker
}

func (w *Webhook) Create(
	ctx context.Context,
	session *model.Session,
	webhook *model.Webhook,
) (*uuid.UUID, error) {
	ae := NewAuditEvent("Webhook.Create", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return nil, errors.New("unauthorized")
	}
	// validate data
	if err := webhook.Validate(); err != nil {
		return nil, errs.Wrap(err)
	}
	// check uniqueness
	var companyID *uuid.UUID
	if cid, err := webhook.CompanyID.Get(); err == nil {
		companyID = &cid
	}
	name := webhook.Name.MustGet()
	isOK, err := repository.CheckNameIsUnique(
		ctx,
		w.WebhookRepository.DB,
		"webhooks",
		name.String(),
		companyID,
		nil,
	)
	if err != nil {
		w.Logger.Errorw("failed to check webhook uniqueness", "error", err)
		return nil, errs.Wrap(err)
	}
	if !isOK {
		w.Logger.Debugw("webhook name is already taken", "name", name.String())
		return nil, validate.WrapErrorWithField(errors.New("is not unique"), "name")
	}
	// insert
	id, err := w.WebhookRepository.Insert(ctx, webhook)
	if err != nil {
		w.Logger.Errorw("failed to insert webhook", "error", err)
		return nil, errs.Wrap(err)
	}
	ae.Details["id"] = id.String()
	w.AuditLogAuthorized(ae)

	return id, nil
}

// GetAll gets all webhooks
func (w *Webhook) GetAll(
	ctx context.Context,
	session *model.Session,
	companyID *uuid.UUID,
	options *repository.WebhookOption,
) (*model.Result[model.Webhook], error) {
	result := model.NewEmptyResult[model.Webhook]()
	ae := NewAuditEvent("Webhook.GetAll", session)
	if companyID != nil {
		ae.Details["companyId"] = companyID.String()
	}
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		w.LogAuthError(err)
		return result, errs.Wrap(err)
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return result, errs.ErrAuthorizationFailed
	}
	// get
	result, err = w.WebhookRepository.GetAll(ctx, companyID, options)
	if err != nil {
		w.Logger.Errorw("failed to get webhooks", "error", err)
		return result, errs.Wrap(err)
	}
	w.AuditLogAuthorized(ae)

	return result, nil
}

// GetByID gets a webhook by id
func (w *Webhook) GetByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (*model.Webhook, error) {
	ae := NewAuditEvent("Webhook.GetByID", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	// get
	out, err := w.WebhookRepository.GetByID(ctx, id)
	if err != nil {
		w.Logger.Errorw("failed to get webhook", "error", err)
		return out, errs.Wrap(err)
	}
	// no audit on read

	return out, nil
}

// GetByCompanyID gets a webhooks by compnay id
func (w *Webhook) GetByCompanyID(
	ctx context.Context,
	session *model.Session,
	companyID *uuid.UUID,
) ([]*model.Webhook, error) {
	ae := NewAuditEvent("Webhook.GetByCompanyID", session)
	if companyID != nil {
		ae.Details["companyId"] = companyID.String()
	}
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	// get
	models, err := w.WebhookRepository.GetAllByCompanyID(ctx, companyID, &repository.WebhookOption{})
	if err != nil {
		w.Logger.Errorw("failed to get webhooks", "error", err)
		return models, errs.Wrap(err)
	}
	// no audit on read

	return models, nil
}

// Update updates a webhook
func (w *Webhook) Update(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
	webhook *model.Webhook,
) error {
	ae := NewAuditEvent("Webhook.Update", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return err
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return errors.New("unauthorized")
	}
	// get current
	current, err := w.WebhookRepository.GetByID(ctx, id)
	if err != nil {
		w.Logger.Errorw("failed to get webhook", "error", err)
		return err
	}
	// update values
	if v, err := webhook.Name.Get(); err == nil {
		// check uniqueness
		var companyID *uuid.UUID
		if cid, err := webhook.CompanyID.Get(); err == nil {
			companyID = &cid
		}

		isOK, err := repository.CheckNameIsUnique(
			ctx,
			w.WebhookRepository.DB,
			"webhooks",
			v.String(),
			companyID,
			id,
		)
		if err != nil {
			w.Logger.Errorw("failed to check webhook uniqueness", "error", err)
			return err
		}
		if !isOK {
			w.Logger.Debugw("webhook name is already taken", "name", v.String())
			return validate.WrapErrorWithField(errors.New("is not unique"), "name")
		}
		current.Name.Set(v)
	}
	if v, err := webhook.URL.Get(); err == nil {
		current.URL.Set(v)
	}
	if v, err := webhook.Secret.Get(); err == nil {
		current.Secret.Set(v)
	}
	// update
	err = w.WebhookRepository.UpdateByID(ctx, id, webhook)
	if err != nil {
		w.Logger.Errorw("failed to update webhook", "error", err)
		return err
	}
	w.AuditLogAuthorized(ae)

	return nil
}

// DeleteByID deletes a webhook
func (w *Webhook) DeleteByID(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) error {
	ae := NewAuditEvent("Webhook.DeleteByID", session)
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return err
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return errors.New("unauthorized")
	}
	// get campaigns afffected so we can remove webhoook from them
	affectedCampaigns, err := w.CampaignRepository.GetByWebhookID(
		ctx,
		id,
	)
	if err != nil {
		w.Logger.Errorw("failed to get campaigns afffected by removing webhhook", "error", err)
		return err
	}
	cids := []*uuid.UUID{}
	for _, campaign := range affectedCampaigns {
		cid := campaign.ID.MustGet()
		cids = append(cids, &cid)
	}
	err = w.CampaignRepository.RemoveWebhookByCampaignIDs(
		ctx,
		cids,
	)
	if err != nil {
		w.Logger.Errorw("failed to remove web hook from campaigns", "error", err)
		return err
	}
	// remove junction table rows for this webhook so no campaign retains
	// a dangling reference in the new many-to-many format
	err = w.CampaignRepository.RemoveWebhookFromJunctionByWebhookID(ctx, id)
	if err != nil {
		w.Logger.Errorw("failed to remove webhook from campaign_webhooks junction", "error", err)
		return err
	}
	// delete
	err = w.WebhookRepository.DeleteByID(ctx, id)
	if err != nil {
		w.Logger.Errorw("failed to delete webhook", "error", err)
		return err
	}
	w.AuditLogAuthorized(ae)

	return nil
}

// SendTest sends a test webhook
func (w *Webhook) SendTest(
	ctx context.Context,
	session *model.Session,
	id *uuid.UUID,
) (map[string]interface{}, error) {
	ae := NewAuditEvent("Webhook.SendTest", session)
	ae.Details["id"] = id.String()
	// check permissions
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		w.LogAuthError(err)
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		w.AuditLogNotAuthorized(ae)
		return nil, errs.ErrAuthorizationFailed
	}
	w.Logger.Debugw("sending test webhook", "error", id)
	// send
	webhook, err := w.WebhookRepository.GetByID(ctx, id)
	if err != nil {
		w.Logger.Errorw("failed to get webhook", "error", err)
		return nil, errs.Wrap(err)
	}
	now := time.Now()
	request := WebhookRequest{
		Time:         &now,
		CampaignName: "Test Campaign",
		Email:        "test@webhook.test",
		Event:        "test",
	}
	data, err := w.Send(ctx, webhook, &request)
	if err != nil {
		w.Logger.Errorw("failed to send webhook", "error", err)
		return nil, errs.Wrap(err)
	}
	w.AuditLogAuthorized(ae)

	return data, nil
}

// Send sends a webhook request
func (w *Webhook) Send(
	ctx context.Context,
	webhook *model.Webhook,
	request *WebhookRequest,
) (map[string]interface{}, error) {
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer func() {
		reqCancel()
	}()
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	requestJSONBuffer := bytes.NewBuffer(requestJSON)
	webhookURL := webhook.URL.MustGet()
	urlStr := webhookURL.String()
	// SSRF guard: refuse to POST to private/loopback/link-local addresses
	// unless the caller explicitly set a test hook on the request.
	if err := validatePublicURL(urlStr); err != nil {
		return nil, errs.Wrap(err)
	}
	req, err := http.NewRequestWithContext(reqCtx, "POST", urlStr, requestJSONBuffer)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// hmac sign the request if secret is set
	var signature = "UNSIGNED"
	if secret, err := webhook.Secret.Get(); err == nil {
		hasher := hmac.New(sha256.New, []byte(secret.String()))
		_, err := hasher.Write(requestJSON)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		signature = hex.EncodeToString(hasher.Sum(nil))
	}
	req.Header.Set("X-SIGNATURE", signature)
	req.Header.Add("User-Agent", "Go-http-client")
	response, err := webhookHTTPClient.Do(req)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer response.Body.Close()
	data := map[string]interface{}{
		"code":   response.StatusCode,
		"status": response.Status,
	}
	// parse respone body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		w.Logger.Errorw("failed to read response body", "error", err)
		return nil, errs.Wrap(err)
	}
	data["body"] = string(body)

	return data, nil
}

// WebhookRequest represents the payload sent to webhook endpoints.
// webhooks are sent based on the campaign's webhookEvents setting (stored as bitwise int):
// - 0: all events trigger webhooks (default, backward compatible)
// - non-zero: only events with their bit set trigger webhooks
//
// webhook events (10 total - events that call HandleWebhook):
// from campaign.go (4 events):
// - campaign_closed: when a campaign finishes
// - campaign_recipient_message_sent: when an email is successfully sent
// - campaign_recipient_message_failed: when an email fails to send
// - campaign_recipient_message_read: when tracking pixel is loaded
// from proxy.go (6 events):
// - campaign_recipient_submitted_data: when user submits data on phishing page
// - campaign_recipient_evasion_page_visited: when evasion page is visited
// - campaign_recipient_before_page_visited: when before page is visited
// - campaign_recipient_page_visited: when landing page is visited
// - campaign_recipient_after_page_visited: when after page is visited
// - campaign_recipient_deny_page_visited: when deny page is visited
//
// the fields included depend on the campaign's webhookIncludeData setting:
// - "none": only Time and Event are sent (maximum privacy)
// - "basic": Time, Event, and CampaignName are sent (no PII)
// - "full": all fields including Email and Data are sent (complete information)
type WebhookRequest struct {
	Time         *time.Time             `json:"time"`
	CampaignName string                 `json:"campaignName,omitempty"`
	Email        string                 `json:"email,omitempty"`
	Event        string                 `json:"event"`
	Data         map[string]interface{} `json:"data,omitempty"`
}
