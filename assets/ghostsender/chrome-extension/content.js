/**
 * ╔════════════════════════════════════════════════════════════════════════════════════╗
 * ║              🔌 GHOST HACKER OAUTH CAPTURE - CONTENT SCRIPT 🔌                    ║
 * ║                         Chrome Extension - Premium Edition                         ║
 * ╚════════════════════════════════════════════════════════════════════════════════════╝
 */

console.log('Ghost Hacker OS OAuth Capture - Content script loaded on', window.location.hostname);

/**
 * Microsoft OAuth domains to watch
 */
const OAUTH_DOMAINS = ['login.microsoftonline.com', 'login.live.com', 'account.live.com'];

/**
 * Monitor URL changes (for single-page apps)
 */
let lastUrl = location.href;
new MutationObserver(() => {
  const currentUrl = location.href;
  if (currentUrl !== lastUrl) {
    lastUrl = currentUrl;
    checkCurrentUrl();
  }
}).observe(document, { subtree: true, childList: true });

/**
 * Check current URL for OAuth redirect
 */
function checkCurrentUrl() {
  const url = window.location.href;
  const hostname = window.location.hostname;
  
  // Check if this is on a Microsoft OAuth domain
  const isOAuthDomain = OAUTH_DOMAINS.includes(hostname);
  
  // Check for authorization code in URL params or hash
  const hasCode = url.includes('code=');
  
  // Also watch for the nativeclient redirect (the final redirect after consent)
  const isNativeRedirect = url.includes('oauth2/nativeclient');
  
  if ((isOAuthDomain && hasCode) || isNativeRedirect) {
    console.log('OAuth redirect detected in content script:', url);
    
    // Send to background script for processing
    chrome.runtime.sendMessage({
      action: 'oauthRedirect',
      url: url
    });
  }
}

// Check on load
checkCurrentUrl();

/**
 * Inject visual indicator when OAuth code is captured
 */
function showCaptureIndicator() {
  const indicator = document.createElement('div');
  indicator.id = 'ghost-hacker-oauth-indicator';
  indicator.innerHTML = `
    <div style="
      position: fixed;
      top: 20px;
      right: 20px;
      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
      color: white;
      padding: 15px 25px;
      border-radius: 10px;
      box-shadow: 0 10px 30px rgba(0,0,0,0.3);
      z-index: 999999;
      font-family: 'Segoe UI', sans-serif;
      font-size: 14px;
      animation: slideIn 0.3s ease-out;
    ">
      <div style="display: flex; align-items: center; gap: 10px;">
        <span style="font-size: 24px;">✅</span>
        <div>
          <div style="font-weight: bold;">OAuth Code Captured!</div>
          <div style="font-size: 12px; opacity: 0.9;">Sending to Ghost Hacker OS...</div>
        </div>
      </div>
    </div>
    <style>
      @keyframes slideIn {
        from {
          transform: translateX(400px);
          opacity: 0;
        }
        to {
          transform: translateX(0);
          opacity: 1;
        }
      }
    </style>
  `;
  
  document.body.appendChild(indicator);
  
  // Remove after 3 seconds
  setTimeout(() => {
    indicator.style.animation = 'slideOut 0.3s ease-in';
    setTimeout(() => {
      indicator.remove();
    }, 300);
  }, 3000);
}

// Listen for messages from background script
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'showIndicator') {
    showCaptureIndicator();
  }
});

