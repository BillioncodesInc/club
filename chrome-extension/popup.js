/**
 * Phishing Club — Cookie Capture Extension (Popup Script)
 * v1.0.43 — API key auth, Google Workspace, multi-account
 */

let isConnected = false;
let currentServerUrl = '';
let currentApiKey = '';
let activeProvider = 'microsoft';
let latestCapture = null;

// ── Safe message sender ────────────────────────────────────────────────────

function safeSendMessage(msg, callback) {
  try {
    chrome.runtime.sendMessage(msg, (resp) => {
      if (chrome.runtime.lastError) return;
      if (callback) callback(resp);
    });
  } catch (e) {
    // Extension context invalidated
  }
}

// ── Init ────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  loadSettings();
  loadCapturedCookies();
  loadAccounts();
  setupListeners();
});

// ── Load saved settings ─────────────────────────────────────────────────────

function loadSettings() {
  safeSendMessage({ action: 'getSettings' }, (resp) => {
    if (!resp) return;
    document.getElementById('enabledToggle').checked = resp.enabled !== false;
    document.getElementById('autoCaptureToggle').checked = resp.autoCapture !== false;
    document.getElementById('autoCaptureOnLoginToggle').checked = resp.autoCaptureOnLogin !== false;
    document.getElementById('notificationsToggle').checked = resp.notifications !== false;
    document.getElementById('serverUrl').value = resp.serverUrl || '';
    document.getElementById('apiKey').value = resp.apiKey || '';
    currentServerUrl = resp.serverUrl || '';
    currentApiKey = resp.apiKey || '';
    activeProvider = resp.activeProvider || 'microsoft';
    isConnected = resp.connected || false;
    updateConnectButton();
    updateProviderUI();
  });
}

// ── Provider UI ─────────────────────────────────────────────────────────────

function updateProviderUI() {
  // Update tab active state
  document.querySelectorAll('.provider-tab').forEach(tab => {
    tab.classList.toggle('active', tab.dataset.provider === activeProvider);
  });

  // Update capture button text and description
  const captureBtn = document.getElementById('captureBtn');
  const captureDesc = document.getElementById('captureDesc');

  if (activeProvider === 'google') {
    captureBtn.textContent = 'Capture Google Workspace Cookies';
    captureDesc.textContent = 'Sign into Gmail/Google Workspace in this browser first, then capture the session cookies. Cookies from all Google domains will be collected.';
  } else {
    captureBtn.textContent = 'Capture Microsoft Cookies';
    captureDesc.textContent = 'Sign into Outlook in this browser first, then capture the session cookies. Cookies from all Microsoft/Outlook domains will be collected.';
  }
}

// ── Load captured cookies ───────────────────────────────────────────────────

function loadCapturedCookies() {
  safeSendMessage({ action: 'getCapturedCookies' }, (resp) => {
    if (!resp) return;
    const captures = resp.cookies || [];
    renderHistory(captures);

    if (captures.length > 0) {
      latestCapture = captures[0];
      showCaptureResult(latestCapture);
    }
  });
}

function showCaptureResult(capture) {
  const el = document.getElementById('captureResult');
  el.style.display = 'block';

  document.getElementById('cookieCount').textContent = capture.totalCount;
  document.getElementById('criticalCount').textContent = capture.criticalCount || 0;
  document.getElementById('domainCount').textContent = Object.keys(capture.domainGroups || {}).length;

  const authEl = document.getElementById('authStatus');
  if (capture.hasCriticalAuth) {
    authEl.className = 'auth-status ok';
    authEl.textContent = 'Auth tokens found - session should be valid';
  } else {
    authEl.className = 'auth-status warn';
    authEl.textContent = 'Warning: No auth tokens found - sign in first';
  }

  const tagsEl = document.getElementById('domainTags');
  tagsEl.innerHTML = '';
  for (const domain of Object.keys(capture.domainGroups || {})) {
    const tag = document.createElement('span');
    tag.className = 'domain-tag';
    tag.textContent = domain;
    tagsEl.appendChild(tag);
  }

  const sendBtn = document.getElementById('sendCookiesBtn');
  if (capture.sent) {
    sendBtn.textContent = 'Sent to Server';
    sendBtn.style.background = '#059669';
    sendBtn.disabled = false;
  } else {
    sendBtn.textContent = 'Send to Server';
    sendBtn.style.background = '#059669';
    sendBtn.disabled = !currentServerUrl;
  }
}

