package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func (h *Handler) ListModels(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/registered_models"
	if q := c.Request.URL.RawQuery; q != "" {
		upstreamPath += "?" + q
	}
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) GetModel(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s", id)
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) GetModelByParams(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/registered_model"
	if q := c.Request.URL.RawQuery; q != "" {
		upstreamPath += "?" + q
	}
	h.forwardAndRespond(c, http.MethodGet, upstreamPath)
}

func (h *Handler) CreateModel(c *gin.Context) {
	upstreamPath := "/api/model_registry/v1alpha3/registered_models"
	h.forwardAndRespond(c, http.MethodPost, upstreamPath)
}

func (h *Handler) UpdateModel(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s", id)
	h.forwardAndRespond(c, http.MethodPatch, upstreamPath)
}

func (h *Handler) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	upstreamPath := fmt.Sprintf("/api/model_registry/v1alpha3/registered_models/%s", id)
	h.forwardAndRespond(c, http.MethodDelete, upstreamPath)
}

// forwardAndRespond proxies the request to upstream and writes the response back.
func (h *Handler) forwardAndRespond(c *gin.Context, method, upstreamPath string) {
	resp, err := h.proxy.Forward(method, upstreamPath, c.Request.Body, c.Request.Header)
	if err != nil {
		log.WithError(err).Error("upstream request failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("read upstream response")
		c.JSON(http.StatusBadGateway, gin.H{"error": "read upstream response failed"})
		return
	}

	// Copy upstream response headers
	for key, values := range resp.Header {
		for _, v := range values {
			c.Header(key, v)
		}
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}
