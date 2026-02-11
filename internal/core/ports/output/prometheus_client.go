package ports

import (
	"context"
	"time"
)

// TimeRange for metric queries
type TimeRange struct {
	Start time.Time
	End   time.Time
	Step  time.Duration // e.g., 1m, 5m, 1h
}

// DataPoint represents a single metric value at a point in time
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries represents a time series of metric values
type MetricSeries struct {
	Labels map[string]string `json:"labels"`
	Values []DataPoint       `json:"values"`
}

// TokenUsageMetrics aggregated token metrics
type TokenUsageMetrics struct {
	ProjectID    string  `json:"project_id"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost,omitempty"`
}

// DeploymentMetrics aggregated deployment performance metrics (OpenTelemetry Gen AI)
type DeploymentMetrics struct {
	ISVCName        string  `json:"isvc_name"`
	VariantName     string  `json:"variant_name"`
	ModelName       string  `json:"model_name"`
	Requests        int64   `json:"requests"`
	LatencyP50      float64 `json:"latency_p50_ms"`
	LatencyP99      float64 `json:"latency_p99_ms"`
	TTFTP50         float64 `json:"ttft_p50_ms"`
	TimePerTokenP50 float64 `json:"time_per_token_p50_ms"`
	ErrorRate       float64 `json:"error_rate_percent"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
}

// PrometheusClient defines the contract for Prometheus queries
type PrometheusClient interface {
	// Request duration queries (gen_ai_server_request_duration)
	QueryLatencyP50(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)
	QueryLatencyP99(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)

	// Time to first token (gen_ai_server_time_to_first_token)
	QueryTTFT(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)

	// Time per output token (gen_ai_server_time_per_output_token)
	QueryTimePerToken(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)

	// Throughput queries
	QueryRequestRate(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)
	QueryErrorRate(ctx context.Context, modelName string, tr TimeRange) ([]DataPoint, error)

	// Token usage queries (gen_ai_client_token_usage_sum)
	QueryTokenUsage(ctx context.Context, projectID string, tr TimeRange) (*TokenUsageMetrics, error)
	QueryTokenUsageTimeSeries(ctx context.Context, projectID string, tr TimeRange) ([]MetricSeries, error)
	QueryTokenUsageByModel(ctx context.Context, modelName string, tr TimeRange) (*TokenUsageMetrics, error)

	// Deployment comparison (for A/B testing)
	QueryDeploymentMetrics(ctx context.Context, modelName, variantName string, tr TimeRange) (*DeploymentMetrics, error)
	CompareVariants(ctx context.Context, modelName string, variants []string, tr TimeRange) (map[string]*DeploymentMetrics, error)

	// Health check
	IsAvailable() bool
}
