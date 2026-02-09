package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetModelArtifact(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/model_artifacts/%s", id)
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) CreateModelArtifact(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/model_artifacts"
	h.forwardAndRespond(c, http.MethodPost, upstreamPath)
}

func (h *Handler) UpdateModelArtifact(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/model_artifacts/%s", id)
	h.forwardAndRespond(c, http.MethodPatch, upstreamPath)
}
