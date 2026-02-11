# Phase 4: Metrics Integration

## Objective

Integrate Prometheus metrics for traffic monitoring, token usage tracking, and deployment performance visualization through existing Grafana stack. Uses OpenTelemetry Gen AI Semantic Conventions as exposed by AI Gateway.

**Key Metrics (OpenTelemetry Gen AI):**
- `gen_ai_client_token_usage_sum` - Token usage (input, output, total)
- `gen_ai_server_request_duration` - Request duration histogram
- `gen_ai_server_time_to_first_token` - Time to first token (TTFT)
- `gen_ai_server_time_per_output_token` - Inter-token latency

**Key Attributes:**
- `gen_ai_request_model` - Requested model name
- `gen_ai_response_model` - Response model name
- `gen_ai_token_type` - input, output, total
- `gen_ai_operation_name` - chat, completion, embedding, etc.

---

## 4.1 Prometheus Client Port

**File**: `internal/core/ports/output/prometheus_client.go`

```go
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
	ISVCName          string  `json:"isvc_name"`
	VariantName       string  `json:"variant_name"`
	ModelName         string  `json:"model_name"`          // gen_ai_request_model
	Requests          int64   `json:"requests"`
	LatencyP50        float64 `json:"latency_p50_ms"`      // gen_ai_server_request_duration
	LatencyP99        float64 `json:"latency_p99_ms"`
	TTFTP50           float64 `json:"ttft_p50_ms"`         // gen_ai_server_time_to_first_token
	TimePerTokenP50   float64 `json:"time_per_token_p50_ms"` // gen_ai_server_time_per_output_token
	ErrorRate         float64 `json:"error_rate_percent"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
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
```

---

## 4.2 Prometheus Adapter

**File**: `internal/adapters/secondary/prometheus/client.go`

```go
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	output "model-registry-service/internal/core/ports/output"
)

type prometheusClient struct {
	baseURL string
	client  *http.Client
}

func NewPrometheusClient(baseURL string) output.PrometheusClient {
	return &prometheusClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *prometheusClient) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/-/healthy", nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Prometheus API response structures
type promResponse struct {
	Status string   `json:"status"`
	Data   promData `json:"data"`
}

type promData struct {
	ResultType string       `json:"resultType"`
	Result     []promResult `json:"result"`
}

type promResult struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"` // [timestamp, value]
	Value  []interface{}     `json:"value"`  // for instant queries
}

func (c *prometheusClient) query(ctx context.Context, promQL string, tr output.TimeRange) (*promResponse, error) {
	params := url.Values{}
	params.Set("query", promQL)
	params.Set("start", strconv.FormatInt(tr.Start.Unix(), 10))
	params.Set("end", strconv.FormatInt(tr.End.Unix(), 10))
	params.Set("step", tr.Step.String())

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var promResp promResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, err
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", promResp.Status)
	}

	return &promResp, nil
}

func (c *prometheusClient) parseDataPoints(result []promResult) []output.DataPoint {
	if len(result) == 0 {
		return nil
	}

	var points []output.DataPoint
	for _, r := range result {
		for _, v := range r.Values {
			if len(v) >= 2 {
				ts, _ := v[0].(float64)
				valStr, _ := v[1].(string)
				val, _ := strconv.ParseFloat(valStr, 64)
				points = append(points, output.DataPoint{
					Timestamp: time.Unix(int64(ts), 0),
					Value:     val,
				})
			}
		}
	}
	return points
}

// --- Latency Queries ---

