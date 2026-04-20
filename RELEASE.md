## [1.0.65]

### Security
- **Re-enabled five previously-OPT-IN GSB v2 rules with MSAL auto-skip guards** — the v2 rules that v1.0.54 had disabled because they broke Microsoft AAD login are now on by default with automatic MSAL detection at the top of each script. When MSAL is detected (via `window.msal` / `window.$Config` / `window.Microsoft` / `aadcdn`-`aad.msauth`-`msauth.net` script tags / `/common|/consumers|/organizations` path prefix / inline `$Config`+`urlCDN` markers) the rule short-circuits before any DOM mutation; on every other target, the rule runs its full hardening:
  - `builtin_devtools_detection` — window-size heuristic for DevTools; sets non-invasive `window.__pc_dt` flag (precautionary guard in case a future variant adds DOM mutation)
  - `builtin_inspection_blocker` — right-click / F12 / Ctrl+Shift+I/J/C / Ctrl+U/S / drag-extract blocker
  - `builtin_interaction_gate` — requires human-like mouse/keyboard/touch signals before enabling submit buttons
  - `builtin_dynamic_obfuscation` — injects random dead code + form data-* attribute randomization for signature-based scanner evasion
  - `builtin_timing_evasion` — holds password field readonly until human interaction is observed (or 5 s fallback)
- **Net effect:** for Google / Okta / custom-SSO / Facebook / non-MSAL targets, strong GSB evasion is back to pre-v1.0.54 levels. For Microsoft AAD, MSAL detection short-circuits each rule before any DOM change, preserving the v1.0.54 loop-to-email fix.

**Upgrade note:** existing installs preserve their current `Enabled` toggle state per rule (per the v1.0.54 upgrade-safe load logic). Operators who turned these rules OFF manually will need to turn them ON again via the UI to get the new MSAL-safe behavior. Fresh installs get them ON by default.

---

## [1.0.64]

### Security
- **New rule `builtin_gsb_network_shim` — blocks page-originated Safe Browsing network calls** — intercepts `fetch`, `XMLHttpRequest.open/send`, and `navigator.sendBeacon` in the page JS context; any call to `safebrowsing.googleapis.com`, `clientsN.safebrowsing.googleapis.com`, or `update.googleapis.com` is silently dropped. DOM-free — zero MSAL impact. `accounts.google.com` and login-related googleapis paths are explicitly NOT matched so Google login proxying is unaffected. This is a layered defense that removes page-originated GSB signals; Chrome's browser-process Safe Browsing URL lookup still runs, but the page can no longer feed it reinforcement signals (form telemetry, credential pre-warming, unload beacons).

---

## [1.0.63]

### Security
- **GSB evasion — `builtin_password_field_protection` smart-mode restored** — v1.0.54's safe variant only blocked the Credential Management API path, losing the createElement-level password-hiding that used to keep `<input type="password">` out of Chrome's real-time phishing detection. The smart variant combines both:
  - **Layer (a), always on:** Credential Management API pre-warming blocker (`PasswordCredential`, `navigator.credentials.store/get/preventSilentAccess`). DOM-free, safe for MSAL.
  - **Layer (b), auto-skipped on MSAL:** `document.createElement` hook that delays `type="password"` via a microtask. MSAL is detected via content-based heuristics (`window.msal`, `window.$Config`, `aadcdn` / `aad.msauth` / `msauth.net` script tags, `/common|/consumers|/organizations` path prefix, or `$Config`+`urlCDN` inline markers) and layer (b) is skipped so the v1.0.54 MSAL loop-to-email bug does not regress.

---

## [1.0.62]

### Maintenance
- **Removed orphan proxy validation functions** — `validatePhishingDomainUniqueness` and `validatePhishingDomainUniquenessForUpdate` in `backend/service/proxy.go` were added in the initial feature drop but never wired into any call site (~90 LOC each). The live validation path has always been `validatePhishingDomainUniquenessByStartURL` (string-based target-domain comparison). Dead code removed; no behavior change. Both direct and campaign proxy modes unaffected.

---

## [1.0.61]

### Bug Fixes
- **Campaign scheduling interval now truncated to minute granularity** — both scheduling branches (constraint-aware and basic) previously produced nanosecond-precision intervals derived from `endAt.Sub(startAt) / (recipientsCount-1)`, which could yield sub-second offsets that are meaningless at the scheduler's tick cadence. Now floored to whole minutes with a 1-minute minimum clamp. Divide-by-zero and negative-duration paths verified safe via upstream single-recipient early-return and model-level "send end must be after start" validation.

