package seed

import (
	"context"
	"embed"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/vo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

//go:embed templates/pages/*.html
var embeddedPages embed.FS

//go:embed templates/emails/*.html
var embeddedEmails embed.FS

// PrebuiltPageTemplate defines a prebuilt page template
type PrebuiltPageTemplate struct {
	Name    string
	Content string
}

// PrebuiltEmailTemplate defines a prebuilt email template
type PrebuiltEmailTemplate struct {
	Name             string
	Subject          string
	MailEnvelopeFrom string
	MailHeaderFrom   string
	Content          string
}

// SeedPrebuiltTemplates seeds all prebuilt page and email templates.
// This runs on every startup (both dev and production) but is idempotent —
// it skips templates that already exist by name.
func SeedPrebuiltTemplates(
	pageRepository *repository.Page,
	emailRepository *repository.Email,
	logger *zap.SugaredLogger,
) error {
	// Seed prebuilt pages (redirector templates)
	err := seedPrebuiltPages(pageRepository, logger)
	if err != nil {
		return errors.Errorf("failed to seed prebuilt pages: %w", err)
	}

	// Seed prebuilt emails (GhostSender email templates)
	err = seedPrebuiltEmails(emailRepository, logger)
	if err != nil {
		return errors.Errorf("failed to seed prebuilt emails: %w", err)
	}

	return nil
}

func seedPrebuiltPages(pageRepository *repository.Page, logger *zap.SugaredLogger) error {
	entries, err := embeddedPages.ReadDir("templates/pages")
	if err != nil {
		return errors.Errorf("failed to read embedded pages directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}

		content, err := embeddedPages.ReadFile("templates/pages/" + entry.Name())
		if err != nil {
			logger.Warnw("failed to read embedded page template", "file", entry.Name(), "error", err)
			continue
		}

		// Derive a human-readable name from the filename
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		displayName := "[Prebuilt] " + humanizeName(baseName)

		name := vo.NewString64Must(displayName)

		// Check if already exists (idempotent)
		existing, err := pageRepository.GetByNameAndCompanyID(
			context.Background(),
			name,
			nil,
			&repository.PageOption{},
		)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Errorf("failed to check existing page %s: %w", displayName, err)
		}
		if existing != nil {
			continue // already seeded
		}

		id := uuid.New()
		contentVO := vo.NewOptionalString1MBMust(string(content))
		createPage := model.Page{
			ID:        nullable.NewNullableWithValue(id),
			Name:      nullable.NewNullableWithValue(*name),
			Content:   nullable.NewNullableWithValue(*contentVO),
			CompanyID: nullable.NewNullNullable[uuid.UUID](),
		}

		_, err = pageRepository.Insert(context.TODO(), &createPage)
		if err != nil {
			logger.Warnw("failed to insert prebuilt page", "name", displayName, "error", err)
			continue
		}
		logger.Infow("seeded prebuilt page template", "name", displayName)
	}

	return nil
}

func seedPrebuiltEmails(emailRepository *repository.Email, logger *zap.SugaredLogger) error {
	entries, err := embeddedEmails.ReadDir("templates/emails")
	if err != nil {
		return errors.Errorf("failed to read embedded emails directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}

		content, err := embeddedEmails.ReadFile("templates/emails/" + entry.Name())
		if err != nil {
			logger.Warnw("failed to read embedded email template", "file", entry.Name(), "error", err)
			continue
		}

		// Derive a human-readable name from the filename
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		displayName := "[Prebuilt] " + humanizeName(baseName)

		// Generate subject from template name
		subject := generateSubjectFromName(baseName)

		name := vo.NewString64Must(displayName)

		// Check if already exists (idempotent)
		existing, err := emailRepository.GetByNameAndCompanyID(
			context.Background(),
			name,
			nil,
			&repository.EmailOption{},
		)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Errorf("failed to check existing email %s: %w", displayName, err)
		}
		if existing != nil {
			continue // already seeded
		}

		id := uuid.New()
		contentStr := string(content) + "{{.Tracker}}"
		createEmail := model.Email{
			ID:                nullable.NewNullableWithValue(id),
			Name:              nullable.NewNullableWithValue(*name),
			MailEnvelopeFrom:  nullable.NewNullableWithValue(*vo.NewMailEnvelopeFromMust("noreply@example.com")),
			MailHeaderSubject: nullable.NewNullableWithValue(*vo.NewOptionalString255Must(subject)),
			MailHeaderFrom:    nullable.NewNullableWithValue(*vo.NewEmailMust("Notification <noreply@example.com>")),
			AddTrackingPixel:  nullable.NewNullableWithValue(true),
			Content:           nullable.NewNullableWithValue(*vo.NewOptionalString1MBMust(contentStr)),
		}

		_, err = emailRepository.Insert(context.TODO(), &createEmail)
		if err != nil {
			logger.Warnw("failed to insert prebuilt email", "name", displayName, "error", err)
			continue
		}
		logger.Infow("seeded prebuilt email template", "name", displayName)
	}

	return nil
}

// humanizeName converts a filename like "coming_soon" to "Coming Soon"
func humanizeName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// generateSubjectFromName creates a reasonable email subject from template name
func generateSubjectFromName(name string) string {
	subjectMap := map[string]string{
		"Excel_essential":    "Action Required: Review Spreadsheet",
		"Exel":               "Shared Excel Document",
		"body":               "Secure Message Notification",
		"body1":              "New Voicemail Message",
		"docusgnTop":         "DocuSign: Please Review and Sign",
		"docusign":           "DocuSign: Document Ready for Signature",
		"docusign02":         "DocuSign: Signature Request",
		"dropbox":            "Dropbox: File Shared With You",
		"dropbox2":           "Dropbox: New Shared Document",
		"dropbox3":           "Dropbox: Shared File Notification",
		"email_template":     "Important Account Notification",
		"email_template_1":   "Account Security Alert",
		"email_template_2":   "Action Required: Verify Account",
		"letter":             "Important Notice",
		"letter1":            "Official Communication",
		"message":            "New Message Received",
		"payment":            "Payment Notification",
		"qr_template":        "Scan to Verify Your Account",
		"test-image-template": "Image Verification Required",
		"voicemessage":       "New Voicemail from {{.FromName}}",

		// v1.0.43 – New templates
		"microsoft_password_reset":   "Microsoft account password reset",
		"microsoft_mfa_alert":        "Microsoft account security alert: Unusual sign-in activity",
		"sharepoint_file_shared":     "{{.FromName}} shared a file with you",
		"teams_missed_message":       "You have a missed message in Microsoft Teams",
		"it_password_expiry":         "IT Notice: Your password expires soon",
		"slack_notification":         "New message in #general from {{.FromName}}",
		"zoom_meeting_invite":        "{{.FromName}} is inviting you to a scheduled Zoom meeting",
		"linkedin_message":           "{{.FromName}} sent you a new message",
		"paypal_payment_alert":       "Receipt for your payment to Coinbase Global, Inc.",
		"fedex_delivery_failed":      "FedEx: Delivery attempt failed – action required",
		"apple_icloud_storage":       "Your iCloud storage is almost full",
		"aws_billing_alert":          "Action Required: AWS payment method update needed",
		"google_workspace_security":  "Critical security alert for your Google Workspace account",
		"onedrive_file_share":        "{{.FromName}} shared a file with you via OneDrive",
		"m365_subscription_alert":    "Action Required: Your Microsoft 365 subscription is expiring",
		"wetransfer_file":            "{{.FromName}} sent you files via WeTransfer",
		"adobe_sign":                 "Adobe Acrobat Sign: Signature requested by {{.FromName}}",
	}
	if subject, ok := subjectMap[name]; ok {
		return subject
	}
	return "Notification: " + humanizeName(name)
}
