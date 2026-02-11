package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/adapters/primary/http/dto"
	"model-registry-service/internal/core/domain"
	output "model-registry-service/internal/core/ports/output"
	"model-registry-service/internal/core/services"
)

// ============================================================================
// Traffic Config CRUD
// ============================================================================

func (h *Handler) ListTrafficConfigs(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := output.TrafficConfigFilter{
		ProjectID: projectID,
		Strategy:  c.Query("strategy"),
		Status:    c.Query("status"),
		Limit:     limit,
		Offset:    offset,
	}

	if isvcID := c.Query("inference_service_id"); isvcID != "" {
		if id, err := uuid.Parse(isvcID); err == nil {
			filter.InferenceServiceID = &id
		}
	}

	configs, total, err := h.trafficSvc.ListConfigs(c.Request.Context(), filter)
	if err != nil {
		log.WithError(err).Error("list traffic configs failed")
		mapDomainError(c, err)
		return
	}

	items := make([]dto.TrafficConfigResponse, 0, len(configs))
	for _, config := range configs {
		items = append(items, dto.ToTrafficConfigResponse(config))
	}

	c.JSON(http.StatusOK, dto.ListTrafficConfigsResponse{
		Items:      items,
		Total:      total,
		PageSize:   limit,
		NextOffset: offset + len(items),
	})
}

func (h *Handler) GetTrafficConfig(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	config, err := h.trafficSvc.GetConfig(c.Request.Context(), projectID, id)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) CreateTrafficConfig(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	var req dto.CreateTrafficConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	strategy := domain.TrafficStrategyCanary
	if req.Strategy != "" {
		strategy = domain.TrafficStrategy(req.Strategy)
	}

	config, err := h.trafficSvc.CreateConfig(c.Request.Context(), services.CreateTrafficConfigRequest{
		ProjectID:          projectID,
		InferenceServiceID: req.InferenceServiceID,
		Strategy:           strategy,
		StableVersionID:    req.StableVersionID,
	})
	if err != nil {
		log.WithError(err).Error("create traffic config failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) DeleteTrafficConfig(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.trafficSvc.DeleteConfig(c.Request.Context(), projectID, id); err != nil {
		log.WithError(err).Error("delete traffic config failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ============================================================================
// Canary Operations
// ============================================================================

func (h *Handler) StartCanary(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req dto.StartCanaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.StartCanary(c.Request.Context(), services.StartCanaryRequest{
		ProjectID:      projectID,
		ConfigID:       configID,
		ModelVersionID: req.ModelVersionID,
		InitialWeight:  req.InitialWeight,
	})
	if err != nil {
		log.WithError(err).Error("start canary failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) UpdateCanaryWeight(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req dto.UpdateCanaryWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.UpdateCanaryWeight(c.Request.Context(), projectID, configID, req.Weight)
	if err != nil {
		log.WithError(err).Error("update canary weight failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) PromoteCanary(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	config, err := h.trafficSvc.PromoteCanary(c.Request.Context(), projectID, configID)
	if err != nil {
		log.WithError(err).Error("promote canary failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) Rollback(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	config, err := h.trafficSvc.Rollback(c.Request.Context(), projectID, configID)
	if err != nil {
		log.WithError(err).Error("rollback failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

// ============================================================================
// Variant CRUD
// ============================================================================

func (h *Handler) ListVariants(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	config, err := h.trafficSvc.GetConfig(c.Request.Context(), projectID, configID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	variants := make([]dto.TrafficVariantResponse, 0)
	for _, v := range config.Variants {
		variants = append(variants, dto.TrafficVariantResponse{
			ID:               v.ID,
			VariantName:      v.VariantName,
			ModelVersionID:   v.ModelVersionID,
			ModelVersionName: v.ModelVersionName,
			Weight:           v.Weight,
			Status:           string(v.Status),
			KServeISVCName:   v.KServeISVCName,
		})
	}

	c.JSON(http.StatusOK, dto.ListVariantsResponse{
		ConfigID: configID,
		Variants: variants,
		Total:    len(variants),
	})
}

func (h *Handler) GetVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	variantName := c.Param("name")

	config, err := h.trafficSvc.GetConfig(c.Request.Context(), projectID, configID)
	if err != nil {
		mapDomainError(c, err)
		return
	}

	variant := config.GetVariant(variantName)
	if variant == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": domain.ErrTrafficVariantNotFound.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.TrafficVariantResponse{
		ID:               variant.ID,
		VariantName:      variant.VariantName,
		ModelVersionID:   variant.ModelVersionID,
		ModelVersionName: variant.ModelVersionName,
		Weight:           variant.Weight,
		Status:           string(variant.Status),
		KServeISVCName:   variant.KServeISVCName,
	})
}

func (h *Handler) AddVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req dto.AddVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.AddVariant(c.Request.Context(), services.AddVariantRequest{
		ProjectID:      projectID,
		ConfigID:       configID,
		VariantName:    req.VariantName,
		ModelVersionID: req.ModelVersionID,
		Weight:         req.Weight,
	})
	if err != nil {
		log.WithError(err).Error("add variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) UpdateVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	variantName := c.Param("name")

	var req dto.UpdateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.UpdateVariant(c.Request.Context(), projectID, configID, variantName, req.Weight)
	if err != nil {
		log.WithError(err).Error("update variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) DeleteVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	variantName := c.Param("name")

	_, err = h.trafficSvc.DeleteVariant(c.Request.Context(), projectID, configID, variantName)
	if err != nil {
		log.WithError(err).Error("delete variant failed")
		mapDomainError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ============================================================================
// Bulk Operations
// ============================================================================

func (h *Handler) BulkUpdateWeights(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	var req dto.BulkUpdateWeightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.trafficSvc.BulkUpdateWeights(c.Request.Context(), services.BulkUpdateWeightsRequest{
		ProjectID: projectID,
		ConfigID:  configID,
		Weights:   req.Weights,
	})
	if err != nil {
		log.WithError(err).Error("bulk update weights failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}

func (h *Handler) PromoteVariant(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config id"})
		return
	}

	variantName := c.Param("variant_name")

	config, err := h.trafficSvc.PromoteVariant(c.Request.Context(), projectID, configID, variantName)
	if err != nil {
		log.WithError(err).Error("promote variant failed")
		mapDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToTrafficConfigResponse(config))
}
