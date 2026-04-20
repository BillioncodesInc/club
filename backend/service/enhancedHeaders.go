package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
)

// EnhancedHeaders generates realistic email headers that mimic legitimate
// mail servers (Exchange, O365, Google Workspace) to improve deliverability.
//
// Ported from: ghostsenderintegration/ghost-sender-node/services/enhanced-headers.js
//
// This service generates header sets that can be added via the existing
// SMTPConfiguration.Headers mechanism (SMTP headers) or APISender.RequestHeaders
// (API sender headers). It does NOT replace those systems — it generates
// header values that users can then add to their SMTP configs.
type EnhancedHeaders struct {
	Common
}

// HeaderProfile defines a set of headers that mimic a specific mail server
type HeaderProfile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Headers     map[string]string `json:"headers"`
}

// GenerateExchangeHeaders generates headers that mimic Microsoft Exchange/O365
func (e *EnhancedHeaders) GenerateExchangeHeaders(
	ctx context.Context,
	session *model.Session,
	fromDomain string,
	fromEmail string,
	toEmail string,
) (*HeaderProfile, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	messageID := generateMessageID(fromDomain)
	tenantID := generateUUID()
	now := time.Now().UTC()

	headers := map[string]string{
		"X-MS-Exchange-Organization-SCL":             "-1",
		"X-MS-Exchange-Organization-AuthSource":      fmt.Sprintf("mail.%s", fromDomain),
		"X-MS-Exchange-Organization-AuthAs":          "Internal",
		"X-MS-Exchange-Organization-AuthMechanism":   "04",
		"X-MS-Exchange-Organization-Network-Message-Id": generateUUID(),
		"X-MS-Has-Attach":                            "",
		"X-MS-TNEF-Correlator":                       "",
		"X-MS-Exchange-MessageSentRepresentingType":  "1",
		"X-OriginatorOrg":                            fromDomain,
		"X-MS-Exchange-CrossTenant-OriginalArrivalTime": now.Format("02 Jan 2006 15:04:05.0000 (UTC)"),
		"X-MS-Exchange-CrossTenant-Network-Message-Id":  generateUUID(),
		"X-MS-Exchange-CrossTenant-Id":               tenantID,
		"X-MS-Exchange-CrossTenant-AuthSource":        fmt.Sprintf("mail.%s", fromDomain),
		"X-MS-Exchange-CrossTenant-AuthAs":            "Internal",
		"X-MS-Exchange-CrossTenant-originalarrivaltime": now.Format("02 Jan 2006 15:04:05.0000 (UTC)"),
		"X-MS-Exchange-CrossTenant-fromentityheader":    "Hosted",
		"X-MS-Exchange-Transport-CrossTenantHeadersStamped": fmt.Sprintf("SN6PR0%d.namprd%02d.prod.outlook.com", randInt(1, 9), randInt(1, 20)),
		"Message-ID":    messageID,
		"Return-Path":   fromEmail,
		"X-Mailer":      "Microsoft Outlook 16.0",
		"Thread-Index":  generateThreadIndex(),
		"X-MS-Exchange-Organization-AVStamp-Enterprise": "1.0",
	}

	return &HeaderProfile{
		Name:        "Microsoft Exchange / O365",
		Description: "Headers that mimic Microsoft Exchange Online / Office 365 mail flow",
		Headers:     headers,
	}, nil
}

