package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"model-registry-service/internal/core/domain"
	"model-registry-service/internal/core/services"
)

func (h *Handler) GetDeploymentMetrics(c *gin.Context) {
	isvcName := c.Param("isvc_name")
	variantName := c.DefaultQuery("variant", "")

	from, to, step := parseTimeRange(c)

	metrics, err := h.metricsSvc.GetDeploymentMetrics(c.Request.Context(), services.DeploymentMetricsRequest{
		ISVCName:    isvcName,
		VariantName: variantName,
		From:        from,
		To:          to,
		Step:        step,
	})
	if err != nil {
		log.WithError(err).Error("get deployment metrics failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) CompareVariants(c *gin.Context) {
	isvcName := c.Param("isvc_name")
	variants := c.QueryArray("variant")

	if len(variants) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least 2 variants required"})
		return
	}

	from, to, _ := parseTimeRange(c)

	comparison, err := h.metricsSvc.CompareVariants(c.Request.Context(), services.VariantComparisonRequest{
		ISVCName: isvcName,
		Variants: variants,
		From:     from,
		To:       to,
	})
	if err != nil {
		log.WithError(err).Error("compare variants failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compare variants"})
		return
	}

	c.JSON(http.StatusOK, comparison)
}

func (h *Handler) GetTokenUsageMetrics(c *gin.Context) {
	projectID, err := getProjectID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": domain.ErrMissingProjectID.Error()})
		return
	}

	from, to, step := parseTimeRange(c)

	usage, err := h.metricsSvc.GetTokenUsage(c.Request.Context(), services.TokenUsageRequest{
		ProjectID: projectID.String(),
		From:      from,
		To:        to,
		Step:      step,
	})
	if err != nil {
		log.WithError(err).Error("get token usage failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get token usage"})
		return
	}

	c.JSON(http.StatusOK, usage)
}

func parseTimeRange(c *gin.Context) (from, to time.Time, step time.Duration) {
	// Default: last 1 hour
	to = time.Now()
	from = to.Add(-1 * time.Hour)
	step = time.Minute

	if fromStr := c.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}
	if stepStr := c.Query("step"); stepStr != "" {
		if parsed, err := time.ParseDuration(stepStr); err == nil {
			step = parsed
		}
	}

	return from, to, step
}