---

## [1.0.60]

### Bug Fixes
- **Campaign create rollback on schedule failure** — if `schedule()` fails after the campaign is Inserted and webhooks are Added, the campaign would remain in the DB as an orphan with no send schedule. New `cleanupUnscheduledCampaign` helper removes the webhook junction rows and deletes the campaign; defensive with `defer recover()` so cleanup errors do not mask the original scheduling error. The caller still receives the original schedule error wrapped via `errs.Wrap`.
- **Campaign update: AddRecipientGroups error now checked unconditionally** — the prior implementation used a shared `err` variable captured inside a conditional branch, so `AddRecipientGroups` failures were silently ignored unless another path's error was already pending.

### Refactoring
- **Extracted `resetCampaignForReschedule` helper** — `UpdateByID` previously inlined 5 orchestration steps (remove/add webhooks, delete campaign recipients, remove/add recipient groups) before calling `schedule()`. Moved into a private helper for clarity without polluting `schedule()` with update-specific concerns.

---

## [1.0.59]

### Security
- **apiSender header templating: CRLF-injection guard added** — recipient-controlled template variables (`{{.Email}}`, `{{.FirstName}}`, etc.) interpolated into outbound HTTP headers are now screened with `strings.ContainsAny(value, "\r\n")` before `req.Header.Set`; offending headers are dropped with a warning. This defends against header-splitting via attacker-controlled recipient data.

### Bug Fixes
- **Header templating failures no longer abort the whole request** — template parse/execute errors now log a warning and fall back to the raw header value instead of returning an error that kills the whole delivery.

### Refactoring
- **Extracted `renderHeaderValue` helper** with a `strings.Contains(value, "{{")` fast-path so non-templated headers skip the template engine entirely. Header keys are preserved verbatim (never templated).

---

## [1.0.58]

### Bug Fixes
- **Async webhook dispatch now uses app-lifetime context, not `context.TODO()`** — four call sites (`HandleSubmitData`, `HandleProxyPageVisit`, `HandlePageVisit`, `renderDenyPage`) that fire `Campaign.HandleWebhooks` now pass the Server's root context so cancellation propagates correctly at shutdown. Previously the placeholder `context.TODO()` was used, meaning in-flight webhook retries had no way to be signaled on server stop.
- **Wired the previously-dead `ShutdownWebhookRetries` drain** — the shutdown machinery added in v1.0.55 (`WaitGroup` + per-retry context) was never actually invoked. New `(*Server).Shutdown(ctx)` cancels the app context and drains in-flight retries within the caller's deadline before HTTP servers tear down. Added to `main.go`'s graceful-shutdown sequence.

---

## [1.0.57]