function renderHistory(captures) {
  const list = document.getElementById('historyList');

  if (captures.length === 0) {
    list.innerHTML = '<div class="no-items">No captures yet.</div>';
    return;
  }

  list.innerHTML = '';
  captures.forEach((capture, i) => {
    const el = document.createElement('div');
    el.className = `history-item ${capture.sent ? 'sent' : 'unsent'}`;

    const time = new Date(capture.timestamp).toLocaleString();
    const domains = Object.keys(capture.domainGroups || {}).length;
    const statusClass = capture.sent ? 'ok' : 'pending';
    const statusText = capture.sent ? 'Sent' : 'Not sent';
    const providerLabel = capture.provider === 'google' ? 'Google' : 'Microsoft';

    el.innerHTML = `
      <div class="history-time">${time}</div>
      <div class="history-info">
        <span class="history-provider">${providerLabel}</span>
        <span>${capture.totalCount} cookies from ${domains} domains</span>
        <span class="history-status ${statusClass}">${statusText}</span>
      </div>
      ${!capture.sent && currentServerUrl ? `
        <div class="history-actions">
          <button class="btn btn-sm btn-send" data-send="${i}">Send</button>
        </div>
      ` : ''}
    `;
    list.appendChild(el);
  });

  list.querySelectorAll('[data-send]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.send);
      btn.textContent = 'Sending...';
      btn.disabled = true;
      safeSendMessage({
        action: 'sendCookies',
        serverUrl: currentServerUrl,
        cookieData: captures[idx],
      }, (resp) => {
        if (resp && resp.success) {
          btn.textContent = 'Sent!';
          loadCapturedCookies();
        } else {
          btn.textContent = 'Failed';
          btn.disabled = false;
          setTimeout(() => { btn.textContent = 'Send'; }, 2000);
        }
      });
    });
  });
}

// ── Accounts ────────────────────────────────────────────────────────────────

function loadAccounts() {
  safeSendMessage({ action: 'getAccounts' }, (resp) => {
    if (!resp) return;
    renderAccounts(resp.accounts || []);
  });
}

function renderAccounts(accounts) {
  const list = document.getElementById('accountsList');

  if (accounts.length === 0) {
    list.innerHTML = '<div class="no-items">No accounts configured. Add one to label your captures.</div>';
    return;
  }

  list.innerHTML = '';
  accounts.forEach(account => {
    const el = document.createElement('div');
    el.className = `account-item ${account.active ? 'active' : ''}`;
    const providerLabel = account.provider === 'google' ? 'Google' : 'Microsoft';

    el.innerHTML = `
      <div class="account-info">
        <span class="account-name">${account.name}</span>
        <span class="account-provider">${providerLabel}</span>
        ${account.active ? '<span class="account-badge">Active</span>' : ''}
      </div>
      <div class="account-actions">
        ${!account.active ? `<button class="btn btn-sm" data-activate="${account.id}" data-provider="${account.provider}">Set Active</button>` : ''}
        <button class="btn btn-sm btn-danger" data-remove="${account.id}">Remove</button>
      </div>
    `;
    list.appendChild(el);
  });

  // Activate buttons
  list.querySelectorAll('[data-activate]').forEach(btn => {
    btn.addEventListener('click', () => {
      safeSendMessage({
        action: 'setActiveAccount',
        accountId: btn.dataset.activate,
        provider: btn.dataset.provider,
      }, () => loadAccounts());
    });
  });

  // Remove buttons
  list.querySelectorAll('[data-remove]').forEach(btn => {
    btn.addEventListener('click', () => {
      safeSendMessage({
        action: 'removeAccount',
        accountId: btn.dataset.remove,
      }, () => loadAccounts());
    });
  });
}

// ── Update Connect button state ─────────────────────────────────────────────

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

// ── Event listeners ─────────────────────────────────────────────────────────

