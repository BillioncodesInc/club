package data

const (
	OptionKeyIsInstalled      = "is_installed"
	OptionValueIsInstalled    = "true"
	OptionValueIsNotInstalled = "false"
	// KeyIsInstalled is the key for the is_installed option
	OptionKeyInstanceID = "instance_id"

	OptionKeyLogLevel   = "log_level"
	OptionKeyDBLogLevel = "db_log_level"

	OptionKeyUsingSystemd      = "systemd_install"
	OptionValueUsingSystemdYes = "true"
	OptionValueUsingSystemdNo  = "false"

	OptionKeyDevelopmentSeeded = "development_seeded"
	OptionValueSeeded          = "true"

	OptionKeyMaxFileUploadSizeMB             = "max_file_upload_size_mb"
	OptionValueKeyMaxFileUploadSizeMBDefault = "100"

	OptionKeyRepeatOffenderMonths = "repeat_offender_months"

	OptionKeyAdminSSOLogin = "sso_login"

	OptionKeyProxyCookieName = "proxy_cookie_name"

	OptionKeyDisplayMode           = "display_mode"
	OptionValueDisplayModeWhitebox = "whitebox"
	OptionValueDisplayModeBlackbox = "blackbox"

	OptionKeyObfuscationTemplate = "obfuscation_template"
	// OptionValueObfuscationTemplateDefault is the default HTML template for obfuscation
	// the template receives {{.Script}} variable containing the obfuscated javascript
	OptionValueObfuscationTemplateDefault = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
<script>{{.Script}}</script>
</body>
</html>`

	// Telegram notifications
	OptionKeyTelegramBotToken  = "telegram_bot_token"
	OptionKeyTelegramChatID    = "telegram_chat_id"
	OptionKeyTelegramEnabled   = "telegram_enabled"
	OptionKeyTelegramDataLevel = "telegram_data_level"
	OptionValueTelegramEnabled  = "true"
	OptionValueTelegramDisabled = "false"

	// Cloudflare Turnstile
	OptionKeyTurnstileEnabled   = "turnstile_enabled"
	OptionKeyTurnstileSiteKey   = "turnstile_site_key"
	OptionKeyTurnstileSecretKey = "turnstile_secret_key"
	OptionValueTurnstileEnabled  = "true"
	OptionValueTurnstileDisabled = "false"

	// BotGuard
	OptionKeyBotGuardEnabled          = "botguard_enabled"
	OptionKeyBotGuardJSChallenge      = "botguard_js_challenge"
	OptionKeyBotGuardBehaviorAnalysis = "botguard_behavior_analysis"
	OptionValueBotGuardEnabled  = "true"
	OptionValueBotGuardDisabled = "false"
	OptionKeyBotGuardConfig     = "botguard_config"

	// SMS sending
	OptionKeySMSProvider      = "sms_provider"
	OptionKeySMSTwilioSID     = "sms_twilio_sid"
	OptionKeySMSTwilioToken   = "sms_twilio_token"
	OptionKeySMSTwilioFrom    = "sms_twilio_from"
	OptionKeySMSTextBeeKey    = "sms_textbee_key"
	OptionKeySMSTextBeeDevice = "sms_textbee_device"

	// Domain rotation
	OptionKeyDomainRotationEnabled  = "domain_rotation_enabled"
	OptionKeyDomainRotationInterval = "domain_rotation_interval"
	OptionKeyDomainRotationDomains  = "domain_rotation_domains"

	// Email warming
	OptionKeyEmailWarmingEnabled = "email_warming_enabled"

	// Enhanced headers
	OptionKeyEnhancedHeadersEnabled = "enhanced_headers_enabled"

	// Content balancer
	OptionKeyContentBalancerEnabled = "content_balancer_enabled"

	// JS injection rules (JSON blob)
	OptionKeyJsInjectRules = "js_inject_rules"

	// Domain rotator configuration (JSON blob)
	OptionKeyDomainRotatorConfig = "domain_rotator_config"

	// Turnstile configuration (JSON blob)
	OptionKeyTurnstileConfig = "turnstile_config"

	// Google Safe Browsing API key for domain reputation checks
	OptionKeyGSBApiKey = "gsb_api_key"
)
