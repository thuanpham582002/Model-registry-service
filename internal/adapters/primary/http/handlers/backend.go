package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
)

// ListBackends lists all Envoy Gateway Backends in a namespace
func (h *Handler) ListBackends(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "model-serving")

	backends, err := h.backendSvc.List(c.Request.Context(), namespace)
	if err != nil {
		log.WithError(err).Error("list backends failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.BackendResponse, 0, len(backends))
	for _, b := range backends {
		items = append(items, dto.ToBackendResponse(b))
	}

	c.JSON(http.StatusOK, dto.ListBackendsResponse{
		Items: items,
		Total: len(items),
	})
}

// GetBackend retrieves a single Backend
func (h *Handler) GetBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	backend, err := h.backendSvc.Get(c.Request.Context(), namespace, name)
	if err != nil {
		log.WithError(err).Error("get backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToBackendResponse(backend))
}

// CreateBackend creates a new Backend
func (h *Handler) CreateBackend(c *gin.Context) {
	var req dto.CreateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate: at least one endpoint with valid type
	for i, ep := range req.Endpoints {
		if ep.FQDN == nil && ep.IP == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint must have fqdn or ip", "index": i})
			return
		}
		if ep.FQDN != nil && ep.IP != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint cannot have both fqdn and ip", "index": i})
			return
		}
	}

	backend := dto.ToBackend(&req)
	if backend.Namespace == "" {
		backend.Namespace = "model-serving"
	}

	if err := h.backendSvc.Create(c.Request.Context(), backend); err != nil {
		log.WithError(err).Error("create backend failed")
		mapDomainError(c, err)
		return
	}

	// Fetch created backend to get full response
	created, _ := h.backendSvc.Get(c.Request.Context(), backend.Namespace, backend.Name)
	if created != nil {
		c.JSON(http.StatusCreated, dto.ToBackendResponse(created))
	} else {
		c.JSON(http.StatusCreated, dto.ToBackendResponse(backend))
	}
}

// UpdateBackend updates an existing Backend
func (h *Handler) UpdateBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	var req dto.UpdateEnvoyBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing
	existing, err := h.backendSvc.Get(c.Request.Context(), namespace, name)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	// Apply updates
	if req.Endpoints != nil {
		existing.Endpoints = nil
		for _, ep := range req.Endpoints {
			existing.Endpoints = append(existing.Endpoints, dto.EndpointRequestToPorts(&ep))
		}
	}
	if req.Labels != nil {
		existing.Labels = req.Labels
	}

	if err := h.backendSvc.Update(c.Request.Context(), existing); err != nil {
		log.WithError(err).Error("update backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToBackendResponse(existing))
}

// DeleteBackend deletes a Backend
func (h *Handler) DeleteBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	if err := h.backendSvc.Delete(c.Request.Context(), namespace, name); err != nil {
		log.WithError(err).Error("delete backend failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
