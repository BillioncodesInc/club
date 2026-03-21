package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

type DKIM struct {
	Common
	DKIMService *service.DKIM
}

type dkimGenerateKeyRequest struct {
	Domain   string `json:"domain" binding:"required"`
	Selector string `json:"selector"`
}

type dkimSignRequest struct {
	Domain           string            `json:"domain" binding:"required"`
	Selector         string            `json:"selector"`
	PrivateKeyPEM    string            `json:"privateKeyPem" binding:"required"`
	Canonicalization string            `json:"canonicalization"`
	HeaderFields     string            `json:"headerFields"`
	Headers          map[string]string `json:"headers" binding:"required"`
	Body             string            `json:"body" binding:"required"`
}

type dkimVerifyRequest struct {
	RawHeader string `json:"rawHeader" binding:"required"`
}

type dkimDNSRecordRequest struct {
	Domain       string `json:"domain" binding:"required"`
	Selector     string `json:"selector" binding:"required"`
	PublicKeyPEM string `json:"publicKeyPem" binding:"required"`
}

// GenerateKeyPair generates a new DKIM key pair
func (c *DKIM) GenerateKeyPair(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req dkimGenerateKeyRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	keyPair, err := c.DKIMService.GenerateKeyPair(req.Domain, req.Selector)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, keyPair)
}

// SignEmail signs an email with DKIM
func (c *DKIM) SignEmail(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req dkimSignRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	config := &service.DKIMConfig{
		Domain:           req.Domain,
		Selector:         req.Selector,
		PrivateKeyPEM:    req.PrivateKeyPEM,
		Canonicalization: req.Canonicalization,
		HeaderFields:     req.HeaderFields,
	}

	signature, err := c.DKIMService.SignEmail(config, req.Headers, req.Body)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, gin.H{"signature": signature})
}

// VerifyDKIMHeader verifies a DKIM-Signature header
func (c *DKIM) VerifyDKIMHeader(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req dkimVerifyRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	result := c.DKIMService.VerifyDKIMHeader(req.RawHeader)
	c.Response.OK(g, result)
}

// GenerateDNSRecord generates the DNS TXT record for DKIM
func (c *DKIM) GenerateDNSRecord(g *gin.Context) {
	_, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req dkimDNSRecordRequest
	if !c.handleParseRequest(g, &req) {
		return
	}

	dnsName, dnsValue, err := c.DKIMService.GenerateDNSRecord(req.Domain, req.Selector, req.PublicKeyPEM)
	if ok := c.handleErrors(g, err); !ok {
		return
	}

	c.Response.OK(g, gin.H{
		"dnsName":  dnsName,
		"dnsValue": dnsValue,
	})
}
