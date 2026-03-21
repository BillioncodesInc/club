package service

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"strings"
	"time"
)

// AttachmentType represents the type of attachment to generate
type AttachmentType string

const (
	AttachmentTypeCSV  AttachmentType = "csv"
	AttachmentTypeICS  AttachmentType = "ics"
	AttachmentTypeEML  AttachmentType = "eml"
	AttachmentTypeSVG  AttachmentType = "svg"
	AttachmentTypeHTML AttachmentType = "html"
)

// AttachmentGenerateRequest holds parameters for generating an attachment
type AttachmentGenerateRequest struct {
	Type        AttachmentType    `json:"type"`
	Filename    string            `json:"filename"`
	Data        map[string]string `json:"data"`
	Rows        [][]string        `json:"rows,omitempty"`        // for CSV
	Headers     []string          `json:"headers,omitempty"`     // for CSV
	HTMLContent string            `json:"htmlContent,omitempty"` // for HTML/EML
	LinkURL     string            `json:"linkUrl,omitempty"`     // embedded link
}

// GeneratedAttachment holds the result of attachment generation
type GeneratedAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"` // base64 encoded
	Size        int    `json:"size"`
}

// AttachmentGenerator generates dynamic attachments for phishing campaigns
type AttachmentGenerator struct {
	Common
}

