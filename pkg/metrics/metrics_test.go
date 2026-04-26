package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistration(t *testing.T) {
	assert.NotNil(t, HTTPRequestsTotal)
	assert.NotNil(t, HTTPRequestDuration)
	assert.NotNil(t, GRPCRequestsTotal)
	assert.NotNil(t, GRPCRequestDuration)
}

func TestHTTPMetrics(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("test-service", "GET", "/test", "200").Inc()
	HTTPRequestDuration.WithLabelValues("test-service", "GET", "/test").Observe(0.1)
	
	count := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("test-service", "GET", "/test", "200"))
	assert.Equal(t, float64(1), count)
}

func TestHandler(t *testing.T) {
	handler := Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "kd48_http_requests_total")
}