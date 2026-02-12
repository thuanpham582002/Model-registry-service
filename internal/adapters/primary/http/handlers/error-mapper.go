package handlers

import (
	"errors"
	"net/http"

	"model-registry-service/internal/core/domain"

	"github.com/gin-gonic/gin"
)

func mapDomainError(c *gin.Context, err error) {
	switch {
	// Not found errors
	case errors.Is(err, domain.ErrModelNotFound),
		errors.Is(err, domain.ErrVersionNotFound),
		errors.Is(err, domain.ErrArtifactNotFound),
		errors.Is(err, domain.ErrServingEnvNotFound),
		errors.Is(err, domain.ErrInferenceServiceNotFound),
		errors.Is(err, domain.ErrServeModelNotFound),
		errors.Is(err, domain.ErrTrafficConfigNotFound),
		errors.Is(err, domain.ErrTrafficVariantNotFound),
		errors.Is(err, domain.ErrVirtualModelNotFound),
		errors.Is(err, domain.ErrBackendNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	// Conflict errors
	case errors.Is(err, domain.ErrModelNameConflict),
		errors.Is(err, domain.ErrVersionNameConflict),
		errors.Is(err, domain.ErrServingEnvNameConflict),
		errors.Is(err, domain.ErrInferenceServiceNameConflict),
		errors.Is(err, domain.ErrVariantAlreadyExists),
		errors.Is(err, domain.ErrCanaryAlreadyExists),
		errors.Is(err, domain.ErrVirtualModelExists),
		errors.Is(err, domain.ErrBackendAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})

	// Bad request / validation errors
	case errors.Is(err, domain.ErrInvalidModelName),
		errors.Is(err, domain.ErrMissingProjectID),
		errors.Is(err, domain.ErrCannotDeleteModel),
		errors.Is(err, domain.ErrInvalidServingEnvName),
		errors.Is(err, domain.ErrInvalidServingEnvID),
		errors.Is(err, domain.ErrInvalidInferenceServiceName),
		errors.Is(err, domain.ErrInvalidInferenceServiceID),
		errors.Is(err, domain.ErrInvalidModelID),
		errors.Is(err, domain.ErrInvalidVersionID),
		errors.Is(err, domain.ErrInvalidState),
		errors.Is(err, domain.ErrServingEnvHasDeployments),
		errors.Is(err, domain.ErrCannotDeleteDeployed),
		errors.Is(err, domain.ErrVersionNotReady),
		errors.Is(err, domain.ErrModelHasActiveDeployments),
		errors.Is(err, domain.ErrInvalidVariantName),
		errors.Is(err, domain.ErrInvalidTrafficWeight),
		errors.Is(err, domain.ErrWeightSumExceeds100),
		errors.Is(err, domain.ErrNoStableVariant),
		errors.Is(err, domain.ErrCannotPromoteInactive),
		errors.Is(err, domain.ErrCannotPromoteStable),
		errors.Is(err, domain.ErrCannotDeleteStable),
		errors.Is(err, domain.ErrInvalidVirtualModelName),
		errors.Is(err, domain.ErrInvalidBackendName),
		errors.Is(err, domain.ErrInvalidPriority),
		errors.Is(err, domain.ErrInvalidSchema):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	// Service unavailable errors
	case errors.Is(err, domain.ErrAIGatewayNotAvailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
