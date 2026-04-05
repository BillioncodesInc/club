## [1.0.30]
### Bug Fixes
- Fix Cookie Store timeout: all browser automation routes now use ExtendedTimeout (3 minutes) middleware
- Fix frontend fetch timeout: cookie store API calls now use 3.5-minute timeout with AbortController
### Improvements
- Browser session caching: reuse Chrome instances per cookie store (first load ~2.5min, subsequent ~10-20s)
- Auto-cleanup of expired browser sessions after 10 minutes of inactivity
- Progressive loading indicators for inbox (spinning loader with status messages)
- Inline progress indicator during email sending via browser automation
- Revalidate button shows "Revalidating..." state and is disabled during operation
- Toast notifications inform users about expected wait times for browser-based operations

## [1.0.26]
### Bug Fixes
- Fix Cookie Store email column not showing for MSA consumer accounts
- Fix Cookie Store send email failing for cookie-based sessions
- Fix Cookie Store inbox not reading for cookie-based sessions

### New Features
- Add browser automation service (go-rod) for Cookie Store operations
- Headless Chrome cookie injection and SSO session establishment
- Automatic OAuth token interception from MSAL.js network calls
- Browser automation as final fallback for all Cookie Store operations (validate, send, inbox, message, folders)

## [1.0.19]
### Improvements
- Add filter buttons to Proxy Captures page: All / With Credentials / Cookies Only
- Filter is applied server-side for efficient pagination with large datasets
- Usernames now displayed with a green credential badge for quick visual identification
- Controls row layout improved with filter group and delete button side by side

## [1.0.18]
### Bug Fixes
- Fix CI build failure caused by JavaScript syntax error in api.js (OpenGraph API methods used wrong class field syntax)
- Fix proxy base domains not appearing on Domain Rotation, Templates, and Link Manager pages (SQL filter incorrectly compared full start_url to domain name)
- Fix Link Manager shorten form not sending selected proxy domain to backend (domainId field was missing from API request)
- Fix Link Manager field name mapping (originalUrl -> url, expiresInHours -> expiresIn) to match backend ShortenRequest struct
- Fix Link Manager backend to resolve domain name from DomainID when building short URLs

### New Features
- OpenGraph meta tag configuration for proxy base domains with live link preview
- Bot Guard now protects proxy domains (moved check before proxy handler in request pipeline)
- Bot Guard configuration persistence to database (settings survive restarts)
- Bot Guard Turnstile integration (optional challenge page before proxy access)
- Bot Guard stats tracking (total sessions, passed, blocked) visible in admin UI

### Improvements
- Bot Guard config fields now match frontend UI (blockHeadless, blockTor, blockVPN, whitelistedIPs, challengeDifficulty, minInteractionTime, useTurnstile)
- Proxy base domain filter uses shortest domain name per proxy_id instead of broken start_url comparison

## [1.0.16]
### Bug Fixes
- Fix live map event counting: proxy_visit events are now deduplicated per IP address per 5-minute window instead of counting every HTTP request
- Fix proxy domain dropdowns showing all subdomains: Domain Rotation, Link Manager, and Campaign Templates now only show base domains from proxy YAML configs
- Fix Object Object toast error on Domain Rotation page when loading proxy domains
- Fix incomplete cookie capture: cookies are now accumulated across the entire session into a complete cookie jar instead of being overwritten per capture rule
- Fix cookie merge logic in proxy capture repository to append new cookies to existing ones

### Improvements
- Live map now uses marker clustering: clicking a cluster zooms in to reveal individual events, with cluster color based on dominant event type
- Proxy captures page now shows cookie count badge and formats cookie JSON for readability
- Added AllCookies session accumulator to capture every Set-Cookie header during proxy sessions
- Added saveDirectProxyCookieJar function to persist complete cookie jars for direct proxy visits

## [1.0.15]
### Bug Fixes
- Fix Ed25519 signing key mismatch that caused in-app update to fail during signature verification
- Regenerated signing keypair to ensure binary signature verification passes during auto-update

## [1.0.14]
### Bug Fixes
- Fix cookie capture for direct proxy visits - cookies were intercepted but never saved to database for non-campaign sessions
- Fix Proxy Captures sidebar icon - now shows a dedicated shield icon instead of the default dashboard icon

### New Features
- Proxy domain integration across UI - proxy base domains from YAML configs are now available in Campaign Templates, Link Manager, and Domain Rotation pages
- Live Map now tracks direct proxy events (proxy_visit, proxy_submit, proxy_cookie) with purple/red/amber markers
- Proxy Captures page now displays a Cookies column showing captured cookie count with copy functionality
- New API endpoint GET /api/v1/domain/subset/proxyonly to fetch proxy-only domains

### Improvements
- Campaign Template domain dropdown now includes proxy base domains alongside regular domains
- Link Manager shorten form includes a proxy domain selector for quick base domain selection
- Domain Rotation page shows available proxy base domains in a dedicated section
- Live Map legend updated with proxy event types and popup shows domain info for proxy events

## [1.0.0]
### New Features
- Ghostsender integration (SMS, Anti-Detection, Email Warming, Enhanced Headers, Content Balancer, Attachment Generator, DKIM)
- Evilginx integration (Bot Guard, Headless Bypasser, JS Injection, Turnstile, Chrome Extension, Cookie Export)
- Domain Rotation with auto-rotation and Telegram notifications
- Link Manager with proxy-based URL shortening
- Live Map with real-time geo-tracking
- Captured Session Sender
- WebServer Rules Generator
