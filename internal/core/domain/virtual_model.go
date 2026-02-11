package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Virtual Model
// ============================================================================

// VirtualModel represents a virtual model name that maps to multiple backends
type VirtualModel struct {
	ID                 uuid.UUID `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	ProjectID          uuid.UUID `json:"project_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	AIGatewayRouteName string    `json:"ai_gateway_route_name,omitempty"`
	Status             string    `json:"status"`

	// Computed/joined fields
	Backends []*VirtualModelBackend `json:"backends,omitempty"`
}

// NewVirtualModel creates a new VirtualModel
func NewVirtualModel(projectID uuid.UUID, name string) (*VirtualModel, error) {
	if projectID == uuid.Nil {
		return nil, ErrMissingProjectID
	}
	if name == "" {
		return nil, ErrInvalidVirtualModelName
	}

	now := time.Now()
	return &VirtualModel{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		ProjectID: projectID,
		Name:      name,
		Status:    "active",
	}, nil
}

// GetBackend returns backend by AI service backend name
func (vm *VirtualModel) GetBackend(backendName string) *VirtualModelBackend {
	for _, b := range vm.Backends {
		if b.AIServiceBackendName == backendName {
			return b
		}
	}
	return nil
}

// GetBackendByID returns backend by ID
func (vm *VirtualModel) GetBackendByID(id uuid.UUID) *VirtualModelBackend {
	for _, b := range vm.Backends {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// GetActiveBackends returns all active backends sorted by priority
func (vm *VirtualModel) GetActiveBackends() []*VirtualModelBackend {
	var active []*VirtualModelBackend
	for _, b := range vm.Backends {
		if b.Status == "active" {
			active = append(active, b)
		}
	}
	return active
}

// TotalWeight returns sum of all active backend weights
func (vm *VirtualModel) TotalWeight() int {
	total := 0
	for _, b := range vm.Backends {
		if b.Status == "active" {
			total += b.Weight
		}
	}
	return total
}

// ============================================================================
// Virtual Model Backend
// ============================================================================

// VirtualModelBackend represents a backend mapping for a virtual model
type VirtualModelBackend struct {
	ID                        uuid.UUID `json:"id"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
	VirtualModelID            uuid.UUID `json:"virtual_model_id"`
	AIServiceBackendName      string    `json:"ai_service_backend_name"`
	AIServiceBackendNamespace string    `json:"ai_service_backend_namespace,omitempty"`
	ModelNameOverride         *string   `json:"model_name_override,omitempty"` // nil = use virtual model name
	Weight                    int       `json:"weight"`
	Priority                  int       `json:"priority"` // 0 = primary, 1+ = fallback
	Status                    string    `json:"status"`
}

// NewVirtualModelBackend creates a new VirtualModelBackend
func NewVirtualModelBackend(vmID uuid.UUID, backendName string, modelOverride *string, weight, priority int) (*VirtualModelBackend, error) {
	if backendName == "" {
		return nil, ErrInvalidBackendName
	}
	if weight < 0 || weight > 100 {
		return nil, ErrInvalidTrafficWeight
	}
	if priority < 0 {
		return nil, ErrInvalidPriority
	}

	now := time.Now()
	return &VirtualModelBackend{
		ID:                   uuid.New(),
		CreatedAt:            now,
		UpdatedAt:            now,
		VirtualModelID:       vmID,
		AIServiceBackendName: backendName,
		ModelNameOverride:    modelOverride,
		Weight:               weight,
		Priority:             priority,
		Status:               "active",
	}, nil
}

// GetEffectiveModelName returns the model name to use (override or virtual model name)
func (b *VirtualModelBackend) GetEffectiveModelName(virtualModelName string) string {
	if b.ModelNameOverride != nil && *b.ModelNameOverride != "" {
		return *b.ModelNameOverride
	}
	return virtualModelName
}
