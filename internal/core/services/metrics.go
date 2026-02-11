package services

import (
	"context"
	"time"

	ports "model-registry-service/internal/core/ports/output"
)

// MetricsService handles metrics operations
type MetricsService struct {
	prometheus ports.PrometheusClient
	isvcRepo   ports.InferenceServiceRepository
}

// NewMetricsService creates a new metrics service
func NewMetricsService(
	prometheus ports.PrometheusClient,
	isvcRepo ports.InferenceServiceRepository,
) *MetricsService {
	return &MetricsService{
		prometheus: prometheus,
		isvcRepo:   isvcRepo,
	}
}

// DeploymentMetricsRequest contains parameters for deployment metrics request
type DeploymentMetricsRequest struct {
	ISVCName    string
	VariantName string
	From        time.Time
	To          time.Time
	Step        time.Duration
}

// TimeRangeInfo contains time range information for responses
type TimeRangeInfo struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	Step string    `json:"step"`
}

// DeploymentMetricsResponse contains deployment metrics response
type DeploymentMetricsResponse struct {
	ISVC        string                   `json:"isvc_name"`
	Variant     string                   `json:"variant_name"`
	TimeRange   TimeRangeInfo            `json:"time_range"`
	Summary     *ports.DeploymentMetrics `json:"summary"`
	LatencyP50  []ports.DataPoint        `json:"latency_p50,omitempty"`
	LatencyP99  []ports.DataPoint        `json:"latency_p99,omitempty"`
	RequestRate []ports.DataPoint        `json:"request_rate,omitempty"`
	ErrorRate   []ports.DataPoint        `json:"error_rate,omitempty"`
}

// GetDeploymentMetrics retrieves deployment metrics
func (s *MetricsService) GetDeploymentMetrics(ctx context.Context, req DeploymentMetricsRequest) (*DeploymentMetricsResponse, error) {
	tr := ports.TimeRange{
		Start: req.From,
		End:   req.To,
		Step:  req.Step,
	}

	// Get summary metrics
	summary, err := s.prometheus.QueryDeploymentMetrics(ctx, req.ISVCName, req.VariantName, tr)
	if err != nil {
		return nil, err
	}

	// Get time series
	latencyP50, _ := s.prometheus.QueryLatencyP50(ctx, req.ISVCName, tr)
	latencyP99, _ := s.prometheus.QueryLatencyP99(ctx, req.ISVCName, tr)
	requestRate, _ := s.prometheus.QueryRequestRate(ctx, req.ISVCName, tr)
	errorRate, _ := s.prometheus.QueryErrorRate(ctx, req.ISVCName, tr)

	return &DeploymentMetricsResponse{
		ISVC:    req.ISVCName,
		Variant: req.VariantName,
		TimeRange: TimeRangeInfo{
			From: req.From,
			To:   req.To,
			Step: req.Step.String(),
		},
		Summary:     summary,
		LatencyP50:  latencyP50,
		LatencyP99:  latencyP99,
		RequestRate: requestRate,
		ErrorRate:   errorRate,
	}, nil
}

// VariantComparisonRequest contains parameters for variant comparison request
type VariantComparisonRequest struct {
	ISVCName string
	Variants []string
	From     time.Time
	To       time.Time
}

// VariantComparisonResponse contains variant comparison response
type VariantComparisonResponse struct {
	ISVC      string                              `json:"isvc_name"`
	Variants  map[string]*ports.DeploymentMetrics `json:"variants"`
	Winner    string                              `json:"winner,omitempty"`
	TimeRange TimeRangeInfo                       `json:"time_range"`
}

// CompareVariants compares metrics between variants
func (s *MetricsService) CompareVariants(ctx context.Context, req VariantComparisonRequest) (*VariantComparisonResponse, error) {
	tr := ports.TimeRange{
		Start: req.From,
		End:   req.To,
		Step:  time.Minute * 5,
	}

	variants, err := s.prometheus.CompareVariants(ctx, req.ISVCName, req.Variants, tr)
	if err != nil {
		return nil, err
	}

	// Determine winner based on latency and error rate
	winner := s.determineWinner(variants)

	return &VariantComparisonResponse{
		ISVC:     req.ISVCName,
		Variants: variants,
		Winner:   winner,
		TimeRange: TimeRangeInfo{
			From: req.From,
			To:   req.To,
		},
	}, nil
}

func (s *MetricsService) determineWinner(variants map[string]*ports.DeploymentMetrics) string {
	var winner string
	var bestScore float64 = -1

	for name, m := range variants {
		// Score = (1 / latency) * (1 - error_rate)
		// Higher is better
		if m.LatencyP99 > 0 {
			score := (1 / m.LatencyP99) * (1 - m.ErrorRate/100)
			if score > bestScore {
				bestScore = score
				winner = name
			}
		}
	}
	return winner
}

// TokenUsageRequest contains parameters for token usage request
type TokenUsageRequest struct {
	ProjectID string
	From      time.Time
	To        time.Time
	Step      time.Duration
}

// TokenUsageResponse contains token usage response
type TokenUsageResponse struct {
	ProjectID  string                   `json:"project_id"`
	Summary    *ports.TokenUsageMetrics `json:"summary"`
	TimeSeries []ports.MetricSeries     `json:"time_series,omitempty"`
	TimeRange  TimeRangeInfo            `json:"time_range"`
}

// GetTokenUsage retrieves token usage metrics
func (s *MetricsService) GetTokenUsage(ctx context.Context, req TokenUsageRequest) (*TokenUsageResponse, error) {
	tr := ports.TimeRange{
		Start: req.From,
		End:   req.To,
		Step:  req.Step,
	}

	summary, err := s.prometheus.QueryTokenUsage(ctx, req.ProjectID, tr)
	if err != nil {
		return nil, err
	}

	timeSeries, _ := s.prometheus.QueryTokenUsageTimeSeries(ctx, req.ProjectID, tr)

	return &TokenUsageResponse{
		ProjectID:  req.ProjectID,
		Summary:    summary,
		TimeSeries: timeSeries,
		TimeRange: TimeRangeInfo{
			From: req.From,
			To:   req.To,
			Step: req.Step.String(),
		},
	}, nil
}

// IsAvailable checks if Prometheus is available
func (s *MetricsService) IsAvailable() bool {
	if s.prometheus == nil {
		return false
	}
	return s.prometheus.IsAvailable()
}
