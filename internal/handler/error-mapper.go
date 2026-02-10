package handler

import (
	"errors"
	"net/http"

	"model-registry-service/internal/domain"

	"github.com/gin-gonic/gin"
)

func mapDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrModelNotFound),
		errors.Is(err, domain.ErrVersionNotFound),
		errors.Is(err, domain.ErrArtifactNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, domain.ErrModelNameConflict),
		errors.Is(err, domain.ErrVersionNameConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

	case errors.Is(err, domain.ErrInvalidModelName),
		errors.Is(err, domain.ErrMissingProjectID),
		errors.Is(err, domain.ErrCannotDeleteModel):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