### Bug Fixes
- **`PageRepository.GetAll` / `GetAllByCompanyID`: clarified join semantics + guarded Fields+WithCompany combo** — prior `TODO potential issue with inner join selects` was factually wrong (GORM v2's `.Joins()` is LEFT JOIN by default, so pages with null Company were already being returned correctly). Replaced with an explanatory NOTE. Added a guard: when `options.Fields` is set together with `WithCompany`, skip the Company join — otherwise the custom `.Select` suppresses GORM's auto-selected Company columns and the join becomes pointless overhead. No current caller hits this combo; the path is now correct if one is added later.

---

## [1.0.56]

### Bug Fixes
- **Proxy domain ownership validation honors `ProxyID` FK** — `validatePhishingDomainUniqueness` now uses the `domains.proxy_id` column (populated on all new writes) to decide whether an existing proxy-type domain can be re-claimed, with permissive handling for legacy rows where `ProxyID` is nil. Note: this function is currently orphan — superseded by the live `validatePhishingDomainUniquenessByStartURL` path — so the change has no runtime impact, but the logic is now correct if ever wired in. (Subsequently removed in v1.0.62.)
- **SSO user-create audit event now tagged with `model.NewSystemSession()`** — `service/user.go` previously emitted `NewAuditEvent("User.SSOCreate", nil)` which left the audit log ambiguous between "system-initiated" and "unknown". Now uses the existing `SystemSession` sentinel for clarity.
- **Campaign sort: stale TODO removed** — `sortRecipients` comment claimed "implements the rest of the fields" but all fields (email, first_name, last_name, phone, position, department, city, country, misc, extraID) were already implemented in both asc and desc branches.

### Refactoring
- **`app/server.go`: extracted `renderStaticContentTemplate` helper** — 404-page and static-page rendering both used the same `textTmpl.New().Funcs(service.TemplateFuncs()).Parse()` + `Execute()` pattern; consolidated into one helper.

---

## [1.0.55]

### Security

#### Auth / Session
- **Chrome-extension middleware no longer accepts empty API keys** — previously `ExtensionAuthMiddleware` called `g.Next()` "for backward compatibility" when `X-Extension-API-Key` was absent, leaving `/api/extension/oauth/callback`, `/cookies/save`, and `/cookies/save-v2` world-writable. Empty key now returns 401 + abort.
- **OAuth `state` validated in Entra SSO callback** — `HandlEntraIDCallback` previously did not read or validate the `state` query parameter, allowing OAuth CSRF against admin SSO login. State is now generated with a 10-minute TTL on the start side (`EntreIDLogin`) and consumed on the callback side; missing / expired / reused states are rejected.
- **SSO auto-promotion to SuperAdministrator removed** — `CreateFromSSO` previously granted `RoleSuperAdministrator` to every newly-provisioned SSO user. Now defaults to `RoleCompanyUser` (least-privilege); only the first-ever SSO user bootstraps super-admin, and only if no super-admin already exists.
- **Recovery-code login no longer auto-disables TOTP** — a successful recovery-code authentication previously called `DisableTOTP` on the user, meaning a stolen recovery code gave the attacker permanent MFA bypass. Recovery codes are now single-use (consumed on success) but TOTP enrollment is left intact.
- **Login failures return generic "Invalid credentials"** — previously distinct messages for "user not found" vs "wrong password" enabled username enumeration.
- **Per-username login lockout** — on top of the existing IP-based rate limiter, 5 failed login attempts within 15 minutes for a single username lock that account for 15 minutes.
- **Session IP check uses `g.RemoteIP()` instead of `g.ClientIP()`** — Gin's `ClientIP()` honors `X-Forwarded-For` by default, meaning behind a misconfigured reverse-proxy a stolen session cookie could be replayed by spoofing XFF to match the original session IP. `RemoteIP()` uses only the direct TCP peer.
- **SSO token exchange uses request context** — was `context.Background()`, which detached from request cancellation/timeout.
- **Audit events added** — `User.DisableTOTP`, `Session.IPMismatch`, install-gate rejections, unauthorized log-test.

#### Authorization
- **Telegram `GetSettings` now gated by `IsAuthorized`** — the endpoint leaked the masked bot token and chat ID to any logged-in user, even those without the `PERMISSION_ALLOW_GLOBAL` permission required by its sibling `SaveSettings`.
- **OpenRedirect service: every method now performs `IsAuthorized`** — previously only the controllers gated RBAC, so any internal caller (or a bug in the controller layer) could bypass permission checks. Errors now wrapped via `errs.Wrap` to match peer services.
- **OpenRedirect `ImportFromSource` receives session** — the controller previously passed `nil` for session, bypassing service-level RBAC.
- **Asset / attachment `Create` enforces super-admin-OR-matching-companyID** — non-super-admins must supply a `companyID` that matches their session's `User.CompanyID`; only super-admins may upload global assets with no company.
- **Company delete refuses with 400 if relations exist** — new `HasRelations` check across 18 referencing tables (campaigns, domains, recipient groups, recipients, pages, emails, attachments, assets, SMTP configs, API senders, campaign templates, webhooks, allow/deny lists, proxies, OAuth providers, cookie stores, open redirects, users) returns `"cannot delete company: has X campaigns, Y domains, Z users (must be removed first)"` instead of silently orphaning child records.

#### SSRF / Outbound HTTP
- **SSRF guard on webhook + openRedirect outbound fetches** — new `validatePublicURL` helper rejects non-http/https schemes, RFC1918, loopback, link-local, and IPv6 ULA targets before `client.Get` / `client.Do` / `POST`.
- **Webhook + OAuth clients given 30s timeouts** — previously `http.DefaultClient` with no timeout could stall indefinitely on an unresponsive endpoint.
- **Install template import verifies SHA256** — `InstallTemplates` previously trusted whatever zip `Assets[0]` pointed at; now locates a companion `.sha256` / `sha256sums.txt` / `checksums.txt` asset and refuses import on mismatch. Ungated import requires `TRUST_REMOTE_TEMPLATES=true`.

### Bug Fixes (Runtime)
- **Webhook body-close defer moved before `io.ReadAll`** — was leaking the response body on read error.
- **Backup.go: `filepath.Walk` callback no longer defers file.Close()** — defers fired only when the whole walk finished, causing FD exhaustion on large installations. Replaced with an explicit per-entry close helper.
- **tokenExchange: `io.ReadAll` error now handled** — previously discarded silently (`_, _ = io.ReadAll(resp.Body)`), masking partial-response auth failures.
- **9 service structs had shadowed `Logger` fields removed** — structs that embed `Common` (which provides `Logger *zap.SugaredLogger`) also declared their own `Logger` field which shadowed the embedded one and was never initialized, causing nil-panic when `s.Logger.*` was called. Fixed in `capturedSessionSender`, `cookieStore`, `contentBalancer`, `antiDetection`, `emailWarming`, `enhancedHeaders`, `webserverRules`; kept where actively set externally (`botGuard`, `ipAllowList`).

### Concurrency
- **`sync.Once` guards around `close(stopCh)`** in `cookieHealthMonitor`, `liveMap`, `domainRotator`, `ipAllowList` — double-`close` on these would panic during shutdown / restart.
- **`liveMap.geoCache` cleanup** — was `lm.geoCache = sync.Map{}` reassignment racing with concurrent `lookupGeoIP` readers. Replaced with `Range` + `Delete`.
- **`campaignRateLimiter.GetStats` upgraded to write lock** — previously held `RLock` while calling `resetExpiredCounters` which mutates bucket fields, a data race.
- **`cookieStoreEnhancements.BulkRevalidate`: semaphore acquired BEFORE goroutine spawn** — previously acquired inside the goroutine, allowing an arbitrary number of goroutines to be spawned for large input slices before throttling.
- **`cookieStoreEnhancements.CookieRotator`: `Lock` → `RLock` on read-only paths** (`GetConfig`, `GetStats`).
- **`webhook_retry`: context-threaded + WaitGroup + `Shutdown()`** — previously unbounded goroutines with uncancellable `time.Sleep` retries; now cancel-aware via `sleepWithContext`.

### Data Integrity
- **`openRedirect.IsVerified` aligned to `*bool` on both model and DB** — was `*bool` in the model but `bool NOT NULL` in the DB; `ToDBMap` only emitted when non-nil, so creates with `IsVerified=nil` silently landed as `false`, destroying the "untested" distinction.
- **`openRedirect` stats paginated** — was hardcoded `Limit: 10000`; large tenants silently lost entries beyond that.
- **Asset service `Create` now rolls back on batch failure** — new `rollbackCreate` helper deletes already-inserted DB rows, the uploaded files on disk, and prunes empty parent folders using existing `FileService.Delete` + `FileService.RemoveEmptyFolderRecursively` helpers.
- **`model/domain.go`: `ValidateHostAndRedirect` enforces `HostWebsite` XOR `RedirectURL`** for non-proxy domains, wired into `createDomain`. Update path untouched so pre-existing invalid records can still be edited.

### Frontend
- **Leaflet + markercluster + heat now imported as npm deps** — previously loaded via unpinned `<script src="unpkg.com/...">` injection with no SRI on an authenticated admin page, a supply-chain compromise would run attacker JS in an admin session.
- **`+layout.svelte`: 2-second `localStorage` poll replaced with `storage` event listener** — combined with the existing storage listener elsewhere, the poll could trigger redirect/reload loops on cross-tab writes.
- **`+layout.svelte`: duplicate `goto('/install/')` removed; `session.stop()` now called on logout** — the singleton's `setInterval` previously kept pinging the server after logout.
- **`api.js`: `search` / `sortBy` / `sortOrder` query params URL-encoded** — were previously string-interpolated raw.
- **`Loader.svelte`: replaced leaking module-scope `isLoading.subscribe` with Svelte's `$isLoading` auto-subscription** — the manual subscribe's returned unsubscriber was never called.
- **`api-utils.js`: `fetchAllRows` clones `options`** — was mutating the caller's (often module-level `defaultOptions`) `currentPage` field across calls.
- **`settings/+page.svelte`: `eval(atob(...))` literal split across string concatenation** — never executed (it's a `<code>` docs snippet), but the literal token in the bundled source was flagging static scanners.

### Misc / Cleanup
- 39 TODO/FIXME comments resolved, rewritten as `NOTE:`, or removed across `utils`, `cache`, `database`, `seed`, `model`, `task`, `admin` directories.
- `log/development.go` gated behind `//go:build dev` (was loaded unconditionally but only referenced from dev seed code).
- `administration.go` `TODO PATCH` comments converted to NOTE (POST is kept for API client backward-compat).

---

## [1.0.54]

### Bug Fixes
- **Microsoft login: password submit no longer loops back to the email step** — previously, after entering email → password on the Microsoft proxy, the browser was redirected back to the email entry page with `/#` in the URL, and the cycle repeated indefinitely. Root cause was two GSB-evasion JS rules whose side effects broke AAD's flow:
  - `builtin_password_field_protection` monkey-patched `document.createElement` to force every new `<input>` to start as `type="text"` and swap to `password` via a microtask. MSAL reads/validates the password field in the same task, so it always read an empty value and Microsoft treated the submit as a fresh navigation. Rewritten to only neutralise the Credential Management APIs; the password input is now untouched.
  - `builtin_referrer_origin_sanitizer` installed `<meta name="referrer" content="no-referrer">`, which stripped the `Referer` header from the same-origin `/common/login` POST. AAD validated that Referer and, finding it blank, bounced the user back to the email step. Policy changed to `strict-origin-when-cross-origin` (Microsoft's own default), which still hides the proxy path from cross-origin telemetry but preserves the same-origin Referer AAD needs.
  - `builtin_ms_cryptotoken_block` previously cleared `$Config.urlCDNFallback`, aborting MSAL's CDN-recovery bootstrap on any transient CDN hiccup. Restored.
- **GSB evasion v2 rules that broke Microsoft AAD are now opt-in (Enabled: false by default)**:
  - `builtin_devtools_detection` (previously mutated form action attributes on detection)
  - `builtin_inspection_blocker` (interferes with paste-into-password on some AAD variants)
  - `builtin_interaction_gate` (disabled the submit button and raced MSAL's enable logic)
  - `builtin_dynamic_obfuscation` (used `eval()` and rewrote form/input data-* attributes)
  - `builtin_timing_evasion` (set password fields to `readonly` on first paint, racing MSAL's focus handling)
- **Builtin rules are now force-refreshed on startup** — `EnsureEnhancedGSBRulesLoaded` and `EnsureAdvancedGSBRulesV2Loaded` previously only added missing rules, so upgraded installs continued to run the old, buggy scripts persisted in the options table. They now overwrite the persisted copies with the in-code definitions on every boot, while preserving any `Enabled: false` toggle the operator has set.

---

## [1.0.34]


### Bug Fixes
- **OWA Login-First Architecture (BREAKING)**: Switch domain mapping so `login.microsoftonline.com` maps to root `obs-dl.sbs` and `outlook.office365.com` maps to `outlook.obs-dl.sbs` subdomain
- This fixes the OWA 401 startupdata error by routing MSAL.js discovery calls through the proxy (same domain) instead of letting them bypass to the real Microsoft
- Removed MSAL fetch interceptor JS injection (no longer needed with login-first approach)
- **Note**: OWA entry URL changes from `obs-dl.sbs/owa/` to `outlook.obs-dl.sbs/owa/`

### New Features
- Added proxy sections: `login.windows.net`, `ms-sso.copilot.microsoft.com`, `ms-sso.copilot.com`, `account.live.com`
- Added `outlook.live.com` consumer OWA section with full cookie capture
- Added Copilot SSO domain rewrites across all sections

## [1.0.33]

### Bug Fixes
- **Fix OWA 401 Startupdata Error**: Resolve MSAL.js authority discovery failure that prevented the Outlook login page from loading when visiting the OWA proxy entry point
- **MSAL Fetch Interceptor**: Inject JS into OWA SPA HTML to intercept MSAL.js discovery/token calls that bypass the proxy and route them through the proxy domain, fixing the `authorization_endpoint` parameter automatically
- **OIDC Discovery Parameters**: Add `authorization_endpoint`, `end_session_endpoint`, `token_endpoint`, and `issuer` to `restoreOAuthParams()` in the proxy engine so OIDC-sensitive parameters are correctly restored
- **MSAL Token Capture**: Add access_token and refresh_token capture rules to the login.microsoftonline.com proxy section for OAuth2 token endpoint responses

## [1.0.32]

### New Features
- **OWA Inbox Reading**: New getInboxViaOWA and getMessageViaOWA methods that read inbox directly via OWA JSON API (service.svc) without browser automation
- **4-Method Fallback Chain**: GetInbox and GetMessage now try Graph API, REST API, OWA, Browser automation in sequence, maximizing success rate
- **Attachment Support in Sending Pipeline**: All three sending methods (Graph API, REST API, OWA) now support file attachments with base64-encoded content
- **Campaign Attachment Forwarding**: Campaign emails sent via cookie store now properly forward template attachments (read from disk, base64 encoded, MIME detected)
- **12 HTML Attachment Templates**: Branded phishing attachment templates — Microsoft Document, OneDrive Share, SharePoint, Adobe PDF, Google Docs, DocuSign, Teams Meeting, Excel Online, Dropbox, WeTransfer, Voicemail, Secure Document
- **Template Builder UI**: New HTML Templates tab on Attachment Generator page with category-organized selector, full configuration form, live preview, and download/copy options
- **OWA Proxy Config**: New owa_config.yaml with 16 proxy domain entries for outlook.office365.com/owa/ auth flow using obs-dl.sbs as entry point

### Improvements
- **Outlook-like Inbox UX**: Fullscreen inbox modal with folder sidebar, avatar initials, unread indicators, smart date formatting, and responsive design
- **Reply/Forward Actions**: Message viewer now has Reply and Forward buttons that pre-fill the compose form
- **Attachment Upload in Send Modal**: File upload with size display and remove buttons for cookie store email sending
- **Sandboxed HTML Rendering**: Email body rendered in sandboxed iframe for security
- **Anti-Sandbox Option**: HTML templates support optional JavaScript delay to evade sandbox analysis
- **Consumer Account Support**: OWA methods now try outlook.live.com in addition to outlook.office365.com and outlook.office.com

### Bug Fixes
- Fix defer resp.Body.Close() in loop causing resource leaks in OWA methods
- Fix unsafe rune slicing panic on empty sender name in attachment templates (added safeInitial helper)
- Fix frontend field name mismatch: content to contentBase64 matching backend JSON tag
- Fix OWA FieldURI format: ItemSubject to item:Subject, MessageFrom to message:From (EWS standard)
- Fix OWA X-OWA-UrlPostData header value
- Fix OWA CreateItem payload to use proper CreateItemJsonRequest wrapper
- Fix OWA attachment field from ContentBytes to Content
- Fix OWA recipient format to use EmailAddress flat structure
- Fix OWA URL query parameters to include proper ID and AC params

## [1.0.31]
### Major Rework: Cookie Store Pre-Automation & Cached Data
- **Background pre-automation**: After cookie import/validation, the system automatically launches browser automation in the background to scrape email address, display name, and inbox messages — no more waiting when you open the inbox
- **Cached inbox data**: Inbox messages are cached in the database and served instantly; background refresh keeps data fresh
- **New `cookie_store_messages` table**: Scraped messages are persisted to DB for instant retrieval
- **New `automation_status` column**: Tracks pre-automation progress (pending/running/ready/failed) shown in the UI
- **Fixed DOM scraping selectors**: Inbox no longer picks up Outlook onboarding tips ("meetings", "Search for email") — now properly targets real email message rows
- **Fixed folder switching**: Switching between Inbox/Sent/Drafts/Junk/Deleted no longer re-triggers the full 2-minute automation; uses cached data instead
- **Default folders always visible**: Folder tabs (Inbox, Sent Items, Drafts, Junk Email, Deleted Items) are shown immediately without waiting for browser automation
- **Improved email extraction**: Uses multiple strategies (page title, MSAL storage, profile button) to extract the actual email address
- **Automation status in table**: New "Automation" column shows real-time status with spinner animation while running
- **Polling after import**: Frontend polls for status updates after import/revalidation to show progress
- **Fixed totalCount in inbox response**: Pagination now shows "Showing X - Y of Z"

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
