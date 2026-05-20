package telemetry

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// MetricsHandler returns an http.Handler that serves Prometheus-format metrics
// collected via the OTel SDK metric pipeline.  The OTel Prometheus exporter
// bridges OTel instruments into the default Prometheus registry, and promhttp
// serves that registry over HTTP.
func MetricsHandler() (http.Handler, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)
	return promhttp.Handler(), nil
}