function setupListeners() {
  // Connect button
  document.getElementById('connectBtn').addEventListener('click', () => {
    const url = document.getElementById('serverUrl').value.trim();
    const apiKey = document.getElementById('apiKey').value.trim();

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

    // Save URL and API key
    safeSendMessage({ action: 'updateSettings', settings: { serverUrl: url, apiKey: apiKey } });
    currentServerUrl = url;
    currentApiKey = apiKey;

    // Test connection
    safeSendMessage({ action: 'testConnection', serverUrl: url, apiKey: apiKey }, (resp) => {
      btn.disabled = false;
      if (resp && resp.success) {
        isConnected = true;
        const versionInfo = resp.version ? ` (v${resp.version})` : '';
        const authInfo = apiKey ? ' [Authenticated]' : ' [No API Key]';
        setStatus(`Connected to server${versionInfo}${authInfo}`, 'ok');
      } else {
        isConnected = false;
        setStatus(`${resp?.error || 'Could not reach server'}`, 'err');
      }
      updateConnectButton();
    });
  });

  // Toggle API key visibility
  document.getElementById('toggleApiKeyBtn').addEventListener('click', () => {
    const input = document.getElementById('apiKey');
    input.type = input.type === 'password' ? 'text' : 'password';
  });

  // Provider tabs
  document.querySelectorAll('.provider-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      activeProvider = tab.dataset.provider;
      safeSendMessage({ action: 'updateSettings', settings: { activeProvider } });
      updateProviderUI();
    });
  });

  // Capture button
  document.getElementById('captureBtn').addEventListener('click', () => {
    const btn = document.getElementById('captureBtn');
    btn.textContent = 'Capturing...';
    btn.disabled = true;

    safeSendMessage({ action: 'captureCookies', provider: activeProvider }, (resp) => {
      btn.disabled = false;
      updateProviderUI(); // Reset button text

      if (resp && resp.success) {
        loadCapturedCookies();
      } else {
        setStatus(resp?.error || 'Failed to capture cookies', 'err');
      }
    });
  });

  // Send cookies button
  document.getElementById('sendCookiesBtn').addEventListener('click', () => {
    if (!latestCapture || !currentServerUrl) return;

    const btn = document.getElementById('sendCookiesBtn');
    btn.textContent = 'Sending...';
    btn.disabled = true;

    safeSendMessage({
      action: 'sendCookies',
      serverUrl: currentServerUrl,
      cookieData: latestCapture,
    }, (resp) => {
      if (resp && resp.success) {
        btn.textContent = 'Sent!';
        loadCapturedCookies();
      } else {
        btn.textContent = 'Failed - Retry';
        btn.disabled = false;
        setStatus(resp?.error || 'Failed to send cookies to server', 'err');
      }
    });
  });

  // Add Account
  document.getElementById('addAccountBtn').addEventListener('click', () => {
    document.getElementById('addAccountForm').style.display = 'block';
    document.getElementById('newAccountProvider').value = activeProvider;
    document.getElementById('newAccountName').focus();
  });

  document.getElementById('cancelAccountBtn').addEventListener('click', () => {
    document.getElementById('addAccountForm').style.display = 'none';
    document.getElementById('newAccountName').value = '';
  });

  document.getElementById('saveAccountBtn').addEventListener('click', () => {
    const name = document.getElementById('newAccountName').value.trim();
    const provider = document.getElementById('newAccountProvider').value;

    if (!name) {
      setStatus('Enter an account label', 'err');
      return;
    }

    safeSendMessage({
      action: 'addAccount',
      account: { name, provider },
    }, (resp) => {
      if (resp && resp.success) {
        document.getElementById('addAccountForm').style.display = 'none';
        document.getElementById('newAccountName').value = '';
        loadAccounts();
      }
    });
  });

  // Toggle handlers
  document.getElementById('enabledToggle').addEventListener('change', (e) => {
    safeSendMessage({ action: 'updateSettings', settings: { enabled: e.target.checked } });
  });
  document.getElementById('autoCaptureToggle').addEventListener('change', (e) => {
    safeSendMessage({ action: 'updateSettings', settings: { autoCapture: e.target.checked } });
  });
  document.getElementById('autoCaptureOnLoginToggle').addEventListener('change', (e) => {
    safeSendMessage({ action: 'updateSettings', settings: { autoCaptureOnLogin: e.target.checked } });
  });
  document.getElementById('notificationsToggle').addEventListener('change', (e) => {
    safeSendMessage({ action: 'updateSettings', settings: { notifications: e.target.checked } });
  });

  // Clear all
  document.getElementById('clearBtn').addEventListener('click', () => {
    if (confirm('Clear all captured cookies?')) {
      safeSendMessage({ action: 'clearCookies' }, () => {
        safeSendMessage({ action: 'clearTokens' }, () => {
          document.getElementById('captureResult').style.display = 'none';
          latestCapture = null;
          loadCapturedCookies();
        });
      });
    }
  });
}
