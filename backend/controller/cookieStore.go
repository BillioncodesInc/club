package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/service"
	"strconv"
)

// CookieStoreController handles cookie store CRUD, sending, and inbox reading
type CookieStoreController struct {
	Common
	Service *service.CookieStoreService
}

// GetAll returns all cookie stores with pagination
func (c *CookieStoreController) GetAll(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	queryArgs, ok := c.handleQueryArgs(g)
	if !ok {
		return
	}
	queryArgs.DefaultSortByUpdatedAt()

	result, err := c.Service.GetAll(g, session, &repository.CookieStoreOption{
		QueryArgs: queryArgs,
	})
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// GetByID returns a cookie store by ID
func (c *CookieStoreController) GetByID(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	store, err := c.Service.GetByID(g, session, id)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, store)
}

// Import imports cookies from a manual import request
func (c *CookieStoreController) Import(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req model.CookieStoreImportRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if req.Name == "" {
		c.Response.BadRequestMessage(g, "name is required")
		return
	}
	if len(req.Cookies) == 0 {
		c.Response.BadRequestMessage(g, "cookies array is required")
		return
	}

	id, err := c.Service.Import(g, session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{"id": id, "message": "cookies imported, validating session..."})
}

// ImportFromCapture imports cookies from a proxy capture record
func (c *CookieStoreController) ImportFromCapture(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req struct {
		CaptureID  string `json:"captureId"`
		Name       string `json:"name"`
		CookieJSON string `json:"cookieJSON"`
	}
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	captureID, err := uuid.Parse(req.CaptureID)
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid capture ID")
		return
	}

	if req.Name == "" {
		req.Name = "Proxy Capture Import"
	}

	id, err := c.Service.ImportFromProxyCapture(g, session, captureID, req.Name, req.CookieJSON)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{"id": id, "message": "cookies imported from capture, validating session..."})
}

// Revalidate re-checks if a cookie session is still valid
func (c *CookieStoreController) Revalidate(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	store, err := c.Service.Revalidate(g, session, id)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, store)
}

// Delete deletes a cookie store by ID
func (c *CookieStoreController) Delete(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	err = c.Service.Delete(g, session, id)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{"message": "deleted"})
}

// DeleteAll deletes all cookie stores
func (c *CookieStoreController) DeleteAll(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	err := c.Service.DeleteAll(g, session)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{"message": "all cookie stores deleted"})
}

// Send sends an email using captured cookies
func (c *CookieStoreController) Send(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	var req model.CookieSendRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}

	if req.CookieStoreID == "" {
		c.Response.BadRequestMessage(g, "cookieStoreId is required")
		return
	}
	if len(req.To) == 0 {
		c.Response.BadRequestMessage(g, "at least one recipient is required")
		return
	}
	if req.Subject == "" {
		c.Response.BadRequestMessage(g, "subject is required")
		return
	}

	result, err := c.Service.SendEmail(g, session, &req)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, result)
}

// GetInbox reads the inbox of a cookie session
func (c *CookieStoreController) GetInbox(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	folder := g.DefaultQuery("folder", "inbox")
	limit := 25
	skip := 0

	if l := g.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if s := g.Query("skip"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			skip = v
		}
	}

	messages, total, err := c.Service.GetInbox(g, session, id, folder, limit, skip)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{
		"messages": messages,
		"total":    total,
		"folder":   folder,
	})
}

// GetMessage reads a specific email message
func (c *CookieStoreController) GetMessage(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	messageID := g.Param("messageId")
	if messageID == "" {
		c.Response.BadRequestMessage(g, "messageId is required")
		return
	}

	msg, err := c.Service.GetMessage(g, session, id, messageID)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, msg)
}

// GetFolders lists mail folders for a cookie session
func (c *CookieStoreController) GetFolders(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}

	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}

	folders, err := c.Service.GetFolders(g, session, id)
	if !c.handleErrors(g, err) {
		return
	}

	c.Response.OK(g, gin.H{"folders": folders})
}
