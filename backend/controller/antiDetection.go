package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/phishingclub/phishingclub/service"
)

// AntiDetection is the controller for anti-detection tools
type AntiDetection struct {
	Common
	AntiDetectionService *service.AntiDetection
}

// ScanDirtyWordsRequest is the request body for scanning dirty words
type ScanDirtyWordsRequest struct {
	Content string `json:"content"`
}

// MutateHTMLRequest is the request body for HTML mutation
type MutateHTMLRequest struct {
	HTML      string  `json:"html"`
	Method    int     `json:"method"`    // 0-4 (MutationMethod)
	Intensity float64 `json:"intensity"` // 0.0 to 1.0
}

// EncodeTextRequest is the request body for text encoding
type EncodeTextRequest struct {
	Text   string `json:"text"`
	Method int    `json:"method"` // 0-2 (EncodingMethod)
}

// ScanDirtyWords scans content for spam trigger words
func (a *AntiDetection) ScanDirtyWords(g *gin.Context) {
	session, _, ok := a.handleSession(g)
	if !ok {
		return
	}

	var req ScanDirtyWordsRequest
	if ok := a.handleParseRequest(g, &req); !ok {
		return
	}

	result, err := a.AntiDetectionService.ScanForDirtyWords(g.Request.Context(), session, req.Content)
	if !a.handleErrors(g, err) {
		return
	}

	a.Response.OK(g, result)
}

// MutateHTML applies anti-fingerprinting mutations to HTML content
func (a *AntiDetection) MutateHTML(g *gin.Context) {
	session, _, ok := a.handleSession(g)
	if !ok {
		return
	}

	var req MutateHTMLRequest
	if ok := a.handleParseRequest(g, &req); !ok {
		return
	}

	result, err := a.AntiDetectionService.MutateHTML(
		g.Request.Context(),
		session,
		req.HTML,
		service.MutationMethod(req.Method),
		req.Intensity,
	)
	if !a.handleErrors(g, err) {
		return
	}

	a.Response.OK(g, map[string]string{"html": result})
}

// EncodeText applies anti-scanning encoding to text content
func (a *AntiDetection) EncodeText(g *gin.Context) {
	session, _, ok := a.handleSession(g)
	if !ok {
		return
	}

	var req EncodeTextRequest
	if ok := a.handleParseRequest(g, &req); !ok {
		return
	}

	result, err := a.AntiDetectionService.EncodeText(
		g.Request.Context(),
		session,
		req.Text,
		service.EncodingMethod(req.Method),
	)
	if !a.handleErrors(g, err) {
		return
	}

	a.Response.OK(g, map[string]string{"text": result})
}

// GetMutationMethods returns available HTML mutation methods
func (a *AntiDetection) GetMutationMethods(g *gin.Context) {
	_, _, ok := a.handleSession(g)
	if !ok {
		return
	}

	methods := a.AntiDetectionService.GetMutationMethods()
	a.Response.OK(g, methods)
}

// GetEncodingMethods returns available text encoding methods
func (a *AntiDetection) GetEncodingMethods(g *gin.Context) {
	_, _, ok := a.handleSession(g)
	if !ok {
		return
	}

	methods := a.AntiDetectionService.GetEncodingMethods()
	a.Response.OK(g, methods)
}
