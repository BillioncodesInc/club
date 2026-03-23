package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/phishingclub/phishingclub/data"
	"github.com/phishingclub/phishingclub/repository"
	"github.com/phishingclub/phishingclub/service"
)

// ProxyCapture is the controller for proxy capture management.
type ProxyCapture struct {
	Common
	ProxyCaptureRepository *repository.ProxyCapture
}

// GetAll returns all proxy captures with pagination.
func (c *ProxyCapture) GetAll(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	queryArgs, ok := c.handleQueryArgs(g)
	if !ok {
		return
	}
	queryArgs.DefaultSortByUpdatedAt()

	captures, hasNextPage, err := c.ProxyCaptureRepository.GetAll(
		g.Request.Context(),
		&repository.ProxyCaptureOption{QueryArgs: queryArgs},
	)
	if err != nil {
		c.Response.ServerError(g)
		return
	}

	c.Response.OK(g, gin.H{
		"rows":        captures,
		"hasNextPage": hasNextPage,
	})
}

// GetByID returns a single proxy capture by ID.
func (c *ProxyCapture) GetByID(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	idStr := g.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	capture, err := c.ProxyCaptureRepository.GetByID(g.Request.Context(), &id)
	if err != nil {
		c.Response.NotFound(g)
		return
	}

	c.Response.OK(g, capture)
}

// DeleteByID deletes a proxy capture by ID.
func (c *ProxyCapture) DeleteByID(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	idStr := g.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.Response.BadRequest(g)
		return
	}

	if err := c.ProxyCaptureRepository.DeleteByID(g.Request.Context(), &id); err != nil {
		c.Response.ServerError(g)
		return
	}

	c.Response.OK(g, gin.H{"message": "Proxy capture deleted"})
}

// DeleteAll deletes all proxy captures.
func (c *ProxyCapture) DeleteAll(g *gin.Context) {
	session, _, ok := c.handleSession(g)
	if !ok {
		return
	}
	isAuthorized, err := service.IsAuthorized(session, data.PERMISSION_ALLOW_GLOBAL)
	if err != nil || !isAuthorized {
		c.Response.Forbidden(g)
		return
	}

	if err := c.ProxyCaptureRepository.DeleteAll(g.Request.Context()); err != nil {
		c.Response.ServerError(g)
		return
	}

	c.Response.OK(g, gin.H{"message": "All proxy captures deleted"})
}
