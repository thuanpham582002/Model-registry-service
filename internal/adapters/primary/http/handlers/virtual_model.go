package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/services"
)

// ============================================================================
// Virtual Model CRUD
// ============================================================================

func (h *Handler) ListVirtualModels(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vms, err := h.virtualModelSvc.List(c.Request.Context(), projectID)
	if err != nil {
		log.WithError(err).Error("list virtual models failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.VirtualModelResponse, 0, len(vms))
	for _, vm := range vms {
		items = append(items, dto.ToVirtualModelResponse(vm))
	}

	c.JSON(http.StatusOK, dto.ListVirtualModelsResponse{
		Items: items,
		Total: len(items),
	})
}

func (h *Handler) GetVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Param("name")

	vm, err := h.virtualModelSvc.Get(c.Request.Context(), projectID, name)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) CreateVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateVirtualModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.virtualModelSvc.Create(c.Request.Context(), services.CreateVirtualModelRequest{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		log.WithError(err).Error("create virtual model failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) DeleteVirtualModel(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	name := c.Param("name")

	if err := h.virtualModelSvc.Delete(c.Request.Context(), projectID, name); err != nil {
		log.WithError(err).Error("delete virtual model failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ============================================================================
// Backend CRUD
// ============================================================================

func (h *Handler) ListVirtualModelBackends(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")

	vm, err := h.virtualModelSvc.Get(c.Request.Context(), projectID, vmName)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	backends := make([]dto.VirtualModelBackendRes, 0, len(vm.Backends))
	for _, b := range vm.Backends {
		backends = append(backends, dto.VirtualModelBackendRes{
			ID:                        b.ID,
			AIServiceBackendName:      b.AIServiceBackendName,
			AIServiceBackendNamespace: b.AIServiceBackendNamespace,
			ModelNameOverride:         b.ModelNameOverride,
			Weight:                    b.Weight,
			Priority:                  b.Priority,
			Status:                    b.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"virtual_model_name": vmName,
		"backends":           backends,
		"total":              len(backends),
	})
}

func (h *Handler) AddVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")

	var req dto.AddBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.virtualModelSvc.AddBackend(c.Request.Context(), services.AddBackendRequest{
		ProjectID:            projectID,
		VirtualModelName:     vmName,
		AIServiceBackendName: req.AIServiceBackendName,
		AIServiceBackendNS:   req.AIServiceBackendNS,
		ModelNameOverride:    req.ModelNameOverride,
		Weight:               req.Weight,
		Priority:             req.Priority,
	})
	if err != nil {
		log.WithError(err).Error("add virtual model backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) UpdateVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")
	backendID, err := uuid.Parse(c.Param("backend_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backend_id"})
		return
	}

	var req dto.UpdateBackendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.virtualModelSvc.UpdateBackend(c.Request.Context(), projectID, vmName, backendID, req.Weight, req.Priority)
	if err != nil {
		log.WithError(err).Error("update virtual model backend failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToVirtualModelResponse(vm))
}

func (h *Handler) DeleteVirtualModelBackend(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	vmName := c.Param("name")
	backendID, err := uuid.Parse(c.Param("backend_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backend_id"})
		return
	}

	_, err = h.virtualModelSvc.DeleteBackend(c.Request.Context(), projectID, vmName, backendID)
	if err != nil {
		log.WithError(err).Error("delete virtual model backend failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