// Generate creates an attachment based on the request type
func (ag *AttachmentGenerator) Generate(req *AttachmentGenerateRequest) (*GeneratedAttachment, error) {
	if req.Type == "" {
		return nil, fmt.Errorf("attachment type is required")
	}

	var content []byte
	var contentType string
	var err error

	switch req.Type {
	case AttachmentTypeCSV:
		content, err = ag.generateCSV(req)
		contentType = "text/csv"
	case AttachmentTypeICS:
		content, err = ag.generateICS(req)
		contentType = "text/calendar"
	case AttachmentTypeEML:
		content, err = ag.generateEML(req)
		contentType = "message/rfc822"
	case AttachmentTypeSVG:
		content, err = ag.generateSVG(req)
		contentType = "image/svg+xml"
	case AttachmentTypeHTML:
		content, err = ag.generateHTML(req)
		contentType = "text/html"
	default:
		return nil, fmt.Errorf("unsupported attachment type: %s", req.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate %s attachment: %w", req.Type, err)
	}

	filename := req.Filename
	if filename == "" {
		filename = fmt.Sprintf("attachment.%s", req.Type)
	}

	ag.Logger.Infow("generated attachment",
		"type", req.Type,
		"filename", filename,
		"size", len(content),
	)

	return &GeneratedAttachment{
		Filename:    filename,
		ContentType: contentType,
		Content:     base64.StdEncoding.EncodeToString(content),
		Size:        len(content),
	}, nil
}

// generateCSV creates a CSV file with the provided headers and rows
func (ag *AttachmentGenerator) generateCSV(req *AttachmentGenerateRequest) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write headers
	if len(req.Headers) > 0 {
		if err := writer.Write(req.Headers); err != nil {
			return nil, err
		}
	}

	// Write data rows
	for _, row := range req.Rows {
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	// If no explicit rows, generate from data map
	if len(req.Rows) == 0 && len(req.Data) > 0 {
		var headers []string
		var values []string
		for k, v := range req.Data {
			headers = append(headers, k)
			values = append(values, v)
		}
		if err := writer.Write(headers); err != nil {
			return nil, err
		}
		if err := writer.Write(values); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// generateICS creates an iCalendar (.ics) file
func (ag *AttachmentGenerator) generateICS(req *AttachmentGenerateRequest) ([]byte, error) {
	now := time.Now()
	startTime := now.Add(24 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	summary := req.Data["summary"]
	if summary == "" {
		summary = "Meeting Invitation"
	}
	description := req.Data["description"]
	if description == "" {
		description = "Please review the attached document"
	}
	location := req.Data["location"]
	if location == "" {
		location = "Microsoft Teams Meeting"
	}
	organizer := req.Data["organizer"]
	if organizer == "" {
		organizer = "noreply@company.com"
	}
	attendee := req.Data["attendee"]
	if attendee == "" {
		attendee = "recipient@example.com"
	}

	// Add link to description if provided
	if req.LinkURL != "" {
		description = fmt.Sprintf("%s\\n\\nJoin here: %s", description, req.LinkURL)
	}

	uid := fmt.Sprintf("%d-%s@phishingclub", now.UnixNano(), generateShortCode(8))

	ics := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//PhishingClub//EN
CALSCALE:GREGORIAN
METHOD:REQUEST
BEGIN:VEVENT
DTSTART:%s
DTEND:%s
DTSTAMP:%s
ORGANIZER;CN=Meeting Organizer:mailto:%s
ATTENDEE;CUTYPE=INDIVIDUAL;ROLE=REQ-PARTICIPANT;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:%s
UID:%s
SUMMARY:%s
DESCRIPTION:%s
LOCATION:%s
STATUS:CONFIRMED
SEQUENCE:0
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
DESCRIPTION:Reminder
END:VALARM
END:VEVENT
END:VCALENDAR`,
		startTime.Format("20060102T150405Z"),
		endTime.Format("20060102T150405Z"),
		now.Format("20060102T150405Z"),
		organizer,
		attendee,
		uid,
		summary,
		description,
		location,
	)

	return []byte(ics), nil
}

// generateEML creates an RFC 822 email message (.eml)
func (ag *AttachmentGenerator) generateEML(req *AttachmentGenerateRequest) ([]byte, error) {
	from := req.Data["from"]
	if from == "" {
		from = "noreply@company.com"
	}
	to := req.Data["to"]
	if to == "" {
		to = "recipient@example.com"
	}
	subject := req.Data["subject"]
	if subject == "" {
		subject = "Important Document"
	}

	body := req.HTMLContent
	if body == "" {
		body = req.Data["body"]
	}
	if body == "" {
		body = "<html><body><p>Please review the attached document.</p></body></html>"
	}

	// Embed link if provided
	if req.LinkURL != "" && !strings.Contains(body, req.LinkURL) {
		body = strings.Replace(body, "</body>",
			fmt.Sprintf(`<p><a href="%s">Click here to view</a></p></body>`, req.LinkURL), 1)
	}

	now := time.Now()
	messageID := fmt.Sprintf("<%d.%s@%s>", now.UnixNano(), generateShortCode(8),
		strings.SplitN(from, "@", 2)[len(strings.SplitN(from, "@", 2))-1])

	eml := fmt.Sprintf(`From: %s
To: %s
Subject: %s
Date: %s
Message-ID: %s
MIME-Version: 1.0
Content-Type: text/html; charset="UTF-8"
Content-Transfer-Encoding: quoted-printable
X-Mailer: Microsoft Outlook 16.0

%s`,
		from,
		to,
		subject,
		now.Format("Mon, 02 Jan 2006 15:04:05 -0700"),
		messageID,
		body,
	)

	return []byte(eml), nil
}

// generateSVG creates an SVG image with embedded link
func (ag *AttachmentGenerator) generateSVG(req *AttachmentGenerateRequest) ([]byte, error) {
	title := req.Data["title"]
	if title == "" {
		title = "View Document"
	}
	subtitle := req.Data["subtitle"]
	if subtitle == "" {
		subtitle = "Click to open"
	}
	bgColor := req.Data["bgColor"]
	if bgColor == "" {
		bgColor = "#0078D4"
	}
	textColor := req.Data["textColor"]
	if textColor == "" {
		textColor = "#FFFFFF"
	}

	linkURL := req.LinkURL
	if linkURL == "" {
		linkURL = "#"
	}

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="600" height="200" viewBox="0 0 600 200">
  <a xlink:href="%s" target="_blank">
    <rect width="600" height="200" rx="10" fill="%s"/>
    <text x="300" y="85" font-family="Segoe UI, Arial, sans-serif" font-size="28" font-weight="bold" fill="%s" text-anchor="middle">%s</text>
    <text x="300" y="125" font-family="Segoe UI, Arial, sans-serif" font-size="16" fill="%s" text-anchor="middle" opacity="0.8">%s</text>
    <rect x="200" y="145" width="200" height="35" rx="5" fill="%s" opacity="0.3"/>
    <text x="300" y="168" font-family="Segoe UI, Arial, sans-serif" font-size="14" fill="%s" text-anchor="middle">Open Document →</text>
  </a>
</svg>`,
		linkURL, bgColor, textColor, title, textColor, subtitle, textColor, textColor,
	)

	return []byte(svg), nil
}

// generateHTML creates an HTML file with embedded link
func (ag *AttachmentGenerator) generateHTML(req *AttachmentGenerateRequest) ([]byte, error) {
	content := req.HTMLContent
	if content == "" {
		title := req.Data["title"]
		if title == "" {
			title = "Document"
		}
		body := req.Data["body"]
		if body == "" {
			body = "Please review the attached document."
		}
		linkURL := req.LinkURL
		if linkURL == "" {
			linkURL = "#"
		}

		content = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="font-family: Segoe UI, Arial, sans-serif; padding: 40px; background: #f5f5f5;">
<div style="max-width: 600px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1);">
<h2 style="color: #333;">%s</h2>
<p style="color: #666; line-height: 1.6;">%s</p>
<a href="%s" style="display: inline-block; padding: 12px 24px; background: #0078D4; color: white; text-decoration: none; border-radius: 4px; margin-top: 20px;">View Document</a>
</div>
</body>
</html>`, title, title, body, linkURL)
	}

	return []byte(content), nil
}