// GenerateGoogleHeaders generates headers that mimic Google Workspace
func (e *EnhancedHeaders) GenerateGoogleHeaders(
	ctx context.Context,
	session *model.Session,
	fromDomain string,
	fromEmail string,
	toEmail string,
) (*HeaderProfile, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	messageID := fmt.Sprintf("<%s@mail.gmail.com>", generateHex(32))

	headers := map[string]string{
		"X-Gm-Message-State":    generateBase64Like(76),
		"X-Google-DKIM-Signature": fmt.Sprintf("v=1; a=rsa-sha256; c=relaxed/relaxed; d=1e100.net; s=20230601; t=%d", time.Now().Unix()),
		"X-Google-Smtp-Source":   generateBase64Like(44),
		"X-Received":            fmt.Sprintf("by 2002:a05:6a00:%04x:b0:%s with SMTP id %s; %s",
			randInt(1, 0xffff),
			generateHex(12),
			generateGoogleSmtpID(),
			time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700 (MST)")),
		"Message-ID":   messageID,
		"Return-Path":  fromEmail,
		"X-Mailer":     "",
		"Precedence":   "bulk",
	}

	return &HeaderProfile{
		Name:        "Google Workspace / Gmail",
		Description: "Headers that mimic Google Workspace / Gmail mail flow",
		Headers:     headers,
	}, nil
}

// GenerateGenericHeaders generates clean, generic headers for any SMTP server
func (e *EnhancedHeaders) GenerateGenericHeaders(
	ctx context.Context,
	session *model.Session,
	fromDomain string,
	fromEmail string,
	toEmail string,
) (*HeaderProfile, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	messageID := generateMessageID(fromDomain)

	headers := map[string]string{
		"Message-ID":                messageID,
		"Return-Path":              fromEmail,
		"X-Mailer":                 "Postfix",
		"X-Originating-IP":         fmt.Sprintf("[10.%d.%d.%d]", randInt(0, 255), randInt(0, 255), randInt(1, 254)),
		"X-Priority":               "3",
		"Importance":               "Normal",
		"X-Auto-Response-Suppress": "All",
		"Auto-Submitted":           "no",
	}

	return &HeaderProfile{
		Name:        "Generic SMTP",
		Description: "Clean, generic headers suitable for any SMTP server",
		Headers:     headers,
	}, nil
}

// GenerateAllProfiles returns all available header profiles for a given sender
func (e *EnhancedHeaders) GenerateAllProfiles(
	ctx context.Context,
	session *model.Session,
	fromDomain string,
	fromEmail string,
	toEmail string,
) ([]*HeaderProfile, error) {
	exchange, err := e.GenerateExchangeHeaders(ctx, session, fromDomain, fromEmail, toEmail)
	if err != nil {
		return nil, err
	}

	google, err := e.GenerateGoogleHeaders(ctx, session, fromDomain, fromEmail, toEmail)
	if err != nil {
		return nil, err
	}

	generic, err := e.GenerateGenericHeaders(ctx, session, fromDomain, fromEmail, toEmail)
	if err != nil {
		return nil, err
	}

	return []*HeaderProfile{exchange, google, generic}, nil
}

// ─── Email Spoofing Headers (ported from GhostSender identity services) ────

// SpoofConfig holds configuration for email spoofing headers
type SpoofConfig struct {
	DisplayFrom   string `json:"displayFrom"`   // Display name shown to recipient
	ActualFrom    string `json:"actualFrom"`    // Actual sending email address
	ReplyTo       string `json:"replyTo"`       // Reply-To redirect address
	ReturnPath    string `json:"returnPath"`    // Return-Path for bounces
	SenderAddress string `json:"senderAddress"` // Sender header (can differ from From)
	DispositionNotificationTo string `json:"dispositionNotificationTo,omitempty"` // Read receipt
}

// GenerateSpoofHeaders generates email headers for sender identity spoofing
func (e *EnhancedHeaders) GenerateSpoofHeaders(
	ctx context.Context,
	session *model.Session,
	config *SpoofConfig,
) (*HeaderProfile, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil && !errors.Is(err, errs.ErrAuthorizationFailed) {
		return nil, errs.Wrap(err)
	}
	if !isAuthorized {
		return nil, errs.ErrAuthorizationFailed
	}

	headers := make(map[string]string)

	// Display name masking: "John Smith" <actual@sender.com>
	if config.DisplayFrom != "" && config.ActualFrom != "" {
		headers["From"] = fmt.Sprintf("\"%s\" <%s>", config.DisplayFrom, config.ActualFrom)
	}

	// Reply-To redirect: replies go to a different address
	if config.ReplyTo != "" {
		headers["Reply-To"] = config.ReplyTo
	}

	// Return-Path: bounces go to a different address
	if config.ReturnPath != "" {
		headers["Return-Path"] = fmt.Sprintf("<%s>", config.ReturnPath)
	}

	// Sender header: can differ from From to indicate delegation
	if config.SenderAddress != "" {
		headers["Sender"] = config.SenderAddress
	}

	// Read receipt request
	if config.DispositionNotificationTo != "" {
		headers["Disposition-Notification-To"] = config.DispositionNotificationTo
	}

	return &HeaderProfile{
		Name:        "Email Spoofing",
		Description: "Headers for sender identity spoofing with display name masking and reply-to redirect",
		Headers:     headers,
	}, nil
}

// ─── Helper Functions ────────────────────────────────────────────────

func generateMessageID(domain string) string {
	return fmt.Sprintf("<%s@%s>", generateHex(16), domain)
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateHex(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func generateBase64Like(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

func generateThreadIndex() string {
	// Thread-Index is a base64-encoded 22-byte value in Exchange
	b := make([]byte, 22)
	rand.Read(b)
	return generateBase64Like(30)
}

func generateGoogleSmtpID() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	var result strings.Builder
	for i := 0; i < 20; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result.WriteByte(chars[n.Int64()])
	}
	return result.String()
}

func randInt(min, max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	return int(n.Int64()) + min
}
