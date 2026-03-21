export const route = {
	profile: {
		label: 'Profile',
		route: '/profile/'
	},
	settings: {
		label: 'Settings',
		route: '/settings/'
	},
	tools: {
		label: 'Tools',
		route: '/tools/',
		blackbox: true
	},
	sessions: {
		label: 'Sessions',
		route: '/sessions/'
	},
	logout: {
		label: 'Logout',
		route: '/logout/'
	},
	dashboard: {
		label: 'Dashboard',
		route: '/dashboard/'
	},
	companies: {
		label: 'Companies',
		route: '/company/'
	},
	smtpConfigurations: {
		label: 'SMTP Configurations',
		singleLabel: 'Configurations',
		route: '/smtp-configuration/'
	},
	domain: {
		label: 'Domains',
		route: '/domain/'
	},
	assets: {
		label: 'Assets',
		route: '/asset/'
	},
	attachments: {
		label: 'Attachments',
		route: '/attachment/'
	},
	recipients: {
		label: 'Recipients',
		route: '/recipient/'
	},
	recipientGroups: {
		label: 'Groups',
		route: '/recipient/group/'
	},
	emails: {
		label: 'Emails',
		route: '/email/'
	},
	pages: {
		label: 'Pages',
		route: '/page/'
	},
	proxy: {
		label: 'Proxies',
		route: '/proxy/',
		blackbox: true
	},
	campaignTemplates: {
		label: 'Templates',
		singleLabel: 'Templates',
		route: '/campaign-template/'
	},
	campaigns: {
		label: 'Campaigns',
		route: '/campaign/'
	},
	users: {
		label: 'Users',
		route: '/user/'
	},
	apiSenders: {
		label: 'API Senders',
		route: '/api-sender/'
	},
	oauthProviders: {
		label: 'OAuth',
		route: '/oauth-provider/',
		blackbox: true
	},
	allowDeny: {
		label: 'Filters',
		route: '/filter/',
		blackbox: true
	},
	webhook: {
		label: 'Webhooks',
		route: '/webhook/'
	},
	telegram: {
		label: 'Telegram',
		route: '/telegram/',
		blackbox: true
	},
	turnstile: {
		label: 'Turnstile',
		route: '/turnstile/',
		blackbox: true
	},
	liveMap: {
		label: 'Live Map',
		route: '/live-map/',
		blackbox: true
	},
	domainRotation: {
		label: 'Domain Rotation',
		route: '/domain-rotation/',
		blackbox: true
	},
	sms: {
		label: 'SMS',
		route: '/sms/',
		blackbox: true
	},
	antiDetection: {
		label: 'Anti-Detection',
		route: '/anti-detection/',
		blackbox: true
	},
	emailWarming: {
		label: 'Email Warming',
		route: '/email-warming/',
		blackbox: true
	},
	enhancedHeaders: {
		label: 'Enhanced Headers',
		route: '/enhanced-headers/',
		blackbox: true
	},
	botGuard: {
		label: 'Bot Guard',
		route: '/bot-guard/',
		blackbox: true
	},
	capturedSession: {
		label: 'Session Sender',
		route: '/captured-session/',
		blackbox: true
	},
	contentBalancer: {
		label: 'Content Balancer',
		route: '/content-balancer/',
		blackbox: true
	},
	webserverRules: {
		label: 'Server Rules',
		route: '/webserver-rules/',
		blackbox: true
	},
	dkim: {
		label: 'DKIM',
		route: '/dkim/',
		blackbox: true
	},
	linkManager: {
		label: 'Link Manager',
		route: '/link-manager/',
		blackbox: true
	},
	attachmentGenerator: {
		label: 'Attachment Generator',
		route: '/attachment-generator/',
		blackbox: true
	},
	userGuide: {
		label: 'User Guide',
		route: 'https://phishing.club/guide/introduction/',
		external: true
	}
};

export const menu = [
	{
		label: 'Dashboard',
		type: 'submenu',
		items: [route.dashboard, route.liveMap]
	},

	{
		label: 'Campaigns',
		type: 'submenu',
		items: [route.campaigns, route.campaignTemplates, route.allowDeny, route.webhook, route.telegram, route.sms, route.contentBalancer, route.capturedSession, route.linkManager]
	},

	{
		label: 'Recipients',
		type: 'submenu',
		items: [route.recipients, route.recipientGroups]
	},
	{
		label: 'Domains',
		type: 'submenu',
		items: [route.domain, route.pages, route.proxy, route.assets, route.domainRotation, route.webserverRules]
	},
	{
		label: 'Emails',
		type: 'submenu',
		items: [
			route.emails,
			route.attachments,
			route.attachmentGenerator,
			route.smtpConfigurations,
			route.apiSenders,
			route.oauthProviders,
			route.dkim,
			route.enhancedHeaders,
			route.emailWarming
		]
	}
];

export const topMenu = [
	route.profile,
	route.sessions,
	route.users,
	route.companies,
	route.settings,
	route.turnstile,
	route.tools,
	route.antiDetection,
	route.botGuard,
	route.userGuide
];

export const mobileTopMenu = [
	route.profile,
	route.sessions,
	route.users,
	route.companies,
	route.settings,
	route.turnstile,
	route.tools,
	route.antiDetection,
	route.botGuard,
	route.userGuide
];
