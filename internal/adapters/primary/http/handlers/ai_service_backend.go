package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
)

// ListAIServiceBackends lists all AIServiceBackends in a namespace
func (h *Handler) ListAIServiceBackends(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "model-serving")

	backends, err := h.aiBackendSvc.List(c.Request.Context(), namespace)
	if err != nil {
		log.WithError(err).Error("list ai service backends failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.AIServiceBackendResponse, 0, len(backends))
	for _, b := range backends {
		items = append(items, dto.ToAIServiceBackendResponse(b))
	}

	c.JSON(http.StatusOK, dto.ListAIServiceBackendsResponse{
		Items: items,
		Total: len(items),
	})
}

// GetAIServiceBackend retrieves a single AIServiceBackend
func (h *Handler) GetAIServiceBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	backend, err := h.aiBackendSvc.Get(c.Request.Context(), namespace, name)
	if err != nil {
		log.WithError(err).Error("get ai service backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToAIServiceBackendResponse(backend))
}

// CreateAIServiceBackend creates a new AIServiceBackend
func (h *Handler) CreateAIServiceBackend(c *gin.Context) {
	var req dto.CreateAIServiceBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	backend := dto.ToAIServiceBackend(&req)
	if backend.Namespace == "" {
		backend.Namespace = "model-serving"
	}

	if err := h.aiBackendSvc.Create(c.Request.Context(), backend); err != nil {
		log.WithError(err).Error("create ai service backend failed")
		mapDomainError(c, err)
		return
	}

	// Fetch created backend to get full response
	created, _ := h.aiBackendSvc.Get(c.Request.Context(), backend.Namespace, backend.Name)
	if created != nil {
		c.JSON(http.StatusCreated, dto.ToAIServiceBackendResponse(created))
	} else {
		c.JSON(http.StatusCreated, dto.ToAIServiceBackendResponse(backend))
	}
}

// UpdateAIServiceBackend updates an existing AIServiceBackend
func (h *Handler) UpdateAIServiceBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	var req dto.UpdateAIServiceBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing
	existing, err := h.aiBackendSvc.Get(c.Request.Context(), namespace, name)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	// Apply updates
	if req.Schema != nil {
		existing.Schema = *req.Schema
	}
	if req.Labels != nil {
		existing.Labels = req.Labels
	}
	if req.HeaderMutation != nil {
		hm := dto.HeaderMutationToPorts(req.HeaderMutation)
		existing.HeaderMutation = &hm
	}

	if err := h.aiBackendSvc.Update(c.Request.Context(), existing); err != nil {
		log.WithError(err).Error("update ai service backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToAIServiceBackendResponse(existing))
}

// DeleteAIServiceBackend deletes an AIServiceBackend
func (h *Handler) DeleteAIServiceBackend(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "model-serving")

	if err := h.aiBackendSvc.Delete(c.Request.Context(), namespace, name); err != nil {
		log.WithError(err).Error("delete ai service backend failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
