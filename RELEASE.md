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
