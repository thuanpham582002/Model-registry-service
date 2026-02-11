package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"model-registry-service/internal/config"
	ports "model-registry-service/internal/core/ports/output"
)

type prometheusClient struct {
	baseURL string
	client  *http.Client
	enabled bool
}

// NewPrometheusClient creates a new Prometheus client adapter
func NewPrometheusClient(cfg *config.PrometheusConfig) ports.PrometheusClient {
	if !cfg.Enabled {
		return &prometheusClient{enabled: false}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &prometheusClient{
		baseURL: cfg.URL,
		enabled: true,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *prometheusClient) IsAvailable() bool {
	if !c.enabled {
		return false
	}

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

func (c *prometheusClient) query(ctx context.Context, promQL string, tr ports.TimeRange) (*promResponse, error) {
	if !c.enabled {
		return &promResponse{Status: "success"}, nil
	}

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

func (c *prometheusClient) instantQuery(ctx context.Context, promQL string) ([]ports.DataPoint, error) {
	if !c.enabled {
		return nil, nil
	}

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

	var points []ports.DataPoint
	for _, r := range promResp.Data.Result {
		if len(r.Value) >= 2 {
			ts, _ := r.Value[0].(float64)
			valStr, _ := r.Value[1].(string)
			val, _ := strconv.ParseFloat(valStr, 64)
			points = append(points, ports.DataPoint{
				Timestamp: time.Unix(int64(ts), 0),
				Value:     val,
			})
		}
	}
	return points, nil
}

func (c *prometheusClient) parseDataPoints(result []promResult) []ports.DataPoint {
	if len(result) == 0 {
		return nil
	}

	var points []ports.DataPoint
	for _, r := range result {
		for _, v := range r.Values {
			if len(v) >= 2 {
				ts, _ := v[0].(float64)
				valStr, _ := v[1].(string)
				val, _ := strconv.ParseFloat(valStr, 64)
				points = append(points, ports.DataPoint{
					Timestamp: time.Unix(int64(ts), 0),
					Value:     val,
				})
			}
		}
	}
	return points
}

// --- Latency Queries ---

func (c *prometheusClient) QueryLatencyP50(ctx context.Context, isvcName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
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

func (c *prometheusClient) QueryLatencyP99(ctx context.Context, isvcName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
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

// --- Gen AI Metrics (OpenTelemetry) ---

func (c *prometheusClient) QueryTTFT(ctx context.Context, modelName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
	// Time to first token (gen_ai_server_time_to_first_token)
	promQL := fmt.Sprintf(`
		histogram_quantile(0.50,
			sum(rate(gen_ai_server_time_to_first_token_bucket{gen_ai_request_model=~"%s.*"}[5m])) by (le)
		)
	`, modelName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

func (c *prometheusClient) QueryTimePerToken(ctx context.Context, modelName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
	// Time per output token (gen_ai_server_time_per_output_token)
	promQL := fmt.Sprintf(`
		histogram_quantile(0.50,
			sum(rate(gen_ai_server_time_per_output_token_bucket{gen_ai_request_model=~"%s.*"}[5m])) by (le)
		)
	`, modelName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

// --- Throughput Queries ---

func (c *prometheusClient) QueryRequestRate(ctx context.Context, isvcName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
	promQL := fmt.Sprintf(`
		sum(rate(revision_request_count{revision_name=~"%s.*"}[5m]))
	`, isvcName)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}
	return c.parseDataPoints(resp.Data.Result), nil
}

func (c *prometheusClient) QueryErrorRate(ctx context.Context, isvcName string, tr ports.TimeRange) ([]ports.DataPoint, error) {
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

func (c *prometheusClient) QueryTokenUsage(ctx context.Context, projectID string, tr ports.TimeRange) (*ports.TokenUsageMetrics, error) {
	duration := tr.End.Sub(tr.Start).String()

	// AI Gateway emits gen_ai_client_token_usage_sum metric
	inputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{x_project_id="%s", gen_ai_token_type="input"}[%s]))
	`, projectID, duration)

	outputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{x_project_id="%s", gen_ai_token_type="output"}[%s]))
	`, projectID, duration)

	inputResp, _ := c.instantQuery(ctx, inputQL)
	outputResp, _ := c.instantQuery(ctx, outputQL)

	var inputTokens, outputTokens int64
	if len(inputResp) > 0 {
		inputTokens = int64(inputResp[0].Value)
	}
	if len(outputResp) > 0 {
		outputTokens = int64(outputResp[0].Value)
	}

	return &ports.TokenUsageMetrics{
		ProjectID:    projectID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}, nil
}

func (c *prometheusClient) QueryTokenUsageTimeSeries(ctx context.Context, projectID string, tr ports.TimeRange) ([]ports.MetricSeries, error) {
	promQL := fmt.Sprintf(`
		sum by (gen_ai_token_type) (rate(gen_ai_client_token_usage_sum{x_project_id="%s"}[5m]))
	`, projectID)

	resp, err := c.query(ctx, promQL, tr)
	if err != nil {
		return nil, err
	}

	var series []ports.MetricSeries
	for _, r := range resp.Data.Result {
		ms := ports.MetricSeries{
			Labels: r.Metric,
			Values: []ports.DataPoint{},
		}
		for _, v := range r.Values {
			if len(v) >= 2 {
				ts, _ := v[0].(float64)
				valStr, _ := v[1].(string)
				val, _ := strconv.ParseFloat(valStr, 64)
				ms.Values = append(ms.Values, ports.DataPoint{
					Timestamp: time.Unix(int64(ts), 0),
					Value:     val,
				})
			}
		}
		series = append(series, ms)
	}
	return series, nil
}

func (c *prometheusClient) QueryTokenUsageByModel(ctx context.Context, modelName string, tr ports.TimeRange) (*ports.TokenUsageMetrics, error) {
	duration := tr.End.Sub(tr.Start).String()

	inputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{gen_ai_request_model="%s", gen_ai_token_type="input"}[%s]))
	`, modelName, duration)

	outputQL := fmt.Sprintf(`
		sum(increase(gen_ai_client_token_usage_sum{gen_ai_request_model="%s", gen_ai_token_type="output"}[%s]))
	`, modelName, duration)

	inputResp, _ := c.instantQuery(ctx, inputQL)
	outputResp, _ := c.instantQuery(ctx, outputQL)

	var inputTokens, outputTokens int64
	if len(inputResp) > 0 {
		inputTokens = int64(inputResp[0].Value)
	}
	if len(outputResp) > 0 {
		outputTokens = int64(outputResp[0].Value)
	}

	return &ports.TokenUsageMetrics{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}, nil
}

// --- Deployment Comparison ---

func (c *prometheusClient) QueryDeploymentMetrics(ctx context.Context, isvcName, variantName string, tr ports.TimeRange) (*ports.DeploymentMetrics, error) {
	revisionName := isvcName
	if variantName != "" {
		revisionName = fmt.Sprintf("%s-%s", isvcName, variantName)
	}

	duration := tr.End.Sub(tr.Start).String()

	// Get aggregated metrics
	requestsQL := fmt.Sprintf(`sum(increase(revision_request_count{revision_name=~"%s.*"}[%s]))`, revisionName, duration)
	latencyP50QL := fmt.Sprintf(`histogram_quantile(0.50, sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le))`, revisionName)
	latencyP99QL := fmt.Sprintf(`histogram_quantile(0.99, sum(rate(revision_request_latencies_bucket{revision_name=~"%s.*"}[5m])) by (le))`, revisionName)
	errorRateQL := fmt.Sprintf(`sum(rate(revision_request_count{revision_name=~"%s.*", response_code!="200"}[5m])) / sum(rate(revision_request_count{revision_name=~"%s.*"}[5m])) * 100`, revisionName, revisionName)

	requests, _ := c.instantQuery(ctx, requestsQL)
	latencyP50, _ := c.instantQuery(ctx, latencyP50QL)
	latencyP99, _ := c.instantQuery(ctx, latencyP99QL)
	errorRate, _ := c.instantQuery(ctx, errorRateQL)

	metrics := &ports.DeploymentMetrics{
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

func (c *prometheusClient) CompareVariants(ctx context.Context, isvcName string, variants []string, tr ports.TimeRange) (map[string]*ports.DeploymentMetrics, error) {
	result := make(map[string]*ports.DeploymentMetrics)
	for _, variant := range variants {
		metrics, err := c.QueryDeploymentMetrics(ctx, isvcName, variant, tr)
		if err != nil {
			continue
		}
		result[variant] = metrics
	}
	return result, nil
}