func (c *prometheusClient) QueryLatencyP50(ctx context.Context, isvcName string, tr output.TimeRange) ([]output.DataPoint, error) {
	// KServe latency histogram
	promQL := fmt.Sprintf(`
		histogram_quantile(0.50,
			sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le)
		)
	`, isvcName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

func (c *prometheusClient) QueryLatencyP99(ctx context.Context, isvcName string, tr output.TimeRange) ([]output.DataPoint, error) {
	promQL := fmt.Sprintf(`
		histogram_quantile(0.99,
			sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le)
		)
	`, isvcName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

// --- Throughput Queries ---

func (c *prometheusClient) QueryRequestRate(ctx context.Context, isvcName string, tr output.TimeRange) ([]output.DataPoint, error) {
	promQL := fmt.Sprintf(`
		sum(rate(revision_request_count{revision_name=~"%s.*"}[5m]))
	`, isvcName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

func (c *prometheusClient) QueryErrorRate(ctx context.Context, isvcName string, tr output.TimeRange) ([]output.DataPoint, error) {
	promQL := fmt.Sprintf(`
		sum(rate(revision_request_count{revision_name=~"%s.*", response_code!="200"}[5m])) /
		sum(rate(revision_request_count{revision_name=~"%s.*"}[5m])) * 100
	`, isvcName, isvcName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

// --- Token Usage Queries (AI Gateway OpenTelemetry Gen AI metrics) ---

func (c *prometheusClient) QueryTokenUsage(ctx context.Context, projectID string, tr output.TimeRange) (*output.TokenUsageMetrics, error) {
	// AI Gateway emits gen_ai_client_token_usage_sum metric (OpenTelemetry Gen AI convention)
	// Filter by gateway label that contains project info from x-project-id header
	inputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{x_project_id="%s", gen_ai_token_type="input"}[%s]))
	`, projectID, tr.End.Sub(tr.Start).String())

	outputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{x_project_id="%s", gen_ai_token_type="output"}[%s]))
	`, projectID, tr.End.Sub(tr.Start).String())

	// Instant queries for totals
	inputResp, _ := c.instantQuery(ctx, inputQL)
	outputResp, _ := c.instantQuery(ctx, outputQL)

	var inputTokens, outputTokens int64
	if len(inputResp) > 0 {
		inputTokens = int64(inputResp[0].Value)
	}
	if len(outputResp) > 0 {
		outputTokens = int64(outputResp[0].Value)
	}

	return &output.TokenUsageMetrics{
		ProjectID:    projectID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}, nil
}

func (c *prometheusClient) QueryTokenUsageTimeSeries(ctx context.Context, projectID string, tr output.TimeRange) ([]output.MetricSeries, error) {
	promQL := fmt.Sprintf(`
		sum by (gen_ai_token_type) (rate(gen_ai_client_token_usage_sum{x_project_id="%s"}[5m]))
	`, projectID)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}

	var series []output.MetricSeries
	for _, r := range resp.Data.Result {
		ms := output.MetricSeries{
			Labels: r.Metric,
			Values: []output.DataPoint{},
		}
		for _, v := range r.Values {
			if len(v) >= 2 {
				ts, _ := v[0].(float64)
				valStr, _ := v[1].(string)
				val, _ := strconv.ParseFloat(valStr, 64)
				ms.Values = append(ms.Values, output.DataPoint{
					Timestamp: time.Unix(int64(ts), 0),
					Value:     val,
				})
			}
		}
		series = append(series, ms)
	}
	return series, nil
}

// --- Deployment Comparison ---

func (c *prometheusClient) QueryDeploymentMetrics(ctx context.Context, isvcName, variantName string, tr output.TimeRange) (*output.DeploymentMetrics, error) {
	revisionName := fmt.Sprintf("%s-%s", isvcName, variantName)

	// Get aggregated metrics
	requestsQL := fmt.Sprintf(`sum(increase(revision_request_count{revision_name=~"%s.*"}[%s]))`, revisionName, tr.End.Sub(tr.Start).String())
	latencyP50QL := fmt.Sprintf(`histogram_quantile(0.50, sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le))`, revisionName)
	latencyP99QL := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le))`, revisionName)
	errorRateQL := fmt.Sprintf(`sum(rate(revision_request_count{revision_name=~"%s.*", response_code!="200"}[5m])) / sum(rate(revision_request_count{revision_name=~"%s.*"}[5m])) * 100`, revisionName, revisionName)

	requests, _ := c.instantQuery(ctx, requestsQL)
	latencyP50, _ := c.instantQuery(ctx, latencyP50QL)
	latencyP99, _ := c.instantQuery(ctx, latencyP99QL)
	errorRate, _ := c.instantQuery(ctx, errorRateQL)

	metrics := &output.DeploymentMetrics{
		ISVCName:    isvcName,
		VariantName: variantName,
	}
	if len(requests) > 0 {
		metrics.Requests = int64(requests[0].Value)
	}
	if len(latencyP50) > 0 {
		metrics.LatencyP50 = latencyP50[0].Value
	}
	if len(latencyP99) > 0 {
		metrics.LatencyP99 = latencyP99[0].Value
	}
	if len(errorRate) > 0 {
		metrics.ErrorRate = errorRate[0].Value
	}

	return metrics, nil
}

func (c *prometheusClient) CompareVariants(ctx context.Context, isvcName string, variants []string, tr output.TimeRange) (map[string]*output.DeploymentMetrics, error) {
	result := make(map[string]*output.DeploymentMetrics)
	for _, variant := range variants {
		metrics, err := c.QueryDeploymentMetrics(ctx, isvcName, variant, tr)
		if err != nil {
			continue
		}
		result[variant] = metrics
	}
	return result, nil
}

func (c *prometheusClient) instantQuery(ctx context.Context, promQL string) ([]output.DataPoint, error) {
	params := url.Values{}
	params.Set("query", promQL)

	reqURL := fmt.Sprintf("%s/api/v1/query?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var promResp promResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, err
	}

	var points []output.DataPoint
	for _, r := range promResp.Data.Result {
		if len(r.Value) >= 2 {
			ts, _ := r.Value[0].(float64)
			valStr, _ := r.Value[1].(string)
			val, _ := strconv.ParseFloat(valStr, 64)
			points = append(points, output.DataPoint{
				Timestamp: time.Unix(int64(ts), 0),
				Value:     val,
			})
		}
	}
	return points, nil
}
```

---

## 4.3 Metrics Service

**File**: `internal/core/services/metrics.go`

```go
package services

import (
	"context"
	"time"

	output "model-registry-service/internal/core/ports/output"
)

type MetricsService struct {
	prometheus output.PrometheusClient
	isvcRepo   output.InferenceServiceRepository
}

func NewMetricsService(
	prometheus output.PrometheusClient,
	isvcRepo output.InferenceServiceRepository,
) *MetricsService {
	return &MetricsService{
		prometheus: prometheus,
		isvcRepo:   isvcRepo,
	}
}

type DeploymentMetricsRequest struct {
	ISVCName    string
	VariantName string
	From        time.Time
	To          time.Time
	Step        time.Duration
}

type DeploymentMetricsResponse struct {
	ISVC         string                      `json:"isvc_name"`
	Variant      string                      `json:"variant_name"`
	TimeRange    TimeRangeInfo               `json:"time_range"`
	Summary      *output.DeploymentMetrics   `json:"summary"`
	LatencyP50   []output.DataPoint          `json:"latency_p50,omitempty"`
	LatencyP99   []output.DataPoint          `json:"latency_p99,omitempty"`
	RequestRate  []output.DataPoint          `json:"request_rate,omitempty"`
	ErrorRate    []output.DataPoint          `json:"error_rate,omitempty"`
}

type TimeRangeInfo struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	Step string    `json:"step"`
}

func (s *MetricsService) GetDeploymentMetrics(ctx context.Context, req DeploymentMetricsRequest) (*DeploymentMetricsResponse, error) {
	tr := output.TimeRange{
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

type VariantComparisonRequest struct {
	ISVCName string
	Variants []string
	From     time.Time
	To       time.Time
}

type VariantComparisonResponse struct {
	ISVC      string                              `json:"isvc_name"`
	Variants  map[string]*output.DeploymentMetrics `json:"variants"`
	Winner    string                              `json:"winner,omitempty"`
	TimeRange TimeRangeInfo                       `json:"time_range"`
}

func (s *MetricsService) CompareVariants(ctx context.Context, req VariantComparisonRequest) (*VariantComparisonResponse, error) {
	tr := output.TimeRange{
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

func (s *MetricsService) determineWinner(variants map[string]*output.DeploymentMetrics) string {
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

type TokenUsageRequest struct {
	ProjectID string
	From      time.Time
	To        time.Time
	Step      time.Duration
}

type TokenUsageResponse struct {
	ProjectID  string                    `json:"project_id"`
	Summary    *output.TokenUsageMetrics `json:"summary"`
	TimeSeries []output.MetricSeries     `json:"time_series,omitempty"`
	TimeRange  TimeRangeInfo             `json:"time_range"`
}

func (s *MetricsService) GetTokenUsage(ctx context.Context, req TokenUsageRequest) (*TokenUsageResponse, error) {
	tr := output.TimeRange{
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
```

---

## 4.4 Metrics Handler

**File**: `internal/adapters/primary/http/handlers/metrics.go`

```go
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

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
```

---

## 4.5 Grafana Dashboard Templates

**File**: `deploy/grafana/model-traffic-dashboard.json`

```json
{
  "title": "Model Registry - Traffic Dashboard",
  "uid": "model-registry-traffic",
  "panels": [
    {
      "title": "Request Rate by Variant",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum by (revision_name) (rate(revision_request_count[5m]))",
          "legendFormat": "{{revision_name}}"
        }
      ]
    },
    {
      "title": "Latency P99",
      "type": "timeseries",
      "targets": [
        {
          "expr": "histogram_quantile(0.99, sum by (le, revision_name) (rate(revision_request_latencies_bucket[5m])))",
          "legendFormat": "{{revision_name}}"
        }
      ]
    },
    {
      "title": "Error Rate",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum by (revision_name) (rate(revision_request_count{response_code!=\"200\"}[5m])) / sum by (revision_name) (rate(revision_request_count[5m])) * 100",
          "legendFormat": "{{revision_name}}"
        }
      ]
    },
    {
      "title": "Traffic Split",
      "type": "piechart",
      "targets": [
        {
          "expr": "sum by (revision_name) (increase(revision_request_count[1h]))",
          "legendFormat": "{{revision_name}}"
        }
      ]
    }
  ]
}
```

**File**: `deploy/grafana/token-usage-dashboard.json`

```json
{
  "title": "Model Registry - Token Usage (OpenTelemetry Gen AI)",
  "uid": "model-registry-tokens",
  "panels": [
    {
      "title": "Token Usage by Model",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum by (gen_ai_request_model) (rate(gen_ai_client_token_usage_sum[5m]))",
          "legendFormat": "{{gen_ai_request_model}}"
        }
      ]
    },
    {
      "title": "Input vs Output Tokens",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum by (gen_ai_token_type) (rate(gen_ai_client_token_usage_sum[5m]))",
          "legendFormat": "{{gen_ai_token_type}}"
        }
      ]
    },
    {
      "title": "Request Duration (P99)",
      "type": "timeseries",
      "targets": [
        {
          "expr": "histogram_quantile(0.99, sum by (le, gen_ai_request_model) (rate(gen_ai_server_request_duration_bucket[5m])))",
          "legendFormat": "{{gen_ai_request_model}}"
        }
      ]
    },
    {
      "title": "Time to First Token",
      "type": "timeseries",
      "targets": [
        {
          "expr": "histogram_quantile(0.50, sum by (le, gen_ai_request_model) (rate(gen_ai_server_time_to_first_token_bucket[5m])))",
          "legendFormat": "{{gen_ai_request_model}}"
        }
      ]
    },
    {
      "title": "Top 10 Models by Token Usage",
      "type": "bargauge",
      "targets": [
        {
          "expr": "topk(10, sum by (gen_ai_request_model) (increase(gen_ai_client_token_usage_sum[24h])))"
        }
      ]
    }
  ]
}
```

---

## 4.6 Config Update

**File**: `internal/config/config.go` (add)

```go
type PrometheusConfig struct {
	Enabled bool
	URL     string
	Timeout time.Duration
}

// In Load():
v.SetDefault("PROMETHEUS_ENABLED", true)
v.SetDefault("PROMETHEUS_URL", "http://prometheus:9090")
v.SetDefault("PROMETHEUS_TIMEOUT", "30s")

cfg.Prometheus = PrometheusConfig{
	Enabled: v.GetBool("PROMETHEUS_ENABLED"),
	URL:     v.GetString("PROMETHEUS_URL"),
	Timeout: v.GetDuration("PROMETHEUS_TIMEOUT"),
}
```

---

## Important Notes

**Header Attribute Mapping**: AI Gateway can extract custom headers as metric attributes.
Configure `controller.requestHeaderAttributes` in AI Gateway Helm values to map `x-project-id` → `x_project_id` attribute.

```yaml
controller:
  requestHeaderAttributes:
    - x-project-id
```

---

## Checklist

- [ ] Create `internal/core/ports/output/prometheus_client.go`
- [ ] Create `internal/adapters/secondary/prometheus/client.go`
- [ ] Implement queries for OpenTelemetry Gen AI metrics:
  - [ ] `gen_ai_client_token_usage_sum` (token counting)
  - [ ] `gen_ai_server_request_duration` (latency)
  - [ ] `gen_ai_server_time_to_first_token` (TTFT)
  - [ ] `gen_ai_server_time_per_output_token` (inter-token latency)
- [ ] Create `internal/core/services/metrics.go`
- [ ] Create `internal/adapters/primary/http/handlers/metrics.go`
- [ ] Update config with Prometheus settings
- [ ] Create Grafana dashboard JSON templates
- [ ] Add metrics routes to handler.go
- [ ] Wire MetricsService in main.go
- [ ] Verify AI Gateway header attribute mapping is configured
- [ ] Test with sample Prometheus data from AI Gateway
- [ ] Document dashboard import process
