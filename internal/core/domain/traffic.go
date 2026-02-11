package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Traffic Strategy Types
// ============================================================================

// TrafficStrategy defines the traffic management strategy
type TrafficStrategy string

const (
	TrafficStrategyCanary    TrafficStrategy = "canary"
	TrafficStrategyABTest    TrafficStrategy = "ab_test"
	TrafficStrategyShadow    TrafficStrategy = "shadow"
	TrafficStrategyBlueGreen TrafficStrategy = "blue_green"
)

// VariantStatus defines the status of a traffic variant
type VariantStatus string

const (
	VariantStatusPending   VariantStatus = "pending"
	VariantStatusActive    VariantStatus = "active"
	VariantStatusPromoting VariantStatus = "promoting"
	VariantStatusDraining  VariantStatus = "draining"
	VariantStatusInactive  VariantStatus = "inactive"
)

// ============================================================================
// Traffic Configuration
// ============================================================================

// TrafficConfig represents traffic management configuration for an inference service
type TrafficConfig struct {
	ID                 uuid.UUID       `json:"id"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	ProjectID          uuid.UUID       `json:"project_id"`
	InferenceServiceID uuid.UUID       `json:"inference_service_id"`
	Strategy           TrafficStrategy `json:"strategy"`
	AIGatewayRouteName string          `json:"ai_gateway_route_name,omitempty"`
	Status             string          `json:"status"`

	// Computed/joined fields
	Variants             []*TrafficVariant `json:"variants,omitempty"`
	InferenceServiceName string            `json:"inference_service_name,omitempty"`
}

// NewTrafficConfig creates a new TrafficConfig
func NewTrafficConfig(projectID, isvcID uuid.UUID, strategy TrafficStrategy) (*TrafficConfig, error) {
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if isvcID == uuid.Nil {
		return nil, ErrInvalidInferenceServiceID
	}

	now := time.Now()
	return &TrafficConfig{
		ID:                 uuid.New(),
		CreatedAt:          now,
		UpdatedAt:          now,
		ProjectID:          projectID,
		InferenceServiceID: isvcID,
		Strategy:           strategy,
		Status:             "active",
	}, nil
}

// GetVariant returns variant by name
func (c *TrafficConfig) GetVariant(name string) *TrafficVariant {
	for _, v := range c.Variants {
		if v.VariantName == name {
			return v
		}
	}
	return nil
}

// HasVariant checks if variant exists
func (c *TrafficConfig) HasVariant(name string) bool {
	return c.GetVariant(name) != nil
}

// GetActiveVariants returns all active variants
func (c *TrafficConfig) GetActiveVariants() []*TrafficVariant {
	var active []*TrafficVariant
	for _, v := range c.Variants {
		if v.Status == VariantStatusActive && v.Weight > 0 {
			active = append(active, v)
		}
	}
	return active
}

// TotalWeight returns sum of all active variant weights
func (c *TrafficConfig) TotalWeight() int {
	total := 0
	for _, v := range c.Variants {
		if v.Status == VariantStatusActive {
			total += v.Weight
		}
	}
	return total
}

// ValidateWeights checks if total weights <= 100
func (c *TrafficConfig) ValidateWeights() error {
	if c.TotalWeight() > 100 {
		return ErrWeightSumExceeds100
	}
	return nil
}

// ============================================================================
// Traffic Variant
// ============================================================================

// TrafficVariant represents a traffic variant (stable, canary, etc.)
type TrafficVariant struct {
	ID              uuid.UUID     `json:"id"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	TrafficConfigID uuid.UUID     `json:"traffic_config_id"`
	VariantName     string        `json:"variant_name"`
	ModelVersionID  uuid.UUID     `json:"model_version_id"`
	Weight          int           `json:"weight"`
	KServeISVCName  string        `json:"kserve_isvc_name,omitempty"`
	KServeRevision  string        `json:"kserve_revision,omitempty"`
	Status          VariantStatus `json:"status"`

	// Computed/joined fields
	ModelVersionName string `json:"model_version_name,omitempty"`
}

// NewTrafficVariant creates a new TrafficVariant
func NewTrafficVariant(configID, versionID uuid.UUID, name string, weight int) (*TrafficVariant, error) {
	if name == "" {
		return nil, ErrInvalidVariantName
	}
	if weight < 0 || weight > 100 {
		return nil, ErrInvalidTrafficWeight
	}

	now := time.Now()
	return &TrafficVariant{
		ID:              uuid.New(),
		CreatedAt:       now,
		UpdatedAt:       now,
		TrafficConfigID: configID,
		VariantName:     name,
		ModelVersionID:  versionID,
		Weight:          weight,
		Status:          VariantStatusPending,
	}, nil
}

// SetWeight updates the weight
func (v *TrafficVariant) SetWeight(weight int) error {
	if weight < 0 || weight > 100 {
		return ErrInvalidTrafficWeight
	}
	v.Weight = weight
	v.UpdatedAt = time.Now()
	return nil
}

// Activate marks the variant as active
func (v *TrafficVariant) Activate() {
	v.Status = VariantStatusActive
	v.UpdatedAt = time.Now()
}

// Deactivate marks the variant as inactive
func (v *TrafficVariant) Deactivate() {
	v.Status = VariantStatusInactive
	v.Weight = 0
	v.UpdatedAt = time.Now()
}
