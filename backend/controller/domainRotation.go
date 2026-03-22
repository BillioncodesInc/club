package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// DomainRotation is a controller for domain rotation and health monitoring
type DomainRotation struct {
	Common
	Service *service.DomainRotator
}

// GetConfig returns the domain rotator configuration
func (dr *DomainRotation) GetConfig(g *gin.Context) {
	_, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	config := dr.Service.GetConfig()
	dr.Response.OK(g, config)
}

// UpdateConfig updates the domain rotator configuration
func (dr *DomainRotation) UpdateConfig(g *gin.Context) {
	session, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	var config service.DomainRotatorConfig
	if ok := dr.handleParseRequest(g, &config); !ok {
		return
	}
	err := dr.Service.UpdateConfig(g.Request.Context(), session, &config)
	if ok := dr.handleErrors(g, err); !ok {
		return
	}
	dr.Response.OK(g, gin.H{})
}

// GetStatus returns the current rotation status
func (dr *DomainRotation) GetStatus(g *gin.Context) {
	_, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	status := dr.Service.GetStatus()
	dr.Response.OK(g, status)
}

// CheckHealth performs a health check on a specific domain
func (dr *DomainRotation) CheckHealth(g *gin.Context) {
	_, _, ok := dr.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if ok := dr.handleParseRequest(g, &req); !ok {
		return
	}

	rep, err := dr.Service.CheckDomainReputation(req.Domain)
	if ok := dr.handleErrors(g, err); !ok {
		return
	}
	dr.Response.OK(g, rep)
}

// CheckAllHealth performs a health check on all domains in the pool
func (dr *DomainRotation) CheckAllHealth(g *gin.Context) {
	session, _, ok := dr.handleSession(g)
	if !ok {
		return
	}

	config := dr.Service.GetConfig()
	results := make(map[string]*service.ReputationInfo)

	for _, d := range config.DomainPool {
		if d.Status == "burned" {
			continue
		}
		rep, err := dr.Service.CheckDomainReputation(d.Domain)
		if err != nil {
			dr.Logger.Warnw("health check failed", "domain", d.Domain, "error", err)
			continue
		}
		results[d.Domain] = rep
	}

	// Update the config with new reputation data
	for i, d := range config.DomainPool {
		if rep, ok := results[d.Domain]; ok {
			config.DomainPool[i].Reputation = rep
		}
	}
	_ = dr.Service.UpdateConfig(g.Request.Context(), session, config)

	dr.Response.OK(g, results)
}

// AddDomain adds a domain to the rotation pool
func (dr *DomainRotation) AddDomain(g *gin.Context) {
	session, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if ok := dr.handleParseRequest(g, &req); !ok {
		return
	}
	err := dr.Service.AddDomain(g.Request.Context(), session, req.Domain)
	if ok := dr.handleErrors(g, err); !ok {
		return
	}
	dr.Response.OK(g, gin.H{})
}

// RemoveDomain removes a domain from the rotation pool
func (dr *DomainRotation) RemoveDomain(g *gin.Context) {
	session, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if ok := dr.handleParseRequest(g, &req); !ok {
		return
	}
	err := dr.Service.RemoveDomain(g.Request.Context(), session, req.Domain)
	if ok := dr.handleErrors(g, err); !ok {
		return
	}
	dr.Response.OK(g, gin.H{})
}

// Rotate triggers a manual domain rotation
func (dr *DomainRotation) Rotate(g *gin.Context) {
	session, _, ok := dr.handleSession(g)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if ok := dr.handleParseRequest(g, &req); !ok {
		return
	}
	result, err := dr.Service.RotateDomain(g.Request.Context(), session, req.Reason)
	if ok := dr.handleErrors(g, err); !ok {
		return
	}
	dr.Response.OK(g, result)
}
