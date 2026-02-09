package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListModelVersions(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s/versions", id)
	if q := c.Request.URL.RawQuery; q != "" {
		upstreamPath += "?" + q
	}
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) GetModelVersion(c *gin.Context) {
	id := c.Param("id")
	ver := c.Param("ver")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s/versions/%s", id, ver)
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) CreateModelVersion(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s/versions", id)
	h.forwardAndRespond(c, http.MethodPost, upstreamPath)
}

func (h *Handler) UpdateModelVersion(c *gin.Context) {
	id := c.Param("id")
	ver := c.Param("ver")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s/versions/%s", id, ver)
	h.forwardAndRespond(c, http.MethodPatch, upstreamPath)
}

// Direct access routes (without parent model context)

func (h *Handler) FindModelVersion(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/model_version"
	if q := c.Request.URL.RawQuery; q != "" {
		upstreamPath += "?" + q
	}
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) ListAllModelVersions(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/model_versions"
	if q := c.Request.URL.RawQuery; q != "" {
		upstreamPath += "?" + q
	}
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) GetModelVersionDirect(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/model_versions/%s", id)
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) UpdateModelVersionDirect(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/model_versions/%s", id)
	h.forwardAndRespond(c, http.MethodPatch, upstreamPath)
}
